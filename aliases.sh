#!/bin/bash
# Izanami Go CLI - Development Aliases
# Source this file in your shell: source aliases.sh

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Loading Izanami CLI development aliases...${NC}"

# Project directory
export IZ_PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ============================================================================
# Building & Running
# ============================================================================

# Quick build for current platform
alias izb='make build'
alias izbuild='make build'

# Build and run
alias izr='make build && ./build/iz'
alias izrun='make build && ./build/iz'

# Build all platforms
alias izba='make build-all'
alias izbuild-all='make build-all'

# Install to system
alias izi='make install'
alias izinstall='make install'

# Run the built binary
alias iz='./build/iz'

# ============================================================================
# Testing
# ============================================================================

# Run tests
alias izt='make test'
alias iztest='make test'

alias izit='make integration-test'

# Run tests with coverage
alias iztc='make test-coverage'
alias iztest-coverage='make test-coverage'

# Run tests and open coverage report
alias iztco='make test-coverage && open coverage.html'

# ============================================================================
# Code Quality
# ============================================================================

# Format code
alias izf='make fmt'
alias izfmt='make fmt'

# Lint code
alias izl='make lint'
alias izlint='make lint'

# Tidy dependencies
alias iztidy='make tidy'

# Format, lint, and test (full check)
alias izcheck='make fmt && make lint && make test'

# ============================================================================
# GoReleaser
# ============================================================================

# Test release build locally
alias izrt='make release-test'
alias izrelease-test='make release-test'

# Build with GoReleaser (single platform)
alias izrb='make release-build'
alias izrelease-build='make release-build'

# Clean GoReleaser artifacts
alias izrc='make release-clean'
alias izrelease-clean='make release-clean'

# ============================================================================
# Git Operations
# ============================================================================

# Quick commit and push
alias izgc='git add . && git commit'
alias izgcp='git add . && git commit && git push'

# Git status
alias izgs='git status'

# Git log (pretty)
alias izgl='git log --oneline --graph --decorate -10'
alias izglog='git log --oneline --graph --decorate --all -20'

# Show recent commits
alias izgr='git log --oneline -5'

# Create and push tag (usage: iztag v1.0.0)
iztag() {
    local tag_version="$1"
    if [[ -z "$tag_version" ]]; then
        echo "Usage: iztag v1.0.0"
        return 1
    fi
    git tag -a "$tag_version" -m "Release $tag_version"
    git push origin "$tag_version"
    echo -e "${GREEN}✅ Tag $tag_version created and pushed!${NC}"
    echo "Check release at: https://github.com/webskin/izanami-go-cli/actions"
    return 0
}

# Delete local and remote tag (usage: izuntag v1.0.0)
izuntag() {
    local tag_version="$1"
    if [[ -z "$tag_version" ]]; then
        echo "Usage: izuntag v1.0.0"
        return 1
    fi
    git push --delete origin "$tag_version" 2>/dev/null
    git tag -d "$tag_version" 2>/dev/null
    echo -e "${GREEN}✅ Tag $tag_version deleted${NC}"
    return 0
}

# ============================================================================
# Cleanup
# ============================================================================

# Clean build artifacts
alias izclean='make clean'

# Clean everything (build + GoReleaser)
alias izclean-all='make clean && make release-clean'

# Clean Go cache
alias izclean-cache='go clean -cache -modcache'

# ============================================================================
# Development Workflow
# ============================================================================

# Full development cycle: format, build, test
alias izdev='make fmt && make build && make test'

# Pre-commit check: format, lint, test
alias izpre='make fmt && make lint && make test'

# Pre-release check: format, lint, test, release-test
alias izpre-release='make fmt && make lint && make test && make release-test'

# ============================================================================
# Version & Info
# ============================================================================

# Show version
alias izv='./build/iz version'
alias izversion='./build/iz version'

# Show help
alias izh='make help'
alias izhelp='make help'

# ============================================================================
# Izanami Server Shortcuts (if using local server)
# ============================================================================

# Set common environment variables
alias izenv-local='export IZ_BASE_URL=http://localhost:9000 IZ_TENANT=test-tenant IZ_PROJECT=test-project'
alias izenv-prod='echo "Set IZ_BASE_URL, IZ_TENANT, IZ_PROJECT, IZ_USERNAME, IZ_TOKEN manually"'

# Show current environment
izenv() {
    echo -e "${BLUE}Current Izanami Environment:${NC}"
    echo "IZ_BASE_URL:      ${IZ_BASE_URL:-not set}"
    echo "IZ_TENANT:        ${IZ_TENANT:-not set}"
    echo "IZ_PROJECT:       ${IZ_PROJECT:-not set}"
    echo "IZ_USERNAME:      ${IZ_USERNAME:-not set}"
    echo "IZ_TOKEN:         ${IZ_TOKEN:+***set***}"
    echo "IZ_CLIENT_ID:     ${IZ_CLIENT_ID:-not set}"
    echo "IZ_CLIENT_SECRET: ${IZ_CLIENT_SECRET:+***set***}"
    return 0
}

# ============================================================================
# Navigation
# ============================================================================

# Go to project root
alias cdiz='cd $IZ_PROJECT_DIR'

# Go to internal packages
alias cdizi='cd $IZ_PROJECT_DIR/internal'
alias cdizcmd='cd $IZ_PROJECT_DIR/internal/cmd'
alias cdiziz='cd $IZ_PROJECT_DIR/internal/izanami'

# ============================================================================
# Useful Functions
# ============================================================================

# Build and test a specific feature (usage: iztest-feature features list)
iztest-feature() {
    make build && ./build/iz "$@"
    return 0
}

# Watch for changes and rebuild (requires entr or inotifywait)
izwatch() {
    if command -v entr &> /dev/null; then
        echo "Watching for changes (Ctrl+C to stop)..."
        find . -name '*.go' | entr -c make build
    else
        echo "Install 'entr' for file watching: sudo apt install entr"
    fi
    return 0
}

# Quick feature test against local server
iztest-local() {
    export IZ_BASE_URL=http://localhost:9000
    export IZ_TENANT=test-tenant
    export IZ_PROJECT=test-project
    make build && ./build/iz "$@"
    return 0
}

# Show all available aliases
izaliases() {
    echo -e "${BLUE}Izanami CLI Development Aliases:${NC}\n"

    echo -e "${GREEN}Building & Running:${NC}"
    echo "  izb, izbuild           - Quick build (make build)"
    echo "  izr, izrun             - Build and run"
    echo "  izba, izbuild-all      - Build all platforms"
    echo "  izi, izinstall         - Install to system"
    echo "  iz                     - Run built binary"

    echo -e "\n${GREEN}Testing:${NC}"
    echo "  izt, iztest            - Run tests"
    echo "  izit,                  - Run integration tests"
    echo "  iztc, iztest-coverage  - Run tests with coverage"
    echo "  iztco                  - Run tests and open coverage"

    echo -e "\n${GREEN}Code Quality:${NC}"
    echo "  izf, izfmt             - Format code"
    echo "  izl, izlint            - Lint code"
    echo "  iztidy                 - Tidy dependencies"
    echo "  izcheck                - Format, lint, and test"

    echo -e "\n${GREEN}GoReleaser:${NC}"
    echo "  izrt, izrelease-test   - Test release build"
    echo "  izrb, izrelease-build  - Build with GoReleaser"
    echo "  izrc, izrelease-clean  - Clean GoReleaser artifacts"

    echo -e "\n${GREEN}Git Operations:${NC}"
    echo "  izgc                   - Git add and commit"
    echo "  izgcp                  - Git add, commit, and push"
    echo "  izgs                   - Git status"
    echo "  izgl, izglog           - Git log (pretty)"
    echo "  iztag <version>        - Create and push tag"
    echo "  izuntag <version>      - Delete local and remote tag"

    echo -e "\n${GREEN}Cleanup:${NC}"
    echo "  izclean                - Clean build artifacts"
    echo "  izclean-all            - Clean everything"
    echo "  izclean-cache          - Clean Go cache"

    echo -e "\n${GREEN}Workflows:${NC}"
    echo "  izdev                  - Format, build, test"
    echo "  izpre                  - Pre-commit check"
    echo "  izpre-release          - Pre-release check"

    echo -e "\n${GREEN}Info:${NC}"
    echo "  izv, izversion         - Show version"
    echo "  izh, izhelp            - Show make help"
    echo "  izenv                  - Show environment variables"
    echo "  izaliases              - Show this help"

    echo -e "\n${GREEN}Navigation:${NC}"
    echo "  cdiz                   - Go to project root"
    echo "  cdizi                  - Go to internal/"
    echo "  cdizcmd                - Go to internal/cmd/"
    echo "  cdiziz                 - Go to internal/izanami/"

    echo -e "\n${GREEN}Functions:${NC}"
    echo "  iztest-feature <args>  - Build and test feature"
    echo "  izwatch                - Watch for changes and rebuild"
    echo "  iztest-local <args>    - Test against local server"
    echo "  izenv-local            - Set local env variables"

    echo -e "\n${BLUE}Tip: Source this file in your ~/.bashrc or ~/.zshrc${NC}"
    echo "  echo 'source $IZ_PROJECT_DIR/aliases.sh' >> ~/.bashrc"
    return 0
}

echo -e "${GREEN}✅ Aliases loaded! Type 'izaliases' to see all available commands.${NC}"
