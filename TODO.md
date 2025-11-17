# TODO

## CLI Configuration Management

### Implement `iz config` Commands
Add commands to manage CLI configuration without manually editing files.

#### Proposed Commands

**Basic Operations:**
- `iz config set <key> <value>` - Set a configuration value
  - Example: `iz config set base-url http://localhost:9000`
  - Example: `iz config set default-tenant my-tenant`
  - Example: `iz config set default-project my-project`

- `iz config get <key>` - Get a configuration value
  - Example: `iz config get base-url`
  - Shows current value and source (file/env/flag)

- `iz config unset <key>` - Remove a configuration value
  - Example: `iz config unset default-project`

- `iz config list` - List all configuration values
  - Show current effective config (merged from all sources)
  - Indicate source for each value (file/env/flag/default)
  - Option: `--show-secrets` to display tokens/secrets

**Advanced Operations:**
- `iz config edit` - Open config file in $EDITOR
  - Auto-validates after save
  - Creates backup before editing

- `iz config path` - Show config file path
  - Useful for troubleshooting
  - Show all searched paths

- `iz config init` - Initialize/create config file
  - Interactive prompts for common settings
  - Option: `--defaults` to create with defaults only

- `iz config validate` - Validate current configuration
  - Check syntax, required fields, connectivity
  - Return exit code for scripting

- `iz config reset` - Reset to defaults
  - Confirmation prompt required
  - Option: `--all` vs specific keys

**Profile Management (Future):**
- `iz config profiles list` - List available profiles
- `iz config profiles create <name>` - Create new profile
- `iz config profiles use <name>` - Switch active profile
- `iz config profiles delete <name>` - Delete profile
- Multiple profiles for different environments (dev/staging/prod)

#### Configuration Keys
- `base-url` - Izanami server URL
- `default-tenant` - Default tenant name
- `default-project` - Default project name
- `default-context` - Default context path
- `client-id` - Client ID for authentication
- `client-secret` - Client secret (stored securely)
- `username` - Admin username
- `token` - Personal access token (stored securely)
- `timeout` - Default request timeout
- `output-format` - Default output format (table/json)
- `color` - Enable/disable colors (auto/always/never)

#### Security Considerations
- Sensitive values (secrets, tokens) should be stored with appropriate permissions
- Option to use system keychain/credential manager
- Warn when setting secrets in plain config file
- Support for secret references: `token: ${env:IZ_TOKEN}`

---

## Feature Management

### Add Confirmation Prompts
- Add confirmation when asking for feature enabling/disabling
- Prevents accidental changes to critical features
- Should prompt "Are you sure you want to enable/disable feature X? [y/N]"

---

## Personal Access Token (PAT) Authentication

### Current Issue
- PATs are currently sent as JWT cookies, which causes the server to try parsing them as JWTs
- This results in error: "Last unit does not have enough valid bits"

### Solution
PATs should be sent via HTTP Basic Authentication instead of cookies.

**PAT Format**: `{uuid}_{secret}` (contains underscore)
**Session JWT Format**: Long base64 string (no underscore)

### Implementation Required

In `internal/izanami/client.go`, detect PAT format and use appropriate auth:

```go
if config.Username != "" && config.Token != "" {
    // Check if token is a Personal Access Token (PAT) or Session JWT
    // PAT format: {uuid}_{secret} (contains underscore)
    // Session JWT: long base64 string (no underscore)
    if strings.Contains(config.Token, "_") {
        // Personal Access Token - use HTTP Basic Auth
        // Format: Authorization: Basic base64(username:token)
        client.SetBasicAuth(config.Username, config.Token)
    } else {
        // Session JWT - use cookie authentication
        cookie := &http.Cookie{
            Name:  "token",
            Value: config.Token,
            Path:  "/",
        }
        client.SetCookie(cookie)
    }
}
```

### Server-side Validation
From `PersonnalAccessTokenTenantAuthAction.scala`:
1. Extracts `Authorization` header
2. Splits on `"Basic "` to get base64-encoded part
3. Base64 decodes to get `username:token`
4. Validates token against database

### Testing
Once implemented, test with:
```bash
export IZ_USERNAME=RESERVED_ADMIN_USER
export IZ_TOKEN=6374d239-ce82-45fa-b3d4-9b6bb349ea75_cyUBBnNsdDYVRZWcyyjRvqOTwuE0YKckuhEacCbtJfu78uGVy26CRhuRKN0uokKF
export IZ_BASE_URL=http://localhost:9000

./build/iz admin tenants list --verbose
```

Should see:
```
Authorization: Basic base64(username:token)
```

### Notes
- Session JWTs should continue using cookie authentication (current behavior)
- PATs require specific tenant permissions in the database
- PAT expiration is checked server-side

### Known Bug: Invalid Cookie Token Prevents PAT Fallback
**Issue**: If a token is stored in cookies but becomes invalid (expired/revoked), the authentication flow does not fall back to check for a Personal Access Token (PAT).

**Impact**: Users with an expired cookie token cannot authenticate even if they have a valid PAT configured, forcing manual cookie deletion.

**Expected Behavior**: The client should attempt PAT authentication if cookie-based authentication fails.

**Investigation Needed**: Check authentication flow in `internal/izanami/client.go` to implement proper fallback mechanism.
