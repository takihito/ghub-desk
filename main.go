package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v55/github"
	"golang.org/x/oauth2"
	_ "modernc.org/sqlite"
)

const (
	envOrg         = "GHUB_DESK_ORGANIZATION"
	envGithubToken = "GHUB_DESK_GITHUB_TOKEN"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "pull":
		pullCmd(os.Args[2:])
	case "view":
		viewCmd(os.Args[2:])
	case "push":
		pushCmd(os.Args[2:])
	case "init":
		initTables()
		fmt.Println("DBテーブルを初期化しました")
		return
	case "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		usage()
		os.Exit(1)
	}
}

// DBテーブル初期化
func initTables() {
	db, err := sql.Open("sqlite", "ghub-desk.db")
	if err != nil {
		fmt.Fprintf(os.Stderr, "SQLiteオープン失敗: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, login TEXT, name TEXT)`,
		`CREATE TABLE IF NOT EXISTS teams (id INTEGER PRIMARY KEY, slug TEXT, name TEXT)`,
		`CREATE TABLE IF NOT EXISTS repos (id INTEGER PRIMARY KEY, name TEXT, full_name TEXT)`,
		`CREATE TABLE IF NOT EXISTS team_users (team_slug TEXT, user_id INTEGER, login TEXT, name TEXT, PRIMARY KEY(team_slug, user_id))`,
	}
	for _, stmt := range stmts {
		_, err := db.Exec(stmt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "テーブル初期化失敗: %v\n", err)
			os.Exit(1)
		}
	}
}

func usage() {
	fmt.Print(`ghub-desk CLI

Usage:
  ghub-desk pull [--store] <target>
  ghub-desk view <target>
  ghub-desk push remove [--exec] <target>

Targets:
  users, teams, {team_name}/users, repos, ...
`)
}

func getEnvVars() (string, string) {
	org := os.Getenv(envOrg)
	githubToken := os.Getenv(envGithubToken)
	if org == "" || githubToken == "" {
		fmt.Fprintln(os.Stderr, "環境変数 GHUB_DESK_ORGANIZATION, GHUB_DESK_GITHUB_TOKEN を設定してください")
		os.Exit(1)
	}
	return org, githubToken
}

func pullCmd(args []string) {
	fs := flag.NewFlagSet("pull", flag.ExitOnError)
	store := fs.Bool("store", false, "Save to SQLite")
	fs.Parse(args)
	targets := fs.Args()
	if len(targets) == 0 {
		fmt.Fprintln(os.Stderr, "pull対象を指定してください")
		os.Exit(1)
	}
	target := targets[0]
	org, githubToken := getEnvVars()
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: githubToken})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	var db *sql.DB
	var err error
	if *store {
		db, err = sql.Open("sqlite", "ghub-desk.db")
		if err != nil {
			fmt.Fprintf(os.Stderr, "SQLiteオープン失敗: %v\n", err)
			os.Exit(1)
		}
		defer db.Close()
	}

	sleepSec := 1 * time.Second
	switch {
	case target == "users":
		if *store {
			db.Exec(`DELETE FROM users`)
		}
		fetchAndStore(
			func(page int) ([]*github.User, *github.Response, error) {
				opt := &github.ListMembersOptions{ListOptions: github.ListOptions{PerPage: 100, Page: page}}
				return client.Organizations.ListMembers(ctx, org, opt)
			},
			func(db *sql.DB, u *github.User) error {
				_, err := db.Exec(`INSERT INTO users(id, login, name) VALUES (?, ?, ?)`, u.GetID(), u.GetLogin(), u.GetName())
				return err
			},
			db, *store, sleepSec,
			func(count int) { fmt.Printf("- %d件取得しました\n", count) },
		)
		fmt.Printf("...組織%sのユーザー一覧を取得完了\n", org)
	case target == "teams":
		if *store {
			db.Exec(`DELETE FROM teams`)
		}
		fetchAndStore(
			func(page int) ([]*github.Team, *github.Response, error) {
				opt := &github.ListOptions{PerPage: 100, Page: page}
				return client.Teams.ListTeams(ctx, org, opt)
			},
			func(db *sql.DB, t *github.Team) error {
				_, err := db.Exec(`INSERT INTO teams(id, slug, name) VALUES (?, ?, ?)`, t.GetID(), t.GetSlug(), t.GetName())
				return err
			},
			db, *store, sleepSec,
			func(count int) { fmt.Printf("- %d件取得しました\n", count) },
		)
		fmt.Printf("...組織%sのチーム一覧を取得完了\n", org)
	case strings.HasSuffix(target, "/users"):
		teamSlug := strings.TrimSuffix(target, "/users")
		if *store {
			_, _ = db.Exec(`DELETE FROM team_users WHERE team_slug = ?`, teamSlug)
		}
		fetchAndStore(
			func(page int) ([]*github.User, *github.Response, error) {
				opt := &github.TeamListTeamMembersOptions{ListOptions: github.ListOptions{PerPage: 100, Page: page}}
				return client.Teams.ListTeamMembersBySlug(ctx, org, teamSlug, opt)
			},
			func(db *sql.DB, u *github.User) error {
				_, err := db.Exec(`INSERT OR REPLACE INTO team_users(team_slug, user_id, login, name) VALUES (?, ?, ?, ?)`, teamSlug, u.GetID(), u.GetLogin(), u.GetName())
				return err
			},
			db, *store, sleepSec,
			func(count int) { fmt.Printf("- %d件取得しました\n", count) },
		)
		fmt.Printf("...チーム%sのユーザー一覧を取得完了\n", teamSlug)
	case target == "repos":
		if *store {
			db.Exec(`DELETE FROM repos`)
		}
		fetchAndStore(
			func(page int) ([]*github.Repository, *github.Response, error) {
				opt := &github.RepositoryListByOrgOptions{ListOptions: github.ListOptions{PerPage: 100, Page: page}}
				return client.Repositories.ListByOrg(ctx, org, opt)
			},
			func(db *sql.DB, r *github.Repository) error {
				_, err := db.Exec(`INSERT INTO repos(id, name, full_name) VALUES (?, ?, ?)`, r.GetID(), r.GetName(), r.GetFullName())
				return err
			},
			db, *store, sleepSec,
			func(count int) { fmt.Printf("- %d件取得しました\n", count) },
		)
		fmt.Printf("...組織%sのリポジトリ一覧を取得完了\n", org)
	default:
		fmt.Fprintf(os.Stderr, "未対応のpull対象: %s\n", target)
		os.Exit(1)
	}
	if *store {
		fmt.Println("(SQLiteに保存します)")
	}
}

func viewCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "view対象を指定してください")
		os.Exit(1)
	}
	target := args[0]
	db, err := sql.Open("sqlite", "ghub-desk.db")
	if err != nil {
		fmt.Fprintf(os.Stderr, "SQLiteオープン失敗: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	switch {
	case target == "users":
		rows, err := db.Query(`SELECT id, login, name FROM users`)
		if err != nil {
			fmt.Fprintf(os.Stderr, "usersテーブル取得失敗: %v\n", err)
			os.Exit(1)
		}
		defer rows.Close()
		fmt.Println("id\tlogin\tname")
		for rows.Next() {
			var id int64
			var login, name string
			rows.Scan(&id, &login, &name)
			fmt.Printf("%d\t%s\t%s\n", id, login, name)
		}
	case target == "teams":
		rows, err := db.Query(`SELECT id, slug, name FROM teams`)
		if err != nil {
			fmt.Fprintf(os.Stderr, "teamsテーブル取得失敗: %v\n", err)
			os.Exit(1)
		}
		defer rows.Close()
		fmt.Println("id\tslug\tname")
		for rows.Next() {
			var id int64
			var slug, name string
			rows.Scan(&id, &slug, &name)
			fmt.Printf("%d\t%s\t%s\n", id, slug, name)
		}
	case target == "repos":
		rows, err := db.Query(`SELECT id, name, full_name FROM repos`)
		if err != nil {
			fmt.Fprintf(os.Stderr, "reposテーブル取得失敗: %v\n", err)
			os.Exit(1)
		}
		defer rows.Close()
		fmt.Println("id\tname\tfull_name")
		for rows.Next() {
			var id int64
			var name, fullName string
			rows.Scan(&id, &name, &fullName)
			fmt.Printf("%d\t%s\t%s\n", id, name, fullName)
		}
	case strings.HasSuffix(target, "/users"):
		teamSlug := strings.TrimSuffix(target, "/users")
		rows, err := db.Query(`SELECT user_id, login, name FROM team_users WHERE team_slug = ?`, teamSlug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "team_usersテーブル取得失敗: %v\n", err)
			os.Exit(1)
		}
		defer rows.Close()
		fmt.Println("user_id\tlogin\tname")
		for rows.Next() {
			var userID int64
			var login, name string
			rows.Scan(&userID, &login, &name)
			fmt.Printf("%d\t%s\t%s\n", userID, login, name)
		}
	default:
		fmt.Fprintf(os.Stderr, "未対応のview対象: %s\n", target)
		os.Exit(1)
	}
}

func pushCmd(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "pushサブコマンドを指定してください")
		os.Exit(1)
	}
	sub := args[0]
	switch sub {
	case "remove":
		pushRemoveCmd(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown push subcommand: %s\n", sub)
		os.Exit(1)
	}
}

func pushRemoveCmd(args []string) {
	fs := flag.NewFlagSet("remove", flag.ExitOnError)
	exec := fs.Bool("exec", false, "実行(DRYRUNでない)")
	fs.Parse(args)
	targets := fs.Args()
	if len(targets) == 0 {
		fmt.Fprintln(os.Stderr, "remove対象を指定してください")
		os.Exit(1)
	}
	target := targets[0]
	if *exec {
		fmt.Printf("%s を削除します (実行)\n", target)
	} else {
		fmt.Printf("%s を削除します (DRYRUN)\n", target)
	}
	// TODO: GitHub API呼び出し
}

// APIページングしつつDB保存も行う共通関数
func fetchAndStore[T any](
	fetchPage func(page int) ([]T, *github.Response, error),
	storeRow func(db *sql.DB, row T) error,
	db *sql.DB,
	store bool,
	sleepSec time.Duration,
	progressMsg func(count int),
) {
	page := 1
	count := 0
	for {
		items, resp, err := fetchPage(page)
		if err != nil {
			fmt.Fprintf(os.Stderr, "GitHub API error: %v\n", err)
			os.Exit(1)
		}
		count += len(items)
		progressMsg(count)
		if store && db != nil {
			for _, item := range items {
				_ = storeRow(db, item)
			}
		}
		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
		time.Sleep(sleepSec)
	}
}
