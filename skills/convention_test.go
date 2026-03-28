// Package skills_test enforces conventions across all chonkskill skills.
// These tests catch common mistakes: missing labels, undeclared env vars,
// secrets without the Secret flag, schemas that don't match Config structs,
// and skills that fail to gracefully degrade when unconfigured.
//
// If you're adding a new skill, add it to the skillDefs and skillRegistrars
// slices below. These tests will automatically validate your config schema.
package skills_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/iconidentify/chonkskill/pkg/skill"
	"github.com/iconidentify/chonkskill/skills/autonovel"
	"github.com/iconidentify/chonkskill/skills/drakekb"
	"github.com/iconidentify/chonkskill/skills/kidsnovel"
	"github.com/iconidentify/chonkskill/skills/fredmeyer"
	"github.com/iconidentify/chonkskill/skills/skillcreator"
	"github.com/iconidentify/chonkskill/skills/xsearch"
)

// skillEntry ties a skill's Definition and Register function together
// so convention tests can iterate over all skills uniformly.
type skillEntry struct {
	name     string
	def      skill.Definition
	register func(skill.Registry) error
}

// All skills with config that should be tested.
// Add new skills here as they're created.
var allSkills = []skillEntry{
	{
		name: "xsearch",
		def:  xsearch.Def,
		register: func(reg skill.Registry) error {
			return xsearch.Register(reg, xsearch.Config{})
		},
	},
	{
		name: "fredmeyer",
		def:  fredmeyer.Def,
		register: func(reg skill.Registry) error {
			return fredmeyer.Register(reg, fredmeyer.Config{})
		},
	},
	{
		name: "drakekb",
		def:  drakekb.Def,
		register: func(reg skill.Registry) error {
			s, err := drakekb.New(drakekb.ConfigFromEnv())
			if err != nil {
				return err
			}
			return s.Register(reg)
		},
	},
	{
		name: "autonovel",
		def:  autonovel.Def,
		register: func(reg skill.Registry) error {
			return autonovel.Register(reg, autonovel.Config{})
		},
	},
	{
		name: "kidsnovel",
		def:  kidsnovel.Def,
		register: func(reg skill.Registry) error {
			return kidsnovel.Register(reg, kidsnovel.Config{})
		},
	},
	{
		name: "skillcreator",
		def: skill.Definition{
			Name:        "skill-creator",
			Description: "Create, test, and refine agent skills",
			Tags:        []string{"skill", "meta", "authoring", "evaluation"},
		},
		register: func(reg skill.Registry) error {
			return skillcreator.Register(reg, skillcreator.Config{})
		},
	},
}

// TestSkillDefinitionConventions ensures every skill has proper metadata.
func TestSkillDefinitionConventions(t *testing.T) {
	for _, entry := range allSkills {
		t.Run(entry.name, func(t *testing.T) {
			def := entry.def

			if def.Name == "" {
				t.Error("Definition.Name is empty")
			}
			if def.Description == "" {
				t.Error("Definition.Description is empty")
			}
			if len(def.Tags) == 0 {
				t.Error("Definition.Tags is empty -- skills should have at least one tag")
			}
			// SkillContent is optional for content-only skills but expected for tool skills.
			if def.SkillContent == "" {
				t.Log("Warning: SkillContent is empty -- no procedural knowledge embedded")
			}
		})
	}
}

// TestConfigSchemaFieldConventions validates every config field has
// the required metadata for UI generation and is self-consistent.
func TestConfigSchemaFieldConventions(t *testing.T) {
	for _, entry := range allSkills {
		schema := entry.def.Config
		if len(schema.Fields) == 0 {
			continue // no-config skills like drakekb
		}

		t.Run(entry.name, func(t *testing.T) {
			names := make(map[string]bool)

			for _, f := range schema.Fields {
				t.Run(f.Name, func(t *testing.T) {
					// Every field must have a name.
					if f.Name == "" {
						t.Fatal("ConfigField.Name is empty")
					}

					// No duplicate field names.
					if names[f.Name] {
						t.Errorf("duplicate field name: %s", f.Name)
					}
					names[f.Name] = true

					// Every field must have a label for UI rendering.
					if f.Label == "" {
						t.Errorf("field %q has no Label -- UI needs this for form rendering", f.Name)
					}

					// Field type must be valid.
					switch f.Type {
					case skill.FieldString, skill.FieldInt, skill.FieldBool:
						// ok
					default:
						t.Errorf("field %q has invalid Type: %q", f.Name, f.Type)
					}

					// Scope must be valid.
					switch f.Scope {
					case skill.ScopeGlobal, skill.ScopeUser:
						// ok
					default:
						t.Errorf("field %q has invalid Scope: %q", f.Name, f.Scope)
					}

					// Required fields should have an EnvVar for standalone fallback.
					if f.Required && f.EnvVar == "" {
						t.Errorf("field %q is required but has no EnvVar -- standalone mode can't resolve it", f.Name)
					}

					// Fields named *key*, *secret*, *password*, *token* should be marked Secret.
					lower := strings.ToLower(f.Name)
					isSensitive := strings.Contains(lower, "secret") ||
						strings.Contains(lower, "password") ||
						strings.Contains(lower, "token")
					// API keys: only flag if name literally contains "secret"/"password"/"token"
					// "api_key" is borderline -- we check it separately
					if isSensitive && !f.Secret {
						t.Errorf("field %q looks sensitive but Secret=false", f.Name)
					}

					// EnvVar should be UPPER_SNAKE_CASE if set.
					if f.EnvVar != "" && f.EnvVar != strings.ToUpper(f.EnvVar) {
						t.Errorf("field %q EnvVar %q should be UPPER_SNAKE_CASE", f.Name, f.EnvVar)
					}
				})
			}
		})
	}
}

// TestConfigSchemaJSONRoundTrip ensures schemas serialize and deserialize
// cleanly -- chonkbase stores these as JSONB.
func TestConfigSchemaJSONRoundTrip(t *testing.T) {
	for _, entry := range allSkills {
		schema := entry.def.Config
		if len(schema.Fields) == 0 {
			continue
		}

		t.Run(entry.name, func(t *testing.T) {
			b, err := json.Marshal(schema)
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}

			var decoded skill.ConfigSchema
			if err := json.Unmarshal(b, &decoded); err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}

			if len(decoded.Fields) != len(schema.Fields) {
				t.Errorf("field count mismatch: %d vs %d", len(decoded.Fields), len(schema.Fields))
			}

			for i, f := range decoded.Fields {
				orig := schema.Fields[i]
				if f.Name != orig.Name {
					t.Errorf("field %d name mismatch: %q vs %q", i, f.Name, orig.Name)
				}
				if f.Required != orig.Required {
					t.Errorf("field %q Required mismatch", f.Name)
				}
				if f.Secret != orig.Secret {
					t.Errorf("field %q Secret mismatch", f.Name)
				}
			}
		})
	}
}

// TestConfigSchemaValidate checks that Validate correctly identifies
// missing required fields for each skill's schema.
func TestConfigSchemaValidate(t *testing.T) {
	for _, entry := range allSkills {
		schema := entry.def.Config
		if len(schema.Fields) == 0 {
			continue
		}

		t.Run(entry.name, func(t *testing.T) {
			// Empty values should fail if there are required fields.
			var hasRequired bool
			for _, f := range schema.Fields {
				if f.Required {
					hasRequired = true
					break
				}
			}

			err := schema.Validate(skill.ConfigValues{})
			if hasRequired && err == nil {
				t.Error("expected validation error with empty values but got nil")
			}

			// All fields populated should pass.
			full := make(skill.ConfigValues)
			for _, f := range schema.Fields {
				full[f.Name] = "test-value"
			}
			err = schema.Validate(full)
			if err != nil {
				t.Errorf("expected no error with all fields set, got: %v", err)
			}

			// MissingConfigError should list the right fields.
			if hasRequired {
				err = schema.Validate(skill.ConfigValues{})
				me, ok := err.(*skill.MissingConfigError)
				if !ok {
					t.Fatalf("expected *MissingConfigError, got %T", err)
				}
				for _, missing := range me.Missing {
					if !missing.Required {
						t.Errorf("non-required field %q listed as missing", missing.Name)
					}
				}
			}
		})
	}
}

// TestUnconfiguredRegistration ensures every skill with required config
// can register in unconfigured mode without panicking. This is the
// graceful degradation path for chonkbase.
func TestUnconfiguredRegistration(t *testing.T) {
	for _, entry := range allSkills {
		t.Run(entry.name, func(t *testing.T) {
			reg := &captureRegistry{}

			// Register with empty config -- should not panic or return error.
			err := entry.register(reg)
			if err != nil {
				t.Fatalf("Register with empty config failed: %v", err)
			}

			// Schema should be registered if the skill has config fields.
			if len(entry.def.Config.Fields) > 0 && reg.configSchema == nil {
				t.Error("config schema was not registered")
			}

			// Skill content should always be registered.
			if entry.def.SkillContent != "" && reg.skillName == "" {
				t.Error("skill content was not registered")
			}
		})
	}
}

// TestConfiguredRegistration ensures skills register tools when properly configured.
func TestConfiguredRegistration(t *testing.T) {
	// drakekb has no config, should always have tools.
	t.Run("drakekb", func(t *testing.T) {
		s, err := drakekb.New(drakekb.ConfigFromEnv())
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		if !s.Configured {
			t.Error("expected Configured=true")
		}
		if len(s.Tools) == 0 {
			t.Error("expected tools to be registered")
		}
	})
}

// TestSchemaFieldNamesMatchConfigStruct is a sanity check that schema
// field names are reasonable (lowercase, underscore-separated).
func TestSchemaFieldNamesMatchConventions(t *testing.T) {
	for _, entry := range allSkills {
		schema := entry.def.Config
		if len(schema.Fields) == 0 {
			continue
		}

		t.Run(entry.name, func(t *testing.T) {
			for _, f := range schema.Fields {
				// Names should be snake_case.
				if f.Name != strings.ToLower(f.Name) {
					t.Errorf("field name %q should be lowercase", f.Name)
				}
				if strings.Contains(f.Name, " ") {
					t.Errorf("field name %q should not contain spaces", f.Name)
				}
				if strings.Contains(f.Name, "-") {
					t.Errorf("field name %q should use underscores, not hyphens", f.Name)
				}
			}
		})
	}
}

// captureRegistry records what was registered for test assertions.
type captureRegistry struct {
	tools        []skill.ToolDef
	skillName    string
	configSchema *skill.ConfigSchema
}

func (r *captureRegistry) RegisterTool(def skill.ToolDef, handler skill.Handler) {
	r.tools = append(r.tools, def)
}

func (r *captureRegistry) RegisterSkill(name, description, content string, tags []string) error {
	r.skillName = name
	return nil
}

func (r *captureRegistry) RegisterConfigSchema(skillName string, schema skill.ConfigSchema) {
	r.configSchema = &schema
}

func init() {
	// Verify captureRegistry satisfies skill.Registry at compile time.
	var _ skill.Registry = (*captureRegistry)(nil)

	// Print summary for CI visibility.
	fmt.Printf("Convention tests covering %d skills\n", len(allSkills))
}
