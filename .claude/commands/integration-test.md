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

## Requirements

### Test Infrastructure (Already Exists)
- Base helper: `integration_test_helpers_test.go`
- Main struct: `IntegrationTestEnv` with methods:
    - `setupIntegrationTest(t)` - creates isolated test environment
    - `Login(t)` - authenticates and returns JWT token
    - `NewAuthenticatedClient(t)` - returns `*izanami.Client`
- Environment variables: `IZ_TEST_BASE_URL`, `IZ_TEST_USERNAME`, `IZ_TEST_PASSWORD`

### Your Task
Analyze `{{target_file}}` and create comprehensive integration tests that:

1. **Extend IntegrationTestEnv** with tenant lifecycle:
```go
   // CreateTestTenant creates isolated tenant: it-tenant-<hash>
   func (env *IntegrationTestEnv) CreateTestTenant(t *testing.T) string
   
   // DeleteTestTenant cleans up the test tenant
   func (env *IntegrationTestEnv) DeleteTestTenant(t *testing.T)
   
   // ExecuteCommand runs CLI commands in test context
   func (env *IntegrationTestEnv) ExecuteCommand(t *testing.T, args ...string) (stdout, stderr string, exitCode int)
```

2. **Use standard test pattern**:
```go
   func TestSomething(t *testing.T) {
       env := setupIntegrationTest(t)
       env.Login(t)
       env.CreateTestTenant(t)
       t.Cleanup(func() { env.DeleteTestTenant(t) })
       
       // Optional: seed fixtures
       // env.SeedDefaultData(t, "testdata/fixtures/baseline.json")
       
       // Execute CLI commands
       stdout, stderr, exitCode := env.ExecuteCommand(t, "subcommand", "--flag", "value")
       
       // Verify CLI output
       require.Equal(t, 0, exitCode)
       require.Contains(t, stdout, "expected output")
       
       // Verify server state via API
       client := env.NewAuthenticatedClient(t)
       // ... API verification ...
   }
```

3. **Test Coverage** - Cover these scenarios:
    - ✅ Happy path (successful operations)
    - ❌ Error cases (invalid input, auth failures, not found)
    - 🔍 State verification (CLI output + API state match)
    - 🔀 Edge cases specific to `{{target_file}}`

4. **Tenant Isolation Strategy**:
    - Unique name: `it-tenant-<8-char-hash>` from `crypto/sha256`
    - One tenant per test (enable `t.Parallel()` where safe)
    - Always cleanup in `t.Cleanup()` even on failure

5. **Fixture Management** (if needed):
    - Store in `testdata/fixtures/`
    - JSON format with tenant resources
    - Load via `SeedDefaultData(t, "path/to/fixture.json")`

6. **CLI Execution**:
    - Use Cobra command execution pattern from existing `Login()` method
    - Capture stdout/stderr/exit code
    - Reset command state after execution

7. **Assertions**:
    - Use `testify/require` for critical checks (fail fast)
    - Use `testify/assert` for multiple checks
    - Include helpful messages: `require.NoError(t, err, "Context about what failed")`

8. **Cleanup Robustness**:
    - Wrap cleanup in `t.Cleanup()` for guaranteed execution
    - Log cleanup errors with `t.Logf()` but don't fail test
    - Handle "tenant not found" gracefully during cleanup

## Analysis Steps

1. **Read** `{{target_file}}` to understand:
    - Public functions/commands exposed
    - Expected inputs/outputs
    - Error conditions
    - State changes in Izanami server

2. **Identify** test scenarios:
    - What CLI commands are being tested?
    - What server resources are created/modified/deleted?
    - What fixtures are needed?

3. **Generate** the test file with:
    - Package declaration matching `{{target_file}}`
    - Necessary imports
    - Helper methods (if needed for this specific file)
    - Test functions following the pattern above
    - Fixture files in `testdata/fixtures/` (if needed)

4. **Include** these best practices:
    - Use `t.Helper()` in helper functions
    - Add descriptive test names: `TestTenantCreate_ValidInput_Success`
    - Group related tests with subtests: `t.Run("subtest", func(t *testing.T) {...})`
    - Add comments explaining complex setup or assertions

## Output Structure

Generate these files:

1. **Primary**: `{{target_file | replace: ".go", "_integration_test.go"}}`
2. **Fixtures** (if needed): `testdata/fixtures/<resource>_fixture.json`
3. **Helper extensions** (if needed): Add methods to `integration_test_helpers_test.go`

## Example Output

For `cmd/tenant.go`, generate `cmd/tenant_integration_test.go`:
```go
package cmd

import (
    "testing"
    "github.com/stretchr/testify/require"
)

func TestTenantCreate_ValidInput_Success(t *testing.T) {
    env := setupIntegrationTest(t)
    env.Login(t)
    env.CreateTestTenant(t)
    t.Cleanup(func() { env.DeleteTestTenant(t) })

    stdout, _, exitCode := env.ExecuteCommand(t, "tenant", "create", "--name", "test-tenant")
    require.Equal(t, 0, exitCode)
    require.Contains(t, stdout, "Tenant created successfully")

    // Verify via API
    client := env.NewAuthenticatedClient(t)
    tenant, err := client.GetTenant("test-tenant")
    require.NoError(t, err)
    require.Equal(t, "test-tenant", tenant.Name)
}
```

## Important Notes

- Skip tests if `IZ_TEST_BASE_URL` not set (use `setupIntegrationTest` - it handles this)
- Design for parallel execution where possible
- Focus on integration (CLI + API state), not unit tests
- Keep tests fast: avoid unnecessary waits, use minimal fixtures
- Make tests deterministic: no race conditions, no flaky assertions

## Constraints from Existing Code

- Testing framework: standard `testing` package
- Assertion library: `testify/require` and `testify/assert`
- CLI framework: Cobra (`github.com/spf13/cobra`)
- Client library: `github.com/webskin/izanami-go-cli/internal/izanami`
- Go version: modern (assume 1.21+)

Now analyze `{{target_file}}` and generate the integration tests.