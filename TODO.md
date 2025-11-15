# TODO

## Feature Management

### Add Confirmation Prompts
- Add confirmation when asking for feature enabling/disabling
- Prevents accidental changes to critical features
- Should prompt "Are you sure you want to enable/disable feature X? [y/N]"

---

## Personal Access Token (PAT) Authentication

### Current Issue
- PATs are currently sent as JWT cookies, which causes the server to try parsing them as JWTs
- This results in error: "Last unit does not have enough valid bits"

### Solution
PATs should be sent via HTTP Basic Authentication instead of cookies.

**PAT Format**: `{uuid}_{secret}` (contains underscore)
**Session JWT Format**: Long base64 string (no underscore)

### Implementation Required

In `internal/izanami/client.go`, detect PAT format and use appropriate auth:

```go
if config.Username != "" && config.Token != "" {
    // Check if token is a Personal Access Token (PAT) or Session JWT
    // PAT format: {uuid}_{secret} (contains underscore)
    // Session JWT: long base64 string (no underscore)
    if strings.Contains(config.Token, "_") {
        // Personal Access Token - use HTTP Basic Auth
        // Format: Authorization: Basic base64(username:token)
        client.SetBasicAuth(config.Username, config.Token)
    } else {
        // Session JWT - use cookie authentication
        cookie := &http.Cookie{
            Name:  "token",
            Value: config.Token,
            Path:  "/",
        }
        client.SetCookie(cookie)
    }
}
```

### Server-side Validation
From `PersonnalAccessTokenTenantAuthAction.scala`:
1. Extracts `Authorization` header
2. Splits on `"Basic "` to get base64-encoded part
3. Base64 decodes to get `username:token`
4. Validates token against database

### Testing
Once implemented, test with:
```bash
export IZ_USERNAME=RESERVED_ADMIN_USER
export IZ_TOKEN=6374d239-ce82-45fa-b3d4-9b6bb349ea75_cyUBBnNsdDYVRZWcyyjRvqOTwuE0YKckuhEacCbtJfu78uGVy26CRhuRKN0uokKF
export IZ_BASE_URL=http://localhost:9000

./build/iz admin tenants list --verbose
```

Should see:
```
Authorization: Basic base64(username:token)
```

### Notes
- Session JWTs should continue using cookie authentication (current behavior)
- PATs require specific tenant permissions in the database
- PAT expiration is checked server-side

---

## GitHub Actions Setup

### Git Workflow Concepts

#### Branches

**main** (or master)
- Your primary branch
- Should always be stable/working
- Protected branch (optional: require PR reviews)

**feature branches**
- `feature/add-api-keys`
- `fix/auth-bug`
- `refactor/client-code`
- Created for each new feature/fix
- Merged into `main` via Pull Requests

**Optional: develop branch**
- Integration branch for active development
- `main` only gets stable releases
- More complex, not needed for small projects

#### Common Workflows

**Simple Flow (Good for this project)**
```
main (stable)
  ↑
  │ PR merge
  │
feature/my-feature (work here)
```

**GitFlow (More complex)**
```
main (releases only)
  ↑
  │ PR for releases
  │
develop (integration)
  ↑
  │ PR merge
  │
feature/my-feature
```

---

### How "Build projects with Make" Works

#### Workflow Triggers

The Make workflow typically runs on:

```yaml
on:
  push:
    branches: [ main, develop ]  # Build when pushing to these branches
  pull_request:
    branches: [ main ]            # Build when PR is opened/updated
```

#### What Happens

1. **On every commit to `main`**:
   - Workflow runs automatically
   - Checks out code
   - Runs `make build` (or `make test`)
   - **No release created** - just validates the build

2. **On Pull Request**:
   - Workflow runs for the PR branch
   - Shows if the PR breaks the build
   - Green checkmark = safe to merge
   - Red X = build failed, don't merge

3. **Build artifacts**:
   - Usually **not saved** for regular commits
   - Just validates that code compiles
   - Keeps history clean

---

### When Releases Are Built

#### Releases vs Builds

**Regular builds** (CI):
- Happen on every commit/PR
- Verify code compiles and tests pass
- **No binaries distributed**
- Version stays "dev"

**Releases**:
- Triggered by **Git tags** (v1.0.0, v1.1.0, etc.)
- Build with proper version number
- Create GitHub Release
- Attach binaries for download
- **Users download these**

#### Release Workflow

```bash
# You're ready to release
git tag v1.0.0
git push origin v1.0.0

# GitHub Actions detects the tag
# Triggers release workflow
# Builds binaries for all platforms
# Creates GitHub Release with binaries attached
```

---

### Typical Setup for This Project

#### Workflow 1: CI (Build with Make)

**File**: `.github/workflows/ci.yml`

**Triggers**: Every push to `main`, every PR

**Purpose**: Validate code quality

```yaml
name: CI

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Build
        run: make build

      - name: Test
        run: make test

      - name: Lint (optional)
        run: make lint
```

**Result**:
- ✅ or ❌ status on commits/PRs
- No releases created
- No binaries saved

---

#### Workflow 2: Release (Optional, for later)

**File**: `.github/workflows/release.yml`

**Triggers**: Only when you push a tag like `v1.0.0`

**Purpose**: Build and distribute binaries

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'  # Triggers on v1.0.0, v1.2.3, etc.

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Build all platforms
        run: VERSION=${{ github.ref_name }} make build-all

      - name: Create GitHub Release
        uses: softprogs/action-gh-release@v1
        with:
          files: build/*
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**Result**:
- Creates GitHub Release page
- Attaches binaries (Linux, macOS, Windows)
- Users can download from releases page

---

### Example Timeline

#### Development Phase (Now)

```
Day 1: Add feature X
  ├─ Create branch: feature/add-x
  ├─ Commit changes
  ├─ Push to GitHub
  ├─ Open PR to main
  └─ CI workflow runs ✅ (build succeeds)

Day 2: Merge PR
  ├─ PR merged to main
  ├─ CI workflow runs on main ✅
  └─ No release created (just validation)

Day 3: Fix bug Y
  ├─ Create branch: fix/bug-y
  ├─ Commit changes
  ├─ Push to GitHub
  ├─ Open PR to main
  ├─ CI workflow runs ✅
  └─ Merge to main
```

**Status**:
- `main` branch has all features/fixes
- Version still shows "dev"
- No public releases yet

---

#### Release Phase (Later)

```
Ready to release v1.0.0:
  ├─ git tag v1.0.0
  ├─ git push origin v1.0.0
  ├─ Release workflow triggers
  ├─ Builds binaries with VERSION=1.0.0
  ├─ Creates GitHub Release
  └─ Binaries available for download

User downloads:
  └─ Visits https://github.com/webskin/izanami-go-cli/releases
      └─ Downloads iz-linux-amd64
          └─ Runs: ./iz version
              # Output: iz version 1.0.0
```

---

### Practical Recommendations

#### Phase 1: Now (WIP)

**Branches:**
- Work directly on `main` for now (simple)
- Or use feature branches if you want practice

**Workflow:**
- Set up "Build with Make" CI workflow
- It validates every commit builds successfully
- No releases yet

#### Phase 2: Pre-Release

**Branches:**
- Use `main` for stable code
- Create feature branches for new work
- Require PRs (even if just you)

**Workflow:**
- CI still running on every commit
- Maybe add a `develop` branch for unstable features

#### Phase 3: Production Ready

**Remove WIP warning:**
- Update README
- Create first release tag `v1.0.0`

**Workflow:**
- Add release workflow (or SLSA)
- CI continues validating builds
- Release workflow creates binaries

---

### Summary

| Event | What Happens | Release Created? |
|-------|--------------|------------------|
| Push to `main` | CI runs, validates build | ❌ No |
| Open PR | CI runs, shows status | ❌ No |
| Merge PR | CI runs on `main` | ❌ No |
| Push tag `v1.0.0` | Release workflow runs | ✅ Yes |

**Key Concept**:
- **CI = Continuous validation** (build on every change)
- **Release = Distribution** (create downloadable binaries)
