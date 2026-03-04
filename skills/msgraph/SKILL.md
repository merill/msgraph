---
name: msgraph
description: Execute Microsoft Graph API calls against Microsoft 365 tenants. Use when the user asks about Microsoft 365 data including users, groups, mail, calendar, Teams, SharePoint, OneDrive, Intune, Entra ID, or any Azure AD / Microsoft 365 administration task. Provides progressive lookup tools — from curated samples to full API docs to 27K+ OpenAPI endpoints — so the agent always finds the right Graph API call even when training data is outdated.
license: MIT
compatibility: Requires network access to login.microsoftonline.com and graph.microsoft.com. A system browser is used for interactive auth; falls back to device code flow in headless environments.
metadata:
  author: merill
  version: "1.0.9"
---

# Microsoft Graph Agent Skill

Query and manage Microsoft 365 data through the Microsoft Graph REST API using the `msgraph` CLI.

The Graph API is updated weekly. Your training data may be stale. Use the lookup tools below to verify endpoints, permissions, and syntax before making calls.

## How to Run

The `msgraph` CLI is bundled with this skill. Run all commands through the launcher script in this skill's directory:

- **macOS / Linux**: `bash <path-to-this-skill>/scripts/run.sh <command> [args...]`
- **Windows**: `powershell <path-to-this-skill>/scripts/run.ps1 <command> [args...]`

In all examples below, `msgraph` is shorthand for the full launcher invocation.

## Quick Start

```
msgraph auth status          # check if signed in
msgraph auth signin          # sign in if needed
msgraph graph-call GET /me   # make a Graph API call
```

## Finding the Right API Call

Follow this progressive lookup strategy. Each level adds detail:

1. **Your own knowledge** — try first for well-known endpoints (`/me`, `/users`, `/groups`).
2. **`sample-search`** — curated, hand-verified samples. Highest quality. Use for common tasks and multi-step workflows.
3. **`api-docs-search`** — per-endpoint permissions, supported query parameters, required headers, default vs $select-only properties, and resource property details with filter operators.
4. **`openapi-search`** — full catalog of 27,000+ endpoints. Use when you cannot find the endpoint any other way.
5. **Reference files** — concept docs on query parameters, advanced queries, paging, batching, throttling, errors, and best practices. Read only when you need specific guidance.

This order is guidance — adapt based on the task. For example, jump straight to `api-docs-search` if you already know the endpoint but need its permissions.

### sample-search

Search curated community samples that map natural-language tasks to exact Graph API queries:

```
msgraph sample-search --query "conditional access policies"
msgraph sample-search --product entra
msgraph sample-search --query "managed devices" --product intune
```

| Flag | Description |
|---|---|
| `--query` | Free-text search (searches intent and query fields) |
| `--product` | Filter by product: `entra`, `intune`, `exchange`, `teams`, `sharepoint`, `security`, `general` |
| `--limit` | Max results (default 10) |

At least one of `--query` or `--product` is required. Results include multi-step workflows.

### api-docs-search

Look up detailed documentation for a specific endpoint or resource type:

```
msgraph api-docs-search --endpoint /users --method GET
msgraph api-docs-search --resource user
msgraph api-docs-search --query "ConsistencyLevel"
```

| Flag | Description |
|---|---|
| `--endpoint` | Search by endpoint path (e.g. `/users`, `/me/messages`) |
| `--resource` | Search by resource type name (e.g. `user`, `group`, `message`) |
| `--method` | Filter by HTTP method: `GET`, `POST`, `PUT`, `PATCH` |
| `--query` | Free-text search across all fields |
| `--limit` | Max results (default 10) |

At least one of `--endpoint`, `--resource`, or `--query` is required.

**Endpoint results** include: required permissions (delegated work/school, delegated personal, application), supported OData query parameters, required headers, default properties, and endpoint-specific notes.

**Resource results** include: all properties with types, supported `$filter` operators (eq, ne, startsWith, etc.), and whether each property is returned by default or requires `$select`.

### openapi-search

Search the full OpenAPI catalog of 27,000+ Graph API paths:

```
msgraph openapi-search --query "send mail"
msgraph openapi-search --resource messages --method GET
```

| Flag | Description |
|---|---|
| `--query` | Free-text search (searches path, summary, description) |
| `--resource` | Filter by resource name (e.g. `users`, `groups`, `messages`) |
| `--method` | Filter by HTTP method |
| `--limit` | Max results (default 20) |

At least one of `--query`, `--resource`, or `--method` is required.

## Authentication

**IMPORTANT**: Always run `msgraph auth status` before the first Graph call in a session.

The tool supports both **delegated (user)** and **app-only (application)** authentication. The method is auto-detected from environment variables.

### Delegated Auth (default)

Used when no app-only env vars are set.

- **Interactive browser auth** is the default — opens a browser window for sign-in
- **Device code flow** is used automatically in headless environments (SSH, containers) or forced with `--device-code`
- **Incremental consent** — on 403 Forbidden, the tool re-authenticates with the required scopes and retries automatically
- **Session-scoped cache** — tokens cached in a temp file; no persistent credential storage

### App-Only Auth

For automation and CI/CD. Auto-detected from environment variables in priority order:

1. **Client secret** — `MSGRAPH_CLIENT_SECRET` is set
2. **Client certificate** — `MSGRAPH_CLIENT_CERTIFICATE_PATH` is set
3. **Workload identity** — `MSGRAPH_FEDERATED_TOKEN_FILE` (or `AZURE_FEDERATED_TOKEN_FILE` / `AWS_WEB_IDENTITY_TOKEN_FILE`) is set
4. **Managed identity** — `MSGRAPH_AUTH_METHOD=managed-identity` is set

**IMPORTANT**: App-only auth requires `MSGRAPH_TENANT_ID` set to a specific tenant (not `common`). Incremental consent is not available for app-only auth.

### Auth Commands

| Command | Description |
|---|---|
| `msgraph auth signin` | Sign in (delegated) or verify credentials (app-only) |
| `msgraph auth signin --device-code` | Force device code flow (delegated only) |
| `msgraph auth signin --scopes "Mail.Read,Calendars.Read"` | Request specific scopes (delegated only) |
| `msgraph auth signout` | Clear the current session |
| `msgraph auth status` | Check sign-in state and account info |
| `msgraph auth switch-tenant <tenant-id>` | Switch to a different M365 tenant |

## Making Graph API Calls

```
msgraph graph-call <METHOD> <URL> [flags]
```

### Read Operations

```
msgraph graph-call GET /me
msgraph graph-call GET /users --select "displayName,mail" --top 10
msgraph graph-call GET /me/messages --filter "isRead eq false" --top 5 --select "subject,from,receivedDateTime"
msgraph graph-call GET /users --filter "startsWith(displayName,'John')"
```

### Write Operations

**IMPORTANT**: YOU MUST ask the user for confirmation before any write operation. Write operations require the `--allow-writes` flag.

```
msgraph graph-call POST /me/sendMail --body '{"message":{"subject":"Hello","body":{"content":"Hi"},"toRecipients":[{"emailAddress":{"address":"user@example.com"}}]}}' --allow-writes
msgraph graph-call PATCH /me --body '{"jobTitle":"Engineer"}' --allow-writes
```

**DELETE is always blocked** regardless of flags.

### graph-call Flags

| Flag | Description | Example |
|---|---|---|
| `--select` | OData $select | `--select "displayName,mail"` |
| `--filter` | OData $filter | `--filter "isRead eq false"` |
| `--top` | OData $top (limit results) | `--top 10` |
| `--expand` | OData $expand | `--expand "members"` |
| `--orderby` | OData $orderby | `--orderby "displayName desc"` |
| `--api-version` | `v1.0` or `beta` (default: beta) | `--api-version v1.0` |
| `--scopes` | Request additional permission scopes | `--scopes "Mail.Read"` |
| `--headers` | Custom HTTP headers | `--headers "ConsistencyLevel:eventual"` |
| `--body` | Request body (JSON) | `--body '{"key":"value"}'` |
| `--output` | `json` (default) or `raw` | `--output raw` |
| `--allow-writes` | Allow POST/PUT/PATCH (requires user confirmation) | |

## Critical Rules

1. **Always check auth status** before the first Graph call in a session.
2. **GET is the default** — no special flags needed.
3. **Write operations require `--allow-writes`** — YOU MUST confirm with the user first.
4. **DELETE is always blocked** — inform the user this is not supported.
5. **Default API version is beta** — use `--api-version v1.0` for production-stable endpoints.
6. **403 triggers automatic re-auth** — the tool requests additional scopes and retries.
7. **All output is JSON** — parse `statusCode` and `body` fields from the response.
8. **Use `--select`** to reduce response size — only request fields you need.
9. **Use `--top`** to limit results — avoid fetching thousands of records.
10. **ConsistencyLevel header** is required for `$count` and `$search` on directory objects (users, groups, etc.). Use `--headers "ConsistencyLevel:eventual"`.

## Error Handling

| Status | Meaning | Action |
|---|---|---|
| 401 | Token expired | Run `msgraph auth signin` again |
| 403 | Insufficient permissions | Tool auto-retries with incremental consent. If still fails, user needs admin consent. |
| 404 | Resource not found | Verify the endpoint path |
| 429 | Rate limited | Wait for Retry-After duration, then retry |

## Environment Variables

| Variable | Description | Default |
|---|---|---|
| `MSGRAPH_CLIENT_ID` | Custom Entra ID app client ID | Microsoft Graph CLI Tools app |
| `MSGRAPH_TENANT_ID` | Target tenant ID (required for app-only) | `common` |
| `MSGRAPH_API_VERSION` | Default API version | `beta` |
| `MSGRAPH_CLIENT_SECRET` | App registration client secret | — |
| `MSGRAPH_CLIENT_CERTIFICATE_PATH` | Path to PEM certificate file | — |
| `MSGRAPH_CLIENT_CERTIFICATE_PASSWORD` | Password for encrypted certificate key | — |
| `MSGRAPH_AUTH_METHOD` | Set to `managed-identity` for Azure managed identity | — |
| `MSGRAPH_MANAGED_IDENTITY_CLIENT_ID` | Client ID for user-assigned managed identity | — |
| `MSGRAPH_FEDERATED_TOKEN_FILE` | Path to federated token file (workload identity) | — |
| `MSGRAPH_INDEX_PATH` | Path to OpenAPI index JSON | Auto-detected |
| `MSGRAPH_SAMPLES_PATH` | Path to samples index JSON | Auto-detected |
| `MSGRAPH_API_DOCS_PATH` | Path to API docs index JSON | Auto-detected |

Also auto-reads: `AZURE_FEDERATED_TOKEN_FILE`, `AWS_WEB_IDENTITY_TOKEN_FILE`, `AZURE_CLIENT_ID`, `AZURE_TENANT_ID`.

## Reference Files

Load these on demand when you need specific guidance. Do NOT load them preemptively.

| File | When to Read | Size |
|---|---|---|
| [references/REFERENCE.md](references/REFERENCE.md) | Common resource paths, OData patterns, permission scopes | Quick reference |
| [references/docs/query-parameters.md](references/docs/query-parameters.md) | OData $select, $filter, $expand, $top, $orderby, $search syntax and gotchas | Concept doc |
| [references/docs/advanced-queries.md](references/docs/advanced-queries.md) | ConsistencyLevel header, $count, $search, ne/not/endsWith on directory objects | Concept doc |
| [references/docs/paging.md](references/docs/paging.md) | @odata.nextLink pagination, server-side vs client-side paging | Concept doc |
| [references/docs/batching.md](references/docs/batching.md) | $batch endpoint, combining multiple requests, dependsOn sequencing | Concept doc |
| [references/docs/throttling.md](references/docs/throttling.md) | 429 handling, Retry-After, backoff strategy | Concept doc |
| [references/docs/errors.md](references/docs/errors.md) | HTTP status codes, error response format, error codes | Concept doc |
| [references/docs/best-practices.md](references/docs/best-practices.md) | $select for performance, pagination, delta queries, batching | Concept doc |
