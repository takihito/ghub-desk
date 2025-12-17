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
| デフォルト | `go build ./...`<br>`ghub-desk mcp --debug` | `github.com/modelcontextprotocol/go-sdk` をリンクした MCP サーバーを stdio で提供します。|
| 互換ターゲット | `make build_mcp`<br>`./build/ghub-desk mcp --debug` | `make build` と同じ成果物を生成する後方互換ターゲットです。|

> 以前のスタブモードは廃止され、標準ビルドでそのまま go-sdk MCP サーバーを利用できます。

## 設定例
`~/.config/ghub-desk/config.yaml`

```yaml
organization: "your-org"
github_token: "${GHUB_DESK_GITHUB_TOKEN}" # または github_app セクション

database_path: "./ghub-desk.db"

mcp:
  allow_pull: true   # pull_* を登録
  allow_write: false # push_* は無効化 (安全側)
```

`allow_pull` と `allow_write` は MCP サーバーのツール登録に直結します。`nil` 設定時は false と同等です。

## 提供ツール
### 共通
| ツール名 | 説明 | 入力 | 備考 |
| --- | --- | --- | --- |
| `health` | サーバーヘルス確認 | なし | `{"status":"ok","time":"RFC3339"}` を返却 |

### view_* (常時利用可能)
すべてローカル DB を参照し、最大 200 件（チームメンバーは 500 件）を返します。

| ツール名 | 説明 | 入力 | 出力概要 |
| --- | --- | --- | --- |
| `view_users` | 組織ユーザー一覧 | なし | `users[]` に `id`, `login`, `name`, `email` など |
| `view_detail-users` | 詳細ユーザー（現状は `view_users` と同じ） | なし | `users[]` |
| `view_teams` | チーム情報 | なし | `teams[]` に `id`, `slug`, `name`, `description`, `privacy` |
| `view_repos` | リポジトリ情報 | なし | `repositories[]` に `name`, `full_name`, `private`, `language`, `stars` |
| `view_team-user` | 指定チームのメンバー | `{ "team": "team-slug" }` | `team` は英数字+ハイフンで構成された slug |
| `view_repos-users` | リポジトリの直接コラボレーター | `{ "repository": "repo-name" }` | `repository` は 1-100 文字・英数字/アンダースコア/ハイフン |
| `view_repos-teams` | リポジトリに紐づくチーム | `{ "repository": "repo-name" }` | 同上 |
| `view_user-repos` | ユーザーがアクセスできるリポジトリ | `{ "user": "github-login" }` | `user` は 1-39 文字の英数字/ハイフン |
| `view_outside-users` | Outside Collaborator | なし | `users[]` |
| `view_token-permission` | `pull_token-permission` の保存内容 | なし | PAT/GitHub App 権限情報。未取得の場合はエラー |
| `view_settings` | マスク済み設定の確認 | なし | `organization`, `allow_pull`/`allow_write`, DB パスなど |

### pull_* (`allow_pull: true` の場合のみ)
GitHub API を呼び出し、成功時に既定で SQLite を更新します。`no_store: true` で保存を抑止、`stdout: true` で API レスポンスを標準出力にコピーします。

| ツール名 | 説明 | 入力 | 備考 |
| --- | --- | --- | --- |
| `pull_users` | 組織ユーザー取得 | `{ "no_store"?, "stdout"?, "interval_seconds"?, "detail"? }` | `detail:true` で `detail-users` を取得 |
| `pull_teams` | チーム一覧取得 | `{ "no_store"?, "stdout"?, "interval_seconds"? }` |  |
| `pull_repositories` | リポジトリ一覧取得 | `{ "no_store"?, "stdout"?, "interval_seconds"? }` |  |
| `pull_team-user` | チームメンバー取得 | `{ "team", "no_store"?, "stdout"?, "interval_seconds"? }` | `team` は slug 形式 (`team-slug`) |
| `pull_repos-users` | リポジトリの直接コラボ取得 | `{ "repository", "no_store"?, "stdout"?, "interval_seconds"? }` | `repository` は 1-100 文字の英数字/アンダースコア/ハイフン |
| `pull_repos-teams` | リポジトリに紐づくチーム取得 | `{ "repository", "no_store"?, "stdout"?, "interval_seconds"? }` | 同上 |
| `pull_outside-users` | Outside Collaborator 取得 | `{ "no_store"?, "stdout"?, "interval_seconds"? }` |  |
| `pull_token-permission` | トークン権限情報取得 | `{ "no_store"?, "stdout"?, "interval_seconds"? }` | 最新のレスポンスを DB に保存 |

### push_* (`allow_write: true` の場合のみ)
GitHub 側へ変更を加える操作です。既定では DRYRUN として実行内容を返し、`exec: true` を指定したときのみ API を呼び出します。`no_store: true` で成功後のローカル DB 同期 (`SyncPushAdd/Remove`) をスキップできます。

| ツール名 | 説明 | 入力 | 備考 |
| --- | --- | --- | --- |
| `push_add` | チームへのユーザー追加 or 外部コラボ招待 | `{ "team_user"?, "outside_user"?, "permission"?, "exec"?, "no_store"? }` | `team_user` は `team-slug/username`、`outside_user` は `repo-name/username`。`permission` で `pull`/`push`/`admin` を指定可能 |
| `push_remove` | チーム削除 / 組織ユーザー削除 / 各種コラボ削除 | `{ "team"?, "user"?, "team_user"?, "outside_user"?, "repos_user"?, "exec"?, "no_store"? }` | いずれか 1 つだけ対象を指定（`team_user`/`outside_user`/`repos_user` は `repo-or-team/username` 形式）。`exec:false` は DRYRUN |

## ドキュメントリソース（resources/*）
MCP の `resources/list` と `resources/read` で、ツールの使い方ガイドを取得できます。`tools/list` にも各ツールの Description 末尾へリソース URI を付与しているので、エージェントは URI をたどって詳細を読む前提で設計しています。

| リソース URI | 内容 |
| --- | --- |
| `resource://ghub-desk/mcp-overview` | サーバー概要、`allow_pull` / `allow_write`、DB の扱い、起動例の要約。 |
| `resource://ghub-desk/mcp-tools` | 各ツールの入力例・必須/任意パラメータ・レスポンス概要。`#view_team-user` などのアンカーで特定ツールへジャンプ可能。 |
| `resource://ghub-desk/mcp-safety` | `push_*` の DRYRUN/exec、`no_store`、権限設定の注意点。 |

呼び出し例:
```json
// リソース一覧を取得
{ "method": "resources/list" }

// 特定リソースを読む（例: pull_users の詳細）
{
  "method": "resources/read",
  "params": { "uri": "resource://ghub-desk/mcp-tools#pull_users" }
}
```

## 起動例
```bash
make build
./build/ghub-desk mcp --debug --log-path /tmp/ghub-desk.log  # stderr/デバッグログをファイルへ出力（エイリアス: --error-log-path）
```

`make build_mcp` を使用しても同じバイナリ (`./build/ghub-desk`) が生成されます。MCP クライアント（例: MCP Inspector）をサブプロセスとして起動すると、`health` / `view_*` / `pull_*` / `push_*` を呼び出せます。`allow_write` を有効にする前に DRYRUN 出力で影響範囲を確認してください。

## エラーハンドリングと注意点
- 認証設定が無い、もしくは PAT と GitHub App を同時に設定している場合はサーバー起動時にエラーになります。
- `view_token-permission` は保存済みデータが無いとエラーを返します（`pull_token-permission` を `no_store:false` で実行してください）。
- DB パスは `config.yaml` や `GHUB_DESK_DB_PATH` で上書き可能です。MCP 側でも同じパスを利用します。
- MCP サーバーからの pull/push 実行は CLI と同じレート制限・権限に従います。GitHub App 認証を利用する場合は `GHUB_DESK_APP_ID` / `GHUB_DESK_INSTALLATION_ID` / `GHUB_DESK_PRIVATE_KEY` を設定してください。

## AI エージェントへの組み込み

Gemini や Codex などの AI エージェントから MCP サーバーを利用する場合、各クライアントの設定に MCP サーバーの起動コマンドを登録します。以下は代表的な設定例です。

### resources/list と公開リソース
- `resources/list` → `resources/read` の順に呼び出すと、MCP サーバーに組み込まれた使い方ガイドを取得できます。
- 各 `tools` の Description 末尾にも `resource://ghub-desk/...` の URI を記載しているため、エージェントはリンク先を `resources/read` で取得してから `tools/call` を実行してください。

| リソース URI | 説明 |
| --- | --- |
| `resource://ghub-desk/mcp-overview` | 起動手順、`allow_pull`/`allow_write` の意味、SQLite の扱いをまとめた概要。 |
| `resource://ghub-desk/mcp-tools` | 各ツールごとの入力例、レスポンス概要、利用時の注意点。アンカー `#view_team-user` などで特定ツールに直接ジャンプできます。 |
| `resource://ghub-desk/mcp-safety` | `push_*` 実行時の DRYRUN/exec の切り替えや `no_store` などの運用上の注意点。 |

`tools/list` で Description に含まれる URI を確認 → `resources/list` で公開リソース ID を学習 → `resources/read {"uri":"resource://ghub-desk/mcp-tools#pull_users"}` のように読み取る、という流れを推奨します。

### リソース URI の扱い
`resource://ghub-desk/...` というURIはファイルパスではなく、`docs/` ディレクトリからファイルを読み込むものではない点に注意してください。これらのリソースの内容は、`mcp/docs.go` のGoソースコード内にマークダウン文字列として直接埋め込まれています。サーバーはこれらのURIへのリクエストに対し、対応する埋め込み文字列を返します。

`docs/` ディレクトリ内のマークダウンファイルは、開発者がプロジェクトの構造を理解するために用意されたものであり、MCP経由でエージェントに提供されるコンテンツとは区別されます。

### Gemini

`~/.gemini/settings.json` の `mcpServers` セクションに `ghub-desk` MCP サーバーを追加します。

```json
{
  "mcpServers": {
    "ghub-desk": {
      "command": "/home/takihito/bin/ghub-desk",
      "args": [
        "mcp",
        "--debug",
        "--config",
        "/home/takihito/.config/ghub-desk/config.yaml"
      ],
      "transport": "stdio",
      "retry": {
        "maxRestarts": 5,
        "windowSeconds": 60
      }
    }
  }
}
```

パスは環境に合わせて調整してください。`--config` で指定するファイルは CLI と共通の設定を利用できます。

### Codex

`~/.codex/config.toml` の `[mcp_servers]` セクションに `ghub-desk` MCP サーバーを追加します。

```toml
[mcp_servers.ghub-desk]
command = "/home/takihito/bin/ghub-desk"
args = [
  "mcp",
  "--debug",
  "--config",
  "/home/takihito/.config/ghub-desk/config.yaml"
]
```

Codex CLI では設定後に再起動すると登録が反映されます。`allow_pull` や `allow_write` の設定値は CLI と同じ設定ファイルで管理されるため、エージェントに許可したい操作に応じて事前に見直してください。



## 今後のアイデア
- レスポンススキーマの細分化（JSON Schema）
- イベント/ストリーミング対応
- pull 結果の差分比較ツール
