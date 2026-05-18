// Copyright Ricardo Oliveira 2025.
// SPDX-License-Identifier: MPL-2.0

package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/vagrant-mcp/server/internal/cmdexec"
	"github.com/vagrant-mcp/server/internal/core"
	"github.com/vagrant-mcp/server/internal/errors"
	"github.com/vagrant-mcp/server/internal/utils"
)

// Manager represents the configuration for a Vagrant VM Manager

// Manager handles VM lifecycle operations
type Manager struct {
	baseDir   string
	provider  string
}

// NewManager creates a new VM manager
func NewManager() (*Manager, error) {
	// Check if Vagrant CLI is installed
	if err := utils.CheckVagrantInstalled(); err != nil {
		return nil, fmt.Errorf("failed to initialize VM manager: %w", err)
	}

	// Get base directory from environment or use default
	baseDir := os.Getenv("VM_BASE_DIR")
	if baseDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		baseDir = filepath.Join(homeDir, ".vagrant-mcp", "vms")
	}

	// Ensure the base directory exists
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create VM base directory: %w", err)
	}

	// Determine provider from environment or default to libvirt
	provider := os.Getenv("VAGRANT_DEFAULT_PROVIDER")
	if provider == "" {
		provider = "libvirt"
	}

	return &Manager{
		baseDir:  baseDir,
		provider: provider,
	}, nil
}

// CreateVM creates a new Vagrant VM with the given configuration.
// If a Vagrantfile already exists in the project directory, it will be used instead of generating a new one.
func (m *Manager) CreateVM(ctx context.Context, name string, projectPath string, config core.VMConfig) error {
	// Check for existing Vagrantfile in the project directory
	existingVagrantfile := filepath.Join(projectPath, "Vagrantfile")
	if _, err := os.Stat(existingVagrantfile); err == nil {
		return m.registerFromExistingVagrantfile(name, projectPath, existingVagrantfile, config)
	}

	vmDir := m.getVMDir(name)
	if err := os.MkdirAll(vmDir, 0755); err != nil {
		return errors.OperationFailed("create VM directory", err)
	}
	config.Name = name
	config.ProjectPath = projectPath
	if config.Provider == "" {
		config.Provider = m.provider
	}
	if err := m.saveVMConfig(name, config); err != nil {
		return errors.OperationFailed("save VM configuration", err)
	}
	if err := m.generateVagrantfile(name, config); err != nil {
		return errors.OperationFailed("generate Vagrantfile", err)
	}
	log.Info().Str("name", name).Msg("VM created successfully")
	return nil
}

// RegisterExistingVM registers a VM from an existing Vagrantfile in the project directory.
func (m *Manager) RegisterExistingVM(ctx context.Context, name string, projectPath string, vagrantVMName string) error {
	existingVagrantfile := filepath.Join(projectPath, "Vagrantfile")
	if _, err := os.Stat(existingVagrantfile); os.IsNotExist(err) {
		return errors.NotFound("Vagrantfile", existingVagrantfile)
	}

	config := core.VMConfig{
		Name:            name,
		ProjectPath:     projectPath,
		VagrantfilePath: existingVagrantfile,
		Provider:        m.provider,
		VagrantVMName:   vagrantVMName,
	}
	return m.registerFromExistingVagrantfile(name, projectPath, existingVagrantfile, config)
}

// registerFromExistingVagrantfile sets up a VM that uses an existing Vagrantfile from the project directory.
func (m *Manager) registerFromExistingVagrantfile(name string, projectPath string, vagrantfilePath string, config core.VMConfig) error {
	config.Name = name
	config.ProjectPath = projectPath
	config.VagrantfilePath = vagrantfilePath
	if config.Provider == "" {
		config.Provider = m.provider
	}

	if err := m.saveVMConfig(name, config); err != nil {
		return errors.OperationFailed("save VM configuration", err)
	}

	log.Info().Str("name", name).Str("vagrantfile", vagrantfilePath).
		Msg("VM registered with existing Vagrantfile")
	return nil
}

// StartVM starts the specified VM
func (m *Manager) StartVM(ctx context.Context, name string) error {
	cmd := m.vagrantCmd(ctx, name, "up")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, errors.CodeOperationFailed, fmt.Sprintf("failed to start VM: %s", output))
	}
	log.Info().Str("name", name).Msg("VM started successfully")
	return nil
}

// StopVM stops the specified VM
func (m *Manager) StopVM(ctx context.Context, name string) error {
	cmd := m.vagrantCmd(ctx, name, "halt")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, errors.CodeOperationFailed, fmt.Sprintf("failed to stop VM: %s", output))
	}
	log.Info().Str("name", name).Msg("VM stopped successfully")
	return nil
}

// DestroyVM destroys the specified VM and cleans up resources.
// For VMs with existing Vagrantfiles, only the MCP config is removed (not the project Vagrantfile).
func (m *Manager) DestroyVM(ctx context.Context, name string) error {
	vmDir := m.getVMDir(name)

	// Check if this is an existing-Vagrantfile VM
	config, configErr := m.loadVMConfig(name)
	isExistingVagrantfile := configErr == nil && config.VagrantfilePath != ""

	cmd := m.vagrantCmd(ctx, name, "destroy", "-f")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error().Str("name", name).Err(err).Str("output", string(output)).Msg("Failed to destroy VM")
		// Continue with cleanup even if destroy fails
	}

	// Only remove the VM directory for managed VMs (not existing Vagrantfile VMs)
	if !isExistingVagrantfile {
		if err := os.RemoveAll(vmDir); err != nil {
			return errors.OperationFailed("clean up VM directory", err)
		}
	}

	configFile := filepath.Join(filepath.Dir(m.baseDir), fmt.Sprintf("%s.json", name))
	if err := os.Remove(configFile); err != nil && !os.IsNotExist(err) {
		return errors.OperationFailed("clean up VM config", err)
	}
	log.Info().Str("name", name).Msg("VM destroyed successfully")
	return nil
}

// GetVMState returns the current state of the VM as core.VMState
func (m *Manager) GetVMState(ctx context.Context, name string) (core.VMState, error) {
	vmDir := m.getVMDir(name)
	if _, err := os.Stat(vmDir); os.IsNotExist(err) {
		return core.NotCreated, nil
	}
	cmd := m.vagrantCmd(ctx, name, "status", "--machine-readable")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return core.Unknown, errors.OperationFailed("get VM status", err)
	}
	state, err := m.parseVagrantStatus(string(output))
	if err != nil {
		return core.Unknown, errors.OperationFailed("parse vagrant status", err)
	}
	return state, nil
}

// GetVMConfig returns the VM configuration as core.VMConfig
func (m *Manager) GetVMConfig(ctx context.Context, name string) (core.VMConfig, error) {
	configFile := filepath.Join(filepath.Dir(m.baseDir), fmt.Sprintf("%s.json", name))
	data, err := os.ReadFile(configFile)
	if err != nil {
		return core.VMConfig{}, errors.OperationFailed("read VM config", err)
	}
	var config core.VMConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return core.VMConfig{}, errors.OperationFailed("parse VM config", err)
	}
	return config, nil
}

// UpdateVMConfig updates the VM configuration using core.VMConfig
func (m *Manager) UpdateVMConfig(ctx context.Context, name string, config core.VMConfig) error {
	log.Debug().Str("vm", name).Msg("Updating VM configuration")
	vmDir := filepath.Join(m.baseDir, name)
	if _, err := os.Stat(vmDir); os.IsNotExist(err) {
		return errors.NotFound("VM directory", vmDir)
	}
	configPath := filepath.Join(vmDir, "config.json")
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return errors.OperationFailed("marshal VM config", err)
	}
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return errors.OperationFailed("write VM config", err)
	}
	log.Info().Str("vm", name).Msg("VM configuration updated")
	return nil
}

// Close cleans up resources used by the VM manager
func (m *Manager) Close() {
	// Nothing to clean up currently
}

// getVMDir returns the directory path for a VM.
// For VMs with existing Vagrantfiles, this returns the project directory.
// For generated VMs, this returns the managed VM directory.
func (m *Manager) getVMDir(name string) string {
	config, err := m.loadVMConfig(name)
	if err == nil && config.VagrantfilePath != "" {
		return filepath.Dir(config.VagrantfilePath)
	}
	return filepath.Join(m.baseDir, name)
}

// loadVMConfig reads the VM configuration from disk.
func (m *Manager) loadVMConfig(name string) (core.VMConfig, error) {
	configFile := filepath.Join(filepath.Dir(m.baseDir), fmt.Sprintf("%s.json", name))
	data, err := os.ReadFile(configFile)
	if err != nil {
		return core.VMConfig{}, err
	}
	var config core.VMConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return core.VMConfig{}, err
	}
	return config, nil
}

// vagrantCmd returns a vagrant command scoped to the specific VM in multi-VM Vagrantfiles.
// If VagrantVMName isn't explicitly set, auto-detects the first VM from vagrant status.
// The VM name is inserted right after the subcommand (e.g. "vagrant up bridge", "vagrant ssh-config bridge").
func (m *Manager) vagrantCmd(ctx context.Context, mcpName string, args ...string) *exec.Cmd {
	vmName := m.resolveVagrantVMName(ctx, mcpName)
	if vmName != "" {
		// Insert VM name after the subcommand (args[0]) and before any flags
		insertAt := 1
		if insertAt > len(args) {
			insertAt = len(args)
		}
		args = append(args[:insertAt], append([]string{vmName}, args[insertAt:]...)...)
		log.Debug().Str("vm", vmName).Strs("args", args).Msg("Scoped vagrant command to specific VM")
	}
	cmd := exec.CommandContext(ctx, "vagrant", args...)
	cmd.Dir = m.getVMDir(mcpName)
	return cmd
}

// resolveVagrantVMName determines which VM name to use for vagrant commands.
// Returns the explicit VagrantVMName from config, or auto-detects from vagrant status.
func (m *Manager) resolveVagrantVMName(ctx context.Context, mcpName string) string {
	config, err := m.loadVMConfig(mcpName)
	if err != nil {
		return ""
	}
	if config.VagrantVMName != "" {
		return config.VagrantVMName
	}
	// Only auto-detect for existing Vagrantfiles (multi-VM detection)
	if config.VagrantfilePath == "" {
		return ""
	}

	// Auto-detect VM names from vagrant status
	vmDir := filepath.Dir(config.VagrantfilePath)
	cmd := exec.CommandContext(ctx, "vagrant", "status", "--machine-readable")
	cmd.Dir = vmDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	// Parse machine-readable output: lines are "timestamp,vm-name,category,data"
	// Collect all VM names, and check if the MCP name matches one of them
	var vmNames []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		parts := strings.Split(line, ",")
		if len(parts) >= 3 && parts[2] == "state" {
			vmName := parts[1]
			if vmName != "" {
				vmNames = append(vmNames, vmName)
			}
		}
	}

	if len(vmNames) == 0 {
		return ""
	}

	// If there's only one VM, use it
	if len(vmNames) == 1 {
		config.VagrantVMName = vmNames[0]
		m.saveVMConfig(mcpName, config)
		log.Info().Str("vm", vmNames[0]).Msg("Auto-detected single VM from Vagrantfile")
		return vmNames[0]
	}

	// Multi-VM: check if the MCP name matches one of the VM names
	for _, vmName := range vmNames {
		if vmName == mcpName {
			config.VagrantVMName = vmName
			m.saveVMConfig(mcpName, config)
			log.Info().Str("vm", vmName).Msg("Matched MCP name to VM name in Vagrantfile")
			return vmName
		}
	}

	// Also check if MCP name ends with the VM name (handles prefixes like "test-", "vpn-")
	for _, vmName := range vmNames {
		if strings.HasSuffix(mcpName, "-"+vmName) || strings.HasSuffix(mcpName, "_"+vmName) {
			config.VagrantVMName = vmName
			m.saveVMConfig(mcpName, config)
			log.Info().Str("vm", vmName).Str("mcp_name", mcpName).
				Msg("Matched MCP name suffix to VM name in Vagrantfile")
			return vmName
		}
	}

	// No match: log a warning and use the first VM
	log.Warn().Str("mcp_name", mcpName).Strs("available_vms", vmNames).
		Msg("MCP name doesn't match any VM in Vagrantfile. Use vagrant_vm_name parameter. Falling back to first VM.")
	config.VagrantVMName = vmNames[0]
	m.saveVMConfig(mcpName, config)
	return vmNames[0]
}

// saveVMConfig saves the VM configuration to a file
func (m *Manager) saveVMConfig(name string, config core.VMConfig) error {
	configDir := filepath.Dir(m.baseDir)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return errors.OperationFailed("create config directory", err)
	}

	configFile := filepath.Join(configDir, fmt.Sprintf("%s.json", name))
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return errors.OperationFailed("marshal VM config", err)
	}

	return os.WriteFile(configFile, data, 0644)
}

// generateVagrantfile creates a Vagrantfile for the VM and validates it
func (m *Manager) generateVagrantfile(name string, config core.VMConfig) error {
	provider := config.Provider
	if provider == "" {
		provider = m.provider
	}

	providerConfig := m.generateProviderConfig(name, provider, config)

	vagrantfile := `# -*- mode: ruby -*-
# vi: set ft=ruby :
# Generated by Vagrant MCP Server

Vagrant.configure("2") do |config|
  # Box settings
  config.vm.box = "%s"

  # Provider-specific configuration
%s

  # Network settings
%s

  # Sync settings
%s

  # Provisioning
  config.vm.provision "shell", inline: <<-SHELL
    # Update package list
    apt-get update

    # Install basic development tools
    apt-get install -y build-essential curl git unzip
%s
    echo "Development VM setup completed!"
  SHELL
end`

	// Generate port forwarding configuration
	portsConfig := ""
	for _, port := range config.Ports {
		portsConfig += fmt.Sprintf("  config.vm.network \"forwarded_port\", guest: %d, host: %d, host_ip: \"127.0.0.1\"\n",
			port.Guest, port.Host)
	}

	// Generate sync configuration
	syncConfig := ""
	switch config.SyncType {
	case "rsync":
		syncConfig = fmt.Sprintf(`  config.vm.synced_folder "%s", "/vagrant",
    type: "rsync",
    rsync__exclude: [".git/", "node_modules/", "dist/", ".vagrant/"],
    rsync__args: ["--verbose", "--archive", "--delete", "-z"]`, config.ProjectPath)
	case "nfs":
		syncConfig = fmt.Sprintf(`  config.vm.synced_folder "%s", "/vagrant",
    type: "nfs",
    nfs_udp: false,
    nfs_version: 4`, config.ProjectPath)
	case "smb":
		syncConfig = fmt.Sprintf(`  config.vm.synced_folder "%s", "/vagrant",
    type: "smb"`, config.ProjectPath)
	default:
		syncConfig = fmt.Sprintf(`  config.vm.synced_folder "%s", "/vagrant"`, config.ProjectPath)
	}

	// Generate environment setup
	envSetup := ""
	for _, line := range config.Environment {
		envSetup += "    " + line + "\n"
	}

	// Format the complete Vagrantfile
	content := fmt.Sprintf(vagrantfile,
		config.Box,     // Box name
		providerConfig, // Provider config block
		portsConfig,    // Port forwarding
		syncConfig,     // Sync configuration
		envSetup)       // Environment setup

	// Write the Vagrantfile
	vmDir := m.getVMDir(name)
	vagrantfilePath := filepath.Join(vmDir, "Vagrantfile")
	if err := os.WriteFile(vagrantfilePath, []byte(content), 0644); err != nil {
		return errors.OperationFailed("write Vagrantfile", err)
	}

	// Skip Vagrantfile validation in CI environments or when no provider is available
	if m.shouldSkipProviderValidation() {
		log.Info().Str("name", name).Msg("Skipping Vagrantfile validation (CI environment or no provider)")
		return nil
	}

	// Skip validation for sync types that require unavailable infrastructure
	if config.SyncType == "nfs" || config.SyncType == "smb" {
		log.Info().Str("name", name).Str("sync_type", config.SyncType).
			Msg("Skipping validation for sync type that may require unavailable infrastructure")
		return nil
	}

	// Validate the Vagrantfile to ensure it's correct
	cmd := exec.Command("vagrant", "validate")
	cmd.Dir = vmDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, errors.CodeOperationFailed, fmt.Sprintf("vagrantfile validation failed: %s", output))
	}
	log.Info().Str("name", name).Msg("Vagrantfile validated successfully")

	return nil
}

// generateProviderConfig generates the provider-specific Ruby block for the Vagrantfile
func (m *Manager) generateProviderConfig(name, provider string, config core.VMConfig) string {
	switch provider {
	case "libvirt":
		return fmt.Sprintf(`  config.vm.provider "libvirt" do |lv|
    lv.memory = %d
    lv.cpus = %d
    lv.random_hostname = true
  end`, config.Memory, config.CPU)
	case "virtualbox":
		return fmt.Sprintf(`  config.vm.provider "virtualbox" do |vb|
    vb.gui = false
    vb.name = "%s"
    vb.memory = %d
    vb.cpus = %d

    # Performance optimizations
    vb.customize ["modifyvm", :id, "--natdnshostresolver1", "on"]
    vb.customize ["modifyvm", :id, "--natdnsproxy1", "on"]
    vb.customize ["modifyvm", :id, "--ioapic", "on"]
  end`, name, config.Memory, config.CPU)
	case "vmware_desktop", "vmware_fusion":
		return fmt.Sprintf(`  config.vm.provider "vmware_desktop" do |v|
    v.memory = %d
    v.cpus = %d
  end`, config.Memory, config.CPU)
	case "hyperv":
		return fmt.Sprintf(`  config.vm.provider "hyperv" do |h|
    h.memory = %d
    h.cpus = %d
  end`, config.Memory, config.CPU)
	default:
		return fmt.Sprintf(`  config.vm.provider "%s" do |p|
    p.memory = %d
    p.cpus = %d
  end`, provider, config.Memory, config.CPU)
	}
}

// shouldSkipProviderValidation determines if provider-dependent operations should be skipped
func (m *Manager) shouldSkipProviderValidation() bool {
	// Skip if running in CI environment
	if os.Getenv("CI") == "true" {
		return true
	}

	// Skip if GitHub Actions environment
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		return true
	}

	// Skip if explicitly requested
	if os.Getenv("SKIP_VAGRANT_VALIDATION") == "true" {
		return true
	}

	// Skip if the configured provider plugin is not installed
	if !m.isProviderAvailable(m.provider) {
		log.Warn().Str("provider", m.provider).
			Msg("Provider not available, skipping validation")
		return true
	}

	return false
}

// isProviderAvailable checks if a Vagrant provider plugin is installed
func (m *Manager) isProviderAvailable(provider string) bool {
	cmd := exec.Command("vagrant", "plugin", "list")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	// Check for built-in providers (virtualbox, libvirt via vagrant-libvirt)
	// and plugin-based providers
	outputStr := string(output)
	switch provider {
	case "virtualbox":
		// VirtualBox is a built-in provider, check if VBoxManage is available
		if _, err := exec.LookPath("VBoxManage"); err == nil {
			return true
		}
	case "libvirt":
		// Check for vagrant-libvirt plugin
		if contains(outputStr, "vagrant-libvirt") {
			return true
		}
	default:
		// For other providers, check plugin list
		if contains(outputStr, provider) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// parseVagrantStatus parses the output of 'vagrant status --machine-readable'
func (m *Manager) parseVagrantStatus(output string) (core.VMState, error) {
	return GlobalStateMapper.ParseVagrantState(output)
}

// ParseVagrantStatus parses the output of 'vagrant status --machine-readable'
func (m *Manager) ParseVagrantStatus(output string) (core.VMState, error) {
	return m.parseVagrantStatus(output)
}

// parseSSHConfig parses the output of 'vagrant ssh-config'
func (m *Manager) parseSSHConfig(output string) (map[string]string, error) {
	return utils.GlobalOutputParser.ParseSSHConfig(output)
}

// ExecuteCommand executes a command in a VM
func (m *Manager) ExecuteCommand(ctx context.Context, name string, cmd string, args []string, workingDir string) (string, string, int, error) {
	vmDir := m.getVMDir(name)
	options := cmdexec.CmdOptions{
		Directory:  vmDir,
		OutputMode: cmdexec.OutputModeCapture,
	}
	// If a workingDir is provided, use it as a subdirectory inside the VM directory
	if workingDir != "" {
		options.Directory = filepath.Join(vmDir, workingDir)
	}
	result, err := cmdexec.Execute(ctx, cmd, args, options)
	if err != nil {
		return string(result.StdOut), string(result.StdErr), result.ExitCode, errors.OperationFailed("execute command in VM", err)
	}
	return string(result.StdOut), string(result.StdErr), result.ExitCode, nil
}

// UploadToVM uploads a file or directory to the VM using vagrant upload
func (m *Manager) UploadToVM(ctx context.Context, name string, source string, destination string, compress bool, compressionType string) error {
	vmDir := m.getVMDir(name)
	if _, err := os.Stat(vmDir); os.IsNotExist(err) {
		return errors.NotFound("VM", name)
	}
	state, err := m.GetVMState(ctx, name)
	if err != nil {
		return errors.OperationFailed("get VM state", err)
	}
	if state != core.Running {
		return errors.Wrap(fmt.Errorf("VM is not running (current state: %s)", state), errors.CodeInvalidState, "VM is not running")
	}
	if _, err := os.Stat(source); os.IsNotExist(err) {
		return errors.NotFound("source path", source)
	}
	args := []string{"upload"}
	if compress {
		args = append(args, "--compress")
		if compressionType != "" {
			args = append(args, "--compression-type", compressionType)
		}
	}
	args = append(args, source, destination)
	cmd := exec.CommandContext(ctx, "vagrant", args...)
	cmd.Dir = vmDir
	log.Debug().Str("vm", name).Str("source", source).Str("destination", destination).
		Bool("compress", compress).Str("compression", compressionType).
		Msg("Uploading file to VM")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.OperationFailed("upload file to VM", fmt.Errorf("%w: %s", err, output))
	}
	log.Info().Str("vm", name).Str("source", source).Str("destination", destination).
		Msg("File uploaded to VM successfully")
	return nil
}

// SyncToVM synchronizes files from host to VM using rsync
func (m *Manager) SyncToVM(name, source, target string) error {
	// Use rsync to copy files from host to VM
	// This is a simplified implementation; in production, handle SSH config, errors, etc.
	vmDir := m.getVMDir(name)
	if vmDir == "" {
		return fmt.Errorf("could not determine VM directory for %s", name)
	}
	// Assume target is relative to /vagrant in the VM
	cmd := exec.Command("rsync", "-az", "--delete", source+"/", vmDir+"/vagrant/"+target+"/")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rsync to VM failed: %v, output: %s", err, string(output))
	}
	return nil
}

// SyncFromVM synchronizes files from VM to host using rsync
func (m *Manager) SyncFromVM(name, source, target string) error {
	// Use rsync to copy files from VM to host
	vmDir := m.getVMDir(name)
	if vmDir == "" {
		return fmt.Errorf("could not determine VM directory for %s", name)
	}
	cmd := exec.Command("rsync", "-az", "--delete", vmDir+"/vagrant/"+source+"/", target+"/")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rsync from VM failed: %v, output: %s", err, string(output))
	}
	return nil
}

// GetSSHConfig retrieves the SSH configuration for the VM using 'vagrant ssh-config'
func (m *Manager) GetSSHConfig(ctx context.Context, name string) (map[string]string, error) {
	cmd := m.vagrantCmd(ctx, name, "ssh-config")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get SSH config: %w, output: %s", err, string(output))
	}
	return m.parseSSHConfig(string(output))
}
