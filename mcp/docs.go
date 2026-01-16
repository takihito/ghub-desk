package mcp

import (
	"context"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	docsMIMEType    = "text/markdown"
	docsOverviewURI = "resource://ghub-desk/mcp-overview"
	docsToolsURI    = "resource://ghub-desk/mcp-tools"
	docsSafetyURI   = "resource://ghub-desk/mcp-safety"
)

type docsResource struct {
	uri, name, title, description, body string
}

var docsResources = []docsResource{
	{
		uri:         docsOverviewURI,
		name:        "mcp-overview",
		title:       "ghub-desk MCP Overview",
		description: "How the ghub-desk MCP server is configured and how to query it with resources/list.",
		body:        mcpOverviewMarkdown,
	},
	{
		uri:         docsToolsURI,
		name:        "mcp-tools",
		title:       "ghub-desk MCP Tools",
		description: "Reference for every published tool with sample JSON inputs and response hints.",
		body:        mcpToolsMarkdown,
	},
	{
		uri:         docsSafetyURI,
		name:        "mcp-safety",
		title:       "ghub-desk MCP Safety Notes",
		description: "Guidance for DRYRUN vs exec, allow_write usage, and local DB synchronization.",
		body:        mcpSafetyMarkdown,
	},
}

func registerDocsResources(srv *sdk.Server) {
	for _, res := range docsResources {
		srv.AddResource(&sdk.Resource{
			URI:         res.uri,
			Name:        res.name,
			Title:       res.title,
			Description: res.description,
			MIMEType:    docsMIMEType,
		}, staticMarkdownResource(res.uri, res.body))
	}
}

func staticMarkdownResource(uri, body string) sdk.ResourceHandler {
	return func(_ context.Context, req *sdk.ReadResourceRequest) (*sdk.ReadResourceResult, error) {
		if req != nil && req.Params != nil {
			target := req.Params.URI
			if idx := strings.IndexByte(target, '#'); idx >= 0 {
				target = target[:idx]
			}
			if target != "" && target != uri {
				return nil, sdk.ResourceNotFoundError(target)
			}
		}
		return &sdk.ReadResourceResult{
			Contents: []*sdk.ResourceContents{
				{
					URI:      uri,
					MIMEType: docsMIMEType,
					Text:     body,
				},
			},
		}, nil
	}
}

const mcpOverviewMarkdown = `# ghub-desk MCP Overview

Use this document when you need to reason about how the MCP server is configured before
calling tools or resources.

## Launch checklist
1. Provide organization plus either github_token or the GitHub App block inside ~/.config/ghub-desk/config.yaml.
2. Set mcp.allow_pull (enables pull_*) and mcp.allow_write (enables push_*).
3. Optionally point database_path or GHUB_DESK_DB_PATH to a writable location shared with the CLI.
4. Start the server with: ghub-desk mcp --debug --config /path/to/config.yaml.
5. In your MCP client, call resources/list to discover the resources below.

## Permissions and behavior
- allow_pull:false publishes health, view_*, and auditlogs.
- allow_pull:true adds pull_* tools. Use interval_seconds to throttle API calls.
- allow_write:true is required for any push_* tool. Leave it disabled unless you have reviewed the steps in resource://ghub-desk/mcp-safety.
- All tools reuse the SQLite database (ghub-desk.db by default). CLI and MCP share the same file.

## Resource catalog
| URI | Summary |
| --- | --- |
| resource://ghub-desk/mcp-overview | You are here: start-up order, config requirements, and DB notes. |
| resource://ghub-desk/mcp-tools | Usage for every tool plus JSON examples and response hints. |
| resource://ghub-desk/mcp-safety | Push-specific guardrails, DRYRUN vs exec, and post-run cleanup tips. |

Use anchors such as resource://ghub-desk/mcp-tools#view_team-user to deep-link to individual tools.

## Typical MCP workflow
1. Run tools/list and resources/list to understand what is available.
2. Use tools/call (view) or pull commands to populate SQLite.
3. Call view_* tools to inspect cached data before making write calls.
4. Execute push_* with exec:false for DRYRUN, then exec:true after review.

## Database handling
- Pull tools delete table rows inside a transaction before re-populating data.
- View tools respect the latest local snapshot; run a pull first if you need fresh data.
- Push tools call Sync helpers to keep SQLite consistent unless no_store:true is passed.
`

const mcpToolsMarkdown = `# ghub-desk MCP Tools

The tables below describe every published tool. Copy the sample JSON into tools/call requests (omit optional keys when not needed).

## view_* (read-only)
| Tool | Purpose | Sample Input | Response Hints |
| --- | --- | --- | --- |
| view_users | Cached organization members | {} | users[] with id, login, name, email, timestamps |
| view_detail-users | Same payload as view_users for now | {} | Identical schema to view_users |
| view_user | One cached user profile | {"user":"octocat"} | Returns user with timestamps; found=false when missing |
| view_user-teams | Teams for one user | {"user":"octocat"} | Lists team_slug, team_name, role |
| view_teams | Cached teams | {} | teams[] with slug, description, privacy, permission |
| view_repos | Cached repositories | {} | repositories[] with name, language, private, counters |
| view_team-user | Members of one team (slug) | {"team":"platform-team"} | users[] plus role, filter by slug |
| view_repos-users | Direct collaborators for one repo | {"repository":"admin-console"} | Includes permission and user_login |
| view_repos-teams | Teams mapped to a repo | {"repository":"admin-console"} | Shows team_slug, permission, timestamps |
| view_repos-teams-users | Team members linked to a repo | {"repository":"admin-console"} | Lists team_slug, team_permission, user_login, role, and profile fields |
| view_team-repos | Repositories for one team | {"team":"platform-team"} | Lists repo_name/full_name with permission |
| view_user-repos | Access map for one user | {"user":"octocat"} | Response lists repositories and how access is granted |
| view_outside-users | Outside collaborators snapshot | {} | Lists collaborators captured by pull_outside-users |
| view_token-permission | Token permission cache | {} | Latest PAT or GitHub App headers; errors when empty |
| view_settings | Masked configuration | {} | Confirms organization, DB path, and MCP flags |
| view_all-teams-users | Every cached team membership | {} | Returns team_slug, user_login, and role for all records |
| view_all-repos-users | All repository collaborators | {} | Each entry includes repository name, user login, and permission |
| view_all-repos-teams | All repository-team links | {} | Enumerates repository, team slug, permission, timestamps |

## pull_* (requires allow_pull)
| Tool | Purpose | Sample Input | Notes |
| --- | --- | --- | --- |
| pull_users | Fetch members (optionally with details) | {"detail":true,"stdout":false} | Set detail:true for detail-users; defaults save to SQLite |
| pull_detail-users | Fetch members with profile fields | {"no_store":false} | Heavier API variant; mirrors pull_users output to view_detail-users |
| pull_teams | Fetch teams | {"no_store":false} | Set stdout:true to mirror API payloads |
| pull_repositories | Fetch repositories | {} | Accepts interval_seconds to slow requests |
| pull_all-teams-users | Fetch every team membership | {} | Primarily used before view_all-teams-users |
| pull_all-repos-users | Fetch every repository collaborator | {} | Useful when you need the complete org-wide matrix |
| pull_all-repos-teams | Fetch every repository-team mapping | {} | Resets ghub_repos_teams before inserting new rows |
| pull_team-user | Fetch one team | {"team":"platform-team","no_store":false} | team must match slug rules (alnum plus hyphen) |
| pull_repos-users | Fetch repo collaborators | {"repository":"admin-console"} | Same name validation as view tools |
| pull_repos-teams | Fetch repo-team links | {"repository":"admin-console"} | Useful before push_remove team access |
| pull_outside-users | Fetch outside collaborators | {} | Populates view_outside-users |
| pull_token-permission | Fetch token headers | {} | Stores rate limit and scope headers for later inspection |

## auditlogs (always available)
| Tool | Purpose | Sample Input | Notes |
| --- | --- | --- | --- |
| auditlogs | Fetch audit log entries by actor | {"user":"octocat","created":">=2025-01-01","repo":"admin-console","per_page":100} | Returns entries[]; defaults to last 30 days; per_page max 100 |

Created filter formats:
- YYYY-MM-DD (single date)
- >=YYYY-MM-DD (on/after)
- <=YYYY-MM-DD (on/before)
- YYYY-MM-DD..YYYY-MM-DD (range, inclusive)

Optional repo filter expects an organization repository name; the MCP server expands it to org/repo.
This tool calls the GitHub API but is available even when allow_pull is false.

## push_* (requires allow_write)
| Tool | Purpose | Sample Input | Notes |
| --- | --- | --- | --- |
| push_add | Add team members or invite outside collaborators | {"team_user":"team-slug/login","permission":"push","exec":false} | Run once with exec:false, inspect message, then re-run with exec:true |
| push_remove | Remove teams, members, or collaborators | {"team_user":"team-slug/login","exec":false} | Only one target field may be set; no_store:true skips DB sync |

For safety guidance see resource://ghub-desk/mcp-safety.
`

const mcpSafetyMarkdown = `# ghub-desk MCP Safety Notes

## allow_write gating
- Keep allow_write:false unless you are actively rolling out membership changes.
- Review tools/list output after toggling the flag to confirm push tools are either hidden or visible as expected.

## DRYRUN vs exec
- exec:false (default) validates payloads, builds API requests, and returns a message describing the change. Use this to confirm slugs, usernames, and permissions.
- Only set exec:true after reviewing DRYRUN output and confirming the command is scoped correctly.
- The MCP server logs GitHub API errors verbosely when --debug is enabled; check stderr if a write fails.

## Local store after writes
- Successful push operations sync the SQLite database so future view calls stay consistent. Provide no_store:true only if you intentionally want to defer local updates (for example, when another process manages the DB).
- When skipping the sync, run the matching pull_* tool later to refresh the cache.

## Token and permission hygiene
- pull_token-permission records the scopes and rate limit headers seen in the last API response. Use it after enabling a new PAT or GitHub App to verify the server has the rights needed for push operations.

## Operational tips
1. Collect context with view_* first (team slugs, repo names, collaborators).
2. Perform sensitive actions in a tight transaction: DRYRUN -> exec -> pull_* to re-sync.
3. Consider enabling stdout:true during pulls to archive GitHub responses for audits.
4. If multiple agents share the same SQLite file, stagger push calls to avoid conflicting updates.
`
