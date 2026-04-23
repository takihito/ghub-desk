---
layout: default
title: ghub-desk 使い方
description: ghub-desk のコマンドリファレンス — pull, view, push, auditlogs, init, version, mcp。
---

# 使い方

## pull — GitHub からデータを取得

GitHub API から組織データを取得し、ローカルの SQLite データベースに保存します。

**ターゲット:** `users`, `detail-users`, `teams`, `repos`, `repos-users`, `all-repos-users`, `repos-teams`, `all-repos-teams`, `team-user`, `all-teams-users`, `outside-users`, `token-permission`

```bash
# 組織メンバーを取得・保存
ghub-desk pull --users

# 詳細情報を取得しつつ標準出力にも表示
ghub-desk pull --detail-users --stdout

# チーム一覧を取得（DB を更新しない）
ghub-desk pull --teams --no-store

# リポジトリの直接コラボレーターを取得
ghub-desk pull --repos-users repo-name

# 全リポジトリの直接コラボレーターを取得（再開対応）
ghub-desk pull --all-repos-users

# 全リポジトリのチーム情報を取得
ghub-desk pull --all-repos-teams

# 全チームのメンバーを取得（デフォルト間隔: 3s）
ghub-desk pull --all-teams-users
```

`--interval-time` で API 呼び出し間隔を調整できます。`--no-store` / `--stdout` で保存・出力を制御できます。

## view — キャッシュデータを表示

`pull` で保存したデータを SQLite から表示します。

```bash
# 組織メンバー一覧
ghub-desk view --users

# 個別ユーザーのプロファイル
ghub-desk view --user user-login

# ユーザーの所属チーム
ghub-desk view --user-teams user-login

# チームのメンバー
ghub-desk view --team-user team-slug

# リポジトリの直接コラボレーター
ghub-desk view --repos-users repo-name

# リポジトリに紐づくチームのメンバー
# (事前に pull --repos-teams && pull --all-teams-users が必要)
ghub-desk view --repos-teams-users repo-name

# 全リポジトリの直接コラボレーター
ghub-desk view --all-repos-users

# 全リポジトリのチーム権限
ghub-desk view --all-repos-teams

# チームがアクセスできるリポジトリ
ghub-desk view --team-repos team-slug

# ユーザーがアクセスできるリポジトリと権限
# (事前に pull --repos-users, --repos-teams, --team-users が必要)
ghub-desk view --user-repos user-login

# マスク済み設定値の確認
ghub-desk view --settings
```

`--format json` または `--format yaml` で出力形式を変更できます（デフォルト: `table`）。

## push — 組織データを変更

メンバー・チーム・コラボレーターの追加・削除を行います。**デフォルトは DRYRUN。**
`--exec` を付けると GitHub への変更が実行されます。

```bash
# チームにユーザーを追加（DRYRUN）
ghub-desk push add --team-user team-slug/username

# 追加を実行
ghub-desk push add --team-user team-slug/username --exec

# チームからユーザーを削除
ghub-desk push remove --team-user team-slug/username --exec

# 組織からユーザーを削除
ghub-desk push remove --user username --exec

# チームを組織から削除
ghub-desk push remove --team team-slug --exec

# リポジトリに外部コラボレーターを招待（DRYRUN）
ghub-desk push add --outside-user repo-name/username

# 権限を指定して招待
ghub-desk push add --outside-user repo-name/username --permission read

# 外部コラボレーターを削除して DB を同期
ghub-desk push remove --outside-user repo-name/username --exec

# リポジトリの協力者を削除
ghub-desk push remove --repos-user repo-name/username --exec
```

権限値: `pull` / `push` / `admin`（エイリアス: `read`→`pull`, `write`→`push`）

## auditlogs — 監査ログを取得

特定ユーザーの組織監査ログを取得します。

```bash
# デフォルト: 直近 30 日
ghub-desk auditlogs --user user-login

# 単一日付
ghub-desk auditlogs --user user-login --created 2025-01-01

# 日付範囲
ghub-desk auditlogs --user user-login --created "2025-01-01..2025-01-31"

# 指定日以降
ghub-desk auditlogs --user user-login --created ">=2025-01-01"

# リポジトリで絞り込み
ghub-desk auditlogs --user user-login --repo repo-name

# 1 ページの取得件数を指定（最大 100）
ghub-desk auditlogs --user user-login --per-page 50
```

## init — 設定とデータベースを初期化

```bash
# 既定パスに設定ファイルのひな形を作成
ghub-desk init config

# 任意パスに設定ファイルを作成
ghub-desk init config --target-file ~/ghub/config.yaml

# データベースを初期化（設定ファイルのパスを使用）
ghub-desk init db

# 明示的なパスにデータベースを初期化
ghub-desk init db --target-file ~/data/ghub-desk.db
```

## version — バージョン情報を表示

```bash
ghub-desk version
```

## 入力制約

| 種類 | 使用可能文字 | 長さ |
|---|---|---|
| ユーザー名 | 英数字、ハイフン（先頭・末尾不可） | 1〜39 |
| チームスラッグ | 小文字英数字、ハイフン（先頭・末尾不可） | 1〜100 |
| リポジトリ名 | 英数字、アンダースコア、ハイフン（先頭のハイフン不可） | 1〜100 |

- `--team-user` には `{team-slug}/{username}` 形式で指定
- `--outside-user` / `--repos-user` には `{repository}/{username}` 形式で指定

## グローバルフラグ

| フラグ | 説明 |
|---|---|
| `--debug` | 詳細ログを有効化（SQL クエリ、API リクエスト） |
| `--log-path <file>` | ログをファイルに追記 |
| `--config <file>` | カスタム設定ファイルを使用 |
