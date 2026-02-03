# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **`--quiet` / `-q` global flag**: Suppresses all stdout output (exit code only); mutually exclusive with `--verbose`
- **`profiles add --active` flag**: Immediately sets the new profile as active on creation
- **`profiles workers add --default` flag**: Sets the new worker as the default worker
- **Non-interactive `client-keys add`**: New `--client-id` and `--client-secret` flags for scripted/CI usage (both profile-level and worker-level)
- **Worker-level ClientKeys**: Workers now support hierarchical `ClientKeys` maps (tenant → project credentials), matching profile-level behavior
- **Config reset backup**: `config reset` now creates a timestamped backup before deleting
- **`profiles list` Workers column**: Shows worker names with default annotation
- **`worker-url` and `worker-name` in `config list`** output
- New tests for quiet flag, active flag, default worker flag, and `copyConfig` deep-copy

### Changed
- **Credential model**: Removed flat `ClientID`/`ClientSecret` fields from `Profile` and `WorkerConfig`; use `ClientKeys` map exclusively
- **Environment variables**: `IZ_BASE_URL` → `IZ_LEADER_URL`, `IZ_CLIENT_BASE_URL` → `IZ_WORKER_URL`
- **Config key**: `base-url` → `leader-url` in YAML config files
- **Worker resolution** now runs once at root level (`PersistentPreRunE`); per-command re-resolution only when `--worker` flag is explicitly set
- **`profiles add`** no longer prompts for client credentials inline; suggests `iz profiles client-keys add` instead
- **`profiles show`** displays "N tenant(s) configured" for client keys and lists worker names
- **`client-keys` commands** moved from `profiles.go` to dedicated `client_keys.go`
- **`performLogin()`** now accepts and passes through `--insecure`, `--verbose`, and `--timeout` flags

### Fixed
- `--insecure` flag had no effect on health checks
- `performLogin()` ignored `--insecure`, `--verbose`, and `--timeout` settings
- Worker resolution ran unconditionally even without workers, producing spurious errors

### Refactored
- **Credential resolution**: `ResolveClientCredentials` method now delegates to standalone `ResolveClientCredentialsFromKeys`, eliminating duplicate logic
- **Test helpers**: Removed redundant `createTestConfigWithWorkers`; `createTestConfig` handles all profile fields including workers
- **HTTP logging**: Extracted `logRequestToStderr`/`logResponseToStderr` shared functions; admin and feature-check loggers both delegate to them
- **`copyConfig()`** expanded to deep-copy all fields including `ClientKeys`, `OutputFormat`, `Color`, `Username`, `AuthMethod`

## [0.1.0] - 2025-11-14

### Added

#### Core Features
- **Complete CLI implementation** for Izanami feature flag management
- **Cross-platform support**: Linux (amd64, arm64), macOS (Intel, Apple Silicon), Windows
- **Multiple authentication methods**:
  - Session-based authentication (JWT cookies) via `iz login`
  - Client API key authentication for feature checks
  - Personal Access Token (PAT) support (planned)
- **Flexible configuration**: Environment variables, config files, and command-line flags
- **Multiple output formats**: JSON (default) and human-friendly table format

#### Commands
- **Admin operations**:
  - Tenants: list, get, create, update, delete
  - Projects: list, get, create, update, delete
  - Users: list, get, create, update, delete
  - API Keys: list, get, create, update, delete
  - Tags: list, get, create, update, delete
  - Webhooks: list, get, create, update, delete
  - Import/Export: bulk data migration
  - Global search across all resources

- **Feature management**:
  - List, get, create, update, delete features
  - Feature evaluation (check command)
  - Copy features between contexts/projects

- **Context management**:
  - List, get, create, update, delete contexts
  - Hierarchical context structure support

- **Session management**:
  - Login with username/password
  - Logout and session cleanup
  - Session persistence in `~/.izsessions`

- **Utility commands**:
  - Version information with build details
  - Commands overview (tree view of all available commands)
  - Shell completion (Bash, Zsh, Fish, PowerShell)

#### Developer Experience
- **Makefile** with targets for building, testing, and releasing
- **GoReleaser configuration** for automated multi-platform releases
- **GitHub Actions CI/CD**:
  - CI workflow: builds and tests on every push/PR
  - Release workflow: automated releases on git tags
- **Development aliases** (`aliases.sh`) for common workflows
- **Comprehensive documentation**:
  - README.md with usage examples
  - RELEASING.md with release process
  - CLAUDE.md with developer guidelines
  - TODO.md with roadmap and concepts

#### Real-time Features
- **SSE event streaming**: Watch feature flag changes in real-time with auto-reconnection

### Fixed
- Auto-merge required fields for feature creation (resultType, conditions, description, metadata)
- Validation for feature updates with helpful error messages showing current structure
- Context get command now searches recursively (no server-side single-context endpoint)
- Code formatting issues (gofmt)
- Configuration loading priority: environment variables → session → flags

### Changed
- Feature update command requires explicit name field in JSON (not auto-merged from ID)
- Feature update auto-merges `id` and `project` fields from arguments
- Authentication split: separate client creation for Admin API vs Client API
- Feature check command uses client API authentication (will use IZ_CLIENT_ID/IZ_CLIENT_SECRET)

### Documentation
- Added WIP warning to README
- Updated all feature examples with correct required fields
- Documented heredoc syntax for multi-line JSON input
- Added comprehensive TODO with GitHub Actions workflow concepts

### Development
- Build version injection via ldflags (version, git commit, build date)
- Go module dependencies updated for GoReleaser v2 compatibility

### Known Issues
- Personal Access Token authentication not yet implemented (documented in TODO.md)
- Session tokens expire even with regular use (no refresh endpoint in Izanami)

## Project Information

**Repository**: https://github.com/webskin/izanami-go-cli

**License**: MIT (if applicable - update as needed)

**Status**: Work in Progress - Not recommended for production use

---

⚠️ **Note**: This is an initial release and the CLI is under active development. Features may be incomplete, APIs may change, and bugs are expected. Use at your own risk in development/testing environments only.
