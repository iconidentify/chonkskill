// Package autonovel provides a chonkskill for autonomous novel writing.
// It implements the full autonovel pipeline: foundation generation, chapter
// drafting, adversarial evaluation and revision, export, art generation,
// and audiobook narration. Ported from NousResearch/autonovel (Python) to
// idiomatic Go with improvements: shared API client, typed structs,
// in-process orchestration, dynamic chapter counts, no hardcoded content.
package autonovel

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/iconidentify/chonkskill/pkg/skill"
	"github.com/iconidentify/chonkskill/pkg/anthropic"
	"github.com/iconidentify/chonkskill/skills/autonovel/internal/elevenlabs"
	"github.com/iconidentify/chonkskill/pkg/falai"
)

//go:embed skill.md
var SkillContent string

//go:embed agent.md
var AgentContent string

// Schema declares the config fields for the autonovel skill.
// Note: LLM proxy URL and key are internal (set by chonkbase worker, not shown in UI).
// They are in the Config struct but NOT in this Schema.
var Schema = skill.ConfigSchema{
	Fields: []skill.ConfigField{
		{
			Name:        "writer_model",
			Label:       "Writer Model",
			Description: "LLM model for drafting and generation",
			Type:        skill.FieldString,
			Scope:       skill.ScopeGlobal,
			Default:     anthropic.DefaultWriter,
			EnvVar:      "AUTONOVEL_WRITER_MODEL",
		},
		{
			Name:        "judge_model",
			Label:       "Judge Model",
			Description: "LLM model for evaluation and critique (use a different model from writer to avoid self-congratulation)",
			Type:        skill.FieldString,
			Scope:       skill.ScopeGlobal,
			Default:     anthropic.DefaultJudge,
			EnvVar:      "AUTONOVEL_JUDGE_MODEL",
		},
		{
			Name:        "review_model",
			Label:       "Review Model",
			Description: "LLM model for deep manuscript review",
			Type:        skill.FieldString,
			Scope:       skill.ScopeGlobal,
			Default:     anthropic.DefaultReview,
			EnvVar:      "AUTONOVEL_REVIEW_MODEL",
		},
		{
			Name:        "image_model",
			Label:       "Image Model",
			Description: "Model for image generation via LiteLLM (optional, for future use)",
			Type:        skill.FieldString,
			Scope:       skill.ScopeGlobal,
			EnvVar:      "AUTONOVEL_IMAGE_MODEL",
		},
		{
			Name:        "fal_key",
			Label:       "fal.ai API Key",
			Description: "API key for fal.ai image generation (cover art, ornaments)",
			Type:        skill.FieldString,
			Scope:       skill.ScopeGlobal,
			Secret:      true,
			EnvVar:      "FAL_KEY",
		},
		{
			Name:        "elevenlabs_key",
			Label:       "ElevenLabs API Key",
			Description: "API key for ElevenLabs text-to-dialogue (audiobook generation)",
			Type:        skill.FieldString,
			Scope:       skill.ScopeGlobal,
			Secret:      true,
			EnvVar:      "ELEVENLABS_API_KEY",
		},
	},
}

// Def is the skill definition.
var Def = skill.Definition{
	Name:         "autonovel",
	Description:  "Autonomous novel writing pipeline -- generate, draft, evaluate, revise, export, illustrate, and narrate complete novels",
	SkillContent: SkillContent,
	AgentContent: AgentContent,
	Tags:         []string{"novel", "writing", "fiction", "creative", "pipeline", "evaluation", "revision"},
	Config:       Schema,
}

// Config holds the credentials and settings for the autonovel skill.
type Config struct {
	// LLM proxy (internal -- set by chonkbase worker, not shown in UI).
	LLMBaseURL string // LiteLLM proxy URL (e.g. http://localhost:4000)
	LLMAPIKey  string // LiteLLM API key (Bearer token)

	// Model selection (user-configurable in UI).
	WriterModel string
	JudgeModel  string
	ReviewModel string
	ImageModel  string

	// Direct API keys for non-LLM services.
	FalKey        string
	ElevenLabsKey string
}

// ConfigFromEnv loads configuration from environment variables.
func ConfigFromEnv() Config {
	return Config{
		LLMBaseURL:    os.Getenv("LITELLM_URL"),
		LLMAPIKey:     os.Getenv("LITELLM_KEY"),
		WriterModel:   envDefault("AUTONOVEL_WRITER_MODEL", anthropic.DefaultWriter),
		JudgeModel:    envDefault("AUTONOVEL_JUDGE_MODEL", anthropic.DefaultJudge),
		ReviewModel:   envDefault("AUTONOVEL_REVIEW_MODEL", anthropic.DefaultReview),
		ImageModel:    os.Getenv("AUTONOVEL_IMAGE_MODEL"),
		FalKey:        os.Getenv("FAL_KEY"),
		ElevenLabsKey: os.Getenv("ELEVENLABS_API_KEY"),
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
	falClient   *falai.Client      // nil if FAL_KEY not set
	elClient    *elevenlabs.Client  // nil if ELEVENLABS_API_KEY not set
	writerModel string
	judgeModel  string
	reviewModel string
}

// New creates the autonovel skill with all tools registered.
func New(cfg Config) (*skill.Skill, error) {
	if cfg.LLMAPIKey == "" || cfg.LLMBaseURL == "" {
		return nil, fmt.Errorf("LITELLM_URL and LITELLM_KEY are required")
	}

	rt := &runtime{
		client:      anthropic.NewClient(cfg.LLMAPIKey, cfg.LLMBaseURL),
		writerModel: cfg.WriterModel,
		judgeModel:  cfg.JudgeModel,
		reviewModel: cfg.ReviewModel,
	}

	if cfg.FalKey != "" {
		rt.falClient = falai.NewClient(cfg.FalKey)
	}
	if cfg.ElevenLabsKey != "" {
		rt.elClient = elevenlabs.NewClient(cfg.ElevenLabsKey)
	}

	s := skill.New(Def)

	registerPipelineTools(s, rt)
	registerFoundationTools(s, rt)
	registerDraftTools(s, rt)
	registerEvaluateTools(s, rt)
	registerRevisionTools(s, rt)
	registerExportTools(s, rt)
	registerArtTools(s, rt)
	registerAudiobookTools(s, rt)

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
