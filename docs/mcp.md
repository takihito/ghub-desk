# MCP サーバー機能ガイド

`ghub-desk` は Model Context Protocol (MCP) を通じて CLI の pull/view/push 機能をエージェントに公開できます。本書では現在実装済みの MCP サーバーの構成、提供ツール、利用方法を整理します。

## 実装概要
- エントリポイント: `ghub-desk mcp`
- 設定: `config.Config.MCP` の `allow_pull` / `allow_write` で公開ツールを制御
- 永続化: CLI と同じ SQLite (`ghub-desk.db` 既定、`database_path` で変更可能)
- 認証: CLI と同じく Personal Access Token もしくは GitHub App のどちらか 1 つを設定

## ビルドモード
| モード | コマンド | 内容 |
| --- | --- | --- |
| スタブサーバー (既定) | `go build ./...`<br>`ghub-desk mcp` | go-sdk に依存しないビルド。起動すると許可されたツール一覧を表示し、Ctrl+C を待機します。|
| go-sdk サーバー | `make build_mcp`<br>`./build/ghub-desk mcp --debug` | `-tags mcp_sdk` でビルドし、`github.com/modelcontextprotocol/go-sdk` を使った本格的な MCP サーバーを stdio で起動します。|

> go-sdk サーバーのみが MCP クライアントからの JSON-RPC に応答します。スタブは開発時の確認用です。

## 設定例
`~/.config/ghub-desk/config.yaml`

```yaml
organization: "your-org"
github_token: "${GHUB_DESK_GITHUB_TOKEN}" # または github_app セクション

database_path: "./ghub-desk.db"

mcp:
  allow_pull: true   # pull.* を登録
  allow_write: false # push.* は無効化 (安全側)
```

`allow_pull` と `allow_write` は MCP サーバーのツール登録に直結します。`nil` 設定時は false と同等です。

## 提供ツール
### 共通
| ツール名 | 説明 | 入力 | 備考 |
| --- | --- | --- | --- |
| `health` | サーバーヘルス確認 | なし | `{"status":"ok","time":"RFC3339"}` を返却 |

### view.* (常時利用可能)
すべてローカル DB を参照し、最大 200 件（チームメンバーは 500 件）を返します。

| ツール名 | 説明 | 入力 | 出力概要 |
| --- | --- | --- | --- |
| `view.users` | 組織ユーザー一覧 | なし | `users[]` に `id`, `login`, `name`, `email` など |
| `view.detail-users` | 詳細ユーザー（現状は `view.users` と同じ） | なし | `users[]` |
| `view.teams` | チーム情報 | なし | `teams[]` に `id`, `slug`, `name`, `description`, `privacy` |
| `view.repos` | リポジトリ情報 | なし | `repositories[]` に `name`, `full_name`, `private`, `language`, `stars` |
| `view.team-user` | 指定チームのメンバー | `{ "team": "team-slug" }` | `users[]` に `user_id`, `login`, `role` |
| `view.outside-users` | Outside Collaborator | なし | `users[]` |
| `view.token-permission` | `pull.token-permission` の保存内容 | なし | PAT/GitHub App 権限情報。未取得の場合はエラー |

### pull.* (`allow_pull: true` の場合のみ)
GitHub API を呼び出し、成功時に既定で SQLite を更新します。`no_store: true` で保存を抑止、`stdout: true` で API レスポンスを標準出力にコピーします。

| ツール名 | 説明 | 入力 | 備考 |
| --- | --- | --- | --- |
| `pull.users` | 組織ユーザー取得 | `{ "no_store"?, "stdout"?, "detail"? }` | `detail:true` で `detail-users` を取得 |
| `pull.teams` | チーム一覧取得 | `{ "no_store"?, "stdout"? }` |  |
| `pull.repositories` | リポジトリ一覧取得 | `{ "no_store"?, "stdout"? }` |  |
| `pull.team-user` | チームメンバー取得 | `{ "team", "no_store"?, "stdout"? }` | `team` は slug 形式 (`team-slug`) |
| `pull.outside-users` | Outside Collaborator 取得 | `{ "no_store"?, "stdout"? }` |  |
| `pull.token-permission` | トークン権限情報取得 | `{ "no_store"?, "stdout"? }` | 最新のレスポンスを DB に保存 |

### push.* (`allow_write: true` の場合のみ)
GitHub 側へ変更を加える操作です。既定では DRYRUN として実行内容を返し、`exec: true` を指定したときのみ API を呼び出します。`no_store: true` で成功後のローカル DB 同期 (`SyncPushAdd/Remove`) をスキップできます。

| ツール名 | 説明 | 入力 | 備考 |
| --- | --- | --- | --- |
| `push.add` | チームへのユーザー追加 | `{ "team_user", "exec"?, "no_store"? }` | `team_user` は `team-slug/username`。成功時 `message` を返す |
| `push.remove` | チーム削除 / 組織ユーザー削除 / チームメンバー削除 | `{ "team"? \| "user"? \| "team_user"?, "exec"?, "no_store"? }` | 対象はいずれか 1 つだけ指定。`exec:false` は DRYRUN |

## 起動例
```bash
# スタブサーバー (許可ツール一覧のみ表示)
go run ./cmd/ghub-desk mcp

# go-sdk サーバー (実際の MCP を提供)
make build_mcp
./build/ghub-desk mcp --debug
```

MCP クライアント（例: MCP Inspector）をサブプロセスとして起動すると、`health` / `view.*` / `pull.*` / `push.*` を呼び出せます。`allow_write` を有効にする前に DRYRUN 出力で影響範囲を確認してください。

## エラーハンドリングと注意点
- 認証設定が無い、もしくは PAT と GitHub App を同時に設定している場合はサーバー起動時にエラーになります。
- `view.token-permission` は保存済みデータが無いとエラーを返します（`pull.token-permission` を `no_store:false` で実行してください）。
- DB パスは `config.yaml` や `GHUB_DESK_DB_PATH` で上書き可能です。MCP 側でも同じパスを利用します。
- MCP サーバーからの pull/push 実行は CLI と同じレート制限・権限に従います。GitHub App 認証を利用する場合は `GHUB_DESK_APP_ID` / `GHUB_DESK_INSTALLATION_ID` / `GHUB_DESK_PRIVATE_KEY` を設定してください。

## 今後のアイデア
- レスポンススキーマの細分化（JSON Schema）
- イベント/ストリーミング対応
- pull 結果の差分比較ツール
