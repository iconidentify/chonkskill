// Package mcprunner provides a standalone MCP server that can host one or more
// chonkskill skills over stdio or HTTP transport.
package mcprunner

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/iconidentify/chonkskill/pkg/skill"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Options configures the standalone runner.
type Options struct {
	Port      int
	Stdio     bool
	AuthToken string
}

// Serve runs one or more skills as an MCP server.
func Serve(ctx context.Context, skills []*skill.Skill, opts Options) error {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "chonkskill",
		Version: "1.0.0",
	}, nil)

	// Adapt the MCP server to the skill.Registry interface.
	reg := &mcpRegistry{server: srv}

	for _, s := range skills {
		if err := s.Register(reg); err != nil {
			return fmt.Errorf("registering skill %s: %w", s.Def.Name, err)
		}
		slog.Info("registered skill", "name", s.Def.Name, "tools", len(s.Tools))
	}

	if opts.Stdio {
		slog.Info("serving over stdio")
		return srv.Run(ctx, &mcp.StdioTransport{})
	}

	// HTTP mode via StreamableHTTP.
	port := opts.Port
	if port == 0 {
		port = 8080
	}
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	slog.Info("serving over HTTP", "addr", addr)

	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return srv
	}, &mcp.StreamableHTTPOptions{Stateless: true})

	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	return http.ListenAndServe(addr, mux)
}

// mcpRegistry adapts an MCP server to the skill.Registry interface.
type mcpRegistry struct {
	server *mcp.Server
}

// rawArgs is used to receive the raw arguments from an MCP call.
// We use it as the typed arg for mcp.AddTool, then parse the raw JSON
// from the request ourselves.
// We use map[string]any as the typed arg so the MCP SDK schema validation
// accepts arbitrary properties from callers.

func (r *mcpRegistry) RegisterTool(def skill.ToolDef, handler skill.Handler) {
	h := handler // capture for closure
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

func (r *mcpRegistry) RegisterSkill(name, description, content string, tags []string) error {
	slog.Info("skill content available", "name", name, "tags", tags)
	return nil
}

func (r *mcpRegistry) RegisterConfigSchema(skillName string, schema skill.ConfigSchema) {
	slog.Info("config schema registered", "skill", skillName, "fields", len(schema.Fields))
}
