# GitHub Pages セットアップタスク

takihito/glasp ( https://takihito.github.io/glasp/ ) と同じ見た目で  
`https://takihito.github.io/ghub-desk/` を公開する。

**採用構成:** `gh-pages` ブランチのルート (`/`) を公開対象とする。  
glasp は `main` ブランチの `/docs` フォルダを使用しているが、本プロジェクトでは  
ページコンテンツを完全に分離するため専用ブランチ方式を採用する。

**作業ブランチ:** `feature/github-pages`（コンテンツ制作用）→ 最終的に `gh-pages` ブランチへ

テーマ: `pages-themes/slate@v0.2.0`

---

## Phase 1: ページコンテンツの作成（feature/github-pages ブランチ）

### 1-1. `_config.yml` を作成（Jekyll 設定）

```yaml
title: ghub-desk
description: GitHub Organization Management CLI & MCP Server written in Go.
remote_theme: pages-themes/slate@v0.2.0
plugins:
  - jekyll-remote-theme

url: https://takihito.github.io
baseurl: /ghub-desk

nav:
  - title: Home
    url: /ghub-desk/
  - title: Installation
    url: /ghub-desk/installation
  - title: Usage
    url: /ghub-desk/usage
  - title: MCP Server
    url: /ghub-desk/mcp-server
  - title: 日本語
    url: /ghub-desk/ja/
  - title: GitHub
    url: https://github.com/takihito/ghub-desk

defaults:
  - scope:
      path: "*"
    values:
      layout: default
      render_with_liquid: false
```

### 1-2. `index.md` を作成（英語ホームページ）
内容の参考: `README.md` の Overview・Features・Quick Start・Core Commands を整形。  
フロントマター: `layout: default`, `title: ghub-desk`, `description: ...`

### 1-3. `installation.md` を作成（インストール手順）
内容の参考: `README.md` の Installation セクション。  
- go install
- Download release archives (curl)
- Build from source
- Configuration（config.yaml の場所と最小設定例）

### 1-4. `usage.md` を作成（コマンドリファレンス）
内容の参考: `README.md` の Core Commands セクション。  
- pull / view / push / auditlogs / init / version / mcp の各コマンド説明

### 1-5. `mcp-server.md` を作成（MCP サーバードキュメント）
既存の `docs/mcp.md` をベースに整形。  
フロントマターを追加してページとして機能させる。

### 1-6. `ja/` ディレクトリを作成（日本語ページ）
- [ ] `ja/index.md` — `README.ja.md` を基に作成
- [ ] `ja/installation.md` — インストール手順の日本語版
- [ ] `ja/usage.md` — コマンドリファレンスの日本語版
- [ ] `ja/mcp-server.md` — `docs/mcp.ja.md` を整形

### 1-7. `install.sh` を作成（Linux/macOS インストールスクリプト）
glasp の `install.sh` を参考に ghub-desk 用に書き換える。  
- REPO: `takihito/ghub-desk`
- バイナリ名: `ghub-desk`
- デフォルトインストール先: `~/.local/bin`

### 1-8. `install.ps1` を作成（Windows PowerShell インストールスクリプト）
glasp の `install.ps1` を参考に ghub-desk 用に書き換える。

---

## Phase 2: gh-pages ブランチの作成と GitHub Pages 有効化

### 2-1. `gh-pages` ブランチを orphan で作成し、コンテンツを配置
```bash
# gh-pages ブランチを履歴なしで作成
git checkout --orphan gh-pages
git rm -rf .
# feature/github-pages のコンテンツをコピーして commit
git push origin gh-pages
```

または GitHub Actions で feature/github-pages の内容を自動デプロイする方式も可。

### 2-2. GitHub Pages の有効化
**GitHub Web UI:**
1. https://github.com/takihito/ghub-desk/settings/pages を開く
2. Source: `Deploy from a branch`
3. Branch: `gh-pages` / フォルダ: `/`（ルート）
4. Save をクリック

**GitHub CLI:**
```bash
gh api repos/takihito/ghub-desk/pages \
  -X POST \
  -f build_type=legacy \
  -f source[branch]=gh-pages \
  -f "source[path]=/"
```

### 2-3. リポジトリの About に URL を設定
```bash
gh repo edit takihito/ghub-desk --homepage "https://takihito.github.io/ghub-desk/"
```

---

## Phase 3: 動作確認

- [ ] GitHub Actions の Pages ビルドジョブが成功することを確認
- [ ] https://takihito.github.io/ghub-desk/ にアクセスできることを確認
- [ ] ナビゲーションリンク（Installation / Usage / MCP Server / 日本語）が動作することを確認
- [ ] 日本語ページ https://takihito.github.io/ghub-desk/ja/ が表示されることを確認
- [ ] `install.sh` のダウンロードリンクが機能することを確認

---

## ファイル構成（gh-pages ブランチ完成後）

```
/（gh-pages ブランチルート）
├── _config.yml
├── index.md
├── installation.md
├── usage.md
├── mcp-server.md
├── install.sh
├── install.ps1
└── ja/
    ├── index.md
    ├── installation.md
    ├── usage.md
    └── mcp-server.md
```

## 注意事項
- `gh-pages` ブランチは orphan（履歴なし）で作成するのが慣例。main の履歴を含まない
- `_config.yml` の `remote_theme` を使うため、Jekyll のローカルビルドは不要
- `render_with_liquid: false` はコードブロック内の `{{ }}` を誤処理しないための設定
- `main` ブランチの `docs/` は Pages とは無関係のまま維持される（チャットログ等のリスクなし）
