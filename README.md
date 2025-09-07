# ghub-desk

GitHub Organization Management CLI Tool

## 概要

`ghub-desk`は、GitHub組織の管理を効率化するコマンドラインツールです。組織のメンバー、チーム、リポジトリ情報を取得・管理し、ローカルのSQLiteデータベースに保存して分析や管理作業を支援します。

## 主な機能

### データ取得 (pull)
- **ユーザー情報取得**: 組織メンバーの基本情報・詳細情報
- **チーム情報取得**: 組織内チームの一覧・メンバー情報
- **リポジトリ情報取得**: 組織内リポジトリの一覧・詳細

### データ表示 (view)
- SQLiteデータベースに保存された情報をテーブル形式で表示
- 取得したユーザー、チーム、リポジトリ情報の閲覧

### データ削除 (push)
- GitHub上のユーザー・チーム・リポジトリの削除
- DRYRUNモードによる安全な事前確認

## 技術特徴

- **REST API**: GitHub APIの利用対応
- **SQLite統合**: ローカルデータベースによる効率的なデータ管理
- **モジュラー設計**: パッケージ分割による保守性の向上
- **レート制限対応**: GitHub API制限を考慮した安全な実行

## 環境変数

```bash
export GHUB_DESK_ORGANIZATION="your-org-name"      # GitHub組織名
export GHUB_DESK_GITHUB_TOKEN="your-token"        # GitHub Personal Access Token
```

## 基本的な使用例

```bash
# 組織メンバーの基本情報を取得・保存
./ghub-desk pull --store --users

# 組織メンバーの詳細情報を取得・保存 (GraphQL使用)
./ghub-desk pull --store --detail-users

# 保存されたユーザー情報を表示
./ghub-desk view --users

# チーム情報を取得・表示
./ghub-desk pull --store --teams
./ghub-desk view --teams

# データベース初期化
./ghub-desk init
```

## ビルド

```bash
make build
```

## 対応プラットフォーム

- Go 1.23+
- macOS, Linux, Windows
- GitHub Enterprise Server対応