# ghub-desk

GitHub Organization Management CLI & MCP Server

[Read this document in English](README.md)

**Author:** Takeda Akihito

## インストール

### `go install`
Go 1.24 以降が入っている環境で `go install` を使うと `$GOBIN`（既定は `$GOPATH/bin`）に最新版が配置されます。

```bash
GO111MODULE=on go install github.com/takihito/ghub-desk@latest
```

`$GOBIN` を `PATH` に通し、`ghub-desk version` でインストールを確認してください。

### リリースアーカイブをダウンロード (curl)

[Releases](https://github.com/takihito/ghub-desk/releases) からプラットフォーム別バイナリを取得できます。事前に `VERSION` を好みのリリースタグへ設定してから実行してください。


```bash
# 事前に VERSION をリリースタグへ設定してください（例: export VERSION=0.2.0）
OS=${OS:-Darwin}            # Darwin / Linux / Windows
ARCH=${ARCH:-arm64}         # arm64 / x86_64
ARTIFACT="ghub-desk_${VERSION}_${OS}_${ARCH}.tar.gz"

curl -L -o "${ARTIFACT}" \
  "https://github.com/takihito/ghub-desk/releases/download/v${VERSION}/${ARTIFACT}"
# SHA256_FROM_RELEASE をリリースページ記載の値へ差し替えて検証
echo "SHA256_FROM_RELEASE  ${ARTIFACT}" | shasum -a 256 --check
sudo tar -xzf "${ARTIFACT}" -C /usr/local/bin ghub-desk
```

プラットフォームごとのアーティファクト名と SHA-256 はリリースページに記載されています。Windows ではアーカイブ展開後に `ghub-desk.exe` を `%PATH%` 上へ配置してください。新しいバージョンへ更新する際は `VERSION` を入れ替えるだけで同じ手順を再利用できます。

### ソースからビルド

リポジトリをクローンして `make build` を実行すると `./build/ghub-desk` が生成されます。未リリースの変更を取り込みたい場合やコードを確認したい場合に便利です。

```bash
git clone https://github.com/takihito/ghub-desk.git
cd ghub-desk
make deps
make build
sudo cp build/ghub-desk /usr/local/bin/
```

ローカルで改修する際は `make test` を先に実行しておくことを推奨します。

## 概要

`ghub-desk`は、GitHub 組織のメンバー・チーム・リポジトリ情報を整理するためのコマンドラインツールです。GitHub API と連携して組織情報を取得し、SQLite にキャッシュしてオフライン参照を支援します。また、Model Context Protocol (MCP) サーバーとして起動することで、LLM やエージェントから同じ機能を安全に呼び出すことができます。

- GitHub API を利用した `pull` / `view` / `push` コマンド
- 変更系コマンドは DRYRUN を標準とし、`--exec` 指定時のみ実行
- 設定ファイルと環境変数での柔軟な構成 (DB パス、MCP 権限など)
- MCP ツール経由での自動化と統合

## コアコマンド

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

## 使い方

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

### init

`init` コマンドは設定ファイル生成とデータベース初期化のサブコマンドに分かれています（英語版 README の説明と同じ構成です）。

```bash
# 既定パス (~/.config/ghub-desk/config.yaml) に設定ファイルのひな形を作成
./ghub-desk init config

# 任意パスに設定ファイルを作成（必要なディレクトリは自動作成）
./ghub-desk init config --target-file ~/ghub/config.yaml

# 設定ファイルの database_path（未設定なら ./ghub-desk.db）を初期化
./ghub-desk init db

# 明示的な DB パスを初期化（既存ファイルがある場合は警告を出して中断）
./ghub-desk init db --target-file ~/data/ghub-desk.db
```

### version

```bash
# バージョン情報を表示
./ghub-desk version
```

## MCP サーバー (mcp)


```bash
# MCP サーバーをビルドして起動
make build
./build/ghub-desk mcp --debug
```

- MCP サーバーは設定の `mcp.allow_pull` / `allow_write` に応じて `pull_*` / `push_*` ツールを公開します。
- `allow_write` を有効にする場合は、`--exec` フラグの利用や DRYRUN で影響範囲を確認してから実行してください。

### MCP ツール

- `health` — 入力不要のヘルスチェック。

#### 参照系 (`view_*`)
- `view_users`, `view_detail-users`, `view_teams`, `view_repos`, `view_outside-users`, `view_token-permission` — 入力なしでキャッシュ済みレコードを返却。
- `view_team-user`（入力: `team`）— 指定チーム（slug）のメンバー一覧。
- `view_repos-users` / `view_repos-teams`（入力: `repository`）— 特定リポジトリの直接コラボレーター / チーム権限。
- `view_all-teams-users`, `view_all-repos-users`, `view_all-repos-teams` — 組織全体のメンバーシップを一括取得。
- `view_user-repos`（入力: `user`）— ユーザーがアクセスできるリポジトリと経路（直接 or チーム）。
- `view_settings` — マスク済み設定情報を返却。

#### データ更新 (`pull_*`)
- 共通オプション: `no_store` (bool), `stdout` (bool), `interval_seconds` (number; 既定 3 秒)。
- `pull_users`, `pull_detail-users`, `pull_teams`, `pull_repositories`, `pull_all-teams-users`, `pull_all-repos-users`, `pull_all-repos-teams`, `pull_outside-users`, `pull_token-permission` — キャッシュ対象をGitHubから更新。
- `pull_team-user`（共通 + `team`）— 指定チームのメンバーを更新。
- `pull_repos-users` / `pull_repos-teams`（共通 + `repository`）— 指定リポジトリのコラボレーター / チーム権限を更新。

#### 更新系 (`push_*`)
- `push_add` — `team_user` または `outside_user` のどちらか一方を指定。`permission`（`pull`/`push`/`admin`、エイリアス `read`→`pull`, `write`→`push`）と `exec` / `no_store` は任意。
- `push_remove` — 削除対象を1つだけ指定（`team` / `user` / `team_user` / `outside_user` / `repos_user`）。`exec` / `no_store` は任意。

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
