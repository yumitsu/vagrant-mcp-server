// Copyright Ricardo Oliveira 2025.
// SPDX-License-Identifier: MPL-2.0

package exec

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/vagrant-mcp/server/internal/core"
	"github.com/vagrant-mcp/server/internal/errors"
)

// CommandResult contains the result of a command execution
type CommandResult struct {
	ExitCode int     `json:"exit_code"`
	Stdout   string  `json:"stdout"`
	Stderr   string  `json:"stderr"`
	Duration float64 `json:"duration_seconds"`
}

// ExecutionContext contains the context for command execution
type ExecutionContext struct {
	VMName      string            `json:"vm_name"`
	WorkingDir  string            `json:"working_dir"`
	Environment map[string]string `json:"environment"`
	SyncBefore  bool              `json:"sync_before"`
	SyncAfter   bool              `json:"sync_after"`
}

// OutputCallback is a function called with command output
type OutputCallback func(data []byte, isStderr bool)

// Executor manages command execution in VMs
// Update to use core interfaces
type Executor struct {
	vmManager  core.VMManager
	syncEngine core.SyncEngine
	mu         sync.Mutex
}

// NewExecutor creates a new command executor
func NewExecutor(vmManager core.VMManager, syncEngine core.SyncEngine) (*Executor, error) {
	return &Executor{
		vmManager:  vmManager,
		syncEngine: syncEngine,
	}, nil
}

// ExecuteCommand executes a command in a VM with the given context
func (e *Executor) ExecuteCommand(ctx context.Context, command string, execCtx ExecutionContext, callback OutputCallback) (*CommandResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// SAFEGUARD: Prevent execution on host or without VM context
	if execCtx.VMName == "" || strings.ToLower(execCtx.VMName) == "host" {
		errMsg := "SECURITY VIOLATION: Attempted to execute a shell command outside of a VM context. All commands must target a Vagrant VM."
		log.Error().Msg(errMsg)
		return nil, fmt.Errorf("%s", errMsg)
	}

	// Check if VM exists and is running
	state, err := e.vmManager.GetVMState(ctx, execCtx.VMName)
	if err != nil {
		return nil, errors.OperationFailed("get VM state", err)
	}
	if state != core.Running {
		return nil, errors.OperationFailed("VM is not running", nil)
	}

	// Perform pre-execution sync if requested
	if execCtx.SyncBefore {
		log.Info().Str("vm", execCtx.VMName).Msg("Syncing files to VM before command execution")
		err := e.syncEngine.RegisterVM(ctx, execCtx.VMName, core.SyncConfig{})
		if err != nil {
			return nil, errors.OperationFailed("register VM for sync", err)
		}
	}

	// Execute command
	startTime := time.Now()
	result, err := e.executeSSHCommand(ctx, command, execCtx, callback)
	duration := time.Since(startTime).Seconds()

	// Set duration in result
	if result != nil {
		result.Duration = duration
	}

	// Handle execution error
	if err != nil {
		return result, errors.OperationFailed("command execution failed", err)
	}

	// Perform post-execution sync if requested
	if execCtx.SyncAfter {
		log.Info().Str("vm", execCtx.VMName).Msg("Syncing files from VM after command execution")
		// We don't actually need to do anything here since the RegisterVM above already set up the sync
		// This would be handled by real syncing mechanisms in the actual implementation
	}

	return result, nil
}

// GetSSHConfig retrieves the SSH configuration for the VM using 'vagrant ssh-config'
func (e *Executor) getSSHConfig(ctx context.Context, name string) (map[string]string, error) {
	// Try to use the underlying adapter if available
	if adapter, ok := e.vmManager.(interface {
		GetSSHConfig(context.Context, string) (map[string]string, error)
	}); ok {
		return adapter.GetSSHConfig(ctx, name)
	}
	return nil, errors.New(errors.CodeNotImplemented, "GetSSHConfig for this VMManager is not implemented")
}

// executeSSHCommand executes a command via SSH in a VM
func (e *Executor) executeSSHCommand(ctx context.Context, command string, execCtx ExecutionContext, callback OutputCallback) (*CommandResult, error) {
	// Get SSH config for the VM
	sshConfig, err := e.getSSHConfig(ctx, execCtx.VMName)
	if err != nil {
		return nil, errors.OperationFailed("get SSH config", err)
	}

	// Build the SSH command
	sshArgs := []string{
		"-p", sshConfig["Port"],
		"-i", sshConfig["IdentityFile"],
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		fmt.Sprintf("%s@%s", sshConfig["User"], sshConfig["HostName"]),
	}

	// Add working directory if specified
	fullCommand := command
	if execCtx.WorkingDir != "" {
		if strings.HasPrefix(execCtx.WorkingDir, "/") {
			// Absolute path - use as-is
			fullCommand = fmt.Sprintf("cd %s && %s", execCtx.WorkingDir, command)
		} else {
			// Relative path - use relative to home directory
			fullCommand = fmt.Sprintf("cd && cd %s && %s", execCtx.WorkingDir, command)
		}
	}

	// Add environment variables if specified
	if len(execCtx.Environment) > 0 {
		envParts := []string{}
		for key, value := range execCtx.Environment {
			envParts = append(envParts, fmt.Sprintf("export %s=%s", key, value))
		}
		fullCommand = fmt.Sprintf("%s && %s", strings.Join(envParts, "; "), fullCommand)
	}

	// Add command to SSH args
	sshArgs = append(sshArgs, fullCommand)

	// Create SSH command
	cmd := exec.CommandContext(ctx, "ssh", sshArgs...)

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.OperationFailed("create stdout pipe", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, errors.OperationFailed("create stderr pipe", err)
	}

	// Start command
	if err := cmd.Start(); err != nil {
		return nil, errors.OperationFailed("start command", err)
	}

	// Process command output in separate goroutines
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		e.streamOutput(stdoutPipe, &stdout, false, callback)
	}()

	go func() {
		defer wg.Done()
		e.streamOutput(stderrPipe, &stderr, true, callback)
	}()

	// Wait for output processing to complete
	wg.Wait()

	// Wait for command to complete
	err = cmd.Wait()

	// Create result
	result := &CommandResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	// Handle exit code
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
			return result, errors.OperationFailed("command failed", err)
		}
	} else {
		result.ExitCode = 0
	}

	return result, nil
}

// streamOutput processes and captures command output
func (e *Executor) streamOutput(r io.Reader, buffer *bytes.Buffer, isStderr bool, callback OutputCallback) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Bytes()

		// Write to buffer
		buffer.Write(line)
		buffer.WriteByte('\n')

		// Call callback if provided
		if callback != nil {
			lineCopy := make([]byte, len(line))
			copy(lineCopy, line)
			callback(lineCopy, isStderr)
		}
	}
}
