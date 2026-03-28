// Package xsearch provides a chonkskill for searching X (Twitter) in
// real-time using the xAI Grok API. It gives agents access to current posts,
// trends, discussions, and profile-specific searches with source citations.
package xsearch

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"

	"github.com/iconidentify/chonkskill/pkg/skill"
	"github.com/iconidentify/chonkskill/skills/xsearch/internal/xai"
)

//go:embed skill.md
var SkillContent string

//go:embed agent.md
var AgentContent string

// Schema declares the config fields for the X search skill.
// Chonkbase reads this to auto-generate settings UI.
var Schema = skill.ConfigSchema{
	Fields: []skill.ConfigField{
		{
			Name:        "api_key",
			Label:       "xAI API Key",
			Description: "API key for xAI Grok search. Get one at https://console.x.ai",
			Type:        skill.FieldString,
			Scope:       skill.ScopeGlobal,
			Required:    true,
			Secret:      true,
			EnvVar:      "XAI_API_KEY",
		},
		{
			Name:    "model",
			Label:   "Grok Model",
			Description: "Which Grok model to use for X search (grok-4 family required)",
			Type:    skill.FieldString,
			Scope:   skill.ScopeGlobal,
			Default: "grok-4-0709",
			EnvVar:  "XAI_MODEL",
		},
	},
}

// Def is the skill definition, exported so Register() can access it
// for metadata-only registration when config is missing.
var Def = skill.Definition{
	Name:         "xsearch",
	Description:  "X (Twitter) real-time search -- find posts, trends, discussions, and profile activity via xAI Grok",
	SkillContent: SkillContent,
	AgentContent: AgentContent,
	Tags:         []string{"x", "twitter", "social", "search", "grok", "xai"},
	Config:       Schema,
}

// Config holds settings for the X search skill.
type Config struct {
	APIKey string // XAI_API_KEY
	Model  string // xAI model, defaults to grok-4-0709
}

// ConfigFromEnv returns a Config from environment variables.
func ConfigFromEnv() Config {
	return Config{
		APIKey: os.Getenv("XAI_API_KEY"),
		Model:  os.Getenv("XAI_MODEL"),
	}
}

// New creates the X search skill with all tools registered.
// Returns an error if required config is missing -- use Register() for
// graceful degradation in chonkbase.
func New(cfg Config) (*skill.Skill, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("XAI_API_KEY is required -- get one at https://console.x.ai")
	}

	client := xai.NewClient(cfg.APIKey, cfg.Model)

	s := skill.New(Def)

	// --- Search ---
	skill.AddTool(s, "search",
		"Search X (Twitter) for posts about any topic. Returns Grok's synthesis of matching posts with citation links to original X posts.",
		func(ctx context.Context, args SearchArgs) (string, error) {
			if len(args.Handles) > 0 && len(args.ExcludeHandles) > 0 {
				return "", fmt.Errorf("handles and exclude_handles are mutually exclusive -- use one or the other")
			}

			result, err := client.Search(xai.SearchParams{
				Query:           args.Query,
				AllowedHandles:  args.Handles,
				ExcludedHandles: args.ExcludeHandles,
				FromDate:        args.FromDate,
				ToDate:          args.ToDate,
			})
			if err != nil {
				return "", fmt.Errorf("search failed: %w", err)
			}

			return formatResult(result), nil
		})

	// --- Search with Media ---
	skill.AddTool(s, "search_with_media",
		"Search X (Twitter) with image and/or video understanding enabled. Use when the query involves visual content like infographics, screenshots, or video posts.",
		func(ctx context.Context, args SearchWithMediaArgs) (string, error) {
			if len(args.Handles) > 0 && len(args.ExcludeHandles) > 0 {
				return "", fmt.Errorf("handles and exclude_handles are mutually exclusive")
			}

			result, err := client.Search(xai.SearchParams{
				Query:                    args.Query,
				AllowedHandles:           args.Handles,
				ExcludedHandles:          args.ExcludeHandles,
				FromDate:                 args.FromDate,
				ToDate:                   args.ToDate,
				EnableImageUnderstanding: args.Images,
				EnableVideoUnderstanding: args.Video,
			})
			if err != nil {
				return "", fmt.Errorf("search failed: %w", err)
			}

			return formatResult(result), nil
		})

	// --- Profile Search ---
	skill.AddTool(s, "profile",
		"Search a specific X user's posts. Optionally filter by topic and date range.",
		func(ctx context.Context, args ProfileSearchArgs) (string, error) {
			if args.Handle == "" {
				return "", fmt.Errorf("handle is required")
			}

			query := args.Query
			if query == "" {
				query = fmt.Sprintf("latest posts from @%s", args.Handle)
			}

			result, err := client.Search(xai.SearchParams{
				Query:          query,
				AllowedHandles: []string{args.Handle},
				FromDate:       args.FromDate,
				ToDate:         args.ToDate,
			})
			if err != nil {
				return "", fmt.Errorf("profile search failed: %w", err)
			}

			return formatResult(result), nil
		})

	return s, nil
}

// Register is the one-liner for chonkbase integration.
// If config is incomplete, registers the skill as unconfigured -- schema
// and content are published so chonkbase can show the config UI, but no
// tools are active until the required settings are provided.
func Register(reg skill.Registry, cfg Config) error {
	s, err := New(cfg)
	if err != nil {
		// Graceful degradation: register metadata so chonkbase can
		// discover this skill and render its config UI.
		return skill.Unconfigured(Def).Register(reg)
	}
	return s.Register(reg)
}

// formatResult converts a SearchResult to LLM-friendly JSON output.
func formatResult(r *xai.SearchResult) string {
	type citationOut struct {
		URL   string `json:"url"`
		Title string `json:"title,omitempty"`
	}
	type output struct {
		Text      string       `json:"text"`
		Citations []citationOut `json:"citations,omitempty"`
	}

	out := output{Text: r.Text}
	for _, c := range r.Citations {
		out.Citations = append(out.Citations, citationOut{
			URL:   c.URL,
			Title: c.Title,
		})
	}

	b, _ := json.MarshalIndent(out, "", "  ")
	return string(b)
}
