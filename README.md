# ghub-desk

GitHub Organization Management CLI Tool

## 概要

`ghub-desk`は、GitHub組織を管理するためのマンドラインツールです。組織のメンバー、チーム、リポジトリ情報を取得・管理し、ローカルのデータベース(SQLite)に保存し管理をすることができます。

## 主な機能

### データ取得 (pull)
- **ユーザー情報取得**: 組織メンバーの基本情報・詳細情報
- **チーム情報取得**: 組織内チームの一覧・メンバー情報
- **リポジトリ情報取得**: 組織内リポジトリの一覧

### データ表示 (view)
- 取得したユーザー、チーム、リポジトリ情報の閲覧

### データ操作 (push)
- **削除操作**: GitHub上のユーザー・チーム・リポジトリの削除
- **追加操作**: チームへのユーザー追加

## 技術

- **REST API**: GitHub APIの利用対応
- **ローカルデータベース**: SQLiteを使用したオフライン状況でのデータ管理

## 環境変数

```bash
export GHUB_DESK_ORGANIZATION="your-org-name"      # GitHub組織名
export GHUB_DESK_GITHUB_TOKEN="your-token"         # GitHub Access Token
```

## 基本的な使用例

```bash
# 組織メンバーの基本情報を取得・保存
./ghub-desk pull --store --users

# 組織メンバーの詳細情報を取得・保存
./ghub-desk pull --store --detail-users

# 保存されたユーザー情報を表示
./ghub-desk view --users

# チーム情報を取得・表示
./ghub-desk pull --store --teams
./ghub-desk view --teams

# データベース初期化
./ghub-desk init

# チームにユーザーを追加（DRYRUN）
# team-name はチームの slug（小文字英数字とハイフン）を指定
./ghub-desk push add --team-user team-slug/username

# チームにユーザーを追加（実行）
./ghub-desk push add --team-user team-slug/username --exec

# チームからユーザーを削除（DRYRUN）
./ghub-desk push remove --team-user team-slug/username

# チームからユーザーを削除（実行）
./ghub-desk push remove --team-user team-slug/username --exec

## 入力制約（ユーザー名・チーム）

- ユーザー名
  - 許可: 英数字・ハイフンのみ
  - 先頭/末尾にハイフンは不可
  - 長さ: 1〜39 文字
- チーム名（slug）
  - API 指定は slug を使用します（表示名ではありません）
  - 許可: 小文字英数字・ハイフンのみ
  - 先頭/末尾にハイフンは不可
  - 長さ: 1〜100 文字
  - 形式: `{team-slug}/{username}` を `--team-user` に渡してください
```

### 設定の表示（マスク出力）

```bash
# 現在の設定をYAMLで表示（秘匿情報はマスクされます）
./ghub-desk view --settings

# 任意の設定ファイルを使う場合はグローバルの --config でパス指定
./ghub-desk --config ~/.config/ghub-desk/config.yaml view --settings
```

## ビルド

```bash
make build
```

##  テスト

```bash
make test
```

## 対応プラットフォーム

- Go 1.24+
- macOS, Linux, Windows
