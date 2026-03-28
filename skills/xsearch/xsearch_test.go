package xsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestXSearchLive(t *testing.T) {
	key := os.Getenv("XAI_API_KEY")
	if key == "" {
		t.Skip("XAI_API_KEY not set")
	}

	s, err := New(Config{APIKey: key})
	if err != nil {
		t.Fatalf("creating skill: %v", err)
	}

	ctx := context.Background()

	t.Run("search", func(t *testing.T) {
		result, err := s.Handlers["xsearch:search"](ctx, map[string]any{
			"query": "what is trending on X right now",
		})
		if err != nil {
			t.Fatalf("search: %v", err)
		}

		var out map[string]any
		if err := json.Unmarshal([]byte(result), &out); err != nil {
			t.Fatalf("parsing result: %v", err)
		}

		text, _ := out["text"].(string)
		if text == "" {
			t.Fatal("expected non-empty text")
		}
		fmt.Printf("=== Search Result (%d chars) ===\n%s\n", len(text), text)

		if citations, ok := out["citations"].([]any); ok {
			fmt.Printf("\n=== %d Citations ===\n", len(citations))
			for _, c := range citations {
				cm, _ := c.(map[string]any)
				fmt.Printf("  %s\n", cm["url"])
			}
		}
	})
}
