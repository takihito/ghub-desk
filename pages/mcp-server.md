---
layout: default
title: ghub-desk MCP Server
description: MCP server documentation for ghub-desk — tools, configuration, and AI agent integration.
---

# MCP Server

`ghub-desk` exposes its CLI pull/view/push capabilities to AI agents through the
[Model Context Protocol](https://modelcontextprotocol.io/) (MCP).

## Quick Start

```bash
make build
./build/ghub-desk mcp --debug
```

## Configuration

`~/.config/ghub-desk/config.yaml`

```yaml
organization: "your-org"
github_token: "${GHUB_DESK_GITHUB_TOKEN}"

database_path: "./ghub-desk.db"

mcp:
  allow_pull: true    # register pull_* tools
  allow_write: false  # disable push_* tools for safety
```

`allow_pull` and `allow_write` directly govern which tools are registered.
When omitted, both default to `false`.

## Available Tools

### Common

| Tool | Description | Input |
|---|---|---|
| `health` | Server readiness probe | none |

### view_* (always available)

Read from the local SQLite database. Results reflect the data currently cached in SQLite; the current MCP implementation does not enforce a fixed per-tool record limit.

| Tool | Description | Input |
|---|---|---|
| `view_users` | List organization members | none |
| `view_detail-users` | Detailed member view | none |
| `view_user` | Show a single user profile | `{ "user": "github-login" }` |
| `view_user-teams` | Teams a user belongs to | `{ "user": "github-login" }` |
| `view_teams` | List organization teams | none |
| `view_repos` | List repositories | none |
| `view_team-user` | Members of a specific team | `{ "team": "team-slug" }` |
| `view_team-repos` | Repositories a team can access | `{ "team": "team-slug" }` |
| `view_repos-users` | Direct collaborators for a repository | `{ "repository": "repo-name" }` |
| `view_repos-teams` | Team permissions for a repository | `{ "repository": "repo-name" }` |
| `view_repos-teams-users` | Members of teams linked to a repository | `{ "repository": "repo-name" }` |
| `view_all-teams-users` | All team memberships | none |
| `view_all-repos-users` | All direct collaborators | none |
| `view_all-repos-teams` | All repository-team links | none |
| `view_user-repos` | Repositories a user can access | `{ "user": "github-login" }` |
| `view_outside-users` | Outside collaborators | none |
| `view_token-permission` | Cached token permission data | none |
| `view_settings` | Masked configuration values | none |

### auditlogs (always available)

| Tool | Description | Input |
|---|---|---|
| `auditlogs` | Fetch audit log entries by actor | `{ "user": "octocat", "created"?, "repo"?, "per_page"? }` |

### pull_* (requires `allow_pull: true`)

Call the GitHub API and update SQLite. Common optional inputs: `no_store` (bool), `stdout` (bool), `interval_seconds` (number).

| Tool | Description | Required input |
|---|---|---|
| `pull_users` | Fetch organization members | none |
| `pull_detail-users` | Fetch detailed member info | none |
| `pull_teams` | Fetch teams | none |
| `pull_repositories` | Fetch repositories | none |
| `pull_team-user` | Fetch members of one team | `team` |
| `pull_repos-users` | Fetch collaborators for a repository | `repository` |
| `pull_repos-teams` | Fetch team permissions for a repository | `repository` |
| `pull_all-teams-users` | Fetch all team memberships | none |
| `pull_all-repos-users` | Fetch all direct collaborators | none |
| `pull_all-repos-teams` | Fetch all repository-team links | none |
| `pull_outside-users` | Fetch outside collaborators | none |
| `pull_token-permission` | Fetch token permission info | none |

### push_* (requires `allow_write: true`)

Modify GitHub state. **DRYRUN by default** — add `exec: true` to apply.

| Tool | Description | Input |
|---|---|---|
| `push_add` | Add user to team or invite outside collaborator | `team_user` or `outside_user`; optional `permission`, `exec`, `no_store` |
| `push_remove` | Remove team, user, or collaborator | one of: `team`, `user`, `team_user`, `outside_user`, `repos_user`; optional `exec`, `no_store` |

## Built-in Documentation Resources

Call `resources/list` then `resources/read` to retrieve usage guides before invoking tools.

| Resource URI | Content |
|---|---|
| `resource://ghub-desk/mcp-overview` | Startup checklist, configuration, SQLite notes |
| `resource://ghub-desk/mcp-tools` | Detailed tool reference with sample JSON payloads |
| `resource://ghub-desk/mcp-safety` | DRYRUN vs exec, allow_write guidance |

## Integrating with AI Agents

### Claude Code (MCP config)

```json
{
  "mcpServers": {
    "ghub-desk": {
      "command": "/path/to/ghub-desk",
      "args": ["mcp", "--debug", "--config", "~/.config/ghub-desk/config.yaml"],
      "transport": "stdio"
    }
  }
}
```

### Gemini

`~/.gemini/settings.json`:

```json
{
  "mcpServers": {
    "ghub-desk": {
      "command": "/path/to/ghub-desk",
      "args": ["mcp", "--debug", "--config", "/path/to/config.yaml"],
      "transport": "stdio",
      "retry": { "maxRestarts": 5, "windowSeconds": 60 }
    }
  }
}
```

### Codex

`~/.codex/config.toml`:

```toml
[mcp_servers.ghub-desk]
command = "/path/to/ghub-desk"
args = ["mcp", "--debug", "--config", "/path/to/config.yaml"]
```

## Notes

- The server fails to start if both a PAT and GitHub App credentials are configured simultaneously.
- `view_token-permission` returns an error when no cached data exists — run `pull_token-permission` first.
- `resource://ghub-desk/...` URIs are served from embedded Go source (`mcp/docs.go`), not from the `docs/` directory.
