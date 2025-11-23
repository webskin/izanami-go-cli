# Developer Guide for Claude Code

This document contains important information for Claude Code (or other developers) working on this project.

## Building the Project

**IMPORTANT**: Always use `make` to build the project, NOT `go build` directly.

### Why?

The Makefile injects version information (git commit, build date) into the binary using `-ldflags`. If you use `go build` directly, the version information will show as "unknown".

### Build Commands

```bash
# Build for current platform (with version info)
make build

# Build for all platforms (Linux, macOS, Windows)
make build-all

# Install to $GOPATH/bin
make install

# Run tests
make test

# Clean build artifacts
make clean

# Show all available targets
make help
```

### Version Information

The version info is injected at build time:
- **Version**: Set via `VERSION=x.y.z make build` (defaults to "dev")
- **GitCommit**: Automatically extracted from `git rev-parse --short HEAD`
- **BuildDate**: Automatically set to current UTC timestamp

Example:
```bash
VERSION=1.0.0 make build
./build/iz version
# Output:
# iz version 1.0.0
#   Commit:    255294d
#   Built:     2025-11-14T08:14:16Z
#   Go:        go1.25.4
#   Platform:  linux/amd64
```

## Project Status

⚠️ This project is **WORK IN PROGRESS** and not production-ready.

## Known TODOs

See [TODO.md](TODO.md) for pending tasks and features.

## Izanami swagger
Izanami server endpoints description can be found here https://maif.github.io/izanami/swagger/swagger.json
- Follow Cobra best practices by using cmd.OutOrStdout() instead of direct os.Stdout writes, which allows for proper testing while maintaining backward compatibility with normal CLI usage.