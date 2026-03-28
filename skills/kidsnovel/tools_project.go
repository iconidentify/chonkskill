package kidsnovel

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/iconidentify/chonkskill/pkg/skill"
	"github.com/iconidentify/chonkskill/pkg/project"
	"github.com/iconidentify/chonkskill/skills/kidsnovel/internal/readability"
)

func registerProjectTools(s *skill.Skill, rt *runtime) {
	skill.AddTool(s, "init_project",
		"Initialize a new kids book project. Sets the target reading level (grade 3-6) which controls vocabulary, sentence length, chapter size, and book length.",
		func(ctx context.Context, args InitProjectArgs) (string, error) {
			if args.ProjectDir == "" {
				return "", fmt.Errorf("project_dir is required")
			}
			if args.Grade < 3 || args.Grade > 6 {
				return "", fmt.Errorf("grade must be 3, 4, 5, or 6")
			}

			p := project.New(args.ProjectDir)
			if err := p.Init(); err != nil {
				return "", err
			}

			// Save grade config.
			spec := readability.GradeSpecs[args.Grade]
			config := map[string]any{
				"grade":                args.Grade,
				"chapter_word_target":  spec.ChapterWordTarget,
				"chapter_count":        spec.ChapterCount,
				"book_word_min":        spec.BookWordMin,
				"book_word_max":        spec.BookWordMax,
				"fk_min":              spec.FKMin,
				"fk_max":              spec.FKMax,
				"max_sentence_length":  spec.MaxSentenceLen,
				"avg_sentence_length":  spec.AvgSentenceLen,
			}
			configJSON, _ := json.MarshalIndent(config, "", "  ")
			if err := p.SaveFile("book_config.json", string(configJSON)); err != nil {
				return "", err
			}

			return fmt.Sprintf("Kids book project initialized at %s\nTarget: %s (FK %.1f-%.1f)\nChapters: ~%d at ~%d words each\nBook total: %d-%d words",
				args.ProjectDir, spec.Label, spec.FKMin, spec.FKMax,
				spec.ChapterCount, spec.ChapterWordTarget,
				spec.BookWordMin, spec.BookWordMax), nil
		})

	skill.AddTool(s, "get_state",
		"Get current project state including grade level, phase, chapters, and word counts",
		func(ctx context.Context, args GetStateArgs) (string, error) {
			p := project.New(args.ProjectDir)
			state, err := p.LoadState()
			if err != nil {
				return "", err
			}
			grade := loadGrade(p)
			wordCount, _ := p.CountAllWords()
			chNums, _ := p.ChapterNumbers()
			spec := readability.GradeSpecs[grade]

			out := map[string]any{
				"state":          state,
				"grade":          grade,
				"grade_label":    spec.Label,
				"total_words":    wordCount,
				"chapters_on_disk": chNums,
				"target_chapters": spec.ChapterCount,
				"word_range":     fmt.Sprintf("%d-%d", spec.BookWordMin, spec.BookWordMax),
			}
			b, _ := json.MarshalIndent(out, "", "  ")
			return string(b), nil
		})
}

// loadGrade reads the target grade from book_config.json.
func loadGrade(p *project.Project) int {
	data, err := p.LoadFile("book_config.json")
	if err != nil || data == "" {
		return 4 // default
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(data), &config); err != nil {
		return 4
	}
	if g, ok := config["grade"].(float64); ok {
		return int(g)
	}
	return 4
}
