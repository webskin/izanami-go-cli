# OIDC Login Implementation - Authorization Code + Local Callback Server

## Current Implementation Status

✅ **Phase 1 (Implemented)**: Token copy-paste flow
- `iz login --oidc --url <url>` opens browser for OIDC authentication
- User manually copies JWT token from browser cookies
- User pastes token into CLI
- Session saved to `~/.izsessions`

⏳ **Phase 2 (Future)**: Automatic callback flow (requires Izanami server changes)
- Local callback server receives token automatically
- No manual copy-paste needed

### Comparison

| Aspect | Current (Copy-Paste) | Future (Callback) |
|--------|---------------------|-------------------|
| User steps | 5 (open, login, find cookie, copy, paste) | 2 (open, login) |
| Error-prone | Yes (wrong token, typos) | No |
| Server changes | None required | Required |
| Headless support | Yes (`--token` flag) | Yes (`--token` flag) |
| WSL support | ✅ Opens Windows browser | ✅ Opens Windows browser |

## Overview

Implement OIDC authentication for the Izanami CLI using the Authorization Code flow with a local callback server. This is the standard approach used by major CLI tools like `gcloud`, `az`, `gh`, and `aws`.

## Flow Description

```
┌──────────┐     ┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│   CLI    │     │   Browser   │     │   Izanami    │     │     IdP     │
└────┬─────┘     └──────┬──────┘     └──────┬───────┘     └──────┬──────┘
     │                  │                   │                    │
     │ 1. Start local   │                   │                    │
     │    HTTP server   │                   │                    │
     │    (localhost)   │                   │                    │
     │                  │                   │                    │
     │ 2. Open browser ─┼──────────────────>│                    │
     │    to /api/admin/openid-connect      │                    │
     │                  │                   │                    │
     │                  │ 3. Redirect ──────┼───────────────────>│
     │                  │    to IdP auth    │                    │
     │                  │                   │                    │
     │                  │ 4. User authenticates                  │
     │                  │<───────────────────────────────────────│
     │                  │                   │                    │
     │                  │ 5. IdP redirects to Izanami            │
     │                  │    with auth code │                    │
     │                  │──────────────────>│                    │
     │                  │                   │                    │
     │                  │ 6. Izanami exchanges code for tokens   │
     │                  │                   │───────────────────>│
     │                  │                   │<───────────────────│
     │                  │                   │                    │
     │ 7. Redirect to   │<──────────────────│                    │
     │    localhost     │   (with session   │                    │
     │    callback      │    cookie/token)  │                    │
     │                  │                   │                    │
     │<─────────────────│                   │                    │
     │ 8. Extract       │                   │                    │
     │    credentials   │                   │                    │
     │                  │                   │                    │
     │ 9. Store in      │                   │                    │
     │    config        │                   │                    │
     │                  │                   │                    │
     │ 10. Close server │                   │                    │
     │     & browser tab│                   │                    │
```

## Implementation Steps

### Step 1: Add OIDC Login Command

Create `iz login --oidc` or `iz auth login --oidc` command.

```go
// cmd/login.go or cmd/auth/login.go
var loginCmd = &cobra.Command{
    Use:   "login",
    Short: "Authenticate with Izanami server",
    RunE:  runLogin,
}

func init() {
    loginCmd.Flags().Bool("oidc", false, "Use OIDC authentication")
    loginCmd.Flags().Int("port", 0, "Local callback server port (default: random available port)")
    loginCmd.Flags().Duration("timeout", 5*time.Minute, "Authentication timeout")
}
```

### Step 2: Local HTTP Callback Server

```go
// internal/auth/callback_server.go

type CallbackServer struct {
    server   *http.Server
    port     int
    result   chan CallbackResult
    shutdown chan struct{}
}

type CallbackResult struct {
    SessionCookie string
    Token         string
    Error         error
}

func NewCallbackServer(port int) (*CallbackServer, error) {
    // If port is 0, find an available port
    listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
    if err != nil {
        return nil, err
    }

    actualPort := listener.Addr().(*net.TCPAddr).Port

    cs := &CallbackServer{
        port:     actualPort,
        result:   make(chan CallbackResult, 1),
        shutdown: make(chan struct{}),
    }

    mux := http.NewServeMux()
    mux.HandleFunc("/callback", cs.handleCallback)

    cs.server = &http.Server{
        Handler: mux,
    }

    go cs.server.Serve(listener)

    return cs, nil
}

func (cs *CallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
    // Extract session cookie or token from request
    // This depends on how Izanami returns credentials after OIDC flow

    // Option A: Session cookie in redirect
    cookies := r.Cookies()

    // Option B: Token in query parameter
    token := r.URL.Query().Get("token")

    // Option C: Token in fragment (requires JS to extract)

    // Send success page to browser
    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte(`
        <html>
        <body>
            <h1>Authentication Successful</h1>
            <p>You can close this window and return to the CLI.</p>
            <script>window.close();</script>
        </body>
        </html>
    `))

    cs.result <- CallbackResult{
        // Set appropriate values
    }
}

func (cs *CallbackServer) GetRedirectURI() string {
    return fmt.Sprintf("http://localhost:%d/callback", cs.port)
}

func (cs *CallbackServer) WaitForCallback(timeout time.Duration) (CallbackResult, error) {
    select {
    case result := <-cs.result:
        return result, result.Error
    case <-time.After(timeout):
        return CallbackResult{}, fmt.Errorf("authentication timed out after %v", timeout)
    case <-cs.shutdown:
        return CallbackResult{}, fmt.Errorf("authentication cancelled")
    }
}

func (cs *CallbackServer) Shutdown() {
    close(cs.shutdown)
    cs.server.Shutdown(context.Background())
}
```

### Step 3: Browser Opening Utility

✅ **Implemented** in `internal/utils/browser.go` with WSL support:

```go
// internal/utils/browser.go

// isWSL detects if running in Windows Subsystem for Linux
func isWSL() bool {
    data, err := os.ReadFile("/proc/version")
    if err != nil {
        return false
    }
    version := strings.ToLower(string(data))
    return strings.Contains(version, "microsoft") || strings.Contains(version, "wsl")
}

func OpenBrowser(url string) error {
    var cmd string
    var args []string

    switch runtime.GOOS {
    case "windows":
        cmd = "rundll32"
        args = []string{"url.dll,FileProtocolHandler", url}
    case "darwin":
        cmd = "open"
        args = []string{url}
    case "linux":
        if isWSL() {
            // In WSL, use cmd.exe to open Windows browser
            cmd = "cmd.exe"
            args = []string{"/c", "start", "", url}
        } else {
            cmd = "xdg-open"
            args = []string{url}
        }
    default:
        return fmt.Errorf("unsupported platform")
    }

    return exec.Command(cmd, args...).Start()
}
```

### Step 4: Main Login Flow

```go
// internal/auth/oidc_login.go

func OIDCLogin(serverURL string, port int, timeout time.Duration) error {
    // 1. Start callback server
    callbackServer, err := NewCallbackServer(port)
    if err != nil {
        return fmt.Errorf("failed to start callback server: %w", err)
    }
    defer callbackServer.Shutdown()

    // 2. Build authorization URL
    // Note: Izanami may need to support a redirect_uri parameter
    redirectURI := callbackServer.GetRedirectURI()
    authURL := fmt.Sprintf("%s/api/admin/openid-connect?redirect_uri=%s",
        serverURL,
        url.QueryEscape(redirectURI),
    )

    // 3. Open browser
    fmt.Printf("Opening browser for authentication...\n")
    fmt.Printf("If browser doesn't open, visit: %s\n", authURL)

    if err := OpenBrowser(authURL); err != nil {
        fmt.Printf("Warning: Could not open browser: %v\n", err)
    }

    // 4. Wait for callback
    fmt.Printf("Waiting for authentication (timeout: %v)...\n", timeout)
    result, err := callbackServer.WaitForCallback(timeout)
    if err != nil {
        return err
    }

    // 5. Store credentials
    // Save to config file or credential store

    fmt.Println("Authentication successful!")
    return nil
}
```

## Izanami Server Requirements

The current Izanami OIDC flow may need modifications to support CLI authentication:

### Option A: Custom Redirect URI Support (Recommended)

Izanami needs to accept a `redirect_uri` parameter that allows redirecting to `localhost` after successful authentication.

```
GET /api/admin/openid-connect?redirect_uri=http://localhost:8085/callback
```

After successful OIDC authentication, Izanami redirects to:
```
http://localhost:8085/callback?token=<jwt_or_session_token>
```

#### Required Server Changes (Scala)

In `LoginController.scala`, the `openIdConnect` method needs to:

1. Accept optional `redirect_uri` query parameter
2. Validate it's a localhost URL (security)
3. Store it in session for use after OIDC completes
4. Redirect to it with token after successful authentication

```scala
def openIdConnect: Action[AnyContent] = Action.async { implicit request =>
  // Get CLI redirect URI if provided (for CLI authentication)
  val cliRedirectUri = request.getQueryString("redirect_uri")

  // Validate redirect_uri is localhost only (security)
  val isValidCliRedirect = cliRedirectUri.exists(uri =>
    uri.startsWith("http://localhost:") || uri.startsWith("http://127.0.0.1:")
  )

  // ... existing OIDC flow ...

  // Store CLI redirect in session if valid
  val sessionWithCliRedirect = if (isValidCliRedirect) {
    session + ("cli_redirect_uri" -> cliRedirectUri.get)
  } else session

  Redirect(authorizeUrl).withSession(sessionWithCliRedirect)
}

def openIdCodeReturn: Action[AnyContent] = Action.async { implicit request =>
  // ... existing token exchange logic ...

  // After successful authentication, check for CLI redirect
  request.session.get("cli_redirect_uri") match {
    case Some(cliRedirect) =>
      // Redirect to CLI's localhost callback with token
      Redirect(s"$cliRedirect?token=$jwtToken")
        .withSession(request.session - "cli_redirect_uri")
    case None =>
      // Normal web flow - set cookie and redirect to frontend
      NoContent.withCookies(Cookie(name = "token", value = jwtToken, ...))
  }
}
```

### Option B: CLI-Specific Endpoint

Create a dedicated endpoint for CLI authentication:

```
GET /api/admin/cli-auth/start?callback_port=8085
```

This initiates the OIDC flow and handles the callback internally, then redirects to the CLI's localhost server.

### Option C: Token Exchange (Device Flow)

After browser authentication completes:
1. Izanami shows the user a one-time code
2. CLI polls or receives the code
3. CLI exchanges code for API token

This follows [RFC 8628 - Device Authorization Grant](https://tools.ietf.org/html/rfc8628).

## Security Considerations

1. **Localhost Only**: Callback server should only bind to `localhost`/`127.0.0.1`, never `0.0.0.0`

2. **State Parameter**: Include a random `state` parameter to prevent CSRF attacks
   ```go
   state := generateSecureRandomString(32)
   authURL := fmt.Sprintf("%s?...&state=%s", baseURL, state)
   // Verify state in callback
   ```

3. **PKCE (Proof Key for Code Exchange)**: Consider implementing PKCE for additional security
   ```go
   codeVerifier := generateSecureRandomString(64)
   codeChallenge := base64URLEncode(sha256(codeVerifier))
   // Send code_challenge in auth request
   // Send code_verifier in token exchange
   ```

4. **Port Validation**: Validate that the callback comes from expected source

5. **Token Storage**: Store tokens securely
   - Linux: Use `keyring` or encrypted file with proper permissions (0600)
   - macOS: Use Keychain
   - Windows: Use Credential Manager

## Credential Storage

```go
// internal/auth/credential_store.go

type CredentialStore interface {
    Save(serverURL string, credentials Credentials) error
    Load(serverURL string) (Credentials, error)
    Delete(serverURL string) error
}

type Credentials struct {
    Token        string    `json:"token,omitempty"`
    SessionID    string    `json:"session_id,omitempty"`
    RefreshToken string    `json:"refresh_token,omitempty"`
    ExpiresAt    time.Time `json:"expires_at,omitempty"`
}

// File-based implementation (fallback)
type FileCredentialStore struct {
    path string
}

// Keyring-based implementation (preferred)
type KeyringCredentialStore struct {
    serviceName string
}
```

## CLI User Experience

```bash
$ iz login --oidc
Opening browser for authentication...
If browser doesn't open, visit: https://izanami.example.com/api/admin/openid-connect?redirect_uri=http://localhost:54321/callback

Waiting for authentication (timeout: 5m0s)...
Authentication successful!

Logged in as: user@example.com
Token expires: 2025-11-28 10:30:00

$ iz login --oidc --no-browser
Visit this URL to authenticate:
https://izanami.example.com/api/admin/openid-connect?redirect_uri=http://localhost:54321/callback

Waiting for authentication (timeout: 5m0s)...
```

## Dependencies

Consider using these Go libraries:

- `github.com/pkg/browser` - Cross-platform browser opening
- `github.com/zalando/go-keyring` - Cross-platform keyring access
- `golang.org/x/oauth2` - OAuth2 client (if handling token exchange directly)

## Testing

1. **Unit Tests**: Mock HTTP server for callback testing
2. **Integration Tests**: Test against a local OIDC provider (e.g., Keycloak in Docker)
3. **Manual Testing**: Test on Windows, macOS, Linux

## References

- [RFC 6749 - OAuth 2.0](https://tools.ietf.org/html/rfc6749)
- [RFC 7636 - PKCE](https://tools.ietf.org/html/rfc7636)
- [RFC 8628 - Device Authorization Grant](https://tools.ietf.org/html/rfc8628) (alternative approach)
- [Google Cloud CLI Authentication](https://cloud.google.com/sdk/gcloud/reference/auth/login)
- [Azure CLI Authentication](https://docs.microsoft.com/en-us/cli/azure/authenticate-azure-cli)
- [GitHub CLI Authentication](https://cli.github.com/manual/gh_auth_login)
