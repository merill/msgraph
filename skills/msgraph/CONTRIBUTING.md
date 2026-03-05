# Contributing to Microsoft Graph Skill

Thank you for your interest in contributing! This document covers how to add community query samples.

## Why contribute?

Large language models are trained on data that's months old — they don't know about the latest Graph APIs or endpoint changes. Some Graph API docs and endpoints aren't always clear either. Your working query samples go directly into the skill's search index, helping every AI agent using this skill find the right API call.

Read more: [Why Your Samples Matter](https://graph.pm/improve/why-samples-matter/)

## Query Samples

The skill includes a library of community-contributed query samples that help AI agents find the right Microsoft Graph API call. Each sample maps a natural-language intent to an exact API query.

### File format

Each sample is a YAML file with two fields:

```yaml
intent: List all Conditional Access policies
query: GET /beta/identity/conditionalAccess/policies
```

For multi-step queries:

```yaml
intent: Get users assigned to the Global Administrator role
query:
  - GET /v1.0/roleManagement/directory/roleDefinitions?$filter=displayName eq 'Global Administrator'
  - GET /v1.0/roleManagement/directory/roleAssignments?$filter=roleDefinitionId eq '{id from step 1}'&$expand=principal
```

### Directory structure

Place your file in the appropriate product directory under `samples/` at the repo root:

```
samples/
├── entra/          # Entra ID (identity, conditional access, roles, apps)
├── exchange/       # Exchange Online (mail, calendar, mailbox)
├── general/        # Cross-product (licensing, organization, domains)
├── intune/         # Intune (device management, compliance, apps)
├── security/       # Microsoft Security (Defender, alerts, hunting)
├── sharepoint/     # SharePoint Online (sites, lists, documents)
└── teams/          # Microsoft Teams (teams, channels, messages)
```

### How to contribute

1. Fork this repository
2. Create a new `.yaml` file in the appropriate product directory:
   ```
   samples/{product}/{descriptive-name}.yaml
   ```
3. Add your `intent` and `query` fields
4. Submit a pull request

Or use the web form at [graph.pm/improve/add-sample](https://graph.pm/improve/add-sample/) to submit via the browser.

### Guidelines

- **One sample per file** — eliminates merge conflicts when multiple people contribute
- **Use descriptive file names** — `list-conditional-access-policies.yaml`, not `sample1.yaml`
- **Be specific in the intent** — "List all Conditional Access policies" beats "Get policies"
- **Include the full query path** — `GET /beta/identity/conditionalAccess/policies`
- **Use `$select` where appropriate** — helps agents return only needed fields
- **Test your query** — verify it works against the Graph API before submitting

### When to contribute

If an AI agent struggled to find the right Graph API call and you had to manually construct it, that's the perfect candidate for a new sample.

### Build & validation

Run the samples index builder locally to validate your contribution:

```bash
make samples
```

This compiles all YAML files into `skills/msgraph/references/samples-index.json`. If your file has syntax errors or is missing required fields, the build will fail with an error message pointing to the problematic file.

You can then test your sample is searchable:

```bash
go run . sample-search --query "your intent keywords"
```

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
