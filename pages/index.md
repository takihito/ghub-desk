---
layout: default
title: ghub-desk - GitHub Organization Management CLI & MCP Server
description: ghub-desk is a Go CLI tool and MCP server for managing GitHub organization members, teams, and repositories.
---

# ghub-desk

[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/11852/badge)](https://www.bestpractices.dev/projects/11852)

ghub-desk is a Go CLI tool for managing members, teams, and repositories in a GitHub organization.
It fetches data from the GitHub API, caches it in SQLite for offline access,
and exposes the same capabilities as an MCP server for LLMs and AI agents.

[日本語ドキュメント](ja/)

## Features

- **pull / view / push** — Fetch org data from GitHub, inspect it locally, and safely mutate membership
- **DRYRUN by default** — Mutating commands require `--exec` to perform actual changes
- **SQLite cache** — Offline access to organization snapshots; resumable long-running pulls
- **MCP server** — All CLI capabilities exposed over stdio for AI agent integration
- **Dual authentication** — PAT or GitHub App (choose exactly one)
- **Single binary** — download and run; no runtime dependencies

## Quick Start

```bash
# Install (Linux / macOS)
curl -sSL https://takihito.github.io/ghub-desk/install.sh | sh

# Create a config skeleton
ghub-desk init config
# Edit ~/.config/ghub-desk/config.yaml with your org and token

# Initialize the database
ghub-desk init db

# Fetch organization members
ghub-desk pull --users

# View stored data
ghub-desk view --users
```

> **Windows:** `irm https://takihito.github.io/ghub-desk/install.ps1 | iex`

See [Installation](installation), [Usage](usage), and [MCP Server](mcp-server) for details.

## Safety Design

All mutating commands (`push add`, `push remove`) run in **DRYRUN mode by default**.
Add `--exec` to apply the change to GitHub. The local SQLite cache is synced automatically on success.

```bash
# Preview what would happen (DRYRUN)
ghub-desk push add --team-user my-team/octocat

# Apply the change
ghub-desk push add --team-user my-team/octocat --exec
```

## Technology

- **Language:** Go 1.26.1+
- **Database:** SQLite (offline cache)
- **MCP:** [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk)
- **Platforms:** macOS, Linux, Windows
