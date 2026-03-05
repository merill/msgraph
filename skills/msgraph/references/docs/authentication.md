# Authentication

This reference covers detailed authentication configuration for the msgraph skill. For a quick overview, see the Authentication section in the main SKILL.md.

## Auth Method Detection

The auth method is auto-detected from environment variables in priority order:

1. `MSGRAPH_CLIENT_SECRET` set → **Client secret**
2. `MSGRAPH_CLIENT_CERTIFICATE_PATH` set → **Certificate**
3. `MSGRAPH_FEDERATED_TOKEN_FILE` (or `AZURE_FEDERATED_TOKEN_FILE` / `AWS_WEB_IDENTITY_TOKEN_FILE`) set → **Workload identity**
4. `MSGRAPH_AUTH_METHOD=managed-identity` → **Managed identity**
5. None of the above → **Delegated** (interactive browser / device code)

## Delegated Auth

Used when no app-only env vars are set. A user signs in interactively.

### Interactive Browser (default)

Opens the system browser for sign-in. This is the default when a browser is available.

```
msgraph auth signin
```

### Device Code

For headless environments (SSH, containers, CI), use device code flow:

```
msgraph auth signin --device-code
```

The tool prints a URL and code to stderr. Open the URL in any browser, enter the code, and authenticate.

Auto-detection: if the tool detects an SSH session or no display server, it automatically falls back to device code.

### Requesting Specific Scopes

You can request specific scopes at sign-in:

```
msgraph auth signin --scopes "Mail.Read,Calendars.Read"
```

This is useful when you know upfront what permissions you'll need.

### Incremental Consent

When a Graph API call returns 403 Forbidden, the tool:

1. Parses the error message to extract required permission scopes (regex: `[A-Z][a-zA-Z]+\.[A-Z][a-zA-Z]+`)
2. Merges new scopes with existing scopes
3. Re-authenticates with the combined scope set
4. Retries the original request with the new token

This happens transparently — no manual scope management needed.

### Session-Scoped Cache

Tokens are cached in a session-scoped temporary file (`os.TempDir()`) for the duration of the session. The cache is keyed by client ID and tenant ID. No credentials are persisted permanently. The cache file is automatically cleaned up on sign-out.

## App-Only Auth

For automation, CI/CD pipelines, and service-to-service scenarios.

**IMPORTANT**: App-only auth requires `MSGRAPH_TENANT_ID` set to a specific tenant (not `common`). The tool errors early with a clear message if this is missing. Incremental consent is not available — all permissions must be pre-configured and admin-consented in the Entra ID app registration.

All pre-granted application permissions are used via the `https://graph.microsoft.com/.default` scope. The `--device-code` and `--scopes` flags are ignored for app-only auth.

### Client Secret

The simplest app-only method. Set the secret from your Entra ID app registration:

```
export MSGRAPH_CLIENT_ID="your-app-id"
export MSGRAPH_TENANT_ID="contoso.onmicrosoft.com"
export MSGRAPH_CLIENT_SECRET="your-secret-value"
msgraph auth signin
```

**Setup in Entra ID:**
1. Go to **App Registrations** > your app > **Certificates & secrets**
2. Add a new client secret
3. Copy the secret value (it is only shown once)
4. Under **API permissions**, add the Microsoft Graph **Application** permissions you need and grant admin consent

### Client Certificate

More secure than client secrets — uses a certificate for authentication:

```
export MSGRAPH_CLIENT_ID="your-app-id"
export MSGRAPH_TENANT_ID="contoso.onmicrosoft.com"
export MSGRAPH_CLIENT_CERTIFICATE_PATH="/path/to/cert.pem"
msgraph auth signin
```

The PEM file must contain both the certificate and private key. RSA, ECDSA, and PKCS#8 private keys are supported.

If the private key is encrypted:

```
export MSGRAPH_CLIENT_CERTIFICATE_PASSWORD="key-password"
```

**Setup in Entra ID:**
1. Go to **App Registrations** > your app > **Certificates & secrets**
2. Upload the public certificate (.cer or .pem)
3. Under **API permissions**, add the Microsoft Graph **Application** permissions you need and grant admin consent

### Managed Identity

For workloads running on Azure (VMs, App Service, Azure Functions, AKS):

```
export MSGRAPH_AUTH_METHOD="managed-identity"
msgraph auth signin
```

For **user-assigned** managed identities, also set the client ID:

```
export MSGRAPH_AUTH_METHOD="managed-identity"
export MSGRAPH_MANAGED_IDENTITY_CLIENT_ID="your-managed-identity-client-id"
msgraph auth signin
```

No client secret or certificate is needed — Azure handles credential management automatically.

**Setup:**
1. Enable managed identity on your Azure resource
2. In the Entra ID enterprise application for the managed identity, assign the Microsoft Graph app roles you need (via PowerShell or Azure CLI)

### Workload Identity Federation

For workloads running outside Azure (GitHub Actions, GCP, AWS, Kubernetes) that exchange a platform token for a Microsoft Entra ID token:

```
export MSGRAPH_CLIENT_ID="your-app-id"
export MSGRAPH_TENANT_ID="contoso.onmicrosoft.com"
export MSGRAPH_FEDERATED_TOKEN_FILE="/var/run/secrets/token"
msgraph auth signin
```

The tool also auto-reads these standard environment variables:
- `AZURE_FEDERATED_TOKEN_FILE` — set by AKS workload identity
- `AWS_WEB_IDENTITY_TOKEN_FILE` — set by AWS IRSA / EKS

For AKS workload identity, `AZURE_CLIENT_ID` and `AZURE_TENANT_ID` are used as fallbacks if `MSGRAPH_CLIENT_ID` / `MSGRAPH_TENANT_ID` are not set.

The token file is re-read on each token acquisition, so token rotation is handled automatically.

**Setup in Entra ID:**
1. Go to **App Registrations** > your app > **Certificates & secrets** > **Federated credentials**
2. Add a federated credential for your platform (GitHub Actions, Kubernetes, etc.)
3. Under **API permissions**, add the Microsoft Graph **Application** permissions you need and grant admin consent

## Auth Commands

| Command | Description |
|---|---|
| `msgraph auth signin` | Sign in (delegated) or verify credentials (app-only) |
| `msgraph auth signin --device-code` | Force device code flow (delegated only) |
| `msgraph auth signin --scopes "Mail.Read,Calendars.Read"` | Request specific scopes (delegated only) |
| `msgraph auth signout` | Clear the current session |
| `msgraph auth status` | Check sign-in state and account info |
| `msgraph auth switch-tenant <tenant-id>` | Switch to a different M365 tenant |

## Custom Client ID

By default, msgraph uses the **Microsoft Graph Command Line Tools** app ID (`14d82eec-204b-4c2f-b7e8-296a70dab67e`). This is a first-party Microsoft app pre-registered in most M365 tenants.

To use your own Entra ID app registration:

```
export MSGRAPH_CLIENT_ID="your-custom-app-id"
msgraph auth signin
```

## All Authentication Environment Variables

| Variable | Description | Default |
|---|---|---|
| `MSGRAPH_CLIENT_ID` | Custom Entra ID app client ID | Microsoft Graph CLI Tools app |
| `MSGRAPH_TENANT_ID` | Target tenant ID (required for app-only) | `common` |
| `MSGRAPH_CLIENT_SECRET` | App registration client secret | — |
| `MSGRAPH_CLIENT_CERTIFICATE_PATH` | Path to PEM certificate file | — |
| `MSGRAPH_CLIENT_CERTIFICATE_PASSWORD` | Password for encrypted certificate key | — |
| `MSGRAPH_AUTH_METHOD` | Set to `managed-identity` for Azure managed identity | — |
| `MSGRAPH_MANAGED_IDENTITY_CLIENT_ID` | Client ID for user-assigned managed identity | — |
| `MSGRAPH_FEDERATED_TOKEN_FILE` | Path to federated token file (workload identity) | — |

Also auto-reads: `AZURE_FEDERATED_TOKEN_FILE`, `AWS_WEB_IDENTITY_TOKEN_FILE`, `AZURE_CLIENT_ID`, `AZURE_TENANT_ID`.

## Silent Token Acquisition

After initial sign-in, tokens are cached. Subsequent calls use silent acquisition:

1. Check the in-memory/file cache for a valid token
2. If expired, MSAL automatically refreshes using the refresh token (delegated) or re-acquires via credentials (app-only)
3. Only if refresh fails does it prompt for interactive auth (delegated only)
