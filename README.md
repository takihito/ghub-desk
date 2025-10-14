# ghub-desk

GitHub Organization Management CLI & MCP Server

[Read this document in Japanese](README.ja.md)

## Overview

`ghub-desk` is a command-line tool for managing members, teams, and repositories in a GitHub organization. It communicates with the GitHub API to fetch organization data, caches the responses in SQLite for offline access, and can run as a Model Context Protocol (MCP) server so that LLMs and agents can safely reuse the same capabilities.

- GitHub API powered `pull`, `view`, and `push` commands
- Mutating commands run in DRYRUN mode by default and require `--exec` to perform changes
- Flexible configuration via config files and environment variables (database path, MCP permissions, etc.)
- Easy automation through MCP tool integrations

## Core Commands

### Data collection (pull)
- Targets: `users`, `detail-users`, `teams`, `repos`, `repos-users`, `team-user`, `all-teams-users`, `outside-users`, `token-permission`
- Use `--no-store` to skip writing to the local DB, `--stdout` to stream API responses to stdout
- Use `--interval-time` to throttle GitHub API calls

### Data inspection (view)
- Display the data stored by `pull` from SQLite
- Use `--team-user` or `team-slug/users` arguments to inspect specific teams
- Use `--repos-users` to review direct collaborators added to a repository
- Use `--user-repos <login>` to list repositories a user can access along with direct/team routes and permissions (requires `pull --repos-users`, `pull --repos-teams`, and `pull --team-users`)
- Use `--settings` to review masked configuration values

### Data mutations (push add/remove)
- Add or remove users from the organization and its teams, delete teams, or manage outside collaborators on repositories (optional `--permission` to set `pull`, `push`, or `admin`; aliases: `read`→`pull`, `write`→`push`)
- Runs in DRYRUN mode by default; apply changes with `--exec`
- Use `--no-store` to skip syncing the local DB after successful operations

### Database initialization (init)
- Prepare SQLite tables for local storage

### Version information (version)
- Display build-time metadata (version, commit, build time)

### MCP server (mcp)
- Launch an MCP server so clients (for example, MCP Inspector) can call the CLI features
- Control the exposed tools with `mcp.allow_pull` and `mcp.allow_write`

## Configuration

### Environment variables

```bash
export GHUB_DESK_ORGANIZATION="your-org-name"      # GitHub organization name
export GHUB_DESK_GITHUB_TOKEN="your-token"         # GitHub Personal Access Token (PAT)
```

### Authenticating with a GitHub App

You can authenticate with GitHub App credentials instead of a Personal Access Token. Provide the following environment variables or set the `github_app` section in the config file. Do not configure a PAT and a GitHub App at the same time.

```bash
export GHUB_DESK_APP_ID="123456"                 # GitHub App ID
export GHUB_DESK_INSTALLATION_ID="76543210"      # Installation ID for the target org
export GHUB_DESK_PRIVATE_KEY="$(cat /path/to/private-key.pem)" # Full PEM string
```

`GHUB_DESK_PRIVATE_KEY` must contain the entire private key text (including `-----BEGIN ...` and `-----END ...`). Set it directly via the environment or load it as a multi-line string in the config file.

### Example config file (~/.config/ghub-desk/config.yaml)

```yaml
organization: "your-org"
github_token: "${GHUB_DESK_GITHUB_TOKEN}"
database_path: "./ghub-desk.db"          # Optional. Defaults to the current directory.

mcp:
  allow_pull: true                       # expose pull/view tools
  allow_write: false                     # keep push add/remove disabled by default
```

### Input constraints (usernames and teams)

- Usernames
  - Allowed characters: alphanumeric and hyphen
  - Hyphen cannot appear at the beginning or end
  - Length: 1 to 39 characters
- Team slugs
  - Use the slug in API calls (not the display name)
  - Allowed characters: lowercase alphanumeric and hyphen
  - Hyphen cannot appear at the beginning or end
  - Length: 1 to 100 characters
- Team slug with username
  - Use the `{team-slug}/{username}` format when passing to `--team-user`
- Repository names
  - Allowed characters: alphanumeric, underscore (`_`), and hyphen (dot is not allowed)
  - Cannot start with a hyphen
  - Length: 1 to 100 characters
- Repository with username
  - Use the `{repository}/{username}` format when passing to `--outside-user`

## Usage

### pull

```bash
# Fetch and store basic organization member information
./ghub-desk pull --users

# Store detailed user information while streaming the API response
./ghub-desk pull --detail-users --stdout

# Retrieve the team list without updating the DB
./ghub-desk pull --teams --no-store

# Fetch direct collaborators for a repository
./ghub-desk pull --repos-users repo-name

# Fetch members for every team (default interval: 3s)
./ghub-desk pull --all-teams-users
```

### view

```bash
# Show stored user information
./ghub-desk view --users

# Inspect members of a specific team slug
./ghub-desk view --team-user team-slug

# Inspect direct collaborators for a repository
./ghub-desk view --repos-users repo-name

# List repositories a user can access (run pull --repos-users, --repos-teams, and --team-users beforehand)
./ghub-desk view --user-repos user-login

# Review masked configuration values
./ghub-desk view --settings
```

### push

```bash
# Add a user to a team (DRYRUN)
./ghub-desk push add --team-user team-slug/username

# Execute the addition and sync the local DB on success
./ghub-desk push add --team-user team-slug/username --exec

# Remove a user from a team while skipping the DB sync
./ghub-desk push remove --team-user team-slug/username --exec --no-store

# Invite an outside collaborator to a repository (DRYRUN)
./ghub-desk push add --outside-user repo-name/username

# Invite with explicit permission (e.g., read-only access)
./ghub-desk push add --outside-user repo-name/username --permission read

# Remove an outside collaborator from a repository and sync the DB
./ghub-desk push remove --outside-user repo-name/username --exec
```

### init / version

```bash
# Initialize SQLite tables
./ghub-desk init

# Display build metadata
./ghub-desk version
```

## MCP server

```bash
# Launch the MCP server (only exposes allowed tools)
./ghub-desk mcp --debug

# Build with the go-sdk support and launch
make build_mcp
./build/ghub-desk mcp
```

- The server exposes `pull.*` and `push.*` tools based on `mcp.allow_pull` and `mcp.allow_write`.
- When enabling write operations, run with `--exec` and review the DRYRUN output first.

## Technology

- **REST API**: GitHub API for data retrieval and mutations
- **Local database**: SQLite for offline access to cached data
- **MCP**: Model Context Protocol integration (`github.com/modelcontextprotocol/go-sdk`)

## Build

```bash
make build
```

## Test

```bash
make test
```

## Supported platforms

- Go 1.24+
- macOS, Linux, Windows
