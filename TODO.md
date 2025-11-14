# TODO

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
