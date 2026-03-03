---
name: msgraph
description: Execute Microsoft Graph API calls against Microsoft 365 tenants. Use when the user asks about Microsoft 365 data including users, groups, mail, calendar, Teams, SharePoint, OneDrive, Intune, Entra ID, or any Azure AD / Microsoft 365 administration task.
license: MIT
compatibility: Requires network access to login.microsoftonline.com and graph.microsoft.com. A system browser is used for interactive auth; falls back to device code flow in headless environments.
metadata:
  author: merill
  version: "1.0"
---

# Microsoft Graph Agent Skill

You can query and manage Microsoft 365 data through the Microsoft Graph API using the `msgraph` CLI.

## Quick Start

1. **Check auth status** before making any Graph call:
   ```
   msgraph auth status
   ```

2. **Sign in** if not authenticated:
   ```
   msgraph auth signin
   ```

3. **Make Graph API calls**:
   ```
   msgraph graph-call GET /me
   ```

## Authentication

The tool supports both **delegated (user)** and **app-only (application)** authentication via Microsoft's identity platform (MSAL). The auth method is auto-detected from environment variables.

### Delegated Auth (default)

Used when no app-only env vars are set. A user signs in interactively.

- **Interactive browser auth** is the default — a browser window opens for sign-in
- **Device code flow** is used automatically when no browser is detected (SSH sessions, containers) or can be forced with `--device-code`
- **Incremental consent** is handled automatically — if a 403 Forbidden is returned, the tool re-authenticates with the required scopes and retries
- **Session-scoped cache** — tokens are cached in a temp file for the session only; no persistent credential storage

### App-Only Auth

For automation, CI/CD, and service-to-service scenarios. The auth method is auto-detected from environment variables in this priority order:

1. **Client secret** — `MSGRAPH_CLIENT_SECRET` is set
2. **Client certificate** — `MSGRAPH_CLIENT_CERTIFICATE_PATH` is set
3. **Workload identity federation** — `MSGRAPH_FEDERATED_TOKEN_FILE` (or `AZURE_FEDERATED_TOKEN_FILE` / `AWS_WEB_IDENTITY_TOKEN_FILE`) is set
4. **Managed identity** — `MSGRAPH_AUTH_METHOD=managed-identity` is set

**Important**: App-only auth requires `MSGRAPH_TENANT_ID` set to a specific tenant (not `common`). The tool uses `https://graph.microsoft.com/.default` scope which grants all pre-assigned application permissions. Incremental consent is not available for app-only auth.

### Auth Commands

| Command | Description |
|---|---|
| `msgraph auth signin` | Sign in to Microsoft 365 (delegated) or verify credentials (app-only) |
| `msgraph auth signin --device-code` | Sign in using device code flow (delegated only) |
| `msgraph auth signin --scopes "Mail.Read,Calendars.Read"` | Sign in requesting specific scopes (delegated only) |
| `msgraph auth signout` | Clear the current session |
| `msgraph auth status` | Check if signed in and show account/auth method info |
| `msgraph auth switch-tenant <tenant-id>` | Switch to a different M365 tenant |

## Making Graph API Calls

Use `msgraph graph-call <METHOD> <URL>` to execute REST calls against the Graph API.

### Read Operations (default)

```bash
# Get current user profile
msgraph graph-call GET /me

# List users with selected fields
msgraph graph-call GET /users --select "displayName,mail,userPrincipalName" --top 10

# Get user's mail with filtering
msgraph graph-call GET /me/messages --filter "isRead eq false" --top 5 --select "subject,from,receivedDateTime"

# List groups
msgraph graph-call GET /groups --select "displayName,description" --top 25

# Get team channels
msgraph graph-call GET /teams/{team-id}/channels

# Search users
msgraph graph-call GET /users --filter "startsWith(displayName,'John')"
```

### Write Operations (requires --allow-writes)

**IMPORTANT**: Before making any write operation (POST, PUT, PATCH), you MUST ask the user for confirmation. Write operations require the `--allow-writes` flag.

```bash
# Send a message (ask user first!)
msgraph graph-call POST /me/sendMail --body '{"message":{"subject":"Hello","body":{"content":"Hi there"},"toRecipients":[{"emailAddress":{"address":"user@example.com"}}]}}' --allow-writes

# Update user properties (ask user first!)
msgraph graph-call PATCH /me --body '{"jobTitle":"Engineer"}' --allow-writes
```

**DELETE operations are always blocked** for safety regardless of flags.

### Query Parameters

| Flag | Description | Example |
|---|---|---|
| `--select` | OData $select | `--select "displayName,mail"` |
| `--filter` | OData $filter | `--filter "isRead eq false"` |
| `--top` | OData $top (limit results) | `--top 10` |
| `--expand` | OData $expand | `--expand "members"` |
| `--orderby` | OData $orderby | `--orderby "displayName"` |
| `--api-version` | API version (v1.0 or beta) | `--api-version v1.0` |
| `--scopes` | Request additional scopes | `--scopes "Mail.Read"` |
| `--headers` | Custom HTTP headers | `--headers "ConsistencyLevel:eventual"` |
| `--output` | Output format (json or raw) | `--output raw` |

## Finding the Right Endpoint

### Strategy

1. **First**: Try constructing the Graph API call from your training knowledge. The Microsoft Graph API follows consistent patterns:
   - `/me` — current user
   - `/users` — all users
   - `/users/{id}` — specific user
   - `/me/messages` — current user's mail
   - `/groups` — all groups
   - `/teams/{id}/channels` — team channels

2. **If unsure**: Use the OpenAPI search command to find endpoints:
   ```
   msgraph openapi-search --query "send mail"
   msgraph openapi-search --resource users --method GET
   msgraph openapi-search --query "calendar events" --method POST
   ```

3. **Check the reference** for detailed API documentation:
   See [references/REFERENCE.md](references/REFERENCE.md) for common Graph API patterns and endpoint details.

### OpenAPI Search Command

```bash
# Search by keyword
msgraph openapi-search --query "list users"

# Search by resource and method
msgraph openapi-search --resource messages --method GET

# Combined search
msgraph openapi-search --query "create" --resource groups --method POST
```

## Important Rules

1. **Always check auth status** before the first Graph call in a session
2. **GET requests are the default** — no special flags needed
3. **Write operations require `--allow-writes`** — always confirm with the user first
4. **DELETE is always blocked** — inform the user this is not supported
5. **The default API version is beta** — use `--api-version v1.0` for production-stable endpoints
6. **403 errors trigger automatic re-auth** — the tool will request additional scopes and retry
7. **All output is JSON** — parse the `statusCode` and `body` fields from the response

## Error Handling

- **401 Unauthorized**: Token expired. Run `msgraph auth signin` again.
- **403 Forbidden**: Insufficient permissions. The tool automatically attempts incremental consent. If it still fails, the user may need admin consent for those permissions.
- **404 Not Found**: The resource doesn't exist or the URL is incorrect. Verify the endpoint path.
- **429 Too Many Requests**: Rate limited. Wait and retry.

## Environment Variables

| Variable | Description | Default |
|---|---|---|
| `MSGRAPH_CLIENT_ID` | Custom Entra ID app registration client ID | Microsoft Graph CLI Tools app |
| `MSGRAPH_TENANT_ID` | Target tenant ID (required for app-only auth) | `common` (multi-tenant) |
| `MSGRAPH_API_VERSION` | Default API version | `beta` |
| `MSGRAPH_INDEX_PATH` | Path to OpenAPI index JSON | Auto-detected |
| `MSGRAPH_CLIENT_SECRET` | App registration client secret | — |
| `MSGRAPH_CLIENT_CERTIFICATE_PATH` | Path to PEM certificate file | — |
| `MSGRAPH_CLIENT_CERTIFICATE_PASSWORD` | Password for encrypted certificate key | — |
| `MSGRAPH_AUTH_METHOD` | Set to `managed-identity` to use Azure managed identity | — |
| `MSGRAPH_MANAGED_IDENTITY_CLIENT_ID` | Client ID for user-assigned managed identity | — |
| `MSGRAPH_FEDERATED_TOKEN_FILE` | Path to federated token file (workload identity) | — |

Also auto-reads: `AZURE_FEDERATED_TOKEN_FILE`, `AWS_WEB_IDENTITY_TOKEN_FILE`, `AZURE_CLIENT_ID`, `AZURE_TENANT_ID`.
