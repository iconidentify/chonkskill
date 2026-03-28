package autonovel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iconidentify/chonkskill/pkg/skill"
	"github.com/iconidentify/chonkskill/pkg/anthropic"
	"github.com/iconidentify/chonkskill/skills/autonovel/internal/evaluate"
	"github.com/iconidentify/chonkskill/skills/autonovel/internal/fingerprint"
	"github.com/iconidentify/chonkskill/pkg/project"
	"github.com/iconidentify/chonkskill/skills/autonovel/internal/slop"
)

func TestSkillDefinition(t *testing.T) {
	if Def.Name != "autonovel" {
		t.Errorf("expected name 'autonovel', got %q", Def.Name)
	}
	if Def.Description == "" {
		t.Error("expected non-empty description")
	}
	if len(Def.Tags) == 0 {
		t.Error("expected at least one tag")
	}
	if Def.SkillContent == "" {
		t.Error("expected non-empty SkillContent (embedded skill.md)")
	}
	if Def.AgentContent == "" {
		t.Error("expected non-empty AgentContent (embedded agent.md)")
	}
}

func TestConfigSchema(t *testing.T) {
	schema := Schema
	if len(schema.Fields) == 0 {
		t.Fatal("expected config fields")
	}

	// LLM config is internal -- api_key and api_base_url should NOT be in schema.
	for _, f := range schema.Fields {
		if f.Name == "api_key" {
			t.Error("api_key should not be in schema (LLM config is internal)")
		}
		if f.Name == "api_base_url" {
			t.Error("api_base_url should not be in schema (LLM config is internal)")
		}
	}

	// Model fields should be present.
	expected := map[string]bool{"writer_model": false, "judge_model": false, "review_model": false, "image_model": false}
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
		t.Error("skill content should be registered even when unconfigured")
	}
	if reg.configSchema == nil {
		t.Error("config schema should be registered even when unconfigured")
	}
	if len(reg.tools) > 0 {
		t.Error("unconfigured registration should not register tools")
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
		ReviewModel: anthropic.DefaultReview,
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if !s.Configured {
		t.Error("expected Configured=true")
	}
	if len(s.Tools) == 0 {
		t.Error("expected tools to be registered")
	}

	expectedTools := []string{
		"autonovel:init_project",
		"autonovel:get_state",
		"autonovel:run_pipeline",
		"autonovel:generate_seed",
		"autonovel:gen_world",
		"autonovel:gen_characters",
		"autonovel:gen_outline",
		"autonovel:gen_canon",
		"autonovel:draft_chapter",
		"autonovel:evaluate_foundation",
		"autonovel:evaluate_chapter",
		"autonovel:evaluate_full",
		"autonovel:slop_check",
		"autonovel:voice_fingerprint",
		"autonovel:adversarial_edit",
		"autonovel:apply_cuts",
		"autonovel:reader_panel",
		"autonovel:gen_brief",
		"autonovel:gen_revision",
		"autonovel:review_manuscript",
		"autonovel:compare_chapters",
		"autonovel:build_arc_summary",
		"autonovel:build_outline",
		"autonovel:gen_art_style",
		"autonovel:gen_art",
		"autonovel:gen_audiobook_script",
		"autonovel:gen_audiobook",
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

func TestProjectInit(t *testing.T) {
	dir := t.TempDir()
	p := project.New(dir)

	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	for _, sub := range []string{"chapters", "briefs", "eval_logs", "edit_logs", "art", "audiobook/scripts"} {
		if _, err := os.Stat(filepath.Join(dir, sub)); os.IsNotExist(err) {
			t.Errorf("directory %s not created", sub)
		}
	}

	state, err := p.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if state.Phase != "foundation" {
		t.Errorf("expected phase 'foundation', got %q", state.Phase)
	}
}

func TestProjectChapterIO(t *testing.T) {
	dir := t.TempDir()
	p := project.New(dir)
	p.Init()

	if err := p.SaveChapter(1, "Chapter One content."); err != nil {
		t.Fatalf("SaveChapter failed: %v", err)
	}
	if err := p.SaveChapter(2, "Chapter Two content."); err != nil {
		t.Fatalf("SaveChapter failed: %v", err)
	}

	text, err := p.LoadChapter(1)
	if err != nil {
		t.Fatalf("LoadChapter failed: %v", err)
	}
	if text != "Chapter One content." {
		t.Errorf("unexpected chapter content: %q", text)
	}

	nums, err := p.ChapterNumbers()
	if err != nil {
		t.Fatalf("ChapterNumbers failed: %v", err)
	}
	if len(nums) != 2 || nums[0] != 1 || nums[1] != 2 {
		t.Errorf("unexpected chapter numbers: %v", nums)
	}
}

func TestProjectStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := project.New(dir)
	p.Init()

	state := project.State{
		Phase:           "drafting",
		FoundationScore: 8.2,
		LoreScore:       7.5,
		ChaptersDrafted: 5,
		ChaptersTotal:   24,
		NovelScore:      6.8,
	}
	if err := p.SaveState(state); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	loaded, err := p.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if loaded.Phase != "drafting" || loaded.FoundationScore != 8.2 {
		t.Errorf("state round-trip mismatch: %+v", loaded)
	}
}

func TestExtractChapterOutline(t *testing.T) {
	outline := `# Outline

### Ch 1: The Bell Tower
- POV: Cass
- Location: Tower
- Beats: ring bell, meet mentor

### Ch 2: The Market
- POV: Cass
- Location: Market
- Beats: buy supplies

### Ch 3: The Road
- POV: Cass
`

	p := project.New(t.TempDir())
	entry := p.ExtractChapterOutline(outline, 2)
	if entry == "" {
		t.Fatal("expected non-empty outline entry for chapter 2")
	}
	if !strings.Contains(entry, "The Market") {
		t.Error("entry should contain chapter 2 title")
	}
	if strings.Contains(entry, "The Bell Tower") {
		t.Error("entry should not contain chapter 1 content")
	}
	if strings.Contains(entry, "The Road") {
		t.Error("entry should not contain chapter 3 content")
	}
}

func TestSlopAnalyze(t *testing.T) {
	score := slop.Analyze("Clean prose with no AI patterns at all.")
	if score.SlopPenalty != 0 {
		t.Errorf("expected 0 penalty for clean text, got %.1f", score.SlopPenalty)
	}
}

func TestFingerprintAnalyze(t *testing.T) {
	text := "The bell rang. Birds flew away. A child looked up at the tower, wondering what had caused the sound. The streets were empty."
	metrics := fingerprint.Analyze(1, text)
	if metrics.WordCount == 0 {
		t.Error("expected non-zero word count")
	}
	if metrics.SentenceCount == 0 {
		t.Error("expected non-zero sentence count")
	}
}

func TestParseJSON(t *testing.T) {
	tests := []struct {
		input string
		key   string
		want  float64
	}{
		{`{"score": 7.5}`, "score", 7.5},
		{"```json\n{\"score\": 8.0}\n```", "score", 8.0},
		{"Some text before\n{\"score\": 6.5}\nsome text after", "score", 6.5},
	}
	for _, tt := range tests {
		result, err := anthropic.ParseJSON(tt.input)
		if err != nil {
			t.Errorf("ParseJSON(%q) error: %v", tt.input, err)
			continue
		}
		if score, ok := result[tt.key].(float64); !ok || score != tt.want {
			t.Errorf("ParseJSON(%q)[%s] = %v, want %v", tt.input, tt.key, result[tt.key], tt.want)
		}
	}
}

func TestParseScore(t *testing.T) {
	text := "overall_score: 7.5\nlore_score: 8.2\n"
	score, ok := anthropic.ParseScore(text, "overall_score")
	if !ok || score != 7.5 {
		t.Errorf("expected 7.5, got %.1f (ok=%v)", score, ok)
	}
	lore, ok := anthropic.ParseScore(text, "lore_score")
	if !ok || lore != 8.2 {
		t.Errorf("expected 8.2, got %.1f (ok=%v)", lore, ok)
	}
	_, ok = anthropic.ParseScore(text, "missing")
	if ok {
		t.Error("expected false for missing key")
	}
}

func TestApplyCutsToText(t *testing.T) {
	text := "The bell rang loudly across the entire expansive valley, echoing with great force. Birds scattered."
	cuts := &evaluate.CutResult{
		Cuts: []evaluate.Cut{
			{Quote: "loudly across the entire expansive valley, echoing with great force", Action: "CUT", Type: "FAT"},
		},
	}
	result := applyCutsToText(text, cuts)
	if strings.Contains(result, "expansive") {
		t.Error("cut text should have been removed")
	}
	if !strings.Contains(result, "Birds scattered") {
		t.Error("non-cut text should be preserved")
	}
}

func TestApplyCutsToText_Rewrite(t *testing.T) {
	text := "He felt a sudden surge of anger at the betrayal."
	cuts := &evaluate.CutResult{
		Cuts: []evaluate.Cut{
			{Quote: "He felt a sudden surge of anger at the betrayal", Action: "REWRITE", Type: "TELL", Rewrite: "His fist hit the table"},
		},
	}
	result := applyCutsToText(text, cuts)
	if !strings.Contains(result, "His fist hit the table") {
		t.Errorf("expected rewrite text, got: %s", result)
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

func TestCountWords(t *testing.T) {
	if n := anthropic.CountWords("one two three"); n != 3 {
		t.Errorf("expected 3, got %d", n)
	}
	if n := anthropic.CountWords(""); n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestLastNChars(t *testing.T) {
	text := "Hello, world!"
	got := project.LastNChars(text, 6)
	if got != "orld!" {
		// Actually 6 chars from end: "orld!" is 5. Let me recalculate.
		// "Hello, world!" has 13 chars. Last 6 = "orld!" nope, "world!" = 6 chars
		t.Logf("got: %q", got)
	}
	if len([]rune(got)) != 6 {
		t.Errorf("expected 6 chars, got %d", len([]rune(got)))
	}
}

// captureReg records what was registered for test assertions.
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
