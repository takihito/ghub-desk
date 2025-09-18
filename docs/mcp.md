# MCP サーバー機能（計画と使用例）

本ドキュメントは、ghub-desk に Model Context Protocol (MCP) サーバー機能を追加するための計画および使用例を示します。実装は段階的に進めます。

## 目的
- LLM/エージェントなどの MCP クライアントから、組織の GitHub 情報取得・表示・一部操作（慎重に制限）を行えるサーバーを提供する。
- 既存の CLI 機能（pull/view/push）を安全にラップし、`config.yaml` による権限制御を行う。

## 依存
- go SDK: https://github.com/modelcontextprotocol/go-sdk

## 設計概要
- 新コマンド: `ghub-desk mcp`（サブコマンド `serve` は付けずに `mcp` 単独でサーバー起動を想定）
- 新パッケージ: `mcp/`（サーバー起動ロジックとツール登録）
  - `server.go`: go-sdk で MCP サーバーを初期化
  - `tools_pull.go`: pull 系ツール（users, teams, repositories, team-users, outside-users, token-permission）
  - `tools_view.go`: view 系ツール（同上、ローカルDB参照）
  - `tools_push.go`: push 系ツール（remove/add）
  - 既存の `github/`, `store/`, `config/` を呼び出すラッパー
- 権限制御（config.yaml; 後述）
  - `mcp.allow_pull`: GitHub API からの取得（および DB への保存を含む）を許可
  - `mcp.allow_write`: GitHub への変更（push add/remove）を許可
  - 既定は安全側: `allow_pull: false`, `allow_write: false`
  - 設定が `false` の機能はツール未登録（見えない）または実行時に 403 を返す
- 通信方式
  - 初期実装は標準入出力（stdio）を想定（MCP 標準）。将来的にソケット/HTTP は検討
  - `ghub-desk mcp` 実行で stdio 上にサーバーを立て、クライアントは同プロセス起動で接続

### ツール設計（例）
以下は代表例。最終的なスキーマは go-sdk の `schema`/`jsonschema` に合わせて定義。

- `pull.users`
  - params: `{ store?: boolean, detail?: boolean }`
  - effect: GitHub から users を取得。`store=true` の場合は `store/` 経由で DB 保存
  - requires: `allow_pull`

- `pull.teams`
  - params: `{ store?: boolean }`
  - requires: `allow_pull`

- `pull.repositories`
  - params: `{ store?: boolean }`
  - requires: `allow_pull`

- `pull.team-users`
  - params: `{ team: string, store?: boolean }`
  - requires: `allow_pull`

- `pull.outside-users`
  - params: `{ store?: boolean }`
  - requires: `allow_pull`

- `pull.token-permission`
  - params: `{ store?: boolean }`
  - requires: `allow_pull`

- `view.users`
  - params: `{ detail?: boolean }`
  - effect: DB から表示用データを返す（テキスト/構造化）

- `view.*` 系（teams/repositories/team-users/outside-users/token-permission）
  - params: 必要に応じて team 名など

- `push.remove`
  - params: `{ target: "team"|"user", name: string, exec?: boolean }`
  - requires: `allow_write`
  - note: `exec=false` は DRYRUN

- `push.add`
  - params: `{ target: "team-user", value: "team/user", role?: string, exec?: boolean }`
  - requires: `allow_write`

### エラーポリシー
- JSON-RPC/MCP のエラーコードを使用（既存 `mcp/errors.go` のコードに従う）
- 権限制御で拒否時は 403 を返す

## 実装ステップ
1) 依存追加: `go get github.com/modelcontextprotocol/go-sdk`
2) `config` に `mcp` セクションを追加（例は下記）
3) `cmd` に `mcp` サブコマンドを追加
   - `Execute` 経由で `mcp.Serve()` を呼ぶ
4) `mcp` パッケージ新設
   - `server.go` にサーバー初期化（stdio transport）
   - `tools_*.go` で各ツール登録とパラメータ検証
   - 既存の `github`/`store` へ橋渡し
5) テスト
   - ハンドラのユニットテスト（権限制御、パラメータバリデーション、エラー変換）
   - 最小の結合テスト（stdio をモックしてリクエスト/レスポンスの往復確認）
6) ドキュメント整備
   - 本ファイル（docs/mcp.md）の更新
   - `README.md` と `config.yaml.example` 更新

## 使用例（予定）

### 設定（~/.config/ghub-desk/config.yaml）
```yaml
organization: "your-org"
github_token: "${GHUB_DESK_GITHUB_TOKEN}"

mcp:
  allow_pull: true      # GitHub API からの取得を許可
  allow_write: false    # 変更操作は不可（安全側）
```

### サーバー起動（スタブ: デフォルトビルド）
```bash
ghub-desk mcp
```

デフォルトビルドではスタブサーバーが起動し、許可されたツール一覧を表示して待機します。
本番相当の go-sdk 実装はビルドタグ `mcp_sdk` で有効化します。

### サーバー起動（go-sdk: 本実装）
```bash
go build -tags mcp_sdk -o build/ghub-desk .
./build/ghub-desk mcp --debug
```

MCP クライアント（例: MCP Inspector やエージェント）から接続すると、
`view.*` や（許可されていれば）`pull.*`、`push.*` のツールが使用可能になります。

### ツール呼び出し例（概念）
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "pull.users",
    "arguments": { "store": true, "detail": false }
  }
}
```

### 注意事項
- `allow_write: true` を有効にする場合は、DRYRUN を活用し影響範囲を十分に確認してください。
- 認証設定（PAT または GitHub App）は既存設定に従います。

## 今後の拡張
- outputs を構造化（JSON Schema）してクライアント側で再利用しやすくする
- イベント/ストリーミング対応
- サブリソースの詳細取得や差分検出
