# Izanami Go CLI

A cross-platform command-line client for [Izanami](https://github.com/MAIF/izanami) feature flag management.

Izanami is an open-source feature flag and configuration management system. This CLI provides a convenient way to interact with Izanami for both administration tasks and standard feature flag operations.

## Features

- ✅ **Cross-platform**: Works on Linux, macOS (Intel & ARM), and Windows
- 🔐 **Flexible Authentication**: Supports both client API keys and personal access tokens
- ⚙️ **Multiple Configuration Sources**: Environment variables, config files, and command-line flags
- 📊 **Multiple Output Formats**: JSON (default) and human-friendly table format
- 🚀 **Feature Management**: Create, update, delete, and evaluate feature flags
- 🌍 **Context Management**: Manage feature contexts (environments/overrides)
- 👨‍💼 **Admin Operations**: Manage tenants, projects, tags, webhooks, and users
- 📦 **Import/Export**: Bulk data migration capabilities
- 🔍 **Global Search**: Search across all Izanami resources
- 🐚 **Shell Completion**: Bash, Zsh, Fish, and PowerShell support

## Installation

### Pre-built Binaries

Download the latest release for your platform from the [releases page](https://github.com/webskin/izanami-go-cli/releases).

#### Linux

```bash
# AMD64
curl -L https://github.com/webskin/izanami-go-cli/releases/latest/download/izanami-go-cli_linux_amd64.tar.gz | tar xz
sudo mv iz /usr/local/bin/

# ARM64
curl -L https://github.com/webskin/izanami-go-cli/releases/latest/download/izanami-go-cli_linux_arm64.tar.gz | tar xz
sudo mv iz /usr/local/bin/
```

#### macOS

```bash
# Intel
curl -L https://github.com/webskin/izanami-go-cli/releases/latest/download/izanami-go-cli_darwin_amd64.tar.gz | tar xz
sudo mv iz /usr/local/bin/

# Apple Silicon
curl -L https://github.com/webskin/izanami-go-cli/releases/latest/download/izanami-go-cli_darwin_arm64.tar.gz | tar xz
sudo mv iz /usr/local/bin/
```

#### Windows

Download the `.zip` file from the releases page, extract it, and add the directory to your PATH.

### Build from Source

Requires Go 1.21 or later.

```bash
# Clone the repository
git clone https://github.com/webskin/izanami-go-cli.git
cd izanami-go-cli

# Build for your current platform
make build

# Or install to $GOPATH/bin
make install

# Build for all platforms
make build-all
```

## Configuration

The CLI can be configured through three methods (in order of precedence):

1. **Command-line flags** (highest priority)
2. **Environment variables** (prefixed with `IZ_`)
3. **Config file** (lowest priority)

### Config File

The CLI looks for a config file at:
- Linux/macOS: `~/.config/iz/config.yaml`
- Windows: `%APPDATA%\iz\config.yaml`

Example config file:

```yaml
# Base URL of your Izanami instance (required)
base_url: "https://izanami.example.com"

# Client authentication (for feature evaluation)
client_id: "your-client-id"
client_secret: "your-client-secret"

# Admin authentication (for admin operations)
username: "your-username"
token: "your-personal-access-token"

# Default tenant
tenant: "default"

# Default project
project: "my-project"

# Default context (e.g., "prod", "dev", "prod/eu/france")
context: "prod"

# Request timeout in seconds
timeout: 30

# Verbose output
verbose: false
```

### Environment Variables

All config file options can be set via environment variables:

```bash
export IZ_BASE_URL="https://izanami.example.com"
export IZ_CLIENT_ID="your-client-id"
export IZ_CLIENT_SECRET="your-client-secret"
export IZ_USERNAME="your-username"
export IZ_TOKEN="your-personal-access-token"
export IZ_TENANT="default"
export IZ_PROJECT="my-project"
export IZ_CONTEXT="prod"
```

### Authentication

The CLI supports two authentication methods:

#### 1. Client API Key (for feature evaluation)

Used for checking feature flags (read-only operations):

```bash
iz features check my-feature \
  --url https://izanami.example.com \
  --client-id your-client-id \
  --client-secret your-client-secret \
  --user user123
```

#### 2. Personal Access Token (for admin operations)

Used for administrative operations:

```bash
iz admin projects list \
  --url https://izanami.example.com \
  --username your-username \
  --token your-personal-access-token \
  --tenant default
```

## Usage

### Basic Commands

#### Health Check

```bash
# Check if Izanami is reachable
iz health

# Output:
# Status:  UP
# Version: 2.0.0
# URL:     https://izanami.example.com
```

#### Version

```bash
iz version

# Output:
# iz version 1.0.0
#   Commit:    abc1234
#   Built:     2025-01-15T10:00:00Z
#   Go:        go1.21.0
#   Platform:  linux/amd64
```

### Feature Management

#### List Features

```bash
# List all features in a tenant
iz features list --tenant my-tenant

# List features in a specific project
iz features list --tenant my-tenant --project my-project

# Filter by tags
iz features list --tenant my-tenant --tags auth,beta

# Output as table
iz features list --tenant my-tenant -o table
```

#### Get Feature

```bash
# Get detailed feature information (including context overloads)
iz features get my-feature --tenant my-tenant
```

#### Check Feature (Evaluate)

```bash
# Check if a feature is active for a user
iz features check my-feature --user user123

# Check with context
iz features check my-feature --user user123 --context prod/eu

# Output:
# {
#   "active": true,
#   "name": "my-feature",
#   "project": "my-project"
# }
```

#### Create Feature

```bash
# Create a simple boolean feature
iz features create my-new-feature \
  --tenant my-tenant \
  --project my-project \
  --description "My new feature" \
  --enabled

# Create from JSON file
iz features create my-feature \
  --tenant my-tenant \
  --project my-project \
  --data @feature.json

# Create from stdin
cat feature.json | iz features create my-feature \
  --tenant my-tenant \
  --project my-project \
  --data -
```

Example `feature.json`:

```json
{
  "name": "my-feature",
  "description": "A feature with conditions",
  "enabled": true,
  "conditions": [
    {
      "rule": {
        "type": "UserPercentage",
        "percentage": 50
      }
    }
  ]
}
```

#### Update Feature

```bash
iz features update my-feature \
  --tenant my-tenant \
  --data @updated-feature.json
```

#### Delete Feature

```bash
iz features delete my-feature --tenant my-tenant
```

### Context Management

Contexts allow different feature behavior in different environments.

#### List Contexts

```bash
# List root-level contexts
iz contexts list --tenant my-tenant

# List all nested contexts
iz contexts list --tenant my-tenant --all

# List project-specific contexts
iz contexts list --tenant my-tenant --project my-project
```

#### Create Context

```bash
# Create a root-level global context
iz contexts create prod --tenant my-tenant --global

# Create a project-specific context
iz contexts create prod \
  --tenant my-tenant \
  --project my-project

# Create a nested context
iz contexts create france \
  --tenant my-tenant \
  --project my-project \
  --parent prod/eu
```

#### Delete Context

```bash
iz contexts delete prod/eu/france \
  --tenant my-tenant \
  --project my-project
```

### Admin Operations

Admin operations require username and personal access token authentication.

#### Tenant Management

```bash
# List all tenants
iz admin tenants list

# Get a specific tenant
iz admin tenants get my-tenant

# Create a tenant
iz admin tenants create new-tenant --description "New tenant"

# Delete a tenant (WARNING: deletes all data)
iz admin tenants delete old-tenant
```

#### Project Management

```bash
# List projects
iz admin projects list --tenant my-tenant

# Get a project
iz admin projects get my-project --tenant my-tenant

# Create a project
iz admin projects create new-project \
  --tenant my-tenant \
  --description "New project"

# Delete a project
iz admin projects delete old-project --tenant my-tenant
```

#### Tag Management

```bash
# List tags
iz admin tags list --tenant my-tenant

# Create a tag
iz admin tags create beta \
  --tenant my-tenant \
  --description "Beta features"

# Delete a tag
iz admin tags delete old-tag --tenant my-tenant
```

#### Search

```bash
# Search globally
iz admin search "authentication"

# Search within a tenant
iz admin search "user" --tenant my-tenant

# Search with filters
iz admin search "api" --tenant my-tenant --filter FEATURE,PROJECT
```

Available filters: `PROJECT`, `FEATURE`, `KEY`, `TAG`, `SCRIPT`, `GLOBAL_CONTEXT`, `LOCAL_CONTEXT`, `WEBHOOK`

#### Import/Export

```bash
# Export tenant data
iz admin export --tenant my-tenant --output backup.ndjson

# Export to stdout
iz admin export --tenant my-tenant

# Import data
iz admin import backup.ndjson --tenant my-tenant

# Import with conflict resolution
iz admin import backup.ndjson \
  --tenant my-tenant \
  --conflict OVERWRITE

# Import with timezone
iz admin import backup.ndjson \
  --tenant my-tenant \
  --timezone "Europe/Paris"
```

Conflict strategies: `FAIL` (default), `SKIP`, `OVERWRITE`

### Output Formats

The CLI supports two output formats:

#### JSON (default)

```bash
iz features list --tenant my-tenant
```

Output:
```json
[
  {
    "id": "feature-1",
    "name": "feature-1",
    "description": "First feature",
    "project": "my-project",
    "enabled": true,
    "tags": ["beta"]
  }
]
```

#### Table

```bash
iz features list --tenant my-tenant -o table
```

Output:
```
id         name       description     project      enabled  tags
feature-1  feature-1  First feature   my-project   true     [beta]
```

### Shell Completion

Enable shell completion for a better experience:

#### Bash

```bash
# Load in current session
source <(iz completion bash)

# Load for all sessions (Linux)
iz completion bash > /etc/bash_completion.d/iz

# Load for all sessions (macOS)
iz completion bash > $(brew --prefix)/etc/bash_completion.d/iz
```

#### Zsh

```bash
# Enable completion if not already enabled
echo "autoload -U compinit; compinit" >> ~/.zshrc

# Load completions
iz completion zsh > "${fpath[1]}/_iz"

# Restart shell
```

#### Fish

```bash
# Load in current session
iz completion fish | source

# Load for all sessions
iz completion fish > ~/.config/fish/completions/iz.fish
```

#### PowerShell

```powershell
# Load in current session
iz completion powershell | Out-String | Invoke-Expression

# Load for all sessions
iz completion powershell > iz.ps1
# Then source this file from your PowerShell profile
```

## Examples

### DevOps/CI Pipeline Usage

```bash
#!/bin/bash
# deploy-feature.sh - Deploy a feature flag in a CI pipeline

set -e

# Configuration via environment variables
export IZ_BASE_URL="https://izanami.example.com"
export IZ_USERNAME="ci-user"
export IZ_TOKEN="$CI_TOKEN"
export IZ_TENANT="production"

# Check Izanami health
iz health || exit 1

# Create or update feature
if iz features get my-new-feature 2>/dev/null; then
  echo "Updating existing feature..."
  iz features update my-new-feature --data @feature.json
else
  echo "Creating new feature..."
  iz features create my-new-feature --project my-project --data @feature.json
fi

echo "Feature deployed successfully!"
```

### Feature Flag Rollout Script

```bash
#!/bin/bash
# gradual-rollout.sh - Gradually roll out a feature

FEATURE="new-ui"
TENANT="production"
PROJECT="web-app"

# Start at 10%
echo "Rolling out to 10% of users..."
cat <<EOF | iz features update $FEATURE --tenant $TENANT --data -
{
  "enabled": true,
  "conditions": [{
    "rule": {
      "type": "UserPercentage",
      "percentage": 10
    }
  }]
}
EOF

sleep 300  # Wait 5 minutes

# Increase to 50%
echo "Rolling out to 50% of users..."
cat <<EOF | iz features update $FEATURE --tenant $TENANT --data -
{
  "enabled": true,
  "conditions": [{
    "rule": {
      "type": "UserPercentage",
      "percentage": 50
    }
  }]
}
EOF

sleep 300  # Wait 5 minutes

# Full rollout
echo "Full rollout - 100% of users..."
cat <<EOF | iz features update $FEATURE --tenant $TENANT --data -
{
  "enabled": true,
  "conditions": [{
    "rule": {
      "type": "All"
    }
  }]
}
EOF

echo "Rollout complete!"
```

### Backup Script

```bash
#!/bin/bash
# backup-izanami.sh - Backup all tenants

BACKUP_DIR="./backups/$(date +%Y%m%d)"
mkdir -p "$BACKUP_DIR"

# Get all tenants
TENANTS=$(iz admin tenants list -o json | jq -r '.[].name')

for tenant in $TENANTS; do
  echo "Backing up tenant: $tenant"
  iz admin export --tenant "$tenant" --output "$BACKUP_DIR/${tenant}.ndjson"
done

echo "Backup complete! Files saved to $BACKUP_DIR"
```

## Development

### Building

```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Install to $GOPATH/bin
make install
```

### Testing

```bash
# Run tests
make test

# Run tests with coverage
make test-coverage

# Lint code
make lint
```

### Project Structure

```
izanami-go-cli/
├── cmd/
│   └── iz/
│       └── main.go              # CLI entry point
├── internal/
│   ├── cmd/
│   │   ├── root.go              # Root command
│   │   ├── features.go          # Feature commands
│   │   ├── contexts.go          # Context commands
│   │   ├── admin.go             # Admin commands
│   │   ├── version.go           # Version command
│   │   ├── health.go            # Health check
│   │   └── completion.go        # Shell completion
│   ├── izanami/
│   │   ├── client.go            # HTTP client
│   │   ├── client_test.go       # Client tests
│   │   ├── config.go            # Configuration
│   │   └── types.go             # Domain types
│   └── output/
│       ├── formatter.go         # Output formatting
│       └── formatter_test.go    # Formatter tests
├── go.mod                       # Go module definition
├── Makefile                     # Build automation
├── .goreleaser.yaml             # Release configuration
└── README.md                    # This file
```

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- [Izanami](https://github.com/MAIF/izanami) - The feature flag management system
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Resty](https://github.com/go-resty/resty) - HTTP client
- [Viper](https://github.com/spf13/viper) - Configuration management

## Support

- 📖 [Izanami Documentation](https://maif.github.io/izanami/)
- 🐛 [Report Issues](https://github.com/webskin/izanami-go-cli/issues)
- 💬 [Discussions](https://github.com/webskin/izanami-go-cli/discussions)
