# Dependencies

This document lists the Go libraries used in the Izanami CLI and their purposes.

## Direct Dependencies

### CLI Framework

| Library | Version | Purpose |
|---------|---------|---------|
| [github.com/spf13/cobra](https://github.com/spf13/cobra) | v1.8.0 | Command-line interface framework |
| [github.com/spf13/viper](https://github.com/spf13/viper) | v1.18.2 | Configuration management |

**Cobra** is used throughout the `internal/cmd/` package to define all CLI commands,
subcommands, flags, and argument handling. It provides:
- Command hierarchy (`iz admin tenants list`, `iz features check`, etc.)
- Flag parsing (`--tenant`, `--project`, `--verbose`, etc.)
- Help text generation
- Shell completion (`iz completion bash/zsh/fish/powershell`)

```go
// Example: internal/cmd/features.go
var featuresCmd = &cobra.Command{
    Use:   "features",
    Short: "Manage feature flags",
    RunE:  func(cmd *cobra.Command, args []string) error { ... },
}
```

**Viper** is used in `internal/izanami/config.go` for:
- Loading configuration from YAML files (`~/.config/iz/config.yaml`)
- Environment variable binding (`IZ_BASE_URL`, `IZ_TENANT`, etc.)
- Configuration merging (flags > env vars > config file)

---

### HTTP Client

| Library | Version | Purpose |
|---------|---------|---------|
| [github.com/go-resty/resty/v2](https://github.com/go-resty/resty) | v2.11.0 | HTTP client for API calls |

**Resty** is used in `internal/izanami/client.go` for all HTTP communication with
the Izanami server. It provides:
- Fluent API for building requests
- Automatic JSON marshaling/unmarshaling
- Request/response middleware
- Retry mechanisms
- Debug logging

```go
// Example: internal/izanami/client.go
resp, err := c.client.R().
    SetHeader("Content-Type", "application/json").
    SetBody(payload).
    SetResult(&result).
    Post("/api/admin/tenants")
```

Used in:
- `internal/izanami/client.go` - Main API client
- `internal/izanami/features_check.go` - Feature flag checking
- `internal/izanami/events.go` - Server-Sent Events (SSE) handling

---

### Output Formatting

| Library | Version | Purpose |
|---------|---------|---------|
| [github.com/olekukonko/tablewriter](https://github.com/olekukonko/tablewriter) | v0.0.5 | ASCII table formatting |
| [github.com/fatih/color](https://github.com/fatih/color) | v1.18.0 | Colored terminal output |

**Tablewriter** is used in `internal/output/formatter.go` to display data in
formatted ASCII tables:

```
+--------------------------------------+------------+---------+
|                  ID                  |    NAME    | ENABLED |
+--------------------------------------+------------+---------+
| 550e8400-e29b-41d4-a716-446655440000 | my-feature | true    |
| 6ba7b810-9dad-11d1-80b4-00c04fd430c8 | dark-mode  | false   |
+--------------------------------------+------------+---------+
```

**Color** is used in `internal/izanami/types.go` for colored output:
- Green for enabled/success states
- Red for disabled/error states
- Yellow for warnings

---

### Terminal Handling

| Library | Version | Purpose |
|---------|---------|---------|
| [golang.org/x/term](https://pkg.go.dev/golang.org/x/term) | v0.37.0 | Terminal capabilities |

**Term** is used for:
- Secure password input (hiding typed characters) in `internal/cmd/login.go`
- TTY detection for spinner animation in `internal/auth/spinner.go`

```go
// Secure password reading
passwordBytes, err := term.ReadPassword(int(syscall.Stdin))

// TTY detection for spinner
if term.IsTerminal(int(fd)) {
    // Show animated spinner
} else {
    // Show static message
}
```

---

### Data Serialization

| Library | Version | Purpose |
|---------|---------|---------|
| [gopkg.in/yaml.v3](https://github.com/go-yaml/yaml) | v3.0.1 | YAML parsing/writing |

**YAML** is used in `internal/izanami/sessions.go` for:
- Reading/writing session files (`~/.izsessions`)
- Configuration file parsing

```go
// Example: internal/izanami/sessions.go
data, err := yaml.Marshal(sessions)
err = os.WriteFile(path, data, 0600)
```

---

### Testing

| Library | Version | Purpose |
|---------|---------|---------|
| [github.com/stretchr/testify](https://github.com/stretchr/testify) | v1.8.4 | Testing assertions and mocks |

**Testify** is used throughout `*_test.go` files for:
- `assert` - Non-fatal assertions
- `require` - Fatal assertions (stop test on failure)

```go
// Example: internal/izanami/client_test.go
assert.NoError(t, err)
assert.Equal(t, expected, actual)
require.NotNil(t, result)
```

---

## Indirect Dependencies

These are transitive dependencies pulled in by the direct dependencies:

| Library | Pulled By | Purpose |
|---------|-----------|---------|
| `github.com/inconshreveable/mousetrap` | cobra | Windows console detection |
| `github.com/spf13/pflag` | cobra | POSIX flag parsing |
| `github.com/spf13/afero` | viper | Filesystem abstraction |
| `github.com/spf13/cast` | viper | Type casting |
| `github.com/fsnotify/fsnotify` | viper | File change notifications |
| `github.com/hashicorp/hcl` | viper | HCL config format support |
| `github.com/pelletier/go-toml/v2` | viper | TOML config format support |
| `github.com/subosito/gotenv` | viper | .env file loading |
| `github.com/mitchellh/mapstructure` | viper | Map to struct conversion |
| `github.com/magiconair/properties` | viper | Properties file support |
| `github.com/mattn/go-runewidth` | tablewriter | Unicode width calculation |
| `github.com/mattn/go-colorable` | color | Windows color support |
| `github.com/mattn/go-isatty` | color | TTY detection |
| `github.com/davecgh/go-spew` | testify | Deep pretty printer |
| `github.com/pmezard/go-difflib` | testify | Diff generation |
| `golang.org/x/net` | resty | Network utilities |
| `golang.org/x/sys` | term | System calls |
| `golang.org/x/text` | various | Unicode text handling |

---

## Standard Library Usage

The CLI also makes extensive use of Go's standard library:

| Package | Purpose |
|---------|---------|
| `context` | Request cancellation and timeouts |
| `crypto/rand` | Cryptographically secure random generation (OIDC state) |
| `encoding/base64` | Base64 encoding for state and JWT parsing |
| `encoding/json` | JSON marshaling/unmarshaling |
| `fmt` | Formatted I/O |
| `io` | I/O primitives |
| `net/http` | HTTP client (OIDC flow, browser opening) |
| `net/url` | URL parsing |
| `os` | File system operations |
| `os/exec` | External process execution (browser opening) |
| `path/filepath` | Cross-platform path handling |
| `runtime` | Platform detection (GOOS) |
| `strings` | String manipulation |
| `sync` | Synchronization primitives (mutex for spinner) |
| `time` | Time handling and durations |

---

## Updating Dependencies

```bash
# Update all dependencies
go get -u ./...

# Update specific dependency
go get -u github.com/spf13/cobra@latest

# Tidy go.mod (remove unused)
go mod tidy

# Verify dependencies
go mod verify
```
