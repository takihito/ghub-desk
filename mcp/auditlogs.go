package mcp

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	v "ghub-desk/validate"

	ghapi "github.com/google/go-github/v55/github"
)

var (
	auditLogDateRE  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	auditLogCmpRE   = regexp.MustCompile(`^(>=|<=)\d{4}-\d{2}-\d{2}$`)
	auditLogRangeRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\.\.\d{4}-\d{2}-\d{2}$`)
)

func buildAuditLogPhrase(org, user, repo, created string, now time.Time) (string, error) {
	user = strings.TrimSpace(user)
	if user == "" {
		return "", fmt.Errorf("user is required")
	}
	if err := v.ValidateUserName(user); err != nil {
		return "", err
	}

	repo = strings.TrimSpace(repo)
	if repo != "" {
		if err := v.ValidateRepoName(repo); err != nil {
			return "", err
		}
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
		return "", fmt.Errorf("invalid created format: %q", raw)
	}

	switch {
	case auditLogDateRE.MatchString(trimmed):
		if _, err := time.Parse("2006-01-02", trimmed); err != nil {
			return "", fmt.Errorf("invalid created date: %w", err)
		}
	case auditLogCmpRE.MatchString(trimmed):
		if _, err := time.Parse("2006-01-02", trimmed[2:]); err != nil {
			return "", fmt.Errorf("invalid created date: %w", err)
		}
	case auditLogRangeRE.MatchString(trimmed):
		parts := strings.Split(trimmed, "..")
		start, err := time.Parse("2006-01-02", parts[0])
		if err != nil {
			return "", fmt.Errorf("invalid created date: %w", err)
		}
		end, err := time.Parse("2006-01-02", parts[1])
		if err != nil {
			return "", fmt.Errorf("invalid created date: %w", err)
		}
		if end.Before(start) {
			return "", fmt.Errorf("invalid created range: end date is before start date")
		}
	default:
		return "", fmt.Errorf("invalid created format: %q (use YYYY-MM-DD, >=YYYY-MM-DD, <=YYYY-MM-DD, or YYYY-MM-DD..YYYY-MM-DD)", raw)
	}

	return "created:" + trimmed, nil
}

func fetchAuditLogEntries(ctx context.Context, client *ghapi.Client, org string, opts *ghapi.GetAuditLogOptions) ([]*ghapi.AuditEntry, error) {
	var allEntries []*ghapi.AuditEntry
	var lastAfter string

	for {
		entries, resp, err := client.Organizations.GetAuditLog(ctx, org, opts)
		if err != nil {
			return nil, err
		}
		allEntries = append(allEntries, entries...)

		nextAfter := ""
		if resp != nil {
			nextAfter = strings.TrimSpace(resp.After)
		}
		if nextAfter == "" {
			break
		}
		if nextAfter == lastAfter {
			return nil, fmt.Errorf("audit log pagination stalled: cursor did not advance")
		}
		lastAfter = nextAfter
		opts.After = nextAfter
	}

	return allEntries, nil
}

type AuditLogEntry struct {
	Action        string `json:"action,omitempty"`
	Actor         string `json:"actor,omitempty"`
	ActorIP       string `json:"actor_ip,omitempty"`
	User          string `json:"user,omitempty"`
	Repo          string `json:"repo,omitempty"`
	Org           string `json:"org,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
	Timestamp     string `json:"timestamp,omitempty"`
	DocumentID    string `json:"document_id,omitempty"`
	Event         string `json:"event,omitempty"`
	OperationType string `json:"operation_type,omitempty"`
	Permission    string `json:"permission,omitempty"`
	Team          string `json:"team,omitempty"`
	Message       string `json:"message,omitempty"`
	UserAgent     string `json:"user_agent,omitempty"`
}

func normalizeAuditLogEntries(entries []*ghapi.AuditEntry) []AuditLogEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]AuditLogEntry, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		out = append(out, AuditLogEntry{
			Action:        entry.GetAction(),
			Actor:         entry.GetActor(),
			ActorIP:       entry.GetActorIP(),
			User:          auditLogUser(entry),
			Repo:          auditLogRepo(entry),
			Org:           entry.GetOrg(),
			CreatedAt:     formatAuditLogTimestamp(entry.GetCreatedAt()),
			Timestamp:     formatAuditLogTimestamp(entry.GetTimestamp()),
			DocumentID:    entry.GetDocumentID(),
			Event:         entry.GetEvent(),
			OperationType: entry.GetOperationType(),
			Permission:    entry.GetPermission(),
			Team:          entry.GetTeam(),
			Message:       entry.GetMessage(),
			UserAgent:     entry.GetUserAgent(),
		})
	}
	return out
}

func auditLogRepo(entry *ghapi.AuditEntry) string {
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

func auditLogUser(entry *ghapi.AuditEntry) string {
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

func formatAuditLogTimestamp(ts ghapi.Timestamp) string {
	if ts.IsZero() {
		return ""
	}
	return ts.Time.UTC().Format(time.RFC3339Nano)
}
