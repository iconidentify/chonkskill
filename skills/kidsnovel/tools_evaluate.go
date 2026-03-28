package kidsnovel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/iconidentify/chonkskill/pkg/skill"
	"github.com/iconidentify/chonkskill/pkg/anthropic"
	"github.com/iconidentify/chonkskill/pkg/project"
	"github.com/iconidentify/chonkskill/skills/kidsnovel/internal/readability"
)

func registerEvaluateTools(s *skill.Skill, rt *runtime) {
	skill.AddTool(s, "readability_check",
		"Analyze the reading level of a chapter or text. Returns Flesch-Kincaid grade, vocabulary complexity, sentence length distribution, dialogue percentage, and specific issues. No LLM calls -- pure math.",
		func(ctx context.Context, args ReadabilityCheckArgs) (string, error) {
			var text string
			var grade int

			if args.Text != "" {
				text = args.Text
				grade = args.Grade
				if grade == 0 {
					grade = 4
				}
			} else if args.Chapter > 0 && args.ProjectDir != "" {
				p := project.New(args.ProjectDir)
				var err error
				text, err = p.LoadChapter(args.Chapter)
				if err != nil || text == "" {
					return "", fmt.Errorf("chapter %d not found", args.Chapter)
				}
				grade = args.Grade
				if grade == 0 {
					grade = loadGrade(p)
				}
			} else {
				return "", fmt.Errorf("provide either text or project_dir+chapter")
			}

			analysis := readability.Analyze(text, grade)
			return readability.FormatReport(analysis), nil
		})

	skill.AddTool(s, "evaluate_chapter",
		"Full quality evaluation of a chapter: reading level + LLM judge assessing engagement, age-appropriateness, character voice, pacing, and craft. The judge evaluates specifically for children's literature standards.",
		func(ctx context.Context, args EvaluateChapterArgs) (string, error) {
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

			// Mechanical readability.
			analysis := readability.Analyze(text, grade)

			// LLM evaluation.
			outline, _ := p.Outline()
			chOutline := p.ExtractChapterOutline(outline, args.Chapter)

			prompt := fmt.Sprintf(`Evaluate Chapter %d of this children's book for %s readers (ages %d-%d).

## Chapter %d
%s

## Chapter Outline
%s

Score each dimension 1-10 and explain briefly:

1. **Hook Power**: Does the chapter start strong? Would a kid keep reading past page 1?
2. **Engagement**: Is every page earning the reader's attention? Any boring stretches?
3. **Character Voice**: Do the characters sound like real kids? Is dialogue natural?
4. **Pacing**: Is the chapter well-paced? Fast enough to keep attention, slow enough for emotion?
5. **Age Appropriateness**: Are themes, vocabulary, and situations right for the target age?
6. **Fun Factor**: Would a kid enjoy reading this? Would they read it to a friend?
7. **Emotional Resonance**: Does the chapter make the reader FEEL something?
8. **Chapter Arc**: Does it have a clear beginning, middle, and end? Does it end with a hook?
9. **Show Don't Tell**: Does the writing show through action and dialogue, not narration?
10. **Originality**: Does this feel fresh, or like every other kids book?

Also provide:
- **Best Moment**: The single strongest moment in the chapter
- **Weakest Moment**: What drags the chapter down most
- **Overall Score**: Weighted average (engagement and fun factor weighted highest)
- **One-Line Verdict**: What would a kid think of this chapter?

Return as JSON.`, args.Chapter, spec.Label, spec.Grade+5, spec.Grade+8,
				args.Chapter, text, chOutline)

			resp, err := rt.client.Message(anthropic.Request{
				Model:       rt.judgeModel,
				System:      "You evaluate children's literature. You judge by the only standard that matters: would a kid want to keep reading? You score honestly. A 5 is mediocre. An 8 is genuinely good. Respond in JSON.",
				Prompt:      prompt,
				MaxTokens:   4000,
				Temperature: 0.3,
			})
			if err != nil {
				return "", fmt.Errorf("evaluation failed: %w", err)
			}

			// Combine readability + judge results.
			var sb strings.Builder
			sb.WriteString(readability.FormatReport(analysis))
			sb.WriteString("\n\n# Judge Evaluation\n\n")
			sb.WriteString(resp.Text)

			// Save eval log.
			evalData := map[string]any{
				"chapter":     args.Chapter,
				"readability": structToMap(analysis),
				"judge_raw":   resp.Text,
			}
			if parsed, err := anthropic.ParseJSON(resp.Text); err == nil {
				evalData["judge"] = parsed
			}
			logName := fmt.Sprintf("ch%02d", args.Chapter)
			p.SaveEvalLog(logName, evalData)

			return sb.String(), nil
		})

	skill.AddTool(s, "evaluate_book",
		"Full book evaluation: reading level consistency across all chapters + LLM judge assessing the complete story arc, theme delivery, and overall kid-appeal.",
		func(ctx context.Context, args EvaluateBookArgs) (string, error) {
			p := project.New(args.ProjectDir)
			grade := loadGrade(p)
			spec := readability.GradeSpecs[grade]

			chapters, err := p.LoadAllChapters()
			if err != nil || len(chapters) == 0 {
				return "", fmt.Errorf("no chapters found")
			}

			// Per-chapter readability.
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("# Book Evaluation (Target: %s)\n\n", spec.Label))
			sb.WriteString("## Per-Chapter Readability\n\n")
			sb.WriteString("| Ch | Words | FK Grade | Fit | Issues |\n")
			sb.WriteString("|----|-------|----------|-----|--------|\n")

			totalWords := 0
			issueCount := 0
			chNums, _ := p.ChapterNumbers()
			for _, n := range chNums {
				a := readability.Analyze(chapters[n], grade)
				totalWords += a.WordCount
				issueCount += len(a.Issues)
				sb.WriteString(fmt.Sprintf("| %d | %d | %.1f | %s | %d |\n",
					n, a.WordCount, a.FleschKincaid, a.GradeFit, len(a.Issues)))
			}

			sb.WriteString(fmt.Sprintf("\nTotal: %d words, %d chapters, %d readability issues\n",
				totalWords, len(chNums), issueCount))

			// LLM evaluation of full story.
			var manuscript strings.Builder
			for _, n := range chNums {
				manuscript.WriteString(fmt.Sprintf("\n--- Chapter %d ---\n\n%s", n, chapters[n]))
			}

			prompt := fmt.Sprintf(`Evaluate this complete children's book for %s readers (ages %d-%d).

%s

Score each dimension 1-10:
1. **Story Arc**: Does the overall story have a satisfying shape?
2. **Theme Delivery**: Does the theme come through the story (not preaching)?
3. **Character Growth**: Does the main character change in a believable way?
4. **Pacing**: Does the book maintain momentum from start to finish?
5. **Ending**: Is the ending earned, surprising, and satisfying?
6. **Re-readability**: Would a kid want to read this again?
7. **Read-aloud Quality**: Would this work as a bedtime read-aloud?
8. **Overall Kid Appeal**: Bottom line -- would kids love this?

Also:
- **Strongest Chapter**: Which chapter is the best and why?
- **Weakest Chapter**: Which chapter needs the most work?
- **Book Score**: Overall score (1-10)
- **One-Line Verdict**: What would you tell a parent about this book?

Return as JSON.`, spec.Label, spec.Grade+5, spec.Grade+8, manuscript.String())

			resp, err := rt.client.Message(anthropic.Request{
				Model:       rt.judgeModel,
				System:      "You evaluate children's chapter books. The only metric: would kids love it? Score honestly. Respond in JSON.",
				Prompt:      prompt,
				MaxTokens:   4000,
				Temperature: 0.3,
			})
			if err == nil {
				sb.WriteString("\n## Story Evaluation\n\n")
				sb.WriteString(resp.Text)

				evalData := map[string]any{
					"total_words": totalWords,
					"chapters":    len(chNums),
					"judge_raw":   resp.Text,
				}
				if parsed, err := anthropic.ParseJSON(resp.Text); err == nil {
					evalData["judge"] = parsed
				}
				p.SaveEvalLog("full", evalData)
			}

			return sb.String(), nil
		})
}

func structToMap(v any) map[string]any {
	b, _ := json.Marshal(v)
	var m map[string]any
	json.Unmarshal(b, &m)
	return m
}
