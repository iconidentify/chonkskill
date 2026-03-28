package autonovel

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/iconidentify/chonkskill/pkg/skill"
	"github.com/iconidentify/chonkskill/skills/autonovel/internal/evaluate"
	"github.com/iconidentify/chonkskill/skills/autonovel/internal/fingerprint"
	"github.com/iconidentify/chonkskill/pkg/project"
	"github.com/iconidentify/chonkskill/skills/autonovel/internal/slop"
)

func registerEvaluateTools(s *skill.Skill, rt *runtime) {
	skill.AddTool(s, "evaluate_foundation",
		"Evaluate foundation materials (world, characters, outline, canon) using the LLM judge. Returns scores for each dimension plus mechanical slop analysis.",
		func(ctx context.Context, args EvaluateFoundationArgs) (string, error) {
			p := project.New(args.ProjectDir)
			voice, _ := p.Voice()
			world, _ := p.World()
			chars, _ := p.Characters()
			outline, _ := p.Outline()
			canon, _ := p.Canon()

			if world == "" || chars == "" || outline == "" {
				return "", fmt.Errorf("world.md, characters.md, and outline.md are required")
			}

			result, err := evaluate.Foundation(rt.client, rt.judgeModel, voice, world, chars, outline, canon)
			if err != nil {
				return "", err
			}

			// Save eval log.
			resultMap := structToMap(result)
			logPath, _ := p.SaveEvalLog("foundation", resultMap)

			b, _ := json.MarshalIndent(result, "", "  ")
			return fmt.Sprintf("overall_score: %.1f\nlore_score: %.1f\nweakest: %s\nslop_penalty: %.1f\nlog: %s\n\n%s",
				result.OverallScore, result.LoreScore, result.WeakestDimension,
				result.SlopScore.SlopPenalty, logPath, string(b)), nil
		})

	skill.AddTool(s, "evaluate_chapter",
		"Evaluate a single chapter using the LLM judge. Loads all context (voice, world, characters, outline, canon, adjacent chapters) for evaluation. Returns per-dimension scores, revision suggestions, and AI pattern detection.",
		func(ctx context.Context, args EvaluateChapterArgs) (string, error) {
			if args.Chapter == 0 {
				return "", fmt.Errorf("chapter number is required")
			}
			p := project.New(args.ProjectDir)

			chText, err := p.LoadChapter(args.Chapter)
			if err != nil || chText == "" {
				return "", fmt.Errorf("chapter %d not found", args.Chapter)
			}

			voice, _ := p.Voice()
			world, _ := p.World()
			chars, _ := p.Characters()
			outline, _ := p.Outline()
			canon, _ := p.Canon()

			chOutline := p.ExtractChapterOutline(outline, args.Chapter)

			prevTail := ""
			if args.Chapter > 1 {
				if prev, _ := p.LoadChapter(args.Chapter - 1); prev != "" {
					prevTail = project.LastNChars(prev, 2000)
				}
			}
			nextHead := ""
			if next, _ := p.LoadChapter(args.Chapter + 1); next != "" {
				nextHead = project.FirstNChars(next, 1500)
			}

			result, err := evaluate.Chapter(rt.client, rt.judgeModel, args.Chapter, chText,
				voice, world, chars, chOutline, canon, prevTail, nextHead)
			if err != nil {
				return "", err
			}

			resultMap := structToMap(result)
			logPath, _ := p.SaveEvalLog(fmt.Sprintf("ch%02d", args.Chapter), resultMap)

			b, _ := json.MarshalIndent(result, "", "  ")
			return fmt.Sprintf("overall_score: %.1f\nweakest: %s\nslop_penalty: %.1f\nlog: %s\n\n%s",
				result.OverallScore, result.WeakestDimension,
				result.SlopScore.SlopPenalty, logPath, string(b)), nil
		})

	skill.AddTool(s, "evaluate_full",
		"Evaluate the complete novel across all chapters. Returns arc-level scores, weakest chapter identification, and overall novel score.",
		func(ctx context.Context, args EvaluateFullArgs) (string, error) {
			p := project.New(args.ProjectDir)
			chapters, err := p.LoadAllChapters()
			if err != nil || len(chapters) == 0 {
				return "", fmt.Errorf("no chapters found")
			}

			voice, _ := p.Voice()
			world, _ := p.World()
			chars, _ := p.Characters()
			outline, _ := p.Outline()
			canon, _ := p.Canon()
			arcSum, _ := p.ArcSummary()

			result, err := evaluate.Full(rt.client, rt.judgeModel, chapters, voice, world, chars, outline, canon, arcSum)
			if err != nil {
				return "", err
			}

			resultMap := structToMap(result)
			logPath, _ := p.SaveEvalLog("full", resultMap)

			b, _ := json.MarshalIndent(result, "", "  ")
			return fmt.Sprintf("novel_score: %.1f\nweakest_chapter: %d\nweakest_dimension: %s\nlog: %s\n\n%s",
				result.NovelScore, result.WeakestChapter, result.WeakestDimension, logPath, string(b)), nil
		})

	skill.AddTool(s, "slop_check",
		"Run mechanical AI-slop detection on a chapter or raw text. No LLM calls -- pure regex analysis. Returns banned words, suspicious clusters, filler phrases, fiction AI tells, structural tics, and composite penalty score.",
		func(ctx context.Context, args SlopCheckArgs) (string, error) {
			var text string
			if args.Text != "" {
				text = args.Text
			} else if args.Chapter > 0 && args.ProjectDir != "" {
				p := project.New(args.ProjectDir)
				var err error
				text, err = p.LoadChapter(args.Chapter)
				if err != nil || text == "" {
					return "", fmt.Errorf("chapter %d not found", args.Chapter)
				}
			} else {
				return "", fmt.Errorf("provide either text or project_dir+chapter")
			}

			result := slop.Analyze(text)
			b, _ := json.MarshalIndent(result, "", "  ")
			return string(b), nil
		})

	skill.AddTool(s, "voice_fingerprint",
		"Run quantitative prose analysis across all chapters. No LLM calls -- pure statistical analysis. Measures sentence rhythm, vocabulary well density, dialogue ratio, em dash density, and flags outliers.",
		func(ctx context.Context, args VoiceFingerprintArgs) (string, error) {
			p := project.New(args.ProjectDir)
			chapters, err := p.LoadAllChapters()
			if err != nil || len(chapters) == 0 {
				return "", fmt.Errorf("no chapters found")
			}

			report := fingerprint.AnalyzeNovel(chapters)

			// Save report.
			reportMap := structToMap(report)
			p.SaveEditLog("voice_fingerprint.json", reportMap)

			return fingerprint.FormatReport(report), nil
		})
}

// structToMap converts a struct to map[string]any via JSON round-trip.
func structToMap(v any) map[string]any {
	b, _ := json.Marshal(v)
	var m map[string]any
	json.Unmarshal(b, &m)
	return m
}
