# Izanami Go CLI

A cross-platform command-line client for [Izanami](https://github.com/MAIF/izanami) feature flag management.

Izanami is an open-source feature flag and configuration management system. This CLI provides a convenient way to interact with Izanami for both administration tasks and standard feature flag operations.

> **WARNING: WORK IN PROGRESS**
>
> This CLI is currently under active development and **NOT ready for production use**.
> Features may be incomplete, APIs may change, and bugs are expected.
> Use at your own risk in development/testing environments only.

## Features

- **Cross-platform**: Works on Linux, macOS (Intel & ARM), and Windows
- **Profile Management**: Multiple environment profiles with per-tenant/project credentials
- **OAuth/OIDC Login**: Browser-based authentication flow
- **Flexible Authentication**: Supports client API keys, personal access tokens, and JWT sessions
- **Multiple Configuration Sources**: Environment variables, config files, and command-line flags
- **Multiple Output Formats**: JSON (default) and human-friendly table format
- **Feature Management**: Create, update, delete, and evaluate feature flags
- **Real-time Events**: Watch feature flag changes via SSE
- **Context Management**: Manage feature contexts (environments/overrides)
- **Admin Operations**: Manage tenants, projects, tags, webhooks, and users
- **Import/Export**: Bulk data migration capabilities
- **Global Search**: Search across all Izanami resources
- **Config Management**: CLI configuration commands
- **Shell Completion**: Bash, Zsh, Fish, and PowerShell support

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

## Quick Start

```bash
# Login to Izanami (opens browser for OIDC)
iz login --url https://izanami.example.com --oidc

# Or login with username/password
iz login https://izanami.example.com admin

# Create a profile with saved settings
iz profiles add prod --url https://izanami.example.com
iz profiles use prod
iz profiles set tenant my-tenant

# Check a feature flag
iz features check my-feature --tenant my-tenant

# Watch for real-time changes
iz events watch --tenant my-tenant
```

## Configuration

The CLI can be configured through three methods (in order of precedence):

1. **Command-line flags** (highest priority)
2. **Environment variables** (prefixed with `IZ_`)
3. **Profile settings** (via `iz profiles`)
4. **Config file** (lowest priority)

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
export IZ_CLIENT_BASE_URL="https://client.izanami.example.com"  # Optional: separate URL for client operations
export IZ_USERNAME="your-username"
export IZ_TOKEN="your-personal-access-token"
export IZ_JWT_TOKEN="your-jwt-token"  # From login session
export IZ_TENANT="default"
export IZ_PROJECT="my-project"
export IZ_CONTEXT="prod"
```

### Profiles

Profiles allow separate configurations for different environments.

```bash
# Create profiles
iz profiles add local --url http://localhost:9000
iz profiles add prod --url https://izanami.prod.com

# Switch profiles
iz profiles use prod

# Show current profile
iz profiles current

# List all profiles
iz profiles list

# Show profile details
iz profiles show prod

# Update profile settings
iz profiles set tenant my-default-tenant
iz profiles set project my-default-project
iz profiles set client-base-url https://client.example.com

# Remove a profile setting
iz profiles unset tenant

# Use different profile for single command
iz features check my-feature --profile local

# Delete a profile
iz profiles delete old-profile
```

#### Client Keys (per tenant/project)

Store client credentials for feature evaluation at the tenant or project level:

```bash
# Add client credentials for a tenant
iz profiles client-keys add --tenant my-tenant

# Add client credentials for a specific project
iz profiles client-keys add --tenant my-tenant --project my-project

# List stored client credentials
iz profiles client-keys list

# Delete client credentials
iz profiles client-keys delete --tenant my-tenant <client-id>
```

### Sessions

Sessions store JWT tokens from login. Sessions are referenced by profiles.

```bash
# List all sessions
iz sessions list

# Delete a session
iz sessions delete my-session
```

### Configuration Commands

```bash
# View all settings (global and profile-specific)
iz config list

# Get a specific value
iz config get timeout

# Set a global value
iz config set timeout 60

# Remove a global value
iz config unset verbose

# Show config file location
iz config path

# Initialize config file
iz config init

# Validate configuration
iz config validate

# Reset configuration to defaults
iz config reset
```

## Authentication

The CLI supports multiple authentication methods:

### 1. OAuth/OIDC Login (browser-based)

```bash
# Login via OIDC (opens browser, waits for authentication)
iz login --oidc --url https://izanami.example.com

# Login with custom session name
iz login --oidc --url https://izanami.example.com --name prod-session

# Login with custom timeout
iz login --oidc --url https://izanami.example.com --timeout 10m

# Login without opening browser (prints URL only)
iz login --oidc --url https://izanami.example.com --no-browser

# Login with token directly (for scripting)
iz login --oidc --url https://izanami.example.com --token "eyJhbGciOiJIUzI1NiIs..."

# Logout
iz logout

# Logout from specific profile's session
iz logout --profile prod
```

### 2. Username/Password Login

```bash
# Login with username (will prompt for password)
iz login http://localhost:9000 admin

# Login with password (not recommended for security)
iz login http://localhost:9000 admin --password secret
```

### 3. Client API Key (for feature evaluation)

Used for checking feature flags (read-only operations):

```bash
iz features check my-feature \
  --url https://izanami.example.com \
  --client-id your-client-id \
  --client-secret your-client-secret \
  --user user123
```

### 4. Personal Access Token (for admin operations)

Used for administrative operations:

```bash
iz admin projects list \
  --url https://izanami.example.com \
  --personal-access-token-username your-username \
  --personal-access-token your-personal-access-token \
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

### Feature Management (Client)

Client operations for checking feature flags.

#### Check Feature (Evaluate)

```bash
# Check if a feature is active for a user
iz features check my-feature \
  --client-id your-client-id \
  --client-secret your-client-secret \
  --user user123

# Check with context
iz features check my-feature \
  --client-id your-client-id \
  --client-secret your-client-secret \
  --user user123 \
  --context prod/eu

# Output:
# {
#   "active": true,
#   "name": "my-feature",
#   "project": "my-project"
# }
```

#### Bulk Check Multiple Features

```bash
# Check multiple features at once
iz features check-bulk feat1,feat2,feat3 --tenant my-tenant

# With user context
iz features check-bulk feat1,feat2 --tenant my-tenant --user user123
```

### Events

Watch for real-time feature flag changes via Server-Sent Events.

```bash
# Watch all events for a tenant
iz events watch --tenant my-tenant

# Watch specific features (by name, requires --project)
iz events watch --tenant my-tenant --project my-project --features my-feature

# Watch specific features (by UUID)
iz events watch --features 550e8400-e29b-41d4-a716-446655440000

# Watch all features in a project
iz events watch --tenant my-tenant --projects my-project

# Watch with context
iz events watch --tenant my-tenant --projects my-project --context PROD

# Watch with tag filtering
iz events watch --tenant my-tenant --one-tag-in beta,experimental

# Watch events with pretty JSON formatting
iz events watch --pretty

# Watch raw SSE format (shows event IDs and types)
iz events watch --raw
```

### Admin Feature Management

Administrative operations require elevated privileges (JWT or PAT authentication).

#### List Features

```bash
# List all features in a tenant
iz admin features list --tenant my-tenant

# List features in a specific project
iz admin features list --tenant my-tenant --project my-project

# Filter by tags
iz admin features list --tenant my-tenant --tags auth,beta

# Output as table
iz admin features list --tenant my-tenant -o table
```

#### Get Feature

```bash
# Get detailed feature information (including context overloads)
iz admin features get my-feature --tenant my-tenant --project my-project
```

#### Create Feature

```bash
# Create a simple boolean feature
iz admin features create my-new-feature \
  --tenant my-tenant \
  --project my-project \
  --description "My new feature" \
  --enabled

# Create from JSON file
iz admin features create my-feature \
  --tenant my-tenant \
  --project my-project \
  --data @feature.json

# Create from stdin
cat feature.json | iz admin features create my-feature \
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
# Update from file (feature ID is UUID, not name)
iz admin features update e878a149-df86-4f28-b1db-059580304e1e \
  --tenant my-tenant \
  --project my-project \
  --data @updated-feature.json

# Update from stdin
cat <<EOF | iz admin features update e878a149-df86-4f28-b1db-059580304e1e \
  --tenant my-tenant \
  --project my-project \
  --data -
{
  "name": "my-feature",
  "enabled": true,
  "description": "Updated description",
  "resultType": "boolean",
  "conditions": [],
  "metadata": {},
  "tags": ["production"]
}
EOF
```

**Note:** All fields are required for updates:
- `name` - Feature name (not the UUID)
- `enabled` - Boolean flag
- `description` - Feature description
- `resultType` - Type of value ("boolean", "string", "number")
- `conditions` - Array of activation conditions (can be empty)
- `metadata` - Metadata object (can be empty)
- `tags` - Optional tags array

The feature ID (UUID) and project are provided via command arguments and automatically merged into the request.

#### Delete Feature

```bash
iz admin features delete my-feature --tenant my-tenant --project my-project
```

#### Patch Features (Batch Update)

```bash
# Batch update multiple features
iz admin features patch --tenant my-tenant --project my-project --data @patch.json
```

#### Test Features

```bash
# Test an existing feature's evaluation
iz admin features test my-feature --tenant my-tenant --project my-project --user testuser

# Test a feature definition without saving
iz admin features test-definition --tenant my-tenant --project my-project --data @feature-def.json

# Test multiple features at once
iz admin features test-bulk feat1,feat2 --tenant my-tenant --project my-project --user testuser
```

### Context Management

Contexts allow different feature behavior in different environments.

#### List Contexts

```bash
# List root-level contexts
iz admin contexts list --tenant my-tenant

# List all nested contexts
iz admin contexts list --tenant my-tenant --all

# List project-specific contexts
iz admin contexts list --tenant my-tenant --project my-project
```

#### Create Context

```bash
# Create a root-level global context
iz admin contexts create prod --tenant my-tenant --global

# Create a project-specific context
iz admin contexts create prod \
  --tenant my-tenant \
  --project my-project

# Create a nested context
iz admin contexts create france \
  --tenant my-tenant \
  --project my-project \
  --parent prod/eu
```

#### Delete Context

```bash
iz admin contexts delete prod/eu/france \
  --tenant my-tenant \
  --project my-project
```

### Feature Overloads

Overloads allow context-specific feature strategies.

```bash
# Set a simple overload (enable feature for all users in PROD)
iz admin overloads set my-feature --context PROD --project my-project --enabled

# Set an overload with conditions
iz admin overloads set my-feature --context PROD --project my-project --data '{
  "enabled": true,
  "conditions": [{"rule": {"type": "UserList", "users": ["Bob"]}}]
}'

# View an existing overload
iz admin overloads get my-feature --context PROD --project my-project

# Delete an overload
iz admin overloads delete my-feature --context PROD --project my-project
```

### Admin Operations

Admin operations require username and personal access token authentication (or JWT from login).

#### Tenant Management

```bash
# List all tenants
iz admin tenants list

# Get a specific tenant
iz admin tenants get my-tenant

# Create a tenant
iz admin tenants create new-tenant --description "New tenant"

# Update a tenant
iz admin tenants update my-tenant --description "Updated description"

# Delete a tenant (WARNING: deletes all data)
iz admin tenants delete old-tenant

# View tenant event logs
iz admin tenants logs --tenant my-tenant
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

# Update a project
iz admin projects update my-project --tenant my-tenant --description "Updated"

# Delete a project
iz admin projects delete old-project --tenant my-tenant

# View project event logs
iz admin projects logs --tenant my-tenant --project my-project
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

#### API Key Management

```bash
# List API keys
iz admin keys list --tenant my-tenant

# Create an API key
iz admin keys create my-key --tenant my-tenant --project my-project

# Delete an API key
iz admin keys delete my-key --tenant my-tenant
```

#### User Management

```bash
# List all users
iz admin users list

# List users for a specific tenant
iz admin users list-for-tenant --tenant my-tenant

# List users for a specific project
iz admin users list-for-project --tenant my-tenant --project my-project

# Search for users
iz admin users search john

# Get user details
iz admin users get johndoe

# Get user's rights for a tenant
iz admin users get-for-tenant johndoe --tenant my-tenant

# Create a new user
iz admin users create johndoe --email john@example.com --password secret123 --admin

# Update user information
iz admin users update johndoe --email newemail@example.com

# Invite users to a tenant
iz admin users invite-to-tenant --tenant my-tenant --users user1,user2 --level READ

# Invite users to a project
iz admin users invite-to-project --tenant my-tenant --project my-project --users user1 --level WRITE

# Update user's global rights
iz admin users update-rights johndoe --admin

# Update user's tenant rights
iz admin users update-tenant-rights johndoe --tenant my-tenant --level ADMIN

# Update user's project rights
iz admin users update-project-rights johndoe --tenant my-tenant --project my-project --level WRITE

# Delete a user
iz admin users delete johndoe
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

# Check status of async V1 import
iz admin import-status <import-id>
```

Conflict strategies: `FAIL` (default), `SKIP`, `OVERWRITE`

### Output Formats

The CLI supports two output formats:

#### JSON (default with --output json)

```bash
iz admin features list --tenant my-tenant -o json
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

#### Table (default)

```bash
iz admin features list --tenant my-tenant -o table
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
FEATURE_ID="e878a149-df86-4f28-b1db-059580304e1e"  # Feature UUID

if iz admin features get "$FEATURE_ID" --tenant "$IZ_TENANT" --project my-project 2>/dev/null; then
  echo "Updating existing feature..."
  iz admin features update "$FEATURE_ID" --tenant "$IZ_TENANT" --project my-project --data @feature.json
else
  echo "Creating new feature..."
  iz admin features create my-new-feature --tenant "$IZ_TENANT" --project my-project --data @feature.json
fi

echo "Feature deployed successfully!"
```

### Feature Flag Rollout Script

```bash
#!/bin/bash
# gradual-rollout.sh - Gradually roll out a feature

FEATURE_ID="e878a149-df86-4f28-b1db-059580304e1e"  # Feature UUID
FEATURE_NAME="new-ui"
TENANT="production"
PROJECT="web-app"

# Start at 10%
echo "Rolling out to 10% of users..."
cat <<EOF | iz admin features update $FEATURE_ID --tenant $TENANT --project $PROJECT --data -
{
  "name": "$FEATURE_NAME",
  "enabled": true,
  "description": "New UI feature - gradual rollout",
  "resultType": "boolean",
  "conditions": [{
    "rule": {
      "type": "UserPercentage",
      "percentage": 10
    }
  }],
  "metadata": {},
  "tags": ["ui", "rollout"]
}
EOF

sleep 300  # Wait 5 minutes

# Increase to 50%
echo "Rolling out to 50% of users..."
cat <<EOF | iz admin features update $FEATURE_ID --tenant $TENANT --project $PROJECT --data -
{
  "name": "$FEATURE_NAME",
  "enabled": true,
  "description": "New UI feature - gradual rollout",
  "resultType": "boolean",
  "conditions": [{
    "rule": {
      "type": "UserPercentage",
      "percentage": 50
    }
  }],
  "metadata": {},
  "tags": ["ui", "rollout"]
}
EOF

sleep 300  # Wait 5 minutes

# Full rollout
echo "Full rollout - 100% of users..."
cat <<EOF | iz admin features update $FEATURE_ID --tenant $TENANT --project $PROJECT --data -
{
  "name": "$FEATURE_NAME",
  "enabled": true,
  "description": "New UI feature - fully rolled out",
  "resultType": "boolean",
  "conditions": [{
    "rule": {
      "type": "All"
    }
  }],
  "metadata": {},
  "tags": ["ui", "production"]
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
│   │   ├── root.go              # Root command & global flags
│   │   ├── admin.go             # Admin command group
│   │   ├── config.go            # Config commands
│   │   ├── contexts.go          # Context commands
│   │   ├── events.go            # Event commands (SSE)
│   │   ├── features.go          # Admin feature commands
│   │   ├── features_check.go    # Client feature check commands
│   │   ├── health.go            # Health check
│   │   ├── import_export.go     # Import/export commands
│   │   ├── keys.go              # API key commands
│   │   ├── login.go             # Login/logout commands
│   │   ├── overloads.go         # Overload commands
│   │   ├── profiles.go          # Profile commands
│   │   ├── projects.go          # Project commands
│   │   ├── search.go            # Search command
│   │   ├── sessions.go          # Session commands
│   │   ├── tags.go              # Tag commands
│   │   ├── tenants.go           # Tenant commands
│   │   ├── users.go             # User commands
│   │   ├── webhooks.go          # Webhook commands
│   │   ├── completion.go        # Shell completion
│   │   └── version.go           # Version command
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

- [Izanami Documentation](https://maif.github.io/izanami/)
- [Report Issues](https://github.com/webskin/izanami-go-cli/issues)
- [Discussions](https://github.com/webskin/izanami-go-cli/discussions)
