# chonkskill

A Go library for building AI agent skills. Each skill bundles tool handlers, procedural knowledge, and agent configuration into a single importable package.

Skills can be compiled directly into [chonkbase](https://github.com/iconidentify/chonkbase) as a library, or run standalone as an MCP server for local development and testing.

## Quick Start

### As a library (compiled into chonkbase)

```go
import "github.com/iconidentify/chonkskill/skills/fredmeyer"

// One call registers 20 tools + skill knowledge into your registry.
fredmeyer.Register(registry, fredmeyer.ConfigFromEnv())
```

### As a standalone MCP server

```bash
# Over stdio (for Claude Desktop, Claude Code, or any MCP client)
chonkskill serve fredmeyer --stdio

# Over HTTP
chonkskill serve fredmeyer --port 8080

# List available skills
chonkskill list

# Run integration tests against the live API
chonkskill test fredmeyer
```

## What a Skill Contains

Each skill ships three things:

| Component | What it is | Why it matters |
|-----------|-----------|----------------|
| **Tool handlers** | Go functions that call external APIs | The agent can take actions |
| **skill.md** | Procedural knowledge -- workflows, tips, pitfalls | The agent knows *how* to use the tools well |
| **agent.md** | Tool whitelist, system prompt fragment, guardrails | Ready-made agent definition |

A skill is not just a bag of API wrappers. The `skill.md` teaches the agent multi-step workflows (like building a grocery list from a recipe), decision-making strategies (like price comparison by unit cost), and known pitfalls to avoid.

## Available Skills

### fredmeyer

Fred Meyer / Kroger grocery shopping. 20 tools covering:

- Store search and selection
- Product search with real-time, store-specific pricing
- Cart management (add items, view, track orders)
- OAuth2 authentication with Kroger accounts
- Chain and department lookup

The `skill.md` includes workflows for:

- Recipe-to-cart conversion with quantity intelligence
- Multi-recipe ingredient aggregation for meal planning
- Price comparison across brands with unit price calculation
- Budget-aware meal planning
- Seasonal produce awareness
- Dietary filtering
- Substitution logic with a 5-tier priority hierarchy
- Recurring item replenishment from order history

**Requirements:** Free API credentials from [developer.kroger.com](https://developer.kroger.com)

```bash
export KROGER_CLIENT_ID=your_client_id
export KROGER_CLIENT_SECRET=your_client_secret
export KROGER_USER_ZIP_CODE=98052
```

## Architecture

```
github.com/iconidentify/chonkskill/
  pkg/
    skill/          Framework -- Registry interface, AddTool[T], schema generation
    mcprunner/      Standalone MCP server (stdio + HTTP)
  skills/
    fredmeyer/      First skill (more to come)
  cmd/
    chonkskill/     CLI for serving and testing skills
```

### The Framework (pkg/skill)

The framework is small (~170 lines) and defines the contract between skills and their host:

```go
// Registry is what skills register into.
type Registry interface {
    RegisterTool(def ToolDef, handler Handler)
    RegisterSkill(name, description, content string, tags []string) error
}

// Handler executes a tool call.
type Handler func(ctx context.Context, input map[string]any) (string, error)
```

`AddTool[T]` provides typed tool registration. JSON schemas are generated automatically from Go struct tags:

```go
type SearchArgs struct {
    Term  string `json:"term" jsonschema:"Search term"`
    Limit int    `json:"limit,omitempty" jsonschema:"Max results"`
}

skill.AddTool(s, "search", "Search for products", func(ctx context.Context, args SearchArgs) (string, error) {
    // args is already typed and validated
    return results, nil
})
```

### Two Integration Modes

**Library mode** -- tools register directly into the host's tool registry. No network, no separate process, no overhead. This is the default for chonkbase.

**MCP mode** -- the same tools are served over the Model Context Protocol via stdio or HTTP. Used for local development with Claude Desktop, testing, or integration with other MCP-compatible platforms.

Both modes use the exact same tool handlers. The framework adapts.

## Chonkbase Integration

### 1. Add the dependency

```bash
go get github.com/iconidentify/chonkskill@latest
```

### 2. Create an adapter (~20 lines, one-time setup)

```go
// internal/skill/adapter.go
package skilladapter

import (
    "context"
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
    ctx := context.Background()
    existing, _ := r.Skills.GetByName(ctx, name)
    if existing != nil {
        if existing.Content != content {
            existing.Content = content
            existing.Description = description
            existing.Tags = tags
            return r.Skills.Update(ctx, existing)
        }
        return nil
    }
    return r.Skills.Create(ctx, &domain.AgentSkill{
        Name:           name,
        Description:    description,
        Content:        content,
        Tags:           tags,
        CreatedByAgent: "chonkskill",
        IsActive:       true,
    })
}
```

### 3. Register skills at startup

```go
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
})
```

### 4. Add an agent definition

```sql
INSERT INTO agent_definitions (slug, display_name, agent_type, tool_names, is_active)
VALUES (
    'grocery_assistant',
    'Grocery Shopping Assistant',
    'chat',
    ARRAY[
        'fredmeyer:search_locations',
        'fredmeyer:search_products',
        'fredmeyer:get_product_details',
        'fredmeyer:set_preferred_location',
        'fredmeyer:add_to_cart',
        'fredmeyer:view_cart',
        'fredmeyer:start_authentication',
        'fredmeyer:complete_authentication'
    ],
    true
);
```

## Writing a New Skill

### 1. Create the directory

```
skills/myskill/
  myskill.go        Config, New(), Register()
  args.go           Typed argument structs
  skill.md          Procedural knowledge (go:embed)
  agent.md          Agent definition (go:embed)
  internal/         Domain-specific logic
```

### 2. Define tools

```go
package myskill

import (
    "context"
    _ "embed"
    "github.com/iconidentify/chonkskill/pkg/skill"
)

//go:embed skill.md
var SkillContent string

//go:embed agent.md
var AgentContent string

type Config struct {
    APIKey string
}

func New(cfg Config) (*skill.Skill, error) {
    s := skill.New(skill.Definition{
        Name:         "myskill",
        Description:  "What this skill does",
        SkillContent: SkillContent,
        AgentContent: AgentContent,
        Tags:         []string{"tag1", "tag2"},
    })

    skill.AddTool(s, "do_thing", "Does the thing",
        func(ctx context.Context, args DoThingArgs) (string, error) {
            // your logic here
            return "result", nil
        })

    return s, nil
}

func Register(reg skill.Registry, cfg Config) error {
    s, err := New(cfg)
    if err != nil { return err }
    return s.Register(reg)
}
```

### 3. Write skill.md

The `skill.md` is what makes a skill more than just API wrappers. It should include:

- **Quick reference table** mapping tasks to tool names
- **Setup procedure** for first-time use
- **Workflows** for multi-step operations (numbered steps, decision points)
- **Domain knowledge** the agent needs (seasonal produce, unit pricing, etc.)
- **Pitfalls** and troubleshooting for known failure modes

Write it as instructions to the agent, not documentation for a human. Use imperative form. Explain the *why* behind non-obvious steps so the agent can adapt when situations vary.

### 4. Add to the CLI catalog

In `cmd/chonkskill/main.go`:

```go
var catalog = map[string]func() (*skill.Skill, error){
    "fredmeyer": func() (*skill.Skill, error) {
        return fredmeyer.New(fredmeyer.ConfigFromEnv())
    },
    "myskill": func() (*skill.Skill, error) {
        return myskill.New(myskill.ConfigFromEnv())
    },
}
```

### 5. Test

```bash
chonkskill test myskill       # API integration tests
chonkskill serve myskill --stdio  # Manual testing via MCP
```

## Claude Desktop Configuration

To use a skill with Claude Desktop, add it to your MCP server config:

```json
{
  "mcpServers": {
    "chonkskill": {
      "command": "go",
      "args": ["run", "github.com/iconidentify/chonkskill/cmd/chonkskill@latest", "serve", "fredmeyer", "--stdio"],
      "env": {
        "KROGER_CLIENT_ID": "your_client_id",
        "KROGER_CLIENT_SECRET": "your_client_secret",
        "KROGER_USER_ZIP_CODE": "98052"
      }
    }
  }
}
```

## License

MIT
