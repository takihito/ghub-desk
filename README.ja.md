# ghub-desk

GitHub Organization Management CLI & MCP Server

## 概要

`ghub-desk`は、GitHub 組織のメンバー・チーム・リポジトリ情報を整理するためのコマンドラインツールです。GitHub API と連携して組織情報を取得し、SQLite にキャッシュしてオフライン参照を支援します。また、Model Context Protocol (MCP) サーバーとして起動することで、LLM やエージェントから同じ機能を安全に呼び出すことができます。

- GitHub API を利用した `pull` / `view` / `push` コマンド
- 変更系コマンドは DRYRUN を標準とし、`--exec` 指定時のみ実行
- 設定ファイルと環境変数での柔軟な構成 (DB パス、MCP 権限など)
- MCP ツール経由での自動化と統合

## 主な機能

### データ取得 (pull)
- ターゲット: `users`, `detail-users`, `teams`, `repos`, `team-user`, `all-teams-users`, `outside-users`, `token-permission`
- `--no-store` でローカル DB への保存をスキップ、`--stdout` で API レスポンスを標準出力に表示
- `--interval-time` で GitHub API 呼び出し間隔を調整

### データ表示 (view)
- `pull` で保存した情報を SQLite から表示
- `--team-user` や `{team-slug}/users` 引数で特定チームのユーザーを参照
- `--settings` でマスク済み設定値を確認

### データ操作 (push add/remove)
- 組織・チームからのユーザー追加/削除、チーム削除に対応
- デフォルトは DRYRUN (`--exec` 指定時のみ GitHub API を実行)
- `--no-store` で成功後のローカル DB 同期を抑止可能

### データ初期化 (init)
- SQLite テーブルを初期化して保存領域を準備

### バージョン確認 (version)
- ビルド時に埋め込まれたバージョン/コミット/ビルド日時を表示

### MCP サーバー (mcp)
- MCP クライアント (例: MCP Inspector) から CLI 機能を呼び出すためのサーバーを起動
- 設定ファイルの `mcp.allow_pull` / `mcp.allow_write` で利用可能なツールを制御

## 設定

### 環境変数

```bash
export GHUB_DESK_ORGANIZATION="your-org-name"      # GitHub 組織名
export GHUB_DESK_GITHUB_TOKEN="your-token"         # GitHub Access Token
```

### GitHub App での認証

Personal Access Token の代わりに GitHub App の資格情報でも認証できます。App 認証を利用する場合は、下記の環境変数を設定するか、設定ファイルの `github_app` セクションに値を記載してください（PAT と App を同時に指定することはできません）。

```bash
export GHUB_DESK_APP_ID="123456"                 # GitHub App の App ID
export GHUB_DESK_INSTALLATION_ID="7890123"      # インストール先の Installation ID
export GHUB_DESK_PRIVATE_KEY="$(cat /path/to/private-key.pem)" # PEM 文字列全体
```

`GHUB_DESK_PRIVATE_KEY` には秘密鍵（`-----BEGIN...END-----` を含む）を直接文字列として設定するか、設定ファイルで複数行文字列として読み込ませてください。

### 設定ファイル例 (~/.config/ghub-desk/config.yaml)

```yaml
organization: "your-org"
github_token: "${GHUB_DESK_GITHUB_TOKEN}"
database_path: "./ghub-desk.db"          # 任意。指定しない場合はカレントディレクトリを使用

mcp:
  allow_pull: true                       # pull 系ツールを公開
  allow_write: false                     # push add/remove は無効
```

### 入力制約（ユーザー名・チーム）

- ユーザー名
  - 許可: 英数字・ハイフンのみ
  - 先頭/末尾にハイフンは不可
  - 長さ: 1〜39 文字
- チーム名（slug）
  - API 指定は slug を使用します（表示名ではありません）
  - 許可: 小文字英数字・ハイフンのみ
  - 先頭/末尾にハイフンは不可
  - 長さ: 1〜100 文字
- チーム名(slug)/ユーザー名
  - 形式: `{team-slug}/{username}` を `--team-user` に渡してください

## 基本的な使用例

### pull

```bash
# 組織メンバーの基本情報を取得・保存
./ghub-desk pull --users

# 取得結果を標準出力へ出しつつ詳細情報も保存
./ghub-desk pull --detail-users --stdout

# チーム一覧を取得。DB を更新せず API 結果のみ確認
./ghub-desk pull --teams --no-store

# 全チームのメンバーを連続取得（リクエスト間隔は既定 3s）
./ghub-desk pull --all-teams-users
```

### view

```bash
# 保存済みのユーザー情報を表示
./ghub-desk view --users

# チーム slug を指定してメンバーを表示
./ghub-desk view --team-user team-slug

# マスク済みの設定値を確認
./ghub-desk view --settings
```

### push

```bash
# チームにユーザーを追加（DRYRUN）
./ghub-desk push add --team-user team-slug/username

# 追加を実行し、成功時にローカル DB も同期
./ghub-desk push add --team-user team-slug/username --exec

# チームからユーザーを削除し、ローカル DB 更新は抑止
./ghub-desk push remove --team-user team-slug/username --exec --no-store
```

### init / version

```bash
# SQLite テーブルを初期化
./ghub-desk init

# バージョン情報を表示
./ghub-desk version
```

## MCP サーバー

```bash
# MCP サーバーを起動 (許可されたツールのみ公開)
./ghub-desk mcp --debug

```

- MCP サーバーは設定の `mcp.allow_pull` / `allow_write` に応じて `pull.*` / `push.*` ツールを公開します。
- `allow_write` を有効にする場合は、`--exec` フラグの利用や DRYRUN で影響範囲を確認してから実行してください。

## 技術

- **REST API**: GitHub API を利用したデータ取得・操作
- **ローカルデータベース**: SQLite を使用したオフラインでのデータ参照
- **MCP**: Model Context Protocol を介した外部クライアント連携 (`github.com/modelcontextprotocol/go-sdk`)

## ビルド

```bash
make build

make build_mcp
```

## テスト

```bash
make test
```

## 対応プラットフォーム

- Go 1.24+
- macOS, Linux, Windows
