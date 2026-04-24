# MCP ドキュメントと実装の乖離

## 概要

`pages/mcp-server.md`（EN/JA）に掲載されている Available Tools 一覧が、`mcp/serve_mcp.go` に実際に登録されているツールと一致していなかった。PR #165 のレビューで Copilot に指摘されて発覚。

## 詳細

以下のツールが実装済みにもかかわらずドキュメントに未掲載だった:

| ツール名 | 入力 |
|---|---|
| `view_user` | `{ "user": "github-login" }` |
| `view_user-teams` | `{ "user": "github-login" }` |
| `view_team-repos` | `{ "team": "team-slug" }` |
| `pull_detail-users` | なし |

また、`view_*` ツールの説明に「最大 200 件（チームメンバー: 500 件）」という記述があったが、実装側に LIMIT は存在せず事実と異なっていた。

ドキュメントは手動管理されており、新規ツールを `serve_mcp.go` に追加した際に pages 側を更新し忘れるリスクが高い。

## 提案

- ツール追加・削除時に `pages/mcp-server.md`（EN/JA）の更新を PR チェックリストに明記する
- 可能であれば `mcp/serve_mcp.go` のツール一覧から pages のテーブルを自動生成するスクリプトを用意し、CI で差分を検出する
- 件数上限は実装に合わせて「件数上限なし」と明示する（または実装側に LIMIT を追加する）
