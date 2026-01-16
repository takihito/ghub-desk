package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"ghub-desk/auditlog"
	ghubgithub "ghub-desk/github"
	"ghub-desk/store"

	gh "github.com/google/go-github/v55/github"
)

// AuditLogsCmd fetches and displays organization audit log entries.
type AuditLogsCmd struct {
	User    string `name:"user" help:"Actor login to filter audit log entries."`
	Repo    string `name:"repo" help:"Repository name (within the organization) to filter audit log entries."`
	Created string `name:"created" help:"Created filter: YYYY-MM-DD, >=YYYY-MM-DD, <=YYYY-MM-DD, or YYYY-MM-DD..YYYY-MM-DD (default: last 30 days)."`
	PerPage int    `name:"per-page" default:"100" help:"Number of entries per page (max 100)."`
	Format  string `name:"format" default:"table" help:"Output format (table|json|yaml)"`
}

// Run implements the auditlogs command execution.
func (a *AuditLogsCmd) Run(cli *CLI) error {
	user := strings.TrimSpace(a.User)
	if user == "" {
		return fmt.Errorf("--user is required")
	}
	if err := validateUserLogin(user); err != nil {
		return err
	}

	repo := strings.TrimSpace(a.Repo)
	if repo != "" {
		if err := validateRepoName(repo); err != nil {
			return err
		}
	}
	if a.PerPage <= 0 {
		return fmt.Errorf("--per-page must be a positive integer")
	}
	if a.PerPage > 100 {
		return fmt.Errorf("--per-page must be 100 or less")
	}

	cfg, err := cli.Config()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	client, err := ghubgithub.InitClient(cfg)
	if err != nil {
		return fmt.Errorf("github client initialization error: %w", err)
	}

	phrase, err := auditlog.BuildPhrase(cfg.Organization, user, repo, a.Created, time.Now())
	if err != nil {
		return err
	}
	cli.debugf("DEBUG: AuditLogs phrase=%q\n", phrase)

	opts := &gh.GetAuditLogOptions{
		Phrase: gh.String(phrase),
		ListCursorOptions: gh.ListCursorOptions{
			PerPage: a.PerPage,
		},
	}

	entries, err := auditlog.FetchEntries(context.Background(), client, cfg.Organization, opts)
	if err != nil {
		return fmt.Errorf("failed to fetch audit logs: %w", err)
	}

	return renderAuditLogEntries(entries, a.Format)
}

func renderAuditLogEntries(entries []*gh.AuditEntry, format string) error {
	parsedFormat, err := store.ParseOutputFormat(format)
	if err != nil {
		return err
	}

	switch parsedFormat {
	case store.FormatTable:
		if len(entries) == 0 {
			fmt.Println("No audit log entries found.")
			return nil
		}
		printAuditLogTable(entries)
		return nil
	case store.FormatJSON:
		return store.PrintJSON(entries)
	case store.FormatYAML:
		return store.PrintYAML(entries)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func printAuditLogTable(entries []*gh.AuditEntry) {
	printAuditLogTableHeader("Timestamp", "Action", "Actor", "Repo", "User", "IP")

	for _, entry := range entries {
		fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\n",
			formatAuditLogTimestamp(entry),
			entry.GetAction(),
			entry.GetActor(),
			auditlog.RepoFromEntry(entry),
			auditlog.UserFromEntry(entry),
			entry.GetActorIP(),
		)
	}
}

func printAuditLogTableHeader(columns ...string) {
	if len(columns) == 0 {
		return
	}
	fmt.Println(strings.Join(columns, "\t"))

	under := make([]string, len(columns))
	for i, col := range columns {
		width := utf8.RuneCountInString(col)
		if width <= 0 {
			width = 1
		}
		under[i] = strings.Repeat("-", width)
	}
	fmt.Println(strings.Join(under, "\t"))
}

func formatAuditLogTimestamp(entry *gh.AuditEntry) string {
	if entry == nil {
		return ""
	}
	if ts := entry.GetTimestamp(); !ts.IsZero() {
		return ts.Time.UTC().Format(time.RFC3339)
	}
	if created := entry.GetCreatedAt(); !created.IsZero() {
		return created.Time.UTC().Format(time.RFC3339)
	}
	return ""
}
