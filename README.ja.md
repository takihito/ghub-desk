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
- ターゲット: `users`, `detail-users`, `teams`, `repos`, `repos-users`, `all-repos-users`, `repos-teams`, `all-repos-teams`, `team-user`, `all-teams-users`, `outside-users`, `token-permission`
- `--no-store` でローカル DB への保存をスキップ、`--stdout` で API レスポンスを標準出力に表示
- `--interval-time` で GitHub API 呼び出し間隔を調整

### データ表示 (view)
- `pull` で保存した情報を SQLite から表示
- `--team-user` や `{team-slug}/users` 引数で特定チームのユーザーを参照
- `--repos-users` でリポジトリに直接追加されたユーザー一覧を確認
- `--all-repos-users` で SQLite に保存された全リポジトリの直接コラボレーターを一覧表示
- `--user-repos <login>` でユーザーがアクセスできるリポジトリと権限を表示（事前に `pull --repos-users`, `pull --repos-teams`, `pull --team-users` を実行）
- `--settings` でマスク済み設定値を確認

### データ操作 (push add/remove)
- 組織・チームからのユーザー追加/削除、チーム削除、外部コラボレーターの招待/削除、`--repos-user` によるリポジトリ協力者の削除に対応（`--permission` で `pull` / `push` / `admin` を指定可能。エイリアス: `read`→`pull`, `write`→`push`）
- デフォルトは DRYRUN (`--exec` 指定時のみ GitHub API を実行)
- `--no-store` で成功後のローカル DB 同期を抑止可能

### データ初期化 (init)
- SQLite テーブルを初期化して保存領域を準備

### バージョン確認 (version)
- ビルド時に埋め込まれたバージョン/コミット/ビルド日時を表示

### MCP サーバー (mcp)
- `./ghub-desk mcp --debug` は go-sdk を組み込んだ MCP サーバーを stdio 上で起動
- 設定ファイルの `mcp.allow_pull` / `mcp.allow_write` で公開するツールを制御

## 設定

### 環境変数

```bash
export GHUB_DESK_ORGANIZATION="your-org-name"      # GitHub 組織名
export GHUB_DESK_GITHUB_TOKEN="your-token"         # GitHub Personal Access Token (PAT)
```

### GitHub App での認証

Personal Access Token の代わりに GitHub App の資格情報でも認証できます。App 認証を利用する場合は、下記の環境変数を設定するか、設定ファイルの `github_app` セクションに値を記載してください（PAT と App を同時に指定することはできません）。

```bash
export GHUB_DESK_APP_ID="123456"                 # GitHub App の App ID
export GHUB_DESK_INSTALLATION_ID="76543210"      # インストール先の Installation ID
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
- リポジトリ名
  - 許可: 英数字・アンダースコア・ハイフンのみ（ドット不可）
  - 先頭にハイフンは不可
  - 長さ: 1〜100 文字
- リポジトリ/ユーザー指定
  - 形式: `{repository}/{username}` を `--outside-user` に渡してください

## 基本的な使用例

### pull

```bash
# 組織メンバーの基本情報を取得・保存
./ghub-desk pull --users

# 取得結果を標準出力へ出しつつ詳細情報も保存
./ghub-desk pull --detail-users --stdout

# チーム一覧を取得。DB を更新せず API 結果のみ確認
./ghub-desk pull --teams --no-store

# リポジトリに直接追加されたユーザーを取得
./ghub-desk pull --repos-users repo-name

# 全リポジトリの直接コラボレーターを取得（セッション再開対応）
./ghub-desk pull --all-repos-users

# 全リポジトリのチーム情報を取得
./ghub-desk pull --all-repos-teams

# 全チームのメンバーを連続取得（リクエスト間隔は既定 3s）
./ghub-desk pull --all-teams-users
```

### view

```bash
# 保存済みのユーザー情報を表示
./ghub-desk view --users

# チーム slug を指定してメンバーを表示
./ghub-desk view --team-user team-slug

# リポジトリに直接追加されたユーザーを表示
./ghub-desk view --repos-users repo-name

# 全リポジトリの直接コラボレーターを表示
./ghub-desk view --all-repos-users

# 全リポジトリのチーム情報を表示
./ghub-desk view --all-repos-teams

# ユーザーがアクセスできるリポジトリと権限を表示（事前に pull --repos-users, --repos-teams, --team-users を実行）
./ghub-desk view --user-repos user-login

# マスク済みの設定値を確認
./ghub-desk view --settings
```

### push

```bash
# チームにユーザーを追加（DRYRUN）
./ghub-desk push add --team-user team-slug/username

# 追加を実行し、成功時にローカル DB も同期
./ghub-desk push add --team-user team-slug/username --exec

# チームからユーザーを削除
./ghub-desk push remove --team-user team-slug/username --exec

# 組織からユーザーを削除
./ghub-desk push remove --user username --exec

# チームを組織から削除
./ghub-desk push remove --team team-slug --exec

# リポジトリの外部コラボレーターを招待（DRYRUN）
./ghub-desk push add --outside-user repo-name/username

# 読み取り専用など権限を指定して招待
./ghub-desk push add --outside-user repo-name/username --permission read

# 外部コラボレーターを削除して DB を同期
./ghub-desk push remove --outside-user repo-name/username --exec

# リポジトリの協力者（外部/組織メンバー）を削除
./ghub-desk push remove --repos-user repo-name/username --exec

# DRYRUN で協力者を削除
./ghub-desk push remove --repos-user repo-name/username
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
# MCP サーバーをビルドして起動
make build
./build/ghub-desk mcp --debug
```

- MCP サーバーは設定の `mcp.allow_pull` / `allow_write` に応じて `pull.*` / `push.*` ツールを公開します。
- `allow_write` を有効にする場合は、`--exec` フラグの利用や DRYRUN で影響範囲を確認してから実行してください。

### MCP ツール

- `health` — 入力不要のヘルスチェック。

#### 参照系 (`view.*`)
- `view.users`, `view.detail-users`, `view.teams`, `view.repos`, `view.outside-users`, `view.token-permission` — 入力なしでキャッシュ済みレコードを返却。
- `view.team-user`（入力: `team`）— 指定チーム（slug）のメンバー一覧。
- `view.repos-users` / `view.repos-teams`（入力: `repository`）— 特定リポジトリの直接コラボレーター / チーム権限。
- `view.all-teams-users`, `view.all-repos-users`, `view.all-repos-teams` — 組織全体のメンバーシップを一括取得。
- `view.user-repos`（入力: `user`）— ユーザーがアクセスできるリポジトリと経路（直接 or チーム）。
- `view.settings` — マスク済み設定情報を返却。

#### データ更新 (`pull.*`)
- 共通オプション: `no_store` (bool), `stdout` (bool), `interval_seconds` (number; 既定 3 秒)。
- `pull.users`, `pull.detail-users`, `pull.teams`, `pull.repositories`, `pull.all-teams-users`, `pull.all-repos-users`, `pull.all-repos-teams`, `pull.outside-users`, `pull.token-permission` — キャッシュ対象をGitHubから更新。
- `pull.team-user`（共通 + `team`）— 指定チームのメンバーを更新。
- `pull.repos-users` / `pull.repos-teams`（共通 + `repository`）— 指定リポジトリのコラボレーター / チーム権限を更新。

#### 更新系 (`push.*`)
- `push.add` — `team_user` または `outside_user` のどちらか一方を指定。`permission`（`pull`/`push`/`admin`、エイリアス `read`→`pull`, `write`→`push`）と `exec` / `no_store` は任意。
- `push.remove` — 削除対象を1つだけ指定（`team` / `user` / `team_user` / `outside_user` / `repos_user`）。`exec` / `no_store` は任意。

## 技術

- **REST API**: GitHub API を利用したデータ取得・操作
- **ローカルデータベース**: SQLite を使用したオフラインでのデータ参照
- **MCP**: Model Context Protocol を介した外部クライアント連携 (`github.com/modelcontextprotocol/go-sdk`)

## ビルド

```bash
make build

```

## テスト

```bash
make test
```

## 対応プラットフォーム

- Go 1.24+
- macOS, Linux, Windows
