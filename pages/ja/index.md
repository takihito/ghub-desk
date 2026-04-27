---
layout: default
title: ghub-desk - GitHub 組織管理 CLI & MCP サーバー
description: ghub-desk は GitHub 組織のメンバー・チーム・リポジトリを管理する Go 製 CLI ツール & MCP サーバーです。
---

# ghub-desk

[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/11852/badge)](https://www.bestpractices.dev/projects/11852)

`ghub-desk` は GitHub 組織のメンバー・チーム・リポジトリを管理するための Go 製 CLI ツールです。
GitHub API 経由で組織情報を取得し、SQLite にキャッシュしてオフライン参照を実現します。
MCP サーバーとして起動することで、LLM や AI エージェントから同じ機能を安全に呼び出すことができます。

[English Documentation](../)

## 特徴

- **pull / view / push** — GitHub から組織データを取得・表示・安全に変更
- **デフォルト DRYRUN** — 変更系コマンドは `--exec` を指定しない限り実際には何もしない
- **SQLite キャッシュ** — オフラインでの組織スナップショット参照と長時間 pull の再開に対応
- **MCP サーバー** — CLI の全機能を stdio 経由で AI エージェントに公開
- **デュアル認証** — PAT または GitHub App（どちらか一方を選択）
- **シングルバイナリ** — ダウンロードして実行するだけ。ランタイム依存なし

## クイックスタート

```bash
# インストール (Linux / macOS)
curl -sSL https://takihito.github.io/ghub-desk/install.sh | sh

# 設定ファイルのひな形を作成
ghub-desk init config
# ~/.ghub-desk/config.yaml を編集して組織名とトークンを設定

# データベースを初期化
ghub-desk init db

# 組織メンバーを取得
ghub-desk pull --users

# 保存されたデータを表示
ghub-desk view --users
```

> **Windows:** `irm https://takihito.github.io/ghub-desk/install.ps1 | iex`

詳細は [インストール](installation)・[使い方](usage)・[MCP サーバー](mcp-server) をご覧ください。

## 安全設計

変更系コマンド（`push add`, `push remove`）は**デフォルトで DRYRUN**として動作します。
`--exec` を付けると初めて GitHub への変更が実行されます。成功時はローカル SQLite キャッシュも自動更新されます。

```bash
# 実行内容のプレビュー（DRYRUN）
ghub-desk push add --team-user my-team/octocat

# 変更を適用
ghub-desk push add --team-user my-team/octocat --exec
```

## 技術スタック

- **言語:** Go 1.26.1
- **データベース:** SQLite（オフラインキャッシュ）
- **MCP:** [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk)
- **対応プラットフォーム:** macOS, Linux, Windows
