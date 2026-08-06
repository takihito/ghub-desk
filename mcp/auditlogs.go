package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ghub-desk/auditlog"
	appcfg "ghub-desk/config"
	"ghub-desk/ghubclient"
	v "ghub-desk/validate"

	ghapi "github.com/google/go-github/v55/github"
	"github.com/google/jsonschema-go/jsonschema"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// auditLogsToolDef is registered alongside coreViewToolDefs: it's read-only against GitHub
// and always available regardless of mcp.allow_pull/allow_write.
var auditLogsToolDef = toolDef{name: "auditlogs", tier: tierCore, register: registerAuditLogsTool}

type AuditLogsIn struct {
	User    string `json:"user"`
	Repo    string `json:"repo,omitempty"`
	Created string `json:"created,omitempty"`
	PerPage int    `json:"per_page,omitempty"`
}

type AuditLogsOut struct {
	Count   int             `json:"count"`
	Entries []AuditLogEntry `json:"entries"`
}

func registerAuditLogsTool(srv *sdk.Server, name string, cfg *appcfg.Config) {
	sdk.AddTool[AuditLogsIn, AuditLogsOut](srv, &sdk.Tool{
		Name:        name,
		Title:       "Audit Logs",
		Description: "Fetch organization audit log entries by actor. Usage: " + docsToolsURI + "#auditlogs.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"user": {
					Type:        "string",
					Title:       "User Login",
					Description: "GitHub username (1-39 chars, alnum or hyphen).",
					MinLength:   intPtr(v.UserNameMin),
					MaxLength:   intPtr(v.UserNameMax),
					Pattern:     v.UserNamePattern,
				},
				"repo": {
					Type:        "string",
					Title:       "Repository Name",
					Description: "Optional repository name (1-100 chars, alnum/underscore/hyphen).",
					MinLength:   intPtr(v.RepoNameMin),
					MaxLength:   intPtr(v.RepoNameMax),
					Pattern:     v.RepoNamePattern,
				},
				"created": {
					Type:        "string",
					Title:       "Created Filter",
					Description: "Date filter (YYYY-MM-DD, >=YYYY-MM-DD, <=YYYY-MM-DD, or YYYY-MM-DD..YYYY-MM-DD). Defaults to last 30 days.",
				},
				"per_page": {
					Type:        "integer",
					Title:       "Per Page",
					Description: "Entries per page (max 100). Default is 100.",
					Minimum:     floatPtr(1),
					Maximum:     floatPtr(100),
				},
			},
			Required: []string{"user"},
		},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in AuditLogsIn) (*sdk.CallToolResult, AuditLogsOut, error) {
		perPage := in.PerPage
		if perPage == 0 {
			perPage = 100
		}
		if perPage < 0 {
			return &sdk.CallToolResult{}, AuditLogsOut{}, fmt.Errorf("per_page must be positive")
		}
		if perPage > 100 {
			return &sdk.CallToolResult{}, AuditLogsOut{}, fmt.Errorf("per_page must be 100 or less")
		}

		user := strings.TrimSpace(in.User)
		if user == "" {
			return &sdk.CallToolResult{}, AuditLogsOut{}, fmt.Errorf("user is required")
		}
		if err := v.ValidateUserName(user); err != nil {
			return &sdk.CallToolResult{}, AuditLogsOut{}, err
		}
		repo := strings.TrimSpace(in.Repo)
		if repo != "" {
			if err := v.ValidateRepoName(repo); err != nil {
				return &sdk.CallToolResult{}, AuditLogsOut{}, err
			}
		}

		phrase, err := auditlog.BuildPhrase(cfg.Organization, user, repo, in.Created, time.Now())
		if err != nil {
			return &sdk.CallToolResult{}, AuditLogsOut{}, err
		}
		client, err := ghubclient.InitClient(cfg)
		if err != nil {
			return &sdk.CallToolResult{}, AuditLogsOut{}, fmt.Errorf("github client init: %w", err)
		}
		opts := &ghapi.GetAuditLogOptions{
			Phrase: ghapi.String(phrase),
			ListCursorOptions: ghapi.ListCursorOptions{
				PerPage: perPage,
			},
		}
		entries, err := auditlog.FetchEntries(ctx, client, cfg.Organization, opts)
		if err != nil {
			return &sdk.CallToolResult{}, AuditLogsOut{}, fmt.Errorf("failed to fetch audit logs: %w", err)
		}
		normalized := normalizeAuditLogEntries(entries)
		return nil, AuditLogsOut{Count: len(normalized), Entries: normalized}, nil
	})
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
