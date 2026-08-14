# MCP Server Feature Guide

`ghub-desk` exposes its CLI pull/view/push capabilities to agents through the Model Context Protocol (MCP). This guide summarizes the existing MCP server implementation, the tools it provides, and how to use them.

## Implementation Overview
- Entry point: `ghub-desk mcp`
- Configuration: control published tools with `config.Config.MCP` fields `allow_pull` and `allow_write`
- Persistence: identical SQLite database as the CLI (`ghub-desk.db` by default, configurable via `database_path`)
- Authentication: supply either a Personal Access Token or a GitHub App configuration (choose exactly one)

## Build Modes
| Mode | Command | Description |
| --- | --- | --- |
| Default | `go build ./...`<br>`ghub-desk mcp --debug` | Links `github.com/modelcontextprotocol/go-sdk` and serves the MCP server over stdio. |
| Compatibility target | `make build_mcp`<br>`./build/ghub-desk mcp --debug` | Backward-compatible target that produces the same artifact as `make build`. |

> The previous stub mode has been removed. The standard build now serves the go-sdk MCP server directly.

## Configuration Example
`~/.ghub-desk/config.yaml`

```yaml
organization: "your-org"
github_token: "${GHUB_DESK_GITHUB_TOKEN}" # or use the github_app section

database_path: "./ghub-desk.db"

mcp:
  allow_pull: true   # register pull_* tools
  allow_write: false # disable push_* tools for safety
```

`allow_pull` and `allow_write` directly govern which tools are registered by the MCP server. When omitted (`nil`), they default to false.

## Available Tools
### Common
| Tool | Description | Input | Notes |
| --- | --- | --- | --- |
| `health` | Server health check | none | Returns `{"status":"ok","time":"RFC3339"}` |

### view_* (always available)
All view tools read from the local database.

| Tool | Description | Input | Output Overview |
| --- | --- | --- | --- |
| `view_users` | List organization members | none | `users[]` with `id`, `login`, `name`, `email`, ... |
| `view_detail-users` | Detailed member view (currently same as `view_users`) | none | `users[]` |
| `view_user` | Single user profile | `{ "user": "login" }` | `found`, `user` (profile incl. `created_at`/`updated_at`) |
| `view_user-teams` | Teams a user belongs to | `{ "user": "login" }` | `teams[]` with `team_slug`, `team_name`, `role` |
| `view_teams` | List organization teams | none | `teams[]` with `id`, `slug`, `name`, `description`, `privacy` |
| `view_repos` | List repositories | none | `repositories[]` with `name`, `full_name`, `private`, `language`, `stars` |
| `view_team-user` | Members of a specific team | `{ "team": "team-slug" }` | `users[]` with `user_id`, `login`, `role` |
| `view_repos-users` | Direct collaborators of a repository | `{ "repository": "repo-name" }` | `users[]` with `user_id`, `login`, `permission` |
| `view_repos-teams` | Teams with access to a repository | `{ "repository": "repo-name" }` | `teams[]` with `team_slug`, `team_name`, `permission`, `privacy` |
| `view_repos-teams-users` | Members of teams linked to a repository | `{ "repository": "repo-name" }` | `members[]` with `team_slug`, `team_permission`, `user_login`, `role` |
| `view_team-repos` | Repositories a team can access | `{ "team": "team-slug" }` | `repositories[]` with `repo_name`, `full_name`, `permission` |
| `view_user-repos` | Repositories a user can access, and how | `{ "user": "login" }` | `repositories[]` with `repository`, `access_from[]`, `permission` |
| `view_all-teams-users` | Every team membership entry | none | `entries[]` with `team_slug`, `team_name`, `user_login`, `role` |
| `view_all-repos-users` | Every repository collaborator | none | `entries[]` with `repo_name`, `full_name`, `user_login`, `permission` |
| `view_all-repos-teams` | Every repository/team access grant | none | `entries[]` with `repo_name`, `full_name`, `team_slug`, `permission` |
| `view_outside-users` | Outside collaborators | none | `users[]` |
| `view_settings` | Application configuration with secrets masked | none | Masked config, useful for confirming `allow_pull`/`allow_write` |
| `view_token-permission` | Cached response from `pull_token-permission` | none | Permission data for PAT or GitHub App; errors when missing |
| `view_org-plan` | Cached organization plan from `pull_org-plan` | none | Plan name, contracted seats, filled seats, plus `cached_users`/`cached_outside_users` reference counts from the local cache; errors when missing |

### auditlogs (always available)
| Tool | Description | Input | Notes |
| --- | --- | --- | --- |
| `auditlogs` | Fetch audit log entries by actor | `{ "user": "octocat", "created"?, "repo"?, "per_page"? }` | Calls GitHub API; defaults to last 30 days; per_page max is 100 |

### pull_* (requires `allow_pull: true`)
These tools call the GitHub API and update SQLite by default. Every pull_* tool accepts the same three common options: `no_store` (skip persistence), `stdout` (mirror API responses to stdout), and `interval_seconds` (delay between paginated API calls; defaults to 3s).

| Tool | Description | Additional Input | Notes |
| --- | --- | --- | --- |
| `pull_users` | Fetch organization members | none | |
| `pull_detail-users` | Fetch organization members with details | none | |
| `pull_teams` | Fetch organization teams | none | |
| `pull_repositories` | Fetch repositories | none | |
| `pull_team-user` | Fetch members of one team | `{ "team" }` | `team` must be a slug (`team-slug`) |
| `pull_repos-users` | Fetch direct collaborators of one repository | `{ "repository" }` | |
| `pull_repos-teams` | Fetch teams with access to one repository | `{ "repository" }` | |
| `pull_all-teams-users` | Fetch memberships for every team | none | Loops over teams already cached in SQLite (run `pull_teams` first); soft-fails per team on error |
| `pull_all-repos-users` | Fetch collaborators for every repository | none | Loops over repositories already cached in SQLite (run `pull_repositories` first); fails fast on error |
| `pull_all-repos-teams` | Fetch team access for every repository | none | Loops over repositories already cached in SQLite (run `pull_repositories` first); fails fast on error |
| `pull_outside-users` | Fetch outside collaborators | none | |
| `pull_token-permission` | Fetch token permission headers | none | Persists the latest response in the database |
| `pull_org-plan` | Fetch organization plan (seats and contract info) | none | Requires a token with organization member/admin access (`read:org`); errors when plan info is unavailable |

### push_* (requires `allow_write: true`)
These operations modify GitHub state. They run in DRYRUN mode by default, returning the intended action. Set `exec: true` to perform the API call. Use `no_store: true` to skip syncing local state after successful execution (`SyncPushAdd/Remove`).

| Tool | Description | Input | Notes |
| --- | --- | --- | --- |
| `push_add` | Add members to teams or invite outside collaborators | `{ "team_user"? \| "outside_user"?, "permission"?, "exec"?, "no_store"? }` | Provide exactly one of `team_user` (`team-slug/username`) or `outside_user` (`repository/username`). `permission` (outside collaborators only): `pull`\|`push`\|`admin` (aliases `read`→`pull`, `write`→`push`). Returns `message` on success |
| `push_remove` | Remove teams, members, or collaborators | `{ "team"? \| "user"? \| "team_user"? \| "outside_user"? \| "repos_user"?, "exec"?, "no_store"? }` | Provide exactly one target. `repos_user` removes a direct repository collaborator (`repository/username`). `exec:false` returns DRYRUN output |

## Launch Example
```bash
make build
./build/ghub-desk mcp --debug --log-path /tmp/ghub-desk.log  # redirect stderr + debug logs to a file
```

`make build_mcp` produces the same binary (`./build/ghub-desk`). Run an MCP client (for example, MCP Inspector) as a subprocess to call `health`, `view_*`, `pull_*`, and `push_*`. Validate DRYRUN output before enabling `allow_write`.

## Integrating with AI Agents
MCP-compatible agents such as Gemini or Codex can invoke `ghub-desk` by registering its MCP server command in their configuration. Adjust paths to match your environment and reuse the CLI configuration file for consistency.

### Using resources/list
- Call `resources/list` followed by `resources/read` to fetch the built-in usage guides before invoking risky tools.
- Each tool description now ends with a `resource://ghub-desk/...` link. Resolve that URI via `resources/read` (anchors such as `#view_team-user`) to retrieve JSON examples and guardrails.

| Resource URI | Purpose |
| --- | --- |
| `resource://ghub-desk/mcp-overview` | Startup checklist, configuration knobs, and SQLite handling notes. |
| `resource://ghub-desk/mcp-tools` | Detailed reference for every tool including sample JSON payloads and response hints. |
| `resource://ghub-desk/mcp-safety` | Guidance for DRYRUN vs exec, allow_write usage, and local store synchronization. |

Recommended flow: inspect `tools/list`, look up the linked URI via `resources/read`, then run `tools/call` with the confirmed payload.

### Resource URI Handling
It is important to note that `resource://ghub-desk/...` URIs are **not** file paths and do not read from the `docs/` directory. The content for these resources is embedded directly as markdown strings within the Go source code at `mcp/docs.go`. The server handles these URIs by returning the corresponding embedded string.

The markdown files in the `docs/` directory are intended for human developers to understand the project structure and are separate from the content served to the agent via MCP.

### Gemini
Add the `ghub-desk` MCP server to the `mcpServers` section of `~/.gemini/settings.json`.

```json
{
  "mcpServers": {
    "ghub-desk": {
      "command": "/home/takihito/bin/ghub-desk",
      "args": [
        "mcp",
        "--debug",
        "--config",
        "/home/takihito/.ghub-desk/config.yaml"
      ],
      "transport": "stdio",
      "retry": {
        "maxRestarts": 5,
        "windowSeconds": 60
      }
    }
  }
}
```

### Codex
Add the MCP server entry to the `[mcp_servers]` section of `~/.codex/config.toml`.

```toml
[mcp_servers.ghub-desk]
command = "/home/takihito/bin/ghub-desk"
args = [
  "mcp",
  "--debug",
  "--config",
  "/home/takihito/.ghub-desk/config.yaml"
]
```

Restart the Codex CLI after editing the configuration so the new MCP server is detected. Review `allow_pull` and `allow_write` in the shared configuration file beforehand to ensure the agent only exposes the operations you intend to permit.

## Error Handling and Caveats
- The server fails to start if authentication is missing or if both a PAT and GitHub App credentials are supplied.
- `view_token-permission` returns an error when no cached data exists (run `pull_token-permission` with `no_store:false`).
- Override the database location via `config.yaml` or `GHUB_DESK_DB_PATH`; the MCP server uses the same path.
- MCP pull/push operations follow the same rate limits and permissions as the CLI. For GitHub App auth, provide `GHUB_DESK_APP_ID`, `GHUB_DESK_INSTALLATION_ID`, and `GHUB_DESK_PRIVATE_KEY`.

## Future Ideas
- Refine response schemas (JSON Schema validation)
- Add event/streaming support
- Provide diff tooling for successive pull results
