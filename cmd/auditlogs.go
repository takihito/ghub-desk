package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ghub-desk/auditlog"
	"ghub-desk/ghubclient"
	"ghub-desk/store"

	gh "github.com/google/go-github/v84/github"
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

	client, err := ghubclient.InitClient(cfg)
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
		normalized, err := normalizeAuditEntriesForYAML(entries)
		if err != nil {
			return err
		}
		return store.PrintYAML(normalized)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// normalizeAuditEntriesForYAML round-trips entries through JSON before handing them to the
// YAML encoder. yaml.Marshal reflects over struct fields directly and ignores AuditEntry's
// custom MarshalJSON, which flattens go-github v84's catch-all AdditionalFields map (repo,
// actor_ip, event, team, ...) back onto the top level; without this round-trip those fields
// would nest under an "additionalfields" key in YAML output instead of staying top-level.
func normalizeAuditEntriesForYAML(entries []*gh.AuditEntry) ([]map[string]any, error) {
	out := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			return nil, fmt.Errorf("failed to normalize audit entry: %w", err)
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("failed to normalize audit entry: %w", err)
		}
		out = append(out, m)
	}
	return out, nil
}

func printAuditLogTable(entries []*gh.AuditEntry) {
	store.PrintTableHeader("Timestamp", "Action", "Actor", "Repo", "User", "IP")

	for _, entry := range entries {
		fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\n",
			formatAuditLogTimestamp(entry),
			entry.GetAction(),
			entry.GetActor(),
			auditlog.RepoFromEntry(entry),
			auditlog.UserFromEntry(entry),
			auditlog.StringField(entry, "actor_ip"),
		)
	}
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
