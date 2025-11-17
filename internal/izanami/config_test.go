package izanami

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestClientKeysConfigMarshaling tests YAML marshaling and unmarshaling of ClientKeys
func TestClientKeysConfigMarshaling(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "tenant-level credentials only",
			config: Config{
				BaseURL: "http://localhost:9000",
				ClientKeys: map[string]TenantClientKeysConfig{
					"tenant1": {
						ClientID:     "client-id-1",
						ClientSecret: "client-secret-1",
					},
				},
			},
		},
		{
			name: "project-level credentials only",
			config: Config{
				BaseURL: "http://localhost:9000",
				ClientKeys: map[string]TenantClientKeysConfig{
					"tenant1": {
						Projects: map[string]ProjectClientKeysConfig{
							"project1": {
								ClientID:     "proj1-client-id",
								ClientSecret: "proj1-client-secret",
							},
							"project2": {
								ClientID:     "proj2-client-id",
								ClientSecret: "proj2-client-secret",
							},
						},
					},
				},
			},
		},
		{
			name: "both tenant and project level credentials",
			config: Config{
				BaseURL: "http://localhost:9000",
				ClientKeys: map[string]TenantClientKeysConfig{
					"tenant1": {
						ClientID:     "tenant1-client-id",
						ClientSecret: "tenant1-client-secret",
						Projects: map[string]ProjectClientKeysConfig{
							"project1": {
								ClientID:     "proj1-client-id",
								ClientSecret: "proj1-client-secret",
							},
						},
					},
					"tenant2": {
						ClientID:     "tenant2-client-id",
						ClientSecret: "tenant2-client-secret",
					},
				},
			},
		},
		{
			name: "empty client keys",
			config: Config{
				BaseURL:    "http://localhost:9000",
				ClientKeys: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to YAML
			data, err := yaml.Marshal(&tt.config)
			require.NoError(t, err, "Failed to marshal config to YAML")

			// Unmarshal back
			var unmarshaledConfig Config
			err = yaml.Unmarshal(data, &unmarshaledConfig)
			require.NoError(t, err, "Failed to unmarshal YAML to config")

			// Compare
			assert.Equal(t, tt.config.BaseURL, unmarshaledConfig.BaseURL)
			assert.Equal(t, tt.config.ClientKeys, unmarshaledConfig.ClientKeys)
		})
	}
}

// TestResolveClientCredentials tests the credential resolution logic
func TestResolveClientCredentials(t *testing.T) {
	tests := []struct {
		name             string
		config           Config
		tenant           string
		projects         []string
		expectedClientID string
		expectedSecret   string
	}{
		{
			name: "tenant-level credentials",
			config: Config{
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
			config: Config{
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
			config: Config{
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
			config: Config{
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
			config: Config{
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
			config:           Config{},
			tenant:           "tenant1",
			projects:         nil,
			expectedClientID: "",
			expectedSecret:   "",
		},
		{
			name: "empty tenant",
			config: Config{
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
			config: Config{
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
			config: Config{
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

// TestAddClientKeys tests saving client keys to config file
func TestAddClientKeys(t *testing.T) {
	// Create temporary directory for test configs
	tempDir := t.TempDir()
	originalGetConfigDir := getConfigDir
	defer func() { getConfigDir = originalGetConfigDir }()

	// Override getConfigDir to return temp directory
	getConfigDir = func() string {
		return tempDir
	}

	t.Logf("Using temp dir: %s", tempDir)

	tests := []struct {
		name         string
		tenant       string
		projects     []string
		clientID     string
		clientSecret string
		wantErr      bool
	}{
		{
			name:         "add tenant-level credentials",
			tenant:       "tenant1",
			projects:     nil,
			clientID:     "tenant1-id",
			clientSecret: "tenant1-secret",
			wantErr:      false,
		},
		{
			name:         "add project-level credentials",
			tenant:       "tenant1",
			projects:     []string{"project1", "project2"},
			clientID:     "proj-id",
			clientSecret: "proj-secret",
			wantErr:      false,
		},
		{
			name:         "missing tenant",
			tenant:       "",
			projects:     nil,
			clientID:     "id",
			clientSecret: "secret",
			wantErr:      true,
		},
		{
			name:         "missing client ID",
			tenant:       "tenant1",
			projects:     nil,
			clientID:     "",
			clientSecret: "secret",
			wantErr:      true,
		},
		{
			name:         "missing client secret",
			tenant:       "tenant1",
			projects:     nil,
			clientID:     "id",
			clientSecret: "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up before each test
			configPath := filepath.Join(tempDir, "config.yaml")
			os.Remove(configPath)

			err := AddClientKeys(tt.tenant, tt.projects, tt.clientID, tt.clientSecret)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Verify file was created with correct permissions
			info, err := os.Stat(configPath)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0600), info.Mode().Perm(), "Config file should have 0600 permissions")

			// Debug: Read and log the YAML content
			yamlContent, _ := os.ReadFile(configPath)
			t.Logf("YAML content:\n%s", string(yamlContent))

			// Load and verify the saved config
			config, err := LoadConfig()
			require.NoError(t, err)
			t.Logf("Loaded ClientKeys: %+v", config.ClientKeys)
			require.NotNil(t, config.ClientKeys)

			if len(tt.projects) == 0 {
				// Verify tenant-level credentials
				tenantConfig, exists := config.ClientKeys[tt.tenant]
				require.True(t, exists, "Tenant config should exist")
				assert.Equal(t, tt.clientID, tenantConfig.ClientID)
				assert.Equal(t, tt.clientSecret, tenantConfig.ClientSecret)
			} else {
				// Verify project-level credentials
				tenantConfig, exists := config.ClientKeys[tt.tenant]
				require.True(t, exists, "Tenant config should exist")
				require.NotNil(t, tenantConfig.Projects, "Projects map should exist")

				for _, project := range tt.projects {
					projectConfig, exists := tenantConfig.Projects[project]
					require.True(t, exists, "Project config should exist for %s", project)
					assert.Equal(t, tt.clientID, projectConfig.ClientID)
					assert.Equal(t, tt.clientSecret, projectConfig.ClientSecret)
				}
			}
		})
	}
}

// TestAddClientKeysOverwrite tests overwriting existing credentials
func TestAddClientKeysOverwrite(t *testing.T) {
	tempDir := t.TempDir()
	originalGetConfigDir := getConfigDir
	defer func() { getConfigDir = originalGetConfigDir }()

	getConfigDir = func() string {
		return tempDir
	}

	// Add initial credentials
	err := AddClientKeys("tenant1", nil, "old-id", "old-secret")
	require.NoError(t, err)

	// Verify initial credentials
	config, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "old-id", config.ClientKeys["tenant1"].ClientID)

	// Overwrite with new credentials
	err = AddClientKeys("tenant1", nil, "new-id", "new-secret")
	require.NoError(t, err)

	// Verify credentials were updated
	config, err = LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "new-id", config.ClientKeys["tenant1"].ClientID)
	assert.Equal(t, "new-secret", config.ClientKeys["tenant1"].ClientSecret)
}

// TestBackwardCompatibility tests that configs without ClientKeys still load
func TestBackwardCompatibility(t *testing.T) {
	tempDir := t.TempDir()
	originalGetConfigDir := getConfigDir
	defer func() { getConfigDir = originalGetConfigDir }()

	getConfigDir = func() string {
		return tempDir
	}

	// Create a config file without client-keys section (old format)
	configPath := filepath.Join(tempDir, "config.yaml")
	oldConfigYAML := `base-url: http://localhost:9000
client-id: test-client
client-secret: test-secret
tenant: my-tenant
timeout: 30
`
	err := os.WriteFile(configPath, []byte(oldConfigYAML), 0600)
	require.NoError(t, err)

	// Load config - should not error
	config, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:9000", config.BaseURL)
	assert.Equal(t, "test-client", config.ClientID)
	assert.Equal(t, "test-secret", config.ClientSecret)
	assert.Equal(t, "my-tenant", config.Tenant)
	assert.Nil(t, config.ClientKeys, "ClientKeys should be nil for old config format")
}

// TestAddClientKeysMultipleTenants tests adding credentials for multiple tenants
func TestAddClientKeysMultipleTenants(t *testing.T) {
	tempDir := t.TempDir()
	originalGetConfigDir := getConfigDir
	defer func() { getConfigDir = originalGetConfigDir }()

	getConfigDir = func() string {
		return tempDir
	}

	// Add credentials for tenant1
	err := AddClientKeys("tenant1", nil, "tenant1-id", "tenant1-secret")
	require.NoError(t, err)

	// Add credentials for tenant2
	err = AddClientKeys("tenant2", nil, "tenant2-id", "tenant2-secret")
	require.NoError(t, err)

	// Verify both tenants have credentials
	config, err := LoadConfig()
	require.NoError(t, err)
	require.NotNil(t, config.ClientKeys)
	assert.Equal(t, 2, len(config.ClientKeys))
	assert.Equal(t, "tenant1-id", config.ClientKeys["tenant1"].ClientID)
	assert.Equal(t, "tenant2-id", config.ClientKeys["tenant2"].ClientID)
}
