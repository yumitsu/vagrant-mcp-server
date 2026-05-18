// Copyright Ricardo Oliveira 2025.
// SPDX-License-Identifier: MPL-2.0

package exec

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/vagrant-mcp/server/internal/core"
	syncmod "github.com/vagrant-mcp/server/internal/sync"
	"github.com/vagrant-mcp/server/internal/vm"
)

// VMManagerAdapter adapts *vm.Manager to the core.VMManager interface
// Only implements the methods needed by Executor

type VMManagerAdapter struct {
	Real *vm.Manager
}

func (a *VMManagerAdapter) CreateVM(ctx context.Context, name, projectPath string, config core.VMConfig) error {
	return a.Real.CreateVM(ctx, name, projectPath, config)
}
func (a *VMManagerAdapter) RegisterExistingVM(ctx context.Context, name, projectPath string) error {
	return a.Real.RegisterExistingVM(ctx, name, projectPath)
}
func (a *VMManagerAdapter) StartVM(ctx context.Context, name string) error {
	return a.Real.StartVM(ctx, name)
}
func (a *VMManagerAdapter) StopVM(ctx context.Context, name string) error {
	return a.Real.StopVM(ctx, name)
}
func (a *VMManagerAdapter) DestroyVM(ctx context.Context, name string) error {
	return a.Real.DestroyVM(ctx, name)
}
func (a *VMManagerAdapter) GetVMState(ctx context.Context, name string) (core.VMState, error) {
	return a.Real.GetVMState(ctx, name)
}
func (a *VMManagerAdapter) UploadToVM(ctx context.Context, name, source, destination string, compress bool, compressionType string) error {
	return a.Real.UploadToVM(ctx, name, source, destination, compress, compressionType)
}
func (a *VMManagerAdapter) GetSSHConfig(ctx context.Context, name string) (map[string]string, error) {
	return a.Real.GetSSHConfig(ctx, name)
}
func (a *VMManagerAdapter) GetVMConfig(ctx context.Context, name string) (core.VMConfig, error) {
	return a.Real.GetVMConfig(ctx, name)
}
func (a *VMManagerAdapter) UpdateVMConfig(ctx context.Context, name string, config core.VMConfig) error {
	return a.Real.UpdateVMConfig(ctx, name, config)
}
func (a *VMManagerAdapter) GetBaseDir() string {
	return a.Real.GetBaseDir()
}
func (a *VMManagerAdapter) ListVMs(ctx context.Context) ([]string, error) {
	return a.Real.ListVMs(ctx)
}

// ExecuteCommand runs a command in the VM using SSH
func (a *VMManagerAdapter) ExecuteCommand(ctx context.Context, name string, cmd string, args []string, workingDir string) (string, string, int, error) {
	sshConfig, err := a.Real.GetSSHConfig(ctx, name)
	if err != nil {
		return "", "", 1, err
	}
	sshArgs := []string{
		"-p", sshConfig["Port"],
		"-i", sshConfig["IdentityFile"],
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		fmt.Sprintf("%s@%s", sshConfig["User"], sshConfig["HostName"]),
	}
	fullCmd := cmd
	if workingDir != "" {
		fullCmd = fmt.Sprintf("cd %s && %s", workingDir, cmd)
	}
	sshArgs = append(sshArgs, fullCmd)
	c := exec.CommandContext(ctx, "ssh", sshArgs...)
	out, err := c.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	return string(out), "", exitCode, err
}

// SyncEngineAdapter adapts *sync.Engine to the core.SyncEngine interface
// All methods now match core.SyncEngine (context.Context, core types)
type SyncEngineAdapter struct {
	Real *syncmod.Engine
}

func (a *SyncEngineAdapter) RegisterVM(ctx context.Context, vmName string, config core.SyncConfig) error {
	mapped := syncmod.SyncConfig{
		VMName:          config.VMName,
		ProjectPath:     config.ProjectPath,
		Method:          syncmod.SyncMethod(config.Method),
		Direction:       syncmod.SyncDirection(config.Direction),
		ExcludePatterns: config.ExcludePatterns,
		WatchEnabled:    config.WatchEnabled,
		WatchInterval:   config.WatchInterval,
	}
	return a.Real.RegisterVM(vmName, mapped)
}
func (a *SyncEngineAdapter) UnregisterVM(ctx context.Context, vmName string) error {
	return a.Real.UnregisterVM(vmName)
}
func (a *SyncEngineAdapter) SyncToVM(ctx context.Context, vmName string, sourcePath string) (*core.SyncResult, error) {
	r, err := a.Real.SyncToVM(vmName, sourcePath)
	if err != nil {
		return nil, err
	}
	return &core.SyncResult{
		SyncedFiles: r.SyncedFiles,
		SyncTimeMs:  r.SyncTimeMs,
	}, nil
}
func (a *SyncEngineAdapter) SyncFromVM(ctx context.Context, vmName string, sourcePath string) (*core.SyncResult, error) {
	r, err := a.Real.SyncFromVM(vmName, sourcePath)
	if err != nil {
		return nil, err
	}
	return &core.SyncResult{
		SyncedFiles: r.SyncedFiles,
		SyncTimeMs:  r.SyncTimeMs,
	}, nil
}
func (a *SyncEngineAdapter) GetSyncStatus(ctx context.Context, vmName string) (core.SyncStatus, error) {
	s, err := a.Real.GetSyncStatus(vmName)
	if err != nil {
		return core.SyncStatus{}, err
	}
	conflicts := make([]core.SyncConflict, len(s.Conflicts))
	for i, c := range s.Conflicts {
		conflicts[i] = core.SyncConflict{
			Path:         c.Path,
			HostModTime:  c.HostModTime,
			VMModTime:    c.VMModTime,
			HostContent:  c.HostContent,
			VMContent:    c.VMContent,
			ConflictType: c.ConflictType,
		}
	}
	return core.SyncStatus{
		LastSyncTime:         s.LastSyncTime,
		InProgress:           s.InProgress,
		Conflicts:            conflicts,
		SynchronizedFiles:    s.SynchronizedFiles,
		Error:                s.Error,
		LastSyncToVM:         s.LastSyncToVM,
		LastSyncFromVM:       s.LastSyncFromVM,
		FilesPendingUpload:   s.FilesPendingUpload,
		FilesPendingDownload: s.FilesPendingDownload,
		TotalSyncs:           s.TotalSyncs,
		TotalFilesSynced:     s.TotalFilesSynced,
		TotalSyncTimeMs:      s.TotalSyncTimeMs,
	}, nil
}
func (a *SyncEngineAdapter) GetSyncConfig(ctx context.Context, vmName string) (core.SyncConfig, error) {
	// No direct method in sync.Engine; return a minimal config for now
	return core.SyncConfig{VMName: vmName}, nil
}
func (a *SyncEngineAdapter) UpdateSyncConfig(ctx context.Context, vmName string, config core.SyncConfig) error {
	// No direct method in sync.Engine; simulate by unregistering and re-registering
	err := a.Real.UnregisterVM(vmName)
	if err != nil {
		return err
	}
	mapped := syncmod.SyncConfig{
		VMName:          config.VMName,
		ProjectPath:     config.ProjectPath,
		Method:          syncmod.SyncMethod(config.Method),
		Direction:       syncmod.SyncDirection(config.Direction),
		ExcludePatterns: config.ExcludePatterns,
		WatchEnabled:    config.WatchEnabled,
		WatchInterval:   config.WatchInterval,
	}
	return a.Real.RegisterVM(vmName, mapped)
}
func (a *SyncEngineAdapter) SemanticSearch(ctx context.Context, vmName string, query string, maxResults int) ([]core.SearchResult, error) {
	r, err := a.Real.SemanticSearch(vmName, query, maxResults)
	if err != nil {
		return nil, err
	}
	results := make([]core.SearchResult, len(r))
	for i, v := range r {
		results[i] = core.SearchResult{
			Path:      v.Path,
			Line:      v.Line,
			Content:   v.Content,
			MatchType: v.MatchType,
		}
	}
	return results, nil
}
func (a *SyncEngineAdapter) ExactSearch(ctx context.Context, vmName string, query string, caseSensitive bool, maxResults int) ([]core.SearchResult, error) {
	r, err := a.Real.ExactSearch(vmName, query, caseSensitive, maxResults)
	if err != nil {
		return nil, err
	}
	results := make([]core.SearchResult, len(r))
	for i, v := range r {
		results[i] = core.SearchResult{
			Path:      v.Path,
			Line:      v.Line,
			Content:   v.Content,
			MatchType: v.MatchType,
		}
	}
	return results, nil
}
func (a *SyncEngineAdapter) FuzzySearch(ctx context.Context, vmName string, query string, maxResults int) ([]core.SearchResult, error) {
	r, err := a.Real.FuzzySearch(vmName, query, maxResults)
	if err != nil {
		return nil, err
	}
	results := make([]core.SearchResult, len(r))
	for i, v := range r {
		results[i] = core.SearchResult{
			Path:      v.Path,
			Line:      v.Line,
			Content:   v.Content,
			MatchType: v.MatchType,
		}
	}
	return results, nil
}
func (a *SyncEngineAdapter) Start(ctx context.Context) error { return nil }
func (a *SyncEngineAdapter) Stop(ctx context.Context) error  { return nil }
func (a *SyncEngineAdapter) IsRunning() bool                 { return true }
func (a *SyncEngineAdapter) ResolveSyncConflict(ctx context.Context, vmName string, path string, resolution string) error {
	return a.Real.ResolveSyncConflict(vmName, path, resolution)
}

func (a *VMManagerAdapter) SyncToVM(name, source, target string) error {
	return a.Real.SyncToVM(name, source, target)
}

func (a *VMManagerAdapter) SyncFromVM(name, source, target string) error {
	return a.Real.SyncFromVM(name, source, target)
}
