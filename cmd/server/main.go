// Copyright Ricardo Oliveira 2025.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/vagrant-mcp/server/internal/exec"
	"github.com/vagrant-mcp/server/internal/handlers"
	"github.com/vagrant-mcp/server/internal/resources"
	"github.com/vagrant-mcp/server/internal/sync"
	"github.com/vagrant-mcp/server/internal/utils"
	"github.com/vagrant-mcp/server/internal/vm"
)

// Build-time variables injected via ldflags
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
	GoVersion = "unknown"
)

const (
	Author  = "Ricardo Oliveira"
	Contact = "https://github.com/gitrgoliveira/"
)

func main() {
	// Handle version flag
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.BoolVar(&showVersion, "v", false, "Show version information (shorthand)")
	flag.Parse()

	if showVersion {
		fmt.Printf("Vagrant MCP Server %s\n", Version)
		fmt.Printf("Git Commit: %s\n", GitCommit)
		fmt.Printf("Build Time: %s\n", BuildTime)
		fmt.Printf("Go Version: %s\n", GoVersion)
		fmt.Printf("Author: %s\n", Author)
		fmt.Printf("Contact: %s\n", Contact)
		fmt.Printf("Repository: https://github.com/yumitsu/vagrant-mcp-server\n")
		return
	}

	// Configure logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	// Check if we're in MCP mode (via stdio) and disable color output if so
	transportType := os.Getenv("MCP_TRANSPORT")
	if transportType == "" && os.Getenv("VSCODE_MCP") != "true" {
		// Use colored console output for interactive use
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	} else {
		// Use plain JSON output when running as an MCP server to avoid parsing issues
		log.Logger = log.Output(os.Stdout)
	}

	// Set log level from environment or default to info
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	log.Info().
		Str("version", Version).
		Str("contact", Contact).
		Msg("Starting Vagrant MCP Server")

	// Check if Vagrant CLI is installed
	if err := utils.CheckVagrantInstalled(); err != nil {
		log.Fatal().Err(err).Msg("Vagrant CLI is required to run this server")
	}
	log.Info().Msg("Vagrant CLI detected")

	// Initialize VM manager, sync engine, and executor
	vmManager, err := vm.NewManager()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create VM manager")
	}

	syncEngine, err := sync.NewEngine()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create sync engine")
	}

	adapterVM := &exec.VMManagerAdapter{Real: vmManager}
	// Set the VM manager on the sync engine before creating the adapter
	syncEngine.SetVMManager(adapterVM)
	adapterSync := &exec.SyncEngineAdapter{Real: syncEngine}

	executor, err := exec.NewExecutor(adapterVM, adapterSync)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create executor")
	}

	// Create a new MCP server with recovery middleware
	srv := server.NewMCPServer(
		"Vagrant Development VM MCP Server",
		Version,
		server.WithRecovery(),
	)

	// Register all tools using the unified registry
	handlerRegistry := handlers.NewHandlerRegistry(adapterVM, adapterSync, executor)
	handlerRegistry.RegisterAllTools(srv)

	// Register resources using the MCP-go implementation
	resources.RegisterMCPResources(srv, adapterVM, executor)

	// Determine which transport to use
	transportType = os.Getenv("MCP_TRANSPORT")
	if transportType == "" {
		transportType = "stdio" // Default to stdio if not specified
	}

	log.Info().Str("transport", transportType).Msg("Vagrant MCP Server starting")

	// Start the server with the selected transport
	switch transportType {
	case "stdio":
		// Start with stdio transport
		log.Info().Msg("Starting with STDIO transport")
		if err := server.ServeStdio(srv); err != nil {
			log.Fatal().Err(err).Msg("STDIO server error")
		}
	case "sse":
		// Start with SSE transport
		port := os.Getenv("MCP_PORT")
		if port == "" {
			port = "8080" // Default port
		}
		log.Info().Str("port", port).Msg("Starting with SSE transport")
		sseServer := server.NewSSEServer(srv)
		if err := sseServer.Start(":" + port); err != nil {
			log.Fatal().Err(err).Msg("SSE server error")
		}
	default:
		log.Fatal().Str("transport", transportType).Msg("Unsupported transport type")
	}

	log.Info().Msg("Vagrant MCP Server shutdown complete")
}
