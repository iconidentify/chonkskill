// Package skillcreator provides a meta-skill for creating, testing, and
// refining agent skills on the Chonkbase platform. It registers the skill
// creator methodology plus grader, comparator, and analyzer sub-agent prompts
// as discoverable skills.
package skillcreator

import (
	_ "embed"

	"github.com/iconidentify/chonkskill/pkg/skill"
)

//go:embed skill.md
var SkillContent string

//go:embed grader.md
var GraderContent string

//go:embed comparator.md
var ComparatorContent string

//go:embed analyzer.md
var AnalyzerContent string

// Config holds settings for the skill creator. No credentials needed.
type Config struct{}

// Register registers the skill creator methodology and its sub-agent skills.
// This is a content-only skill -- no tools are registered because the skill
// creator uses platform tools (create_skill, delegate_task, etc.) directly.
func Register(reg skill.Registry, _ Config) error {
	// Main skill creator methodology.
	if err := reg.RegisterSkill(
		"skill-creator",
		"Create, test, and refine agent skills -- use when asked to build a new capability, improve an existing skill, or teach agents a new procedure",
		SkillContent,
		[]string{"skill", "meta", "authoring", "evaluation"},
	); err != nil {
		return err
	}

	// Grader sub-agent prompt.
	if err := reg.RegisterSkill(
		"skill-grader",
		"Grade agent outputs against skill quality criteria -- structured pass/fail with evidence",
		GraderContent,
		[]string{"evaluation", "grading"},
	); err != nil {
		return err
	}

	// Comparator sub-agent prompt.
	if err := reg.RegisterSkill(
		"skill-comparator",
		"Blind A/B comparison of two agent outputs to determine if a skill improved quality",
		ComparatorContent,
		[]string{"evaluation", "comparison"},
	); err != nil {
		return err
	}

	// Analyzer sub-agent prompt.
	if err := reg.RegisterSkill(
		"skill-analyzer",
		"Analyze benchmark results from skill testing -- identify failure patterns and recommend improvements",
		AnalyzerContent,
		[]string{"evaluation", "analysis"},
	); err != nil {
		return err
	}

	return nil
}
