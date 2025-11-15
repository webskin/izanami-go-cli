package errors

// Common error messages used across the application
const (
	// MsgTenantRequired is the error message when tenant is not specified
	MsgTenantRequired = "tenant is required (use --tenant flag or set IZ_TENANT)"

	// Session-related error messages
	MsgNoActiveSession           = "no active session"
	MsgNoActiveSessionWithLogin  = "no active session (use 'iz login' to authenticate)"
	MsgNoSavedSessions           = "No saved sessions. Use 'iz login' to create one."
	MsgSessionNotFound           = "session '%s' not found"
	MsgActiveSessionNotFound     = "active session '%s' not found"
	MsgFailedToSaveSessions      = "failed to save sessions"
	MsgFailedToReadSessionsFile  = "failed to read sessions file"
	MsgFailedToParseSessionsFile = "failed to parse sessions file"
	MsgFailedToMarshalSessions   = "failed to marshal sessions"
	MsgFailedToWriteSessionsFile = "failed to write sessions file"

	// Authentication error messages
	MsgBaseURLRequired      = "base URL is required"
	MsgLoginRequestFailed   = "login request failed"
	MsgLoginFailed          = "login failed (status %d): invalid credentials"
	MsgNoJWTTokenInResponse = "no JWT token in login response"

	// Feature error messages
	MsgFailedToListFeatures   = "failed to list features"
	MsgFailedToGetFeature     = "failed to get feature"
	MsgFailedToCreateFeature  = "failed to create feature"
	MsgFailedToUpdateFeature  = "failed to update feature"
	MsgFailedToDeleteFeature  = "failed to delete feature"
	MsgFailedToCheckFeature   = "failed to check feature"

	// Context error messages
	MsgFailedToListContexts   = "failed to list contexts"
	MsgFailedToCreateContext  = "failed to create context"
	MsgFailedToDeleteContext  = "failed to delete context"

	// Tenant error messages
	MsgFailedToListTenants   = "failed to list tenants"
	MsgFailedToGetTenant     = "failed to get tenant"
	MsgFailedToCreateTenant  = "failed to create tenant"
	MsgFailedToUpdateTenant  = "failed to update tenant"
	MsgFailedToDeleteTenant  = "failed to delete tenant"

	// Project error messages
	MsgFailedToListProjects   = "failed to list projects"
	MsgFailedToGetProject     = "failed to get project"
	MsgFailedToCreateProject  = "failed to create project"
	MsgFailedToDeleteProject  = "failed to delete project"

	// API Key error messages
	MsgFailedToListAPIKeys   = "failed to list API keys"
	MsgFailedToGetAPIKey     = "failed to get API key"
	MsgFailedToCreateAPIKey  = "failed to create API key"
	MsgFailedToUpdateAPIKey  = "failed to update API key"
	MsgFailedToDeleteAPIKey  = "failed to delete API key"

	// Tag error messages
	MsgFailedToListTags   = "failed to list tags"
	MsgFailedToCreateTag  = "failed to create tag"
	MsgFailedToDeleteTag  = "failed to delete tag"

	// Event/Stream error messages
	MsgFailedToConnectToEventStream = "failed to connect to event stream"
	MsgEventStreamReturnedStatus    = "event stream returned status %d"
	MsgErrorReadingEventStream      = "error reading event stream"

	// Utility error messages
	MsgFailedToCheckHealth = "failed to check health"
	MsgFailedToSearch      = "failed to search"
	MsgFailedToExport      = "failed to export"
	MsgFailedToImport      = "failed to import"
)
