package izanami

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/viper"
	"github.com/webskin/izanami-go-cli/internal/errors"
)

// Config key constants
const (
	ConfigKeyBaseURL             = "base-url"
	ConfigKeyClientID            = "client-id"
	ConfigKeyClientSecret        = "client-secret"
	ConfigKeyUsername            = "username"
	ConfigKeyJwtToken            = "jwt-token"
	ConfigKeyPersonalAccessToken = "personal-access-token"
	ConfigKeyTenant              = "tenant"
	ConfigKeyProject             = "project"
	ConfigKeyContext             = "context"
	ConfigKeyTimeout             = "timeout"
	ConfigKeyVerbose             = "verbose"
	ConfigKeyOutputFormat        = "output-format"
	ConfigKeyColor               = "color"
	ConfigKeyClientKeys          = "client-keys"
)

// Error message constants
const (
	ErrMsgFailedToCreateConfigDir = "failed to create config directory: %w"
	ErrMsgFailedToReadConfigFile  = "failed to read config file: %w"
)

// Config holds the configuration for the Izanami client
type Config struct {
	BaseURL      string                            `yaml:"base-url" mapstructure:"base-url"`
	ClientID     string                            `yaml:"client-id" mapstructure:"client-id"`
	ClientSecret string                            `yaml:"client-secret" mapstructure:"client-secret"`
	Username     string                            `yaml:"username" mapstructure:"username"`
	JwtToken     string                            `yaml:"jwt-token" mapstructure:"jwt-token"`                         // JWT token from login
	PatToken     string                            `yaml:"personal-access-token" mapstructure:"personal-access-token"` // Personal Access Token
	Tenant       string                            `yaml:"tenant" mapstructure:"tenant"`
	Project      string                            `yaml:"project" mapstructure:"project"`
	Context      string                            `yaml:"context" mapstructure:"context"`
	Timeout      int                               `yaml:"timeout" mapstructure:"timeout"`
	Verbose      bool                              `yaml:"verbose" mapstructure:"verbose"`
	OutputFormat string                            `yaml:"output-format" mapstructure:"output-format"`       // Default output format (table/json)
	Color        string                            `yaml:"color" mapstructure:"color"`                       // Color output (auto/always/never)
	ClientKeys   map[string]TenantClientKeysConfig `yaml:"client-keys,omitempty" mapstructure:"client-keys"` // Tenant-specific client credentials
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
		return fmt.Errorf("username is required when using personal access token (set IZ_USERNAME or --username)")
	}

	// Check authentication: either client ID/secret, username/jwtToken, or patToken+username
	hasClientAuth := c.ClientID != "" && c.ClientSecret != ""
	hasUserAuth := (c.Username != "" && c.JwtToken != "") || (c.Username != "" && c.PatToken != "")

	if !hasClientAuth && !hasUserAuth {
		return fmt.Errorf("authentication required: either client-id/client-secret, username/jwt-token, or username/personal-access-token must be set")
	}

	return nil
}

// ValidateAdminAuth checks if admin authentication is configured
func (c *Config) ValidateAdminAuth() error {
	if c.BaseURL == "" {
		return fmt.Errorf("base URL is required (set IZ_BASE_URL or --url)")
	}

	if (c.Username == "" || c.JwtToken == "") && c.PatToken == "" {
		return fmt.Errorf("admin operations require login (iz login), username/jwt-token or personal-access-token (set IZ_USERNAME and IZ_JWT_TOKEN, or IZ_PERSONAL_ACCESS_TOKEN)")
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
		return fmt.Errorf(ErrMsgFailedToCreateConfigDir, err)
	}

	configPath := filepath.Join(configDir, "config.yaml")
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("config file already exists at %s", configPath)
	}

	sampleConfig := `# Izanami CLI Configuration
# You can also use environment variables (IZ_*) or command-line flags

# Base URL of your Izanami instance (required)
# base-url: "https://izanami.example.com"

# Client authentication (for feature evaluation)
# client-id: "your-client-id"
# client-secret: "your-client-secret"

# Admin authentication (for admin operations)
# Option 1: Username + JWT token (from login)
# username: "your-username"
# jwt-token: "your-jwt-token"

# Option 2: Username + Personal Access Token (requires username)
# username: "your-username"
# personal-access-token: "your-personal-access-token"

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

# Default output format (table or json)
output-format: table

# Color output (auto, always, never)
color: auto
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
	ConfigKeyBaseURL:             true,
	ConfigKeyClientID:            true,
	ConfigKeyClientSecret:        true,
	ConfigKeyUsername:            true,
	ConfigKeyJwtToken:            true,
	ConfigKeyPersonalAccessToken: true,
	ConfigKeyTenant:              true,
	ConfigKeyProject:             true,
	ConfigKeyContext:             true,
	ConfigKeyTimeout:             true,
	ConfigKeyVerbose:             true,
	ConfigKeyOutputFormat:        true,
	ConfigKeyColor:               true,
	ConfigKeyClientKeys:          true,
}

// SensitiveKeys defines which keys contain sensitive information
var SensitiveKeys = map[string]bool{
	ConfigKeyClientSecret:        true,
	ConfigKeyJwtToken:            true,
	ConfigKeyPersonalAccessToken: true,
	ConfigKeyClientKeys:          true, // Contains client secrets
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
		return fmt.Errorf(ErrMsgFailedToCreateConfigDir, err)
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// Read existing config if it exists
	if _, err := os.Stat(configPath); err == nil {
		if err := v.ReadInConfig(); err != nil {
			return fmt.Errorf(ErrMsgFailedToReadConfigFile, err)
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
		return fmt.Errorf(ErrMsgFailedToReadConfigFile, err)
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
	hasUserAuth := (config.Username != "" && config.JwtToken != "") || (config.Username != "" && config.PatToken != "")

	if !hasClientAuth && !hasUserAuth {
		errs = append(errs, ValidationError{
			Field:   "auth",
			Message: "Authentication required: either client-id/client-secret, username/jwt-token, or username/personal-access-token must be set",
		})
	}

	// Personal access token requires username
	if config.PatToken != "" && config.Username == "" {
		errs = append(errs, ValidationError{
			Field:   "username",
			Message: "Username is required when using personal-access-token",
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

	configPath := GetConfigPath()
	configDir := getConfigDir()

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf(ErrMsgFailedToCreateConfigDir, err)
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// Read existing config if it exists
	if _, err := os.Stat(configPath); err == nil {
		if err := v.ReadInConfig(); err != nil {
			return fmt.Errorf(ErrMsgFailedToReadConfigFile, err)
		}
	}

	// Load existing client-keys structure or create new one
	clientKeysMap := make(map[string]interface{})
	if v.IsSet(ConfigKeyClientKeys) {
		clientKeysMap = v.GetStringMap(ConfigKeyClientKeys)
	}

	// Get or create tenant entry
	var tenantData map[string]interface{}
	if tenantRaw, exists := clientKeysMap[tenant]; exists {
		if tenantMap, ok := tenantRaw.(map[string]interface{}); ok {
			tenantData = tenantMap
		} else {
			tenantData = make(map[string]interface{})
		}
	} else {
		tenantData = make(map[string]interface{})
	}

	if len(projects) == 0 {
		// Store at tenant level
		tenantData[ConfigKeyClientID] = clientID
		tenantData[ConfigKeyClientSecret] = clientSecret
	} else {
		// Store at project level
		var projectsData map[string]interface{}
		if projectsRaw, exists := tenantData["projects"]; exists {
			if projectsMap, ok := projectsRaw.(map[string]interface{}); ok {
				projectsData = projectsMap
			} else {
				projectsData = make(map[string]interface{})
			}
		} else {
			projectsData = make(map[string]interface{})
		}

		// Add credentials for each project
		for _, project := range projects {
			projectsData[project] = map[string]interface{}{
				ConfigKeyClientID:     clientID,
				ConfigKeyClientSecret: clientSecret,
			}
		}

		tenantData["projects"] = projectsData
	}

	clientKeysMap[tenant] = tenantData
	v.Set(ConfigKeyClientKeys, clientKeysMap)

	// Write back to file
	if err := v.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf(errors.MsgFailedToWriteConfigFile, err)
	}

	// Ensure secure file permissions
	if err := os.Chmod(configPath, 0600); err != nil {
		return fmt.Errorf("failed to set config file permissions: %w", err)
	}

	return nil
}
