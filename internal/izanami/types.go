package izanami

import (
	"fmt"
	"sort"
	"strings"
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
	Active     interface{}                `json:"active"` // Can be bool, string, or number (same as FeatureCheckResult)
	Project    string                     `json:"project"`
	Conditions map[string]ContextOverload `json:"conditions,omitempty"`
}

// ContextOverload represents feature conditions for a specific context
type ContextOverload struct {
	Enabled    bool                  `json:"enabled"`
	Conditions []ActivationCondition `json:"conditions"`
}

// ActivationsWithConditions is a map of feature IDs to their activation results
type ActivationsWithConditions map[string]ActivationWithConditions

// ActivationTableView represents a feature activation for table display
type ActivationTableView struct {
	ID      string      `json:"id"`
	Name    string      `json:"name"`
	Active  interface{} `json:"active"` // Can be bool, string, or number
	Project string      `json:"project"`
}

// FormatActive formats the Active field with color
// Red for false/0/""/nil, Green for any other value
func (a ActivationTableView) FormatActive() string {
	switch v := a.Active.(type) {
	case bool:
		if v {
			return color.GreenString("true")
		}
		return color.RedString("false")
	case string:
		if v == "" || v == "false" || v == "0" {
			return color.RedString(v)
		}
		return color.GreenString(v)
	case int, int8, int16, int32, int64:
		if v == 0 {
			return color.RedString("0")
		}
		return color.GreenString("%v", v)
	case float32, float64:
		if v == 0.0 {
			return color.RedString("0")
		}
		return color.GreenString("%v", v)
	case nil:
		return color.RedString("false")
	default:
		// For other types, just show in green if not false/0
		str := fmt.Sprintf("%v", v)
		if str == "false" || str == "0" || str == "" {
			return color.RedString(str)
		}
		return color.GreenString(str)
	}
}

// ActivationTableRow represents a single row for table display with formatted Active field
type ActivationTableRow struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Active  string `json:"active"` // Formatted and colored
	Project string `json:"project"`
}

// ToTableView converts ActivationsWithConditions to a sorted slice for table display
func (a ActivationsWithConditions) ToTableView() []ActivationTableRow {
	var temp []ActivationTableView

	// First create the intermediate views
	for id, activation := range a {
		temp = append(temp, ActivationTableView{
			ID:      id,
			Name:    activation.Name,
			Active:  activation.Active,
			Project: activation.Project,
		})
	}

	// Sort by name for consistent output
	sort.Slice(temp, func(i, j int) bool {
		return temp[i].Name < temp[j].Name
	})

	// Convert to rows with formatted Active field
	result := make([]ActivationTableRow, len(temp))
	for i, item := range temp {
		result[i] = ActivationTableRow{
			ID:      item.ID,
			Name:    item.Name,
			Active:  item.FormatActive(),
			Project: item.Project,
		}
	}

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
	Username      string     `json:"username"`
	Email         string     `json:"email"`
	Password      string     `json:"password,omitempty"` // Only for creation/update
	Admin         bool       `json:"admin"`
	UserType      string     `json:"userType"`      // INTERNAL, OTOROSHI, OIDC
	DefaultTenant *string    `json:"defaultTenant"` // Pointer to handle null
	Rights        UserRights `json:"rights,omitempty"`
}

// UserRights wraps the tenants map as returned by the API
type UserRights struct {
	Tenants map[string]TenantRight `json:"tenants"`
}

// UserListItem represents a user in the list response (simplified format)
type UserListItem struct {
	Username     string            `json:"username"`
	Email        string            `json:"email"`
	Admin        bool              `json:"admin"`
	UserType     string            `json:"userType"`
	TenantRights map[string]string `json:"tenantRights,omitempty"` // Simple map of tenant->level
}

// FormatTenantRights formats tenant rights for table display
// Shows up to 3 items as "tenant:level,tenant2:level2", then "[x items]" if more
func (u UserListItem) FormatTenantRights() string {
	if len(u.TenantRights) == 0 {
		return "-"
	}

	if len(u.TenantRights) > 3 {
		return fmt.Sprintf("[%d items]", len(u.TenantRights))
	}

	var items []string
	for tenant, level := range u.TenantRights {
		items = append(items, fmt.Sprintf("%s:%s", tenant, level))
	}

	return strings.Join(items, ",")
}

// UserWithSingleLevelRight represents a user with tenant-level right
type UserWithSingleLevelRight struct {
	Username      string `json:"username"`
	Email         string `json:"email"`
	UserType      string `json:"userType"`
	Admin         bool   `json:"admin"`
	DefaultTenant string `json:"defaultTenant,omitempty"`
	Right         string `json:"right,omitempty"` // Read, Write, Admin - for tenant level
}

// ProjectScopedUser represents a user with project-level right
type ProjectScopedUser struct {
	Username      string  `json:"username"`
	Email         string  `json:"email"`
	UserType      string  `json:"userType"`
	Admin         bool    `json:"admin"`
	DefaultTenant string  `json:"defaultTenant,omitempty"`
	TenantAdmin   bool    `json:"tenantAdmin"`
	Right         string  `json:"right"`        // Project-level right
	DefaultRight  *string `json:"defaultRight"` // Default project right at tenant level
}

// TenantRight represents user rights for a tenant
type TenantRight struct {
	Level               string                        `json:"level"` // Read, Write, Admin
	Projects            map[string]ProjectRight       `json:"projects,omitempty"`
	Keys                map[string]GeneralAtomicRight `json:"keys,omitempty"`
	Webhooks            map[string]GeneralAtomicRight `json:"webhooks,omitempty"`
	DefaultProjectRight *string                       `json:"defaultProjectRight,omitempty"`
	DefaultKeyRight     *string                       `json:"defaultKeyRight,omitempty"`
	DefaultWebhookRight *string                       `json:"defaultWebhookRight,omitempty"`
}

// ProjectRight represents user rights for a project
type ProjectRight struct {
	Level string `json:"level"` // Read, Update, Write, Admin
}

// GeneralAtomicRight represents atomic rights for keys/webhooks
type GeneralAtomicRight struct {
	Level string `json:"level"` // Read, Write, Admin
}

// UserInformationUpdateRequest represents a request to update user information
type UserInformationUpdateRequest struct {
	Username      string  `json:"username"`
	Email         string  `json:"email"`
	Password      string  `json:"password"`
	DefaultTenant *string `json:"defaultTenant,omitempty"`
}

// UserRightsUpdateRequest represents a request to update user rights
type UserRightsUpdateRequest struct {
	Rights map[string]TenantRight `json:"rights"`
	Admin  *bool                  `json:"admin,omitempty"`
}

// TenantRightUpdateRequest represents a request to update user rights for a specific tenant
type TenantRightUpdateRequest struct {
	Level               *string                       `json:"level,omitempty"`
	DefaultProjectRight *string                       `json:"defaultProjectRight,omitempty"`
	DefaultKeyRight     *string                       `json:"defaultKeyRight,omitempty"`
	DefaultWebhookRight *string                       `json:"defaultWebhookRight,omitempty"`
	Projects            map[string]ProjectRight       `json:"projects,omitempty"`
	Keys                map[string]GeneralAtomicRight `json:"keys,omitempty"`
	Webhooks            map[string]GeneralAtomicRight `json:"webhooks,omitempty"`
}

// ProjectRightUpdateRequest represents a request to update user's project rights
type ProjectRightUpdateRequest struct {
	Level string `json:"level"` // Read, Update, Write, Admin
}

// UserInvitation represents a user invitation for bulk operations
type UserInvitation struct {
	Username string `json:"username"`
	Level    string `json:"level"` // Right level (RightLevel for tenants, ProjectRightLevel for projects)
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

// AuditEvent represents an audit log event in Izanami
type AuditEvent struct {
	EventID            int64                  `json:"eventId"`
	ID                 string                 `json:"id"`
	Name               string                 `json:"name,omitempty"`
	Tenant             string                 `json:"tenant"`
	Project            string                 `json:"project,omitempty"`
	User               string                 `json:"user"`
	Type               string                 `json:"type"`
	Origin             string                 `json:"origin"`
	EmittedAt          string                 `json:"emittedAt"`
	Authentication     string                 `json:"authentication"`
	Conditions         map[string]interface{} `json:"conditions,omitempty"`
	PreviousConditions map[string]interface{} `json:"previousConditions,omitempty"`
}

// AuditEventTableView represents an audit event for table display
type AuditEventTableView struct {
	EventID        int64  `json:"eventId"`
	Type           string `json:"type"`
	User           string `json:"user"`
	Name           string `json:"name"`
	Project        string `json:"project"`
	EmittedAt      string `json:"emittedAt"`
	Authentication string `json:"authentication"`
}

// ToTableView converts an AuditEvent to a table-friendly view
func (e *AuditEvent) ToTableView() AuditEventTableView {
	name := e.Name
	if name == "" {
		name = e.ID
	}
	return AuditEventTableView{
		EventID:        e.EventID,
		Type:           e.Type,
		User:           e.User,
		Name:           name,
		Project:        e.Project,
		EmittedAt:      e.EmittedAt,
		Authentication: e.Authentication,
	}
}

// LogsResponse represents the response from the logs endpoint
type LogsResponse struct {
	Events []AuditEvent `json:"events"`
	Count  int          `json:"count,omitempty"`
}

// ToTableView converts LogsResponse events to table-friendly views
func (r *LogsResponse) ToTableView() []AuditEventTableView {
	views := make([]AuditEventTableView, len(r.Events))
	for i, e := range r.Events {
		views[i] = e.ToTableView()
	}
	return views
}

// LogsRequest represents the query parameters for fetching logs
type LogsRequest struct {
	Order    string // asc or desc
	Users    string // comma-separated user filter
	Types    string // comma-separated event type filter
	Features string // comma-separated feature filter
	Projects string // comma-separated project filter
	Start    string // ISO 8601 date-time
	End      string // ISO 8601 date-time
	Cursor   int64  // cursor for pagination
	Count    int    // number of results (default 50)
	Total    bool   // include total count
}

// OutputFormat represents the output format for CLI commands
type OutputFormat string

const (
	OutputJSON  OutputFormat = "json"
	OutputTable OutputFormat = "table"
)
