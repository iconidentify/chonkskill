package kidsnovel

import (
	"context"
	"fmt"
	"strings"

	"github.com/iconidentify/chonkskill/pkg/skill"
	"github.com/iconidentify/chonkskill/pkg/anthropic"
	"github.com/iconidentify/chonkskill/pkg/project"
	"github.com/iconidentify/chonkskill/skills/kidsnovel/internal/readability"
)

func registerRevisionTools(s *skill.Skill, rt *runtime) {
	skill.AddTool(s, "simplify_chapter",
		"Bring a chapter's reading level down to the target grade. Shortens sentences, replaces complex vocabulary, increases dialogue, and tightens prose while preserving the story. Run this when readability_check shows 'too-hard'.",
		func(ctx context.Context, args SimplifyChapterArgs) (string, error) {
			if args.Chapter == 0 {
				return "", fmt.Errorf("chapter number is required")
			}
			p := project.New(args.ProjectDir)
			grade := loadGrade(p)
			spec := readability.GradeSpecs[grade]

			text, _ := p.LoadChapter(args.Chapter)
			if text == "" {
				return "", fmt.Errorf("chapter %d not found", args.Chapter)
			}

			// Get current readability.
			before := readability.Analyze(text, grade)

			// Build a targeted simplification prompt.
			var issues []string
			for _, issue := range before.Issues {
				issues = append(issues, fmt.Sprintf("- %s: %s", issue.Type, issue.Detail))
			}
			issueList := "No specific issues found."
			if len(issues) > 0 {
				issueList = strings.Join(issues, "\n")
			}

			prompt := fmt.Sprintf(`Simplify this chapter to %s reading level (Flesch-Kincaid %.1f-%.1f).

## Current Readability Issues
%s

## Current Stats
- FK Grade: %.1f (target: %.1f-%.1f)
- Avg sentence: %.1f words (target: ~%.0f)
- Complex words: %.1f%% (target: <%.0f%%)
- Dialogue: %.0f%% (target: %.0f-%.0f%%)

## Chapter %d
%s

## SIMPLIFICATION RULES

1. Break long sentences into shorter ones. Two short sentences are better than one long one.
2. Replace complex words with simpler ones (use the most common word that works).
3. Add more dialogue. Turn narration into conversation where possible.
4. Cut unnecessary adjectives and adverbs. One good detail beats three vague ones.
5. Keep the story EXACTLY the same. Same plot, same characters, same emotions.
6. Keep the best writing. If a sentence is already great, don't touch it.
7. Don't dumb it down. Simple is not the same as stupid. Hemingway wrote simply.
8. Preserve character voices. If a character uses big words, that's their personality.
9. Target ~%d words (current: %d).

Return the complete simplified chapter.`,
				spec.Label, spec.FKMin, spec.FKMax,
				issueList,
				before.FleschKincaid, spec.FKMin, spec.FKMax,
				before.AvgSentenceLen, spec.AvgSentenceLen,
				before.ComplexPct, spec.VocabComplexity,
				before.DialoguePct, spec.DialoguePctMin, spec.DialoguePctMax,
				args.Chapter, text,
				spec.ChapterWordTarget, before.WordCount)

			resp, err := rt.client.Message(anthropic.Request{
				Model:       rt.writerModel,
				System:      fmt.Sprintf("You simplify children's book prose to the %s reading level. You make writing clearer and stronger, never weaker. Simple prose has power.", spec.Label),
				Prompt:      prompt,
				MaxTokens:   8000,
				Temperature: 0.5,
			})
			if err != nil {
				return "", err
			}

			if err := p.SaveChapter(args.Chapter, resp.Text); err != nil {
				return "", err
			}

			after := readability.Analyze(resp.Text, grade)
			return fmt.Sprintf("Chapter %d simplified:\n  FK: %.1f -> %.1f (%s)\n  Words: %d -> %d\n  Avg sentence: %.1f -> %.1f\n  Complex: %.1f%% -> %.1f%%\n  Dialogue: %.0f%% -> %.0f%%",
				args.Chapter,
				before.FleschKincaid, after.FleschKincaid, after.GradeFit,
				before.WordCount, after.WordCount,
				before.AvgSentenceLen, after.AvgSentenceLen,
				before.ComplexPct, after.ComplexPct,
				before.DialoguePct, after.DialoguePct), nil
		})

	skill.AddTool(s, "revise_chapter",
		"Revise a chapter based on evaluation feedback. Fixes specific issues while maintaining reading level. Use after evaluate_chapter or readability_check identifies problems.",
		func(ctx context.Context, args ReviseChapterArgs) (string, error) {
			if args.Chapter == 0 {
				return "", fmt.Errorf("chapter number is required")
			}
			p := project.New(args.ProjectDir)
			grade := loadGrade(p)
			spec := readability.GradeSpecs[grade]
			constraints := readability.GradeConstraints(grade)

			text, _ := p.LoadChapter(args.Chapter)
			if text == "" {
				return "", fmt.Errorf("chapter %d not found", args.Chapter)
			}

			feedback := args.Feedback
			if feedback == "" {
				// Auto-generate feedback from latest eval.
				evalData, _ := p.LatestEvalLog(fmt.Sprintf("ch%02d", args.Chapter))
				if evalData != nil {
					if judge, ok := evalData["judge"].(map[string]any); ok {
						if weakest, ok := judge["weakest_moment"].(string); ok {
							feedback = fmt.Sprintf("Weakest moment: %s", weakest)
						}
						if verdict, ok := judge["one_line_verdict"].(string); ok {
							feedback += fmt.Sprintf("\nVerdict: %s", verdict)
						}
					}
				}
			}
			if feedback == "" {
				feedback = "General quality improvement. Make it more engaging, more fun, more vivid."
			}

			prevTail := ""
			if args.Chapter > 1 {
				if prev, _ := p.LoadChapter(args.Chapter - 1); prev != "" {
					prevTail = project.LastNChars(prev, 1000)
				}
			}

			prompt := fmt.Sprintf(`Revise Chapter %d of this children's book based on the feedback below.

## Feedback
%s

## Reading Level Rules
%s

## Previous Chapter Ending
%s

## Chapter %d (TO REVISE)
%s

## REVISION RULES

1. Fix what the feedback identifies. Don't rewrite what works.
2. Maintain the %s reading level.
3. Keep the same plot events. Change HOW they're told, not WHAT happens.
4. If the feedback says "boring", add conflict, dialogue, or sensory details.
5. If the feedback says "confusing", simplify and clarify.
6. If the feedback says "preachy", remove the lesson and let the story speak.
7. Keep chapter length at ~%d words.
8. End with a hook.

Return the complete revised chapter.`,
				args.Chapter, feedback, constraints,
				prevTail, args.Chapter, text,
				spec.Label, spec.ChapterWordTarget)

			resp, err := rt.client.Message(anthropic.Request{
				Model:       rt.writerModel,
				System:      fmt.Sprintf("You revise children's chapter books at the %s reading level. You are a surgeon, not a demolition crew. Fix what's broken. Keep what works.", spec.Label),
				Prompt:      prompt,
				MaxTokens:   8000,
				Temperature: 0.7,
			})
			if err != nil {
				return "", err
			}

			if err := p.SaveChapter(args.Chapter, resp.Text); err != nil {
				return "", err
			}

			before := readability.Analyze(text, grade)
			after := readability.Analyze(resp.Text, grade)
			return fmt.Sprintf("Chapter %d revised:\n  FK: %.1f -> %.1f\n  Words: %d -> %d\n  Grade fit: %s -> %s",
				args.Chapter,
				before.FleschKincaid, after.FleschKincaid,
				before.WordCount, after.WordCount,
				before.GradeFit, after.GradeFit), nil
		})
}
