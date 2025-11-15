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

// Session represents a saved authentication session
type Session struct {
	URL       string    `yaml:"url"`
	Username  string    `yaml:"username"`
	JwtToken  string    `yaml:"jwtToken"`
	CreatedAt time.Time `yaml:"created_at"`
}

// Sessions represents the sessions file structure
type Sessions struct {
	Active   string              `yaml:"active"`
	Sessions map[string]*Session `yaml:"sessions"`
}

// getSessionsPath returns the path to the sessions file
func getSessionsPath() string {
	var sessionsPath string

	switch runtime.GOOS {
	case "windows":
		sessionsPath = filepath.Join(os.Getenv("USERPROFILE"), ".izsessions")
	default: // linux, darwin, etc.
		sessionsPath = filepath.Join(os.Getenv("HOME"), ".izsessions")
	}

	return sessionsPath
}

// LoadSessions loads sessions from the sessions file
func LoadSessions() (*Sessions, error) {
	sessionsPath := getSessionsPath()

	data, err := os.ReadFile(sessionsPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty sessions if file doesn't exist
			return &Sessions{
				Active:   "",
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
	sessionsPath := getSessionsPath()

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

	// Set as active if it's the first session
	if s.Active == "" {
		s.Active = name
	}
}

// GetActiveSession returns the currently active session
func (s *Sessions) GetActiveSession() (*Session, string, error) {
	if s.Active == "" {
		return nil, "", fmt.Errorf(errors.MsgNoActiveSessionWithLogin)
	}

	session, ok := s.Sessions[s.Active]
	if !ok {
		return nil, "", fmt.Errorf(errors.MsgActiveSessionNotFound, s.Active)
	}

	return session, s.Active, nil
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

	// If we deleted the active session, clear it or set to another
	if s.Active == name {
		s.Active = ""
		// Set to first available session if any exist
		for sessionName := range s.Sessions {
			s.Active = sessionName
			break
		}
	}

	return nil
}

// SetActiveSession sets the active session
func (s *Sessions) SetActiveSession(name string) error {
	if _, ok := s.Sessions[name]; !ok {
		return fmt.Errorf(errors.MsgSessionNotFound, name)
	}
	s.Active = name
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

// LoadConfigFromSession loads config from the active session
func LoadConfigFromSession() (*Config, string, error) {
	sessions, err := LoadSessions()
	if err != nil {
		return nil, "", err
	}

	session, name, err := sessions.GetActiveSession()
	if err != nil {
		return nil, "", err
	}

	config := &Config{
		BaseURL:  session.URL,
		Username: session.Username,
		JwtToken: session.JwtToken,
		Timeout:  30,
	}

	return config, name, nil
}
