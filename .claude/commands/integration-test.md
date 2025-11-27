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

## Command Execution Pattern

We execute CLI commands using Cobra directly:

```go
func TestIntegration_SomeCommand(t *testing.T) {
    env := setupIntegrationTest(t)
    env.Login(t)

    var buf bytes.Buffer
    cmd := &cobra.Command{Use: "iz"}
    cmd.AddCommand(targetCmd)  // The command being tested
    cmd.SetOut(&buf)
    cmd.SetErr(&buf)
    targetCmd.SetOut(&buf)
    targetCmd.SetErr(&buf)

    cmd.SetArgs([]string{"subcommand", "--flag", "value"})
    err := cmd.Execute()

    // IMPORTANT: Reset command state after execution
    targetCmd.SetOut(nil)
    targetCmd.SetErr(nil)

    // Assertions
    require.NoError(t, err)
    output := buf.String()
    assert.Contains(t, output, "expected text")
}
```

## Standard Test Patterns

### Pattern 1: Command that requires login
```go
func TestIntegration_CommandAfterLogin(t *testing.T) {
    env := setupIntegrationTest(t)
    env.Login(t)

    var buf bytes.Buffer
    cmd := &cobra.Command{Use: "iz"}
    cmd.AddCommand(targetCmd)
    cmd.SetOut(&buf)
    cmd.SetErr(&buf)
    targetCmd.SetOut(&buf)
    targetCmd.SetErr(&buf)

    cmd.SetArgs([]string{"command", "arg"})
    err := cmd.Execute()

    targetCmd.SetOut(nil)
    targetCmd.SetErr(nil)

    require.NoError(t, err)
    output := buf.String()
    assert.Contains(t, output, "Success")

    t.Logf("Command output:\n%s", output)
}
```

### Pattern 2: Command that should fail without login
```go
func TestIntegration_CommandWithoutLogin(t *testing.T) {
    _ = setupIntegrationTest(t)  // No login

    var buf bytes.Buffer
    cmd := &cobra.Command{Use: "iz"}
    cmd.AddCommand(targetCmd)
    cmd.SetOut(&buf)
    cmd.SetErr(&buf)
    targetCmd.SetOut(&buf)
    targetCmd.SetErr(&buf)

    cmd.SetArgs([]string{"command"})
    err := cmd.Execute()

    targetCmd.SetOut(nil)
    targetCmd.SetErr(nil)

    require.Error(t, err)
    assert.Contains(t, err.Error(), "no active profile")
}
```

### Pattern 3: Verify state via API after command
```go
func TestIntegration_CommandVerifyState(t *testing.T) {
    env := setupIntegrationTest(t)
    env.Login(t)

    // Execute command
    var buf bytes.Buffer
    cmd := &cobra.Command{Use: "iz"}
    cmd.AddCommand(targetCmd)
    cmd.SetOut(&buf)
    cmd.SetErr(&buf)
    targetCmd.SetOut(&buf)
    targetCmd.SetErr(&buf)

    cmd.SetArgs([]string{"command", "--tenant", "test-tenant"})
    err := cmd.Execute()

    targetCmd.SetOut(nil)
    targetCmd.SetErr(nil)

    require.NoError(t, err)

    // Verify via API
    client := env.NewAuthenticatedClient(t)
    ctx := context.Background()
    result, err := client.SomeAPICall(ctx, "test-tenant")
    require.NoError(t, err)
    assert.Equal(t, expected, result.Field)
}
```

### Pattern 4: Testing izanami package functions directly
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

### Pattern 5: Commands with interactive input
```go
func TestIntegration_CommandWithInput(t *testing.T) {
    env := setupIntegrationTest(t)

    var buf bytes.Buffer
    input := bytes.NewBufferString("user-input\n")

    cmd := &cobra.Command{Use: "iz"}
    cmd.AddCommand(targetCmd)
    cmd.SetOut(&buf)
    cmd.SetErr(&buf)
    cmd.SetIn(input)
    targetCmd.SetOut(&buf)
    targetCmd.SetErr(&buf)
    targetCmd.SetIn(input)

    cmd.SetArgs([]string{"command"})
    err := cmd.Execute()

    targetCmd.SetIn(nil)
    targetCmd.SetOut(nil)
    targetCmd.SetErr(nil)

    require.NoError(t, err)
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

## Important Notes

- Tests are skipped if `IZ_TEST_BASE_URL` not set (handled by `setupIntegrationTest`)
- Each test gets isolated temp directory (no cross-test pollution)
- Always reset command streams after execution (SetOut/SetErr/SetIn to nil)
- Use `t.Logf()` for helpful debug output
- Add `t.Helper()` in any helper functions you create

Now analyze `{{target_file}}` and generate the integration tests.
