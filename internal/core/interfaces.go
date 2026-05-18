// Copyright Ricardo Oliveira 2025.
// SPDX-License-Identifier: MPL-2.0

// Package core provides the core interfaces for the Vagrant MCP Server
package core

import "context"

// VMManager defines the interface for VM lifecycle management operations
type VMManager interface {
	// CreateVM creates a new VM with the given configuration
	CreateVM(ctx context.Context, name string, projectPath string, config VMConfig) error

	// RegisterExistingVM registers a VM from an existing Vagrantfile in the project directory
	RegisterExistingVM(ctx context.Context, name string, projectPath string) error

	// StartVM starts an existing VM
	StartVM(ctx context.Context, name string) error

	// StopVM stops a running VM
	StopVM(ctx context.Context, name string) error

	// DestroyVM destroys a VM and cleans up resources
	DestroyVM(ctx context.Context, name string) error

	// GetVMState gets the current state of a VM
	GetVMState(ctx context.Context, name string) (VMState, error)

	// UploadToVM uploads a file or directory to the VM
	UploadToVM(ctx context.Context, name, source, destination string, compress bool, compressionType string) error

	// GetVMConfig gets the configuration of a VM
	GetVMConfig(ctx context.Context, name string) (VMConfig, error)

	// UpdateVMConfig updates the configuration of a VM
	UpdateVMConfig(ctx context.Context, name string, config VMConfig) error

	// GetBaseDir gets the base directory for VMs
	GetBaseDir() string

	// ListVMs lists all VMs
	ListVMs(ctx context.Context) ([]string, error)

	// ExecuteCommand executes a command in a VM
	ExecuteCommand(ctx context.Context, name string, cmd string, args []string, workingDir string) (string, string, int, error)
}

// SyncEngine defines the interface for file synchronization operations
type SyncEngine interface {
	// RegisterVM registers a VM with the sync engine
	RegisterVM(ctx context.Context, vmName string, config SyncConfig) error

	// UnregisterVM unregisters a VM from the sync engine
	UnregisterVM(ctx context.Context, vmName string) error

	// SyncToVM synchronizes files from host to VM
	SyncToVM(ctx context.Context, vmName string, sourcePath string) (*SyncResult, error)

	// SyncFromVM synchronizes files from VM to host
	SyncFromVM(ctx context.Context, vmName string, sourcePath string) (*SyncResult, error)

	// GetSyncStatus returns the sync status for a VM
	GetSyncStatus(ctx context.Context, vmName string) (SyncStatus, error)

	// GetSyncConfig returns the sync configuration for a VM
	GetSyncConfig(ctx context.Context, vmName string) (SyncConfig, error)

	// UpdateSyncConfig updates the sync configuration for a VM
	UpdateSyncConfig(ctx context.Context, vmName string, config SyncConfig) error

	// ResolveSyncConflict resolves a sync conflict
	ResolveSyncConflict(ctx context.Context, vmName string, path string, resolution string) error

	// SemanticSearch performs a semantic search across synchronized files
	SemanticSearch(ctx context.Context, vmName string, query string, maxResults int) ([]SearchResult, error)

	// ExactSearch performs an exact string search across synchronized files
	ExactSearch(ctx context.Context, vmName string, query string, caseSensitive bool, maxResults int) ([]SearchResult, error)

	// FuzzySearch performs a fuzzy search across synchronized files
	FuzzySearch(ctx context.Context, vmName string, query string, maxResults int) ([]SearchResult, error)

	// Start starts the sync engine
	Start(ctx context.Context) error

	// Stop stops the sync engine
	Stop(ctx context.Context) error

	// IsRunning checks if the sync engine is running
	IsRunning() bool
}

// Executor defines the interface for executing commands in VMs
type Executor interface {
	// ExecuteCommand executes a command in a VM
	ExecuteCommand(ctx context.Context, command string, execContext ExecutionContext, outputCallback OutputCallback) (*CommandResult, error)

	// ExecuteBackground executes a command in a VM as a background task
	ExecuteBackground(ctx context.Context, command string, options ExecutionContext) (string, error)
}
