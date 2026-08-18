package mcp

import (
	"context"
	"regexp"
	"strings"
	"testing"

	appcfg "ghub-desk/config"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// resourceURIPattern matches any resource://ghub-desk/... reference embedded in prose,
// stopping before the trailing sentence punctuation used in tool descriptions.
var resourceURIPattern = regexp.MustCompile(`resource://[^\s]+?(?:[.,]\s|[.,]?$)`)

// connectTestSession starts an in-memory server exposing every tool tier and returns a
// connected client session.
func connectTestSession(t *testing.T) *sdk.ClientSession {
	t.Helper()

	cfg := &appcfg.Config{}
	cfg.MCP.AllowPull = true
	cfg.MCP.AllowWrite = true

	srv := sdk.NewServer(&sdk.Implementation{Name: "ghub-desk", Version: "test"},
		&sdk.ServerOptions{HasTools: true, HasResources: true})
	registerDocsResources(srv)
	registerAllowedTools(srv, cfg)

	serverTransport, clientTransport := sdk.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "test"}, nil).
		Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// TestDocsResourcesReadable verifies every registered documentation resource can actually
// be read and returns a non-empty markdown body.
func TestDocsResourcesReadable(t *testing.T) {
	cs := connectTestSession(t)
	ctx := context.Background()

	listed, err := cs.ListResources(ctx, nil)
	if err != nil {
		t.Fatalf("resources/list: %v", err)
	}
	if len(listed.Resources) != len(docsResources) {
		t.Fatalf("resources/list returned %d resources, want %d", len(listed.Resources), len(docsResources))
	}

	for _, res := range docsResources {
		t.Run(res.name, func(t *testing.T) {
			got, err := cs.ReadResource(ctx, &sdk.ReadResourceParams{URI: res.uri})
			if err != nil {
				t.Fatalf("resources/read %s: %v", res.uri, err)
			}
			if len(got.Contents) != 1 {
				t.Fatalf("resources/read %s returned %d contents, want 1", res.uri, len(got.Contents))
			}
			if got.Contents[0].Text == "" {
				t.Errorf("resources/read %s returned an empty body", res.uri)
			}
			if got.Contents[0].MIMEType != docsMIMEType {
				t.Errorf("resources/read %s MIME type = %q, want %q", res.uri, got.Contents[0].MIMEType, docsMIMEType)
			}
		})
	}
}

// TestFragmentURIIsNotResolvable documents why tool descriptions must not carry anchors:
// the SDK resolves resources by exact URI, so a fragment-bearing URI is rejected before
// reaching the handler.
func TestFragmentURIIsNotResolvable(t *testing.T) {
	cs := connectTestSession(t)

	if _, err := cs.ReadResource(context.Background(),
		&sdk.ReadResourceParams{URI: docsToolsURI + "#view_users"}); err == nil {
		t.Fatal("reading a fragment-bearing URI succeeded; descriptions may now use anchors")
	}
}

// TestToolDescriptionsReferenceExistingResources is the regression guard for the anchored
// URIs that previously made every "Usage:" pointer resolve to ResourceNotFound.
func TestToolDescriptionsReferenceExistingResources(t *testing.T) {
	cs := connectTestSession(t)
	ctx := context.Background()

	registered := make(map[string]bool, len(docsResources))
	for _, res := range docsResources {
		registered[res.uri] = true
	}

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	if len(tools.Tools) == 0 {
		t.Fatal("tools/list returned no tools")
	}

	for _, tool := range tools.Tools {
		for _, match := range resourceURIPattern.FindAllString(tool.Description, -1) {
			uri := strings.TrimRight(strings.TrimSpace(match), ".,")
			if !registered[uri] {
				t.Errorf("tool %q references unknown resource %q; descriptions must use a registered URI without a fragment",
					tool.Name, uri)
			}
		}
	}
}
