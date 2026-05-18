# Vagrant MCP Server (Go Implementation)

A Model Context Protocol (MCP) Server for HashiCorp Vagrant that gives AI agents the ability to create, manage, and interact with development VMs -- including projects that already have their own Vagrantfiles.

> **Note:** This server must run on the host where Vagrant and your virtualization provider are installed. Docker is not supported because the server needs direct access to the Vagrant CLI, virtualization drivers, and project files.

## Features

- **Development VM Management** -- Create, ensure, and destroy development VMs
- **Existing Vagrantfile Support** -- Automatically detects and uses Vagrantfiles already present in project directories
- **Multi-Provider** -- Supports libvirt, VirtualBox, VMware, Hyper-V, and other Vagrant providers
- **Synchronized Command Execution** -- Execute commands inside VMs with guaranteed file synchronization
- **File System Synchronization** -- Configure sync methods (rsync, NFS, SMB), monitor status, resolve conflicts
- **Development Environment Setup** -- Install language runtimes, tools, and dependencies inside VMs

## System Requirements

- **Vagrant CLI** -- Installed and on your PATH
- **Virtualization Provider** -- libvirt, VirtualBox, VMware, Hyper-V, or any Vagrant-supported provider
- **Go 1.24+** -- Only needed when building from source

Verify Vagrant is installed:

```bash
vagrant --version
```

## Installation

### Download Pre-built Binary (Recommended)

Download the latest release from the [Releases page](https://github.com/yumitsu/vagrant-mcp-server/releases).

| Platform | Architecture | Binary |
|----------|--------------|--------|
| Linux | x86_64 | `vagrant-mcp-server-linux-amd64` |
| Linux | ARM64 | `vagrant-mcp-server-linux-arm64` |
| macOS | Intel | `vagrant-mcp-server-darwin-amd64` |
| macOS | Apple Silicon | `vagrant-mcp-server-darwin-arm64` |
| Windows | x86_64 | `vagrant-mcp-server-windows-amd64.exe` |

```bash
# Download
curl -L -o vagrant-mcp-server https://github.com/yumitsu/vagrant-mcp-server/releases/latest/download/vagrant-mcp-server-linux-amd64

# Make executable (Linux/macOS)
chmod +x vagrant-mcp-server

# Move to PATH
sudo mv vagrant-mcp-server /usr/local/bin/

# Verify
vagrant-mcp-server -version
```

Verify integrity:

```bash
curl -L -O https://github.com/yumitsu/vagrant-mcp-server/releases/latest/download/checksums.txt
sha256sum -c checksums.txt --ignore-missing
```

### Build from Source

```bash
git clone https://github.com/yumitsu/vagrant-mcp-server.git vagrant-mcp-server
cd vagrant-mcp-server
make build
./bin/vagrant-mcp-server -version
```

## Configuration

All configuration is via environment variables. Copy `.env.example` and adjust:

| Variable | Default | Description |
|----------|---------|-------------|
| `MCP_TRANSPORT` | `stdio` | Transport: `stdio` or `sse` |
| `MCP_PORT` | `8080` | Port for SSE transport |
| `LOG_LEVEL` | `info` | Logging: `debug`, `info`, `warn`, `error` |
| `VAGRANT_DEFAULT_PROVIDER` | `libvirt` | Vagrant provider: `libvirt`, `virtualbox`, `vmware_desktop`, `hyperv` |
| `VM_BASE_DIR` | `~/.vagrant-mcp/vms` | Where managed VM files are stored |
| `SKIP_VAGRANT_VALIDATION` | `false` | Set to `true` to skip Vagrantfile validation |

## MCP Client Setup

### Claude Code (CLI)

Add to your `~/.claude/mcp.json` or project `.mcp.json`:

```json
{
  "mcpServers": {
    "vagrant": {
      "type": "stdio",
      "command": "/usr/local/bin/vagrant-mcp-server",
      "env": {
        "VAGRANT_DEFAULT_PROVIDER": "libvirt",
        "LOG_LEVEL": "info"
      }
    }
  }
}
```

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows):

```json
{
  "mcpServers": {
    "vagrant": {
      "command": "/usr/local/bin/vagrant-mcp-server",
      "env": {
        "VAGRANT_DEFAULT_PROVIDER": "libvirt"
      }
    }
  }
}
```

### VS Code (Copilot / Continue)

Add to VS Code `settings.json` or `.vscode/mcp.json`:

```json
{
  "mcp": {
    "servers": {
      "vagrant": {
        "type": "stdio",
        "command": "/usr/local/bin/vagrant-mcp-server",
        "env": {
          "VAGRANT_DEFAULT_PROVIDER": "libvirt",
          "VSCODE_MCP": "true"
        }
      }
    }
  }
}
```

### SSE Mode (for remote or multi-client access)

```bash
# Start the server
MCP_TRANSPORT=sse MCP_PORT=8080 vagrant-mcp-server
```

Then connect MCP clients to `http://localhost:8080/sse`.

## How to Use (Agent Guide)

### Quick Setup via Agentic Prompt

To configure this MCP server in your project, give your AI agent this prompt:

```
Add the Vagrant MCP Server to my project's MCP configuration.

1. Build the server from source:
   git clone https://github.com/yumitsu/vagrant-mcp-server.git /tmp/vagrant-mcp-server
   cd /tmp/vagrant-mcp-server && make build
   cp bin/vagrant-mcp-server ~/.local/bin/vagrant-mcp-server

2. Create or update .mcp.json in the project root with:
   {
     "mcpServers": {
       "vagrant": {
         "type": "stdio",
         "command": "<HOME>/.local/bin/vagrant-mcp-server",
         "env": {
           "VAGRANT_DEFAULT_PROVIDER": "libvirt",
           "LOG_LEVEL": "info"
         }
       }
     }
   }
   Replace <HOME> with the actual home directory path.

3. If this project has a Vagrantfile, use use_project_vm to register it.
   If not, use create_dev_vm to create a VM with appropriate settings.

4. Verify by calling get_vm_status to confirm the server is responding.
```

**For Claude Desktop**, use this prompt instead:

```
Add the Vagrant MCP Server to Claude Desktop's configuration.

1. Build the server:
   git clone https://github.com/yumitsu/vagrant-mcp-server.git /tmp/vagrant-mcp-server
   cd /tmp/vagrant-mcp-server && make build
   cp bin/vagrant-mcp-server ~/.local/bin/vagrant-mcp-server

2. Update Claude Desktop config at:
   - macOS: ~/Library/Application Support/Claude/claude_desktop_config.json
   - Linux: ~/.config/Claude/claude_desktop_config.json

   Add this entry to mcpServers:
   {
     "vagrant": {
       "command": "<HOME>/.local/bin/vagrant-mcp-server",
       "env": {
         "VAGRANT_DEFAULT_PROVIDER": "libvirt"
       }
     }
   }

3. Restart Claude Desktop to load the new MCP server.
```

**For VS Code Copilot**, use this prompt:

```
Add the Vagrant MCP Server to VS Code's MCP configuration.

1. Build the server:
   git clone https://github.com/yumitsu/vagrant-mcp-server.git /tmp/vagrant-mcp-server
   cd /tmp/vagrant-mcp-server && make build
   cp bin/vagrant-mcp-server ~/.local/bin/vagrant-mcp-server

2. Create or update .vscode/mcp.json with:
   {
     "servers": {
       "vagrant": {
         "type": "stdio",
         "command": "<HOME>/.local/bin/vagrant-mcp-server",
         "env": {
           "VAGRANT_DEFAULT_PROVIDER": "libvirt",
           "VSCODE_MCP": "true"
         }
       }
     }
   }

3. Reload VS Code window to activate the MCP server.
```

Replace `libvirt` with your actual provider (`virtualbox`, `vmware_desktop`, `hyperv`) if different.

### Decision Flow: Which Tool to Use

```
Does the project directory contain a Vagrantfile?
├── YES → use use_project_vm to register it, then ensure_dev_vm to start it
└── NO  → use create_dev_vm to create a new VM, then ensure_dev_vm to start it
```

### Tool Reference

#### VM Lifecycle

**`use_project_vm`** -- Register an existing Vagrantfile from the project directory.
Use this first when the project already has a `Vagrantfile`.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `name` | Yes | Identifier for this VM in MCP operations |
| `project_path` | Yes | Absolute path to the directory containing the Vagrantfile |

The server will use the existing Vagrantfile as-is. No new Vagrantfile is generated. Vagrant commands (`up`, `halt`, `ssh`, etc.) will run in the project directory.

**`create_dev_vm`** -- Create a new VM with a generated Vagrantfile.
Use when the project does not have a Vagrantfile.

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `name` | Yes | - | VM name |
| `project_path` | Yes | - | Project directory to sync |
| `cpu` | No | 2 | CPU cores |
| `memory` | No | 2048 | RAM in MB |
| `box` | No | `ubuntu/focal64` | Vagrant box |
| `sync_type` | No | `rsync` | Sync method: `rsync`, `nfs`, `smb` |
| `provider` | No | `libvirt` | Provider: `libvirt`, `virtualbox`, `vmware_desktop`, `hyperv` |
| `ports` | No | 3000,8000,5432,3306,6379 | Port forwards `[{guest, host}]` |
| `exclude_patterns` | No | common dirs | Sync exclusions |

If a `Vagrantfile` already exists in `project_path`, the server automatically uses it instead of generating one.

**`ensure_dev_vm`** -- Make sure a VM is running (starts it if stopped, creates it if missing).

| Parameter | Required | Description |
|-----------|----------|-------------|
| `name` | Yes | VM name |
| `project_path` | No | Required only if the VM doesn't exist yet |

**`destroy_dev_vm`** -- Remove a VM and its resources.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `name` | Yes | VM name |

For existing-Vagrantfile VMs, only the MCP registration is removed -- the project's Vagrantfile and files are preserved.

**`get_vm_status`** -- Check VM state.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `name` | No | Specific VM name. Omit to list all VMs. |

Returns states: `running`, `poweroff`, `saved`, `not_created`, `unknown`.

#### Command Execution

**`exec_in_vm`** -- Run a command in the VM.

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `vm_name` | Yes | - | VM name |
| `command` | Yes | - | Shell command |
| `working_dir` | No | `/home/vagrant` | Directory to run in |

**`exec_with_sync`** -- Run a command with file sync before/after.

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `vm_name` | Yes | - | VM name |
| `command` | Yes | - | Shell command |
| `working_dir` | No | `/home/vagrant` | Directory to run in |
| `sync_before` | No | `true` | Sync host -> VM first |
| `sync_after` | No | `true` | Sync VM -> host after |

**`run_background_task`** -- Start a background process in the VM.

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `vm_name` | Yes | - | VM name |
| `command` | Yes | - | Shell command |
| `working_dir` | No | `/home/vagrant` | Directory to run in |
| `sync_before` | No | `true` | Sync host -> VM first |

#### File Synchronization

**`configure_sync`** -- Change sync method for a VM.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `vm_name` | Yes | VM name |
| `sync_type` | Yes | `rsync`, `nfs`, `smb`, `virtualbox` |
| `exclude_patterns` | No | Patterns to exclude |
| `host_path` | No | Host path |
| `guest_path` | No | Guest path |

**`sync_to_vm`** -- Push files from host to VM.

**`sync_from_vm`** -- Pull files from VM to host.

**`upload_to_vm`** -- Upload specific files/directories.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `vm_name` | Yes | VM name |
| `source` | Yes | Source path on host |
| `destination` | Yes | Destination path in VM |
| `compress` | No | Compress before upload |
| `compression_type` | No | `tgz` or `zip` |

**`sync_status`** -- Check current sync state.

**`resolve_sync_conflicts`** -- Resolve a file conflict.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `vm_name` | Yes | VM name |
| `path` | Yes | Conflicted file path |
| `resolution` | Yes | `use_host`, `use_vm`, `merge`, `keep_both` |

**`search_code`** -- Search files in the VM.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `vm_name` | Yes | VM name |
| `query` | Yes | Search query |
| `search_type` | No | `semantic`, `exact`, `fuzzy` |
| `max_results` | No | Max results |
| `case_sensitive` | No | Case sensitive |

#### Environment Setup

**`setup_dev_environment`** -- Install language runtimes.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `vm_name` | Yes | VM name |
| `runtimes` | Yes | e.g. `["node", "python", "go"]` |
| `tools` | No | Additional tools |

**`install_dev_tools`** -- Install specific dev tools.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `vm_name` | Yes | VM name |
| `tools` | Yes | e.g. `["docker", "git", "postgresql"]` |

**`configure_shell`** -- Set up shell environment.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `vm_name` | Yes | VM name |
| `shell_type` | No | `bash`, `zsh`, etc. |
| `env_vars` | No | Environment variables |
| `aliases` | No | Shell aliases |

### Typical Agent Workflows

#### Workflow 1: Project with an existing Vagrantfile

```
1. use_project_vm(name="myproject", project_path="/home/user/projects/myproject")
   → Registers the VM. The existing Vagrantfile is used as-is.

2. ensure_dev_vm(name="myproject")
   → Starts the VM if not running.

3. exec_in_vm(vm_name="myproject", command="npm test")
   → Runs tests inside the VM.

4. destroy_dev_vm(name="myproject")
   → Stops and unregisters. Vagrantfile and project files are preserved.
```

#### Workflow 2: Project without a Vagrantfile

```
1. create_dev_vm(
     name="dev-env",
     project_path="/home/user/projects/newapp",
     provider="libvirt",
     cpu=4,
     memory=4096
   )
   → Creates a VM with a generated Vagrantfile.

2. ensure_dev_vm(name="dev-env")
   → Starts the VM.

3. setup_dev_environment(vm_name="dev-env", runtimes=["node", "python"])
   → Installs Node.js and Python.

4. exec_with_sync(vm_name="dev-env", command="npm install && npm run build")
   → Syncs files, runs build, syncs results back.

5. destroy_dev_vm(name="dev-env")
   → Removes VM and all generated files.
```

#### Workflow 3: Check status and run a quick command

```
1. get_vm_status()
   → Lists all VMs and their states.

2. exec_in_vm(vm_name="myproject", command="ls -la /vagrant")
   → Lists files in the synced project directory.
```

### Important Notes for Agents

1. **Always check for existing Vagrantfiles first.** If the project has a `Vagrantfile`, use `use_project_vm` -- don't generate a new one.
2. **`create_dev_vm` auto-detects Vagrantfiles.** Even if you call `create_dev_vm` on a project with an existing `Vagrantfile`, it will use the existing one.
3. **Provider matters.** Set `provider` to match your system's virtualization (libvirt on Linux, VirtualBox on cross-platform, etc.).
4. **`destroy_dev_vm` is safe for existing Vagrantfiles.** It only removes the MCP registration, not the project files.
5. **Use `ensure_dev_vm` instead of manually checking state.** It handles create/start/idempotency.
6. **Sync before running commands that depend on local changes.** Use `exec_with_sync` or call `sync_to_vm` before `exec_in_vm`.

## Development

### Prerequisites

- Go 1.24+
- Vagrant CLI
- A virtualization provider (libvirt, VirtualBox, etc.)

### Common Tasks

```bash
make build              # Build binary
make test               # Fast unit tests (no VMs started)
make test-integration   # Creates VMs but doesn't start them
make test-vm-start      # Full VM lifecycle (very slow)
make fmt                # Format code
make lint               # Lint
make sec                # Security scan
make all                # fmt + lint + sec + test + build
make tools              # Install dev tools (golangci-lint, gosec)
```

### Testing with MCP Inspector

```bash
npm install -g @modelcontextprotocol/inspector
mcp-inspector ./bin/vagrant-mcp-server
```

### Release Process

Releases are created by pushing a version tag:

```bash
git tag v1.0.0
git push origin v1.0.0
```

GitHub Actions builds cross-platform binaries, generates checksums, and creates a release.

## Privacy

- No data is collected, stored, or transmitted to external servers
- All operations are local (host <-> VM only)
- No telemetry or analytics
- Logs stay on your machine

## Security Considerations

- The server runs with your user privileges
- It can create/modify/delete files in project directories
- It can execute commands in VMs (which may affect host via sync)
- VMs may forward ports to localhost
- Use sync exclusion patterns for sensitive files (`.env`, keys, etc.)
- Destroy VMs when no longer needed

## License

Mozilla Public License 2.0 (MPL-2.0). Copyright (c) 2025 Ricardo Oliveira.
