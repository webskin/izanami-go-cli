---
name: test
description: Generate unit tests for a Go file following Izanami patterns
arguments:
  - name: $target_file
    description: The Go source file to create unit tests for
    required: true
---

# Unit Test Generator for Izanami CLI

You are generating unit tests for the Izanami Go CLI following established patterns.

## Target File
- Source file: {{target_file}}
- Test file: `{{target_file | replace: ".go", "_test.go"}}`

## Running Tests

```bash
# Run all unit tests
make test

# Run specific test file
go test -v ./internal/cmd/ -run "TestProfileListCmd"

# Run with coverage
go test -v -race -coverprofile=coverage.out ./...
```

## Test Patterns by Package

### Pattern 1: Command Tests (`internal/cmd/`)

For Cobra command tests, use isolated temp directories and mock path functions:

```go
package cmd

import (
    "bytes"
    "os"
    "path/filepath"
    "testing"

    "github.com/spf13/cobra"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/webskin/izanami-go-cli/internal/izanami"
)

// Test helper: Setup paths for isolated test environment
type testPaths struct {
    configPath   string
    sessionsPath string
    configDir    string
    homeDir      string
}

func setupTestPaths(t *testing.T) *testPaths {
    t.Helper()
    tempDir := t.TempDir()
    return &testPaths{
        homeDir:      tempDir,
        configDir:    filepath.Join(tempDir, ".config", "iz"),
        configPath:   filepath.Join(tempDir, ".config", "iz", "config.yaml"),
        sessionsPath: filepath.Join(tempDir, ".izsessions"),
    }
}

// Test helper: Override izanami path functions
func overridePathFunctions(t *testing.T, paths *testPaths) {
    t.Helper()
    originalGetConfigDir := izanami.GetConfigDir
    originalGetSessionsPath := izanami.GetSessionsPath

    izanami.SetGetConfigDirFunc(func() string { return paths.configDir })
    izanami.SetGetSessionsPathFunc(func() string { return paths.sessionsPath })

    t.Cleanup(func() {
        izanami.SetGetConfigDirFunc(originalGetConfigDir)
        izanami.SetGetSessionsPathFunc(originalGetSessionsPath)
    })
}

// Test helper: Setup command with I/O streams
func setupCommand(buf *bytes.Buffer, input *bytes.Buffer, args []string) (*cobra.Command, func()) {
    cmd := &cobra.Command{Use: "test"}
    cmd.AddCommand(targetCmd)  // The command being tested
    cmd.SetOut(buf)
    cmd.SetErr(buf)
    if input != nil {
        cmd.SetIn(input)
        targetCmd.SetIn(input)
    }
    targetCmd.SetOut(buf)
    targetCmd.SetErr(buf)
    cmd.SetArgs(args)

    cleanup := func() {
        targetCmd.SetIn(nil)
        targetCmd.SetOut(nil)
        targetCmd.SetErr(nil)
    }
    return cmd, cleanup
}

func TestCommand_Scenario(t *testing.T) {
    paths := setupTestPaths(t)
    overridePathFunctions(t, paths)

    // Create test files if needed
    createTestFile(t, paths.configPath, "content", 0600)

    var buf bytes.Buffer
    cmd, cleanup := setupCommand(&buf, nil, []string{"command", "--flag"})
    defer cleanup()

    err := cmd.Execute()
    require.NoError(t, err)

    output := buf.String()
    assert.Contains(t, output, "expected")
}
```

### Pattern 2: API Client Tests (`internal/izanami/`)

For API client tests, use `httptest.Server` to mock HTTP responses:

```go
package izanami

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// mockServer creates a test HTTP server
func mockServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
    return httptest.NewServer(handler)
}

func TestClient_ListResources(t *testing.T) {
    expectedResources := []Resource{
        {ID: "resource-1", Name: "Test 1"},
        {ID: "resource-2", Name: "Test 2"},
    }

    server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
        // Verify request
        assert.Equal(t, "/api/admin/tenants/test-tenant/resources", r.URL.Path)
        assert.Equal(t, "GET", r.Method)

        // Return mock response
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(expectedResources)
    })
    defer server.Close()

    config := &Config{
        BaseURL:  server.URL,
        Username: "test-user",
        JwtToken: "test-jwt-token",
        Timeout:  30,
    }

    client, err := NewClient(config)
    require.NoError(t, err)

    ctx := context.Background()
    resources, err := client.ListResources(ctx, "test-tenant")

    assert.NoError(t, err)
    assert.Len(t, resources, 2)
    assert.Equal(t, "resource-1", resources[0].ID)
}

func TestClient_CreateResource(t *testing.T) {
    server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
        assert.Equal(t, "POST", r.Method)
        assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

        // Decode and verify request body
        var body map[string]interface{}
        json.NewDecoder(r.Body).Decode(&body)
        assert.Equal(t, "new-resource", body["name"])

        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(map[string]string{"id": "new-resource"})
    })
    defer server.Close()

    // ... create client and test
}

func TestClient_ErrorHandling(t *testing.T) {
    server := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusNotFound)
        json.NewEncoder(w).Encode(ErrorResponse{Message: "Not found"})
    })
    defer server.Close()

    // ... verify error is returned correctly
}
```

### Pattern 3: Table-Driven Tests

For functions with multiple input/output combinations:

```go
func TestFunction(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {
            name:    "valid input",
            input:   "test",
            want:    "expected",
            wantErr: false,
        },
        {
            name:    "invalid input",
            input:   "",
            want:    "",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Function(tt.input)
            if tt.wantErr {
                assert.Error(t, err)
                return
            }
            assert.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### Pattern 4: Config/File Tests

For tests involving config files:

```go
// Test helper: Create config file with profiles
func createTestConfig(t *testing.T, configPath string, profiles map[string]*izanami.Profile, activeProfile string) {
    t.Helper()

    dir := filepath.Dir(configPath)
    err := os.MkdirAll(dir, 0700)
    require.NoError(t, err)

    config := map[string]interface{}{
        "timeout":       30,
        "verbose":       false,
        "output-format": "table",
        "color":         "auto",
    }

    if activeProfile != "" {
        config["active_profile"] = activeProfile
    }

    if len(profiles) > 0 {
        profilesMap := make(map[string]interface{})
        for name, profile := range profiles {
            profileMap := make(map[string]interface{})
            if profile.LeaderURL != "" {
                profileMap["leader-url"] = profile.LeaderURL
            }
            if profile.Tenant != "" {
                profileMap["tenant"] = profile.Tenant
            }
            // ... other fields
            profilesMap[name] = profileMap
        }
        config["profiles"] = profilesMap
    }

    data, err := yaml.Marshal(config)
    require.NoError(t, err)

    err = os.WriteFile(configPath, data, 0600)
    require.NoError(t, err)
}

// Test helper: Create generic test file
func createTestFile(t *testing.T, path string, content string, perm os.FileMode) {
    t.Helper()
    dir := filepath.Dir(path)
    err := os.MkdirAll(dir, 0755)
    require.NoError(t, err)
    err = os.WriteFile(path, []byte(content), perm)
    require.NoError(t, err)
}
```

### Pattern 5: Tests with User Input

For commands that read user input:

```go
func TestCommand_WithUserInput(t *testing.T) {
    paths := setupTestPaths(t)
    overridePathFunctions(t, paths)

    var buf bytes.Buffer
    input := bytes.NewBufferString("user input\n")

    cmd, cleanup := setupCommand(&buf, input, []string{"command"})
    defer cleanup()

    err := cmd.Execute()
    require.NoError(t, err)

    output := buf.String()
    assert.Contains(t, output, "prompt shown")
}
```

## Test Coverage Categories

For each function/command, create tests for:

1. **Happy Path** - Normal operation with valid inputs
2. **Error Cases** - Invalid inputs, missing required values
3. **Edge Cases** - Empty values, special characters, boundary conditions
4. **State Verification** - Verify side effects (files created, config updated)

## Naming Conventions

```go
// For exported functions/types
func TestTypeName_MethodName_Scenario(t *testing.T)
func TestFunctionName_Scenario(t *testing.T)

// For private functions
func Test_functionName_Scenario(t *testing.T)

// Examples:
func TestClient_ListFeatures(t *testing.T)
func TestClient_ListFeatures_WithTagFilter(t *testing.T)
func TestProfileListCmd_NoProfiles(t *testing.T)
func TestProfileListCmd_MultipleProfiles(t *testing.T)
func Test_buildPath_SimplePath(t *testing.T)
```

## Assertions

```go
// Use require for critical checks (stops test on failure)
require.NoError(t, err, "Setup should succeed")
require.NotNil(t, result, "Result should not be nil")

// Use assert for multiple checks (continues on failure)
assert.Equal(t, expected, actual, "Values should match")
assert.Contains(t, output, "expected text")
assert.True(t, condition, "Condition should be true")
assert.Error(t, err)
assert.NoFileExists(t, path)
assert.FileExists(t, path)
assert.Regexp(t, pattern, value)
```

## Example: Reference Existing Tests

Look at these files for patterns:
- `internal/cmd/reset_test.go` - Command tests with file operations
- `internal/cmd/profiles_test.go` - Profile command tests
- `internal/izanami/features_test.go` - API client tests with mock server
- `internal/izanami/client_test.go` - Table-driven tests, error handling
- `internal/output/formatter_test.go` - Output formatting tests

## Important Notes

- Always use `t.Helper()` in test helper functions
- Use `t.TempDir()` for temp directories (auto-cleaned up)
- Use `t.Cleanup()` for cleanup functions
- Reset command streams after execution (`SetIn/SetOut/SetErr(nil)`)
- Use `defer server.Close()` for mock servers
- Test both success and error cases
- Verify side effects (file created, config updated, etc.)

## Analysis Steps

1. **Read** `{{target_file}}` to understand:
   - Functions and methods defined
   - Input parameters and return values
   - Error conditions
   - Side effects (file I/O, API calls)

2. **Identify** test scenarios:
   - What are valid/invalid inputs?
   - What errors can occur?
   - What state changes should be verified?

3. **Generate** the test file:
   - Same package as source
   - Appropriate imports
   - Test helpers if needed
   - Test functions following patterns above

Now analyze `{{target_file}}` and generate the unit tests.
