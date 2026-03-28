package skill

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// ConfigScope determines where a config value is managed.
type ConfigScope string

const (
	// ScopeGlobal is for admin-set config shared across all users (API keys, secrets, base URLs).
	ScopeGlobal ConfigScope = "global"
	// ScopeUser is for per-user preferences (ZIP codes, default handles, personal API keys).
	ScopeUser ConfigScope = "user"
)

// ConfigFieldType is the value type for a config field.
type ConfigFieldType string

const (
	FieldString ConfigFieldType = "string"
	FieldInt    ConfigFieldType = "int"
	FieldBool   ConfigFieldType = "bool"
)

// ConfigField describes a single configuration parameter for a skill.
// Skills declare these statically. Chonkbase reads them to auto-generate
// settings UI and validate that required config is present.
type ConfigField struct {
	Name        string          `json:"name"`                  // Machine key, e.g. "api_key"
	Label       string          `json:"label"`                 // Human label for UI, e.g. "xAI API Key"
	Description string          `json:"description,omitempty"` // Help text shown in UI
	Type        ConfigFieldType `json:"type"`                  // string, int, bool
	Scope       ConfigScope     `json:"scope"`                 // global or user
	Required    bool            `json:"required"`              // Must be set for skill to function
	Secret      bool            `json:"secret"`                // Mask in UI, encrypt at rest
	Default     string          `json:"default,omitempty"`     // Default value (string-encoded)
	EnvVar      string          `json:"env_var,omitempty"`     // Env var name for standalone/fallback
}

// ConfigSchema is the full config declaration for a skill.
// It is static -- declared once at definition time and never changes.
type ConfigSchema struct {
	Fields []ConfigField `json:"fields"`
}

// ConfigValues is a string-keyed map of resolved config values.
// Chonkbase populates this by merging database settings, env vars, and defaults,
// then injects it into the request context for per-user config.
type ConfigValues map[string]string

// Resolve populates ConfigValues from environment variables and defaults.
// This is the standalone/MCP mode resolver. Chonkbase uses its own resolver
// that also reads from the database before falling back to env vars.
func (s ConfigSchema) Resolve() ConfigValues {
	vals := make(ConfigValues)
	for _, f := range s.Fields {
		if f.EnvVar != "" {
			if v := os.Getenv(f.EnvVar); v != "" {
				vals[f.Name] = v
				continue
			}
		}
		if f.Default != "" {
			vals[f.Name] = f.Default
		}
	}
	return vals
}

// ResolveWith populates ConfigValues from an external provider first (e.g.
// chonkbase database), then falls back to env vars, then defaults.
// The provider function is called once per field.
func (s ConfigSchema) ResolveWith(provider func(fieldName string) string) ConfigValues {
	vals := make(ConfigValues)
	for _, f := range s.Fields {
		// Priority: provider > env var > default
		if provider != nil {
			if v := provider(f.Name); v != "" {
				vals[f.Name] = v
				continue
			}
		}
		if f.EnvVar != "" {
			if v := os.Getenv(f.EnvVar); v != "" {
				vals[f.Name] = v
				continue
			}
		}
		if f.Default != "" {
			vals[f.Name] = f.Default
		}
	}
	return vals
}

// Validate checks that all required fields have values.
// Returns nil if all required fields are present.
// Returns a MissingConfigError listing what's missing.
func (s ConfigSchema) Validate(vals ConfigValues) error {
	var missing []ConfigField
	for _, f := range s.Fields {
		if f.Required && vals[f.Name] == "" {
			missing = append(missing, f)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &MissingConfigError{Missing: missing}
}

// MissingConfigError is returned when required config fields are not set.
// It carries the full field metadata so callers (chonkbase UI, CLI) can
// render helpful messages telling the user exactly what to configure.
type MissingConfigError struct {
	Missing []ConfigField
}

func (e *MissingConfigError) Error() string {
	names := make([]string, len(e.Missing))
	for i, f := range e.Missing {
		if f.EnvVar != "" {
			names[i] = fmt.Sprintf("%s (env: %s)", f.Label, f.EnvVar)
		} else {
			names[i] = f.Label
		}
	}
	return "missing required config: " + strings.Join(names, ", ")
}

type configContextKey struct{}

// WithConfigValues attaches resolved config values to a context.
// Chonkbase middleware calls this before tool handlers execute.
func WithConfigValues(ctx context.Context, values ConfigValues) context.Context {
	return context.WithValue(ctx, configContextKey{}, values)
}

// ConfigFromContext retrieves config values from context.
// Returns nil if no config was injected.
func ConfigFromContext(ctx context.Context) ConfigValues {
	v, _ := ctx.Value(configContextKey{}).(ConfigValues)
	return v
}
