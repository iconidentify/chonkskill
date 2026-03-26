package fredmeyer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/iconidentify/chonkskill/pkg/skill"
	"github.com/joho/godotenv"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSkillViaInMemoryMCP(t *testing.T) {
	godotenv.Load("../../.env")

	if os.Getenv("KROGER_CLIENT_ID") == "" {
		t.Skip("KROGER_CLIENT_ID not set")
	}

	// Create the skill.
	s, err := New(ConfigFromEnv())
	if err != nil {
		t.Fatalf("creating skill: %v", err)
	}

	// Create an MCP server and register the skill's tools.
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1.0"}, nil)
	reg := &testMCPRegistry{server: srv}
	if err := s.Register(reg); err != nil {
		t.Fatalf("registering skill: %v", err)
	}

	// Connect via in-memory transport.
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	serverSession, err := srv.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer clientSession.Close()

	// List tools -- should have all 20.
	t.Run("list_tools", func(t *testing.T) {
		result, err := clientSession.ListTools(ctx, nil)
		if err != nil {
			t.Fatalf("list tools: %v", err)
		}
		t.Logf("registered %d tools", len(result.Tools))
		for _, tool := range result.Tools {
			t.Logf("  %s", tool.Name)
		}
		if len(result.Tools) != 20 {
			t.Errorf("expected 20 tools, got %d", len(result.Tools))
		}
	})

	// Call search_products via MCP.
	t.Run("search_products", func(t *testing.T) {
		result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
			Name:      "fredmeyer:search_products",
			Arguments: map[string]any{"term": "organic milk", "location_id": "70100023", "limit": 3},
		})
		if err != nil {
			t.Fatalf("call: %v", err)
		}
		if result.IsError {
			t.Fatalf("tool error: %v", result.Content)
		}
		text := result.Content[0].(*mcp.TextContent).Text
		var parsed map[string]any
		if err := json.Unmarshal([]byte(text), &parsed); err != nil {
			t.Fatalf("result not JSON: %v", err)
		}
		data := parsed["data"].([]any)
		t.Logf("found %d products", len(data))
		for _, p := range data {
			prod := p.(map[string]any)
			fmt.Fprintf(os.Stderr, "  - %s %s\n", prod["brand"], prod["description"])
		}
	})

	// Call search_locations via MCP.
	t.Run("search_locations", func(t *testing.T) {
		result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
			Name:      "fredmeyer:search_locations",
			Arguments: map[string]any{"zip_code": "98052", "limit": 3},
		})
		if err != nil {
			t.Fatalf("call: %v", err)
		}
		if result.IsError {
			t.Fatalf("tool error: %v", result.Content)
		}
		t.Logf("OK (%d bytes)", len(result.Content[0].(*mcp.TextContent).Text))
	})

	// Cart round-trip.
	t.Run("cart_round_trip", func(t *testing.T) {
		// Clear
		callTool(t, clientSession, ctx, "fredmeyer:clear_cart", nil)

		// View empty
		result := callTool(t, clientSession, ctx, "fredmeyer:view_cart", nil)
		if result != "Cart is empty." {
			t.Errorf("expected empty cart, got: %s", result)
		}

		// Auth check
		result = callTool(t, clientSession, ctx, "fredmeyer:test_authentication", nil)
		t.Logf("auth: %s", result)
	})
}

func callTool(t *testing.T, cs *mcp.ClientSession, ctx context.Context, name string, args map[string]any) string {
	t.Helper()
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	return result.Content[0].(*mcp.TextContent).Text
}

// testMCPRegistry adapts the MCP server for testing.
type testMCPRegistry struct {
	server *mcp.Server
}

func (r *testMCPRegistry) RegisterTool(def skill.ToolDef, handler skill.Handler) {
	h := handler
	mcp.AddTool(r.server, &mcp.Tool{
		Name:        def.Name,
		Description: def.Description,
	}, func(ctx context.Context, req *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
		if args == nil {
			args = map[string]any{}
		}
		result, err := h(ctx, args)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: result}},
		}, nil, nil
	})
}

func (r *testMCPRegistry) RegisterSkill(name, description, content string, tags []string) error {
	return nil
}
