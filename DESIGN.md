# chonkskill -- Skills + Tools, All-in-One

## What It Is

A Go library that ships everything an agent needs for a new capability:

1. **SKILL.md** -- Hermes-compatible procedural knowledge (how to use the tools, workflows, domain expertise)
2. **Go tool handlers** -- The actual API integrations registered into ExpertToolRegistry
3. **Agent definition** -- Tool whitelist, description, suggested system prompt fragments

One `go get`, one `Register()` call. Chonkbase gets the tools in the registry AND the skills in the database. The agent knows both *what* it can do and *how* to do it well.

## The Problem With Tools Alone

If you just register `fredmeyer:search_products` and `fredmeyer:add_to_cart` into the registry, the agent has no idea:

- That it should search locations first, set a preferred store, then search products
- That product search needs a location_id for pricing
- That add_to_cart requires OAuth and the user needs to visit a URL
- That local cart tracking drifts from the real Kroger cart
- The optimal workflow for building a grocery list from a recipe

That's what the SKILL.md provides -- the procedural knowledge that makes the tools useful.

## How It Works

### What a Skill Package Contains

```
skills/fredmeyer/
  fredmeyer.go          # Config, New(), Register() -- the Go tools
  args.go               # Typed arg structs
  skill.md              # Hermes-compatible skill document (embedded)
  agent.md              # Agent definition fragment (embedded)
  internal/
    kroger/client.go    # Domain logic
    cart/cart.go
    auth/pkce.go
```

### The SKILL.md (embedded in the Go binary)

```markdown
---
name: fred_meyer_grocery
description: Search Fred Meyer products, manage your grocery cart, find stores, and compare prices using the Kroger API.
tags: [grocery, shopping, fredmeyer, kroger, cart]
metadata:
  author: chonkskill
  version: "1.0"
  requires_tools:
    - fredmeyer:search_locations
    - fredmeyer:search_products
    - fredmeyer:get_product_details
    - fredmeyer:set_preferred_location
    - fredmeyer:add_to_cart
    - fredmeyer:view_cart
    - fredmeyer:start_authentication
    - fredmeyer:complete_authentication
---

# Fred Meyer Grocery Shopping

You can help users browse Fred Meyer stores, search for products with
real-time pricing, and manage their shopping cart through the Kroger API.

## Setup Workflow

Before searching products, you need a store location for accurate
pricing and availability:

1. Ask the user for their zip code (or use `get_zip_code` for the configured default)
2. Call `fredmeyer:search_locations` with the zip code
3. Present the nearest Fred Meyer stores (chain "FRED" or "FRED MEYER STORES")
4. Call `fredmeyer:set_preferred_location` with the chosen store ID
5. Now product searches will include store-specific pricing

## Searching for Products

- Always include the location_id (preferred location) for pricing
- The `term` parameter is a full-text search -- be specific
- Results include: product name, brand, UPC, price (regular + promo), aisle location, size
- Use `fredmeyer:get_product_details` for full info on a specific product

## Adding to Cart (Requires Authentication)

Cart operations require the user to authenticate with their Kroger account:

1. Call `fredmeyer:start_authentication` -- this returns an authorization URL
2. Ask the user to visit that URL in their browser and sign in
3. After signing in, Kroger redirects to a callback URL -- ask the user to paste it back
4. Call `fredmeyer:complete_authentication` with the redirect URL
5. Now `fredmeyer:add_to_cart` will work

Important limitations:
- The Kroger API can ADD items but cannot remove them or read the real cart
- `fredmeyer:view_cart` shows locally tracked items only -- it can drift from reality
- `fredmeyer:remove_from_cart` and `fredmeyer:clear_cart` are local-only operations
- There is no checkout API -- the user must complete checkout on fredmeyer.com

## Building a Grocery List from a Recipe

1. Ask for the recipe or URL
2. Extract the ingredient list
3. For each ingredient, call `fredmeyer:search_products`
4. Present options with prices, let the user choose
5. Use `fredmeyer:add_to_cart` for each selected item (requires auth)
6. Show cart summary with `fredmeyer:view_cart`
7. Remind user to complete checkout on fredmeyer.com or the Kroger app

## Price Comparison Tips

- Search the same term at different stores by passing different location_ids
- Check `promo` price field for sale items
- The `size` field helps compare unit pricing
- Use `brand` filter to narrow results

## Troubleshooting

- "Not authenticated" error: run `fredmeyer:start_authentication`
- No prices in results: make sure location_id is set
- Token expired: tokens last 30 minutes, the system auto-refreshes, but if refresh fails call `fredmeyer:start_authentication` again
```

### The agent.md (optional agent definition fragment)

```markdown
---
slug: grocery_assistant
display_name: Grocery Shopping Assistant
description: Browse Fred Meyer, search products, compare prices, and manage your shopping cart.
agent_type: chat
tool_names:
  - fredmeyer:search_locations
  - fredmeyer:get_location_details
  - fredmeyer:set_preferred_location
  - fredmeyer:get_preferred_location
  - fredmeyer:search_products
  - fredmeyer:get_product_details
  - fredmeyer:add_to_cart
  - fredmeyer:view_cart
  - fredmeyer:remove_from_cart
  - fredmeyer:clear_cart
  - fredmeyer:mark_order_placed
  - fredmeyer:view_order_history
  - fredmeyer:start_authentication
  - fredmeyer:complete_authentication
  - fredmeyer:test_authentication
  - fredmeyer:list_chains
  - fredmeyer:list_departments
  - fredmeyer:get_user_profile
  - fredmeyer:get_zip_code
---
```

### Go Side: Everything Embedded

```go
package fredmeyer

import "embed"

//go:embed skill.md
var SkillContent string

//go:embed agent.md
var AgentContent string
```

## The Register() Function

Ships tools, skill, and agent definition in one call:

```go
package fredmeyer

import (
    "context"
    "embed"

    "github.com/iconidentify/chonkskill/pkg/skill"
)

//go:embed skill.md
var SkillContent string

//go:embed agent.md
var AgentContent string

type Config struct {
    ClientID     string `env:"KROGER_CLIENT_ID"`
    ClientSecret string `env:"KROGER_CLIENT_SECRET"`
    RedirectURI  string `env:"KROGER_REDIRECT_URI"  default:"http://localhost:8000/callback"`
    BaseURL      string `env:"KROGER_API_BASE"       default:"https://api.kroger.com/v1"`
    ZipCode      string `env:"KROGER_USER_ZIP_CODE"`
    DataDir      string `env:"KROGER_DATA_DIR"       default:"."`
}

func New(cfg Config) (*skill.Skill, error) {
    client := newKrogerClient(cfg)
    am := newAuthManager(client, cfg)
    lc := newLocalCart(cfg)

    s := skill.New(skill.Definition{
        Name:         "fredmeyer",
        Description:  "Fred Meyer grocery shopping",
        SkillContent: SkillContent,     // the SKILL.md
        AgentContent: AgentContent,     // the agent definition
        Tags:         []string{"grocery", "shopping", "fredmeyer", "kroger"},
    })

    skill.AddTool(s, "search_locations", "Find Fred Meyer/Kroger stores near a zip code",
        func(ctx context.Context, args SearchLocationsArgs) (string, error) {
            // ...
        })

    skill.AddTool(s, "search_products", "Search products with pricing and availability",
        func(ctx context.Context, args SearchProductsArgs) (string, error) {
            // ...
        })

    // ... all other tools

    return s, nil
}

func Register(reg skill.Registry, cfg Config) error {
    s, err := New(cfg)
    if err != nil {
        return err
    }
    s.Register(reg)
    return nil
}
```

## Chonkbase Integration

### The Adapter (~30 lines, lives in chonkbase)

```go
// internal/skill/adapter.go
package skilladapter

import (
    "github.com/iconidentify/chonkbase/internal/chatengine"
    "github.com/iconidentify/chonkbase/internal/domain"
    "github.com/iconidentify/chonkbase/internal/repository"
    "github.com/iconidentify/chonkbase/pkg/ai"
    "github.com/iconidentify/chonkskill/pkg/skill"
)

type ChonkbaseRegistry struct {
    Tools  *chatengine.ExpertToolRegistry
    Skills repository.AgentSkillRepository
}

func (r *ChonkbaseRegistry) RegisterTool(def skill.ToolDef, handler skill.Handler) {
    r.Tools.Register(ai.ToolDefinition{
        Name:        def.Name,
        Description: def.Description,
        InputSchema: def.InputSchema,
    }, chatengine.ExpertToolHandler(handler))
}

func (r *ChonkbaseRegistry) RegisterSkill(name, description, content string, tags []string) error {
    existing, _ := r.Skills.GetByName(context.Background(), name)
    if existing != nil {
        // Update if version changed
        if existing.Content != content {
            existing.Content = content
            existing.Description = description
            existing.Tags = tags
            return r.Skills.Update(context.Background(), existing)
        }
        return nil
    }
    return r.Skills.Create(context.Background(), &domain.AgentSkill{
        Name:           name,
        Description:    description,
        Content:        content,
        Tags:           tags,
        CreatedByAgent: "chonkskill",
        IsActive:       true,
    })
}
```

### Startup (3 lines per skill)

```go
// internal/handler/router.go, during startup

import (
    skilladapter "github.com/iconidentify/chonkbase/internal/skill"
    "github.com/iconidentify/chonkskill/skills/fredmeyer"
)

reg := &skilladapter.ChonkbaseRegistry{
    Tools:  toolRegistry,
    Skills: deps.AgentSkillRepo,
}

fredmeyer.Register(reg, fredmeyer.Config{
    ClientID:     os.Getenv("KROGER_CLIENT_ID"),
    ClientSecret: os.Getenv("KROGER_CLIENT_SECRET"),
    RedirectURI:  os.Getenv("KROGER_REDIRECT_URI"),
    ZipCode:      os.Getenv("KROGER_USER_ZIP_CODE"),
})
```

This single call:
1. Registers all 20 fredmeyer tools into ExpertToolRegistry
2. Upserts the SKILL.md into `chonk_agent_skills` table
3. The skill shows up in `list_skills` / `get_skill` for any agent
4. The agent definition fragment can be used to create/update agent_definitions

### What the Agent Sees

After registration, when an agent calls `list_skills`:

```
## Available Skills
- fred_meyer_grocery: Search Fred Meyer products, manage your grocery cart,
  find stores, and compare prices using the Kroger API. [grocery, shopping, fredmeyer, kroger]
```

When it calls `get_skill("fred_meyer_grocery")`, it gets the full SKILL.md with workflows, tips, and troubleshooting. Progressive loading -- the agent only loads the full skill content when it needs it.

## Module Structure

```
github.com/iconidentify/chonkskill/
  go.mod
  go.sum

  pkg/
    skill/
      skill.go         # Skill, Registry, ToolDef, AddTool[T], Definition
      schema.go        # JSON schema generation from Go struct tags
    mcprunner/
      runner.go        # Standalone MCP server (HTTP + stdio)
      jsonrpc.go       # JSON-RPC 2.0 handler

  skills/
    fredmeyer/
      fredmeyer.go     # Config, New(), Register()
      args.go          # Typed arg structs
      skill.md         # Hermes-compatible SKILL.md (go:embed)
      agent.md         # Agent definition fragment (go:embed)
      internal/
        kroger/client.go
        cart/cart.go
        auth/pkce.go
    # future skills:
    # banking/
    # calendar/
    # slack/

  cmd/
    chonkskill/
      main.go          # CLI: serve, test, list
```

## The Full Lifecycle

### Shipping a New Skill

```
1. Create skills/myskill/ directory
2. Write skill.md           -- procedural knowledge, workflows, tips
3. Write agent.md           -- tool whitelist, agent type, description
4. Write myskill.go         -- Config, New(), Register(), tool handlers
5. Write internal/           -- domain-specific API clients, logic
6. Add to cmd/chonkskill    -- one line in the catalog
7. Tag release

Chonkbase team:
8. go get github.com/iconidentify/chonkskill@v1.x.0
9. Add Register() call in startup
10. Add env vars for skill config
11. Rebuild, deploy
```

### What Happens at Startup

```
chonkbase starts
  |
  +-- fredmeyer.Register(reg, cfg)
  |     |
  |     +-- Creates Kroger API client, auth manager, local cart
  |     +-- Registers 20 tools into ExpertToolRegistry
  |     +-- Upserts skill.md into chonk_agent_skills table
  |     +-- (optionally) upserts agent definition
  |
  +-- Agent receives user message
  |     |
  |     +-- System prompt includes "Available Skills: fred_meyer_grocery: ..."
  |     +-- Agent calls get_skill("fred_meyer_grocery") to load full workflow
  |     +-- Agent follows the SKILL.md procedure
  |     +-- Agent calls fredmeyer:search_locations, fredmeyer:search_products, etc.
  |     +-- Tools execute via ExpertToolRegistry handlers (in-process, zero overhead)
```

### Standalone MCP Mode (dev/testing)

```bash
# Serve fredmeyer as MCP over stdio (for Claude Desktop)
chonkskill serve fredmeyer --stdio

# Serve over HTTP (for testing with curl or chonkbase's MCPClient)
chonkskill serve fredmeyer --port 8080

# Run API integration tests
chonkskill test fredmeyer

# List all available skills
chonkskill list
```

In standalone mode, the SKILL.md is served as an MCP resource, and the tools are served as MCP tools. Same code, different transport.

## Why This Design

| Concern | How It's Handled |
|---|---|
| No idle services | Skills compile into chonkbase, no separate processes |
| Agent knows how to use tools | SKILL.md provides procedural knowledge via Hermes pattern |
| Progressive loading | Agent sees skill summaries, loads full content on demand |
| Tools work everywhere | Same handlers power in-process registry AND standalone MCP |
| Easy to add skills | One directory, one Register() call |
| Skills can evolve | Agents can update skills via `update_skill` based on experience |
| Version coherence | One Go module, atomic versioning |
| Existing infra reused | ExpertToolRegistry, AgentSkillRepository, SkillLoader -- all exist |

## Open Questions

1. **Skill versioning**: When chonkskill updates a SKILL.md and chonkbase redeploys, should it overwrite agent-modified versions in the DB? Or should embedded content be treated as a "seed" that agents can evolve?

2. **Per-user state**: Skills like fredmeyer need per-user OAuth tokens. Should the skill manage file-based state, or should chonkbase provide a user-scoped key-value store accessible via context?

3. **Skill dependencies**: The `process_documentation` skill references `process_documentation_templates` via `get_skill()`. Should chonkskill support declaring skill dependencies so they're always co-registered?

4. **Agent definition management**: Should `Register()` auto-create agent definitions in the DB, or just provide the fragment for manual setup? Auto-create risks overwriting admin customizations.
