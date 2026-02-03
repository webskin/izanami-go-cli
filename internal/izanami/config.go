package izanami

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/spf13/viper"
	"github.com/webskin/izanami-go-cli/internal/errors"
)

// Config key constants
const (
	ConfigKeyLeaderURL                   = "leader-url"
	ConfigKeyClientID                    = "client-id"
	ConfigKeyClientSecret                = "client-secret"
	ConfigKeyPersonalAccessTokenUsername = "personal-access-token-username"
	ConfigKeyJwtToken                    = "jwt-token"
	ConfigKeyPersonalAccessToken         = "personal-access-token"
	ConfigKeyTenant                      = "tenant"
	ConfigKeyProject                     = "project"
	ConfigKeyContext                     = "context"
	ConfigKeyTimeout                     = "timeout"
	ConfigKeyVerbose                     = "verbose"
	ConfigKeyOutputFormat                = "output-format"
	ConfigKeyColor                       = "color"
	ConfigKeyClientKeys                  = "client-keys"
	ConfigKeyProfiles                    = "profiles"
	ConfigKeyDefaultWorker               = "default-worker"
)

// Display constants
const (
	RedactedValue = "<redacted>"
)

// WorkerConfig holds configuration for a named worker instance
type WorkerConfig struct {
	URL        string                            `yaml:"url" mapstructure:"url"`
	ClientKeys map[string]TenantClientKeysConfig `yaml:"client-keys,omitempty" mapstructure:"client-keys"`
}

// Config represents the on-disk config.yaml file structure.
// This is what gets serialized/deserialized from the YAML config file.
// For the resolved runtime state used by commands, see ResolvedConfig.
type Config struct {
	Timeout       int                 `yaml:"timeout" mapstructure:"timeout"`
	Verbose       bool                `yaml:"verbose" mapstructure:"verbose"`
	OutputFormat  string              `yaml:"output-format" mapstructure:"output-format"`
	Color         string              `yaml:"color" mapstructure:"color"`
	ActiveProfile string              `yaml:"active_profile,omitempty" mapstructure:"active_profile"`
	Profiles      map[string]*Profile `yaml:"profiles,omitempty" mapstructure:"profiles"`
}

// ResolvedConfig holds the fully resolved configuration for a CLI invocation.
// It is built from the on-disk Config, merged with profile/session/env/flag values.
// Commands should use this type, not Config.
type ResolvedConfig struct {
	// Global settings (from Config file)
	Timeout      int
	Verbose      bool
	OutputFormat string
	Color        string

	// Resolved from profile/session/flags/env
	LeaderURL                   string
	ClientID                    string
	ClientSecret                string
	Username                    string
	PersonalAccessTokenUsername string
	JwtToken                    string
	PersonalAccessToken         string
	Tenant                      string
	Project                     string
	Context                     string
	ClientKeys                  map[string]TenantClientKeysConfig
	AuthMethod                  string
	InsecureSkipVerify          bool

	// Worker resolution (set by cmd layer after ResolveWorker)
	WorkerURL  string
	WorkerName string
}

// ResolvedWorker holds the result of worker resolution.
type ResolvedWorker struct {
	URL        string
	Name       string
	Source     string // "flag", "env-name", "env-url", "default", "standalone"
	ClientKeys map[string]TenantClientKeysConfig
}

// NewResolvedConfig creates a ResolvedConfig from an on-disk Config,
// copying global settings.
func NewResolvedConfig(fileConfig *Config) *ResolvedConfig {
	return &ResolvedConfig{
		Timeout:      fileConfig.Timeout,
		Verbose:      fileConfig.Verbose,
		OutputFormat: fileConfig.OutputFormat,
		Color:        fileConfig.Color,
	}
}

// TenantClientKeysConfig holds client credentials for a specific tenant
type TenantClientKeysConfig struct {
	ClientID     string                             `yaml:"client-id,omitempty" mapstructure:"client-id"`
	ClientSecret string                             `yaml:"client-secret,omitempty" mapstructure:"client-secret"`
	Projects     map[string]ProjectClientKeysConfig `yaml:"projects,omitempty" mapstructure:"projects"` // Project-specific overrides
}

// ProjectClientKeysConfig holds client credentials for a specific project within a tenant
type ProjectClientKeysConfig struct {
	ClientID     string `yaml:"client-id,omitempty" mapstructure:"client-id"`
	ClientSecret string `yaml:"client-secret,omitempty" mapstructure:"client-secret"`
}

// Profile holds configuration for a specific environment (e.g., local, sandbox, build, prod)
type Profile struct {
	Session                     string                            `yaml:"session,omitempty" mapstructure:"session"`                                               // Reference to session name in ~/.izsessions
	LeaderURL                   string                            `yaml:"leader-url,omitempty" mapstructure:"leader-url"`                                         // Admin API URL
	PersonalAccessTokenUsername string                            `yaml:"personal-access-token-username,omitempty" mapstructure:"personal-access-token-username"` // Username for PAT authentication (required with personal-access-token)
	PersonalAccessToken         string                            `yaml:"personal-access-token,omitempty" mapstructure:"personal-access-token"`                   // Personal Access Token (long-lived)
	Tenant                      string                            `yaml:"tenant,omitempty" mapstructure:"tenant"`                                                 // Default tenant for this profile
	Project                     string                            `yaml:"project,omitempty" mapstructure:"project"`                                               // Default project for this profile
	Context                     string                            `yaml:"context,omitempty" mapstructure:"context"`                                               // Default context for this profile
	ClientKeys                  map[string]TenantClientKeysConfig `yaml:"client-keys,omitempty" mapstructure:"client-keys"`                                       // Profile-specific hierarchical client keys
	InsecureSkipVerify          bool                              `yaml:"insecure-skip-verify,omitempty" mapstructure:"insecure-skip-verify"`                     // Skip TLS certificate verification
	DefaultWorker               string                            `yaml:"default-worker,omitempty" mapstructure:"default-worker"`                                 // Default worker name
	Workers                     map[string]*WorkerConfig          `yaml:"workers,omitempty" mapstructure:"workers"`                                               // Named worker instances
}

// FlagValues holds command-line flag values for merging with config
type FlagValues struct {
	LeaderURL                   string
	ClientID                    string
	ClientSecret                string
	PersonalAccessTokenUsername string
	JwtToken                    string
	PersonalAccessToken         string
	Tenant                      string
	Project                     string
	Context                     string
	Timeout                     int
	Verbose                     bool
	OutputFormat                string
	Color                       string
	InsecureSkipVerify          bool
}

// LoadConfig loads configuration from multiple sources:
// 1. Config file (~/.config/iz/config.yaml or platform-equivalent)
// 2. Environment variables (IZ_*)
// 3. Command-line flags (set by cobra, highest priority)
func LoadConfig() (*Config, error) {
	// Repair file permissions on every load (protects users upgrading from older versions)
	repairConfigPermissions()

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
	v.SetDefault(ConfigKeyTimeout, 30)
	v.SetDefault(ConfigKeyVerbose, false)
	v.SetDefault(ConfigKeyOutputFormat, "table")
	v.SetDefault(ConfigKeyColor, "auto")

	// Read config file if it exists (ignore if not found)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Unmarshal the entire config to properly handle nested structures
	config := &Config{}
	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return config, nil
}

// MergeWithFlags merges configuration with command-line flags
func (c *ResolvedConfig) MergeWithFlags(flags FlagValues) {
	if flags.LeaderURL != "" {
		c.LeaderURL = flags.LeaderURL
	}
	if flags.ClientID != "" {
		c.ClientID = flags.ClientID
	}
	if flags.ClientSecret != "" {
		c.ClientSecret = flags.ClientSecret
	}
	if flags.PersonalAccessTokenUsername != "" {
		c.PersonalAccessTokenUsername = flags.PersonalAccessTokenUsername
	}
	if flags.JwtToken != "" {
		c.JwtToken = flags.JwtToken
	}
	if flags.PersonalAccessToken != "" {
		c.PersonalAccessToken = flags.PersonalAccessToken
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
	if flags.OutputFormat != "" {
		c.OutputFormat = flags.OutputFormat
	}
	if flags.Color != "" {
		c.Color = flags.Color
	}
	if flags.InsecureSkipVerify {
		c.InsecureSkipVerify = flags.InsecureSkipVerify
	}
}

// Validate checks if required configuration is present
func (c *ResolvedConfig) Validate() error {
	if c.LeaderURL == "" {
		return fmt.Errorf("leader URL is required (set IZ_LEADER_URL or --url)")
	}

	// Personal access token requires username
	if c.PersonalAccessToken != "" && c.PersonalAccessTokenUsername == "" {
		return fmt.Errorf("personal-access-token-username is required when using personal access token (set IZ_PERSONAL_ACCESS_TOKEN_USERNAME or --personal-access-token-username)")
	}

	// Check authentication: either client ID/secret, jwt-token, or personal-access-token+username
	hasClientAuth := c.ClientID != "" && c.ClientSecret != ""
	hasUserAuth := c.JwtToken != "" || (c.PersonalAccessTokenUsername != "" && c.PersonalAccessToken != "")

	if !hasClientAuth && !hasUserAuth {
		return fmt.Errorf("authentication required: either client-id/client-secret, jwt-token, or personal-access-token with personal-access-token-username must be set")
	}

	return nil
}

// ValidateAdminAuth checks if admin authentication is configured
func (c *ResolvedConfig) ValidateAdminAuth() error {
	if c.LeaderURL == "" {
		return fmt.Errorf("leader URL is required (set IZ_LEADER_URL or --url)")
	}

	// Check authentication: PAT token OR JWT token (username not required for JWT)
	hasPatAuth := c.PersonalAccessToken != ""
	hasJwtAuth := c.JwtToken != ""

	if !hasPatAuth && !hasJwtAuth {
		return fmt.Errorf("admin operations require authentication: use 'iz login' for JWT, or set IZ_JWT_TOKEN, or set IZ_PERSONAL_ACCESS_TOKEN (with IZ_PERSONAL_ACCESS_TOKEN_USERNAME)")
	}

	// If using PAT, username is required (for Basic auth)
	if hasPatAuth && c.PersonalAccessTokenUsername == "" {
		return fmt.Errorf("personal-access-token-username required when using personal access token (set IZ_PERSONAL_ACCESS_TOKEN_USERNAME or --personal-access-token-username)")
	}

	return nil
}

// ValidateTenant checks if a tenant is configured (required for most operations)
func (c *ResolvedConfig) ValidateTenant() error {
	if c.Tenant == "" {
		return fmt.Errorf(errors.MsgTenantRequired)
	}
	return nil
}

// ValidateClientAuth checks if client authentication (client-id/secret) is configured.
// This is required for client operations (feature checks, events).
// Uses WorkerURL if resolved, otherwise falls back to LeaderURL.
// The caller is responsible for applying credential precedence (flags > worker > env > client-keys > profile)
// before calling this method.
func (c *ResolvedConfig) ValidateClientAuth() error {
	baseURL := c.GetWorkerURL()
	if baseURL == "" {
		return fmt.Errorf("URL is required: set IZ_LEADER_URL, IZ_WORKER_URL, or configure a worker")
	}

	if c.ClientID == "" || c.ClientSecret == "" {
		return fmt.Errorf("client credentials required: set IZ_CLIENT_ID and IZ_CLIENT_SECRET, or configure client-keys in your profile")
	}

	return nil
}

// getConfigDir is a variable that returns the platform-specific config directory
// It's a variable (not a function) to allow tests to override it
var getConfigDir = func() string {
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

// SetGetConfigDirFunc allows tests to override the config directory resolution
func SetGetConfigDirFunc(fn func() string) {
	getConfigDir = fn
}

// GetConfigDir returns the platform-specific config directory
func GetConfigDir() string {
	return getConfigDir()
}

// InitConfigFile creates a sample config file at the default location
func InitConfigFile() error {
	configDir := getConfigDir()
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf(errors.MsgFailedToCreateConfigDir, err)
	}

	configPath := filepath.Join(configDir, "config.yaml")
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("config file already exists at %s", configPath)
	}

	sampleConfig := `# Izanami CLI Configuration
# Configuration is organized by profiles for different environments

# Global settings (apply to all profiles unless overridden)
timeout: 30
verbose: false
output-format: table
color: auto

# Profiles for different environments
# Use 'iz profile' commands to manage profiles, or edit this file directly
# Example:
#
# profiles:
#   active: sandbox
#   profiles:
#     sandbox:
#       leader-url: "http://localhost:9000"
#       tenant: "sandbox-tenant"
#       project: "test"
#       context: "dev"
#       # Optional: client credentials for feature checks
#       # client-id: "your-client-id"
#       # client-secret: "your-client-secret"
#
#     prod:
#       # Option 1: Reference an existing login session (for JWT auth)
#       session: "prod-session"
#       # Option 2: Specify URL directly with Personal Access Token (long-lived)
#       # leader-url: "https://izanami.example.com"
#       # username: "your-username"
#       # personal-access-token: "your-pat-token"
#       tenant: "production"
#       project: "main"
#       context: "prod"
#
# Create profiles with: iz profile add <name> or iz profile init <template>
`

	if err := os.WriteFile(configPath, []byte(sampleConfig), 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Create .gitignore to prevent accidental commits
	gitignorePath := filepath.Join(configDir, ".gitignore")
	gitignoreContent := `# Izanami CLI - Do not commit credentials
config.yaml
*.yaml
`
	// Ignore error - .gitignore is optional
	os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644)

	return nil
}

// ConfigValue represents a configuration value with its source
type ConfigValue struct {
	Value  string
	Source string // "file", "env", "default", or "not set"
}

// GlobalConfigKeys defines keys that can be set via 'iz config set'
// These are stored in the top-level config.yaml and apply to all profiles
var GlobalConfigKeys = map[string]bool{
	ConfigKeyTimeout:      true,
	ConfigKeyVerbose:      true,
	ConfigKeyOutputFormat: true,
	ConfigKeyColor:        true,
}

// ProfileConfigKeys defines keys that are profile-specific
// These should be set via 'iz profiles set' or 'iz profiles add'
var ProfileConfigKeys = map[string]bool{
	ConfigKeyLeaderURL:                   true,
	ConfigKeyPersonalAccessTokenUsername: true,
	ConfigKeyJwtToken:                    true,
	ConfigKeyPersonalAccessToken:         true,
	ConfigKeyTenant:                      true,
	ConfigKeyProject:                     true,
	ConfigKeyContext:                     true,
	ConfigKeyClientKeys:                  true,
	ConfigKeyDefaultWorker:               true,
}

// ValidConfigKeys defines all valid configuration keys (for reading/listing)
var ValidConfigKeys = map[string]bool{
	ConfigKeyLeaderURL:                   true,
	ConfigKeyPersonalAccessTokenUsername: true,
	ConfigKeyJwtToken:                    true,
	ConfigKeyPersonalAccessToken:         true,
	ConfigKeyTenant:                      true,
	ConfigKeyProject:                     true,
	ConfigKeyContext:                     true,
	ConfigKeyTimeout:                     true,
	ConfigKeyVerbose:                     true,
	ConfigKeyOutputFormat:                true,
	ConfigKeyColor:                       true,
	ConfigKeyClientKeys:                  true,
	ConfigKeyProfiles:                    true,
	ConfigKeyDefaultWorker:               true,
}

// SensitiveKeys defines which keys contain sensitive information
var SensitiveKeys = map[string]bool{
	ConfigKeyJwtToken:            true,
	ConfigKeyPersonalAccessToken: true,
	ConfigKeyClientKeys:          true, // Contains client secrets
	ConfigKeyProfiles:            true, // Contains client secrets and credentials
}

// GetConfigPath returns the path to the config file
func GetConfigPath() string {
	return filepath.Join(getConfigDir(), "config.yaml")
}

// GetConfigDirPath returns the path to the config directory
func GetConfigDirPath() string {
	return getConfigDir()
}

// ConfigExists checks if the config file exists
func ConfigExists() bool {
	_, err := os.Stat(GetConfigPath())
	return err == nil
}

// repairConfigPermissions ensures config files have secure permissions
// This is called on every config load to protect users who upgrade from older versions
func repairConfigPermissions() {
	configDir := getConfigDir()
	configPath := GetConfigPath()

	// Fix directory permissions (should be 0700 - owner only)
	if info, err := os.Stat(configDir); err == nil {
		currentPerms := info.Mode().Perm()
		if currentPerms != 0700 {
			os.Chmod(configDir, 0700)
		}
	}

	// Fix config file permissions (should be 0600 - owner read/write only)
	if info, err := os.Stat(configPath); err == nil {
		currentPerms := info.Mode().Perm()
		if currentPerms != 0600 {
			os.Chmod(configPath, 0600)
		}
	}
}

// GetConfigValue gets a single configuration value with its source
func GetConfigValue(key string) (*ConfigValue, error) {
	if !ValidConfigKeys[key] {
		return nil, fmt.Errorf(errors.MsgInvalidConfigKey, key)
	}

	// Repair permissions before reading
	repairConfigPermissions()

	v := viper.New()
	configDir := getConfigDir()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(configDir)
	v.SetEnvPrefix("IZ")
	v.AutomaticEnv()

	// Set defaults
	v.SetDefault(ConfigKeyTimeout, 30)
	v.SetDefault(ConfigKeyVerbose, false)
	v.SetDefault(ConfigKeyOutputFormat, "table")
	v.SetDefault(ConfigKeyColor, "auto")

	// Read config file if it exists
	fileExists := false
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	} else {
		fileExists = true
	}

	// Determine source
	value := v.GetString(key)
	var source string

	// Check if value is from environment
	envKey := "IZ_" + convertToEnvKey(key)
	if os.Getenv(envKey) != "" {
		source = "env"
	} else if fileExists && v.InConfig(key) {
		source = "file"
	} else if value != "" {
		source = "default"
	} else {
		source = "not set"
	}

	return &ConfigValue{
		Value:  value,
		Source: source,
	}, nil
}

// convertToEnvKey converts a kebab-case key to UPPER_SNAKE_CASE for env vars
func convertToEnvKey(key string) string {
	// Replace hyphens with underscores and convert to uppercase
	result := ""
	for _, c := range key {
		if c == '-' {
			result += "_"
		} else {
			result += string(c)
		}
	}
	// Convert to uppercase
	return fmt.Sprintf("%s", result)
}

// SetConfigValue sets a global configuration value and persists it to the config file
// Only global keys (timeout, verbose, output-format, color) can be set via this function.
// Profile-specific keys should be set via profile commands.
func SetConfigValue(key, value string) error {
	if !GlobalConfigKeys[key] {
		if ProfileConfigKeys[key] {
			return fmt.Errorf("'%s' is a profile-specific setting. Use 'iz profiles set %s <value>' instead", key, key)
		}
		return fmt.Errorf(errors.MsgInvalidConfigKey, key)
	}

	configPath := GetConfigPath()
	configDir := getConfigDir()

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf(errors.MsgFailedToCreateConfigDir, err)
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// Read existing config if it exists
	if _, err := os.Stat(configPath); err == nil {
		if err := v.ReadInConfig(); err != nil {
			return fmt.Errorf(errors.MsgFailedToReadConfigFile, err)
		}
	}

	// Set the value
	v.Set(key, value)

	// Write back to file
	if err := v.WriteConfig(); err != nil {
		// If config doesn't exist, create it
		if err := v.SafeWriteConfig(); err != nil {
			return fmt.Errorf(errors.MsgFailedToWriteConfigFile, err)
		}
	}

	// Ensure secure file permissions (in case viper created with default perms)
	if err := os.Chmod(configPath, 0600); err != nil {
		return fmt.Errorf("failed to set config file permissions: %w", err)
	}

	return nil
}

// UnsetConfigValue removes a configuration value from the config file
func UnsetConfigValue(key string) error {
	if !ValidConfigKeys[key] {
		return fmt.Errorf(errors.MsgInvalidConfigKey, key)
	}

	configPath := GetConfigPath()

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file does not exist")
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// Read existing config
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf(errors.MsgFailedToReadConfigFile, err)
	}

	// Get all settings
	settings := v.AllSettings()

	// Remove the key
	delete(settings, key)

	// Create a new viper instance and set all values except the removed key
	newV := viper.New()
	newV.SetConfigFile(configPath)
	newV.SetConfigType("yaml")

	for k, val := range settings {
		newV.Set(k, val)
	}

	// Write back to file
	if err := newV.WriteConfig(); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetAllConfigValues returns all configuration values with their sources
func GetAllConfigValues() (map[string]*ConfigValue, error) {
	result := make(map[string]*ConfigValue)

	for key := range ValidConfigKeys {
		value, err := GetConfigValue(key)
		if err != nil {
			return nil, err
		}
		result[key] = value
	}

	return result, nil
}

// ResetConfig deletes the config file
func ResetConfig() error {
	configPath := GetConfigPath()

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file does not exist")
	}

	if err := os.Remove(configPath); err != nil {
		return fmt.Errorf("failed to delete config file: %w", err)
	}

	return nil
}

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

// ValidateConfigFile validates the global configuration file settings.
// Note: Profile-specific settings (leader-url, auth, tenant, etc.) are not validated here
// as they are stored in profiles, not the global config file.
// Use ValidateProfile to validate profile settings.
func ValidateConfigFile() []ValidationError {
	var errs []ValidationError

	fileConfig, err := LoadConfig()
	if err != nil {
		errs = append(errs, ValidationError{
			Field:   "general",
			Message: fmt.Sprintf("Failed to load config: %v", err),
		})
		return errs
	}

	// Validate timeout (must be positive if set)
	if fileConfig.Timeout < 0 {
		errs = append(errs, ValidationError{
			Field:   "timeout",
			Message: "Timeout must be a positive number",
		})
	}

	// Validate output format
	if fileConfig.OutputFormat != "" && fileConfig.OutputFormat != "table" && fileConfig.OutputFormat != "json" {
		errs = append(errs, ValidationError{
			Field:   "output-format",
			Message: "Output format must be 'table' or 'json'",
		})
	}

	// Validate color
	if fileConfig.Color != "" && fileConfig.Color != "auto" && fileConfig.Color != "always" && fileConfig.Color != "never" {
		errs = append(errs, ValidationError{
			Field:   "color",
			Message: "Color must be 'auto', 'always', or 'never'",
		})
	}

	return errs
}

// ResolveClientCredentials looks up client credentials from the config's ClientKeys
// based on the provided tenant and projects. It searches with the following precedence:
// 1. Project-specific credentials (for each project in the list)
// 2. Tenant-wide credentials
// Returns empty strings if no credentials are found for the given tenant/projects.
// Also returns the client base URL if configured at the tenant level.
func (c *ResolvedConfig) ResolveClientCredentials(tenant string, projects []string) (clientID, clientSecret string) {
	return ResolveClientCredentialsFromKeys(c.ClientKeys, tenant, projects)
}

// ResolveClientCredentialsFromKeys looks up client credentials from an arbitrary ClientKeys map.
// Same logic as ResolvedConfig.ResolveClientCredentials but operates on a standalone map.
// Used to resolve worker-level keys before falling back to profile-level.
func ResolveClientCredentialsFromKeys(clientKeys map[string]TenantClientKeysConfig, tenant string, projects []string) (clientID, clientSecret string) {
	if clientKeys == nil || tenant == "" {
		return "", ""
	}

	tenantConfig, ok := clientKeys[tenant]
	if !ok {
		return "", ""
	}

	// First, try project-specific credentials
	if len(projects) > 0 && tenantConfig.Projects != nil {
		for _, project := range projects {
			if projectConfig, exists := tenantConfig.Projects[project]; exists {
				if projectConfig.ClientID != "" && projectConfig.ClientSecret != "" {
					return projectConfig.ClientID, projectConfig.ClientSecret
				}
			}
		}
	}

	// Fall back to tenant-wide credentials
	if tenantConfig.ClientID != "" && tenantConfig.ClientSecret != "" {
		return tenantConfig.ClientID, tenantConfig.ClientSecret
	}

	return "", ""
}

// GetWorkerURL returns the URL for client operations (features/events).
// Returns WorkerURL if resolved, otherwise falls back to LeaderURL (standalone mode).
func (c *ResolvedConfig) GetWorkerURL() string {
	if c.WorkerURL != "" {
		return c.WorkerURL
	}
	return c.LeaderURL
}

// ResolveWorker resolves the worker URL and credentials based on priority:
// 1. --worker <name> flag (workerFlag)
// 2. IZ_WORKER env var
// 3. IZ_WORKER_URL env var
// 4. Profile default-worker
// 5. (none) -> standalone mode (use leader-url)
//
// Returns an error if a named worker (from flag or IZ_WORKER) is not found.
// A dangling default-worker emits a warning via stderr and falls back to standalone.
// This is a standalone function that takes the workers map and defaultWorker from the profile.
func ResolveWorker(workerFlag string, workers map[string]*WorkerConfig, defaultWorker string, stderr func(format string, a ...interface{})) (*ResolvedWorker, error) {
	// Priority 1: --worker flag
	if workerFlag != "" {
		return resolveNamedWorker(workerFlag, "flag", workers)
	}

	// Priority 2: IZ_WORKER env var
	if envWorker := os.Getenv("IZ_WORKER"); envWorker != "" {
		return resolveNamedWorker(envWorker, "env-name", workers)
	}

	// Priority 3: IZ_WORKER_URL env var (direct URL, no name lookup)
	if envURL := os.Getenv("IZ_WORKER_URL"); envURL != "" {
		return &ResolvedWorker{
			URL:    envURL,
			Source: "env-url",
		}, nil
	}

	// Priority 4: Profile default-worker
	if defaultWorker != "" {
		worker, ok := workers[defaultWorker]
		if !ok {
			// Default worker references missing worker - warn and fall through to standalone
			if stderr != nil {
				availableNames := WorkerNames(workers)
				if len(availableNames) > 0 {
					stderr("[warning] default-worker '%s' not found in current profile; available: %s; falling back to standalone mode\n",
						defaultWorker, strings.Join(availableNames, ", "))
				} else {
					stderr("[warning] default-worker '%s' not found in current profile; falling back to standalone mode\n",
						defaultWorker)
				}
			}
		} else {
			return &ResolvedWorker{
				URL:        worker.URL,
				Name:       defaultWorker,
				Source:     "default",
				ClientKeys: worker.ClientKeys,
			}, nil
		}
	}

	// Priority 5: standalone mode (use leader-url)
	return &ResolvedWorker{
		Source: "standalone",
	}, nil
}

// resolveNamedWorker looks up a named worker in the workers map.
// Returns an error if the worker is not found.
func resolveNamedWorker(name, source string, workers map[string]*WorkerConfig) (*ResolvedWorker, error) {
	if workers == nil || len(workers) == 0 {
		return nil, fmt.Errorf("worker '%s' not found: no workers configured. Add workers with: iz profiles workers add <name> --url <url>", name)
	}

	worker, ok := workers[name]
	if !ok {
		availableNames := WorkerNames(workers)
		return nil, fmt.Errorf("worker '%s' not found; available workers: %s", name, strings.Join(availableNames, ", "))
	}

	return &ResolvedWorker{
		URL:        worker.URL,
		Name:       name,
		Source:     source,
		ClientKeys: worker.ClientKeys,
	}, nil
}

// WorkerNames returns sorted worker names from a workers map.
func WorkerNames(workers map[string]*WorkerConfig) []string {
	if workers == nil {
		return nil
	}
	names := make([]string, 0, len(workers))
	for name := range workers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// AddClientKeys adds or updates client credentials in the active profile.
// If projects is empty, credentials are stored at the tenant level.
// If projects are specified, credentials are stored for each project.
// Credentials are stored at the tenant or project level for client operations.
func AddClientKeys(tenant string, projects []string, clientID, clientSecret string) error {
	if tenant == "" {
		return fmt.Errorf("tenant is required")
	}
	if clientID == "" || clientSecret == "" {
		return fmt.Errorf("both client-id and client-secret are required")
	}

	// Get active profile name
	profileName, err := GetActiveProfileName()
	if err != nil {
		return err
	}
	if profileName == "" {
		return fmt.Errorf("no active profile. Use 'iz profiles use <name>' to select a profile first")
	}

	// Load the active profile
	profile, err := GetProfile(profileName)
	if err != nil {
		return fmt.Errorf("failed to load active profile: %w", err)
	}

	// Initialize ClientKeys map if nil
	if profile.ClientKeys == nil {
		profile.ClientKeys = make(map[string]TenantClientKeysConfig)
	}

	// Get or create tenant entry
	tenantConfig := profile.ClientKeys[tenant]

	// Store credentials at appropriate level
	if len(projects) == 0 {
		// Tenant-level credentials
		tenantConfig.ClientID = clientID
		tenantConfig.ClientSecret = clientSecret
	} else {
		// Project-level credentials
		if tenantConfig.Projects == nil {
			tenantConfig.Projects = make(map[string]ProjectClientKeysConfig)
		}
		for _, project := range projects {
			tenantConfig.Projects[project] = ProjectClientKeysConfig{
				ClientID:     clientID,
				ClientSecret: clientSecret,
			}
		}
	}

	// Update the profile
	profile.ClientKeys[tenant] = tenantConfig

	// Save the profile back
	if err := AddProfile(profileName, profile); err != nil {
		return fmt.Errorf("failed to save profile: %w", err)
	}

	return nil
}

// GetActiveProfileName returns the name of the currently active profile from the config file
func GetActiveProfileName() (string, error) {
	fileConfig, err := LoadConfig()
	if err != nil {
		return "", err
	}

	if fileConfig.ActiveProfile == "" {
		return "", nil // No active profile
	}

	return fileConfig.ActiveProfile, nil
}

// GetProfile retrieves a specific profile from the config by name
func GetProfile(name string) (*Profile, error) {
	fileConfig, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	if fileConfig.Profiles == nil {
		return nil, fmt.Errorf("no profiles defined")
	}

	profile, exists := fileConfig.Profiles[name]
	if !exists {
		return nil, fmt.Errorf("profile '%s' not found", name)
	}

	return profile, nil
}

// MergeWithProfile merges profile settings into the resolved config.
// Profile settings override top-level config but are overridden by env vars and flags.
// Priority: Direct profile fields > Session data > Config defaults
func (c *ResolvedConfig) MergeWithProfile(profile *Profile) {
	if profile == nil {
		return
	}

	// If profile references a session, load session data first (as fallback)
	var sessionData *Session
	if profile.Session != "" {
		sessions, err := LoadSessions()
		if err == nil {
			sessionData, err = sessions.GetSession(profile.Session)
			if err != nil {
				// Session not found - ignore error and continue with profile fields
				sessionData = nil
			}
		}
	}

	// Merge authentication fields with priority: profile > session > config
	// LeaderURL: prefer profile.LeaderURL, fallback to session.URL
	if profile.LeaderURL != "" && c.LeaderURL == "" {
		c.LeaderURL = profile.LeaderURL
	} else if sessionData != nil && sessionData.URL != "" && c.LeaderURL == "" {
		c.LeaderURL = sessionData.URL
	}

	// Username (display): from session only (for showing who's logged in)
	if sessionData != nil && sessionData.Username != "" && c.Username == "" {
		c.Username = sessionData.Username
	}

	// PersonalAccessTokenUsername: from profile only (for PAT authentication)
	if profile.PersonalAccessTokenUsername != "" && c.PersonalAccessTokenUsername == "" {
		c.PersonalAccessTokenUsername = profile.PersonalAccessTokenUsername
	}

	// JwtToken: ONLY from session (short-lived, not stored in profiles)
	if sessionData != nil && sessionData.JwtToken != "" && c.JwtToken == "" {
		c.JwtToken = sessionData.JwtToken
	}

	// AuthMethod: from session only (to detect OIDC sessions for auto-login)
	if sessionData != nil && sessionData.AuthMethod != "" && c.AuthMethod == "" {
		c.AuthMethod = sessionData.AuthMethod
	}

	// PersonalAccessToken: only from profile (long-lived, not stored in sessions)
	if profile.PersonalAccessToken != "" && c.PersonalAccessToken == "" {
		c.PersonalAccessToken = profile.PersonalAccessToken
	}

	// Merge environment-specific settings (only from profile, not from session)
	if profile.Tenant != "" && c.Tenant == "" {
		c.Tenant = profile.Tenant
	}
	if profile.Project != "" && c.Project == "" {
		c.Project = profile.Project
	}
	if profile.Context != "" && c.Context == "" {
		c.Context = profile.Context
	}
	// InsecureSkipVerify: profile value takes precedence if not already set via flag
	if profile.InsecureSkipVerify && !c.InsecureSkipVerify {
		c.InsecureSkipVerify = profile.InsecureSkipVerify
	}

	// Merge ClientKeys if profile has them and config doesn't
	if profile.ClientKeys != nil && len(profile.ClientKeys) > 0 {
		if c.ClientKeys == nil {
			c.ClientKeys = make(map[string]TenantClientKeysConfig)
		}
		// Merge tenant keys from profile (don't override existing ones)
		for tenant, tenantKeys := range profile.ClientKeys {
			if _, exists := c.ClientKeys[tenant]; !exists {
				c.ClientKeys[tenant] = tenantKeys
			}
		}
	}
}

// LoadConfigWithProfile loads the config and merges with the specified profile.
// Returns the resolved config, the active profile (if any), and any error.
// Priority order:
// 1. Command-line flags (handled by caller via MergeWithFlags)
// 2. Environment variables (handled by viper)
// 3. Profile settings
// 4. Session settings (for auth)
// 5. Top-level config (fallback)
func LoadConfigWithProfile(profileName string) (*ResolvedConfig, *Profile, error) {
	// Load base config first
	fileConfig, err := LoadConfig()
	if err != nil {
		return nil, nil, err
	}

	// Build ResolvedConfig from file config
	resolved := NewResolvedConfig(fileConfig)

	// If no profile name specified, try to use active profile
	if profileName == "" {
		profileName = fileConfig.ActiveProfile
	}

	// If we have a profile name, merge it
	var activeProfile *Profile
	if profileName != "" {
		activeProfile, err = GetProfile(profileName)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load profile '%s': %w", profileName, err)
		}
		resolved.MergeWithProfile(activeProfile)
	}

	return resolved, activeProfile, nil
}

// SetActiveProfile sets the active profile in the config file
func SetActiveProfile(profileName string) error {
	configPath := GetConfigPath()
	configDir := getConfigDir()

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf(errors.MsgFailedToCreateConfigDir, err)
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// Read existing config if it exists
	if _, err := os.Stat(configPath); err == nil {
		if err := v.ReadInConfig(); err != nil {
			return fmt.Errorf(errors.MsgFailedToReadConfigFile, err)
		}
	}

	// Verify the profile exists
	profilesMap := v.GetStringMap(ConfigKeyProfiles)
	if profilesMap == nil {
		profilesMap = make(map[string]interface{})
	}

	if _, exists := profilesMap[profileName]; !exists {
		return fmt.Errorf("profile '%s' does not exist", profileName)
	}

	// Get global settings with defaults
	timeout := v.GetInt("timeout")
	if timeout == 0 {
		timeout = 30
	}
	verbose := v.GetBool("verbose")
	outputFormat := v.GetString("output-format")
	if outputFormat == "" {
		outputFormat = "table"
	}
	colorSetting := v.GetString("color")
	if colorSetting == "" {
		colorSetting = "auto"
	}

	// Write the clean config back to file in correct order
	newV := viper.New()
	newV.SetConfigFile(configPath)
	newV.SetConfigType("yaml")

	// Set values in order: global settings first, then active_profile, then profiles
	newV.Set("timeout", timeout)
	newV.Set("verbose", verbose)
	newV.Set("output-format", outputFormat)
	newV.Set("color", colorSetting)
	newV.Set("active_profile", profileName)
	newV.Set("profiles", profilesMap)

	if err := newV.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf(errors.MsgFailedToWriteConfigFile, err)
	}

	// Ensure secure file permissions
	if err := os.Chmod(configPath, 0600); err != nil {
		return fmt.Errorf("failed to set config file permissions: %w", err)
	}

	return nil
}

// AddProfile adds or updates a profile in the config file
func AddProfile(name string, profile *Profile) error {
	if name == "" {
		return fmt.Errorf("profile name is required")
	}
	if profile == nil {
		return fmt.Errorf("profile data is required")
	}

	configPath := GetConfigPath()
	configDir := getConfigDir()

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf(errors.MsgFailedToCreateConfigDir, err)
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// Read existing config if it exists
	if _, err := os.Stat(configPath); err == nil {
		if err := v.ReadInConfig(); err != nil {
			return fmt.Errorf(errors.MsgFailedToReadConfigFile, err)
		}
	}

	// Get or create profiles map
	profilesMap := v.GetStringMap(ConfigKeyProfiles)
	if profilesMap == nil {
		profilesMap = make(map[string]interface{})
	}

	// Get current active profile
	activeProfile := v.GetString("active_profile")

	// Convert profile to map (exclude JwtToken - it's short-lived and should only be in sessions)
	profileMap := make(map[string]interface{})
	if profile.Session != "" {
		profileMap["session"] = profile.Session
	}
	if profile.LeaderURL != "" {
		profileMap["leader-url"] = profile.LeaderURL
	}
	if profile.PersonalAccessTokenUsername != "" {
		profileMap["personal-access-token-username"] = profile.PersonalAccessTokenUsername
	}
	if profile.PersonalAccessToken != "" {
		profileMap["personal-access-token"] = profile.PersonalAccessToken
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
	if profile.ClientKeys != nil && len(profile.ClientKeys) > 0 {
		profileMap["client-keys"] = profile.ClientKeys
	}
	if profile.InsecureSkipVerify {
		profileMap["insecure-skip-verify"] = profile.InsecureSkipVerify
	}
	if profile.DefaultWorker != "" {
		profileMap["default-worker"] = profile.DefaultWorker
	}
	if profile.Workers != nil && len(profile.Workers) > 0 {
		profileMap["workers"] = profile.Workers
	}

	profilesMap[name] = profileMap

	// If this is the first profile, set it as active
	if len(profilesMap) == 1 {
		activeProfile = name
	}

	// Get global settings with defaults
	timeout := v.GetInt("timeout")
	if timeout == 0 {
		timeout = 30
	}
	verbose := v.GetBool("verbose")
	outputFormat := v.GetString("output-format")
	if outputFormat == "" {
		outputFormat = "table"
	}
	colorSetting := v.GetString("color")
	if colorSetting == "" {
		colorSetting = "auto"
	}

	// Write the clean config back to file in correct order
	newV := viper.New()
	newV.SetConfigFile(configPath)
	newV.SetConfigType("yaml")

	// Set values in order: global settings first, then active_profile, then profiles
	newV.Set("timeout", timeout)
	newV.Set("verbose", verbose)
	newV.Set("output-format", outputFormat)
	newV.Set("color", colorSetting)
	if v.IsSet("active_profile") || activeProfile != "" {
		newV.Set("active_profile", activeProfile)
	}
	newV.Set("profiles", profilesMap)

	if err := newV.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf(errors.MsgFailedToWriteConfigFile, err)
	}

	// Ensure secure file permissions
	if err := os.Chmod(configPath, 0600); err != nil {
		return fmt.Errorf("failed to set config file permissions: %w", err)
	}

	return nil
}

// DeleteProfile removes a profile from the config file
func DeleteProfile(name string) error {
	if name == "" {
		return fmt.Errorf("profile name is required")
	}

	configPath := GetConfigPath()

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file does not exist")
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// Read existing config
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf(errors.MsgFailedToReadConfigFile, err)
	}

	// Get profiles map
	profilesMap := v.GetStringMap(ConfigKeyProfiles)
	if profilesMap == nil {
		return fmt.Errorf("no profiles defined")
	}

	// Check if profile exists
	if _, exists := profilesMap[name]; !exists {
		return fmt.Errorf("profile '%s' not found", name)
	}

	// Remove the profile
	delete(profilesMap, name)

	// Get current active profile
	activeProfile := v.GetString("active_profile")

	// If we deleted the active profile, clear the active setting
	if activeProfile == name {
		activeProfile = ""
	}

	// Get global settings with defaults
	timeout := v.GetInt("timeout")
	if timeout == 0 {
		timeout = 30
	}
	verbose := v.GetBool("verbose")
	outputFormat := v.GetString("output-format")
	if outputFormat == "" {
		outputFormat = "table"
	}
	colorSetting := v.GetString("color")
	if colorSetting == "" {
		colorSetting = "auto"
	}

	// Write the clean config back to file in correct order
	newV := viper.New()
	newV.SetConfigFile(GetConfigPath())
	newV.SetConfigType("yaml")

	// Set values in order: global settings first, then active_profile, then profiles
	newV.Set("timeout", timeout)
	newV.Set("verbose", verbose)
	newV.Set("output-format", outputFormat)
	newV.Set("color", colorSetting)
	if v.IsSet("active_profile") || activeProfile != "" {
		newV.Set("active_profile", activeProfile)
	}
	newV.Set("profiles", profilesMap)

	if err := newV.WriteConfigAs(GetConfigPath()); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// ListProfiles returns all defined profiles
func ListProfiles() (map[string]*Profile, string, error) {
	fileConfig, err := LoadConfig()
	if err != nil {
		return nil, "", err
	}

	if fileConfig.Profiles == nil {
		return make(map[string]*Profile), "", nil
	}

	return fileConfig.Profiles, fileConfig.ActiveProfile, nil
}

// NormalizeURL removes protocol and trailing slashes for URL comparison
func NormalizeURL(url string) string {
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimRight(url, "/")
	return strings.ToLower(url)
}

// FindProfileByLeaderURL finds a profile that matches the given leader URL
// Returns (profileName, profile, nil) if found, ("", nil, nil) if not found
func FindProfileByLeaderURL(leaderURL string) (string, *Profile, error) {
	profiles, _, err := ListProfiles()
	if err != nil {
		return "", nil, err
	}

	// Normalize the search URL
	normalizedSearchURL := NormalizeURL(leaderURL)

	// Search through all profiles
	for name, profile := range profiles {
		profileURL := profile.LeaderURL

		// If profile uses session reference, load session to get URL
		if profileURL == "" && profile.Session != "" {
			sessions, err := LoadSessions()
			if err == nil {
				if session, err := sessions.GetSession(profile.Session); err == nil {
					profileURL = session.URL
				}
			}
		}

		// Compare normalized URLs
		if NormalizeURL(profileURL) == normalizedSearchURL {
			return name, profile, nil
		}
	}

	return "", nil, nil
}

// AddWorker adds a named worker to the active profile.
// If the profile has no workers yet, the first worker becomes the default.
func AddWorker(name string, worker *WorkerConfig, force bool) error {
	if name == "" {
		return fmt.Errorf("worker name is required")
	}
	if worker == nil || worker.URL == "" {
		return fmt.Errorf("worker URL is required")
	}

	profileName, err := GetActiveProfileName()
	if err != nil {
		return err
	}
	if profileName == "" {
		return fmt.Errorf(errors.MsgNoActiveProfileForWorker)
	}

	profile, err := GetProfile(profileName)
	if err != nil {
		return fmt.Errorf("failed to load active profile: %w", err)
	}

	if profile.Workers == nil {
		profile.Workers = make(map[string]*WorkerConfig)
	}

	if _, exists := profile.Workers[name]; exists && !force {
		return fmt.Errorf(errors.MsgWorkerAlreadyExists, name, profileName)
	}

	profile.Workers[name] = worker

	// First worker auto-becomes default
	if profile.DefaultWorker == "" {
		profile.DefaultWorker = name
	}

	return AddProfile(profileName, profile)
}

// DeleteWorker removes a named worker from the active profile.
// If the deleted worker was the default, the default is cleared.
func DeleteWorker(name string) error {
	if name == "" {
		return fmt.Errorf("worker name is required")
	}

	profileName, err := GetActiveProfileName()
	if err != nil {
		return err
	}
	if profileName == "" {
		return fmt.Errorf(errors.MsgNoActiveProfileForWorker)
	}

	profile, err := GetProfile(profileName)
	if err != nil {
		return fmt.Errorf("failed to load active profile: %w", err)
	}

	if profile.Workers == nil {
		return fmt.Errorf(errors.MsgWorkerNotFound, name, profileName)
	}

	if _, exists := profile.Workers[name]; !exists {
		return fmt.Errorf(errors.MsgWorkerNotFound, name, profileName)
	}

	delete(profile.Workers, name)

	// Clear default if it was the deleted worker
	if profile.DefaultWorker == name {
		profile.DefaultWorker = ""
	}

	return AddProfile(profileName, profile)
}

// SetDefaultWorker sets the default worker for the active profile.
func SetDefaultWorker(name string) error {
	if name == "" {
		return fmt.Errorf("worker name is required")
	}

	profileName, err := GetActiveProfileName()
	if err != nil {
		return err
	}
	if profileName == "" {
		return fmt.Errorf(errors.MsgNoActiveProfileForWorker)
	}

	profile, err := GetProfile(profileName)
	if err != nil {
		return fmt.Errorf("failed to load active profile: %w", err)
	}

	if profile.Workers == nil || len(profile.Workers) == 0 {
		return fmt.Errorf(errors.MsgNoWorkersConfigured, profileName)
	}

	if _, exists := profile.Workers[name]; !exists {
		return fmt.Errorf(errors.MsgWorkerNotFound, name, profileName)
	}

	profile.DefaultWorker = name
	return AddProfile(profileName, profile)
}

// AddWorkerClientKeys adds or updates client credentials on a named worker in the active profile.
// If workerName is empty, uses the profile's default worker.
func AddWorkerClientKeys(workerName, tenant string, projects []string, clientID, clientSecret string) error {
	if tenant == "" {
		return fmt.Errorf("tenant is required")
	}
	if clientID == "" || clientSecret == "" {
		return fmt.Errorf("both client-id and client-secret are required")
	}

	profileName, err := GetActiveProfileName()
	if err != nil {
		return err
	}
	if profileName == "" {
		return fmt.Errorf("no active profile. Use 'iz profiles use <name>' to select a profile first")
	}

	profile, err := GetProfile(profileName)
	if err != nil {
		return fmt.Errorf("failed to load active profile: %w", err)
	}

	// Resolve worker name
	if workerName == "" {
		workerName = profile.DefaultWorker
	}
	if workerName == "" {
		return fmt.Errorf("no worker specified and no default-worker set. Use --worker <name> or set a default worker")
	}

	if profile.Workers == nil {
		return fmt.Errorf(errors.MsgWorkerNotFound, workerName, profileName)
	}
	worker, exists := profile.Workers[workerName]
	if !exists {
		return fmt.Errorf(errors.MsgWorkerNotFound, workerName, profileName)
	}

	// Initialize ClientKeys map if nil
	if worker.ClientKeys == nil {
		worker.ClientKeys = make(map[string]TenantClientKeysConfig)
	}

	tenantConfig := worker.ClientKeys[tenant]

	if len(projects) == 0 {
		tenantConfig.ClientID = clientID
		tenantConfig.ClientSecret = clientSecret
	} else {
		if tenantConfig.Projects == nil {
			tenantConfig.Projects = make(map[string]ProjectClientKeysConfig)
		}
		for _, project := range projects {
			tenantConfig.Projects[project] = ProjectClientKeysConfig{
				ClientID:     clientID,
				ClientSecret: clientSecret,
			}
		}
	}

	worker.ClientKeys[tenant] = tenantConfig
	return AddProfile(profileName, profile)
}

// ListWorkerClientKeys returns the ClientKeys map for a named worker in the active profile.
// If workerName is empty, uses the profile's default worker.
func ListWorkerClientKeys(workerName string) (map[string]TenantClientKeysConfig, string, error) {
	profileName, err := GetActiveProfileName()
	if err != nil {
		return nil, "", err
	}
	if profileName == "" {
		return nil, "", fmt.Errorf("no active profile. Use 'iz profiles use <name>' first")
	}

	profile, err := GetProfile(profileName)
	if err != nil {
		return nil, "", err
	}

	if workerName == "" {
		workerName = profile.DefaultWorker
	}
	if workerName == "" {
		return nil, "", fmt.Errorf("no worker specified and no default-worker set. Use --worker <name> or set a default worker")
	}

	if profile.Workers == nil {
		return nil, workerName, fmt.Errorf(errors.MsgWorkerNotFound, workerName, profileName)
	}
	worker, exists := profile.Workers[workerName]
	if !exists {
		return nil, workerName, fmt.Errorf(errors.MsgWorkerNotFound, workerName, profileName)
	}

	return worker.ClientKeys, workerName, nil
}

// DeleteWorkerClientKeys deletes client credentials from a named worker in the active profile.
// If workerName is empty, uses the profile's default worker.
func DeleteWorkerClientKeys(workerName, tenant, project string) error {
	profileName, err := GetActiveProfileName()
	if err != nil {
		return err
	}
	if profileName == "" {
		return fmt.Errorf("no active profile. Use 'iz profiles use <name>' first")
	}

	profile, err := GetProfile(profileName)
	if err != nil {
		return err
	}

	if workerName == "" {
		workerName = profile.DefaultWorker
	}
	if workerName == "" {
		return fmt.Errorf("no worker specified and no default-worker set. Use --worker <name> or set a default worker")
	}

	if profile.Workers == nil {
		return fmt.Errorf(errors.MsgWorkerNotFound, workerName, profileName)
	}
	worker, exists := profile.Workers[workerName]
	if !exists {
		return fmt.Errorf(errors.MsgWorkerNotFound, workerName, profileName)
	}

	if worker.ClientKeys == nil {
		return fmt.Errorf("no client keys configured for worker '%s'", workerName)
	}

	cfg, tenantExists := worker.ClientKeys[tenant]
	if !tenantExists {
		return fmt.Errorf("tenant '%s' not found in worker '%s'", tenant, workerName)
	}

	if project == "" {
		cfg.ClientID = ""
		cfg.ClientSecret = ""
		worker.ClientKeys[tenant] = cfg
	} else {
		if cfg.Projects == nil {
			return fmt.Errorf("project '%s' not found in tenant '%s' for worker '%s'", project, tenant, workerName)
		}
		if _, projExists := cfg.Projects[project]; !projExists {
			return fmt.Errorf("project '%s' not found in tenant '%s' for worker '%s'", project, tenant, workerName)
		}
		delete(cfg.Projects, project)
		worker.ClientKeys[tenant] = cfg
	}

	// Clean up: remove tenant entry if empty
	cfg = worker.ClientKeys[tenant]
	if cfg.ClientID == "" && len(cfg.Projects) == 0 {
		delete(worker.ClientKeys, tenant)
	}

	return AddProfile(profileName, profile)
}
