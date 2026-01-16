package mcp

import (
	"time"

	"ghub-desk/auditlog"

	ghapi "github.com/google/go-github/v55/github"
)

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
			User:          auditlog.UserFromEntry(entry),
			Repo:          auditlog.RepoFromEntry(entry),
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

func formatAuditLogTimestamp(ts ghapi.Timestamp) string {
	if ts.IsZero() {
		return ""
	}
	return ts.Time.UTC().Format(time.RFC3339Nano)
}
