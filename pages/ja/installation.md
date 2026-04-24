---
layout: default
title: ghub-desk インストール
description: ghub-desk のインストール手順。Linux / macOS / Windows 対応。
---

# インストール

## クイックインストール（推奨）

**Linux / macOS:**

```bash
curl -sSL https://takihito.github.io/ghub-desk/install.sh | sh
```

**Windows (PowerShell):**

```powershell
irm https://takihito.github.io/ghub-desk/install.ps1 | iex
```

最新バージョンを自動検出し、チェックサムを検証して `~/.local/bin` にインストールします。`sudo` 不要。

`~/.local/bin` が PATH に含まれていない場合は、シェルのプロファイル（`~/.bashrc`, `~/.zshrc` など）に以下を追加してください:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

インストール先を変更する場合:

```bash
curl -sSL https://takihito.github.io/ghub-desk/install.sh | GHUB_DESK_INSTALL_DIR=/usr/local/bin sh
```

## go install

```bash
GO111MODULE=on go install github.com/takihito/ghub-desk@latest
```

`$GOBIN` を `PATH` に通し、`ghub-desk version` でインストールを確認してください。

## 事前ビルドバイナリ

手動でダウンロードする場合は [Releases](https://github.com/takihito/ghub-desk/releases) ページをご覧ください。

```bash
# 例: macOS arm64, バージョン 0.2.3
VERSION=0.2.3
OS=Darwin      # Darwin, Linux, または Windows
ARCH=arm64     # arm64 または x86_64
ARTIFACT="ghub-desk_${VERSION}_${OS}_${ARCH}.tar.gz"

curl -L -o "${ARTIFACT}" \
  "https://github.com/takihito/ghub-desk/releases/download/v${VERSION}/${ARTIFACT}"

# チェックサム検証（リリースページの checksums.txt から値を確認）
echo "EXPECTED_SHA256  ${ARTIFACT}" | shasum -a 256 --check

tar -xzf "${ARTIFACT}" ghub-desk
sudo mv ghub-desk /usr/local/bin/
```

## ソースからビルド

```bash
git clone https://github.com/takihito/ghub-desk.git
cd ghub-desk
make deps
make build
sudo cp build/ghub-desk /usr/local/bin/
```

ローカルで改修する場合は、インストール前に `make test` を実行してください。

## 設定

インストール後、設定ファイルのひな形を作成します:

```bash
ghub-desk init config
```

`~/.config/ghub-desk/config.yaml` を編集してください:

```yaml
organization: "your-org"
github_token: "${GHUB_DESK_GITHUB_TOKEN}"
database_path: "./ghub-desk.db"   # 省略可

mcp:
  allow_pull: true    # pull/view 系ツールを MCP クライアントに公開
  allow_write: false  # push 系はデフォルト無効
```

その後、データベースを初期化します:

```bash
ghub-desk init db
```

### 認証方式

**いずれか一方**を選択してください:

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

PAT と GitHub App を同時に設定するとサーバー起動時にエラーになります。
