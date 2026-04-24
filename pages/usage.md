---
layout: default
title: ghub-desk Usage
description: Command reference for ghub-desk — pull, view, push, auditlogs, init, version, and mcp.
---

# Usage

## pull — Fetch data from GitHub

Fetch organization data from the GitHub API and store it in the local SQLite database.

**Targets:** `users`, `detail-users`, `teams`, `repos`, `repos-users`, `all-repos-users`, `repos-teams`, `all-repos-teams`, `team-user`, `all-teams-users`, `outside-users`, `token-permission`

```bash
# Fetch and store organization members
ghub-desk pull --users

# Fetch detailed user profiles and stream to stdout
ghub-desk pull --detail-users --stdout

# Fetch teams without updating the database
ghub-desk pull --teams --no-store

# Fetch direct collaborators for a specific repository
ghub-desk pull --repos-users repo-name

# Fetch collaborators for every repository (resumable)
ghub-desk pull --all-repos-users

# Fetch teams for every repository
ghub-desk pull --all-repos-teams

# Fetch members of every team (default interval: 3s)
ghub-desk pull --all-teams-users
```

Use `--interval-time` to throttle API calls and `--no-store` / `--stdout` to control output.

## view — Inspect cached data

Display the data stored by `pull` from SQLite.

```bash
# List all organization members
ghub-desk view --users

# Show a single user's profile
ghub-desk view --user user-login

# List teams a user belongs to
ghub-desk view --user-teams user-login

# Show members of a specific team
ghub-desk view --team-user team-slug

# Show direct collaborators for a repository
ghub-desk view --repos-users repo-name

# Show members of teams linked to a repository
# (requires: pull --repos-teams && pull --all-teams-users)
ghub-desk view --repos-teams-users repo-name

# Show direct collaborators across all repositories
ghub-desk view --all-repos-users

# Show team permissions across all repositories
ghub-desk view --all-repos-teams

# List repositories a team can access
ghub-desk view --team-repos team-slug

# List repositories a user can access with permission details
# (requires: pull --all-repos-users or --repos-users <repo>,
#            pull --repos-teams <repo>,
#            and pull --all-teams-users or --team-user <team-slug>)
ghub-desk view --user-repos user-login

# Review masked configuration values
ghub-desk view --settings
```

Use `--format json` or `--format yaml` to change output format (default: `table`).

## push — Mutate organization data

Add or remove members, teams, and collaborators. **Runs in DRYRUN mode by default.**
Add `--exec` to apply the change to GitHub.

```bash
# Add a user to a team (DRYRUN)
ghub-desk push add --team-user team-slug/username

# Apply the addition
ghub-desk push add --team-user team-slug/username --exec

# Remove a user from a team
ghub-desk push remove --team-user team-slug/username --exec

# Remove a user from the organization
ghub-desk push remove --user username --exec

# Remove a team from the organization
ghub-desk push remove --team team-slug --exec

# Invite an outside collaborator to a repository (DRYRUN)
ghub-desk push add --outside-user repo-name/username

# Invite with a specific permission level
ghub-desk push add --outside-user repo-name/username --permission read

# Remove an outside collaborator and sync the database
ghub-desk push remove --outside-user repo-name/username --exec

# Remove a direct repository collaborator
ghub-desk push remove --repos-user repo-name/username --exec
```

Permission values: `pull` / `push` / `admin` (aliases: `read`→`pull`, `write`→`push`)

## auditlogs — Fetch audit logs

Retrieve organization audit log entries for a specific actor.

```bash
# Default: last 30 days
ghub-desk auditlogs --user user-login

# Filter by single date
ghub-desk auditlogs --user user-login --created 2025-01-01

# Filter by date range
ghub-desk auditlogs --user user-login --created "2025-01-01..2025-01-31"

# Filter by date (on or after)
ghub-desk auditlogs --user user-login --created ">=2025-01-01"

# Narrow to a specific repository
ghub-desk auditlogs --user user-login --repo repo-name

# Limit entries per page (max 100)
ghub-desk auditlogs --user user-login --per-page 50
```

## init — Initialize config and database

```bash
# Create a config skeleton at the default location
ghub-desk init config

# Create a config at a custom path
ghub-desk init config --target-file ~/ghub/config.yaml

# Initialize the database (uses path from config)
ghub-desk init db

# Initialize the database at an explicit path
ghub-desk init db --target-file ~/data/ghub-desk.db
```

## version — Display build info

```bash
ghub-desk version
```

## Input constraints

| Type | Allowed characters | Length |
|---|---|---|
| Username | Alphanumeric, hyphen (not at start/end) | 1–39 |
| Team slug | Lowercase alphanumeric, hyphen (not at start/end) | 1–100 |
| Repository | Alphanumeric, underscore, hyphen (not at start) | 1–100 |

- `--team-user` accepts `{team-slug}/{username}`
- `--outside-user` / `--repos-user` accepts `{repository}/{username}`

## Global flags

| Flag | Description |
|---|---|
| `--debug` | Verbose logging (SQL queries, API requests) |
| `--log-path <file>` | Append logs to a file |
| `--config <file>` | Use a custom config file path |
