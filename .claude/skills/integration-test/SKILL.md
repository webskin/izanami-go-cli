---
name: integration-test
description: Generate integration tests for a Go file following Izanami patterns
arguments:
  - name: $target_file
    description: The Go source file to create integration tests for
    required: true
---

# Integration Test Generator for Izanami CLI

You are generating integration tests for the Izanami Go CLI following established patterns.

## Target File
- Source file: {{target_file}}
- Test file: `{{target_file | replace: ".go", "_integration_test.go"}}`

## Test Infrastructure (What Exists)

### Base Helper: `integration_test_helpers_test.go`

```go
// IntegrationTestEnv holds the test environment configuration
type IntegrationTestEnv struct {
    TempDir      string  // Isolated temp directory for test
    ConfigDir    string  // Path to .config/iz/ in temp
    ConfigPath   string  // Path to config.yaml
    SessionsPath string  // Path to .izsessions
    BaseURL      string  // From IZ_TEST_BASE_URL
    Username     string  // From IZ_TEST_USERNAME
    Password     string  // From IZ_TEST_PASSWORD
    ClientID     string  // From IZ_TEST_CLIENT_ID
    ClientSecret string  // From IZ_TEST_CLIENT_SECRET
}
```

### Available Helper Methods

```go
// Creates isolated test environment, skips if IZ_TEST_BASE_URL not set
env := setupIntegrationTest(t)

// Performs login, returns JWT token
token := env.Login(t)

// Returns authenticated *izanami.Client for API calls
client := env.NewAuthenticatedClient(t)

// Gets stored JWT token from session
token := env.GetJwtToken(t)

// File existence checks
env.SessionsFileExists() bool
env.ConfigFileExists() bool

// Read file contents
content := env.ReadSessionsFile(t)
content := env.ReadConfigFile(t)
```

### Environment Variables
- `IZ_TEST_BASE_URL` - Izanami server URL (required)
- `IZ_TEST_USERNAME` - Test user username
- `IZ_TEST_PASSWORD` - Test user password
- `IZ_TEST_CLIENT_ID` - Client credentials (optional)
- `IZ_TEST_CLIENT_SECRET` - Client credentials (optional)

### Local Development Setup

For immediate integration testing with a local Izanami instance:

```bash
export IZ_TEST_BASE_URL=http://localhost:9000
export IZ_TEST_USERNAME=RESERVED_ADMIN_USER
export IZ_TEST_PASSWORD=password
```

Or run tests inline:

```bash
IZ_TEST_BASE_URL=http://localhost:9000 IZ_TEST_USERNAME=RESERVED_ADMIN_USER IZ_TEST_PASSWORD=password go test -v ./internal/cmd/ -run "TestIntegration_"
```

Or use `make integration-test` which uses these defaults.

## Command Execution Pattern

We execute CLI commands using Cobra directly. **CRITICAL**: You must set global variables directly rather than using command-line flags, and set `SetOut`/`SetErr` on ALL subcommands in the hierarchy.

### Why Global Variables Instead of Flags

The CLI uses global package variables (e.g., `tenant`, `outputFormat`) that are populated via Cobra's flag binding. In tests, the `PersistentPreRunE` hook calls `MergeWithFlags()` which reads these global variables. Using `--tenant` as a flag won't work properly in tests because:
1. Flag parsing happens differently in test context
2. The global variable remains empty unless explicitly set

### Why SetOut/SetErr on ALL Subcommands

Output capture only works if `SetOut`/`SetErr` is called on the **exact command that produces output**. Parent commands don't propagate these settings to children.

```go
func TestIntegration_SomeCommand(t *testing.T) {
    env := setupIntegrationTest(t)
    env.Login(t)

    // Get JWT token and configure global state
    token := env.GetJwtToken(t)

    // Save and set global variables (NOT flags!)
    origCfg := cfg
    origTenant := tenant
    origOutputFormat := outputFormat

    cfg = &izanami.Config{
        BaseURL:  env.BaseURL,
        Username: env.Username,
        JwtToken: token,
        Timeout:  30,
    }
    tenant = "test-tenant"      // Set global variable directly
    outputFormat = "json"       // Set global variable directly

    defer func() {
        cfg = origCfg
        tenant = origTenant
        outputFormat = origOutputFormat
    }()

    var buf bytes.Buffer
    cmd := &cobra.Command{Use: "iz"}
    cmd.AddCommand(parentCmd)

    // CRITICAL: Set Out/Err on ALL commands in the hierarchy
    cmd.SetOut(&buf)
    cmd.SetErr(&buf)
    parentCmd.SetOut(&buf)
    parentCmd.SetErr(&buf)
    targetCmd.SetOut(&buf)      // The actual command being tested
    targetCmd.SetErr(&buf)

    cmd.SetArgs([]string{"parent", "subcommand", "arg"})
    err := cmd.Execute()

    // IMPORTANT: Reset command state after execution
    parentCmd.SetOut(nil)
    parentCmd.SetErr(nil)
    targetCmd.SetOut(nil)
    targetCmd.SetErr(nil)

    // Assertions
    require.NoError(t, err)
    output := buf.String()
    assert.Contains(t, output, "expected text")
}
```

## Standard Test Patterns

### Pattern 1: Reusable Setup Function (Recommended)

Create a setup function that handles global state management. This avoids repetition and ensures proper cleanup:

```go
// setupMyCommandTest sets up the test environment for myCmd tests
func setupMyCommandTest(t *testing.T, env *IntegrationTestEnv) func() {
    t.Helper()
    env.Login(t)
    token := env.GetJwtToken(t)

    // Save original values (include ALL global flag variables!)
    origCfg := cfg
    origOutputFormat := outputFormat
    origTenant := tenant
    origProject := project  // Don't forget project!

    // Set up config
    cfg = &izanami.Config{
        BaseURL:  env.BaseURL,
        Username: env.Username,
        JwtToken: token,
        Timeout:  30,
    }
    outputFormat = "table"
    tenant = ""  // Will be set per-test
    project = "" // Reset to avoid pollution from other tests

    // Reset command-specific flags to defaults
    myCommandFlag = ""
    myOtherFlag = false

    return func() {
        cfg = origCfg
        outputFormat = origOutputFormat
        tenant = origTenant
        project = origProject
        myCommandFlag = ""
        myOtherFlag = false
    }
}

// executeMyCommand executes my command with proper output capture
func executeMyCommand(t *testing.T, args []string) (string, error) {
    t.Helper()

    var buf bytes.Buffer
    cmd := &cobra.Command{Use: "iz"}
    // Add --project flag if your command uses it (binds to global 'project' variable)
    // This allows tests to pass --project as an argument
    cmd.PersistentFlags().StringVar(&project, "project", "", "Default project")
    cmd.AddCommand(parentCmd)

    // Set Out/Err on ALL commands in hierarchy
    cmd.SetOut(&buf)
    cmd.SetErr(&buf)
    parentCmd.SetOut(&buf)
    parentCmd.SetErr(&buf)
    myCmd.SetOut(&buf)
    myCmd.SetErr(&buf)

    fullArgs := append([]string{"parent", "mycommand"}, args...)
    cmd.SetArgs(fullArgs)
    err := cmd.Execute()

    // Reset all commands
    parentCmd.SetOut(nil)
    parentCmd.SetErr(nil)
    myCmd.SetOut(nil)
    myCmd.SetErr(nil)

    return buf.String(), err
}
```

### Pattern 2: Command that requires login and tenant
```go
func TestIntegration_CommandAfterLogin(t *testing.T) {
    env := setupIntegrationTest(t)
    cleanup := setupMyCommandTest(t, env)
    defer cleanup()

    // Set tenant via global variable (NOT --tenant flag)
    tenant = "test-tenant"

    output, err := executeMyCommand(t, []string{"list"})
    require.NoError(t, err)
    assert.Contains(t, output, "expected")

    t.Logf("Command output:\n%s", output)
}
```

### Pattern 3: Command that should fail without login
```go
func TestIntegration_CommandWithoutLogin(t *testing.T) {
    _ = setupIntegrationTest(t)  // No login, no setup

    // The command should fail because there's no authenticated session
    output, err := executeMyCommand(t, []string{"list"})

    require.Error(t, err)
    assert.Contains(t, err.Error(), "no active profile")
}
```

### Pattern 4: Verify state via API after command
```go
func TestIntegration_CommandVerifyState(t *testing.T) {
    env := setupIntegrationTest(t)
    cleanup := setupMyCommandTest(t, env)
    defer cleanup()

    tenant = "test-tenant"

    // Execute command
    output, err := executeMyCommand(t, []string{"create", "my-item"})
    require.NoError(t, err)
    assert.Contains(t, output, "created successfully")

    // Verify via direct API call
    client := env.NewAuthenticatedClient(t)
    ctx := context.Background()
    result, err := client.GetItem(ctx, "test-tenant", "my-item")
    require.NoError(t, err)
    assert.Equal(t, "my-item", result.Name)
}
```

### Pattern 5: Testing izanami package functions directly
```go
func TestIntegration_PackageFunction(t *testing.T) {
    env := setupIntegrationTest(t)
    env.Login(t)

    // Call izanami package functions directly
    err := izanami.AddClientKeys("tenant", nil, "client-id", "secret")
    require.NoError(t, err)

    // Verify persistence
    profile, err := izanami.GetProfile("default")
    require.NoError(t, err)
    assert.NotNil(t, profile.ClientKeys)
}
```

### Pattern 6: Commands with interactive input (confirmations)
```go
func executeMyCommandWithInput(t *testing.T, args []string, input string) (string, error) {
    t.Helper()

    var buf bytes.Buffer
    inputBuf := bytes.NewBufferString(input)

    cmd := &cobra.Command{Use: "iz"}
    cmd.AddCommand(parentCmd)

    // Set Out/Err/In on ALL commands
    cmd.SetOut(&buf)
    cmd.SetErr(&buf)
    cmd.SetIn(inputBuf)
    parentCmd.SetOut(&buf)
    parentCmd.SetErr(&buf)
    parentCmd.SetIn(inputBuf)
    myCmd.SetOut(&buf)
    myCmd.SetErr(&buf)
    myCmd.SetIn(inputBuf)

    fullArgs := append([]string{"parent", "mycommand"}, args...)
    cmd.SetArgs(fullArgs)
    err := cmd.Execute()

    // Reset all
    parentCmd.SetIn(nil)
    parentCmd.SetOut(nil)
    parentCmd.SetErr(nil)
    myCmd.SetIn(nil)
    myCmd.SetOut(nil)
    myCmd.SetErr(nil)

    return buf.String(), err
}

func TestIntegration_DeleteWithConfirmation(t *testing.T) {
    env := setupIntegrationTest(t)
    cleanup := setupMyCommandTest(t, env)
    defer cleanup()

    tenant = "test-tenant"

    // User types "y" to confirm
    output, err := executeMyCommandWithInput(t, []string{"delete", "item-name"}, "y\n")
    require.NoError(t, err)
    assert.Contains(t, output, "deleted successfully")
}

func TestIntegration_DeleteCancelled(t *testing.T) {
    env := setupIntegrationTest(t)
    cleanup := setupMyCommandTest(t, env)
    defer cleanup()

    tenant = "test-tenant"

    // User types "n" to cancel
    output, err := executeMyCommandWithInput(t, []string{"delete", "item-name"}, "n\n")
    require.NoError(t, err)
    assert.Contains(t, output, "Cancelled")  // Note: capital C
}
```

### Pattern 7: Using TempResource helpers for cleanup

For commands that create resources, use TempResource helpers to ensure cleanup:

```go
func TestIntegration_CreateAndVerify(t *testing.T) {
    env := setupIntegrationTest(t)
    cleanup := setupMyCommandTest(t, env)
    defer cleanup()

    // Create temp tenant for isolation
    client := env.NewAuthenticatedClient(t)
    tempTenant := NewTempTenant(t, client).MustCreate(t)
    defer tempTenant.Delete(t)

    tenant = tempTenant.Name

    // Now test creating something in this tenant
    output, err := executeMyCommand(t, []string{"create", "my-item"})
    require.NoError(t, err)
    assert.Contains(t, output, "created successfully")
}
```

## Test Coverage Categories

For each command in `{{target_file}}`, create tests for:

1. **Happy Path** - Successful operation with valid input
2. **Error Cases** - Invalid input, missing auth, not found
3. **State Verification** - CLI output matches API state
4. **Edge Cases** - Empty results, special characters, etc.

## Assertions

```go
// Use require for critical checks (fail fast)
require.NoError(t, err, "Description of what failed")
require.NotNil(t, result, "Result should not be nil")

// Use assert for multiple checks
assert.Contains(t, output, "expected")
assert.Equal(t, expected, actual)
assert.True(t, condition, "Condition description")
```

## Naming Convention

```go
// Format: TestIntegration_<Command>_<Scenario>
func TestIntegration_LoginWithValidCredentials(t *testing.T)
func TestIntegration_LoginWithInvalidCredentials(t *testing.T)
func TestIntegration_ProfileListShowsSessionURL(t *testing.T)
func TestIntegration_ResetNoFilesError(t *testing.T)
```

## Analysis Steps

1. **Read** `{{target_file}}` to understand:
   - Commands and subcommands defined
   - Required flags and arguments
   - Expected outputs and error messages
   - State changes (files created, API calls made)

2. **Identify** test scenarios for each command:
   - What inputs are valid/invalid?
   - What authentication is required?
   - What state should be verified after?

3. **Generate** the test file:
   - Package: `cmd` (same as source)
   - Imports: `bytes`, `testing`, `context`, `github.com/spf13/cobra`, `github.com/stretchr/testify/assert`, `github.com/stretchr/testify/require`, `github.com/webskin/izanami-go-cli/internal/izanami`
   - Test functions following patterns above

## Example: Reference Existing Tests

Look at these files for patterns:
- `internal/cmd/login_logout_integration_test.go` - Login/logout flows
- `internal/cmd/profiles_integration_test.go` - Profile commands + API verification
- `internal/cmd/reset_integration_test.go` - Commands modifying local files
- `internal/cmd/health_integration_test.go` - Commands using global config
- `internal/cmd/contexts_integration_test.go` - **Best example**: Shows TempContext helper, setup/execute helpers, global variable management, nested command hierarchy (admin > contexts > subcommands)
- `internal/cmd/tenants_integration_test.go` - TempTenant helper pattern
- `internal/cmd/projects_integration_test.go` - TempProject helper pattern

## Important Notes

- Tests are skipped if `IZ_TEST_BASE_URL` not set (handled by `setupIntegrationTest`)
- Each test gets isolated temp directory (no cross-test pollution)
- Always reset command streams after execution (SetOut/SetErr/SetIn to nil)
- Use `t.Logf()` for helpful debug output
- Add `t.Helper()` in any helper functions you create

### Critical Gotchas (Learned the Hard Way)

1. **Variable Shadowing**: When creating temp resources, avoid naming local variables the same as global flag variables:
   ```go
   // BAD - shadows global 'tenant' variable
   tenant := NewTempTenant(t, client)

   // GOOD - use different name
   tempTenant := NewTempTenant(t, client)
   tenant = tempTenant.Name  // Set global variable explicitly
   ```

2. **Global Variables vs Flags - Know When to Use Each**:
   - For variables NOT bound to flags in your test command (e.g., `tenant`, `outputFormat`): set globals directly
   - For variables bound via `StringVar` in your test command (e.g., `project`): pass as flag argument (see gotcha #6)
   ```go
   // tenant and outputFormat: set globals (no flag binding in test cmd)
   tenant = "my-tenant"
   outputFormat = "json"

   // project: pass as flag (because test cmd has StringVar(&project, "project", ...))
   cmd.SetArgs([]string{"list", "--project", "my-project"})
   ```

3. **SetOut/SetErr Inheritance**: Cobra does NOT propagate SetOut/SetErr to child commands. You must set it on EVERY command that produces output:
   ```go
   // BAD - output won't be captured
   cmd.SetOut(&buf)
   parentCmd.SetOut(&buf)
   // Missing: childCmd.SetOut(&buf)

   // GOOD - set on all commands in hierarchy
   cmd.SetOut(&buf)
   parentCmd.SetOut(&buf)
   childCmd.SetOut(&buf)
   ```

4. **output.Print vs output.PrintTo**: Commands must use `output.PrintTo(cmd.OutOrStdout(), ...)` not `output.Print(...)` for output to be captured in tests. If tests show empty output, check the command implementation.

5. **Confirmation Prompts**: The confirmation helper uses "Cancelled" with capital C. Match exact case in assertions.

6. **Cobra flag binding resets variables during Execute()**: When you add a persistent flag with `StringVar(&project, "project", "", "")`, Cobra may reset the bound variable to its default during flag parsing in `Execute()`. If you set `project = "foo"` before calling `Execute()`, it may be overwritten to `""`. **Solution**: Pass the flag as a command-line argument instead:
   ```go
   // BAD - project may be reset to "" during Execute()
   project = tempProject.Name
   output, err := executeMyCommand(t, []string{"list"})

   // GOOD - pass flag as argument
   output, err := executeMyCommand(t, []string{"list", "--project", tempProject.Name})
   ```
   For this to work, the test command must define the flag:
   ```go
   cmd := &cobra.Command{Use: "iz"}
   cmd.PersistentFlags().StringVar(&project, "project", "", "Default project")
   cmd.AddCommand(adminCmd)
   ```

7. **Save/restore ALL global flag variables**: Setup functions must save and restore ALL global variables that could be modified, including `project`, not just `tenant`. Otherwise tests that use `--project` will pollute subsequent tests:
   ```go
   func setupMyTest(t *testing.T, env *IntegrationTestEnv) func() {
       origTenant := tenant
       origProject := project  // Don't forget this!
       // ...
       tenant = ""
       project = ""  // Reset to avoid pollution

       return func() {
           tenant = origTenant
           project = origProject  // Restore!
       }
   }
   ```

8. **`-tags=integration` build tag is required**: Integration test files use `//go:build integration`. Without `-tags=integration`, tests won't compile or run:
   ```bash
   # WRONG - tests appear to pass instantly (they're not running!)
   go test ./internal/cmd/... -run "TestIntegration"

   # CORRECT
   go test -tags=integration ./internal/cmd/... -run "TestIntegration"
   ```

Now analyze `{{target_file}}` and generate the integration tests.
