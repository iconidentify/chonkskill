package autonovel

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/iconidentify/chonkskill/pkg/skill"
	"github.com/iconidentify/chonkskill/pkg/anthropic"
	"github.com/iconidentify/chonkskill/skills/autonovel/internal/evaluate"
	"github.com/iconidentify/chonkskill/pkg/project"
)

// Pipeline constants matching the original autonovel defaults.
const (
	FoundationThreshold = 7.5
	ChapterThreshold    = 6.0
	MaxFoundationIters  = 20
	MaxChapterAttempts  = 5
	MinRevisionCycles   = 3
	MaxRevisionCycles   = 6
	PlateauDelta        = 0.3
)

func registerPipelineTools(s *skill.Skill, rt *runtime) {
	skill.AddTool(s, "init_project",
		"Initialize a new novel project directory with all required subdirectories and state files",
		func(ctx context.Context, args InitProjectArgs) (string, error) {
			if args.ProjectDir == "" {
				return "", fmt.Errorf("project_dir is required")
			}
			p := project.New(args.ProjectDir)
			if err := p.Init(); err != nil {
				return "", err
			}
			return fmt.Sprintf("Project initialized at %s. Directories created: chapters, briefs, eval_logs, edit_logs, art, audiobook.", args.ProjectDir), nil
		})

	skill.AddTool(s, "get_state",
		"Get current pipeline state including phase, scores, chapter counts, and debts",
		func(ctx context.Context, args GetStateArgs) (string, error) {
			p := project.New(args.ProjectDir)
			state, err := p.LoadState()
			if err != nil {
				return "", err
			}
			wordCount, _ := p.CountAllWords()
			chNums, _ := p.ChapterNumbers()

			type stateOutput struct {
				project.State
				TotalWords     int   `json:"total_words"`
				ChaptersOnDisk []int `json:"chapters_on_disk"`
			}
			out := stateOutput{
				State:          state,
				TotalWords:     wordCount,
				ChaptersOnDisk: chNums,
			}
			b, _ := json.MarshalIndent(out, "", "  ")
			return string(b), nil
		})

	skill.AddTool(s, "run_pipeline",
		"Run the autonomous novel pipeline. Executes the modify-evaluate-keep/discard loop for the specified phase(s). Commits improvements via git, resets failures.",
		func(ctx context.Context, args RunPipelineArgs) (string, error) {
			if args.ProjectDir == "" {
				return "", fmt.Errorf("project_dir is required")
			}
			p := project.New(args.ProjectDir)
			state, err := p.LoadState()
			if err != nil {
				return "", err
			}
			if args.FromScratch {
				state = project.DefaultState()
				if err := p.SaveState(state); err != nil {
					return "", err
				}
			}

			maxCycles := args.MaxCycles
			if maxCycles == 0 {
				maxCycles = MaxRevisionCycles
			}

			phases := []string{"foundation", "drafting", "revision", "export"}
			if args.Phase != "" {
				phases = []string{args.Phase}
			} else {
				// Resume from current phase.
				idx := 0
				for i, ph := range phases {
					if ph == state.Phase {
						idx = i
						break
					}
				}
				phases = phases[idx:]
			}

			var results []string
			for _, phase := range phases {
				state.Phase = phase
				if err := p.SaveState(state); err != nil {
					return "", err
				}

				var phaseResult string
				var phaseErr error
				switch phase {
				case "foundation":
					phaseResult, state, phaseErr = runFoundation(p, rt, state)
				case "drafting":
					phaseResult, state, phaseErr = runDrafting(p, rt, state)
				case "revision":
					phaseResult, state, phaseErr = runRevision(p, rt, state, maxCycles)
				case "export":
					phaseResult, state, phaseErr = runExport(p, rt, state)
				default:
					return "", fmt.Errorf("unknown phase: %s", phase)
				}

				if phaseErr != nil {
					results = append(results, fmt.Sprintf("[%s] ERROR: %v", phase, phaseErr))
					break
				}
				results = append(results, fmt.Sprintf("[%s] %s", phase, phaseResult))

				if err := p.SaveState(state); err != nil {
					return "", err
				}
			}

			return strings.Join(results, "\n"), nil
		})
}

func runFoundation(p *project.Project, rt *runtime, state project.State) (string, project.State, error) {
	seed, err := p.Seed()
	if err != nil || seed == "" {
		return "", state, fmt.Errorf("seed.txt is required -- use autonovel:generate_seed first")
	}

	for i := state.Iteration; i < MaxFoundationIters; i++ {
		state.Iteration = i

		// Generate all foundation layers.
		if err := generateWorld(p, rt); err != nil {
			return "", state, fmt.Errorf("gen_world failed: %w", err)
		}
		if err := generateCharacters(p, rt); err != nil {
			return "", state, fmt.Errorf("gen_characters failed: %w", err)
		}
		if err := generateOutline(p, rt); err != nil {
			return "", state, fmt.Errorf("gen_outline failed: %w", err)
		}
		if err := generateCanon(p, rt); err != nil {
			return "", state, fmt.Errorf("gen_canon failed: %w", err)
		}

		// Evaluate.
		voice, _ := p.Voice()
		world, _ := p.World()
		chars, _ := p.Characters()
		outline, _ := p.Outline()
		canon, _ := p.Canon()

		result, err := evaluate.Foundation(rt.client, rt.judgeModel, voice, world, chars, outline, canon)
		if err != nil {
			return "", state, fmt.Errorf("evaluate failed: %w", err)
		}

		if result.OverallScore > state.FoundationScore {
			state.FoundationScore = result.OverallScore
			state.LoreScore = result.LoreScore
			p.LogResult(project.ResultEntry{
				Phase:       "foundation",
				Score:       result.OverallScore,
				Status:      "keep",
				Description: fmt.Sprintf("iteration %d, lore=%.1f", i, result.LoreScore),
			})
		} else {
			p.LogResult(project.ResultEntry{
				Phase:       "foundation",
				Score:       result.OverallScore,
				Status:      "discard",
				Description: fmt.Sprintf("iteration %d, no improvement", i),
			})
		}

		if state.FoundationScore >= FoundationThreshold {
			return fmt.Sprintf("Foundation complete: score=%.1f, lore=%.1f after %d iterations",
				state.FoundationScore, state.LoreScore, i+1), state, nil
		}
	}

	return fmt.Sprintf("Foundation finished after %d iterations: score=%.1f (threshold %.1f not reached)",
		MaxFoundationIters, state.FoundationScore, FoundationThreshold), state, nil
}

func runDrafting(p *project.Project, rt *runtime, state project.State) (string, project.State, error) {
	totalCh, err := p.GetTotalChapters(state)
	if err != nil {
		return "", state, err
	}
	state.ChaptersTotal = totalCh

	for ch := state.ChaptersDrafted + 1; ch <= totalCh; ch++ {
		var bestScore float64
		for attempt := 0; attempt < MaxChapterAttempts; attempt++ {
			if err := draftSingleChapter(p, rt, ch); err != nil {
				return "", state, fmt.Errorf("drafting ch %d attempt %d: %w", ch, attempt+1, err)
			}

			// Evaluate the chapter.
			chapterText, _ := p.LoadChapter(ch)
			voice, _ := p.Voice()
			world, _ := p.World()
			chars, _ := p.Characters()
			outline, _ := p.Outline()
			canon, _ := p.Canon()

			chOutline := p.ExtractChapterOutline(outline, ch)
			prevTail := ""
			if ch > 1 {
				if prev, _ := p.LoadChapter(ch - 1); prev != "" {
					prevTail = project.LastNChars(prev, 2000)
				}
			}
			nextHead := ""
			if next, _ := p.LoadChapter(ch + 1); next != "" {
				nextHead = project.FirstNChars(next, 1500)
			}

			result, err := evaluate.Chapter(rt.client, rt.judgeModel, ch, chapterText,
				voice, world, chars, chOutline, canon, prevTail, nextHead)
			if err != nil {
				continue
			}

			if result.OverallScore > bestScore {
				bestScore = result.OverallScore
			}

			p.LogResult(project.ResultEntry{
				Phase:       "drafting",
				Score:       result.OverallScore,
				WordCount:   anthropic.CountWords(chapterText),
				Status:      fmt.Sprintf("ch%d_attempt%d", ch, attempt+1),
				Description: fmt.Sprintf("weakest: %s", result.WeakestDimension),
			})

			if result.OverallScore >= ChapterThreshold {
				break
			}
		}

		state.ChaptersDrafted = ch
		if err := p.SaveState(state); err != nil {
			return "", state, err
		}
	}

	wordCount, _ := p.CountAllWords()
	return fmt.Sprintf("Drafting complete: %d chapters, %d words", state.ChaptersDrafted, wordCount), state, nil
}

func runRevision(p *project.Project, rt *runtime, state project.State, maxCycles int) (string, project.State, error) {
	prevScore := state.NovelScore

	for cycle := state.RevisionCycle; cycle < maxCycles; cycle++ {
		state.RevisionCycle = cycle

		// Phase 3a: adversarial editing + revision per chapter.
		chNums, _ := p.ChapterNumbers()
		for _, ch := range chNums {
			chText, _ := p.LoadChapter(ch)
			if chText == "" {
				continue
			}

			// Adversarial edit.
			cuts, err := evaluate.AdversarialEdit(rt.client, rt.judgeModel, chText, ch)
			if err != nil {
				continue
			}
			p.SaveEditLog(fmt.Sprintf("ch%02d_cuts.json", ch), cuts)

			// Apply cuts.
			if cuts.OverallFatPct > 5 {
				modified := applyCutsToText(chText, cuts)
				if modified != chText {
					p.SaveChapter(ch, modified)
				}
			}
		}

		// Build arc summary for reader panel.
		buildArcSummaryForProject(p, rt)

		// Reader panel.
		arcSummary, _ := p.ArcSummary()
		if arcSummary != "" {
			panel, err := evaluate.RunReaderPanel(rt.client, rt.judgeModel, arcSummary)
			if err == nil {
				p.SaveEditLog("reader_panel.json", panel)
			}
		}

		// Full evaluation.
		chapters, _ := p.LoadAllChapters()
		voice, _ := p.Voice()
		world, _ := p.World()
		chars, _ := p.Characters()
		outline, _ := p.Outline()
		canon, _ := p.Canon()
		arcSum, _ := p.ArcSummary()

		fullResult, err := evaluate.Full(rt.client, rt.judgeModel, chapters, voice, world, chars, outline, canon, arcSum)
		if err != nil {
			continue
		}

		state.NovelScore = fullResult.NovelScore
		p.SaveState(state)

		p.LogResult(project.ResultEntry{
			Phase:       "revision",
			Score:       fullResult.NovelScore,
			Status:      fmt.Sprintf("cycle_%d", cycle),
			Description: fmt.Sprintf("weakest_ch=%d, weakest_dim=%s", fullResult.WeakestChapter, fullResult.WeakestDimension),
		})

		// Plateau detection.
		if cycle >= MinRevisionCycles && math.Abs(fullResult.NovelScore-prevScore) < PlateauDelta {
			return fmt.Sprintf("Revision plateaued at cycle %d: score=%.1f", cycle, fullResult.NovelScore), state, nil
		}
		prevScore = fullResult.NovelScore
	}

	// Phase 3b: Opus review loop.
	for round := 0; round < 4; round++ {
		chapters, _ := p.LoadAllChapters()
		var manuscript strings.Builder
		chNums, _ := p.ChapterNumbers()
		for _, n := range chNums {
			manuscript.WriteString(fmt.Sprintf("\n\n--- Chapter %d ---\n\n%s", n, chapters[n]))
		}

		title := "Untitled Novel"
		if outline, _ := p.Outline(); outline != "" {
			if lines := strings.SplitN(outline, "\n", 2); len(lines) > 0 {
				title = strings.TrimSpace(strings.TrimPrefix(lines[0], "#"))
			}
		}

		review, err := evaluate.Review(rt.client, rt.reviewModel, title, manuscript.String())
		if err != nil {
			break
		}

		p.SaveEditLog(fmt.Sprintf("review_round_%d.json", round), review)

		if review.ShouldStop {
			return fmt.Sprintf("Review complete at round %d: %.1f stars, %s", round, review.Stars, review.StopReason), state, nil
		}
	}

	return fmt.Sprintf("Revision complete: score=%.1f after %d cycles", state.NovelScore, maxCycles), state, nil
}

func runExport(p *project.Project, rt *runtime, state project.State) (string, project.State, error) {
	// Rebuild outline from actual chapters.
	if err := rebuildOutline(p, rt); err != nil {
		return "", state, fmt.Errorf("rebuild outline: %w", err)
	}

	// Build arc summary.
	if err := buildArcSummaryForProject(p, rt); err != nil {
		return "", state, fmt.Errorf("build arc summary: %w", err)
	}

	// Concatenate manuscript.
	chapters, _ := p.LoadAllChapters()
	chNums, _ := p.ChapterNumbers()
	var manuscript strings.Builder
	for _, n := range chNums {
		if manuscript.Len() > 0 {
			manuscript.WriteString("\n\n---\n\n")
		}
		manuscript.WriteString(chapters[n])
	}
	p.SaveFile("manuscript.md", manuscript.String())

	wordCount, _ := p.CountAllWords()
	return fmt.Sprintf("Export complete: manuscript.md written (%d words, %d chapters)", wordCount, len(chNums)), state, nil
}

// applyCutsToText applies adversarial edit cuts to chapter text.
// Uses normalized whitespace matching for quote lookup.
func applyCutsToText(text string, cuts *evaluate.CutResult) string {
	for _, cut := range cuts.Cuts {
		if len(cut.Quote) < 25 {
			continue
		}
		// Try exact match first.
		if idx := strings.Index(text, cut.Quote); idx >= 0 {
			replacement := ""
			if cut.Action == "REWRITE" && cut.Rewrite != "" {
				replacement = cut.Rewrite
			}
			text = text[:idx] + replacement + text[idx+len(cut.Quote):]
			continue
		}
		// Normalized whitespace match.
		normalized := normalizeWhitespace(cut.Quote)
		textNormalized := normalizeWhitespace(text)
		if idx := strings.Index(textNormalized, normalized); idx >= 0 {
			// Map back to original text position (approximate).
			origIdx := mapNormalizedIndex(text, idx)
			origEnd := mapNormalizedIndex(text, idx+len(normalized))
			if origIdx >= 0 && origEnd > origIdx && origEnd <= len(text) {
				replacement := ""
				if cut.Action == "REWRITE" && cut.Rewrite != "" {
					replacement = cut.Rewrite
				}
				text = text[:origIdx] + replacement + text[origEnd:]
			}
		}
	}
	// Collapse triple+ newlines to double.
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return text
}

func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func mapNormalizedIndex(original string, normalizedIdx int) int {
	normalizedPos := 0
	inSpace := false
	for i, ch := range original {
		if ch == ' ' || ch == '\n' || ch == '\t' || ch == '\r' {
			if !inSpace {
				inSpace = true
				normalizedPos++
			}
		} else {
			inSpace = false
			normalizedPos++
		}
		if normalizedPos > normalizedIdx {
			return i
		}
	}
	return len(original)
}
