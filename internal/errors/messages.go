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
)
