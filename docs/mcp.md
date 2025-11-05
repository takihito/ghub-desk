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
`~/.config/ghub-desk/config.yaml`

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
All view tools read from the local database and return up to 200 records (team membership calls return up to 500 entries).

| Tool | Description | Input | Output Overview |
| --- | --- | --- | --- |
| `view_users` | List organization members | none | `users[]` with `id`, `login`, `name`, `email`, ... |
| `view_detail-users` | Detailed member view (currently same as `view_users`) | none | `users[]` |
| `view_teams` | List organization teams | none | `teams[]` with `id`, `slug`, `name`, `description`, `privacy` |
| `view_repos` | List repositories | none | `repositories[]` with `name`, `full_name`, `private`, `language`, `stars` |
| `view_team-user` | Members of a specific team | `{ "team": "team-slug" }` | `users[]` with `user_id`, `login`, `role` |
| `view_outside-users` | Outside collaborators | none | `users[]` |
| `view_token-permission` | Cached response from `pull_token-permission` | none | Permission data for PAT or GitHub App; errors when missing |

### pull_* (requires `allow_pull: true`)
These tools call the GitHub API and update SQLite by default. Set `no_store: true` to skip persistence, or `stdout: true` to mirror API responses to stdout.

| Tool | Description | Input | Notes |
| --- | --- | --- | --- |
| `pull_users` | Fetch organization members | `{ "no_store"?, "stdout"?, "detail"? }` | `detail:true` fetches `detail-users` |
| `pull_teams` | Fetch organization teams | `{ "no_store"?, "stdout"? }` |  |
| `pull_repositories` | Fetch repositories | `{ "no_store"?, "stdout"? }` |  |
| `pull_team-user` | Fetch members of one team | `{ "team", "no_store"?, "stdout"? }` | `team` must be a slug (`team-slug`) |
| `pull_outside-users` | Fetch outside collaborators | `{ "no_store"?, "stdout"? }` |  |
| `pull_token-permission` | Fetch token permission headers | `{ "no_store"?, "stdout"? }` | Persists the latest response in the database |

### push_* (requires `allow_write: true`)
These operations modify GitHub state. They run in DRYRUN mode by default, returning the intended action. Set `exec: true` to perform the API call. Use `no_store: true` to skip syncing local state after successful execution (`SyncPushAdd/Remove`).

| Tool | Description | Input | Notes |
| --- | --- | --- | --- |
| `push_add` | Add members to teams or invite outside collaborators | `{ "team_user", "exec"?, "no_store"? }` | `team_user` format: `team-slug/username`. Returns `message` on success |
| `push_remove` | Remove teams, members, or collaborators | `{ "team"? \| "user"? \| "team_user"?, "exec"?, "no_store"? }` | Provide exactly one target. `exec:false` returns DRYRUN output |

## Launch Example
```bash
make build
./build/ghub-desk mcp --debug
```

`make build_mcp` produces the same binary (`./build/ghub-desk`). Run an MCP client (for example, MCP Inspector) as a subprocess to call `health`, `view_*`, `pull_*`, and `push_*`. Validate DRYRUN output before enabling `allow_write`.

## Integrating with AI Agents
MCP-compatible agents such as Gemini or Codex can invoke `ghub-desk` by registering its MCP server command in their configuration. Adjust paths to match your environment and reuse the CLI configuration file for consistency.

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
        "/home/takihito/.config/ghub-desk/config.yaml"
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
  "/home/takihito/.config/ghub-desk/config.yaml"
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
