GitHubメンバー操作ツール


## 概要

GitHubオーガニゼーション上でメンバーやチームを操作するツールになります

## 機能

コマンドライン(CLI)で実行します

以下の機能を有します

### 組織情報の取得

以下の情報をGitHubAPI経由で取得します。取得した情報はSQLiteとして保存管理されます。

情報を取得する際には進捗を表示してください。

````
- 100件取得しました
- 200件取得しました
- 200件取得しました
...組織XXXのチーム一覧を得中です
````


* ユーザー一覧の取得
* チーム一覧の取得
* チームに所属するユーザーの一覧の取得
* リポジトリ一覧の取得

### 組織の変更

"組織情報の取得”で取得しSQLiteで保存された情報をもとに組織情報をGitHubAPI経由で更新します。

* チームを組織から削除
* ユーザーを組織から削除
* ユーザーをチームから削除

## 使い方

環境変数で組織名とトークンを指定します

* GHUB_DESK_ORGANIZATION
* GHUB_DESK_GITHUB_TOKEN


### 使用例

* ユーザー一覧の取得

````
# SQLiteには保存する
$ ghub-desk pull --store --users

# SQLiteには保存せず
$ ghub-desk pull --users

# SQLiteをソースとして表示する 
$ ghub-desk view --users
````

* チーム一覧の取得

````
# SQLiteには保存する
$ ghub-desk pull --store --teams

# SQLiteには保存せず
$ ghub-desk pull --teams

# SQLiteをソースとして表示する 
$ ghub-desk view --teams 
````

* チームに所属するユーザーの一覧の取得

````
# SQLiteには保存する
$ ghub-desk pull --store --teams-users {team_name}

# SQLiteには保存せず
$ ghub-desk pull --teams-users {team_name}

# SQLiteをソースとして表示する 
$ ghub-desk view --teams-users {team_name}
````

````
# SQLiteに保存したチーム一覧を元に、チームユーザに所属するユーザ一覧を取得する
# SQLiteには保存する
$ ghub-desk pull --store --all-teams-users 

# SQLiteには保存せず
$ ghub-desk pull --all-teams-users 

# SQLiteに保存されるとviewで確認することができます
$ ghub-desk view {team_name}/users
````


* リポジトリ一覧の取得

````
# SQLiteには保存する
$ ghub-desk pull --store --repos 

# SQLiteには保存せず
$ ghub-desk pull --repos

# SQLiteをソースとして表示する 
$ ghub-desk view --repos
````



* チームを組織から削除

````
$ ghub-desk push --remove --team {team_name} --exec

# DRYRUN
$ ghub-desk push --remove --team {team_name}
````

* ユーザーを組織から削除

````
$ ghub-desk push --remove --user {user_name} --exec

# DRYRUN
$ ghub-desk push --remove --user {user_name}
````

* ユーザーをチームから削除

````
$ ghub-desk push --remove --team-user {team_name}/{user_name} --exec

# DRYRUN
$ ghub-desk push --remove --team-user  {team_name}/{user_name}
````

* TOKENの権限チェック

````
# GITHUB API で使用しているトークンの権限を表示します
$ ghub-desk pull --token-permission

# SQLiteに保存します
$ ghub-desk pull --token-permission --store

#SQLiteに保存したトークンの権限情報を表示します
$ ghub-desk view --token-permission
````



## 修正指示

### 1.重複したコードをまとめてください

APIを実行しDBに保存するコードが各所にあります。
このコードを一つの関数にまとめてほしいです。関数には以下を渡す形式で再利用できるのが望ましいです。

* client.Organization
* db.execにわたすSQLと値
 

````
		for {
			users, resp, err := client.Organizations.ListMembers(ctx, org, opt)
			if err != nil {
				fmt.Fprintf(os.Stderr, "GitHub API error: %v\n", err)
				os.Exit(1)
			}
			count += len(users)
			fmt.Printf("- %d件取得しました\n", count)
			if *store {
				for _, u := range users {
					_, _ = db.Exec(`INSERT INTO users(id, login, name) VALUES (?, ?, ?)`, u.GetID(), u.GetLogin(), u.GetName())
				}
			}
			if resp.NextPage == 0 {
				break
			}
			opt.Page = resp.NextPage
			time.Sleep(sleepSec)
		}
````


------

### 今後の差別化提案


「組織管理ダッシュボード」としてのポジション
# 組織の健康状態チェック
ghub-desk analytics --teams-activity
ghub-desk report --user-distribution


「データ分析・レポート機能」
# 時系列分析
ghub-desk analyze --team-growth --period 6months
ghub-desk export --format csv --teams-users




「コンプライアンス・監査機能」

# アクセス権監査
ghub-desk audit --inactive-users --days 90
ghub-desk compliance --team-permissions


推奨される方向性

A. 組織管理特化ツールとして発展
# 管理者向け機能
ghub-desk dashboard          # 組織概要表示
ghub-desk alerts --inactive  # 非アクティブユーザー検出
ghub-desk sync --with-ldap   # 外部システムとの同期

データ分析・BI機能
# 分析機能
ghub-desk metrics --team-activity
ghub-desk visualize --user-contributions
ghub-desk benchmark --against-industry

C. 自動化・CI/CD連携
# 自動化
ghub-desk schedule --daily-sync
ghub-desk webhook --team-changes
ghub-desk integration --slack-notifications
