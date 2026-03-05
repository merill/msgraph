# msgraph

An [Agent Skill](https://agentskills.io) that gives AI agents up-to-date knowledge of the complete Microsoft Graph API — and optionally executes calls directly.

## The Problem

LLMs are trained on data that is months old. The Microsoft Graph API has **27,700 Graph APIs** and is updated weekly. Without current API knowledge, agents hallucinate endpoints, use deprecated paths, and miss required permissions.

## The Solution

This skill bundles the complete Microsoft Graph API surface as local indexes — searchable instantly with zero network calls.

| Index | Count |
|---|---|
| OpenAPI endpoints | **27,700+** |
| Endpoint docs (permissions, query params, headers) | **6,200+** |
| Resource schemas (properties, types, filter operators) | **4,200+** |
| Community samples | **Growing** |

## Features

- **Complete Microsoft Graph API Knowledge** — 27,700 Graph APIs, all indexed locally
- **Instant Local Search** — All lookups run locally in milliseconds, no network calls needed
- **Community Samples** — Curated, hand-verified samples mapping tasks to exact API queries
- **MCP Server Compatible** — Works with [lokka.dev](https://lokka.dev) or any Microsoft Graph MCP server for execution
- **Direct API Execution** — Authenticate and call the Microsoft Graph API directly when no MCP server is available
- **MSAL Authentication** — Interactive browser flow with device code fallback, plus app-only auth
- **Safe by Default** — GET operations allowed; write operations require explicit confirmation
- **Cross-Platform** — Works on macOS, Linux, and Windows (amd64/arm64) with zero runtime dependencies

## Quick Start

### Install the Skill

**Using [skills.sh](https://skills.sh) (recommended):**

```bash
npx skills add merill/msgraph
```

**Or download from GitHub Releases:**

Download `msgraph.zip` from the [latest release](https://github.com/merill/msgraph/releases/latest), then extract it into your agent's skills directory:

```bash
# Download and extract
curl -fsSL -o msgraph.zip https://github.com/merill/msgraph/releases/latest/download/msgraph.zip
unzip msgraph.zip -d ~/.claude/skills/
```

### Search the Microsoft Graph API (no auth needed)

```bash
# Search curated samples
msgraph sample-search --query "conditional access policies"

# Look up endpoint docs with permissions
msgraph api-docs-search --endpoint /users --method GET

# Search 27,700 Graph APIs
msgraph openapi-search --query "send mail"

# Look up resource schema and filter operators
msgraph api-docs-search --resource user
```

### Execute Microsoft Graph API Calls (auth required)

```bash
# Sign in
msgraph auth signin

# Get current user profile
msgraph graph-call GET /me

# List messages
msgraph graph-call GET /me/messages --top 10
```

## Configuration

| Environment Variable | Description | Default |
|---|---|---|
| `MSGRAPH_CLIENT_ID` | Override the default app client ID | `14d82eec-204b-4c2f-b7e8-296a70dab67e` |
| `MSGRAPH_TENANT_ID` | Target a specific tenant | `common` |
| `MSGRAPH_API_VERSION` | Default API version (`beta` or `v1.0`) | `beta` |

## Building from Source

Requires [Go 1.22+](https://go.dev/dl/) to build from source.

```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Run the OpenAPI indexer
make index
```

## Project Structure

```
msgraph/
├── skills/msgraph/   # The installable Agent Skill
│   ├── SKILL.md      # Agent Skills spec entry point
│   ├── scripts/      # Launcher scripts + binary cache
│   └── references/   # OpenAPI index, API docs index, samples, reference docs
├── cmd/              # CLI subcommands (Cobra)
├── internal/         # Internal packages
├── tools/            # Build-time tools (OpenAPI indexer)
├── docs/             # Documentation site (Astro + Starlight)
└── .github/          # CI/CD workflows
```

## License

[MIT](LICENSE)
