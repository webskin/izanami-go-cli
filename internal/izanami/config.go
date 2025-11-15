package izanami

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/viper"
)

// Config holds the configuration for the Izanami client
type Config struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	Username     string
	Token        string
	Tenant       string
	Project      string
	Context      string
	Timeout      int
	Verbose      bool
}

// FlagValues holds command-line flag values for merging with config
type FlagValues struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	Username     string
	Token        string
	Tenant       string
	Project      string
	Context      string
	Timeout      int
	Verbose      bool
}

// LoadConfig loads configuration from multiple sources:
// 1. Config file (~/.config/iz/config.yaml or platform-equivalent)
// 2. Environment variables (IZ_*)
// 3. Command-line flags (set by cobra, highest priority)
func LoadConfig() (*Config, error) {
	v := viper.New()

	// Set config file location
	configDir := getConfigDir()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(configDir)
	v.AddConfigPath(".")

	// Set environment variable prefix
	v.SetEnvPrefix("IZ")
	v.AutomaticEnv()

	// Set defaults
	v.SetDefault("timeout", 30)
	v.SetDefault("verbose", false)

	// Read config file if it exists (ignore if not found)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	config := &Config{
		BaseURL:      v.GetString("base_url"),
		ClientID:     v.GetString("client_id"),
		ClientSecret: v.GetString("client_secret"),
		Username:     v.GetString("username"),
		Token:        v.GetString("token"),
		Tenant:       v.GetString("tenant"),
		Project:      v.GetString("project"),
		Context:      v.GetString("context"),
		Timeout:      v.GetInt("timeout"),
		Verbose:      v.GetBool("verbose"),
	}

	return config, nil
}

// MergeWithFlags merges configuration with command-line flags
func (c *Config) MergeWithFlags(flags FlagValues) {
	if flags.BaseURL != "" {
		c.BaseURL = flags.BaseURL
	}
	if flags.ClientID != "" {
		c.ClientID = flags.ClientID
	}
	if flags.ClientSecret != "" {
		c.ClientSecret = flags.ClientSecret
	}
	if flags.Username != "" {
		c.Username = flags.Username
	}
	if flags.Token != "" {
		c.Token = flags.Token
	}
	if flags.Tenant != "" {
		c.Tenant = flags.Tenant
	}
	if flags.Project != "" {
		c.Project = flags.Project
	}
	if flags.Context != "" {
		c.Context = flags.Context
	}
	if flags.Timeout > 0 {
		c.Timeout = flags.Timeout
	}
	if flags.Verbose {
		c.Verbose = flags.Verbose
	}
}

// Validate checks if required configuration is present
func (c *Config) Validate() error {
	if c.BaseURL == "" {
		return fmt.Errorf("base URL is required (set IZ_BASE_URL or --url)")
	}

	// Check authentication: either client ID/secret or username/token
	hasClientAuth := c.ClientID != "" && c.ClientSecret != ""
	hasUserAuth := c.Username != "" && c.Token != ""

	if !hasClientAuth && !hasUserAuth {
		return fmt.Errorf("authentication required: either client_id/client_secret or username/token must be set")
	}

	return nil
}

// ValidateAdminAuth checks if admin authentication is configured
func (c *Config) ValidateAdminAuth() error {
	if c.BaseURL == "" {
		return fmt.Errorf("base URL is required (set IZ_BASE_URL or --url)")
	}

	if c.Username == "" || c.Token == "" {
		return fmt.Errorf("admin operations require username and token (set IZ_USERNAME and IZ_TOKEN)")
	}

	return nil
}

// ValidateTenant checks if a tenant is configured (required for most operations)
func (c *Config) ValidateTenant() error {
	if c.Tenant == "" {
		return fmt.Errorf("tenant is required (set IZ_TENANT or --tenant)")
	}
	return nil
}

// getConfigDir returns the platform-specific config directory
func getConfigDir() string {
	var configDir string

	switch runtime.GOOS {
	case "windows":
		configDir = filepath.Join(os.Getenv("APPDATA"), "iz")
	case "darwin":
		configDir = filepath.Join(os.Getenv("HOME"), ".config", "iz")
	default: // linux and others
		if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
			configDir = filepath.Join(xdgConfig, "iz")
		} else {
			configDir = filepath.Join(os.Getenv("HOME"), ".config", "iz")
		}
	}

	return configDir
}

// InitConfigFile creates a sample config file at the default location
func InitConfigFile() error {
	configDir := getConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "config.yaml")
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("config file already exists at %s", configPath)
	}

	sampleConfig := `# Izanami CLI Configuration
# You can also use environment variables (IZ_*) or command-line flags

# Base URL of your Izanami instance (required)
base_url: "https://izanami.example.com"

# Client authentication (for feature evaluation)
# client_id: "your-client-id"
# client_secret: "your-client-secret"

# Admin authentication (for admin operations)
# username: "your-username"
# token: "your-personal-access-token"

# Default tenant
# tenant: "default"

# Default project
# project: "my-project"

# Default context (e.g., "prod", "dev", "prod/eu/france")
# context: "prod"

# Request timeout in seconds
timeout: 30

# Verbose output
verbose: false
`

	if err := os.WriteFile(configPath, []byte(sampleConfig), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
