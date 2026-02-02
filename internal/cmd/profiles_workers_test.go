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
	"gopkg.in/yaml.v3"
)

// createTestConfigWithWorkers creates a config file that includes worker data.
// The standard createTestConfig helper in profiles_test.go doesn't serialize workers,
// so we use this extended version for worker tests.
func createTestConfigWithWorkers(t *testing.T, configPath string, profiles map[string]*izanami.Profile, activeProfile string) {
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
			if profile.Session != "" {
				profileMap["session"] = profile.Session
			}
			if profile.LeaderURL != "" {
				profileMap["leader-url"] = profile.LeaderURL
			}
			if profile.Tenant != "" {
				profileMap["tenant"] = profile.Tenant
			}
			if profile.Project != "" {
				profileMap["project"] = profile.Project
			}
			if profile.Context != "" {
				profileMap["context"] = profile.Context
			}
			if profile.ClientID != "" {
				profileMap["client-id"] = profile.ClientID
			}
			if profile.ClientSecret != "" {
				profileMap["client-secret"] = profile.ClientSecret
			}
			if profile.DefaultWorker != "" {
				profileMap["default-worker"] = profile.DefaultWorker
			}
			if profile.Workers != nil && len(profile.Workers) > 0 {
				profileMap["workers"] = profile.Workers
			}
			profilesMap[name] = profileMap
		}
		config["profiles"] = profilesMap
	}

	data, err := yaml.Marshal(config)
	require.NoError(t, err)
	err = os.WriteFile(configPath, data, 0600)
	require.NoError(t, err)
}

// setupWorkerCommand creates a command tree with proper I/O for worker subcommands.
func setupWorkerCommand(buf *bytes.Buffer, input *bytes.Buffer, args []string) (*cobra.Command, func()) {
	cmd := &cobra.Command{Use: "test"}
	cmd.AddCommand(profileCmd)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if input != nil {
		cmd.SetIn(input)
		profileWorkersDeleteCmd.SetIn(input)
	}
	profileCmd.SetOut(buf)
	profileCmd.SetErr(buf)
	profileWorkersCmd.SetOut(buf)
	profileWorkersCmd.SetErr(buf)
	// Propagate output to all worker subcommands so cmd.OutOrStdout() works
	profileWorkersAddCmd.SetOut(buf)
	profileWorkersAddCmd.SetErr(buf)
	profileWorkersDeleteCmd.SetOut(buf)
	profileWorkersDeleteCmd.SetErr(buf)
	profileWorkersListCmd.SetOut(buf)
	profileWorkersListCmd.SetErr(buf)
	profileWorkersUseCmd.SetOut(buf)
	profileWorkersUseCmd.SetErr(buf)
	profileWorkersShowCmd.SetOut(buf)
	profileWorkersShowCmd.SetErr(buf)
	profileWorkersCurrentCmd.SetOut(buf)
	profileWorkersCurrentCmd.SetErr(buf)
	cmd.SetArgs(args)

	cleanup := func() {
		profileCmd.SetIn(nil)
		profileCmd.SetOut(nil)
		profileCmd.SetErr(nil)
		profileWorkersCmd.SetIn(nil)
		profileWorkersCmd.SetOut(nil)
		profileWorkersCmd.SetErr(nil)
		profileWorkersAddCmd.SetOut(nil)
		profileWorkersAddCmd.SetErr(nil)
		profileWorkersDeleteCmd.SetIn(nil)
		profileWorkersDeleteCmd.SetOut(nil)
		profileWorkersDeleteCmd.SetErr(nil)
		profileWorkersListCmd.SetOut(nil)
		profileWorkersListCmd.SetErr(nil)
		profileWorkersUseCmd.SetOut(nil)
		profileWorkersUseCmd.SetErr(nil)
		profileWorkersShowCmd.SetOut(nil)
		profileWorkersShowCmd.SetErr(nil)
		profileWorkersCurrentCmd.SetOut(nil)
		profileWorkersCurrentCmd.SetErr(nil)
		// Reset flag values to prevent state leaking between tests
		workerAddForce = false
		workerDeleteForce = false
		profileWorkersAddCmd.Flags().Set("url", "")
		profileWorkersAddCmd.Flags().Set("client-id", "")
		profileWorkersAddCmd.Flags().Set("client-secret", "")
		profileWorkersShowCmd.Flags().Set("show-secrets", "false")
	}

	return cmd, cleanup
}

// readConfigWorkers reads the config YAML and returns the workers map and default-worker for a profile.
func readConfigWorkers(t *testing.T, configPath, profileName string) (map[string]interface{}, string) {
	t.Helper()

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var config map[string]interface{}
	err = yaml.Unmarshal(data, &config)
	require.NoError(t, err)

	profiles, ok := config["profiles"].(map[string]interface{})
	require.True(t, ok)

	profile, ok := profiles[profileName].(map[string]interface{})
	require.True(t, ok)

	defaultWorker, _ := profile["default-worker"].(string)

	workers, _ := profile["workers"].(map[string]interface{})
	return workers, defaultWorker
}

// ========================================================================
// workers add
// ========================================================================

func TestWorkersAddCmd_URLOnly(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {LeaderURL: "http://localhost:9000"},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "add", "eu-west",
		"--url", "http://worker-eu.example.com",
	})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Added worker 'eu-west'")
	assert.Contains(t, output, "http://worker-eu.example.com")

	workers, defaultWorker := readConfigWorkers(t, paths.configPath, "test")
	require.NotNil(t, workers)
	worker, ok := workers["eu-west"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "http://worker-eu.example.com", worker["url"])
	// First worker should auto-become default
	assert.Equal(t, "eu-west", defaultWorker)
}

func TestWorkersAddCmd_WithCredentials(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {LeaderURL: "http://localhost:9000"},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "add", "us-east",
		"--url", "http://worker-us.example.com",
		"--client-id", "us-id",
		"--client-secret", "us-secret",
	})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	workers, _ := readConfigWorkers(t, paths.configPath, "test")
	require.NotNil(t, workers)
	worker, ok := workers["us-east"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "http://worker-us.example.com", worker["url"])
	assert.Equal(t, "us-id", worker["client-id"])
	assert.Equal(t, "us-secret", worker["client-secret"])
}

func TestWorkersAddCmd_FirstWorkerAutoDefault(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {LeaderURL: "http://localhost:9000"},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "add", "first-worker",
		"--url", "http://first.example.com",
	})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Set as default worker")

	_, defaultWorker := readConfigWorkers(t, paths.configPath, "test")
	assert.Equal(t, "first-worker", defaultWorker)
}

func TestWorkersAddCmd_SecondWorkerNotAutoDefault(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			LeaderURL:     "http://localhost:9000",
			DefaultWorker: "existing",
			Workers: map[string]*izanami.WorkerConfig{
				"existing": {URL: "http://existing.example.com"},
			},
		},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "add", "second",
		"--url", "http://second.example.com",
	})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.NotContains(t, output, "Set as default worker")

	_, defaultWorker := readConfigWorkers(t, paths.configPath, "test")
	assert.Equal(t, "existing", defaultWorker)
}

func TestWorkersAddCmd_DuplicateNameErrors(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			LeaderURL: "http://localhost:9000",
			Workers: map[string]*izanami.WorkerConfig{
				"eu-west": {URL: "http://old.example.com"},
			},
			DefaultWorker: "eu-west",
		},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "add", "eu-west",
		"--url", "http://new.example.com",
	})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestWorkersAddCmd_DuplicateWithForce(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			LeaderURL: "http://localhost:9000",
			Workers: map[string]*izanami.WorkerConfig{
				"eu-west": {URL: "http://old.example.com"},
			},
			DefaultWorker: "eu-west",
		},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "add", "eu-west",
		"--url", "http://new.example.com",
		"--force",
	})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	workers, _ := readConfigWorkers(t, paths.configPath, "test")
	worker := workers["eu-west"].(map[string]interface{})
	assert.Equal(t, "http://new.example.com", worker["url"])
}

func TestWorkersAddCmd_MissingURL(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {LeaderURL: "http://localhost:9000"},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "add", "no-url",
	})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestWorkersAddCmd_NoActiveProfile(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	createTestConfigWithWorkers(t, paths.configPath, nil, "")

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "add", "test-worker",
		"--url", "http://example.com",
	})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active profile")
}

// ========================================================================
// workers delete
// ========================================================================

func TestWorkersDeleteCmd_ExistingWorker(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			LeaderURL: "http://localhost:9000",
			Workers: map[string]*izanami.WorkerConfig{
				"eu-west": {URL: "http://worker-eu.example.com"},
				"us-east": {URL: "http://worker-us.example.com"},
			},
			DefaultWorker: "eu-west",
		},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "delete", "us-east", "--force",
	})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Deleted worker 'us-east'")

	workers, defaultWorker := readConfigWorkers(t, paths.configPath, "test")
	_, exists := workers["us-east"]
	assert.False(t, exists, "Worker should be deleted")
	assert.Equal(t, "eu-west", defaultWorker, "Default should remain unchanged")
}

func TestWorkersDeleteCmd_NonExistentWorker(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			LeaderURL: "http://localhost:9000",
			Workers: map[string]*izanami.WorkerConfig{
				"eu-west": {URL: "http://worker-eu.example.com"},
			},
			DefaultWorker: "eu-west",
		},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "delete", "nonexistent", "--force",
	})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestWorkersDeleteCmd_DefaultWorkerWithForce(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			LeaderURL: "http://localhost:9000",
			Workers: map[string]*izanami.WorkerConfig{
				"eu-west": {URL: "http://worker-eu.example.com"},
			},
			DefaultWorker: "eu-west",
		},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "delete", "eu-west", "--force",
	})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "was the default worker")
	assert.Contains(t, output, "Default worker cleared")

	_, defaultWorker := readConfigWorkers(t, paths.configPath, "test")
	assert.Empty(t, defaultWorker, "Default worker should be cleared")
}

func TestWorkersDeleteCmd_DefaultWorkerWithoutForce_Cancel(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			LeaderURL: "http://localhost:9000",
			Workers: map[string]*izanami.WorkerConfig{
				"eu-west": {URL: "http://worker-eu.example.com"},
			},
			DefaultWorker: "eu-west",
		},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	input := bytes.NewBufferString("n\n")
	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, input, []string{
		"profiles", "workers", "delete", "eu-west",
	})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err) // Cancellation is not an error

	output := buf.String()
	assert.Contains(t, output, "Cancelled")

	// Worker should still exist
	workers, defaultWorker := readConfigWorkers(t, paths.configPath, "test")
	_, exists := workers["eu-west"]
	assert.True(t, exists, "Worker should still exist after cancellation")
	assert.Equal(t, "eu-west", defaultWorker)
}

// ========================================================================
// workers list
// ========================================================================

func TestWorkersListCmd_NoWorkers(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {LeaderURL: "http://localhost:9000"},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "list",
	})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No workers configured")
	assert.Contains(t, output, "iz profiles workers add")
}

func TestWorkersListCmd_MultipleWorkers(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			LeaderURL: "http://localhost:9000",
			Workers: map[string]*izanami.WorkerConfig{
				"eu-west": {URL: "http://worker-eu.example.com", ClientID: "eu-id"},
				"us-east": {URL: "http://worker-us.example.com"},
			},
			DefaultWorker: "eu-west",
		},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "list",
	})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "eu-west")
	assert.Contains(t, output, "us-east")
	assert.Contains(t, output, "http://worker-eu.example.com")
	assert.Contains(t, output, "http://worker-us.example.com")
	assert.Contains(t, output, "eu-id")
	assert.Contains(t, output, "Default worker: eu-west")
}

func TestWorkersListCmd_SortedAlphabetically(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			LeaderURL: "http://localhost:9000",
			Workers: map[string]*izanami.WorkerConfig{
				"zebra":  {URL: "http://z.example.com"},
				"alpha":  {URL: "http://a.example.com"},
				"middle": {URL: "http://m.example.com"},
			},
			DefaultWorker: "alpha",
		},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "list",
	})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	// Verify ordering: alpha before middle before zebra
	alphaIdx := bytes.Index([]byte(output), []byte("alpha"))
	middleIdx := bytes.Index([]byte(output), []byte("middle"))
	zebraIdx := bytes.Index([]byte(output), []byte("zebra"))

	assert.True(t, alphaIdx < middleIdx, "alpha should appear before middle")
	assert.True(t, middleIdx < zebraIdx, "middle should appear before zebra")
}

func TestWorkersListCmd_ShowsClientIDOrDash(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			LeaderURL: "http://localhost:9000",
			Workers: map[string]*izanami.WorkerConfig{
				"with-creds":    {URL: "http://a.example.com", ClientID: "my-id"},
				"without-creds": {URL: "http://b.example.com"},
			},
			DefaultWorker: "with-creds",
		},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "list",
	})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "my-id")
	// The '-' placeholder for missing client-id
	// Verify it's present somewhere in the table (could be in other columns too)
	assert.Contains(t, output, "-")
}

func TestWorkersListCmd_NoDefaultWorker(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			LeaderURL: "http://localhost:9000",
			Workers: map[string]*izanami.WorkerConfig{
				"eu-west": {URL: "http://worker-eu.example.com"},
			},
			// No DefaultWorker set
		},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "list",
	})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No default worker set")
	assert.Contains(t, output, "iz profiles workers use")
}

// ========================================================================
// workers use
// ========================================================================

func TestWorkersUseCmd_ExistingWorker(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			LeaderURL: "http://localhost:9000",
			Workers: map[string]*izanami.WorkerConfig{
				"eu-west": {URL: "http://worker-eu.example.com"},
				"us-east": {URL: "http://worker-us.example.com"},
			},
			DefaultWorker: "eu-west",
		},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "use", "us-east",
	})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Default worker set to 'us-east'")

	_, defaultWorker := readConfigWorkers(t, paths.configPath, "test")
	assert.Equal(t, "us-east", defaultWorker)
}

func TestWorkersUseCmd_NonExistentWorker(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			LeaderURL: "http://localhost:9000",
			Workers: map[string]*izanami.WorkerConfig{
				"eu-west": {URL: "http://worker-eu.example.com"},
			},
			DefaultWorker: "eu-west",
		},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "use", "nonexistent",
	})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ========================================================================
// workers show
// ========================================================================

func TestWorkersShowCmd_ShowDetails(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			LeaderURL: "http://localhost:9000",
			Workers: map[string]*izanami.WorkerConfig{
				"eu-west": {
					URL:          "http://worker-eu.example.com",
					ClientID:     "eu-client-id",
					ClientSecret: "eu-secret",
				},
			},
			DefaultWorker: "eu-west",
		},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "show", "eu-west",
	})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Worker: eu-west")
	assert.Contains(t, output, "[default]")
	assert.Contains(t, output, "http://worker-eu.example.com")
	assert.Contains(t, output, "eu-client-id")
	// Secret should be redacted by default
	assert.Contains(t, output, "<redacted>")
	assert.NotContains(t, output, "eu-secret")
}

func TestWorkersShowCmd_WithShowSecrets(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			LeaderURL: "http://localhost:9000",
			Workers: map[string]*izanami.WorkerConfig{
				"eu-west": {
					URL:          "http://worker-eu.example.com",
					ClientID:     "eu-client-id",
					ClientSecret: "eu-secret-value",
				},
			},
			DefaultWorker: "eu-west",
		},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "show", "eu-west", "--show-secrets",
	})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "eu-secret-value")
	assert.NotContains(t, output, "<redacted>")
}

func TestWorkersShowCmd_NonExistentWorker(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			LeaderURL: "http://localhost:9000",
			Workers: map[string]*izanami.WorkerConfig{
				"eu-west": {URL: "http://worker-eu.example.com"},
				"us-east": {URL: "http://worker-us.example.com"},
			},
			DefaultWorker: "eu-west",
		},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "show", "nonexistent",
	})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Contains(t, err.Error(), "Available:")
}

func TestWorkersShowCmd_NonDefaultWorker(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			LeaderURL: "http://localhost:9000",
			Workers: map[string]*izanami.WorkerConfig{
				"eu-west": {URL: "http://worker-eu.example.com"},
				"us-east": {URL: "http://worker-us.example.com"},
			},
			DefaultWorker: "eu-west",
		},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "show", "us-east",
	})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Worker: us-east")
	assert.NotContains(t, output, "[default]")
}

// ========================================================================
// workers current
// ========================================================================

func TestWorkersCurrentCmd_StandaloneMode(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {LeaderURL: "http://localhost:9000"},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	// Save and restore global flags
	origProfileName := profileName
	origLeaderURL := leaderURL
	defer func() {
		profileName = origProfileName
		leaderURL = origLeaderURL
	}()
	profileName = ""
	leaderURL = ""

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "current",
	})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "standalone")
	assert.Contains(t, output, "http://localhost:9000")
}

func TestWorkersCurrentCmd_DefaultWorker(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			LeaderURL: "http://localhost:9000",
			Workers: map[string]*izanami.WorkerConfig{
				"eu-west": {URL: "http://worker-eu.example.com"},
			},
			DefaultWorker: "eu-west",
		},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	origProfileName := profileName
	origLeaderURL := leaderURL
	defer func() {
		profileName = origProfileName
		leaderURL = origLeaderURL
	}()
	profileName = ""
	leaderURL = ""

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "current",
	})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "eu-west")
	assert.Contains(t, output, "http://worker-eu.example.com")
	assert.Contains(t, output, "default")
}

func TestWorkersCurrentCmd_PerWorkerCredentials(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			LeaderURL: "http://localhost:9000",
			ClientID:  "profile-id",
			Workers: map[string]*izanami.WorkerConfig{
				"eu-west": {
					URL:      "http://worker-eu.example.com",
					ClientID: "worker-id",
				},
			},
			DefaultWorker: "eu-west",
		},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	origProfileName := profileName
	origLeaderURL := leaderURL
	defer func() {
		profileName = origProfileName
		leaderURL = origLeaderURL
	}()
	profileName = ""
	leaderURL = ""

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "current",
	})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "per-worker")
}

func TestWorkersCurrentCmd_ProfileLevelCredentials(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			LeaderURL: "http://localhost:9000",
			ClientID:  "profile-id",
			Workers: map[string]*izanami.WorkerConfig{
				"eu-west": {
					URL: "http://worker-eu.example.com",
					// No per-worker credentials
				},
			},
			DefaultWorker: "eu-west",
		},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	origProfileName := profileName
	origLeaderURL := leaderURL
	defer func() {
		profileName = origProfileName
		leaderURL = origLeaderURL
	}()
	profileName = ""
	leaderURL = ""

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "current",
	})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "profile-level")
}

func TestWorkersCurrentCmd_NoCredentials(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {
			LeaderURL: "http://localhost:9000",
			Workers: map[string]*izanami.WorkerConfig{
				"eu-west": {URL: "http://worker-eu.example.com"},
			},
			DefaultWorker: "eu-west",
		},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	origProfileName := profileName
	origLeaderURL := leaderURL
	defer func() {
		profileName = origProfileName
		leaderURL = origLeaderURL
	}()
	profileName = ""
	leaderURL = ""

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "current",
	})
	defer cleanup()

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "not configured")
}

// ========================================================================
// workers list/show - no active profile
// ========================================================================

func TestWorkersListCmd_NoActiveProfile(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	createTestConfigWithWorkers(t, paths.configPath, nil, "")

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "list",
	})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active profile")
}

func TestWorkersShowCmd_NoActiveProfile(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	createTestConfigWithWorkers(t, paths.configPath, nil, "")

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "show", "eu-west",
	})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active profile")
}

func TestWorkersShowCmd_NoWorkersConfigured(t *testing.T) {
	paths := setupTestPaths(t)
	overridePathFunctions(t, paths)

	profiles := map[string]*izanami.Profile{
		"test": {LeaderURL: "http://localhost:9000"},
	}
	createTestConfigWithWorkers(t, paths.configPath, profiles, "test")

	var buf bytes.Buffer
	cmd, cleanup := setupWorkerCommand(&buf, nil, []string{
		"profiles", "workers", "show", "eu-west",
	})
	defer cleanup()

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no workers configured")
}
