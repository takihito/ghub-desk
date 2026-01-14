package cmd

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	ghubgithub "ghub-desk/github"
	"ghub-desk/store"

	gh "github.com/google/go-github/v55/github"
)

var (
	auditLogDateRE  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	auditLogCmpRE   = regexp.MustCompile(`^(>=|<=)\d{4}-\d{2}-\d{2}$`)
	auditLogRangeRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\.\.\d{4}-\d{2}-\d{2}$`)
)

// AuditLogsCmd fetches and displays organization audit log entries.
type AuditLogsCmd struct {
	User    string `name:"user" help:"Actor login to filter audit log entries."`
	Repo    string `name:"repo" help:"Repository name (within the organization) to filter audit log entries."`
	Created string `name:"created" help:"Created filter: YYYY-MM-DD, >=YYYY-MM-DD, <=YYYY-MM-DD, or YYYY-MM-DD..YYYY-MM-DD (default: last 30 days)."`
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

	cfg, err := cli.Config()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	client, err := ghubgithub.InitClient(cfg)
	if err != nil {
		return fmt.Errorf("github client initialization error: %w", err)
	}

	phrase, err := buildAuditLogPhrase(cfg.Organization, user, repo, a.Created, time.Now())
	if err != nil {
		return err
	}
	cli.debugf("DEBUG: AuditLogs phrase=%q\n", phrase)

	opts := &gh.GetAuditLogOptions{
		Phrase: gh.String(phrase),
	}

	entries, _, err := client.Organizations.GetAuditLog(context.Background(), cfg.Organization, opts)
	if err != nil {
		return fmt.Errorf("failed to fetch audit logs: %w", err)
	}

	return renderAuditLogEntries(entries, a.Format)
}

func buildAuditLogPhrase(org, user, repo, created string, now time.Time) (string, error) {
	if strings.TrimSpace(user) == "" {
		return "", fmt.Errorf("--user is required")
	}

	createdClause, err := buildAuditLogCreatedClause(created, now)
	if err != nil {
		return "", err
	}

	parts := []string{fmt.Sprintf("actor:%s", user)}
	if repo != "" {
		if org == "" {
			return "", fmt.Errorf("organization is required to filter by repository")
		}
		parts = append(parts, fmt.Sprintf("repo:%s/%s", org, repo))
	}
	parts = append(parts, createdClause)

	return strings.Join(parts, " "), nil
}

func buildAuditLogCreatedClause(raw string, now time.Time) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		date := now.UTC().AddDate(0, 0, -30).Format("2006-01-02")
		return "created:>=" + date, nil
	}

	trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "created:"))
	if trimmed == "" {
		return "", fmt.Errorf("invalid --created format: %q", raw)
	}

	switch {
	case auditLogDateRE.MatchString(trimmed):
		if _, err := time.Parse("2006-01-02", trimmed); err != nil {
			return "", fmt.Errorf("invalid --created date: %w", err)
		}
	case auditLogCmpRE.MatchString(trimmed):
		if _, err := time.Parse("2006-01-02", trimmed[2:]); err != nil {
			return "", fmt.Errorf("invalid --created date: %w", err)
		}
	case auditLogRangeRE.MatchString(trimmed):
		parts := strings.Split(trimmed, "..")
		start, err := time.Parse("2006-01-02", parts[0])
		if err != nil {
			return "", fmt.Errorf("invalid --created date: %w", err)
		}
		end, err := time.Parse("2006-01-02", parts[1])
		if err != nil {
			return "", fmt.Errorf("invalid --created date: %w", err)
		}
		if end.Before(start) {
			return "", fmt.Errorf("invalid --created range: end date is before start date")
		}
	default:
		return "", fmt.Errorf("invalid --created format: %q (use YYYY-MM-DD, >=YYYY-MM-DD, <=YYYY-MM-DD, or YYYY-MM-DD..YYYY-MM-DD)", raw)
	}

	return "created:" + trimmed, nil
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
			auditLogRepo(entry),
			auditLogUser(entry),
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

func auditLogRepo(entry *gh.AuditEntry) string {
	if entry == nil {
		return ""
	}
	if repo := strings.TrimSpace(entry.GetRepo()); repo != "" {
		return repo
	}
	if repo := strings.TrimSpace(entry.GetRepository()); repo != "" {
		return repo
	}
	return ""
}

func auditLogUser(entry *gh.AuditEntry) string {
	if entry == nil {
		return ""
	}
	if user := strings.TrimSpace(entry.GetUser()); user != "" {
		return user
	}
	if user := strings.TrimSpace(entry.GetTargetLogin()); user != "" {
		return user
	}
	return ""
}
