package mcp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"ghub-desk/store"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// withTempStore points the store at an initialized SQLite file under t.TempDir so view
// tools can run without touching the developer's ghub-desk.db.
func withTempStore(t *testing.T) {
	t.Helper()

	store.SetDBPath(filepath.Join(t.TempDir(), "test.db"))
	t.Cleanup(func() { store.SetDBPath("") })

	db, err := store.InitDatabase()
	if err != nil {
		t.Fatalf("init temp database: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close temp database: %v", err)
	}
}

// toolsListBudget caps the serialized size of tools/list. Every tool definition is resent
// to the model on each turn, so growth here is paid repeatedly. Dropping the inferred
// output schemas took the payload from ~35,900 to ~17,300 bytes; this budget leaves room
// for a few new tools while catching an accidental reintroduction of output schemas.
const toolsListBudget = 22000

// TestToolsListStaysWithinBudget guards the per-turn context cost of the tool catalog.
func TestToolsListStaysWithinBudget(t *testing.T) {
	cs := connectTestSession(t)

	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	encoded, err := json.Marshal(tools.Tools)
	if err != nil {
		t.Fatalf("marshal tools: %v", err)
	}
	t.Logf("tools/list: %d tools, %d bytes", len(tools.Tools), len(encoded))
	if len(encoded) > toolsListBudget {
		t.Errorf("tools/list is %d bytes, over the %d budget; check for reintroduced output schemas or verbose descriptions",
			len(encoded), toolsListBudget)
	}
}

// TestToolsOmitOutputSchema pins the deliberate choice to register handlers with an "any"
// output type. The SDK infers an output schema from any concrete Out type, which more than
// doubled the tools/list payload for no behavioral gain.
func TestToolsOmitOutputSchema(t *testing.T) {
	cs := connectTestSession(t)

	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	for _, tool := range tools.Tools {
		if tool.OutputSchema != nil {
			t.Errorf("tool %q publishes an output schema; register it with sdk.AddTool[In, any] to keep tools/list small",
				tool.Name)
		}
	}
}

// TestToolsStillReturnStructuredContent verifies the tradeoff actually held: dropping the
// output schema must not stop tools from returning structured results.
func TestToolsStillReturnStructuredContent(t *testing.T) {
	withTempStore(t)
	cs := connectTestSession(t)

	res, err := cs.CallTool(context.Background(), &sdk.CallToolParams{
		Name:      "view_users",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("tools/call view_users: %v", err)
	}
	if res.IsError {
		t.Fatalf("tools/call view_users reported an error: %+v", res.Content)
	}
	if res.StructuredContent == nil {
		t.Error("view_users returned no structured content")
	}
	if len(res.Content) == 0 {
		t.Error("view_users returned no content blocks")
	}
}
