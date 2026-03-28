package kidsnovel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/iconidentify/chonkskill/pkg/skill"
	"github.com/iconidentify/chonkskill/pkg/anthropic"
	"github.com/iconidentify/chonkskill/pkg/project"
	"github.com/iconidentify/chonkskill/skills/kidsnovel/internal/readability"
)

func TestSkillDefinition(t *testing.T) {
	if Def.Name != "kidsnovel" {
		t.Errorf("expected name 'kidsnovel', got %q", Def.Name)
	}
	if Def.Description == "" {
		t.Error("expected non-empty description")
	}
	if len(Def.Tags) == 0 {
		t.Error("expected at least one tag")
	}
	if Def.SkillContent == "" {
		t.Error("expected non-empty SkillContent")
	}
	if Def.AgentContent == "" {
		t.Error("expected non-empty AgentContent")
	}
}

func TestConfigSchema(t *testing.T) {
	schema := Schema
	if len(schema.Fields) == 0 {
		t.Fatal("expected config fields")
	}

	// LLM config is internal -- api_key should NOT be in schema.
	for _, f := range schema.Fields {
		if f.Name == "api_key" {
			t.Error("api_key should not be in schema (LLM config is internal)")
		}
	}

	// Model fields should be present.
	expected := map[string]bool{"writer_model": false, "judge_model": false, "image_model": false}
	for _, f := range schema.Fields {
		if _, ok := expected[f.Name]; ok {
			expected[f.Name] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("expected field %q in schema", name)
		}
	}
}

func TestUnconfiguredRegistration(t *testing.T) {
	reg := &captureReg{}
	err := Register(reg, Config{})
	if err != nil {
		t.Fatalf("unconfigured registration failed: %v", err)
	}
	if reg.skillName == "" {
		t.Error("skill content should be registered")
	}
	if reg.configSchema == nil {
		t.Error("config schema should be registered")
	}
}

func TestNewWithConfig(t *testing.T) {
	llmURL := os.Getenv("LITELLM_URL")
	llmKey := os.Getenv("LITELLM_KEY")
	if llmURL == "" || llmKey == "" {
		t.Skip("LITELLM_URL and LITELLM_KEY not set")
	}

	s, err := New(Config{
		LLMBaseURL:  llmURL,
		LLMAPIKey:   llmKey,
		WriterModel: anthropic.DefaultWriter,
		JudgeModel:  anthropic.DefaultJudge,
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if !s.Configured {
		t.Error("expected Configured=true")
	}

	expectedTools := []string{
		"kidsnovel:init_project",
		"kidsnovel:get_state",
		"kidsnovel:from_kid_writing",
		"kidsnovel:from_idea",
		"kidsnovel:generate_seed",
		"kidsnovel:gen_world",
		"kidsnovel:gen_characters",
		"kidsnovel:gen_outline",
		"kidsnovel:draft_chapter",
		"kidsnovel:draft_all",
		"kidsnovel:readability_check",
		"kidsnovel:evaluate_chapter",
		"kidsnovel:evaluate_book",
		"kidsnovel:simplify_chapter",
		"kidsnovel:revise_chapter",
		"kidsnovel:build_book",
		"kidsnovel:gen_illustration",
		"kidsnovel:run_pipeline",
	}

	toolMap := make(map[string]bool)
	for _, tool := range s.Tools {
		toolMap[tool.Name] = true
	}
	for _, expected := range expectedTools {
		if !toolMap[expected] {
			t.Errorf("missing expected tool: %s", expected)
		}
	}
}

func TestProjectInitWithGrade(t *testing.T) {
	dir := t.TempDir()
	p := project.New(dir)
	p.Init()

	// Write grade config.
	config := map[string]any{"grade": float64(3)}
	configJSON, _ := json.MarshalIndent(config, "", "  ")
	p.SaveFile("book_config.json", string(configJSON))

	grade := loadGrade(p)
	if grade != 3 {
		t.Errorf("expected grade 3, got %d", grade)
	}
}

func TestReadabilityGrade3(t *testing.T) {
	text := `Sam ran to the park. He saw a big dog. The dog was brown.
"Can I play?" Sam asked. The dog wagged its tail. Sam smiled.
He threw the ball. The dog ran fast. This was a good day.`

	a := readability.Analyze(text, 3)
	if a.FleschKincaid > 4.0 {
		t.Errorf("simple text FK should be low, got %.1f", a.FleschKincaid)
	}
	if a.GradeFit == "too-hard" {
		t.Error("simple text should not be too-hard for grade 3")
	}
}

func TestReadabilityGrade6(t *testing.T) {
	text := `Maya discovered the hidden passage behind the old bookshelf in the library.
The air smelled like dust and something else -- something almost like cinnamon.
"I don't think we should go in there," whispered Kai, gripping her flashlight.
"That's exactly why we should," Maya replied, already stepping through the gap.
The tunnel stretched ahead, dim and narrow, with strange markings carved into the stone walls.`

	a := readability.Analyze(text, 6)
	if a.WordCount == 0 {
		t.Fatal("expected non-zero word count")
	}
	// This text should be in the grade 5-6 range.
	if a.FleschKincaid > 8 {
		t.Errorf("text seems too complex: FK %.1f", a.FleschKincaid)
	}
}

func TestGradeConstraintsAllGrades(t *testing.T) {
	for grade := 3; grade <= 6; grade++ {
		c := readability.GradeConstraints(grade)
		if c == "" {
			t.Errorf("empty constraints for grade %d", grade)
		}
	}
}

func TestConfigSchemaJSONRoundTrip(t *testing.T) {
	b, err := json.Marshal(Schema)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var decoded skill.ConfigSchema
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(decoded.Fields) != len(Schema.Fields) {
		t.Errorf("field count mismatch: %d vs %d", len(decoded.Fields), len(Schema.Fields))
	}
}

func TestProjectDirsCreated(t *testing.T) {
	dir := t.TempDir()
	p := project.New(dir)
	p.Init()

	for _, sub := range []string{"chapters", "briefs", "eval_logs", "edit_logs", "art"} {
		if _, err := os.Stat(filepath.Join(dir, sub)); os.IsNotExist(err) {
			t.Errorf("directory %s not created", sub)
		}
	}
}

type captureReg struct {
	tools        []skill.ToolDef
	skillName    string
	configSchema *skill.ConfigSchema
}

func (r *captureReg) RegisterTool(def skill.ToolDef, handler skill.Handler) {
	r.tools = append(r.tools, def)
}

func (r *captureReg) RegisterSkill(name, description, content string, tags []string) error {
	r.skillName = name
	return nil
}

func (r *captureReg) RegisterConfigSchema(skillName string, schema skill.ConfigSchema) {
	r.configSchema = &schema
}
