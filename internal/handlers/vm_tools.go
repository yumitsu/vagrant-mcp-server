// Copyright Ricardo Oliveira 2025.
// SPDX-License-Identifier: MPL-2.0

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/rs/zerolog/log"
	"github.com/vagrant-mcp/server/internal/core"
	mcp_pkg "github.com/vagrant-mcp/server/pkg/mcp"
)

// RegisterVMTools registers all VM-related tools with the MCP server
func RegisterVMTools(srv *server.MCPServer, vmManager core.VMManager, syncEngine core.SyncEngine) {
	// Create dev VM tool
	type CreateVMArgs struct {
		Name            string                   `json:"name"`
		ProjectPath     string                   `json:"project_path"`
		CPU             float64                  `json:"cpu"`
		Memory          float64                  `json:"memory"`
		Box             string                   `json:"box"`
		SyncType        string                   `json:"sync_type"`
		Provider        string                   `json:"provider"`
		Ports           []map[string]interface{} `json:"ports"`
		ExcludePatterns []string                 `json:"exclude_patterns"`
	}
	createVMTool := mcp.NewTool("create_dev_vm",
		mcp.WithDescription("Create and configure a development VM with Vagrant. If a Vagrantfile already exists in the project directory, it will be used automatically."),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Name for the development VM")),
		mcp.WithString("project_path",
			mcp.Required(),
			mcp.Description("Path to the project directory to sync")),
		mcp.WithNumber("cpu",
			mcp.Description("Number of CPU cores"),
			mcp.DefaultNumber(2)),
		mcp.WithNumber("memory",
			mcp.Description("Amount of memory in MB"),
			mcp.DefaultNumber(2048)),
		mcp.WithString("box",
			mcp.Description("Vagrant box to use"),
			mcp.DefaultString("ubuntu/focal64")),
		mcp.WithString("sync_type",
			mcp.Description("Sync type to use"),
			mcp.DefaultString("rsync")),
		mcp.WithString("provider",
			mcp.Description("Vagrant provider to use (libvirt, virtualbox, vmware_desktop, hyperv)"),
			mcp.DefaultString("libvirt")),
		mcp.WithArray("ports",
			mcp.Description("Ports to forward (format: [host:guest])"),
			mcp.Items(map[string]any{"type": "object"})),
		mcp.WithArray("exclude_patterns",
			mcp.Description("Patterns to exclude from sync"),
			mcp.Items(map[string]any{"type": "string"})),
	)

	mcp_pkg.RegisterTypedTool(srv, createVMTool, func(ctx context.Context, request mcp.CallToolRequest, args CreateVMArgs) (*mcp.CallToolResult, error) {
		if args.Name == "" || args.ProjectPath == "" {
			return mcp.NewToolResultError("Missing required parameter: name or project_path"), nil
		}
		// Convert ports
		var ports []core.Port
		for _, portMap := range args.Ports {
			var port core.Port
			if guest, ok := portMap["guest"].(float64); ok {
				port.Guest = int(guest)
			}
			if host, ok := portMap["host"].(float64); ok {
				port.Host = int(host)
			}
			ports = append(ports, port)
		}
		if len(ports) == 0 {
			// Default ports
			ports = []core.Port{
				{Guest: 3000, Host: 3000},
				{Guest: 8000, Host: 8000},
				{Guest: 5432, Host: 5432},
				{Guest: 3306, Host: 3306},
				{Guest: 6379, Host: 6379},
			}
		}
		// Exclude patterns
		excludePatterns := args.ExcludePatterns
		if len(excludePatterns) == 0 {
			excludePatterns = []string{"node_modules", ".git", "*.log", "dist", "build", "__pycache__", "*.pyc", "venv", ".venv", "*.o", "*.out"}
		}
		config := core.VMConfig{
			Box:                 args.Box,
			CPU:                 int(args.CPU),
			Memory:              int(args.Memory),
			SyncType:            args.SyncType,
			Provider:            args.Provider,
			Ports:               ports,
			SyncExcludePatterns: excludePatterns,
		}
		if err := vmManager.CreateVM(ctx, args.Name, args.ProjectPath, config); err != nil {
			return mcp.NewToolResultErrorf("Failed to create VM: %v", err), nil
		}
		response := map[string]interface{}{
			"name":         args.Name,
			"project_path": args.ProjectPath,
			"config":       config,
			"status":       "created",
			"timestamp":    time.Now().Format(time.RFC3339),
		}
		jsonResponse, err := json.Marshal(response)
		if err != nil {
			return mcp.NewToolResultError("Failed to marshal response"), nil
		}
		return mcp.NewToolResultText(string(jsonResponse)), nil
	})

	// Ensure dev VM tool
	type EnsureVMArgs struct {
		Name        string `json:"name"`
		ProjectPath string `json:"project_path"`
	}
	ensureVMTool := mcp.NewTool("ensure_dev_vm",
		mcp.WithDescription("Ensure development VM is running, create if it doesn't exist"),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Name of the development VM")),
		mcp.WithString("project_path",
			mcp.Description("Path to the project directory to sync (only needed for creation)")),
	)

	mcp_pkg.RegisterTypedTool(srv, ensureVMTool, func(ctx context.Context, request mcp.CallToolRequest, args EnsureVMArgs) (*mcp.CallToolResult, error) {
		if args.Name == "" {
			return mcp.NewToolResultError("Missing required parameter: name"), nil
		}
		// Get VM state
		state, err := vmManager.GetVMState(ctx, args.Name)
		if err != nil {
			// VM doesn't exist, see if we can create it
			if args.ProjectPath == "" {
				return mcp.NewToolResultError("VM doesn't exist. Missing required parameter for creation: project_path"), nil
			}
			config := core.VMConfig{
				Box:    "ubuntu/focal64",
				CPU:    2,
				Memory: 2048,
				Ports: []core.Port{
					{Guest: 3000, Host: 3000},
					{Guest: 8000, Host: 8000},
					{Guest: 5432, Host: 5432},
				},
				SyncType: "rsync",
				SyncExcludePatterns: []string{
					"node_modules", ".git", "*.log", "dist", "build",
				},
			}
			if err := vmManager.CreateVM(ctx, args.Name, args.ProjectPath, config); err != nil {
				return mcp.NewToolResultErrorf("Failed to create VM: %v", err), nil
			}
			syncConfig := core.SyncConfig{
				VMName:          args.Name,
				ProjectPath:     args.ProjectPath,
				Method:          core.SyncMethod(config.SyncType),
				Direction:       core.SyncToVM,
				ExcludePatterns: config.SyncExcludePatterns,
			}
			if err := syncEngine.RegisterVM(ctx, args.Name, syncConfig); err != nil {
				log.Error().Err(err).Msg("Failed to register VM with sync engine")
			}
			return mcp.NewToolResultText(fmt.Sprintf("VM '%s' created and started", args.Name)), nil
		}
		if state != core.Running {
			if err := vmManager.StartVM(ctx, args.Name); err != nil {
				return mcp.NewToolResultErrorf("Failed to start VM: %v", err), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("VM '%s' started", args.Name)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("VM '%s' is already running", args.Name)), nil
	})

	// Destroy dev VM tool
	type DestroyVMArgs struct {
		Name string `json:"name"`
	}
	destroyVMTool := mcp.NewTool("destroy_dev_vm",
		mcp.WithDescription("Clean up development VM and associated resources"),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Name of the development VM")),
	)
	mcp_pkg.RegisterTypedTool(srv, destroyVMTool, func(ctx context.Context, request mcp.CallToolRequest, args DestroyVMArgs) (*mcp.CallToolResult, error) {
		if args.Name == "" {
			return mcp.NewToolResultError("Missing required parameter: name"), nil
		}
		if err := vmManager.DestroyVM(ctx, args.Name); err != nil {
			return mcp.NewToolResultErrorf("Failed to destroy VM: %v", err), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("VM '%s' destroyed", args.Name)), nil
	})

	// Get VM status tool
	type GetVMStatusArgs struct {
		Name string `json:"name"`
	}
	getStatusTool := mcp.NewTool("get_vm_status",
		mcp.WithDescription("Get status of one or all development VMs"),
		mcp.WithString("name",
			mcp.Description("Name of the development VM (optional)")),
	)
	mcp_pkg.RegisterTypedTool(srv, getStatusTool, func(ctx context.Context, request mcp.CallToolRequest, args GetVMStatusArgs) (*mcp.CallToolResult, error) {
		if args.Name != "" {
			state, err := vmManager.GetVMState(ctx, args.Name)
			if err != nil {
				return mcp.NewToolResultErrorf("Failed to get VM status: %v", err), nil
			}
			response := map[string]interface{}{
				"name":  args.Name,
				"state": state,
			}
			jsonResponse, err := json.Marshal(response)
			if err != nil {
				return mcp.NewToolResultError("Failed to marshal response"), nil
			}
			return mcp.NewToolResultText(string(jsonResponse)), nil
		}
		vmNames, err := vmManager.ListVMs(ctx)
		if err != nil {
			return mcp.NewToolResultErrorf("Failed to list VMs: %v", err), nil
		}
		vmStates := make([]map[string]interface{}, 0, len(vmNames))
		for _, vmName := range vmNames {
			state, err := vmManager.GetVMState(ctx, vmName)
			var stateStr string
			if err != nil {
				stateStr = "unknown"
			} else {
				stateStr = string(state)
			}
			vmStates = append(vmStates, map[string]interface{}{
				"name":  vmName,
				"state": stateStr,
			})
		}
		response := map[string]interface{}{
			"vms": vmStates,
		}
		jsonResponse, err := json.Marshal(response)
		if err != nil {
			return mcp.NewToolResultError("Failed to marshal response"), nil
		}
		return mcp.NewToolResultText(string(jsonResponse)), nil
	})

	// Use project VM tool - register a VM from an existing Vagrantfile in the project directory
	type UseProjectVMArgs struct {
		Name          string `json:"name"`
		ProjectPath   string `json:"project_path"`
		VagrantVMName string `json:"vagrant_vm_name"`
	}
	useProjectVMTool := mcp.NewTool("use_project_vm",
		mcp.WithDescription("Register and manage a VM from an existing Vagrantfile in the project directory. "+
			"Use this when the project already has a Vagrantfile for provisioning VMs. "+
			"The MCP server will use the existing Vagrantfile instead of generating a new one. "+
			"For multi-VM Vagrantfiles, specify vagrant_vm_name to target a specific VM."),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Name to identify this VM in MCP operations")),
		mcp.WithString("project_path",
			mcp.Required(),
			mcp.Description("Path to the project directory containing the Vagrantfile")),
		mcp.WithString("vagrant_vm_name",
			mcp.Description("Name of the VM inside the Vagrantfile (required for multi-VM Vagrantfiles). "+
				"Omit for single-VM Vagrantfiles.")),
	)
	mcp_pkg.RegisterTypedTool(srv, useProjectVMTool, func(ctx context.Context, request mcp.CallToolRequest, args UseProjectVMArgs) (*mcp.CallToolResult, error) {
		if args.Name == "" || args.ProjectPath == "" {
			return mcp.NewToolResultError("Missing required parameter: name or project_path"), nil
		}
		if err := vmManager.RegisterExistingVM(ctx, args.Name, args.ProjectPath, args.VagrantVMName); err != nil {
			return mcp.NewToolResultErrorf("Failed to register project VM: %v", err), nil
		}
		response := map[string]interface{}{
			"name":                args.Name,
			"project_path":        args.ProjectPath,
			"status":              "registered",
			"using_existing_vagrantfile": true,
				"vagrant_vm_name":                args.VagrantVMName,
			"timestamp":           time.Now().Format(time.RFC3339),
		}
		jsonResponse, err := json.Marshal(response)
		if err != nil {
			return mcp.NewToolResultError("Failed to marshal response"), nil
		}
		return mcp.NewToolResultText(string(jsonResponse)), nil
	})
}
