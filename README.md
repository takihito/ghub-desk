# ghub-desk

GitHub Organization Management CLI & MCP Server

[Read this document in Japanese](README.ja.md)

## Overview

`ghub-desk` is a command-line tool for managing members, teams, and repositories in a GitHub organization. It communicates with the GitHub API to fetch organization data, caches the responses in SQLite for offline access, and can run as a Model Context Protocol (MCP) server so that LLMs and agents can safely reuse the same capabilities.

- GitHub API powered `pull`, `view`, and `push` commands
- Mutating commands run in DRYRUN mode by default and require `--exec` to perform changes
- Flexible configuration via config files and environment variables (database path, MCP permissions, etc.)
- Easy automation through MCP tool integrations

## Installation

### `go install`
Use the Go toolchain (1.24 or later) to install the latest tagged release into your `$GOBIN` (defaults to `$GOPATH/bin`).

```bash
GO111MODULE=on go install github.com/takihito/ghub-desk@latest
```

Ensure `$GOBIN` is on your `$PATH`, then run `ghub-desk version` to confirm the installation.

### Download release archives (curl)

Pre-built binaries are published on the [releases page](https://github.com/takihito/ghub-desk/releases). Pick a version tag and export it before running the commands below:

```bash
# Set VERSION to a release tag before running these commands, for example:
# export VERSION=0.2.0
OS=${OS:-Darwin}            # Darwin, Linux, or Windows
ARCH=${ARCH:-arm64}         # arm64 or x86_64
ARTIFACT="ghub-desk_${VERSION}_${OS}_${ARCH}.tar.gz"

curl -L -o "${ARTIFACT}" \
  "https://github.com/takihito/ghub-desk/releases/download/v${VERSION}/${ARTIFACT}"
# Replace SHA256_FROM_RELEASE with the checksum published for your artifact
echo "SHA256_FROM_RELEASE  ${ARTIFACT}" | shasum -a 256 --check
sudo tar -xzf "${ARTIFACT}" -C /usr/local/bin ghub-desk

```

Look up the artifact name and checksum that match your platform on the releases page. On Windows, extract the archive and place `ghub-desk.exe` somewhere on your `%PATH%`. Updating to future versions only requires changing `VERSION`.

### Build from source

Clone the repository and run the standard build to produce `./build/ghub-desk`. This method is useful when you need unreleased changes or want to audit the source.

```bash
git clone https://github.com/takihito/ghub-desk.git
cd ghub-desk
make deps
make build
sudo cp build/ghub-desk /usr/local/bin/
```

Run `make test` before installing if you are modifying the codebase locally.

## Core Commands

### Data collection (pull)
- Targets: `users`, `detail-users`, `teams`, `repos`, `repos-users`, `all-repos-users`, `repos-teams`, `all-repos-teams`, `team-user`, `all-teams-users`, `outside-users`, `token-permission`
- Use `--no-store` to skip writing to the local DB, `--stdout` to stream API responses to stdout
- Use `--interval-time` to throttle GitHub API calls

### Data inspection (view)
- Display the data stored by `pull` from SQLite
- Use `--team-user` or `team-slug/users` arguments to inspect specific teams
- Use `--repos-users` to review direct collaborators added to a repository
- Use `--all-repos-users` to review collaborators across every repository stored in SQLite
- Use `--user-repos <login>` to list repositories a user can access along with direct/team routes and permissions (requires `pull --repos-users`, `pull --repos-teams`, and `pull --team-users`)
- Use `--settings` to review masked configuration values

### Data mutations (push add/remove)
- Add or remove users from the organization and its teams, delete teams, manage outside collaborators on repositories, or remove direct repository collaborators with `--repos-user` (optional `--permission` to set `pull`, `push`, or `admin`; aliases: `read`→`pull`, `write`→`push`)
- Runs in DRYRUN mode by default; apply changes with `--exec`
- Use `--no-store` to skip syncing the local DB after successful operations

### Database initialization (init)
- Prepare SQLite tables for local storage

### Version information (version)
- Display build-time metadata (version, commit, build time)

### MCP server (mcp)
- `./ghub-desk mcp --debug` launches the go-sdk MCP server over stdio
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

# Fetch direct collaborators for every repository (respects resume sessions)
./ghub-desk pull --all-repos-users

# Fetch teams for every repository stored in GitHub
./ghub-desk pull --all-repos-teams

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

# Inspect direct collaborators across every repository in the database
./ghub-desk view --all-repos-users

# Inspect repository teams across every repository in the database
./ghub-desk view --all-repos-teams

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

# Remove a user from a team
./ghub-desk push remove --team-user team-slug/username --exec

# Remove a user from the organization
./ghub-desk push remove --user username --exec

# Remove a team from the organization
./ghub-desk push remove --team team-slug --exec

# Invite an outside collaborator to a repository (DRYRUN)
./ghub-desk push add --outside-user repo-name/username

# Invite with explicit permission (e.g., read-only access)
./ghub-desk push add --outside-user repo-name/username --permission read

# Remove an outside collaborator from a repository and sync the DB
./ghub-desk push remove --outside-user repo-name/username --exec

# Remove a direct repository collaborator (outside collaborator or org member)
./ghub-desk push remove --repos-user repo-name/username --exec

# DRYRUN removal of a repository collaborator
./ghub-desk push remove --repos-user repo-name/username
```

### init

`init` exposes subcommands for configuring the application and preparing the database.

```bash
# Create a config skeleton at the default location (~/.config/ghub-desk/config.yaml)
./ghub-desk init config

# Create a config file at a custom path (missing directories are created automatically)
./ghub-desk init config --target-file ~/ghub/config.yaml

# Initialize the database using the path from the config (defaults to ./ghub-desk.db)
./ghub-desk init db

# Initialize the database at an explicit path (warns and skips when the file already exists)
./ghub-desk init db --target-file ~/data/ghub-desk.db
```

### version

```bash
# Display build metadata
./ghub-desk version
```

## MCP server

```bash
# Build and launch the MCP server
make build
./build/ghub-desk mcp --debug
```

- The server exposes `pull_*` and `push_*` tools based on `mcp.allow_pull` and `mcp.allow_write`.
- When enabling write operations, run with `--exec` and review the DRYRUN output first.

### MCP tools

- `health` — readiness probe with no inputs.

#### Read-only (`view_*`)
- `view_users`, `view_detail-users`, `view_teams`, `view_repos`, `view_outside-users`, `view_token-permission` — return cached records without inputs.
- `view_team-user` (input: `team`) — members for a specific team slug.
- `view_repos-users` / `view_repos-teams` (input: `repository`) — direct collaborators or team permissions for one repository.
- `view_all-teams-users`, `view_all-repos-users`, `view_all-repos-teams` — organization-wide membership snapshots.
- `view_user-repos` (input: `user`) — repositories a user can access with direct/team paths.
- `view_settings` — configuration values with secrets masked.

#### Data refresh (`pull_*`)
- Common optional inputs: `no_store` (bool), `stdout` (bool), `interval_seconds` (number; defaults to 3 seconds).
- `pull_users`, `pull_detail-users`, `pull_teams`, `pull_repositories`, `pull_all-teams-users`, `pull_all-repos-users`, `pull_all-repos-teams`, `pull_outside-users`, `pull_token-permission` — operate on cached scopes.
- `pull_team-user` (inputs: common + `team`) — refresh one team membership list.
- `pull_repos-users` / `pull_repos-teams` (inputs: common + `repository`) — refresh collaborators or team permissions for one repository.

#### Write operations (`push_*`)
- `push_add` — accepts either `team_user` *or* `outside_user`; optional `permission` (`pull`/`push`/`admin`, aliases `read`→`pull`, `write`→`push`), plus `exec`/`no_store`.
- `push_remove` — accepts a single target (`team`, `user`, `team_user`, `outside_user`, or `repos_user`) with optional `exec`/`no_store`.

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
