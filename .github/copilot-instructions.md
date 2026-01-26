# Copilot Instructions

Project: ghub-desk (GitHub organization management CLI + MCP server).

## Scope
- CLI manages org members/teams/repos via GitHub API and caches to SQLite.
- MCP server exposes the same pull/view/push capabilities over stdio.

## Safety and Behavior
- Push operations are DRYRUN by default; require `--exec` to mutate GitHub state.
- Use `mcp.allow_pull` and `mcp.allow_write` to control exposed MCP tools.
- Prefer read-only operations unless explicitly asked to mutate.

## Configuration
- Auth uses either a PAT (`GHUB_DESK_GITHUB_TOKEN`) or GitHub App settings (choose one).
- Config file example lives at `~/.config/ghub-desk/config.yaml`.
- Database defaults to `./ghub-desk.db`; override with `database_path`.

## Core Commands
- `pull`: fetch org data from GitHub API and optionally persist to SQLite.
- `view`: read cached data from SQLite.
- `push add/remove`: manage members/teams/repos (requires `--exec` to apply).
- `auditlogs`: fetch org audit logs by actor.
- `init`: create config or initialize the SQLite DB.
- `version`: show build metadata.

## MCP Tools
- Always available: `health`, `view_*`, `auditlogs`.
- Requires allow_pull: `pull_*` tools.
- Requires allow_write: `push_*` tools (`exec:true` to apply).
- `resource://ghub-desk/...` URIs are embedded in `mcp/docs.go` (not files in `docs/`).

## Build and Test
- Build: `make build` (MCP server via `./build/ghub-desk mcp --debug`).
- Test: `make test`.

## Review Criteria

Act as an experienced **Senior Software Engineer** conducting a code review.
When asked to review code or a Pull Request, analyze the context and provide feedback based on the following standards.

### 1. Review Priority (Order of Importance)
Prioritize your findings in this order:
1.  **Bugs & Logic Errors**: Runtime errors, race conditions, infinite loops, and broken logic.
2.  **Security**: Vulnerabilities (Injection, XSS), exposed secrets, and weak authentication.
3.  **Performance**: inefficient algorithms (e.g., N+1 queries), memory leaks, and unoptimized loops.
4.  **Maintainability**: Violations of DRY/SOLID principles, overly complex code, and poor naming.

### 2. Review Checklist (Mental Model)
Evaluate the code against these specific questions:
- Are `nil`, zero values, empty slices/maps, and boundary values handled correctly?
- Are types defined strictly? Is overuse of `interface{}`/`any` or unsafe casts avoided?
- Are there sufficient unit tests covering the changes? Are failure scenarios tested?
- Are errors (and any panics) handled and logged properly? Is the user experience degraded gracefully on failure?
- Does this change negatively impact existing features or shared state?
- Do not log secrets, auth codes, or tokens.

### 3. Output Guidelines
- **Be Concise**: Avoid trivial nitpicking (e.g., minor formatting preferences). Focus on high-impact issues.
- **Show, Don't Just Tell**: Always provide **refactored code snippets** to demonstrate the fix.
- **Explain "Why"**: Briefly explain the reasoning behind your suggestion (e.g., "This prevents a potential null pointer exception").
