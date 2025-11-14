package izanami

import "time"

// Feature represents a feature flag in Izanami
type Feature struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Project     string                 `json:"project"`
	Enabled     bool                   `json:"enabled"`
	Tags        []string               `json:"tags,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// FeatureWithOverloads represents a feature with context-specific overloads
type FeatureWithOverloads struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Project    string                 `json:"project"`
	Enabled    bool                   `json:"enabled"`
	Tags       []string               `json:"tags,omitempty"`
	Conditions []ActivationCondition  `json:"conditions,omitempty"`
	Overloads  map[string]interface{} `json:"overloads,omitempty"`
}

// ActivationCondition represents a condition for feature activation
type ActivationCondition struct {
	Period *FeaturePeriod  `json:"period,omitempty"`
	Rule   *ActivationRule `json:"rule,omitempty"`
}

// FeaturePeriod represents temporal constraints for a feature
type FeaturePeriod struct {
	Begin       *time.Time   `json:"begin,omitempty"`
	End         *time.Time   `json:"end,omitempty"`
	HourPeriods []HourPeriod `json:"hourPeriods,omitempty"`
	Days        []string     `json:"days,omitempty"`
	Timezone    string       `json:"timezone,omitempty"`
}

// HourPeriod represents a time range within a day
type HourPeriod struct {
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

// ActivationRule represents user targeting rules
type ActivationRule struct {
	Type       string   `json:"type"` // "All", "UserList", "UserPercentage"
	Users      []string `json:"users,omitempty"`
	Percentage float64  `json:"percentage,omitempty"`
}

// FeatureCheckResult represents the result of a feature check
type FeatureCheckResult struct {
	Active  interface{} `json:"active"` // bool, string, or number
	Name    string      `json:"name"`
	Project string      `json:"project"`
}

// Context represents a feature context (environment/override)
type Context struct {
	Name        string            `json:"name"`
	Project     string            `json:"project,omitempty"`
	Path        string            `json:"path,omitempty"`
	IsProtected bool              `json:"protected"`
	Global      bool              `json:"global"`
	Overloads   []FeatureOverload `json:"overloads,omitempty"`
	Children    []*Context        `json:"children,omitempty"`
}

// FeatureOverload represents a feature override in a context
type FeatureOverload struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Project     string                 `json:"project"`
	Description string                 `json:"description,omitempty"`
	Enabled     bool                   `json:"enabled"`
	ResultType  string                 `json:"resultType,omitempty"`
	Value       interface{}            `json:"value,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Conditions  []ActivationCondition  `json:"conditions,omitempty"`
}

// Tenant represents an Izanami tenant
type Tenant struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Projects    []Project `json:"projects,omitempty"`
	Tags        []Tag     `json:"tags,omitempty"`
}

// Project represents an Izanami project
type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Features    []Feature `json:"features,omitempty"`
}

// Tag represents a feature tag
type Tag struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// APIKey represents an API key for client authentication
type APIKey struct {
	ClientID     string   `json:"clientId"`
	ClientSecret string   `json:"clientSecret,omitempty"` // Only returned on creation
	Name         string   `json:"name"`
	Projects     []string `json:"projects,omitempty"`
	Description  string   `json:"description"`
	Enabled      bool     `json:"enabled"`
	Admin        bool     `json:"admin"`
}

// Webhook represents a webhook configuration
type Webhook struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	URL          string            `json:"url"`
	Headers      map[string]string `json:"headers,omitempty"`
	Features     []string          `json:"features,omitempty"`
	Projects     []string          `json:"projects,omitempty"`
	Enabled      bool              `json:"enabled"`
	Global       bool              `json:"global"`
	Context      string            `json:"context,omitempty"`
	User         string            `json:"user,omitempty"`
	BodyTemplate string            `json:"bodyTemplate,omitempty"`
}

// User represents an Izanami user
type User struct {
	Username      string                 `json:"username"`
	Email         string                 `json:"email"`
	Password      string                 `json:"password,omitempty"` // Only for creation/update
	Admin         bool                   `json:"admin"`
	UserType      string                 `json:"userType"` // INTERNAL, OTOROSHI, OIDC
	DefaultTenant string                 `json:"defaultTenant,omitempty"`
	Rights        map[string]TenantRight `json:"rights,omitempty"`
}

// TenantRight represents user rights for a tenant
type TenantRight struct {
	Level    string                  `json:"level"` // Read, Write, Admin
	Projects map[string]ProjectRight `json:"projects,omitempty"`
}

// ProjectRight represents user rights for a project
type ProjectRight struct {
	Level string `json:"level"` // Read, Update, Write, Admin
}

// SearchResult represents a search result
type SearchResult struct {
	Type        string      `json:"type"` // PROJECT, FEATURE, KEY, TAG, etc.
	Name        string      `json:"name"`
	ID          string      `json:"id,omitempty"`
	Description string      `json:"description,omitempty"`
	Tenant      string      `json:"tenant,omitempty"`
	Project     string      `json:"project,omitempty"`
	Data        interface{} `json:"data,omitempty"`
}

// ImportRequest represents an import operation request
type ImportRequest struct {
	Conflict        string `json:"conflict,omitempty"` // FAIL, SKIP, OVERWRITE
	Timezone        string `json:"timezone,omitempty"`
	DeduceProject   bool   `json:"deduceProject,omitempty"`
	CreateProjects  bool   `json:"create,omitempty"`
	Project         string `json:"project,omitempty"`
	ProjectPartSize int    `json:"projectPartSize,omitempty"`
	InlineScript    bool   `json:"inlineScript,omitempty"`
}

// ImportStatus represents the status of an import operation
type ImportStatus struct {
	ID         string    `json:"id"`
	Status     string    `json:"status"` // PENDING, RUNNING, COMPLETED, FAILED
	Message    string    `json:"message,omitempty"`
	Progress   int       `json:"progress,omitempty"`
	Total      int       `json:"total,omitempty"`
	StartedAt  time.Time `json:"startedAt,omitempty"`
	FinishedAt time.Time `json:"finishedAt,omitempty"`
}

// HealthStatus represents the health status of Izanami
type HealthStatus struct {
	Database bool   `json:"database"`         // true if database is healthy
	Status   string `json:"status,omitempty"` // Optional status field
	Version  string `json:"version,omitempty"`
}

// ErrorResponse represents an error response from the API
type ErrorResponse struct {
	Message string `json:"message"`
}

// OutputFormat represents the output format for CLI commands
type OutputFormat string

const (
	OutputJSON  OutputFormat = "json"
	OutputTable OutputFormat = "table"
)
