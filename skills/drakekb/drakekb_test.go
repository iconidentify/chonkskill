package drakekb

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestDrakeKBLive(t *testing.T) {
	s, err := New(Config{})
	if err != nil {
		t.Fatalf("creating skill: %v", err)
	}
	t.Logf("skill created: %s (%d tools)", s.Def.Name, len(s.Tools))

	ctx := context.Background()

	// Test 1: Index stats (forces index download).
	t.Run("index_stats", func(t *testing.T) {
		handler := s.Handlers["drakekb:index_stats"]
		result, err := handler(ctx, map[string]any{})
		if err != nil {
			t.Fatalf("index_stats: %v", err)
		}

		var stats map[string]any
		json.Unmarshal([]byte(result), &stats)
		total := stats["total_articles"].(float64)
		t.Logf("total articles: %.0f", total)
		if total < 100 {
			t.Errorf("expected 100+ articles, got %.0f", total)
		}

		cats := stats["categories"].(map[string]any)
		for cat, count := range cats {
			t.Logf("  %s: %.0f", cat, count.(float64))
		}
	})

	// Test 2: Search for "dependent care credit".
	t.Run("search_dependent_care", func(t *testing.T) {
		handler := s.Handlers["drakekb:search"]
		result, err := handler(ctx, map[string]any{
			"query": "dependent care credit",
			"limit": float64(5),
		})
		if err != nil {
			t.Fatalf("search: %v", err)
		}

		var articles []map[string]any
		json.Unmarshal([]byte(result), &articles)
		t.Logf("found %d articles", len(articles))
		for _, a := range articles {
			fmt.Fprintf(os.Stderr, "  - %s\n    %s\n", a["title"], a["url"])
		}
		if len(articles) == 0 {
			t.Error("expected results for 'dependent care credit'")
		}
	})

	// Test 3: Form lookup for 2441.
	t.Run("lookup_form_2441", func(t *testing.T) {
		handler := s.Handlers["drakekb:lookup_form"]
		result, err := handler(ctx, map[string]any{
			"form_number": "2441",
		})
		if err != nil {
			t.Fatalf("lookup_form: %v", err)
		}

		var articles []map[string]any
		json.Unmarshal([]byte(result), &articles)
		t.Logf("found %d articles for Form 2441", len(articles))
		for _, a := range articles {
			fmt.Fprintf(os.Stderr, "  - %s\n", a["title"])
		}
		if len(articles) == 0 {
			t.Error("expected results for Form 2441")
		}
	})

	// Test 4: Read an article.
	t.Run("read_article", func(t *testing.T) {
		handler := s.Handlers["drakekb:read_article"]
		result, err := handler(ctx, map[string]any{
			"url": "https://kb.drakesoftware.com/kb/Drake-Tax/11750.htm",
		})
		if err != nil {
			t.Fatalf("read_article: %v", err)
		}

		var article map[string]any
		json.Unmarshal([]byte(result), &article)
		t.Logf("title: %s", article["title"])
		t.Logf("toc_path: %s", article["toc_path"])

		body := article["body"].(string)
		t.Logf("body length: %d chars", len(body))
		if len(body) < 100 {
			t.Error("article body seems too short")
		}

		// Print first 500 chars of body.
		preview := body
		if len(preview) > 500 {
			preview = preview[:500]
		}
		fmt.Fprintf(os.Stderr, "\n--- Article Preview ---\n%s\n--- End Preview ---\n", preview)
	})

	// Test 5: Search with category filter.
	t.Run("search_drake_tax_category", func(t *testing.T) {
		handler := s.Handlers["drakekb:search"]
		result, err := handler(ctx, map[string]any{
			"query":    "Schedule C",
			"category": "Drake-Tax",
			"limit":    float64(5),
		})
		if err != nil {
			t.Fatalf("search: %v", err)
		}

		var articles []map[string]any
		json.Unmarshal([]byte(result), &articles)
		t.Logf("found %d Drake-Tax articles for 'Schedule C'", len(articles))
		for _, a := range articles {
			fmt.Fprintf(os.Stderr, "  - %s\n", a["title"])
		}
	})
}
