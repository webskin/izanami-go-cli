package izanami

import (
	"sort"
	"time"

	"github.com/fatih/color"
)

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
	Tenant  string      `json:"tenant,omitempty"` // Tenant (populated by CLI, not from API)
	ID      string      `json:"id,omitempty"`     // Feature ID (populated by CLI, not from API)
}

// ActivationWithConditions represents a feature activation with optional conditions
type ActivationWithConditions struct {
	Name       string                     `json:"name"`
	Active     bool                       `json:"active"`
	Project    string                     `json:"project"`
	Conditions map[string]ContextOverload `json:"conditions,omitempty"`
}

// ContextOverload represents feature conditions for a specific context
type ContextOverload struct {
	Enabled    bool                   `json:"enabled"`
	Conditions []ActivationCondition  `json:"conditions"`
}

// ActivationsWithConditions is a map of feature IDs to their activation results
type ActivationsWithConditions map[string]ActivationWithConditions

// ActivationTableView represents a feature activation for table display
type ActivationTableView struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Active  bool   `json:"active"`
	Project string `json:"project"`
}

// ToTableView converts ActivationsWithConditions to a sorted slice for table display
func (a ActivationsWithConditions) ToTableView() []ActivationTableView {
	var result []ActivationTableView

	for id, activation := range a {
		result = append(result, ActivationTableView{
			ID:      id,
			Name:    activation.Name,
			Active:  activation.Active,
			Project: activation.Project,
		})
	}

	// Sort by name for consistent output
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// CheckFeaturesRequest represents the request parameters for bulk feature checking
type CheckFeaturesRequest struct {
	User       string   `json:"-"` // Query param, not body
	Context    string   `json:"-"` // Query param, not body
	Features   []string `json:"-"` // Query param: feature IDs to check
	Projects   []string `json:"-"` // Query param: project IDs to check
	Conditions bool     `json:"-"` // Query param: whether to return conditions
	Date       string   `json:"-"` // Query param: ISO 8601 datetime
	OneTagIn   []string `json:"-"` // Query param: at least one tag must match
	AllTagsIn  []string `json:"-"` // Query param: all tags must match
	NoTagIn    []string `json:"-"` // Query param: none of these tags can match
	Payload    string   `json:"-"` // Optional JSON payload for POST requests (script features)
}

// EventsWatchRequest represents the request parameters for watching events
type EventsWatchRequest struct {
	User              string   `json:"-"` // Query param: user for feature evaluation (default: "*")
	Context           string   `json:"-"` // Query param: context for evaluation
	Features          []string `json:"-"` // Query param: feature IDs to watch
	Projects          []string `json:"-"` // Query param: project IDs to watch
	Conditions        bool     `json:"-"` // Query param: whether to include conditions
	Date              string   `json:"-"` // Query param: ISO 8601 datetime
	OneTagIn          []string `json:"-"` // Query param: at least one tag must match
	AllTagsIn         []string `json:"-"` // Query param: all tags must match
	NoTagIn           []string `json:"-"` // Query param: none of these tags can match
	RefreshInterval   int      `json:"-"` // Query param: periodic refresh interval in seconds
	KeepAliveInterval int      `json:"-"` // Query param: keep-alive interval in seconds (default: 25)
	Payload           string   `json:"-"` // Optional JSON payload for POST requests (script features)
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

// ContextTableView represents a context for table display with reordered columns
type ContextTableView struct {
	Path        string            `json:"path"`
	Name        string            `json:"name"`
	Project     string            `json:"project,omitempty"`
	IsProtected bool              `json:"protected"`
	Global      bool              `json:"global"`
	Overloads   []FeatureOverload `json:"overloads,omitempty"`
}

// ContextTableViewSimple represents a context for table display without overloads
// Used when listing contexts at tenant level (without --project flag)
type ContextTableViewSimple struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	Project     string `json:"project,omitempty"`
	IsProtected bool   `json:"protected"`
	Global      bool   `json:"global"`
}

// ToTableView converts a Context to ContextTableView for table display
func (c *Context) ToTableView(parentPath string) ContextTableView {
	// Build the full path
	path := c.Name
	if parentPath != "" {
		path = parentPath + "/" + c.Name
	}

	// Override with the context's Path field if it's set
	if c.Path != "" {
		path = c.Path
	}

	return ContextTableView{
		Path:        path,
		Name:        c.Name,
		Project:     c.Project,
		IsProtected: c.IsProtected,
		Global:      c.Global,
		Overloads:   c.Overloads,
	}
}

// ToTableViewSimple converts a Context to ContextTableViewSimple for table display without overloads
func (c *Context) ToTableViewSimple(parentPath string) ContextTableViewSimple {
	// Build the full path
	path := c.Name
	if parentPath != "" {
		path = parentPath + "/" + c.Name
	}

	// Override with the context's Path field if it's set
	if c.Path != "" {
		path = c.Path
	}

	return ContextTableViewSimple{
		Path:        path,
		Name:        c.Name,
		Project:     c.Project,
		IsProtected: c.IsProtected,
		Global:      c.Global,
	}
}

// FlattenContextsForTable converts a hierarchical context list to a flat list
// of ContextTableView with proper paths, sorted by Global (false first) then Path
func FlattenContextsForTable(contexts []Context) []ContextTableView {
	var result []ContextTableView

	var flatten func(ctx Context, parentPath string)
	flatten = func(ctx Context, parentPath string) {
		// Add this context
		result = append(result, ctx.ToTableView(parentPath))

		// Build the path for children
		childPath := ctx.Name
		if parentPath != "" {
			childPath = parentPath + "/" + ctx.Name
		}
		if ctx.Path != "" {
			childPath = ctx.Path
		}

		// Recursively add children
		for _, child := range ctx.Children {
			if child != nil {
				flatten(*child, childPath)
			}
		}
	}

	for _, ctx := range contexts {
		flatten(ctx, "")
	}

	// Sort results: Global false first, then by Path
	sort.Slice(result, func(i, j int) bool {
		// If Global values differ, false comes first
		if result[i].Global != result[j].Global {
			return !result[i].Global
		}
		// If Global values are the same, sort by Path
		return result[i].Path < result[j].Path
	})

	return result
}

// FlattenContextsForTableSimple converts a hierarchical context list to a flat list
// of ContextTableViewSimple (without overloads) with proper paths, sorted by Global (false first) then Path
func FlattenContextsForTableSimple(contexts []Context) []ContextTableViewSimple {
	var result []ContextTableViewSimple

	var flatten func(ctx Context, parentPath string)
	flatten = func(ctx Context, parentPath string) {
		// Add this context
		result = append(result, ctx.ToTableViewSimple(parentPath))

		// Build the path for children
		childPath := ctx.Name
		if parentPath != "" {
			childPath = parentPath + "/" + ctx.Name
		}
		if ctx.Path != "" {
			childPath = ctx.Path
		}

		// Recursively add children
		for _, child := range ctx.Children {
			if child != nil {
				flatten(*child, childPath)
			}
		}
	}

	for _, ctx := range contexts {
		flatten(ctx, "")
	}

	// Sort results: Global false first, then by Path
	sort.Slice(result, func(i, j int) bool {
		// If Global values differ, false comes first
		if result[i].Global != result[j].Global {
			return !result[i].Global
		}
		// If Global values are the same, sort by Path
		return result[i].Path < result[j].Path
	})

	return result
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

// FormatForTable implements custom table formatting for FeatureOverload
// Shows only name and enabled status with color (green=enabled, red=disabled)
func (f FeatureOverload) FormatForTable() string {
	var status string
	if f.Enabled {
		status = color.GreenString("enabled")
	} else {
		status = color.RedString("disabled")
	}
	return f.Name + " (" + status + ")"
}

// Tenant represents an Izanami tenant
type Tenant struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Projects    []Project `json:"projects,omitempty"`
	Tags        []Tag     `json:"tags,omitempty"`
}

// TenantSummary represents a tenant summary for list operations
// The list endpoint doesn't return projects and tags
type TenantSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
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

// RightLevel represents user permission levels in Izanami
type RightLevel string

const (
	RightLevelRead  RightLevel = "Read"
	RightLevelWrite RightLevel = "Write"
	RightLevelAdmin RightLevel = "Admin"
)

// String returns the string representation of the right level
func (r RightLevel) String() string {
	return string(r)
}

// ImportRequest represents an import operation request
type ImportRequest struct {
	Version         int    // Import version (query parameter, not in body)
	Conflict        string `json:"conflict,omitempty"`        // FAIL, SKIP, OVERWRITE
	Timezone        string `json:"timezone,omitempty"`        // Timezone for date/time fields
	DeduceProject   bool   `json:"deduceProject,omitempty"`   // Automatically deduce project from feature IDs
	CreateProjects  bool   `json:"create,omitempty"`          // Create projects if they don't exist
	Project         string `json:"project,omitempty"`         // Target project for import
	ProjectPartSize int    `json:"projectPartSize,omitempty"` // Batch size for project imports
	InlineScript    bool   `json:"inlineScript,omitempty"`    // Whether to inline WASM scripts
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
