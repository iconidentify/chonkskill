// Package skill provides the framework for building chonkskill skills.
// Skills register typed tool handlers and embed procedural knowledge (SKILL.md)
// that teaches agents how to use the tools effectively.
//
// A skill can be consumed two ways:
//   - Compiled into chonkbase via Register() into an ExpertToolRegistry adapter
//   - Run standalone as an MCP server via the mcprunner package
package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// Registry is the interface skills register tools into.
// Chonkbase implements this by wrapping ExpertToolRegistry.
// The standalone MCP runner implements it with a JSON-RPC server.
type Registry interface {
	RegisterTool(def ToolDef, handler Handler)
	RegisterSkill(name, description, content string, tags []string) error
}

// ToolDef describes a tool's name, description, and input schema.
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// Handler executes a tool call and returns the result as a string.
type Handler func(ctx context.Context, input map[string]any) (string, error)

// Definition provides metadata for a skill.
type Definition struct {
	Name         string
	Description  string
	SkillContent string   // Full SKILL.md content (go:embed)
	AgentContent string   // Full agent.md content (go:embed)
	Tags         []string
}

// Skill is a named collection of tools with embedded procedural knowledge.
type Skill struct {
	Def      Definition
	Tools    []ToolDef
	Handlers map[string]Handler
}

// New creates a new skill with the given definition.
func New(def Definition) *Skill {
	return &Skill{
		Def:      def,
		Handlers: make(map[string]Handler),
	}
}

// Register dumps all tools and the skill content into a registry.
func (s *Skill) Register(reg Registry) error {
	for _, def := range s.Tools {
		reg.RegisterTool(def, s.Handlers[def.Name])
	}
	if s.Def.SkillContent != "" {
		if err := reg.RegisterSkill(s.Def.Name, s.Def.Description, s.Def.SkillContent, s.Def.Tags); err != nil {
			return fmt.Errorf("registering skill content: %w", err)
		}
	}
	return nil
}

// AddTool registers a tool with typed arguments. The JSON schema is generated
// from the struct's json tags. The tool name is automatically prefixed with
// the skill name (e.g., "fredmeyer:search_products").
func AddTool[T any](s *Skill, name, description string, fn func(ctx context.Context, args T) (string, error)) {
	schema := SchemaFrom[T]()
	qualifiedName := s.Def.Name + ":" + name

	def := ToolDef{
		Name:        qualifiedName,
		Description: description,
		InputSchema: schema,
	}
	s.Tools = append(s.Tools, def)
	s.Handlers[qualifiedName] = func(ctx context.Context, input map[string]any) (string, error) {
		data, err := json.Marshal(input)
		if err != nil {
			return "", fmt.Errorf("marshaling input: %w", err)
		}
		var args T
		if err := json.Unmarshal(data, &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
		return fn(ctx, args)
	}
}

// SchemaFrom generates a JSON schema from a Go struct's json tags.
func SchemaFrom[T any]() map[string]any {
	var zero T
	t := reflect.TypeOf(zero)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	properties := make(map[string]any)
	var required []string

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		parts := strings.Split(jsonTag, ",")
		jsonName := parts[0]
		isOptional := false
		for _, p := range parts[1:] {
			if p == "omitempty" {
				isOptional = true
			}
		}

		prop := map[string]any{}

		switch field.Type.Kind() {
		case reflect.String:
			prop["type"] = "string"
		case reflect.Int, reflect.Int32, reflect.Int64:
			prop["type"] = "integer"
		case reflect.Float32, reflect.Float64:
			prop["type"] = "number"
		case reflect.Bool:
			prop["type"] = "boolean"
		case reflect.Slice:
			prop["type"] = "array"
			switch field.Type.Elem().Kind() {
			case reflect.String:
				prop["items"] = map[string]any{"type": "string"}
			default:
				prop["items"] = map[string]any{"type": "object"}
			}
		default:
			prop["type"] = "string"
		}

		if desc := field.Tag.Get("jsonschema"); desc != "" {
			prop["description"] = desc
		}

		properties[jsonName] = prop
		if !isOptional {
			required = append(required, jsonName)
		}
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}
