# msgraph-skill

An [Agent Skill](https://agentskills.io) for the Microsoft Graph API. Enables AI agents to authenticate to Microsoft 365 tenants and make Graph API calls.

## Features

- **MSAL Authentication** - Interactive browser flow with device code fallback
- **Graph API Calls** - Execute REST API calls against Microsoft Graph (beta and v1.0)
- **Read-Only by Default** - GET operations allowed; write operations require explicit confirmation
- **Incremental Consent** - Automatically requests additional permissions when needed
- **OpenAPI Lookup** - Pre-processed Graph API index for endpoint discovery
- **Cross-Platform** - Pre-compiled Go binaries for macOS, Linux, and Windows (amd64/arm64)
- **Zero Runtime Dependencies** - Static Go binaries, no runtime installation needed

## Quick Start

### Install the Skill

**Using [skills.sh](https://skills.sh) (recommended):**

```bash
npx skills add merill/msgraph-skill
```

**Or download from GitHub Releases:**

Download `msgraph-skill.zip` from the [latest release](https://github.com/merill/msgraph-skill/releases/latest), then extract it into your agent's skills directory:

```bash
# Download and extract
curl -fsSL -o msgraph-skill.zip https://github.com/merill/msgraph-skill/releases/latest/download/msgraph-skill.zip
unzip msgraph-skill.zip -d ~/.claude/skills/
```

### First Run

The launcher script automatically downloads the correct binary for your platform on first run.

**macOS/Linux:**
```bash
bash ~/.claude/skills/msgraph/scripts/run.sh auth signin
```

**Windows:**
```powershell
powershell ~/.claude/skills/msgraph/scripts/run.ps1 auth signin
```

### Make API Calls

```bash
# Get current user profile
msgraph-skill graph-call GET /me

# List messages
msgraph-skill graph-call GET /me/messages --top 10

# Search the OpenAPI index
msgraph-skill openapi-search --query "send mail"
```

## Configuration

| Environment Variable | Description | Default |
|---|---|---|
| `MSGRAPH_CLIENT_ID` | Override the default app client ID | `14d82eec-204b-4c2f-b7e8-296a70dab67e` |
| `MSGRAPH_TENANT_ID` | Target a specific tenant | `common` |
| `MSGRAPH_API_VERSION` | Default API version (`beta` or `v1.0`) | `beta` |

## Building from Source

Requires [Go 1.22+](https://go.dev/dl/).

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
msgraph-skill/
├── skills/msgraph/   # The installable Agent Skill
│   ├── SKILL.md      # Agent Skills spec entry point
│   ├── scripts/      # Launcher scripts + binary cache
│   └── references/   # OpenAPI index + reference docs
├── cmd/              # CLI subcommands (Cobra)
├── internal/         # Go internal packages
├── tools/            # Build-time tools (OpenAPI indexer)
├── docs/             # Documentation site (Astro + Starlight)
└── .github/          # CI/CD workflows
```

## License

[MIT](LICENSE)
