---
layout: default
title: ghub-desk MCP サーバー
description: ghub-desk MCP サーバーのドキュメント — ツール一覧、設定、AI エージェント連携。
---

# MCP サーバー

`ghub-desk` は [Model Context Protocol](https://modelcontextprotocol.io/)（MCP）を通じて、
CLI の pull/view/push 機能を AI エージェントに公開できます。

## クイックスタート

```bash
make build
./build/ghub-desk mcp --debug
```

## 設定

`~/.config/ghub-desk/config.yaml`

```yaml
organization: "your-org"
github_token: "${GHUB_DESK_GITHUB_TOKEN}"

database_path: "./ghub-desk.db"

mcp:
  allow_pull: true    # pull_* ツールを登録
  allow_write: false  # push_* は安全側で無効
```

`allow_pull` と `allow_write` は MCP サーバーのツール登録に直結します。省略時はどちらも `false` です。

## 提供ツール

### 共通

| ツール名 | 説明 | 入力 |
|---|---|---|
| `health` | サーバーヘルス確認 | なし |

### view_* (常時利用可能)

ローカル SQLite を参照して結果を返します。現状の実装では件数上限は適用されておらず、返却件数はキャッシュされているデータ量に依存します。

| ツール名 | 説明 | 入力 |
|---|---|---|
| `view_users` | 組織ユーザー一覧 | なし |
| `view_detail-users` | 詳細ユーザー | なし |
| `view_user` | 単一ユーザーのプロフィール | `{ "user": "github-login" }` |
| `view_user-teams` | ユーザーが所属するチーム | `{ "user": "github-login" }` |
| `view_teams` | チーム情報 | なし |
| `view_repos` | リポジトリ情報 | なし |
| `view_team-user` | 指定チームのメンバー | `{ "team": "team-slug" }` |
| `view_team-repos` | チームがアクセスできるリポジトリ | `{ "team": "team-slug" }` |
| `view_repos-users` | リポジトリの直接コラボレーター | `{ "repository": "repo-name" }` |
| `view_repos-teams` | リポジトリに紐づくチーム | `{ "repository": "repo-name" }` |
| `view_repos-teams-users` | リポジトリに紐づくチームのメンバー | `{ "repository": "repo-name" }` |
| `view_all-teams-users` | 全チームメンバーシップ | なし |
| `view_all-repos-users` | 全直接コラボレーター | なし |
| `view_all-repos-teams` | 全リポジトリ-チームリンク | なし |
| `view_user-repos` | ユーザーがアクセスできるリポジトリ | `{ "user": "github-login" }` |
| `view_outside-users` | Outside Collaborator | なし |
| `view_token-permission` | トークン権限情報（キャッシュ） | なし |
| `view_settings` | マスク済み設定情報 | なし |

### auditlogs (常時利用可能)

| ツール名 | 説明 | 入力 |
|---|---|---|
| `auditlogs` | 監査ログを actor で取得 | `{ "user": "octocat", "created"?, "repo"?, "per_page"? }` |

### pull_* (`allow_pull: true` の場合のみ)

GitHub API を呼び出し、SQLite を更新します。共通オプション: `no_store` (bool), `stdout` (bool), `interval_seconds` (number)。

| ツール名 | 説明 | 必須入力 |
|---|---|---|
| `pull_users` | 組織ユーザー取得 | なし |
| `pull_detail-users` | 詳細ユーザー情報取得 | なし |
| `pull_teams` | チーム一覧取得 | なし |
| `pull_repositories` | リポジトリ一覧取得 | なし |
| `pull_team-user` | チームメンバー取得 | `team` |
| `pull_repos-users` | リポジトリの直接コラボ取得 | `repository` |
| `pull_repos-teams` | リポジトリのチーム権限取得 | `repository` |
| `pull_all-teams-users` | 全チームメンバー取得 | なし |
| `pull_all-repos-users` | 全直接コラボレーター取得 | なし |
| `pull_all-repos-teams` | 全リポジトリ-チームリンク取得 | なし |
| `pull_outside-users` | Outside Collaborator 取得 | なし |
| `pull_token-permission` | トークン権限情報取得 | なし |

### push_* (`allow_write: true` の場合のみ)

GitHub 側の変更操作。**デフォルトは DRYRUN** — `exec: true` を指定して初めて API を呼び出します。

| ツール名 | 説明 | 入力 |
|---|---|---|
| `push_add` | チームへの追加 / 外部コラボ招待 | `team_user` または `outside_user`; `permission`, `exec`, `no_store` は任意 |
| `push_remove` | チーム・ユーザー・コラボの削除 | `team`, `user`, `team_user`, `outside_user`, `repos_user` のいずれか1つ; `exec`, `no_store` は任意 |

## 組み込みドキュメントリソース

`resources/list` → `resources/read` の順に呼び出すと、使い方ガイドを取得できます。

| リソース URI | 内容 |
|---|---|
| `resource://ghub-desk/mcp-overview` | 起動手順、設定、SQLite の扱い |
| `resource://ghub-desk/mcp-tools` | 各ツールの詳細リファレンス（入力例・レスポンス概要） |
| `resource://ghub-desk/mcp-safety` | DRYRUN/exec の使い分け、allow_write の注意点 |

## AI エージェントへの組み込み

### Claude Code

```json
{
  "mcpServers": {
    "ghub-desk": {
      "command": "/path/to/ghub-desk",
      "args": ["mcp", "--debug", "--config", "~/.config/ghub-desk/config.yaml"],
      "transport": "stdio"
    }
  }
}
```

### Gemini

`~/.gemini/settings.json`:

```json
{
  "mcpServers": {
    "ghub-desk": {
      "command": "/path/to/ghub-desk",
      "args": ["mcp", "--debug", "--config", "/path/to/config.yaml"],
      "transport": "stdio",
      "retry": { "maxRestarts": 5, "windowSeconds": 60 }
    }
  }
}
```

### Codex

`~/.codex/config.toml`:

```toml
[mcp_servers.ghub-desk]
command = "/path/to/ghub-desk"
args = ["mcp", "--debug", "--config", "/path/to/config.yaml"]
```

## 注意事項

- PAT と GitHub App を同時に設定するとサーバー起動時にエラーになります
- `view_token-permission` は保存済みデータがないとエラーを返します（先に `pull_token-permission` を実行してください）
- `resource://ghub-desk/...` URI はファイルパスではなく、`mcp/docs.go` に埋め込まれた Go ソース内の文字列として提供されます
