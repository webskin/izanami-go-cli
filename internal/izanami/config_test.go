package izanami

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const testLeaderURL = "http://localhost:9000"

// TestResolveClientCredentials tests the credential resolution logic
func TestResolveClientCredentials(t *testing.T) {
	tests := []struct {
		name             string
		config           ResolvedConfig
		tenant           string
		projects         []string
		expectedClientID string
		expectedSecret   string
	}{
		{
			name: "tenant-level credentials",
			config: ResolvedConfig{
				ClientKeys: map[string]TenantClientKeysConfig{
					"tenant1": {
						ClientID:     "tenant1-id",
						ClientSecret: "tenant1-secret",
					},
				},
			},
			tenant:           "tenant1",
			projects:         nil,
			expectedClientID: "tenant1-id",
			expectedSecret:   "tenant1-secret",
		},
		{
			name: "project-level credentials override tenant",
			config: ResolvedConfig{
				ClientKeys: map[string]TenantClientKeysConfig{
					"tenant1": {
						ClientID:     "tenant1-id",
						ClientSecret: "tenant1-secret",
						Projects: map[string]ProjectClientKeysConfig{
							"project1": {
								ClientID:     "proj1-id",
								ClientSecret: "proj1-secret",
							},
						},
					},
				},
			},
			tenant:           "tenant1",
			projects:         []string{"project1"},
			expectedClientID: "proj1-id",
			expectedSecret:   "proj1-secret",
		},
		{
			name: "project not found falls back to tenant",
			config: ResolvedConfig{
				ClientKeys: map[string]TenantClientKeysConfig{
					"tenant1": {
						ClientID:     "tenant1-id",
						ClientSecret: "tenant1-secret",
						Projects: map[string]ProjectClientKeysConfig{
							"project1": {
								ClientID:     "proj1-id",
								ClientSecret: "proj1-secret",
							},
						},
					},
				},
			},
			tenant:           "tenant1",
			projects:         []string{"nonexistent-project"},
			expectedClientID: "tenant1-id",
			expectedSecret:   "tenant1-secret",
		},
		{
			name: "multiple projects - first match wins",
			config: ResolvedConfig{
				ClientKeys: map[string]TenantClientKeysConfig{
					"tenant1": {
						ClientID:     "tenant1-id",
						ClientSecret: "tenant1-secret",
						Projects: map[string]ProjectClientKeysConfig{
							"project1": {
								ClientID:     "proj1-id",
								ClientSecret: "proj1-secret",
							},
							"project2": {
								ClientID:     "proj2-id",
								ClientSecret: "proj2-secret",
							},
						},
					},
				},
			},
			tenant:           "tenant1",
			projects:         []string{"project2", "project1"},
			expectedClientID: "proj2-id",
			expectedSecret:   "proj2-secret",
		},
		{
			name: "tenant not found",
			config: ResolvedConfig{
				ClientKeys: map[string]TenantClientKeysConfig{
					"tenant1": {
						ClientID:     "tenant1-id",
						ClientSecret: "tenant1-secret",
					},
				},
			},
			tenant:           "nonexistent",
			projects:         nil,
			expectedClientID: "",
			expectedSecret:   "",
		},
		{
			name:             "no client keys configured",
			config:           ResolvedConfig{},
			tenant:           "tenant1",
			projects:         nil,
			expectedClientID: "",
			expectedSecret:   "",
		},
		{
			name: "empty tenant",
			config: ResolvedConfig{
				ClientKeys: map[string]TenantClientKeysConfig{
					"tenant1": {
						ClientID:     "tenant1-id",
						ClientSecret: "tenant1-secret",
					},
				},
			},
			tenant:           "",
			projects:         nil,
			expectedClientID: "",
			expectedSecret:   "",
		},
		{
			name: "incomplete project credentials falls back to tenant",
			config: ResolvedConfig{
				ClientKeys: map[string]TenantClientKeysConfig{
					"tenant1": {
						ClientID:     "tenant1-id",
						ClientSecret: "tenant1-secret",
						Projects: map[string]ProjectClientKeysConfig{
							"project1": {
								ClientID: "proj1-id",
								// Missing ClientSecret
							},
						},
					},
				},
			},
			tenant:           "tenant1",
			projects:         []string{"project1"},
			expectedClientID: "tenant1-id",
			expectedSecret:   "tenant1-secret",
		},
		{
			name: "incomplete tenant credentials returns empty",
			config: ResolvedConfig{
				ClientKeys: map[string]TenantClientKeysConfig{
					"tenant1": {
						ClientID: "tenant1-id",
						// Missing ClientSecret
					},
				},
			},
			tenant:           "tenant1",
			projects:         nil,
			expectedClientID: "",
			expectedSecret:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientID, clientSecret := tt.config.ResolveClientCredentials(tt.tenant, tt.projects)
			assert.Equal(t, tt.expectedClientID, clientID, "Client ID mismatch")
			assert.Equal(t, tt.expectedSecret, clientSecret, "Client secret mismatch")
		})
	}
}

// TestGetWorkerURL tests the GetWorkerURL method
func TestGetWorkerURL(t *testing.T) {
	tests := []struct {
		name     string
		config   ResolvedConfig
		expected string
	}{
		{
			name: "returns WorkerURL when set",
			config: ResolvedConfig{
				LeaderURL: "http://admin.example.com",
				WorkerURL: "http://client.example.com",
			},
			expected: "http://client.example.com",
		},
		{
			name: "falls back to LeaderURL when WorkerURL is empty",
			config: ResolvedConfig{
				LeaderURL: "http://admin.example.com",
				WorkerURL: "",
			},
			expected: "http://admin.example.com",
		},
		{
			name:     "returns empty string for zero-value ResolvedConfig",
			config:   ResolvedConfig{},
			expected: "",
		},
		{
			name: "returns LeaderURL when only LeaderURL is set",
			config: ResolvedConfig{
				LeaderURL: "http://example.com",
			},
			expected: "http://example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetWorkerURL()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestResolveWorker tests the worker resolution priority chain
func TestResolveWorker(t *testing.T) {
	tests := []struct {
		name           string
		leaderURL      string
		defaultWorker  string
		workers        map[string]*WorkerConfig
		flagWorker     string
		envWorker      string
		envWorkerURL   string
		expectedURL    string
		expectedSource string
	}{
		{
			name:           "standalone mode when no workers configured",
			leaderURL:      "http://leader.example.com",
			expectedURL:    "",
			expectedSource: "standalone",
		},
		{
			name:          "uses default worker when configured",
			leaderURL:     "http://leader.example.com",
			defaultWorker: "eu-west",
			workers: map[string]*WorkerConfig{
				"eu-west": {URL: "http://worker-eu.example.com"},
			},
			expectedURL:    "http://worker-eu.example.com",
			expectedSource: "default",
		},
		{
			name:          "flag overrides default worker",
			leaderURL:     "http://leader.example.com",
			defaultWorker: "eu-west",
			workers: map[string]*WorkerConfig{
				"eu-west": {URL: "http://worker-eu.example.com"},
				"us-east": {URL: "http://worker-us.example.com"},
			},
			flagWorker:     "us-east",
			expectedURL:    "http://worker-us.example.com",
			expectedSource: "flag",
		},
		{
			name:          "IZ_WORKER env overrides default",
			leaderURL:     "http://leader.example.com",
			defaultWorker: "eu-west",
			workers: map[string]*WorkerConfig{
				"eu-west": {URL: "http://worker-eu.example.com"},
				"us-east": {URL: "http://worker-us.example.com"},
			},
			envWorker:      "us-east",
			expectedURL:    "http://worker-us.example.com",
			expectedSource: "env-name",
		},
		{
			name:           "IZ_WORKER_URL env provides direct URL",
			leaderURL:      "http://leader.example.com",
			envWorkerURL:   "http://direct-worker.example.com",
			expectedURL:    "http://direct-worker.example.com",
			expectedSource: "env-url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env vars
			if tt.envWorker != "" {
				t.Setenv("IZ_WORKER", tt.envWorker)
			}
			if tt.envWorkerURL != "" {
				t.Setenv("IZ_WORKER_URL", tt.envWorkerURL)
			}

			resolved, err := ResolveWorker(tt.flagWorker, tt.workers, tt.defaultWorker, func(format string, a ...interface{}) {})
			require.NoError(t, err)
			assert.Equal(t, tt.expectedURL, resolved.URL)
			assert.Equal(t, tt.expectedSource, resolved.Source)
		})
	}
}

// ========================================================================
// ResolveWorker — additional edge cases
// ========================================================================

// TestResolveWorker_FlagOverridesBothEnvVars verifies --worker flag takes priority
// over both IZ_WORKER and IZ_WORKER_URL simultaneously.
func TestResolveWorker_FlagOverridesBothEnvVars(t *testing.T) {
	t.Setenv("IZ_WORKER", "env-name-worker")
	t.Setenv("IZ_WORKER_URL", "http://env-url-worker.example.com")

	workers := map[string]*WorkerConfig{
		"env-name-worker": {URL: "http://env-name.example.com"},
		"flag-worker":     {URL: "http://flag.example.com"},
	}

	resolved, err := ResolveWorker("flag-worker", workers, "", func(format string, a ...interface{}) {})
	require.NoError(t, err)
	assert.Equal(t, "http://flag.example.com", resolved.URL)
	assert.Equal(t, "flag", resolved.Source)
	assert.Equal(t, "flag-worker", resolved.Name)
}

// TestResolveWorker_EnvNameOverridesEnvURL verifies IZ_WORKER takes priority over IZ_WORKER_URL.
func TestResolveWorker_EnvNameOverridesEnvURL(t *testing.T) {
	t.Setenv("IZ_WORKER", "named-worker")
	t.Setenv("IZ_WORKER_URL", "http://direct-url.example.com")

	workers := map[string]*WorkerConfig{
		"named-worker": {URL: "http://named.example.com"},
	}

	resolved, err := ResolveWorker("", workers, "", func(format string, a ...interface{}) {})
	require.NoError(t, err)
	assert.Equal(t, "http://named.example.com", resolved.URL)
	assert.Equal(t, "env-name", resolved.Source)
}

// TestResolveWorker_FlagNonExistentWorker verifies that --worker flag
// referencing a non-existent worker returns an error with available workers.
func TestResolveWorker_FlagNonExistentWorker(t *testing.T) {
	workers := map[string]*WorkerConfig{
		"eu-west": {URL: "http://worker-eu.example.com"},
	}

	_, err := ResolveWorker("nonexistent", workers, "", func(format string, a ...interface{}) {})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
	assert.Contains(t, err.Error(), "not found")
	assert.Contains(t, err.Error(), "eu-west", "Error should list available workers")
}

// TestResolveWorker_FlagNoWorkersConfigured verifies that --worker flag
// with no workers configured returns an error with helpful message.
func TestResolveWorker_FlagNoWorkersConfigured(t *testing.T) {
	_, err := ResolveWorker("some-worker", nil, "", func(format string, a ...interface{}) {})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "some-worker")
	assert.Contains(t, err.Error(), "no workers configured")
}

// TestResolveWorker_EnvNameNonExistentWorker verifies that IZ_WORKER
// referencing a non-existent worker returns an error.
func TestResolveWorker_EnvNameNonExistentWorker(t *testing.T) {
	t.Setenv("IZ_WORKER", "nonexistent")

	workers := map[string]*WorkerConfig{
		"eu-west": {URL: "http://worker-eu.example.com"},
	}

	_, err := ResolveWorker("", workers, "", func(format string, a ...interface{}) {})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
	assert.Contains(t, err.Error(), "eu-west")
}

// TestResolveWorker_DefaultWorkerMissingFallsBack verifies that a dangling default-worker
// reference falls back to standalone mode with a warning (not an error).
func TestResolveWorker_DefaultWorkerMissingFallsBack(t *testing.T) {
	var warnings []string
	workers := map[string]*WorkerConfig{
		"remaining": {URL: "http://remaining.example.com"},
	}

	resolved, err := ResolveWorker("", workers, "deleted-worker", func(format string, a ...interface{}) {
		warnings = append(warnings, fmt.Sprintf(format, a...))
	})

	require.NoError(t, err, "Dangling default-worker should warn, not error")
	assert.Equal(t, "standalone", resolved.Source)
	assert.Equal(t, "", resolved.URL)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "deleted-worker")
	assert.Contains(t, warnings[0], "remaining")
}

// TestResolveWorker_PerWorkerCredentialsSet verifies that worker-specific
// credentials are returned in the ResolvedWorker result.
func TestResolveWorker_PerWorkerCredentialsSet(t *testing.T) {
	workers := map[string]*WorkerConfig{
		"eu-west": {
			URL: "http://worker-eu.example.com",
			ClientKeys: map[string]TenantClientKeysConfig{
				"my-tenant": {
					ClientID:     "worker-id",
					ClientSecret: "worker-secret",
				},
			},
		},
	}

	resolved, err := ResolveWorker("", workers, "eu-west", func(format string, a ...interface{}) {})
	require.NoError(t, err)

	require.Contains(t, resolved.ClientKeys, "my-tenant")
	assert.Equal(t, "worker-id", resolved.ClientKeys["my-tenant"].ClientID)
	assert.Equal(t, "worker-secret", resolved.ClientKeys["my-tenant"].ClientSecret)
	assert.Equal(t, "http://worker-eu.example.com", resolved.URL)
	assert.Equal(t, "default", resolved.Source)
}

// TestResolveWorker_StandaloneEmptyLeaderURL verifies that standalone mode
// with no workers results in standalone source and empty URL.
func TestResolveWorker_StandaloneEmptyLeaderURL(t *testing.T) {
	resolved, err := ResolveWorker("", nil, "", func(format string, a ...interface{}) {})
	require.NoError(t, err)

	assert.Equal(t, "standalone", resolved.Source)
	assert.Equal(t, "", resolved.URL)
}

// TestResolveWorker_DefaultWorkerNoWorkersList verifies that default-worker
// with an empty workers map falls back to standalone with a warning.
func TestResolveWorker_DefaultWorkerNoWorkersList(t *testing.T) {
	var warnings []string

	resolved, err := ResolveWorker("", nil, "missing", func(format string, a ...interface{}) {
		warnings = append(warnings, fmt.Sprintf(format, a...))
	})

	require.NoError(t, err, "Dangling default-worker should warn, not error")
	assert.Equal(t, "standalone", resolved.Source)
	assert.Equal(t, "", resolved.URL)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "missing")
}

// ========================================================================
// Worker CRUD functions (AddWorker, DeleteWorker, SetDefaultWorker)
// ========================================================================

// setupWorkerCRUDTest creates a temp config with an active profile for worker CRUD tests.
func setupWorkerCRUDTest(t *testing.T, profile *Profile) {
	t.Helper()
	tempDir := t.TempDir()
	originalGetConfigDir := getConfigDir
	t.Cleanup(func() { getConfigDir = originalGetConfigDir })
	getConfigDir = func() string { return tempDir }

	configPath := filepath.Join(tempDir, "config.yaml")
	config := map[string]interface{}{
		"timeout":        30,
		"verbose":        false,
		"output-format":  "table",
		"color":          "auto",
		"active_profile": "test",
	}

	profileMap := make(map[string]interface{})
	if profile.LeaderURL != "" {
		profileMap["leader-url"] = profile.LeaderURL
	}
	if profile.DefaultWorker != "" {
		profileMap["default-worker"] = profile.DefaultWorker
	}
	if profile.Workers != nil && len(profile.Workers) > 0 {
		profileMap["workers"] = profile.Workers
	}

	config["profiles"] = map[string]interface{}{
		"test": profileMap,
	}

	data, err := yaml.Marshal(config)
	require.NoError(t, err)
	err = os.WriteFile(configPath, data, 0600)
	require.NoError(t, err)
}

func TestAddWorker_NewWorker(t *testing.T) {
	setupWorkerCRUDTest(t, &Profile{
		LeaderURL: testLeaderURL,
	})

	err := AddWorker("eu-west", &WorkerConfig{URL: "http://eu.example.com"}, false)
	require.NoError(t, err)

	profile, err := GetProfile("test")
	require.NoError(t, err)
	require.NotNil(t, profile.Workers)
	assert.Equal(t, "http://eu.example.com", profile.Workers["eu-west"].URL)
	// First worker auto-becomes default
	assert.Equal(t, "eu-west", profile.DefaultWorker)
}

func TestAddWorker_DuplicateErrors(t *testing.T) {
	setupWorkerCRUDTest(t, &Profile{
		LeaderURL:     testLeaderURL,
		DefaultWorker: "eu-west",
		Workers: map[string]*WorkerConfig{
			"eu-west": {URL: "http://old.example.com"},
		},
	})

	err := AddWorker("eu-west", &WorkerConfig{URL: "http://new.example.com"}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestAddWorker_DuplicateWithForce(t *testing.T) {
	setupWorkerCRUDTest(t, &Profile{
		LeaderURL:     testLeaderURL,
		DefaultWorker: "eu-west",
		Workers: map[string]*WorkerConfig{
			"eu-west": {URL: "http://old.example.com"},
		},
	})

	err := AddWorker("eu-west", &WorkerConfig{URL: "http://new.example.com"}, true)
	require.NoError(t, err)

	profile, err := GetProfile("test")
	require.NoError(t, err)
	assert.Equal(t, "http://new.example.com", profile.Workers["eu-west"].URL)
}

func TestAddWorker_FirstAutoDefault(t *testing.T) {
	setupWorkerCRUDTest(t, &Profile{
		LeaderURL: testLeaderURL,
	})

	err := AddWorker("first", &WorkerConfig{URL: "http://first.example.com"}, false)
	require.NoError(t, err)

	profile, err := GetProfile("test")
	require.NoError(t, err)
	assert.Equal(t, "first", profile.DefaultWorker)
}

func TestAddWorker_SecondNotAutoDefault(t *testing.T) {
	setupWorkerCRUDTest(t, &Profile{
		LeaderURL:     testLeaderURL,
		DefaultWorker: "first",
		Workers: map[string]*WorkerConfig{
			"first": {URL: "http://first.example.com"},
		},
	})

	err := AddWorker("second", &WorkerConfig{URL: "http://second.example.com"}, false)
	require.NoError(t, err)

	profile, err := GetProfile("test")
	require.NoError(t, err)
	assert.Equal(t, "first", profile.DefaultWorker, "Default should remain 'first'")
}

func TestDeleteWorker_Existing(t *testing.T) {
	setupWorkerCRUDTest(t, &Profile{
		LeaderURL:     testLeaderURL,
		DefaultWorker: "eu-west",
		Workers: map[string]*WorkerConfig{
			"eu-west": {URL: "http://eu.example.com"},
			"us-east": {URL: "http://us.example.com"},
		},
	})

	err := DeleteWorker("us-east")
	require.NoError(t, err)

	profile, err := GetProfile("test")
	require.NoError(t, err)
	_, exists := profile.Workers["us-east"]
	assert.False(t, exists)
	assert.Equal(t, "eu-west", profile.DefaultWorker, "Non-default worker deletion should not affect default")
}

func TestDeleteWorker_NonExistent(t *testing.T) {
	setupWorkerCRUDTest(t, &Profile{
		LeaderURL:     testLeaderURL,
		DefaultWorker: "eu-west",
		Workers: map[string]*WorkerConfig{
			"eu-west": {URL: "http://eu.example.com"},
		},
	})

	err := DeleteWorker("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDeleteWorker_DefaultClearsDefault(t *testing.T) {
	setupWorkerCRUDTest(t, &Profile{
		LeaderURL:     testLeaderURL,
		DefaultWorker: "eu-west",
		Workers: map[string]*WorkerConfig{
			"eu-west": {URL: "http://eu.example.com"},
		},
	})

	err := DeleteWorker("eu-west")
	require.NoError(t, err)

	profile, err := GetProfile("test")
	require.NoError(t, err)
	assert.Empty(t, profile.DefaultWorker, "Deleting default worker should clear default-worker")
}

func TestSetDefaultWorker_Existing(t *testing.T) {
	setupWorkerCRUDTest(t, &Profile{
		LeaderURL:     testLeaderURL,
		DefaultWorker: "eu-west",
		Workers: map[string]*WorkerConfig{
			"eu-west": {URL: "http://eu.example.com"},
			"us-east": {URL: "http://us.example.com"},
		},
	})

	err := SetDefaultWorker("us-east")
	require.NoError(t, err)

	profile, err := GetProfile("test")
	require.NoError(t, err)
	assert.Equal(t, "us-east", profile.DefaultWorker)
}

func TestSetDefaultWorker_NonExistent(t *testing.T) {
	setupWorkerCRUDTest(t, &Profile{
		LeaderURL:     testLeaderURL,
		DefaultWorker: "eu-west",
		Workers: map[string]*WorkerConfig{
			"eu-west": {URL: "http://eu.example.com"},
		},
	})

	err := SetDefaultWorker("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
