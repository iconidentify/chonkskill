// Package drakekb provides a chonkskill for searching and reading the Drake
// Software Knowledge Base. It gives tax preparation agents on-demand access
// to Drake's per-form documentation, screen-level instructions, EF message
// lookups, and general tax software guidance.
package drakekb

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/iconidentify/chonkskill/pkg/skill"
	"github.com/iconidentify/chonkskill/skills/drakekb/internal/drake"
)

//go:embed skill.md
var SkillContent string

//go:embed agent.md
var AgentContent string

// Config holds settings for the Drake KB skill. No credentials needed --
// the KB is publicly accessible.
type Config struct{}

// ConfigFromEnv returns a Config (no env vars needed for this skill).
func ConfigFromEnv() Config {
	return Config{}
}

// New creates the Drake KB skill with all tools registered.
func New(cfg Config) (*skill.Skill, error) {
	client := drake.NewClient()

	s := skill.New(skill.Definition{
		Name:         "drakekb",
		Description:  "Drake Software Knowledge Base -- search tax form documentation, screen instructions, EF messages, and Drake Tax guides",
		SkillContent: SkillContent,
		AgentContent: AgentContent,
		Tags:         []string{"tax", "drake", "knowledge-base", "forms", "tax-preparation"},
	})

	// --- Search ---
	skill.AddTool(s, "search",
		"Search the Drake Software Knowledge Base for articles about forms, screens, EF messages, or tax topics. Returns titles, abstracts, and URLs.",
		func(ctx context.Context, args SearchArgs) (string, error) {
			limit := args.Limit
			if limit == 0 {
				limit = 10
			}
			articles, err := client.Search(args.Query, args.Category, limit)
			if err != nil {
				return "", fmt.Errorf("search failed: %w", err)
			}
			if len(articles) == 0 {
				return "No articles found. Try broader search terms or check the form number.", nil
			}

			// Format results.
			type result struct {
				Title    string `json:"title"`
				Abstract string `json:"abstract"`
				URL      string `json:"url"`
				Category string `json:"category"`
			}
			results := make([]result, len(articles))
			for i, a := range articles {
				results[i] = result{
					Title:    a.Title,
					Abstract: a.Abstract,
					URL:      a.URL,
					Category: a.Category,
				}
			}
			b, _ := json.MarshalIndent(results, "", "  ")
			return string(b), nil
		})

	// --- Read Article ---
	skill.AddTool(s, "read_article",
		"Fetch and read the full content of a Drake KB article by URL. Returns the article title, body text, and navigation path.",
		func(ctx context.Context, args ReadArticleArgs) (string, error) {
			if args.URL == "" {
				return "", fmt.Errorf("url is required")
			}
			if !strings.HasPrefix(args.URL, "https://kb.drakesoftware.com/") {
				return "", fmt.Errorf("url must be a kb.drakesoftware.com URL")
			}

			article, err := client.GetArticle(args.URL)
			if err != nil {
				return "", fmt.Errorf("failed to read article: %w", err)
			}

			b, _ := json.MarshalIndent(article, "", "  ")
			return string(b), nil
		})

	// --- Form Lookup ---
	skill.AddTool(s, "lookup_form",
		"Search for Drake KB articles about a specific IRS form or schedule. Searches for the form number in Drake Tax articles.",
		func(ctx context.Context, args FormLookupArgs) (string, error) {
			if args.FormNumber == "" {
				return "", fmt.Errorf("form_number is required")
			}

			// Search with form-specific queries.
			queries := []string{
				args.FormNumber,
				"Form " + args.FormNumber,
			}

			seen := make(map[string]bool)
			var allResults []drake.Article

			for _, q := range queries {
				articles, err := client.Search(q, "Drake-Tax", 10)
				if err != nil {
					continue
				}
				for _, a := range articles {
					if !seen[a.URL] {
						seen[a.URL] = true
						allResults = append(allResults, a)
					}
				}
			}

			if len(allResults) == 0 {
				return fmt.Sprintf("No articles found for form %s. Try searching with different terms using drakekb:search.", args.FormNumber), nil
			}

			if len(allResults) > 15 {
				allResults = allResults[:15]
			}

			type result struct {
				Title    string `json:"title"`
				Abstract string `json:"abstract"`
				URL      string `json:"url"`
			}
			results := make([]result, len(allResults))
			for i, a := range allResults {
				results[i] = result{
					Title:    a.Title,
					Abstract: a.Abstract,
					URL:      a.URL,
				}
			}
			b, _ := json.MarshalIndent(results, "", "  ")
			return string(b), nil
		})

	// --- Index Stats ---
	skill.AddTool(s, "index_stats",
		"Show stats about the Drake KB search index: total articles and article counts by category.",
		func(ctx context.Context, args EmptyArgs) (string, error) {
			total, categories, err := client.IndexStats()
			if err != nil {
				return "", fmt.Errorf("failed to load index: %w", err)
			}

			result := map[string]any{
				"total_articles": total,
				"categories":     categories,
			}
			b, _ := json.MarshalIndent(result, "", "  ")
			return string(b), nil
		})

	return s, nil
}

// Register is the one-liner for chonkbase integration.
func Register(reg skill.Registry, cfg Config) error {
	s, err := New(cfg)
	if err != nil {
		return err
	}
	return s.Register(reg)
}
