package cmd

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/webskin/izanami-go-cli/internal/izanami"
	"golang.org/x/term"
)

var (
	// Global flags
	cfgFile            string
	profileName        string
	leaderURL          string
	tenant             string
	project            string
	contextPath        string
	timeout            int
	verbose            bool
	outputFormat       string
	compactJSON        bool
	insecureSkipVerify bool

	// Global config
	cfg           *izanami.ResolvedConfig
	activeProfile *izanami.Profile
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "iz",
	Short: "Izanami CLI - Manage feature flags and contexts",
	Long: `A cross-platform command-line client for Izanami feature flag management.

Izanami is a feature flag and configuration management system. This CLI
allows you to interact with Izanami for both administration tasks and
standard feature flag operations.

Configuration can be provided via:
  - Profiles: Named environment configurations (local, sandbox, build, prod)
  - Config file: ~/.config/iz/config.yaml (or platform-equivalent)
  - Environment variables: IZ_LEADER_URL, IZ_TENANT, IZ_PROJECT, etc.
  - Command-line flags: --url, --tenant, --project, etc.

Examples:
  # Use a profile for all commands
  iz profiles use sandbox
  iz features check my-feature

  # Or override active profile temporarily
  iz features check my-feature --profile prod

  # List all features in a project
  iz features list --project my-project

  # Admin: Create a new project
  iz admin projects create new-project --description "My new project"

For more information, visit: https://github.com/MAIF/izanami`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for commands that don't need it
		skipCommands := []string{"completion", "version", "help", "login", "logout", "sessions", "config", "profiles", "reset"}
		for _, skip := range skipCommands {
			if cmd.Name() == skip || cmd.Parent() != nil && cmd.Parent().Name() == skip {
				return nil
			}
		}

		var err error

		// Load config with profile support
		// Priority order:
		// 1. Command-line flags (applied later)
		// 2. Environment variables (handled by viper)
		// 3. Profile settings (if --profile specified or active profile exists)
		// 4. Session settings (loaded via profile's session reference)
		// 5. Top-level config (fallback)

		// Load config via profile system (profiles load their referenced sessions)
		if profileName != "" {
			// Load with specific profile from --profile flag
			cfg, activeProfile, err = izanami.LoadConfigWithProfile(profileName)
		} else {
			// Load with active profile (if any)
			cfg, activeProfile, err = izanami.LoadConfigWithProfile("")
		}

		if err != nil {
			return fmt.Errorf("failed to load config: %w (use 'iz login' to authenticate)", err)
		}

		// Command-line flags override everything (highest priority)
		// Environment variables override profile settings but are overridden by flags
		cfg.MergeWithFlags(izanami.FlagValues{
			LeaderURL:          getValueWithEnvFallback(leaderURL, "IZ_LEADER_URL"),
			ClientID:           getValueWithEnvFallback("", "IZ_CLIENT_ID"),
			ClientSecret:       getValueWithEnvFallback("", "IZ_CLIENT_SECRET"),
			Tenant:             getValueWithEnvFallback(tenant, "IZ_TENANT"),
			Project:            getValueWithEnvFallback(project, "IZ_PROJECT"),
			Context:            getValueWithEnvFallback(contextPath, "IZ_CONTEXT"),
			Timeout:            timeout,
			Verbose:            verbose,
			InsecureSkipVerify: insecureSkipVerify,
		})

		// Configure color output based on config setting
		configureColorOutput(cfg.Color)

		// Log authentication mode in verbose mode
		if cfg.Verbose {
			logEnvironmentVariables(cmd)
			logEffectiveConfig(cmd, cfg)
			logAuthenticationMode(cmd, cfg)
			// Also log active profile if any
			if profileName != "" {
				fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Using profile: %s (from --profile flag)\n", profileName)
			} else {
				activeProfile, err := izanami.GetActiveProfileName()
				if err == nil && activeProfile != "" {
					fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Using profile: %s (active profile)\n", activeProfile)
				}
			}
		}

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&profileName, "profile", "p", "", "Use specific profile (overrides active profile)")
	rootCmd.PersistentFlags().StringVar(&leaderURL, "url", "", "Izanami leader URL (env: IZ_LEADER_URL)")
	rootCmd.PersistentFlags().StringVar(&tenant, "tenant", "", "Default tenant (env: IZ_TENANT)")
	rootCmd.PersistentFlags().StringVar(&project, "project", "", "Default project (env: IZ_PROJECT)")
	rootCmd.PersistentFlags().StringVar(&contextPath, "context", "", "Default context path (env: IZ_CONTEXT)")
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 0, "Request timeout in seconds (default: 30)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "Output format: json or table")
	rootCmd.PersistentFlags().BoolVar(&compactJSON, "compact", false, "Output compact JSON (no pretty-printing)")
	rootCmd.PersistentFlags().BoolVarP(&insecureSkipVerify, "insecure", "k", false, "Skip TLS certificate verification (insecure)")

	// Register dynamic flag completions (must be after flags are defined)
	RegisterFlagCompletions()
}

// GetConfig returns the global configuration
func GetConfig() *izanami.ResolvedConfig {
	return cfg
}

// GetOutputFormat returns the current output format
func GetOutputFormat() izanami.OutputFormat {
	if outputFormat == "table" {
		return izanami.OutputTable
	}
	return izanami.OutputJSON
}

// sensitiveEnvVars lists environment variable names whose values should be redacted in verbose output.
var sensitiveEnvVars = map[string]bool{
	"IZ_CLIENT_SECRET":         true,
	"IZ_PERSONAL_ACCESS_TOKEN": true,
	"IZ_JWT_TOKEN":             true,
}

// configFieldInfo describes a config field for verbose source reporting.
type configFieldInfo struct {
	key       string                               // display name (e.g., "leader-url")
	flagName  string                               // cobra flag name (empty if no flag)
	envVar    string                               // env var name (empty if no env)
	getValue  func(*izanami.ResolvedConfig) string // extracts the effective value
	sensitive bool                                 // whether to redact the value
}

// configFields lists the config fields to display in verbose mode.
var configFields = []configFieldInfo{
	{key: "leader-url", flagName: "url", envVar: "IZ_LEADER_URL", getValue: func(c *izanami.ResolvedConfig) string { return c.LeaderURL }},
	{key: "client-id", envVar: "IZ_CLIENT_ID", getValue: func(c *izanami.ResolvedConfig) string { return c.ClientID }},
	{key: "client-secret", envVar: "IZ_CLIENT_SECRET", getValue: func(c *izanami.ResolvedConfig) string { return c.ClientSecret }, sensitive: true},
	{key: "tenant", flagName: "tenant", envVar: "IZ_TENANT", getValue: func(c *izanami.ResolvedConfig) string { return c.Tenant }},
	{key: "project", flagName: "project", envVar: "IZ_PROJECT", getValue: func(c *izanami.ResolvedConfig) string { return c.Project }},
	{key: "context", flagName: "context", envVar: "IZ_CONTEXT", getValue: func(c *izanami.ResolvedConfig) string { return c.Context }},
	{key: "timeout", flagName: "timeout", getValue: func(c *izanami.ResolvedConfig) string { return strconv.Itoa(c.Timeout) }},
	{key: "insecure", flagName: "insecure", getValue: func(c *izanami.ResolvedConfig) string { return strconv.FormatBool(c.InsecureSkipVerify) }},
}

// logEffectiveConfig prints each effective config value with its source in verbose mode.
func logEffectiveConfig(cmd *cobra.Command, cfg *izanami.ResolvedConfig) {
	// Load the active profile for source determination
	var activeProfileName string
	var profile *izanami.Profile
	if profileName != "" {
		activeProfileName = profileName
	} else {
		activeProfileName, _ = izanami.GetActiveProfileName()
	}
	if activeProfileName != "" {
		profile, _ = izanami.GetProfile(activeProfileName)
	}

	// Load session data if profile references one
	var session *izanami.Session
	if profile != nil && profile.Session != "" {
		sessions, err := izanami.LoadSessions()
		if err == nil {
			session, _ = sessions.GetSession(profile.Session)
		}
	}

	for _, field := range configFields {
		value := field.getValue(cfg)

		// Skip unset string fields
		if value == "" {
			continue
		}
		// Skip insecure when false (uninteresting default)
		if field.key == "insecure" && value == "false" {
			continue
		}

		source := determineConfigSource(cmd, field, profile, session)

		displayValue := value
		if field.sensitive {
			displayValue = izanami.RedactedValue
		}

		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Config: %s=%s (source: %s)\n", field.key, displayValue, source)
	}
}

// determineConfigSource checks layers in priority order to determine where
// the effective config value came from.
func determineConfigSource(cmd *cobra.Command, field configFieldInfo, profile *izanami.Profile, session *izanami.Session) string {
	// 1. Flag explicitly set?
	if field.flagName != "" && cmd.Flags().Changed(field.flagName) {
		return "flag"
	}

	// 2. Env var set?
	if field.envVar != "" && os.Getenv(field.envVar) != "" {
		return "env"
	}

	// 3. Session? (leader-url and jwt-token can come from session)
	if session != nil {
		switch field.key {
		case "leader-url":
			if session.URL != "" && (profile == nil || profile.LeaderURL == "") {
				return "session"
			}
		}
	}

	// 4. Profile?
	if profile != nil {
		profileValue := getProfileFieldValue(profile, field.key)
		if profileValue != "" && profileValue != "false" {
			return "profile"
		}
	}

	// 5. Global config keys: use GetConfigValue to distinguish file/env/default
	if izanami.GlobalConfigKeys[field.key] {
		if cv, err := izanami.GetConfigValue(field.key); err == nil {
			return cv.Source
		}
	}

	// 6. Fallback
	return "default"
}

// getProfileFieldValue returns the profile's raw value for a given config key.
func getProfileFieldValue(profile *izanami.Profile, key string) string {
	switch key {
	case "leader-url":
		return profile.LeaderURL
	case "default-worker":
		return profile.DefaultWorker
	case "client-id":
		return profile.ClientID
	case "client-secret":
		return profile.ClientSecret
	case "tenant":
		return profile.Tenant
	case "project":
		return profile.Project
	case "context":
		return profile.Context
	case "insecure":
		return strconv.FormatBool(profile.InsecureSkipVerify)
	default:
		return ""
	}
}

// logEnvironmentVariables prints all IZ_* environment variables in verbose mode,
// redacting values for sensitive variables.
func logEnvironmentVariables(cmd *cobra.Command) {
	var izVars []string
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "IZ_") {
			izVars = append(izVars, env)
		}
	}

	if len(izVars) == 0 {
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Environment: no IZ_* variables set\n")
		return
	}

	sort.Strings(izVars)
	for _, env := range izVars {
		name, value, _ := strings.Cut(env, "=")
		if sensitiveEnvVars[name] {
			value = izanami.RedactedValue
		}
		fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Environment: %s=%s\n", name, value)
	}
}

// logAuthenticationMode logs the available authentication modes
func logAuthenticationMode(cmd *cobra.Command, cfg *izanami.ResolvedConfig) {
	var adminAuth, clientAuth string

	// Check admin authentication (for admin operations)
	if cfg.PersonalAccessToken != "" {
		adminAuth = "Personal Access Token (PAT)"
	} else if cfg.JwtToken != "" {
		adminAuth = "JWT Cookie (session)"
	} else {
		adminAuth = "none"
	}

	// Check client authentication (for feature checks)
	if cfg.ClientID != "" && cfg.ClientSecret != "" {
		clientAuth = "Client API Key"
	} else {
		clientAuth = "none"
	}

	fmt.Fprintf(cmd.OutOrStderr(), "[verbose] Authentication - Admin operations: %s, Feature checks: %s\n", adminAuth, clientAuth)
}

// getValueWithEnvFallback returns the flag value if non-empty, otherwise falls back to the environment variable
func getValueWithEnvFallback(flagValue, envVar string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv(envVar)
}

// configureColorOutput configures color output based on the color setting
func configureColorOutput(colorSetting string) {
	switch colorSetting {
	case "never":
		// Disable all colors
		color.NoColor = true
	case "always":
		// Force colors even if not a TTY
		color.NoColor = false
	case "auto", "":
		// Auto-detect: enable colors only if stdout is a terminal
		color.NoColor = !term.IsTerminal(int(os.Stdout.Fd()))
	}
}
