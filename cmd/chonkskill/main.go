package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/iconidentify/chonkskill/pkg/mcprunner"
	"github.com/iconidentify/chonkskill/pkg/skill"
	"github.com/iconidentify/chonkskill/skills/autonovel"
	"github.com/iconidentify/chonkskill/skills/drakekb"
	"github.com/iconidentify/chonkskill/skills/kidsnovel"
	"github.com/iconidentify/chonkskill/skills/fredmeyer"
	"github.com/iconidentify/chonkskill/skills/xsearch"
	"github.com/joho/godotenv"
)

// catalog maps skill names to their constructors.
// Adding a new skill = one line here.
var catalog = map[string]func() (*skill.Skill, error){
	"fredmeyer": func() (*skill.Skill, error) {
		return fredmeyer.New(fredmeyer.ConfigFromEnv())
	},
	"drakekb": func() (*skill.Skill, error) {
		return drakekb.New(drakekb.ConfigFromEnv())
	},
	"xsearch": func() (*skill.Skill, error) {
		return xsearch.New(xsearch.ConfigFromEnv())
	},
	"autonovel": func() (*skill.Skill, error) {
		return autonovel.New(autonovel.ConfigFromEnv())
	},
	"kidsnovel": func() (*skill.Skill, error) {
		return kidsnovel.New(kidsnovel.ConfigFromEnv())
	},
}

func main() {
	godotenv.Load()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "list":
		cmdList()
	case "serve":
		cmdServe()
	case "test":
		cmdTest()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `chonkskill - Skills, Orchestration, Utilities Library

Usage:
  chonkskill list                         List available skills
  chonkskill serve <skill> [--stdio]      Run skill(s) as MCP server
  chonkskill serve --all [--stdio]        Run all skills as MCP server
  chonkskill test <skill>                 Run API integration tests

Examples:
  chonkskill serve fredmeyer --stdio      Stdio mode for Claude Desktop
  chonkskill serve fredmeyer              HTTP mode on :8080
  chonkskill test fredmeyer               Test Kroger API connectivity`)
}

func cmdList() {
	fmt.Println("Available skills:")
	for name, constructor := range catalog {
		s, err := constructor()
		if err != nil {
			fmt.Printf("  %-20s (error: %v)\n", name, err)
			continue
		}
		fmt.Printf("  %-20s %s (%d tools)\n", name, s.Def.Description, len(s.Tools))
	}
}

func cmdServe() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: chonkskill serve <skill...> [--stdio] [--all]")
		os.Exit(1)
	}

	stdio := false
	all := false
	var skillNames []string

	for _, arg := range os.Args[2:] {
		switch arg {
		case "--stdio":
			stdio = true
		case "--all":
			all = true
		default:
			if !strings.HasPrefix(arg, "--") {
				skillNames = append(skillNames, arg)
			}
		}
	}

	if all {
		for name := range catalog {
			skillNames = append(skillNames, name)
		}
	}

	if len(skillNames) == 0 {
		fmt.Fprintln(os.Stderr, "no skills specified. Use --all or name skills to serve.")
		os.Exit(1)
	}

	var skills []*skill.Skill
	for _, name := range skillNames {
		constructor, ok := catalog[name]
		if !ok {
			fmt.Fprintf(os.Stderr, "unknown skill: %s\n", name)
			os.Exit(1)
		}
		s, err := constructor()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error creating skill %s: %v\n", name, err)
			os.Exit(1)
		}
		skills = append(skills, s)
	}

	if err := mcprunner.Serve(context.Background(), skills, mcprunner.Options{
		Port:  8080,
		Stdio: stdio,
	}); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

func cmdTest() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: chonkskill test <skill>")
		os.Exit(1)
	}

	name := os.Args[2]
	switch name {
	case "fredmeyer":
		runFredMeyerTest()
	case "xsearch":
		runXSearchTest()
	default:
		fmt.Fprintf(os.Stderr, "no tests for skill: %s\n", name)
		os.Exit(1)
	}
}

func runFredMeyerTest() {
	fmt.Println("=== Fred Meyer Skill - API Test ===")
	fmt.Println()

	cfg := fredmeyer.ConfigFromEnv()
	s, err := fredmeyer.New(cfg)
	if err != nil {
		fmt.Printf("FAILED to create skill: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Skill created: %s (%d tools)\n\n", s.Def.Name, len(s.Tools))

	// Use the skill's handlers directly.
	ctx := context.Background()

	// Test search_locations
	fmt.Print("1. Searching for stores near 98052... ")
	handler := s.Handlers["fredmeyer:search_locations"]
	result, err := handler(ctx, map[string]any{"zip_code": "98052", "limit": float64(3)})
	if err != nil {
		fmt.Printf("FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("OK (%d bytes)\n", len(result))

	// Test search_products
	fmt.Print("2. Searching for 'bananas'... ")
	handler = s.Handlers["fredmeyer:search_products"]
	result, err = handler(ctx, map[string]any{"term": "bananas", "location_id": "70100023", "limit": float64(3)})
	if err != nil {
		fmt.Printf("FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("OK (%d bytes)\n", len(result))

	// Test list_chains
	fmt.Print("3. Listing chains... ")
	handler = s.Handlers["fredmeyer:list_chains"]
	result, err = handler(ctx, map[string]any{})
	if err != nil {
		fmt.Printf("FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("OK (%d bytes)\n", len(result))

	// Test cart operations
	fmt.Print("4. Cart operations (local)... ")
	handler = s.Handlers["fredmeyer:clear_cart"]
	_, err = handler(ctx, map[string]any{})
	if err != nil {
		fmt.Printf("FAILED clear: %v\n", err)
		os.Exit(1)
	}
	handler = s.Handlers["fredmeyer:view_cart"]
	result, err = handler(ctx, map[string]any{})
	if err != nil {
		fmt.Printf("FAILED view: %v\n", err)
		os.Exit(1)
	}
	if result != "Cart is empty." {
		fmt.Printf("FAILED: expected empty cart, got %s\n", result)
		os.Exit(1)
	}
	fmt.Println("OK")

	// Test auth status
	fmt.Print("5. Auth status... ")
	handler = s.Handlers["fredmeyer:test_authentication"]
	result, err = handler(ctx, map[string]any{})
	if err != nil {
		fmt.Printf("FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("OK (%s)\n", result)

	fmt.Println("\n=== All tests passed ===")
}

func runXSearchTest() {
	fmt.Println("=== X Search Skill - API Test ===")
	fmt.Println()

	cfg := xsearch.ConfigFromEnv()
	s, err := xsearch.New(cfg)
	if err != nil {
		fmt.Printf("FAILED to create skill: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Skill created: %s (%d tools)\n\n", s.Def.Name, len(s.Tools))

	ctx := context.Background()

	// Test basic search
	fmt.Print("1. Searching X for 'AI news today'... ")
	handler := s.Handlers["xsearch:search"]
	result, err := handler(ctx, map[string]any{"query": "AI news today"})
	if err != nil {
		fmt.Printf("FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("OK (%d bytes)\n", len(result))

	// Test profile search
	fmt.Print("2. Searching profile @xai... ")
	handler = s.Handlers["xsearch:profile"]
	result, err = handler(ctx, map[string]any{"handle": "xai"})
	if err != nil {
		fmt.Printf("FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("OK (%d bytes)\n", len(result))

	// Test filtered search
	fmt.Print("3. Searching with handle filter... ")
	handler = s.Handlers["xsearch:search"]
	result, err = handler(ctx, map[string]any{
		"query":   "latest announcements",
		"handles": []any{"openai", "anthropic"},
	})
	if err != nil {
		fmt.Printf("FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("OK (%d bytes)\n", len(result))

	fmt.Println("\n=== All tests passed ===")
}
