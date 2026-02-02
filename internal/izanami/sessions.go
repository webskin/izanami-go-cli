package izanami

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/webskin/izanami-go-cli/internal/errors"
	"gopkg.in/yaml.v3"
)

// Auth method constants for session tracking
const (
	AuthMethodPassword = "password"
	AuthMethodOIDC     = "oidc"
)

// Session represents a saved authentication session
type Session struct {
	URL        string    `yaml:"url"`
	Username   string    `yaml:"username"`              // Stored for display purposes only (not sent to server for JWT auth)
	JwtToken   string    `yaml:"jwtToken"`              // JWT token cookie value for admin authentication
	AuthMethod string    `yaml:"auth_method,omitempty"` // "password" or "oidc"; empty = "password" (backward compat)
	CreatedAt  time.Time `yaml:"created_at"`
}

// IsOIDC returns true if this session was created via OIDC authentication
func (s *Session) IsOIDC() bool {
	return s.AuthMethod == AuthMethodOIDC
}

// Sessions represents the sessions file structure
type Sessions struct {
	Sessions map[string]*Session `yaml:"sessions"`
}

// getSessionsPath is a variable that returns the path to the sessions file
// It's a variable (not a function) to allow tests to override it
var getSessionsPath = func() string {
	var sessionsPath string

	switch runtime.GOOS {
	case "windows":
		sessionsPath = filepath.Join(os.Getenv("USERPROFILE"), ".izsessions")
	default: // linux, darwin, etc.
		sessionsPath = filepath.Join(os.Getenv("HOME"), ".izsessions")
	}

	return sessionsPath
}

// GetSessionsPath returns the path to the sessions file
func GetSessionsPath() string {
	return getSessionsPath()
}

// SetGetSessionsPathFunc allows tests to override the sessions path resolution
func SetGetSessionsPathFunc(fn func() string) {
	getSessionsPath = fn
}

// LoadSessions loads sessions from the sessions file
func LoadSessions() (*Sessions, error) {
	sessionsPath := GetSessionsPath()

	data, err := os.ReadFile(sessionsPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty sessions if file doesn't exist
			return &Sessions{
				Sessions: make(map[string]*Session),
			}, nil
		}
		return nil, fmt.Errorf("%s: %w", errors.MsgFailedToReadSessionsFile, err)
	}

	var sessions Sessions
	if err := yaml.Unmarshal(data, &sessions); err != nil {
		return nil, fmt.Errorf("%s: %w", errors.MsgFailedToParseSessionsFile, err)
	}

	if sessions.Sessions == nil {
		sessions.Sessions = make(map[string]*Session)
	}

	return &sessions, nil
}

// SaveSessions saves sessions to the sessions file
func (s *Sessions) Save() error {
	sessionsPath := GetSessionsPath()

	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("%s: %w", errors.MsgFailedToMarshalSessions, err)
	}

	// Create with restricted permissions (600)
	if err := os.WriteFile(sessionsPath, data, 0600); err != nil {
		return fmt.Errorf("%s: %w", errors.MsgFailedToWriteSessionsFile, err)
	}

	return nil
}

// AddSession adds or updates a session
func (s *Sessions) AddSession(name string, session *Session) {
	if s.Sessions == nil {
		s.Sessions = make(map[string]*Session)
	}
	s.Sessions[name] = session
}

// GetSession returns a specific session by name
func (s *Sessions) GetSession(name string) (*Session, error) {
	session, ok := s.Sessions[name]
	if !ok {
		return nil, fmt.Errorf(errors.MsgSessionNotFound, name)
	}
	return session, nil
}

// DeleteSession removes a session
func (s *Sessions) DeleteSession(name string) error {
	if _, ok := s.Sessions[name]; !ok {
		return fmt.Errorf(errors.MsgSessionNotFound, name)
	}

	delete(s.Sessions, name)

	return nil
}

// IsTokenExpired checks if a token is likely expired
// JWT tokens contain expiry info, but for simplicity we check age
func (s *Session) IsTokenExpired(maxAge time.Duration) bool {
	if maxAge == 0 {
		maxAge = 24 * time.Hour // Default: tokens valid for 24 hours
	}
	return time.Since(s.CreatedAt) > maxAge
}

// LoadConfigFromSession loads config from a specific session
func LoadConfigFromSession(sessionName string) (*Config, error) {
	sessions, err := LoadSessions()
	if err != nil {
		return nil, err
	}

	session, err := sessions.GetSession(sessionName)
	if err != nil {
		return nil, err
	}

	// Validate that the session has a valid JWT token
	if session.JwtToken == "" {
		return nil, fmt.Errorf("session '%s' has no JWT token (session may be invalid)", sessionName)
	}

	// Load config from file first (to get client-keys and other settings)
	config, err := LoadConfig()
	if err != nil {
		// If config file doesn't exist, create a minimal config
		config = &Config{Timeout: 30}
	}

	// Override with session data (session takes precedence for auth)
	config.BaseURL = session.URL
	config.Username = session.Username
	config.JwtToken = session.JwtToken

	return config, nil
}
