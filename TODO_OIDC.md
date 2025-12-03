# OIDC Login Implementation - State-Based Token Polling

## Current Implementation Status

**Phase 1 (Implemented)**: Token copy-paste flow
- `iz login --oidc --url <url>` opens browser for OIDC authentication
- User manually copies JWT token from browser cookies
- User pastes token into CLI
- Session saved to `~/.izsessions`

**Phase 2 (Planned)**: Automatic state-based polling flow
- CLI generates state, opens browser, polls for token
- No manual copy-paste needed
- Works with ANY OIDC provider (no configuration changes needed)

### Comparison

| Aspect | Phase 1 (Copy-Paste) | Phase 2 (State Polling) |
|--------|---------------------|-------------------------|
| User steps | 5 (open, login, find cookie, copy, paste) | 2 (open, login) |
| Error-prone | Yes (wrong token, typos) | No |
| Server changes | None required | Required (new endpoints) |
| OIDC provider changes | None | None |
| Headless support | Yes (`--token` flag) | Yes (`--token` flag) |
| WSL support | Opens Windows browser | Opens Windows browser |

---

## Phase 2: State-Based Token Polling

### Why This Approach?

**Why not dynamic callback URLs?**
- OIDC providers require pre-registered redirect URIs for security
- CLI tools typically need `http://localhost:{random_port}/callback` which can't be pre-registered
- Some providers support wildcard localhost, but it's not universal

**Why state parameter?**
- The `state` parameter is a standard OAuth 2.0/OIDC mechanism supported by all providers
- CLI generates the state, so it can prove ownership when polling for the token
- No changes needed to the OIDC provider configuration

**Why polling instead of local callback server?**
- Polling is simpler to implement in CLI tools across all platforms
- No firewall/NAT traversal issues (CLI initiates all connections)
- Works with any OIDC provider without configuration

### Authentication Flow

```
┌─────────┐                    ┌─────────────┐                    ┌──────────────┐
│   CLI   │                    │   Izanami   │                    │ OIDC Provider│
└────┬────┘                    └──────┬──────┘                    └──────┬───────┘
     │                                │                                  │
     │ 1. Generate state (32 bytes)   │                                  │
     │ ──────────────────────────>    │                                  │
     │    GET /api/admin/cli-login?state={state}                         │
     │                                │                                  │
     │                                │ 2. Store pending auth            │
     │                                │    Redirect with state=cli:{state}
     │                                │ ─────────────────────────────────>
     │                                │                                  │
     │ 3. Open browser ───────────────┼──────────────────────────────────>
     │                                │                                  │
     │                                │    4. User authenticates         │
     │                                │ <─────────────────────────────────
     │                                │    Callback with code + state    │
     │                                │                                  │
     │                                │ 5. Detect CLI flow (cli: prefix) │
     │                                │    Exchange code for token       │
     │                                │    Store token for CLI pickup    │
     │                                │                                  │
     │ 6. Poll for token              │                                  │
     │ ──────────────────────────>    │                                  │
     │    GET /api/admin/cli-token?state={state}                         │
     │                                │                                  │
     │ <──────────────────────────    │                                  │
     │    { "token": "jwt..." }       │                                  │
     │                                │                                  │
     │ 7. Use token for API calls     │                                  │
     └────────────────────────────────┴──────────────────────────────────┘
```

---

## Izanami Server Implementation (Completed)

### API Endpoints

#### 1. Initiate CLI Login

**Request:**
```
GET /api/admin/cli-login?state={state}
```

**Parameters:**

| Name  | Type   | Description                                                                       |
|-------|--------|-----------------------------------------------------------------------------------|
| state | string | Cryptographically secure random string (32 bytes, base64url encoded, 40-50 chars) |

**Response:**
- `302 Redirect` → OIDC provider authorization URL
- `400 Bad Request` → Invalid state format or OIDC not enabled
- `500 Internal Server Error` → OIDC configuration missing

**Example:**
```bash
# Generate state (CLI should do this)
STATE=$(openssl rand -base64 32 | tr -d '=' | tr '+/' '-_')

# Open browser to this URL
open "http://localhost:9000/api/admin/cli-login?state=${STATE}"
```

#### 2. Poll for Token

**Request:**
```
GET /api/admin/cli-token?state={state}
```

**Responses:**

| Status                | Body                                                   | Meaning                                       |
|-----------------------|--------------------------------------------------------|-----------------------------------------------|
| 200 OK                | `{ "token": "eyJ..." }`                                | Success - use this JWT for API calls          |
| 202 Accepted          | `{ "status": "pending", "message": "..." }`            | Still waiting - continue polling              |
| 400 Bad Request       | `{ "message": "Invalid state..." }`                    | Invalid state format                          |
| 404 Not Found         | `{ "message": "CLI authentication state not found" }`  | State never existed or already expired        |
| 410 Gone              | `{ "message": "CLI authentication state has expired" }`| Token was generated but expired before pickup |
| 429 Too Many Requests | `{ "message": "Too many requests...", "retryAfter": 5 }`| Rate limited (>60 req/min)                   |

**Example:**
```bash
# Poll until token received
while true; do
  RESPONSE=$(curl -s "http://localhost:9000/api/admin/cli-token?state=${STATE}")
  TOKEN=$(echo "$RESPONSE" | jq -r '.token // empty')

  if [ -n "$TOKEN" ]; then
    echo "Got token: $TOKEN"
    break
  fi

  STATUS=$(echo "$RESPONSE" | jq -r '.status // empty')
  if [ "$STATUS" = "pending" ]; then
    sleep 2
    continue
  fi

  echo "Error: $RESPONSE"
  break
done
```

### State Parameter Requirements

The CLI must generate the state parameter with these properties:

| Property   | Requirement                               |
|------------|-------------------------------------------|
| Length     | 40-50 characters                          |
| Entropy    | Minimum 32 bytes (256 bits)               |
| Encoding   | Base64url (characters: A-Za-z0-9_-)       |
| Uniqueness | Must be unique per authentication attempt |

**Generation examples:**

```python
# Python
import secrets
import base64
state = base64.urlsafe_b64encode(secrets.token_bytes(32)).rstrip(b'=').decode()
```

```javascript
// Node.js
const crypto = require('crypto');
const state = crypto.randomBytes(32).toString('base64url');
```

```bash
# Bash
STATE=$(openssl rand -base64 32 | tr -d '=' | tr '+/' '-_')
```

### Timing & Limits

| Parameter                 | Value              | Description                                       |
|---------------------------|--------------------|---------------------------------------------------|
| Pending auth TTL          | 5 minutes          | User must complete browser auth within this time  |
| Completed auth TTL        | 2 minutes          | CLI must poll and retrieve token within this time |
| Rate limit                | 60 requests/minute | Per state, returns 429 if exceeded                |
| Recommended poll interval | 2-3 seconds        | Balance between responsiveness and server load    |

### Using the Token

Once obtained, include the JWT in the Cookie header for API requests:

```bash
curl -H "Cookie: token=${TOKEN}" http://localhost:9000/api/admin/tenants
```

Or use it as a Bearer token (if supported):
```bash
curl -H "Authorization: Bearer ${TOKEN}" http://localhost:9000/api/admin/tenants
```

### Security Features

| Feature           | Description                                          |
|-------------------|------------------------------------------------------|
| State entropy     | 256 bits prevents guessing                           |
| Single-use tokens | Token deleted after first successful claim           |
| Rate limiting     | 60 polls/minute prevents brute force                 |
| Short TTLs        | 5 min pending, 2 min completed limits exposure       |
| PKCE support      | Code verifier stored server-side for CLI flow        |
| cli: prefix       | Distinguishes CLI flow from browser flow in callback |

### Why `cli:` Prefix in State?

The backend uses a `cli:` prefix on the state parameter when redirecting to the OIDC provider:

- **Purpose**: Allows the callback handler to distinguish CLI auth from browser auth
- **Why not separate callback URL?**: Would require OIDC provider configuration changes
- **Why not query param?**: Some providers strip unknown query params
- **Safe?**: Yes - OAuth 2.0 spec says state is opaque; providers pass it through unchanged

The CLI sends `state=abc123`, backend sends `state=cli:abc123` to OIDC provider, callback receives `cli:abc123`, backend strips prefix and looks up `abc123`.

### Storage Backend (for clustered deployments)

By default, pending/completed auth states are stored in-memory. For clustered deployments with multiple Izanami instances behind a load balancer, configure PostgreSQL storage:

```bash
export IZANAMI_CLI_AUTH_STORAGE=postgresql
```

This ensures the CLI login request, OIDC callback, and token polling can hit different instances.

### Server Files Created/Modified

| File | Action | Purpose |
|------|--------|---------|
| `app/fr/maif/izanami/models/CliAuthentication.scala` | Create | Data models |
| `app/fr/maif/izanami/datastores/CliAuthDatastore.scala` | Create | In-memory storage |
| `app/fr/maif/izanami/web/LoginController.scala` | Modify | Add CLI endpoints |
| `conf/routes` | Modify | Add routes |
| `app/views/cliAuthSuccess.scala.html` | Create | Success page |
| `izanami-frontend/src/pages/login.tsx` | Modify | Pass state parameter |

---

## CLI Implementation

### CLI Implementation Checklist

1. Generate secure state (32 bytes, base64url)
2. Open browser to `/api/admin/cli-login?state={state}`
3. Display message to user: "Complete authentication in browser..."
4. Poll `/api/admin/cli-token?state={state}` every 2-3 seconds
5. Handle responses:
   - 202: Continue polling
   - 200: Extract token, stop polling, store token
   - 429: Wait for `retryAfter` seconds, then resume
   - 404/410: Authentication failed/expired, abort
6. Store token securely (e.g., in config file with restricted permissions)
7. Use token in subsequent API requests

### New Flags

```go
// Already implemented (Phase 1)
loginCmd.Flags().BoolVar(&loginOIDC, "oidc", false, "Use OIDC authentication")
loginCmd.Flags().StringVar(&loginToken, "token", "", "JWT token for OIDC (skip browser flow)")
loginCmd.Flags().BoolVar(&loginNoBrowser, "no-browser", false, "Don't open browser, just print URL")

// To be added for Phase 2
loginCmd.Flags().DurationVar(&loginTimeout, "timeout", 5*time.Minute, "OIDC authentication timeout")
loginCmd.Flags().DurationVar(&loginPollInterval, "poll-interval", 2*time.Second, "Token polling interval")
```

### State Generator

```go
// internal/auth/state.go

import (
    "crypto/rand"
    "encoding/base64"
)

// GenerateState generates a cryptographically secure state parameter
// Returns 32 bytes (256 bits) encoded as base64url
func GenerateState() (string, error) {
    bytes := make([]byte, 32)
    if _, err := rand.Read(bytes); err != nil {
        return "", fmt.Errorf("failed to generate random state: %w", err)
    }
    return base64.RawURLEncoding.EncodeToString(bytes), nil
}
```

### Token Poller

```go
// internal/auth/poller.go

type TokenPoller struct {
    client       *http.Client
    baseURL      string
    state        string
    pollInterval time.Duration
}

type PollResult struct {
    Token      string
    Ready      bool
    RetryAfter int // seconds to wait if rate limited
}

func NewTokenPoller(baseURL, state string, interval time.Duration) *TokenPoller {
    return &TokenPoller{
        client:       &http.Client{Timeout: 10 * time.Second},
        baseURL:      strings.TrimSuffix(baseURL, "/"),
        state:        state,
        pollInterval: interval,
    }
}

func (p *TokenPoller) Poll(ctx context.Context) (*PollResult, error) {
    url := fmt.Sprintf("%s/api/admin/cli-token?state=%s", p.baseURL, p.state)

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }

    resp, err := p.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    switch resp.StatusCode {
    case http.StatusOK:
        // Token ready
        var result struct {
            Token string `json:"token"`
        }
        if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
            return nil, err
        }
        return &PollResult{Token: result.Token, Ready: true}, nil

    case http.StatusAccepted:
        // Still pending
        return &PollResult{Ready: false}, nil

    case http.StatusNotFound:
        return nil, fmt.Errorf("invalid or unknown state")

    case http.StatusGone:
        return nil, fmt.Errorf("authentication expired")

    case http.StatusTooManyRequests:
        var result struct {
            RetryAfter int `json:"retryAfter"`
        }
        json.NewDecoder(resp.Body).Decode(&result)
        if result.RetryAfter == 0 {
            result.RetryAfter = 5
        }
        return &PollResult{Ready: false, RetryAfter: result.RetryAfter}, nil

    default:
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
    }
}

func (p *TokenPoller) WaitForToken(ctx context.Context, timeout time.Duration) (string, error) {
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    ticker := time.NewTicker(p.pollInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return "", fmt.Errorf("authentication timed out")
        case <-ticker.C:
            result, err := p.Poll(ctx)
            if err != nil {
                return "", err
            }
            if result.Ready {
                return result.Token, nil
            }
            if result.RetryAfter > 0 {
                // Rate limited - wait before next poll
                ticker.Reset(time.Duration(result.RetryAfter) * time.Second)
            }
            // Continue polling
        }
    }
}
```

### Updated OIDC Login Flow

```go
// internal/cmd/login.go

func runOIDCLogin(cmd *cobra.Command, args []string) error {
    // Get base URL
    baseURL := getBaseURL(args)
    if baseURL == "" {
        return fmt.Errorf("base URL is required")
    }
    baseURL = strings.TrimSuffix(baseURL, "/")

    // If token provided via flag, skip browser flow
    if loginToken != "" {
        return saveOIDCSession(cmd, baseURL, loginToken)
    }

    // Generate state for this authentication session
    state, err := auth.GenerateState()
    if err != nil {
        return fmt.Errorf("failed to generate state: %w", err)
    }

    // Build CLI login URL
    loginURL := fmt.Sprintf("%s/api/admin/cli-login?state=%s", baseURL, state)

    // Open browser
    if !loginNoBrowser {
        fmt.Fprintln(cmd.OutOrStderr(), "Opening browser for OIDC authentication...")
        if err := utils.OpenBrowser(loginURL); err != nil {
            fmt.Fprintf(cmd.OutOrStderr(), "Warning: Could not open browser: %v\n", err)
        }
    }

    fmt.Fprintf(cmd.OutOrStderr(), "\nIf browser doesn't open, visit:\n  %s\n\n", loginURL)
    fmt.Fprintln(cmd.OutOrStderr(), "Waiting for authentication...")

    // Poll for token
    poller := auth.NewTokenPoller(baseURL, state, loginPollInterval)
    token, err := poller.WaitForToken(context.Background(), loginTimeout)
    if err != nil {
        return fmt.Errorf("authentication failed: %w", err)
    }

    // Save session
    return saveOIDCSession(cmd, baseURL, token)
}
```

### Browser Opening Utility

**Already Implemented** in `internal/utils/browser.go` with WSL support:

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

### CLI Files to Create/Modify

| File | Action | Purpose |
|------|--------|---------|
| `internal/auth/state.go` | Create | State generation |
| `internal/auth/poller.go` | Create | Token polling logic |
| `internal/cmd/login.go` | Modify | Add polling flow |

---

## CLI User Experience

```bash
$ iz login --oidc --url https://izanami.example.com
Opening browser for OIDC authentication...

If browser doesn't open, visit:
  https://izanami.example.com/api/admin/cli-login?state=abc123...

Waiting for authentication...
Successfully logged in as user@example.com (via OIDC)
   Session saved as: izanami-example-com-oidc

$ iz login --oidc --url https://izanami.example.com --no-browser
If browser doesn't open, visit:
  https://izanami.example.com/api/admin/cli-login?state=abc123...

Waiting for authentication...
```

---

## References

- [RFC 6749 - OAuth 2.0](https://tools.ietf.org/html/rfc6749)
- [RFC 7636 - PKCE](https://tools.ietf.org/html/rfc7636)
- [RFC 8628 - Device Authorization Grant](https://tools.ietf.org/html/rfc8628)
- [Google Cloud CLI Authentication](https://cloud.google.com/sdk/gcloud/reference/auth/login)
- [Azure CLI Authentication](https://docs.microsoft.com/en-us/cli/azure/authenticate-azure-cli)
- [GitHub CLI Authentication](https://cli.github.com/manual/gh_auth_login)
