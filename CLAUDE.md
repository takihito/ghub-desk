# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## プロジェクト概要

ghub-desk は `go.mod` で定義された Go バージョンを前提とする GitHub 組織管理 CLI および MCP（Model Context Protocol）サーバー。GitHub API 経由で組織のメンバー・チーム・リポジトリを管理し、SQLite にキャッシュする。MCP サーバーは同等の機能を stdio 経由で LLM/エージェントに提供する。

## ビルド・開発コマンド

```bash
make build              # ./build/ghub-desk にバイナリ生成
make test               # 全テスト実行（認証用環境変数はクリアされる）
make deps               # go mod tidy && go mod download
make dev ARGS="pull --users"  # 任意引数で実行
make install            # $GOBIN にインストール
make clean              # ビルド成果物と ghub-desk.db を削除
make goreleaser_build   # GoReleaser のローカルビルド確認
```

単一テスト実行:
```bash
go test ./config -run ^TestGetConfig$
```

コミット前: `gofmt -s -w .` と `go vet ./...` を実行すること。

## アーキテクチャ

### パッケージ構成

- **`cmd/`** — Kong による CLI コマンド定義とハンドラ（`pull`, `view`, `push`, `auditlogs`, `init`, `version`, `mcp`）
- **`github/`** — GitHub API クライアント（PAT または GitHub App 認証）、pull/push ハンドラ
- **`store/`** — SQLite CRUD、バッチ挿入、ビュークエリ、出力フォーマット（table/JSON/YAML）
- **`config/`** — `~/.config/ghub-desk/config.yaml` + 環境変数からの設定読込と検証
- **`session/`** — 長時間の pull 操作用のレジューム状態管理（`~/.config/ghub-desk/session.json`）
- **`mcp/`** — MCP サーバー: ツール登録、権限チェック、ドキュメントリソース（`mcp/docs.go` に埋め込み、`docs/` ではない）
- **`auditlog/`** — 監査ログの取得とパース（日付/リポジトリフィルタ対応）
- **`validate/`** — ユーザー名・チームスラッグ・リポジトリ名の正規表現バリデーション（MCP の JSON Schema にも流用）

### 主要データフロー

**Pull フロー**: `pull` コマンド → `github.InitClient()` で認証 → `github.PullXxx()` でページネーション付き GitHub API 呼出し → `store.StoreXxx()` でバッチ INSERT OR REPLACE → SQLite 保存。各ページ完了後に `session.SavePull()` でレジュームポイントを記録。

**View フロー**: `view` コマンド → `store.Connect()` → `store.HandleViewTarget()` で SELECT → `store.Format()` で table/json/yaml 出力。

**Push フロー**: `push add/remove` → `validate.ParseXxxPair()` で入力検証 → `--exec` なしは DRYRUN 出力のみ → `--exec` 時は GitHub API で変更 → `github.SyncPushXxx()` で DB に反映。

### SQLite テーブル構成

8 テーブル: `ghub_users`, `ghub_teams`, `ghub_repos`, `ghub_team_users`（チーム所属）, `ghub_repos_users`（直接コラボレータ）, `ghub_repos_teams`（チーム権限）, `ghub_outside_users`, `ghub_token_permissions`。

バッチ挿入は SQLite の変数上限（999）を考慮して分割処理される。

### レジューム機構

セッションキーはターゲット + オプション（team/repo/user + store + stdout + interval）のハッシュ。同一パラメータで再実行すると前回中断点から再開。シグナル受信時に状態を保存して終了。

### MCP サーバー

`mcp serve` で stdio モードで起動。`mcp.allow_pull` / `mcp.allow_write` 設定によりツール公開範囲を制御。ツールは view 系（キャッシュ参照）・pull 系（GitHub API 呼出し可能）・push 系（変更操作）に分かれる。

## 安全性ルール

- **push 操作はデフォルトで DRYRUN** — 実際に GitHub を変更するには `--exec` フラグが必要
- MCP ツールの公開範囲は `mcp.allow_pull` と `mcp.allow_write` で制御
- 認証: PAT（`GHUB_DESK_GITHUB_TOKEN`）または GitHub App のいずれか一方のみ使用（`GHUB_DESK_APP_ID`, `GHUB_DESK_INSTALLATION_ID`, `GHUB_DESK_PRIVATE_KEY`）
- シークレットのハードコードやログ出力は禁止。機微情報は出力時にマスクされる
- バージョン情報は `-ldflags` でビルド時に注入（`version.go` と `Makefile` 参照）

## 開発規約

- **コミット**: Conventional Commits 形式（例: `feat(config): --config フラグをサポート`、`fix(store): nil db の扱い`）
- **ブランチ**: `feature/<topic>`, `fix/<topic>`, `chore/<topic>`。`main` への直接 push は禁止
- **PR**: 目安 400 行以内。概要・関連 Issue（`Closes #123`）・テスト結果を含める
- **テスト**: 標準 `testing` パッケージ使用。テーブル駆動推奨。ソースと同じパッケージに `*_test.go` として配置
- **依存追加**: `go get` で追加後、`make deps` で go.mod/go.sum を更新
- **作業ログ**: `work-logs/YYYYMMDD.log` に作業内容を記録すること
