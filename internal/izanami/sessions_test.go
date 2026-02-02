package izanami

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper: Setup paths for isolated test environment
type testSessionPaths struct {
	sessionsPath string
	configDir    string
	configPath   string
	homeDir      string
}

func setupSessionTestPaths(t *testing.T) *testSessionPaths {
	t.Helper()
	tempDir := t.TempDir()
	return &testSessionPaths{
		homeDir:      tempDir,
		configDir:    filepath.Join(tempDir, ".config", "iz"),
		configPath:   filepath.Join(tempDir, ".config", "iz", "config.yaml"),
		sessionsPath: filepath.Join(tempDir, ".izsessions"),
	}
}

// Test helper: Override path functions for isolated test environment
func overrideSessionPathFunctions(t *testing.T, paths *testSessionPaths) {
	t.Helper()
	originalGetSessionsPath := getSessionsPath
	originalGetConfigDir := getConfigDir

	SetGetSessionsPathFunc(func() string { return paths.sessionsPath })
	SetGetConfigDirFunc(func() string { return paths.configDir })

	t.Cleanup(func() {
		SetGetSessionsPathFunc(originalGetSessionsPath)
		SetGetConfigDirFunc(originalGetConfigDir)
	})
}

// Test helper: Create sessions file with test data
func createTestSessionsFile(t *testing.T, path string, sessions *Sessions) {
	t.Helper()

	// Ensure directory exists
	dir := filepath.Dir(path)
	err := os.MkdirAll(dir, 0755)
	require.NoError(t, err)

	// Save sessions
	originalGetSessionsPath := getSessionsPath
	SetGetSessionsPathFunc(func() string { return path })
	defer SetGetSessionsPathFunc(originalGetSessionsPath)

	err = sessions.Save()
	require.NoError(t, err)
}

// Test helper: Create test config file
func createTestConfigFile(t *testing.T, configPath string, content string) {
	t.Helper()
	dir := filepath.Dir(configPath)
	err := os.MkdirAll(dir, 0700)
	require.NoError(t, err)
	err = os.WriteFile(configPath, []byte(content), 0600)
	require.NoError(t, err)
}

func TestGetSessionsPath(t *testing.T) {
	// Test that GetSessionsPath returns a non-empty path
	path := GetSessionsPath()
	assert.NotEmpty(t, path)
	assert.Contains(t, path, ".izsessions")
}

func TestGetSessionsPath_Override(t *testing.T) {
	originalFunc := getSessionsPath
	defer func() { getSessionsPath = originalFunc }()

	customPath := "/custom/path/.izsessions"
	SetGetSessionsPathFunc(func() string { return customPath })

	assert.Equal(t, customPath, GetSessionsPath())
}

func TestLoadSessions_NoFile(t *testing.T) {
	paths := setupSessionTestPaths(t)
	overrideSessionPathFunctions(t, paths)

	// LoadSessions should return empty sessions when file doesn't exist
	sessions, err := LoadSessions()
	require.NoError(t, err)
	require.NotNil(t, sessions)
	assert.Empty(t, sessions.Sessions)
}

func TestLoadSessions_ValidFile(t *testing.T) {
	paths := setupSessionTestPaths(t)
	overrideSessionPathFunctions(t, paths)

	// Create test sessions
	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	testSessions := &Sessions{
		Sessions: map[string]*Session{
			"test-session": {
				URL:       "http://localhost:9000",
				Username:  "testuser",
				JwtToken:  "test-jwt-token",
				CreatedAt: testTime,
			},
			"another-session": {
				URL:       "https://izanami.example.com",
				Username:  "admin",
				JwtToken:  "admin-jwt-token",
				CreatedAt: testTime.Add(time.Hour),
			},
		},
	}
	createTestSessionsFile(t, paths.sessionsPath, testSessions)

	// Load and verify
	loaded, err := LoadSessions()
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Len(t, loaded.Sessions, 2)

	session := loaded.Sessions["test-session"]
	require.NotNil(t, session)
	assert.Equal(t, "http://localhost:9000", session.URL)
	assert.Equal(t, "testuser", session.Username)
	assert.Equal(t, "test-jwt-token", session.JwtToken)
}

func TestLoadSessions_InvalidYAML(t *testing.T) {
	paths := setupSessionTestPaths(t)
	overrideSessionPathFunctions(t, paths)

	// Write invalid YAML content
	err := os.WriteFile(paths.sessionsPath, []byte("invalid: yaml: content: ["), 0600)
	require.NoError(t, err)

	// Should return error
	_, err = LoadSessions()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse sessions file")
}

func TestLoadSessions_EmptyFile(t *testing.T) {
	paths := setupSessionTestPaths(t)
	overrideSessionPathFunctions(t, paths)

	// Write empty file
	err := os.WriteFile(paths.sessionsPath, []byte(""), 0600)
	require.NoError(t, err)

	// Should return empty sessions without error
	sessions, err := LoadSessions()
	require.NoError(t, err)
	require.NotNil(t, sessions)
	assert.NotNil(t, sessions.Sessions)
}

func TestLoadSessions_NilSessionsMap(t *testing.T) {
	paths := setupSessionTestPaths(t)
	overrideSessionPathFunctions(t, paths)

	// Write YAML with null sessions
	content := "sessions: null\n"
	err := os.WriteFile(paths.sessionsPath, []byte(content), 0600)
	require.NoError(t, err)

	// Should initialize the map
	sessions, err := LoadSessions()
	require.NoError(t, err)
	require.NotNil(t, sessions)
	assert.NotNil(t, sessions.Sessions)
}

func TestSessions_Save(t *testing.T) {
	paths := setupSessionTestPaths(t)
	overrideSessionPathFunctions(t, paths)

	sessions := &Sessions{
		Sessions: map[string]*Session{
			"test": {
				URL:       "http://test.local",
				Username:  "user",
				JwtToken:  "token",
				CreatedAt: time.Now(),
			},
		},
	}

	err := sessions.Save()
	require.NoError(t, err)

	// Verify file was created with correct permissions
	info, err := os.Stat(paths.sessionsPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

	// Verify content can be loaded back
	loaded, err := LoadSessions()
	require.NoError(t, err)
	assert.Len(t, loaded.Sessions, 1)
	assert.Equal(t, "http://test.local", loaded.Sessions["test"].URL)
}

func TestSessions_Save_NilSessionsMap(t *testing.T) {
	paths := setupSessionTestPaths(t)
	overrideSessionPathFunctions(t, paths)

	sessions := &Sessions{
		Sessions: nil,
	}

	// Should not error with nil map
	err := sessions.Save()
	require.NoError(t, err)
}

func TestSessions_AddSession(t *testing.T) {
	sessions := &Sessions{}

	session := &Session{
		URL:       "http://localhost:9000",
		Username:  "testuser",
		JwtToken:  "jwt-token",
		CreatedAt: time.Now(),
	}

	sessions.AddSession("new-session", session)

	assert.NotNil(t, sessions.Sessions)
	assert.Len(t, sessions.Sessions, 1)
	assert.Equal(t, session, sessions.Sessions["new-session"])
}

func TestSessions_AddSession_OverwriteExisting(t *testing.T) {
	sessions := &Sessions{
		Sessions: map[string]*Session{
			"existing": {
				URL:      "http://old.url",
				Username: "olduser",
				JwtToken: "old-token",
			},
		},
	}

	newSession := &Session{
		URL:       "http://new.url",
		Username:  "newuser",
		JwtToken:  "new-token",
		CreatedAt: time.Now(),
	}

	sessions.AddSession("existing", newSession)

	assert.Len(t, sessions.Sessions, 1)
	assert.Equal(t, "http://new.url", sessions.Sessions["existing"].URL)
	assert.Equal(t, "newuser", sessions.Sessions["existing"].Username)
}

func TestSessions_AddSession_NilMap(t *testing.T) {
	sessions := &Sessions{Sessions: nil}

	session := &Session{URL: "http://test"}
	sessions.AddSession("test", session)

	assert.NotNil(t, sessions.Sessions)
	assert.Equal(t, session, sessions.Sessions["test"])
}

func TestSessions_GetSession(t *testing.T) {
	testTime := time.Now()
	sessions := &Sessions{
		Sessions: map[string]*Session{
			"test-session": {
				URL:       "http://localhost:9000",
				Username:  "testuser",
				JwtToken:  "jwt-token",
				CreatedAt: testTime,
			},
		},
	}

	session, err := sessions.GetSession("test-session")
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:9000", session.URL)
	assert.Equal(t, "testuser", session.Username)
	assert.Equal(t, "jwt-token", session.JwtToken)
}

func TestSessions_GetSession_NotFound(t *testing.T) {
	sessions := &Sessions{
		Sessions: map[string]*Session{
			"existing": {URL: "http://test"},
		},
	}

	_, err := sessions.GetSession("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
	assert.Contains(t, err.Error(), "not found")
}

func TestSessions_GetSession_EmptySessions(t *testing.T) {
	sessions := &Sessions{
		Sessions: map[string]*Session{},
	}

	_, err := sessions.GetSession("any")
	assert.Error(t, err)
}

func TestSessions_DeleteSession(t *testing.T) {
	sessions := &Sessions{
		Sessions: map[string]*Session{
			"to-delete": {URL: "http://delete.me"},
			"to-keep":   {URL: "http://keep.me"},
		},
	}

	err := sessions.DeleteSession("to-delete")
	require.NoError(t, err)

	assert.Len(t, sessions.Sessions, 1)
	_, exists := sessions.Sessions["to-delete"]
	assert.False(t, exists)
	_, exists = sessions.Sessions["to-keep"]
	assert.True(t, exists)
}

func TestSessions_DeleteSession_NotFound(t *testing.T) {
	sessions := &Sessions{
		Sessions: map[string]*Session{
			"existing": {URL: "http://test"},
		},
	}

	err := sessions.DeleteSession("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
	assert.Contains(t, err.Error(), "not found")
}

func TestSession_IsTokenExpired(t *testing.T) {
	tests := []struct {
		name      string
		createdAt time.Time
		maxAge    time.Duration
		want      bool
	}{
		{
			name:      "token not expired with default maxAge",
			createdAt: time.Now().Add(-1 * time.Hour),
			maxAge:    0, // default 24 hours
			want:      false,
		},
		{
			name:      "token expired with default maxAge",
			createdAt: time.Now().Add(-25 * time.Hour),
			maxAge:    0, // default 24 hours
			want:      true,
		},
		{
			name:      "token not expired with custom maxAge",
			createdAt: time.Now().Add(-30 * time.Minute),
			maxAge:    1 * time.Hour,
			want:      false,
		},
		{
			name:      "token expired with custom maxAge",
			createdAt: time.Now().Add(-2 * time.Hour),
			maxAge:    1 * time.Hour,
			want:      true,
		},
		{
			name:      "token slightly before expiry",
			createdAt: time.Now().Add(-23*time.Hour - 59*time.Minute),
			maxAge:    0, // default 24 hours
			want:      false,
		},
		{
			name:      "token just past expiry",
			createdAt: time.Now().Add(-24*time.Hour - time.Second),
			maxAge:    0,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				CreatedAt: tt.createdAt,
			}

			got := session.IsTokenExpired(tt.maxAge)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLoadConfigFromSession(t *testing.T) {
	paths := setupSessionTestPaths(t)
	overrideSessionPathFunctions(t, paths)

	// Create test config file
	configContent := `timeout: 60
verbose: false
output-format: json
color: never
`
	createTestConfigFile(t, paths.configPath, configContent)

	// Create test session
	testTime := time.Now()
	testSessions := &Sessions{
		Sessions: map[string]*Session{
			"test-session": {
				URL:       "http://localhost:9000",
				Username:  "testuser",
				JwtToken:  "test-jwt-token",
				CreatedAt: testTime,
			},
		},
	}
	createTestSessionsFile(t, paths.sessionsPath, testSessions)

	// Load config from session
	config, err := LoadConfigFromSession("test-session")
	require.NoError(t, err)
	require.NotNil(t, config)

	// Verify session data overrides
	assert.Equal(t, "http://localhost:9000", config.LeaderURL)
	assert.Equal(t, "testuser", config.Username)
	assert.Equal(t, "test-jwt-token", config.JwtToken)
}

func TestLoadConfigFromSession_NotFound(t *testing.T) {
	paths := setupSessionTestPaths(t)
	overrideSessionPathFunctions(t, paths)

	// Create empty sessions
	testSessions := &Sessions{Sessions: map[string]*Session{}}
	createTestSessionsFile(t, paths.sessionsPath, testSessions)

	_, err := LoadConfigFromSession("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestLoadConfigFromSession_NoJwtToken(t *testing.T) {
	paths := setupSessionTestPaths(t)
	overrideSessionPathFunctions(t, paths)

	// Create session without JWT token
	testSessions := &Sessions{
		Sessions: map[string]*Session{
			"no-jwt": {
				URL:       "http://localhost:9000",
				Username:  "testuser",
				JwtToken:  "", // empty JWT token
				CreatedAt: time.Now(),
			},
		},
	}
	createTestSessionsFile(t, paths.sessionsPath, testSessions)

	_, err := LoadConfigFromSession("no-jwt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no JWT token")
}

func TestLoadConfigFromSession_NoConfigFile(t *testing.T) {
	paths := setupSessionTestPaths(t)
	overrideSessionPathFunctions(t, paths)

	// Create test session (but no config file)
	testSessions := &Sessions{
		Sessions: map[string]*Session{
			"test-session": {
				URL:       "http://localhost:9000",
				Username:  "testuser",
				JwtToken:  "test-jwt-token",
				CreatedAt: time.Now(),
			},
		},
	}
	createTestSessionsFile(t, paths.sessionsPath, testSessions)

	// Should create minimal config with default timeout
	config, err := LoadConfigFromSession("test-session")
	require.NoError(t, err)
	require.NotNil(t, config)
	assert.Equal(t, 30, config.Timeout) // default timeout
	assert.Equal(t, "http://localhost:9000", config.LeaderURL)
}

func TestSession_YAML_Marshaling(t *testing.T) {
	paths := setupSessionTestPaths(t)
	overrideSessionPathFunctions(t, paths)

	testTime := time.Date(2024, 6, 15, 14, 30, 45, 0, time.UTC)
	original := &Sessions{
		Sessions: map[string]*Session{
			"session-1": {
				URL:       "http://localhost:9000",
				Username:  "user1",
				JwtToken:  "token-abc-123",
				CreatedAt: testTime,
			},
			"session-2": {
				URL:       "https://prod.example.com",
				Username:  "admin",
				JwtToken:  "token-xyz-789",
				CreatedAt: testTime.Add(2 * time.Hour),
			},
		},
	}

	// Save sessions
	err := original.Save()
	require.NoError(t, err)

	// Load sessions back
	loaded, err := LoadSessions()
	require.NoError(t, err)

	// Verify all data roundtrips correctly
	assert.Len(t, loaded.Sessions, 2)

	s1 := loaded.Sessions["session-1"]
	require.NotNil(t, s1)
	assert.Equal(t, original.Sessions["session-1"].URL, s1.URL)
	assert.Equal(t, original.Sessions["session-1"].Username, s1.Username)
	assert.Equal(t, original.Sessions["session-1"].JwtToken, s1.JwtToken)
	assert.True(t, original.Sessions["session-1"].CreatedAt.Equal(s1.CreatedAt))

	s2 := loaded.Sessions["session-2"]
	require.NotNil(t, s2)
	assert.Equal(t, original.Sessions["session-2"].URL, s2.URL)
	assert.Equal(t, original.Sessions["session-2"].Username, s2.Username)
}

func TestSessions_FilePermissions(t *testing.T) {
	paths := setupSessionTestPaths(t)
	overrideSessionPathFunctions(t, paths)

	sessions := &Sessions{
		Sessions: map[string]*Session{
			"test": {
				URL:       "http://test",
				JwtToken:  "sensitive-token",
				CreatedAt: time.Now(),
			},
		},
	}

	err := sessions.Save()
	require.NoError(t, err)

	// Verify file has restrictive permissions (0600)
	info, err := os.Stat(paths.sessionsPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm(),
		"Sessions file should have 0600 permissions to protect sensitive tokens")
}

func TestSessions_MultipleOperations(t *testing.T) {
	paths := setupSessionTestPaths(t)
	overrideSessionPathFunctions(t, paths)

	// Test a sequence of operations
	sessions := &Sessions{Sessions: make(map[string]*Session)}

	// Add multiple sessions
	sessions.AddSession("session-1", &Session{
		URL:       "http://url1",
		Username:  "user1",
		JwtToken:  "token1",
		CreatedAt: time.Now(),
	})
	sessions.AddSession("session-2", &Session{
		URL:       "http://url2",
		Username:  "user2",
		JwtToken:  "token2",
		CreatedAt: time.Now(),
	})
	sessions.AddSession("session-3", &Session{
		URL:       "http://url3",
		Username:  "user3",
		JwtToken:  "token3",
		CreatedAt: time.Now(),
	})

	assert.Len(t, sessions.Sessions, 3)

	// Delete one session
	err := sessions.DeleteSession("session-2")
	require.NoError(t, err)
	assert.Len(t, sessions.Sessions, 2)

	// Update existing session
	sessions.AddSession("session-1", &Session{
		URL:       "http://updated-url",
		Username:  "updated-user",
		JwtToken:  "updated-token",
		CreatedAt: time.Now(),
	})

	assert.Len(t, sessions.Sessions, 2)
	assert.Equal(t, "http://updated-url", sessions.Sessions["session-1"].URL)

	// Save and reload
	err = sessions.Save()
	require.NoError(t, err)

	loaded, err := LoadSessions()
	require.NoError(t, err)
	assert.Len(t, loaded.Sessions, 2)
	assert.Equal(t, "http://updated-url", loaded.Sessions["session-1"].URL)
	assert.Equal(t, "http://url3", loaded.Sessions["session-3"].URL)
}

func TestSetGetSessionsPathFunc(t *testing.T) {
	// Save original function
	original := getSessionsPath

	// Test setting a custom function
	customPath := "/custom/sessions/path"
	SetGetSessionsPathFunc(func() string {
		return customPath
	})

	assert.Equal(t, customPath, GetSessionsPath())

	// Restore original
	SetGetSessionsPathFunc(original)
	assert.NotEqual(t, customPath, GetSessionsPath())
}

func TestLoadSessions_ReadPermissionError(t *testing.T) {
	// Skip on Windows where permission handling is different
	if os.Getenv("GOOS") == "windows" {
		t.Skip("Skipping permission test on Windows")
	}

	paths := setupSessionTestPaths(t)
	overrideSessionPathFunctions(t, paths)

	// Create sessions file with no read permissions
	err := os.WriteFile(paths.sessionsPath, []byte("sessions: {}"), 0000)
	require.NoError(t, err)

	// Restore permissions for cleanup
	t.Cleanup(func() {
		os.Chmod(paths.sessionsPath, 0644)
	})

	_, err = LoadSessions()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read sessions file")
}

func TestSessions_EmptySessionName(t *testing.T) {
	sessions := &Sessions{
		Sessions: map[string]*Session{},
	}

	// Adding with empty name should work (it's just a map key)
	session := &Session{URL: "http://test"}
	sessions.AddSession("", session)

	assert.Len(t, sessions.Sessions, 1)

	// Getting with empty name should work
	retrieved, err := sessions.GetSession("")
	require.NoError(t, err)
	assert.Equal(t, session, retrieved)

	// Deleting with empty name should work
	err = sessions.DeleteSession("")
	require.NoError(t, err)
	assert.Empty(t, sessions.Sessions)
}

func TestSessions_SpecialCharactersInName(t *testing.T) {
	paths := setupSessionTestPaths(t)
	overrideSessionPathFunctions(t, paths)

	sessions := &Sessions{Sessions: make(map[string]*Session)}

	// Test various special characters in session names
	specialNames := []string{
		"session-with-dashes",
		"session_with_underscores",
		"session.with.dots",
		"session with spaces",
		"session/with/slashes",
		"session:with:colons",
		"émojis🎉",
		"日本語",
	}

	for _, name := range specialNames {
		sessions.AddSession(name, &Session{
			URL:       "http://" + name,
			JwtToken:  "token-" + name,
			CreatedAt: time.Now(),
		})
	}

	// Save and reload
	err := sessions.Save()
	require.NoError(t, err)

	loaded, err := LoadSessions()
	require.NoError(t, err)

	for _, name := range specialNames {
		session, err := loaded.GetSession(name)
		require.NoError(t, err, "Failed to get session with name: %s", name)
		assert.Equal(t, "http://"+name, session.URL)
	}
}

func TestSession_IsOIDC(t *testing.T) {
	tests := []struct {
		name       string
		authMethod string
		want       bool
	}{
		{
			name:       "oidc auth method returns true",
			authMethod: AuthMethodOIDC,
			want:       true,
		},
		{
			name:       "password auth method returns false",
			authMethod: AuthMethodPassword,
			want:       false,
		},
		{
			name:       "empty auth method returns false (backward compat)",
			authMethod: "",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				AuthMethod: tt.authMethod,
			}
			assert.Equal(t, tt.want, session.IsOIDC())
		})
	}
}
