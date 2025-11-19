package izanami

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/viper"
	"github.com/webskin/izanami-go-cli/internal/errors"
)

// Config key constants
const (
	ConfigKeyBaseURL                     = "base-url"
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
)

// Display constants
const (
	RedactedValue = "<redacted>"
)

// Config holds the runtime configuration for the Izanami client
// Profile-specific fields are populated from the active profile
type Config struct {
	// Runtime fields (populated from active profile, not stored in top-level YAML)
	BaseURL      string                            `yaml:"-" mapstructure:"-"` // Comes from active profile
	ClientID     string                            `yaml:"-" mapstructure:"-"` // Comes from active profile
	ClientSecret string                            `yaml:"-" mapstructure:"-"` // Comes from active profile
	Username     string                            `yaml:"-" mapstructure:"-"` // Comes from session/profile
	JwtToken     string                            `yaml:"-" mapstructure:"-"` // Comes from session/profile
	PatToken     string                            `yaml:"-" mapstructure:"-"` // Comes from profile
	Tenant       string                            `yaml:"-" mapstructure:"-"` // Comes from active profile
	Project      string                            `yaml:"-" mapstructure:"-"` // Comes from active profile
	Context      string                            `yaml:"-" mapstructure:"-"` // Comes from active profile
	ClientKeys   map[string]TenantClientKeysConfig `yaml:"-" mapstructure:"-"` // Comes from active profile

	// Global settings (stored in top-level YAML)
	Timeout      int    `yaml:"timeout" mapstructure:"timeout"`
	Verbose      bool   `yaml:"verbose" mapstructure:"verbose"`
	OutputFormat string `yaml:"output-format" mapstructure:"output-format"` // Default output format (table/json)
	Color        string `yaml:"color" mapstructure:"color"`                 // Color output (auto/always/never)

	// Profile management
	ActiveProfile string              `yaml:"active_profile,omitempty" mapstructure:"active_profile"` // Currently active profile
	Profiles      map[string]*Profile `yaml:"profiles,omitempty" mapstructure:"profiles"`             // Named environment profiles
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
	Session      string                            `yaml:"session,omitempty" mapstructure:"session"`                                               // Reference to session name in ~/.izsessions
	BaseURL      string                            `yaml:"base-url,omitempty" mapstructure:"base-url"`                                             // Alternative to session
	Username     string                            `yaml:"personal-access-token-username,omitempty" mapstructure:"personal-access-token-username"` // Username for PAT authentication (required with personal-access-token)
	PatToken     string                            `yaml:"personal-access-token,omitempty" mapstructure:"personal-access-token"`                   // Personal Access Token (long-lived)
	Tenant       string                            `yaml:"tenant,omitempty" mapstructure:"tenant"`                                                 // Default tenant for this profile
	Project      string                            `yaml:"project,omitempty" mapstructure:"project"`                                               // Default project for this profile
	Context      string                            `yaml:"context,omitempty" mapstructure:"context"`                                               // Default context for this profile
	ClientID     string                            `yaml:"client-id,omitempty" mapstructure:"client-id"`                                           // Client ID for this profile
	ClientSecret string                            `yaml:"client-secret,omitempty" mapstructure:"client-secret"`                                   // Client secret for this profile
	ClientKeys   map[string]TenantClientKeysConfig `yaml:"client-keys,omitempty" mapstructure:"client-keys"`                                       // Profile-specific hierarchical client keys
}

// FlagValues holds command-line flag values for merging with config
type FlagValues struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	Username     string
	JwtToken     string
	PatToken     string
	Tenant       string
	Project      string
	Context      string
	Timeout      int
	Verbose      bool
	OutputFormat string
	Color        string
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
	if flags.JwtToken != "" {
		c.JwtToken = flags.JwtToken
	}
	if flags.PatToken != "" {
		c.PatToken = flags.PatToken
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
}

// Validate checks if required configuration is present
func (c *Config) Validate() error {
	if c.BaseURL == "" {
		return fmt.Errorf("base URL is required (set IZ_BASE_URL or --url)")
	}

	// Personal access token requires username
	if c.PatToken != "" && c.Username == "" {
		return fmt.Errorf("personal-access-token-username is required when using personal access token (set IZ_PERSONAL_ACCESS_TOKEN_USERNAME or --personal-access-token-username)")
	}

	// Check authentication: either client ID/secret, jwtToken, or patToken+username
	hasClientAuth := c.ClientID != "" && c.ClientSecret != ""
	hasUserAuth := c.JwtToken != "" || (c.Username != "" && c.PatToken != "")

	if !hasClientAuth && !hasUserAuth {
		return fmt.Errorf("authentication required: either client-id/client-secret, jwt-token, or personal-access-token with personal-access-token-username must be set")
	}

	return nil
}

// ValidateAdminAuth checks if admin authentication is configured
func (c *Config) ValidateAdminAuth() error {
	if c.BaseURL == "" {
		return fmt.Errorf("base URL is required (set IZ_BASE_URL or --url)")
	}

	// Check authentication: PAT token OR JWT token (username not required for JWT)
	hasPatAuth := c.PatToken != ""
	hasJwtAuth := c.JwtToken != ""

	if !hasPatAuth && !hasJwtAuth {
		return fmt.Errorf("admin operations require authentication: use 'iz login' for JWT, or set IZ_JWT_TOKEN, or set IZ_PERSONAL_ACCESS_TOKEN (with IZ_PERSONAL_ACCESS_TOKEN_USERNAME)")
	}

	// If using PAT, username is required (for Basic auth)
	if hasPatAuth && c.Username == "" {
		return fmt.Errorf("personal-access-token-username required when using personal access token (set IZ_PERSONAL_ACCESS_TOKEN_USERNAME or --personal-access-token-username)")
	}

	return nil
}

// ValidateTenant checks if a tenant is configured (required for most operations)
func (c *Config) ValidateTenant() error {
	if c.Tenant == "" {
		return fmt.Errorf(errors.MsgTenantRequired)
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
#       base-url: "http://localhost:9000"
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
#       # base-url: "https://izanami.example.com"
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

// ValidConfigKeys defines all valid configuration keys
var ValidConfigKeys = map[string]bool{
	ConfigKeyBaseURL:                     true,
	ConfigKeyClientID:                    true,
	ConfigKeyClientSecret:                true,
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
}

// SensitiveKeys defines which keys contain sensitive information
var SensitiveKeys = map[string]bool{
	ConfigKeyClientSecret:        true,
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

// SetConfigValue sets a configuration value and persists it to the config file
func SetConfigValue(key, value string) error {
	if !ValidConfigKeys[key] {
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

// ValidateConfigFile validates the current configuration and returns any errors
func ValidateConfigFile() []ValidationError {
	var errs []ValidationError

	config, err := LoadConfig()
	if err != nil {
		errs = append(errs, ValidationError{
			Field:   "general",
			Message: fmt.Sprintf("Failed to load config: %v", err),
		})
		return errs
	}

	// Check base URL
	if config.BaseURL == "" {
		errs = append(errs, ValidationError{
			Field:   "base-url",
			Message: "Base URL is required",
		})
	}

	// Check authentication
	hasClientAuth := config.ClientID != "" && config.ClientSecret != ""
	hasUserAuth := config.JwtToken != "" || (config.Username != "" && config.PatToken != "")

	if !hasClientAuth && !hasUserAuth {
		errs = append(errs, ValidationError{
			Field:   "auth",
			Message: "Authentication required: either client-id/client-secret, jwt-token, or personal-access-token with personal-access-token-username must be set",
		})
	}

	// Personal access token requires username
	if config.PatToken != "" && config.Username == "" {
		errs = append(errs, ValidationError{
			Field:   "personal-access-token-username",
			Message: "personal-access-token-username is required when using personal-access-token",
		})
	}

	// Validate output format
	if config.OutputFormat != "" && config.OutputFormat != "table" && config.OutputFormat != "json" {
		errs = append(errs, ValidationError{
			Field:   "output-format",
			Message: "Output format must be 'table' or 'json'",
		})
	}

	// Validate color
	if config.Color != "" && config.Color != "auto" && config.Color != "always" && config.Color != "never" {
		errs = append(errs, ValidationError{
			Field:   "color",
			Message: "Color must be 'auto', 'always', or 'never'",
		})
	}

	// Validate timeout
	if config.Timeout < 0 {
		errs = append(errs, ValidationError{
			Field:   "timeout",
			Message: "Timeout must be a positive number",
		})
	}

	return errs
}

// ResolveClientCredentials looks up client credentials from the config's ClientKeys
// based on the provided tenant and projects. It searches with the following precedence:
// 1. Project-specific credentials (for each project in the list)
// 2. Tenant-wide credentials
// Returns empty strings if no credentials are found for the given tenant/projects.
func (c *Config) ResolveClientCredentials(tenant string, projects []string) (clientID, clientSecret string) {
	if c.ClientKeys == nil || tenant == "" {
		return "", ""
	}

	tenantConfig, ok := c.ClientKeys[tenant]
	if !ok {
		return "", ""
	}

	// First, try project-specific credentials
	if len(projects) > 0 && tenantConfig.Projects != nil {
		for _, project := range projects {
			if projectConfig, exists := tenantConfig.Projects[project]; exists {
				// Only use project credentials if both ID and secret are present
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

// initViperForClientKeys initializes viper and reads the config file
func initViperForClientKeys() (*viper.Viper, error) {
	configPath := GetConfigPath()
	configDir := getConfigDir()

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf(errors.MsgFailedToCreateConfigDir, err)
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// Read existing config if it exists
	if _, err := os.Stat(configPath); err == nil {
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf(errors.MsgFailedToReadConfigFile, err)
		}
	}

	return v, nil
}

// getOrCreateMap safely gets or creates a map from an interface{}
func getOrCreateMap(data interface{}) map[string]interface{} {
	if dataMap, ok := data.(map[string]interface{}); ok {
		return dataMap
	}
	return make(map[string]interface{})
}

// getOrCreateTenantData gets or creates tenant data from the client keys map
func getOrCreateTenantData(clientKeysMap map[string]interface{}, tenant string) map[string]interface{} {
	if tenantRaw, exists := clientKeysMap[tenant]; exists {
		return getOrCreateMap(tenantRaw)
	}
	return make(map[string]interface{})
}

// getOrCreateProjectsData gets or creates projects data from tenant data
func getOrCreateProjectsData(tenantData map[string]interface{}) map[string]interface{} {
	if projectsRaw, exists := tenantData["projects"]; exists {
		return getOrCreateMap(projectsRaw)
	}
	return make(map[string]interface{})
}

// storeTenantLevelCredentials stores credentials at the tenant level
func storeTenantLevelCredentials(tenantData map[string]interface{}, clientID, clientSecret string) {
	tenantData[ConfigKeyClientID] = clientID
	tenantData[ConfigKeyClientSecret] = clientSecret
}

// storeProjectLevelCredentials stores credentials for multiple projects
func storeProjectLevelCredentials(tenantData map[string]interface{}, projects []string, clientID, clientSecret string) {
	projectsData := getOrCreateProjectsData(tenantData)

	for _, project := range projects {
		projectsData[project] = map[string]interface{}{
			ConfigKeyClientID:     clientID,
			ConfigKeyClientSecret: clientSecret,
		}
	}

	tenantData["projects"] = projectsData
}

// AddClientKeys adds or updates client credentials in the config file.
// If projects is empty, credentials are stored at the tenant level.
// If projects are specified, credentials are stored for each project.
func AddClientKeys(tenant string, projects []string, clientID, clientSecret string) error {
	if tenant == "" {
		return fmt.Errorf("tenant is required")
	}
	if clientID == "" || clientSecret == "" {
		return fmt.Errorf("both client-id and client-secret are required")
	}

	v, err := initViperForClientKeys()
	if err != nil {
		return err
	}

	// Load existing client-keys structure or create new one
	clientKeysMap := make(map[string]interface{})
	if v.IsSet(ConfigKeyClientKeys) {
		clientKeysMap = v.GetStringMap(ConfigKeyClientKeys)
	}

	// Get or create tenant entry
	tenantData := getOrCreateTenantData(clientKeysMap, tenant)

	// Store credentials at appropriate level
	if len(projects) == 0 {
		storeTenantLevelCredentials(tenantData, clientID, clientSecret)
	} else {
		storeProjectLevelCredentials(tenantData, projects, clientID, clientSecret)
	}

	clientKeysMap[tenant] = tenantData
	v.Set(ConfigKeyClientKeys, clientKeysMap)

	// Write back to file
	if err := v.WriteConfigAs(GetConfigPath()); err != nil {
		return fmt.Errorf(errors.MsgFailedToWriteConfigFile, err)
	}

	// Ensure secure file permissions
	if err := os.Chmod(GetConfigPath(), 0600); err != nil {
		return fmt.Errorf("failed to set config file permissions: %w", err)
	}

	return nil
}

// GetActiveProfileName returns the name of the currently active profile from the config file
func GetActiveProfileName() (string, error) {
	config, err := LoadConfig()
	if err != nil {
		return "", err
	}

	if config.ActiveProfile == "" {
		return "", nil // No active profile
	}

	return config.ActiveProfile, nil
}

// GetProfile retrieves a specific profile from the config by name
func GetProfile(name string) (*Profile, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	if config.Profiles == nil {
		return nil, fmt.Errorf("no profiles defined")
	}

	profile, exists := config.Profiles[name]
	if !exists {
		return nil, fmt.Errorf("profile '%s' not found", name)
	}

	return profile, nil
}

// MergeWithProfile merges profile settings into the config
// Profile settings override top-level config but are overridden by env vars and flags
// Priority: Direct profile fields > Session data > Config defaults
func (c *Config) MergeWithProfile(profile *Profile) {
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
	// BaseURL: prefer profile.BaseURL, fallback to session.URL
	if profile.BaseURL != "" && c.BaseURL == "" {
		c.BaseURL = profile.BaseURL
	} else if sessionData != nil && sessionData.URL != "" && c.BaseURL == "" {
		c.BaseURL = sessionData.URL
	}

	// Username: prefer profile.Username, fallback to session.Username
	if profile.Username != "" && c.Username == "" {
		c.Username = profile.Username
	} else if sessionData != nil && sessionData.Username != "" && c.Username == "" {
		c.Username = sessionData.Username
	}

	// JwtToken: ONLY from session (short-lived, not stored in profiles)
	if sessionData != nil && sessionData.JwtToken != "" && c.JwtToken == "" {
		c.JwtToken = sessionData.JwtToken
	}

	// PatToken: only from profile (long-lived, not stored in sessions)
	if profile.PatToken != "" && c.PatToken == "" {
		c.PatToken = profile.PatToken
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
	if profile.ClientID != "" && c.ClientID == "" {
		c.ClientID = profile.ClientID
	}
	if profile.ClientSecret != "" && c.ClientSecret == "" {
		c.ClientSecret = profile.ClientSecret
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

// LoadConfigWithProfile loads the config and merges with the specified profile
// Priority order:
// 1. Command-line flags (handled by caller via MergeWithFlags)
// 2. Environment variables (handled by viper)
// 3. Profile settings
// 4. Session settings (for auth)
// 5. Top-level config (fallback)
func LoadConfigWithProfile(profileName string) (*Config, error) {
	// Load base config first
	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	// If no profile name specified, try to use active profile
	if profileName == "" {
		profileName = config.ActiveProfile
	}

	// If we have a profile name, merge it
	if profileName != "" {
		profile, err := GetProfile(profileName)
		if err != nil {
			return nil, fmt.Errorf("failed to load profile '%s': %w", profileName, err)
		}
		config.MergeWithProfile(profile)
	}

	return config, nil
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
	if profile.BaseURL != "" {
		profileMap["base-url"] = profile.BaseURL
	}
	if profile.Username != "" {
		profileMap["username"] = profile.Username
	}
	if profile.PatToken != "" {
		profileMap["personal-access-token"] = profile.PatToken
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
	if profile.ClientKeys != nil && len(profile.ClientKeys) > 0 {
		profileMap["client-keys"] = profile.ClientKeys
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
	config, err := LoadConfig()
	if err != nil {
		return nil, "", err
	}

	if config.Profiles == nil {
		return make(map[string]*Profile), "", nil
	}

	return config.Profiles, config.ActiveProfile, nil
}

// normalizeURL removes protocol and trailing slashes for URL comparison
func normalizeURL(url string) string {
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimRight(url, "/")
	return strings.ToLower(url)
}

// FindProfileByBaseURL finds a profile that matches the given base URL
// Returns (profileName, profile, nil) if found, ("", nil, nil) if not found
func FindProfileByBaseURL(baseURL string) (string, *Profile, error) {
	profiles, _, err := ListProfiles()
	if err != nil {
		return "", nil, err
	}

	// Normalize the search URL
	normalizedSearchURL := normalizeURL(baseURL)

	// Search through all profiles
	for name, profile := range profiles {
		profileURL := profile.BaseURL

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
		if normalizeURL(profileURL) == normalizedSearchURL {
			return name, profile, nil
		}
	}

	return "", nil, nil
}
