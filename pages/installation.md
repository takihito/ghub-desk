---
layout: default
title: ghub-desk Installation
description: Installation instructions for ghub-desk, the GitHub Organization Management CLI & MCP Server.
---

# Installation

## Quick Install (recommended)

**Linux / macOS:**

```bash
curl -sSL https://takihito.github.io/ghub-desk/install.sh | sh
```

**Windows (PowerShell):**

```powershell
irm https://takihito.github.io/ghub-desk/install.ps1 | iex
```

Automatically detects the latest version, verifies checksums, and installs to `~/.local/bin`. No `sudo` required.

If `~/.local/bin` is not in your PATH, add the following to your shell profile (`~/.bashrc`, `~/.zshrc`, etc.):

```bash
export PATH="$HOME/.local/bin:$PATH"
```

To change the install directory:

```bash
curl -sSL https://takihito.github.io/ghub-desk/install.sh | GHUB_DESK_INSTALL_DIR=/usr/local/bin sh
```

## go install

```bash
GO111MODULE=on go install github.com/takihito/ghub-desk@latest
```

Ensure `$GOBIN` is on your `$PATH`, then run `ghub-desk version` to confirm.

## Pre-built binaries

For manual downloads, see the [Releases](https://github.com/takihito/ghub-desk/releases) page.

```bash
# Example: macOS arm64, version 0.2.3
VERSION=0.2.3
OS=Darwin      # Darwin, Linux, or Windows
ARCH=arm64     # arm64 or x86_64
ARTIFACT="ghub-desk_${VERSION}_${OS}_${ARCH}.tar.gz"

curl -L -o "${ARTIFACT}" \
  "https://github.com/takihito/ghub-desk/releases/download/v${VERSION}/${ARTIFACT}"

# Verify checksum (value from the releases page checksums.txt)
echo "EXPECTED_SHA256  ${ARTIFACT}" | shasum -a 256 --check

tar -xzf "${ARTIFACT}" ghub-desk
sudo mv ghub-desk /usr/local/bin/
```

## Build from source

```bash
git clone https://github.com/takihito/ghub-desk.git
cd ghub-desk
make deps
make build
sudo cp build/ghub-desk /usr/local/bin/
```

Run `make test` before installing if you are modifying the codebase locally.

## Configuration

After installation, create a config skeleton:

```bash
ghub-desk init config
```

Edit `~/.config/ghub-desk/config.yaml`:

```yaml
organization: "your-org"
github_token: "${GHUB_DESK_GITHUB_TOKEN}"
database_path: "./ghub-desk.db"   # optional

mcp:
  allow_pull: true    # expose pull/view tools to MCP clients
  allow_write: false  # keep push disabled by default
```

Then initialize the database:

```bash
ghub-desk init db
```

### Authentication

Choose **one** authentication method:

**Personal Access Token (PAT):**

```bash
export GHUB_DESK_ORGANIZATION="your-org"
export GHUB_DESK_GITHUB_TOKEN="ghp_..."
```

**GitHub App:**

```bash
export GHUB_DESK_APP_ID="123456"
export GHUB_DESK_INSTALLATION_ID="76543210"
export GHUB_DESK_PRIVATE_KEY="$(cat /path/to/private-key.pem)"
```

Do not configure both at the same time — the tool will refuse to start.
