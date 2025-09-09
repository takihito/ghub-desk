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

### データ削除 (push)
- TODO: GitHub上のユーザー・チーム・リポジトリの削除

## 技術

- **REST API**: GitHub APIの利用対応
- **ローカルデータベース**: SQLitewを使用したオフライン状況でのデータ管理

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
```

## ビルド

```bash
make build
```

## 対応プラットフォーム

- Go 1.23+
- macOS, Linux, Windows
