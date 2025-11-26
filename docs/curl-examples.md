# Izanami API - Curl Examples

This document contains the most common curl commands for interacting with the Izanami server.

## Authentication Methods

Izanami supports two authentication methods:

1. **JWT Cookie Authentication** - For admin operations (managing tenants, projects, features, users)
2. **Client Credentials** - For client operations (checking feature flags)

---

## 1. Login to Get JWT Cookie

Authenticate with username/password to obtain a JWT token stored as a cookie.

```bash
curl -X POST "https://<izanami-server>/api/admin/login" \
  -u "admin:password" \
  -c cookies.txt \
  -v
```

**Parameters:**
- `-u "admin:password"` - Basic Auth credentials
- `-c cookies.txt` - Save cookies (including JWT token) to file
- `-v` - Verbose output to see the `Set-Cookie` header

**Response:** The JWT token is returned in the `Set-Cookie: token=<jwt>` header.

---

## 2. Get Tenant List (with JWT)

List all tenants accessible to the authenticated user.

```bash
curl -X GET "https://<izanami-server>/api/admin/tenants" \
  -b cookies.txt
```

**Parameters:**
- `-b cookies.txt` - Use saved cookies for authentication

**Optional query parameters:**
- `?right=Read` - Filter tenants by permission level (`Read`, `Write`, or `Admin`)

**Example with filter:**
```bash
curl -X GET "https://<izanami-server>/api/admin/tenants?right=Admin" \
  -b cookies.txt
```

**Response:**
```json
[
  {
    "name": "my-tenant",
    "description": "My tenant description"
  }
]
```

---

## 3. Check Single Feature (with Client Credentials)

Check if a feature flag is active using API key authentication.

```bash
curl -X GET "https://<izanami-server>/api/v2/features/<feature-id>" \
  -H "Izanami-Client-Id: <client-id>" \
  -H "Izanami-Client-Secret: <client-secret>"
```

**Required headers:**
- `Izanami-Client-Id` - Your API key client ID
- `Izanami-Client-Secret` - Your API key client secret

**Optional query parameters:**
- `user=<userId>` - User identifier for user-targeted features
- `context=<contextPath>` - Context path for context-based evaluation

**Example with context:**
```bash
curl -X GET "https://<izanami-server>/api/v2/features/my-feature?user=user123&context=/prod/eu" \
  -H "Izanami-Client-Id: my-client-id" \
  -H "Izanami-Client-Secret: my-client-secret"
```

**Response:**
```json
{
  "active": true,
  "name": "my-feature",
  "project": "my-project"
}
```

---

## Additional Useful Commands

### Health Check (No Authentication)

```bash
curl -X GET "https://<izanami-server>/api/_health"
```

### Check Multiple Features

```bash
curl -X GET "https://<izanami-server>/api/v2/features?features=feature1,feature2,feature3" \
  -H "Izanami-Client-Id: <client-id>" \
  -H "Izanami-Client-Secret: <client-secret>"
```

### List Projects in a Tenant (with JWT)

```bash
curl -X GET "https://<izanami-server>/api/admin/tenants/<tenant>/projects" \
  -b cookies.txt
```

### List Features in a Tenant (with JWT)

```bash
curl -X GET "https://<izanami-server>/api/admin/tenants/<tenant>/features" \
  -b cookies.txt
```

### List API Keys for a Tenant (with JWT)

```bash
curl -X GET "https://<izanami-server>/api/admin/tenants/<tenant>/keys" \
  -b cookies.txt
```
