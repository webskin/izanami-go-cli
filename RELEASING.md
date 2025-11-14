# Release Process

This document describes how to create a new release of the Izanami Go CLI.

## Prerequisites

Install GoReleaser locally (optional, for testing):

```bash
go install github.com/goreleaser/goreleaser@latest
```

## Release Workflow

### 1. Test the Release Locally (Optional)

Before creating a real release, test that GoReleaser works correctly:

```bash
# Full release simulation (all platforms)
make release-test

# Check the generated binaries
./dist/iz_linux_amd64_v1/iz version
./dist/iz_darwin_amd64_v1/iz version
./dist/iz_windows_amd64_v1/iz.exe version

# Clean up
make release-clean
```

### 2. Prepare the Release

1. Ensure all changes are committed and pushed to `main`
2. Ensure tests pass: `make test`
3. Update version-related documentation if needed

### 3. Create and Push the Tag

```bash
# Create an annotated tag
git tag -a v1.0.0 -m "Release v1.0.0"

# Push the tag to GitHub
git push origin v1.0.0
```

### 4. GitHub Actions Takes Over

Once you push the tag:

1. GitHub Actions workflow (`.github/workflows/release.yml`) is triggered
2. GoReleaser builds binaries for all platforms:
   - Linux (amd64, arm64)
   - macOS (amd64/Intel, arm64/Apple Silicon)
   - Windows (amd64)
3. Creates archives (.tar.gz for Unix, .zip for Windows)
4. Generates checksums
5. Creates a GitHub Release with:
   - Release notes (auto-generated from commits)
   - All binaries attached
   - Installation instructions

### 5. Verify the Release

1. Go to: https://github.com/webskin/izanami-go-cli/releases
2. Check that the release appears
3. Download and test a binary
4. Verify checksums if needed

## Version Numbering

Follow [Semantic Versioning](https://semver.org/):

- `v1.0.0` - Major release (breaking changes)
- `v1.1.0` - Minor release (new features, backwards compatible)
- `v1.1.1` - Patch release (bug fixes)

### Pre-releases

For pre-releases, use suffixes:

- `v1.0.0-alpha.1` - Alpha version
- `v1.0.0-beta.1` - Beta version
- `v1.0.0-rc.1` - Release candidate

## Changelog

GoReleaser automatically generates a changelog from git commits. For better changelogs:

### Commit Message Format

Use conventional commit messages:

```
feat: Add new feature
fix: Fix bug in authentication
perf: Improve API performance
refactor: Restructure client code
docs: Update README
test: Add tests for features
ci: Update GitHub Actions workflow
```

GoReleaser groups these automatically:
- `feat:` → Features
- `fix:` → Bug Fixes
- `perf:` → Performance Improvements
- `refactor:` → Refactorings

## Troubleshooting

### Release Failed

If GitHub Actions fails:

1. Check the workflow run: https://github.com/webskin/izanami-go-cli/actions
2. Read the error logs
3. Fix the issue
4. Delete the tag and recreate:

```bash
# Delete remote tag
git push --delete origin v1.0.0

# Delete local tag
git tag -d v1.0.0

# Fix the issue, commit

# Recreate and push tag
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

### Test Locally Before Release

If unsure, always test with `make release-test` first!

## Manual Release (Not Recommended)

If you need to release manually (e.g., GitHub Actions is down):

```bash
# Set GitHub token
export GITHUB_TOKEN="your-github-token"

# Run GoReleaser
goreleaser release --clean
```

This will create the same release as GitHub Actions would.

## After Release

1. Announce the release (if applicable)
2. Update documentation if needed
3. Monitor for issues
4. Plan next release

## Quick Reference

| Command | Purpose |
|---------|---------|
| `make release-test` | Test release build locally |
| `git tag v1.0.0` | Create release tag |
| `git push origin v1.0.0` | Trigger automated release |
| `make release-clean` | Clean GoReleaser artifacts |

## CI/CD Overview

Two separate workflows:

### CI Workflow (`.github/workflows/ci.yml`)
- Triggers: Every push to `main`, every PR
- Purpose: Validate code builds and tests pass
- Does NOT create releases

### Release Workflow (`.github/workflows/release.yml`)
- Triggers: Only when pushing a tag (`v*`)
- Purpose: Build and publish releases
- Creates GitHub Release with binaries
