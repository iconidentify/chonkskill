package autonovel

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/iconidentify/chonkskill/pkg/skill"
	"github.com/iconidentify/chonkskill/pkg/anthropic"
	"github.com/iconidentify/chonkskill/skills/autonovel/internal/evaluate"
	"github.com/iconidentify/chonkskill/pkg/project"
)

func registerRevisionTools(s *skill.Skill, rt *runtime) {
	skill.AddTool(s, "adversarial_edit",
		"Run the ruthless editor on a chapter. Identifies 10-20 passages to cut or rewrite, classified by type (FAT, REDUNDANT, OVER-EXPLAIN, GENERIC, TELL, STRUCTURAL). Saves cut list to edit_logs/.",
		func(ctx context.Context, args AdversarialEditArgs) (string, error) {
			p := project.New(args.ProjectDir)

			chapters := []int{args.Chapter}
			if args.Chapter == 0 {
				var err error
				chapters, err = p.ChapterNumbers()
				if err != nil {
					return "", err
				}
			}

			var results []string
			for _, ch := range chapters {
				text, err := p.LoadChapter(ch)
				if err != nil || text == "" {
					continue
				}

				cuts, err := evaluate.AdversarialEdit(rt.client, rt.judgeModel, text, ch)
				if err != nil {
					results = append(results, fmt.Sprintf("Ch %d: ERROR: %v", ch, err))
					continue
				}

				p.SaveEditLog(fmt.Sprintf("ch%02d_cuts.json", ch), cuts)
				results = append(results, fmt.Sprintf("Ch %d: %d cuts, %d cuttable words, %.0f%% fat -- %s",
					ch, len(cuts.Cuts), cuts.TotalCuttable, cuts.OverallFatPct, cuts.OneSentenceVerdict))
			}

			return strings.Join(results, "\n"), nil
		})

	skill.AddTool(s, "apply_cuts",
		"Apply adversarial edit cuts to chapter files. Handles quote-matching with normalized whitespace fallback. Supports filtering by cut type and fat percentage threshold.",
		func(ctx context.Context, args ApplyCutsArgs) (string, error) {
			p := project.New(args.ProjectDir)

			chapters := []int{args.Chapter}
			if args.Chapter == 0 {
				var err error
				chapters, err = p.ChapterNumbers()
				if err != nil {
					return "", err
				}
			}

			typeFilter := make(map[string]bool)
			for _, t := range args.Types {
				typeFilter[strings.ToUpper(t)] = true
			}

			var results []string
			for _, ch := range chapters {
				// Load cuts.
				cutsData, err := p.LoadEditLog(fmt.Sprintf("ch%02d_cuts.json", ch))
				if err != nil || cutsData == nil {
					continue
				}
				var cuts evaluate.CutResult
				if err := json.Unmarshal(cutsData, &cuts); err != nil {
					continue
				}

				if args.MinFatPct > 0 && cuts.OverallFatPct < args.MinFatPct {
					continue
				}

				// Filter cuts by type.
				if len(typeFilter) > 0 {
					var filtered []evaluate.Cut
					for _, c := range cuts.Cuts {
						if typeFilter[strings.ToUpper(c.Type)] {
							filtered = append(filtered, c)
						}
					}
					cuts.Cuts = filtered
				}

				text, _ := p.LoadChapter(ch)
				if text == "" {
					continue
				}

				originalWords := anthropic.CountWords(text)
				modified := applyCutsToText(text, &cuts)

				if args.DryRun {
					newWords := anthropic.CountWords(modified)
					results = append(results, fmt.Sprintf("Ch %d: would remove %d words (%d -> %d)",
						ch, originalWords-newWords, originalWords, newWords))
				} else {
					if err := p.SaveChapter(ch, modified); err != nil {
						results = append(results, fmt.Sprintf("Ch %d: ERROR saving: %v", ch, err))
						continue
					}
					newWords := anthropic.CountWords(modified)
					results = append(results, fmt.Sprintf("Ch %d: removed %d words (%d -> %d)",
						ch, originalWords-newWords, originalWords, newWords))
				}
			}

			return strings.Join(results, "\n"), nil
		})

	skill.AddTool(s, "reader_panel",
		"Run four-persona novel evaluation. Editor, genre reader, writer, and first reader each answer 10 structured questions about the novel. Disagreements between readers are flagged. Requires arc_summary.md.",
		func(ctx context.Context, args ReaderPanelArgs) (string, error) {
			p := project.New(args.ProjectDir)
			arcSummary, _ := p.ArcSummary()
			if arcSummary == "" {
				return "", fmt.Errorf("arc_summary.md is required -- use autonovel:build_arc_summary first")
			}

			panel, err := evaluate.RunReaderPanel(rt.client, rt.judgeModel, arcSummary)
			if err != nil {
				return "", err
			}

			p.SaveEditLog("reader_panel.json", panel)

			// Format summary.
			var sb strings.Builder
			sb.WriteString("# Reader Panel Results\n\n")
			for persona, answers := range panel.Readers {
				sb.WriteString(fmt.Sprintf("## %s\n", persona))
				if raw, ok := answers["raw"]; ok {
					sb.WriteString(fmt.Sprintf("%v\n\n", raw))
				} else {
					for q, a := range answers {
						sb.WriteString(fmt.Sprintf("- **%s**: %v\n", q, a))
					}
					sb.WriteString("\n")
				}
			}
			if len(panel.Disagreements) > 0 {
				sb.WriteString("## Disagreements\n")
				for _, d := range panel.Disagreements {
					sb.WriteString(fmt.Sprintf("- %s Ch %d: flagged by %v, not by %v\n",
						d.Question, d.Chapter, d.FlaggedBy, d.NotFlagged))
				}
			}

			return sb.String(), nil
		})

	skill.AddTool(s, "gen_brief",
		"Generate a revision brief for a chapter from evaluation data, reader panel feedback, or adversarial cuts. The brief tells gen_revision exactly what to fix.",
		func(ctx context.Context, args GenBriefArgs) (string, error) {
			p := project.New(args.ProjectDir)
			source := args.Source
			if source == "" {
				source = "auto"
			}

			ch := args.Chapter

			chText, _ := p.LoadChapter(ch)
			if chText == "" {
				return "", fmt.Errorf("chapter %d not found", ch)
			}
			wordCount := anthropic.CountWords(chText)

			var brief string
			var briefType string

			switch source {
			case "panel":
				brief, briefType = buildPanelBrief(p, ch, wordCount)
			case "eval":
				brief, briefType = buildEvalBrief(p, ch, wordCount)
			case "cuts":
				brief, briefType = buildCutsBrief(p, ch, wordCount)
			case "auto":
				brief, briefType = buildAutoBrief(p, rt, ch, wordCount)
			default:
				return "", fmt.Errorf("unknown source: %s (use panel, eval, cuts, or auto)", source)
			}

			if brief == "" {
				return "", fmt.Errorf("no data available to build brief for chapter %d from source %s", ch, source)
			}

			if err := p.SaveBrief(ch, briefType, brief); err != nil {
				return "", err
			}

			return brief, nil
		})

	skill.AddTool(s, "gen_revision",
		"Rewrite a chapter from a revision brief. Loads all context plus the old chapter and the brief, then generates a new version.",
		func(ctx context.Context, args GenRevisionArgs) (string, error) {
			p := project.New(args.ProjectDir)
			if args.Chapter == 0 {
				return "", fmt.Errorf("chapter number is required")
			}

			oldText, _ := p.LoadChapter(args.Chapter)
			if oldText == "" {
				return "", fmt.Errorf("chapter %d not found", args.Chapter)
			}

			brief, err := p.LoadBrief(args.BriefPath)
			if err != nil || brief == "" {
				return "", fmt.Errorf("brief not found at %s", args.BriefPath)
			}

			voice, _ := p.Voice()
			chars, _ := p.Characters()
			world, _ := p.World()

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

			prompt := fmt.Sprintf(`Rewrite Chapter %d according to the revision brief below. You are rewriting, not writing from scratch -- preserve what works, fix what's broken.

## Revision Brief
%s

## Voice Guidelines
%s

## Characters (excerpt)
%s

## World (excerpt)
%s

## Previous Chapter (last 2000 chars)
%s

## Next Chapter (first 1500 chars)
%s

## Old Chapter %d (THE TEXT TO REWRITE)
%s

RULES:
1. Follow the brief's WHAT TO CHANGE section precisely.
2. Preserve the brief's WHAT TO KEEP section completely.
3. Hit the word count TARGET in the brief.
4. Maintain continuity with adjacent chapters.
5. No AI slop: no delve, utilize, leverage, tapestry, symphony, testament.
6. No triadic listings, no "not just X but Y", no sycophantic openers.
7. Show don't tell. Never name an emotion.
8. Vary sentence length. Short sentences have power.
9. Every scene needs micro-conflict.
10. Subtext > text. Characters rarely say what they mean.`,
				args.Chapter,
				brief,
				truncateForContext(voice, 3000),
				truncateForContext(chars, 3000),
				truncateForContext(world, 3000),
				prevTail, nextHead,
				args.Chapter, oldText)

			resp, err := rt.client.Message(anthropic.Request{
				Model:       rt.writerModel,
				System:      "You are rewriting a chapter of a fantasy novel. Preserve the voice and what works. Fix what the brief identifies. You are a surgeon, not a demolition crew.",
				Prompt:      prompt,
				MaxTokens:   16000,
				Temperature: 0.8,
			})
			if err != nil {
				return "", err
			}

			if err := p.SaveChapter(args.Chapter, resp.Text); err != nil {
				return "", err
			}

			oldWords := anthropic.CountWords(oldText)
			newWords := anthropic.CountWords(resp.Text)
			return fmt.Sprintf("Chapter %d revised: %d -> %d words", args.Chapter, oldWords, newWords), nil
		})

	skill.AddTool(s, "review_manuscript",
		"Deep manuscript review via Opus. Sends the full novel for dual-persona review: literary critic then professor of fiction. Returns star rating, actionable items, and stopping condition assessment.",
		func(ctx context.Context, args ReviewManuscriptArgs) (string, error) {
			p := project.New(args.ProjectDir)
			chapters, err := p.LoadAllChapters()
			if err != nil || len(chapters) == 0 {
				return "", fmt.Errorf("no chapters found")
			}

			chNums, _ := p.ChapterNumbers()
			var manuscript strings.Builder
			for _, n := range chNums {
				if manuscript.Len() > 0 {
					manuscript.WriteString("\n\n---\n\n")
				}
				manuscript.WriteString(chapters[n])
			}

			title := extractTitle(p)

			review, err := evaluate.Review(rt.client, rt.reviewModel, title, manuscript.String())
			if err != nil {
				return "", err
			}

			p.SaveEditLog("review.json", structToMap(review))

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("# Manuscript Review\n\nStars: %.1f/5\n", review.Stars))
			sb.WriteString(fmt.Sprintf("Items: %d total, %d major, %d qualified\n", review.TotalItems, review.MajorItems, review.QualifiedItems))
			if review.ShouldStop {
				sb.WriteString(fmt.Sprintf("STOP: %s\n", review.StopReason))
			}
			sb.WriteString("\n## Critic Summary\n" + review.CriticSummary + "\n")
			if len(review.ProfessorItems) > 0 {
				sb.WriteString("\n## Professor Items\n")
				for i, item := range review.ProfessorItems {
					sb.WriteString(fmt.Sprintf("%d. [%s/%s] %s\n", i+1, item.Severity, item.Type, item.Text))
				}
			}

			return sb.String(), nil
		})

	skill.AddTool(s, "compare_chapters",
		"Head-to-head chapter comparison. Provide two chapter numbers for a single matchup, or omit both for a full Swiss-style Elo tournament across all chapters.",
		func(ctx context.Context, args CompareChaptersArgs) (string, error) {
			p := project.New(args.ProjectDir)

			if args.ChapterA > 0 && args.ChapterB > 0 {
				textA, _ := p.LoadChapter(args.ChapterA)
				textB, _ := p.LoadChapter(args.ChapterB)
				if textA == "" || textB == "" {
					return "", fmt.Errorf("both chapters must exist")
				}

				result, err := evaluate.CompareChapters(rt.client, rt.judgeModel,
					args.ChapterA, args.ChapterB, textA, textB)
				if err != nil {
					return "", err
				}
				b, _ := json.MarshalIndent(result, "", "  ")
				return string(b), nil
			}

			// Full tournament.
			chapters, err := p.LoadAllChapters()
			if err != nil || len(chapters) < 2 {
				return "", fmt.Errorf("need at least 2 chapters for tournament")
			}

			chNums, _ := p.ChapterNumbers()
			elo := make(map[int]float64)
			for _, n := range chNums {
				elo[n] = 1500
			}

			var matchups []string
			rounds := 4
			for round := 0; round < rounds; round++ {
				// Sort by Elo, pair adjacent.
				sorted := make([]int, len(chNums))
				copy(sorted, chNums)
				sort.Slice(sorted, func(i, j int) bool {
					return elo[sorted[i]] > elo[sorted[j]]
				})

				for i := 0; i+1 < len(sorted); i += 2 {
					a, b := sorted[i], sorted[i+1]
					result, err := evaluate.CompareChapters(rt.client, rt.judgeModel,
						a, b, chapters[a], chapters[b])
					if err != nil {
						continue
					}

					// Update Elo.
					eloA, eloB := elo[a], elo[b]
					expectedA := 1.0 / (1.0 + math.Pow(10, (eloB-eloA)/400))
					k := 32.0
					if result.Winner == a {
						elo[a] = eloA + k*(1-expectedA)
						elo[b] = eloB + k*(expectedA-1)
					} else {
						elo[a] = eloA + k*(0-expectedA)
						elo[b] = eloB + k*(1-expectedA)
					}

					matchups = append(matchups, fmt.Sprintf("R%d: Ch %d vs Ch %d -> %s (%s)",
						round+1, a, b, result.WinnerChapter, result.Margin))
				}
			}

			// Build ranking.
			sorted := make([]int, len(chNums))
			copy(sorted, chNums)
			sort.Slice(sorted, func(i, j int) bool {
				return elo[sorted[i]] > elo[sorted[j]]
			})

			var sb strings.Builder
			sb.WriteString("# Chapter Tournament Results\n\n## Rankings\n")
			for i, ch := range sorted {
				sb.WriteString(fmt.Sprintf("%2d. Chapter %d (Elo: %.0f)\n", i+1, ch, elo[ch]))
			}
			sb.WriteString("\n## Matchups\n")
			for _, m := range matchups {
				sb.WriteString(m + "\n")
			}

			tournament := map[string]any{
				"ranking":  sorted,
				"elo":      elo,
				"matchups": matchups,
			}
			p.SaveEditLog("tournament_results.json", tournament)

			return sb.String(), nil
		})
}

// Brief builders.

func buildPanelBrief(p *project.Project, ch, wordCount int) (string, string) {
	panelData, err := p.LoadEditLog("reader_panel.json")
	if err != nil || panelData == nil {
		return "", ""
	}
	var panel evaluate.PanelResult
	if err := json.Unmarshal(panelData, &panel); err != nil {
		return "", ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Revision Brief: Chapter %d (PANEL)\n\n", ch))
	sb.WriteString("## PROBLEM\nReader panel flagged issues with this chapter.\n\n")

	// Collect mentions of this chapter.
	sb.WriteString("## WHAT TO CHANGE\n")
	for persona, answers := range panel.Readers {
		for q, a := range answers {
			if s, ok := a.(string); ok && strings.Contains(s, fmt.Sprintf("%d", ch)) {
				sb.WriteString(fmt.Sprintf("- %s (%s): %s\n", q, persona, s))
			}
		}
	}

	sb.WriteString("\n## WHAT TO KEEP\nPreserve all elements not mentioned above.\n\n")
	sb.WriteString("## VOICE RULES\nMaintain established voice, sentence rhythm, vocabulary wells.\n\n")
	sb.WriteString(fmt.Sprintf("## TARGET\nWord count: ~%d (85%% of current %d)\n", int(float64(wordCount)*0.85), wordCount))

	return sb.String(), "panel"
}

func buildEvalBrief(p *project.Project, ch, wordCount int) (string, string) {
	evalData, _ := p.LatestEvalLog(fmt.Sprintf("ch%02d", ch))
	if evalData == nil {
		return "", ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Revision Brief: Chapter %d (EVAL)\n\n", ch))

	// Extract weak dimensions.
	sb.WriteString("## PROBLEM\n")
	for key, val := range evalData {
		if m, ok := val.(map[string]any); ok {
			if score, ok := m["score"].(float64); ok && score <= 7 {
				sb.WriteString(fmt.Sprintf("- %s: %.1f/10", key, score))
				if fix, ok := m["fix"].(string); ok && fix != "" {
					sb.WriteString(fmt.Sprintf(" -- %s", fix))
				}
				sb.WriteString("\n")
			}
		}
	}

	sb.WriteString("\n## WHAT TO CHANGE\n")
	if revisions, ok := evalData["top_3_revisions"].([]any); ok {
		for _, r := range revisions {
			sb.WriteString(fmt.Sprintf("- %v\n", r))
		}
	}

	sb.WriteString("\n## WHAT TO KEEP\n")
	if strongest, ok := evalData["three_strongest_sentences"].([]any); ok {
		for _, s := range strongest {
			sb.WriteString(fmt.Sprintf("- \"%v\"\n", s))
		}
	}

	sb.WriteString("\n## VOICE RULES\nMaintain established voice.\n\n")

	briefType := "REVISE"
	if score, ok := evalData["overall_score"].(float64); ok {
		if score < 4 {
			briefType = "REWRITE"
		} else if score < 6 {
			briefType = "FIX"
		} else {
			briefType = "POLISH"
		}
	}
	sb.WriteString(fmt.Sprintf("## TARGET\nType: %s\nWord count: ~%d\n", briefType, wordCount))

	return sb.String(), "eval"
}

func buildCutsBrief(p *project.Project, ch, wordCount int) (string, string) {
	cutsData, _ := p.LoadEditLog(fmt.Sprintf("ch%02d_cuts.json", ch))
	if cutsData == nil {
		return "", ""
	}
	var cuts evaluate.CutResult
	if err := json.Unmarshal(cutsData, &cuts); err != nil {
		return "", ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Revision Brief: Chapter %d (CUTS)\n\n", ch))
	sb.WriteString(fmt.Sprintf("## PROBLEM\n%s\n\n", cuts.OneSentenceVerdict))

	// Group by type.
	typeGroups := make(map[string][]evaluate.Cut)
	for _, c := range cuts.Cuts {
		typeGroups[c.Type] = append(typeGroups[c.Type], c)
	}

	sb.WriteString("## WHAT TO CHANGE\n")
	for t, group := range typeGroups {
		sb.WriteString(fmt.Sprintf("\n### %s (%d instances)\n", t, len(group)))
		for _, c := range group {
			sb.WriteString(fmt.Sprintf("- \"%s\" -> %s: %s\n",
				truncateQuote(c.Quote, 80), c.Action, c.Reason))
		}
	}

	sb.WriteString(fmt.Sprintf("\n## WHAT TO KEEP\nTightest passage: \"%s\"\n", truncateQuote(cuts.TightestPassage, 120)))
	sb.WriteString("\n## VOICE RULES\nMaintain established voice.\n\n")

	target := wordCount - cuts.TotalCuttable
	sb.WriteString(fmt.Sprintf("## TARGET\nWord count: ~%d (current %d minus %d cuttable)\n", target, wordCount, cuts.TotalCuttable))

	return sb.String(), "cuts"
}

func buildAutoBrief(p *project.Project, rt *runtime, ch, wordCount int) (string, string) {
	// Try eval brief first, fall back to cuts, then panel.
	if brief, typ := buildEvalBrief(p, ch, wordCount); brief != "" {
		return brief, typ
	}
	if brief, typ := buildCutsBrief(p, ch, wordCount); brief != "" {
		return brief, typ
	}
	if brief, typ := buildPanelBrief(p, ch, wordCount); brief != "" {
		return brief, typ
	}
	return "", ""
}

func extractTitle(p *project.Project) string {
	outline, _ := p.Outline()
	if outline != "" {
		lines := strings.SplitN(outline, "\n", 2)
		if len(lines) > 0 {
			title := strings.TrimSpace(strings.TrimPrefix(lines[0], "#"))
			if title != "" {
				return title
			}
		}
	}
	ch1, _ := p.LoadChapter(1)
	if ch1 != "" {
		lines := strings.SplitN(ch1, "\n", 2)
		if len(lines) > 0 {
			return strings.TrimSpace(strings.TrimPrefix(lines[0], "#"))
		}
	}
	return "Untitled Novel"
}

func truncateQuote(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
