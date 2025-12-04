# OIDC CLI Authentication

This document describes how the Izanami CLI authenticates users via OIDC (OpenID Connect)
using automatic state-based token polling.

## Overview

The CLI supports OIDC authentication that works with any OIDC provider. When you run
`iz login --oidc`, the CLI:

1. Opens your browser to the Izanami server's OIDC login page
2. Waits for you to complete authentication in the browser
3. Automatically retrieves the authentication token (no copy-paste needed!)
4. Saves the session for future CLI commands

## Quick Start

```bash
# Basic OIDC login
iz login --oidc --url https://izanami.example.com

# With custom timeout (default is 5 minutes)
iz login --oidc --url https://izanami.example.com --timeout 10m

# Without opening browser (just prints URL)
iz login --oidc --url https://izanami.example.com --no-browser

# Direct token (for scripting or when automatic polling isn't available)
iz login --oidc --url https://izanami.example.com --token "eyJhbGciOiJIUzI1NiIs..."
```

## How It Works

### Authentication Flow

```
┌─────────┐                    ┌─────────────┐                    ┌──────────────┐
│   CLI   │                    │   Izanami   │                    │ OIDC Provider│
└────┬────┘                    └──────┬──────┘                    └──────┬───────┘
     │                                │                                  │
     │ 1. Generate state (32 bytes)   │                                  │
     │                                │                                  │
     │ 2. Initiate login (HTTP GET)   │                                  │
     │ ──────────────────────────>    │                                  │
     │    GET /api/admin/cli-login?state={state}                         │
     │                                │                                  │
     │ <──────────────────────────    │ 3. Store pending auth            │
     │    302 Redirect to OIDC        │    Return redirect URL           │
     │                                │                                  │
     │ 4. Open browser to OIDC ───────┼──────────────────────────────────>
     │                                │                                  │
     │ 5. Start polling               │                                  │
     │ ──────────────────────────>    │    6. User authenticates         │
     │    GET /api/admin/cli-token    │ <─────────────────────────────────
     │                                │    Callback with code + state    │
     │ <──────────────────────────    │                                  │
     │    202 Accepted (pending)      │ 7. Detect CLI flow (cli: prefix) │
     │                                │    Exchange code for token       │
     │ ... continue polling ...       │    Store token for CLI pickup    │
     │                                │                                  │
     │ 8. Poll returns token          │                                  │
     │ <──────────────────────────    │                                  │
     │    { "token": "jwt..." }       │                                  │
     │                                │                                  │
     │                                │ 9. Delete pending auth & token   │
     │                                │    (single-use, prevents replay) │
     │                                │                                  │
     │ 10. Save session & use token   │                                  │
     └────────────────────────────────┴──────────────────────────────────┘
```

### Step-by-Step

1. **State Generation**: The CLI generates a cryptographically secure random state
   (32 bytes, base64url encoded). This state correlates your browser authentication
   with your CLI session.

2. **Login Initiation**: The CLI makes an HTTP request to the Izanami server's
   CLI login endpoint. This creates the pending authentication on the server
   and returns a redirect URL to the OIDC provider.

3. **Pending Auth Created**: The server stores the pending authentication state
   and returns a 302 redirect to the OIDC provider with `state=cli:{state}`.

4. **Browser Opens**: The CLI opens your default browser directly to the OIDC
   provider's authorization URL (not the Izanami server).

5. **Polling Starts**: The CLI begins polling the server every 2 seconds asking
   "Is authentication complete for this state?" Initially returns `202 Accepted`.

6. **User Authentication**: You authenticate with your OIDC provider (may include
   MFA, SSO, etc.).

7. **Token Storage**: After successful authentication, the Izanami server detects
   this is a CLI flow (via the `cli:` state prefix) and stores the JWT token for
   CLI pickup instead of setting a browser cookie.

8. **Token Retrieved**: The CLI's polling receives the token:
   - `202 Accepted`: Still waiting, keep polling
   - `200 OK`: Authentication complete, here's your token
   - `404 Not Found`: Invalid or expired state
   - `429 Too Many Requests`: Slow down (rate limited)

9. **Cleanup**: After delivering the token, the server deletes both the pending
   auth and the stored token. This ensures tokens are single-use and prevents
   replay attacks.

10. **Session Saved**: The CLI receives the JWT token and saves it to your
    session file (`~/.izsessions`).

## Command Options

| Flag | Description | Default |
|------|-------------|---------|
| `--oidc` | Enable OIDC authentication | - |
| `--url` | Izanami server URL | - |
| `--timeout` | Maximum time to wait for authentication | 5m |
| `--poll-interval` | Time between token polling requests | 2s |
| `--no-browser` | Don't open browser, just print URL | false |
| `--token` | Provide JWT token directly (skip polling) | - |
| `--name` | Custom session name | auto-generated |
| `-v, --verbose` | Show detailed progress (URLs, states, polling) | false |

## Security Features

| Feature | Description |
|---------|-------------|
| **State Entropy** | 256 bits of randomness prevents state guessing attacks |
| **Single-Use Tokens** | Token is deleted from server after first retrieval |
| **Rate Limiting** | 60 poll requests per minute per state |
| **Short TTLs** | 5 min for pending auth, 2 min for token pickup |
| **PKCE Support** | Server uses PKCE with OIDC provider when available |

## Troubleshooting

### "Authentication timed out"

The 5-minute default timeout expired before you completed OIDC authentication.
This can happen if:
- MFA took too long
- You got distracted during login
- Network issues delayed the response

**Solution**: Run the command again with a longer timeout:
```bash
iz login --oidc --url https://izanami.example.com --timeout 10m
```

### "Server does not support CLI OIDC authentication"

The Izanami server doesn't have the `/api/admin/cli-login` endpoint required
for automatic token polling. This happens with older server versions.

**Solution**: Use the manual token flow:
1. Visit the URL shown in the error message
2. Copy the JWT from browser cookies after login
3. Run: `iz login --oidc --url <url> --token "your-jwt"`

### "Rate limited"

You're polling too frequently. The server enforces 60 requests/minute per state.
This shouldn't happen with default settings.

**Solution**: The CLI automatically respects rate limits and waits before retrying.
If this persists, try increasing `--poll-interval`:
```bash
iz login --oidc --url https://izanami.example.com --poll-interval 5s
```

### Browser doesn't open

On some systems (headless servers, WSL without browser access), the browser
may not open automatically.

**Solution**: Use `--no-browser` and manually visit the printed URL:
```bash
iz login --oidc --url https://izanami.example.com --no-browser
```

### WSL Environment

In Windows Subsystem for Linux (WSL), the CLI automatically detects WSL and
opens the Windows default browser using `rundll32.exe url.dll,FileProtocolHandler`.
This method properly handles URLs with special characters like `&` in query parameters.

## Server Requirements

The automatic token polling requires Izanami server with the following endpoints:

| Endpoint | Purpose |
|----------|---------|
| `GET /api/admin/cli-login?state={state}` | Initiate CLI OIDC flow |
| `GET /api/admin/cli-token?state={state}` | Poll for completed token |

These endpoints were added to support CLI tools and work with any OIDC provider
without requiring special configuration.

## Technical Details

### State Parameter

The state parameter is:
- 32 bytes (256 bits) of cryptographically secure random data
- Encoded as base64url (43 characters)
- Generated using `crypto/rand` (uses system entropy)
- Unique per authentication attempt

### Polling Behavior

The CLI polls at regular intervals with the following behavior:

| Server Response | CLI Action |
|-----------------|------------|
| `200 OK` | Extract token, stop polling, return success |
| `202 Accepted` | Continue polling at normal interval |
| `404 Not Found` | Stop polling, return error (invalid state) |
| `410 Gone` | Stop polling, return error (token expired) |
| `429 Too Many Requests` | Wait `retryAfter` seconds, then continue |

### Token Storage

Sessions are stored in `~/.izsessions` with permissions `0600` (owner read/write only).
The JWT token is stored alongside session metadata (URL, username, created timestamp).

## Related Documentation

- [Feature Specification](../features/feat-0003-oidc-login.md) - Detailed technical specification
- [CLAUDE.md](../CLAUDE.md) - Developer guide
