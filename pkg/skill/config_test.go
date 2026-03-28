package skill

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

func TestConfigSchemaJSON(t *testing.T) {
	schema := ConfigSchema{
		Fields: []ConfigField{
			{
				Name:     "api_key",
				Label:    "API Key",
				Type:     FieldString,
				Scope:    ScopeGlobal,
				Required: true,
				Secret:   true,
				EnvVar:   "MY_API_KEY",
			},
			{
				Name:    "zip_code",
				Label:   "ZIP Code",
				Type:    FieldString,
				Scope:   ScopeUser,
				Default: "98052",
				EnvVar:  "USER_ZIP",
			},
		},
	}

	b, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ConfigSchema
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(decoded.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(decoded.Fields))
	}
	if decoded.Fields[0].Scope != ScopeGlobal {
		t.Errorf("expected global scope, got %s", decoded.Fields[0].Scope)
	}
	if decoded.Fields[1].Default != "98052" {
		t.Errorf("expected default 98052, got %s", decoded.Fields[1].Default)
	}
	if !decoded.Fields[0].Secret {
		t.Error("expected secret=true for api_key")
	}
}

func TestConfigContext(t *testing.T) {
	ctx := context.Background()

	if v := ConfigFromContext(ctx); v != nil {
		t.Fatalf("expected nil, got %v", v)
	}

	vals := ConfigValues{"api_key": "test-key", "zip_code": "90210"}
	ctx = WithConfigValues(ctx, vals)

	got := ConfigFromContext(ctx)
	if got == nil {
		t.Fatal("expected config values")
	}
	if got["api_key"] != "test-key" {
		t.Errorf("expected test-key, got %s", got["api_key"])
	}
	if got["zip_code"] != "90210" {
		t.Errorf("expected 90210, got %s", got["zip_code"])
	}
}

func TestConfigSchemaResolve(t *testing.T) {
	schema := ConfigSchema{
		Fields: []ConfigField{
			{Name: "key", EnvVar: "TEST_SKILL_KEY"},
			{Name: "model", EnvVar: "TEST_SKILL_MODEL", Default: "default-model"},
			{Name: "extra"},
		},
	}

	// No env vars set -- should get defaults only.
	os.Unsetenv("TEST_SKILL_KEY")
	os.Unsetenv("TEST_SKILL_MODEL")

	vals := schema.Resolve()
	if vals["key"] != "" {
		t.Errorf("expected empty key, got %q", vals["key"])
	}
	if vals["model"] != "default-model" {
		t.Errorf("expected default-model, got %q", vals["model"])
	}
	if vals["extra"] != "" {
		t.Errorf("expected empty extra, got %q", vals["extra"])
	}

	// Set env var -- should override default.
	os.Setenv("TEST_SKILL_KEY", "from-env")
	os.Setenv("TEST_SKILL_MODEL", "custom-model")
	defer os.Unsetenv("TEST_SKILL_KEY")
	defer os.Unsetenv("TEST_SKILL_MODEL")

	vals = schema.Resolve()
	if vals["key"] != "from-env" {
		t.Errorf("expected from-env, got %q", vals["key"])
	}
	if vals["model"] != "custom-model" {
		t.Errorf("expected custom-model, got %q", vals["model"])
	}
}

func TestConfigSchemaResolveWith(t *testing.T) {
	schema := ConfigSchema{
		Fields: []ConfigField{
			{Name: "key", EnvVar: "TEST_RW_KEY", Default: "fallback"},
			{Name: "secret", EnvVar: "TEST_RW_SECRET"},
		},
	}

	os.Setenv("TEST_RW_KEY", "env-value")
	defer os.Unsetenv("TEST_RW_KEY")
	os.Unsetenv("TEST_RW_SECRET")

	// Provider returns value for "secret" but not "key".
	provider := func(name string) string {
		if name == "secret" {
			return "db-secret"
		}
		return ""
	}

	vals := schema.ResolveWith(provider)

	// "key" -- provider empty, env set → env wins
	if vals["key"] != "env-value" {
		t.Errorf("expected env-value, got %q", vals["key"])
	}
	// "secret" -- provider has value → provider wins
	if vals["secret"] != "db-secret" {
		t.Errorf("expected db-secret, got %q", vals["secret"])
	}
}

func TestConfigSchemaValidate(t *testing.T) {
	schema := ConfigSchema{
		Fields: []ConfigField{
			{Name: "key", Label: "API Key", Required: true, EnvVar: "MY_KEY"},
			{Name: "secret", Label: "Secret", Required: true},
			{Name: "optional", Label: "Optional"},
		},
	}

	// All required present.
	err := schema.Validate(ConfigValues{"key": "k", "secret": "s"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Missing one required field.
	err = schema.Validate(ConfigValues{"key": "k"})
	if err == nil {
		t.Fatal("expected error for missing secret")
	}
	me, ok := err.(*MissingConfigError)
	if !ok {
		t.Fatalf("expected MissingConfigError, got %T", err)
	}
	if len(me.Missing) != 1 || me.Missing[0].Name != "secret" {
		t.Errorf("expected missing secret, got %+v", me.Missing)
	}

	// Missing all required.
	err = schema.Validate(ConfigValues{})
	if err == nil {
		t.Fatal("expected error for missing both")
	}
	me = err.(*MissingConfigError)
	if len(me.Missing) != 2 {
		t.Errorf("expected 2 missing, got %d", len(me.Missing))
	}

	// Error message includes env var hint when available.
	msg := me.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}
}

func TestUnconfiguredSkill(t *testing.T) {
	def := Definition{
		Name:        "test-skill",
		Description: "A test skill",
		Config: ConfigSchema{
			Fields: []ConfigField{
				{Name: "key", Required: true},
			},
		},
	}

	s := Unconfigured(def)

	if s.Configured {
		t.Error("expected Configured=false for unconfigured skill")
	}
	if len(s.Tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(s.Tools))
	}
	if s.Def.Name != "test-skill" {
		t.Errorf("expected test-skill, got %s", s.Def.Name)
	}
	if len(s.Def.Config.Fields) != 1 {
		t.Error("expected config schema preserved")
	}
}

func TestConfiguredSkill(t *testing.T) {
	def := Definition{
		Name:        "test-skill",
		Description: "A test skill",
	}

	s := New(def)
	if !s.Configured {
		t.Error("expected Configured=true for New() skill")
	}
}
