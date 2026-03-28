// Package kidsnovel provides a chonkskill for creating high-quality children's
// chapter books at grade 3-6 reading levels. Built on the autonovel pipeline
// architecture but adapted for children's literature: reading level enforcement,
// kid-collaborative creation, age-appropriate evaluation, and illustration support.
//
// Common workflows:
//   - A kid describes their story idea and the system builds a full book from it
//   - A kid's piece of writing becomes the seed for a polished chapter book
//   - A content creator generates grade-leveled chapter books from concepts
//   - A parent and child collaborate on a book together
package kidsnovel

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/iconidentify/chonkskill/pkg/anthropic"
	"github.com/iconidentify/chonkskill/pkg/imagegen"
	"github.com/iconidentify/chonkskill/pkg/skill"
)

//go:embed skill.md
var SkillContent string

//go:embed agent.md
var AgentContent string

// Schema declares the config fields for the kidsnovel skill.
// Note: LLM proxy URL and key are internal (set by chonkbase worker, not shown in UI).
var Schema = skill.ConfigSchema{
	Fields: []skill.ConfigField{
		{
			Name:        "writer_model",
			Label:       "Writer Model",
			Description: "LLM model for drafting (Sonnet recommended for speed and quality)",
			Type:        skill.FieldString,
			Scope:       skill.ScopeGlobal,
			Default:     anthropic.DefaultWriter,
			EnvVar:      "KIDSNOVEL_WRITER_MODEL",
		},
		{
			Name:        "judge_model",
			Label:       "Judge Model",
			Description: "LLM model for evaluation (use a different model from writer for honest critique)",
			Type:        skill.FieldString,
			Scope:       skill.ScopeGlobal,
			Default:     anthropic.DefaultJudge,
			EnvVar:      "KIDSNOVEL_JUDGE_MODEL",
		},
		{
			Name:        "image_model",
			Label:       "Image Model",
			Description: "Model for illustration generation (routed through LiteLLM -- e.g. gemini-2.0-flash-preview-image-generation, dall-e-3, flux-pro)",
			Type:        skill.FieldString,
			Scope:       skill.ScopeGlobal,
			Default:     imagegen.DefaultImageModel,
			EnvVar:      "KIDSNOVEL_IMAGE_MODEL",
		},
	},
}

// Def is the skill definition.
var Def = skill.Definition{
	Name:         "kidsnovel",
	Description:  "Create children's chapter books at grade 3-6 reading levels -- collaborative with kids, reading-level enforced, illustrated",
	SkillContent: SkillContent,
	AgentContent: AgentContent,
	Tags:         []string{"kids", "children", "book", "writing", "education", "reading", "illustration", "creative"},
	Config:       Schema,
}

// Config holds the credentials and settings.
type Config struct {
	// LLM proxy (internal -- set by chonkbase worker, not shown in UI).
	LLMBaseURL string // LiteLLM proxy URL
	LLMAPIKey  string // LiteLLM API key (Bearer token)

	// Model selection (user-configurable in UI).
	WriterModel string
	JudgeModel  string
	ImageModel  string

}

// ConfigFromEnv loads configuration from environment variables.
func ConfigFromEnv() Config {
	return Config{
		LLMBaseURL:  os.Getenv("LITELLM_URL"),
		LLMAPIKey:   os.Getenv("LITELLM_KEY"),
		WriterModel: envDefault("KIDSNOVEL_WRITER_MODEL", anthropic.DefaultWriter),
		JudgeModel:  envDefault("KIDSNOVEL_JUDGE_MODEL", anthropic.DefaultJudge),
		ImageModel: envDefault("KIDSNOVEL_IMAGE_MODEL", imagegen.DefaultImageModel),
	}
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// runtime holds the initialized clients shared across all tool handlers.
type runtime struct {
	client      *anthropic.Client
	imageClient *imagegen.Client
	writerModel string
	judgeModel  string
}

// New creates the kidsnovel skill with all tools registered.
func New(cfg Config) (*skill.Skill, error) {
	if cfg.LLMAPIKey == "" || cfg.LLMBaseURL == "" {
		return nil, fmt.Errorf("LITELLM_URL and LITELLM_KEY are required")
	}

	rt := &runtime{
		client:      anthropic.NewClient(cfg.LLMAPIKey, cfg.LLMBaseURL),
		writerModel: cfg.WriterModel,
		judgeModel:  cfg.JudgeModel,
	}

	if cfg.ImageModel != "" {
		rt.imageClient = imagegen.NewClient(cfg.LLMAPIKey, cfg.LLMBaseURL, cfg.ImageModel)
	}

	s := skill.New(Def)

	registerProjectTools(s, rt)
	registerCreateTools(s, rt)
	registerFoundationTools(s, rt)
	registerDraftTools(s, rt)
	registerEvaluateTools(s, rt)
	registerRevisionTools(s, rt)
	registerExportTools(s, rt)

	return s, nil
}

// Register is the one-liner for chonkbase integration.
func Register(reg skill.Registry, cfg Config) error {
	s, err := New(cfg)
	if err != nil {
		return skill.Unconfigured(Def).Register(reg)
	}
	return s.Register(reg)
}
