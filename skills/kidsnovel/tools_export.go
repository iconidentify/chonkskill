package kidsnovel

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/iconidentify/chonkskill/pkg/anthropic"
	"github.com/iconidentify/chonkskill/pkg/imagegen"
	"github.com/iconidentify/chonkskill/pkg/project"
	"github.com/iconidentify/chonkskill/pkg/skill"
	"github.com/iconidentify/chonkskill/skills/kidsnovel/internal/readability"
)

func registerExportTools(s *skill.Skill, rt *runtime) {
	skill.AddTool(s, "build_book",
		"Assemble the final book: concatenate all chapters into manuscript.md with title page, dedication, and chapter headers. Reports final readability stats.",
		func(ctx context.Context, args BuildBookArgs) (string, error) {
			p := project.New(args.ProjectDir)
			grade := loadGrade(p)
			spec := readability.GradeSpecs[grade]

			chapters, err := p.LoadAllChapters()
			if err != nil || len(chapters) == 0 {
				return "", fmt.Errorf("no chapters found")
			}
			chNums, _ := p.ChapterNumbers()

			// Build title page.
			title := "Untitled Book"
			if outline, _ := p.Outline(); outline != "" {
				lines := strings.SplitN(outline, "\n", 2)
				if len(lines) > 0 {
					t := strings.TrimSpace(strings.TrimLeft(lines[0], "#"))
					if t != "" {
						title = t
					}
				}
			}

			coAuthor := ""
			if name, _ := p.LoadFile("co_author.txt"); name != "" {
				coAuthor = fmt.Sprintf("Co-created with %s", strings.TrimSpace(name))
			}

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("# %s\n\n", title))
			if coAuthor != "" {
				sb.WriteString(fmt.Sprintf("*%s*\n\n", coAuthor))
			}
			sb.WriteString("---\n\n")

			for _, n := range chNums {
				if sb.Len() > len(title)+50 {
					sb.WriteString("\n\n---\n\n")
				}
				sb.WriteString(chapters[n])
			}

			sb.WriteString("\n\n---\n\n*The End*\n")

			if err := p.SaveFile("manuscript.md", sb.String()); err != nil {
				return "", err
			}

			// Final readability stats.
			totalWords, _ := p.CountAllWords()
			fullAnalysis := readability.Analyze(sb.String(), grade)

			return fmt.Sprintf("Book assembled: %s\n  %d chapters, %d words\n  FK Grade: %.1f (target: %s, %.1f-%.1f)\n  Grade fit: %s\n  Saved to: manuscript.md",
				title, len(chNums), totalWords,
				fullAnalysis.FleschKincaid, spec.Label, spec.FKMin, spec.FKMax,
				fullAnalysis.GradeFit), nil
		})

	skill.AddTool(s, "gen_illustration",
		"Generate an illustration for a chapter or the book cover using fal.ai. Style options: cartoon, watercolor, pencil, comic, whimsical. Omit chapter for cover art.",
		func(ctx context.Context, args GenIllustrationArgs) (string, error) {
			if rt.imageClient == nil {
				return "", fmt.Errorf("image_model is required for illustration generation")
			}
			p := project.New(args.ProjectDir)
			grade := loadGrade(p)
			spec := readability.GradeSpecs[grade]

			style := args.Style
			if style == "" {
				style = "whimsical"
			}

			var artPrompt string
			var destPath string
			var resolution string

			if args.Chapter > 0 {
				// Chapter illustration.
				text, _ := p.LoadChapter(args.Chapter)
				if text == "" {
					return "", fmt.Errorf("chapter %d not found", args.Chapter)
				}

				// Ask the writer model to pick the most visual moment.
				scenePrompt := fmt.Sprintf(`Pick the single most visual moment from this chapter of a children's book and describe it as an illustration prompt.

%s

Return ONLY the illustration prompt (no explanation). It should describe:
- Who is in the scene (characters, age %d-%d)
- What they're doing
- The setting/background
- The mood/emotion
- Key visual details

Keep it to 2-3 sentences.`, truncate(text, 4000), spec.Grade+5, spec.Grade+8)

				resp, err := rt.client.Message(anthropic.Request{
					Model:       rt.writerModel,
					System:      "You pick the most illustration-worthy moment from children's book chapters. Return only the scene description.",
					Prompt:      scenePrompt,
					MaxTokens:   300,
					Temperature: 0.3,
				})
				if err != nil {
					return "", err
				}

				artPrompt = fmt.Sprintf("Children's book illustration, %s style: %s. For kids ages %d-%d. Friendly, colorful, age-appropriate.",
					style, resp.Text, spec.Grade+5, spec.Grade+8)
				destPath = filepath.Join(p.Dir, fmt.Sprintf("art/ch%02d.png", args.Chapter))
				resolution = "1024x1024"
			} else {
				// Cover art.
				seed, _ := p.Seed()
				artPrompt = fmt.Sprintf("Children's book cover, %s style. %s. For kids ages %d-%d. Eye-catching, colorful, no text on the image.",
					style, truncate(seed, 500), spec.Grade+5, spec.Grade+8)
				destPath = filepath.Join(p.Dir, "art/cover.png")
				resolution = "1024x1536"
			}

			result, err := rt.imageClient.Generate(imagegen.GenerateParams{
				Prompt: artPrompt,
				Size:   resolution,
			})
			if err != nil {
				return "", fmt.Errorf("generation failed: %w", err)
			}

			if result.ImageURL == "" {
				return "", fmt.Errorf("no image URL in response")
			}

			bytes, err := imagegen.DownloadImage(result.ImageURL, destPath)
			if err != nil {
				return "", fmt.Errorf("download failed: %w", err)
			}

			return fmt.Sprintf("Illustration generated: %s (%d bytes)\nStyle: %s\nPrompt: %s",
				filepath.Base(destPath), bytes, style, artPrompt), nil
		})

	skill.AddTool(s, "run_pipeline",
		"Run the full book creation pipeline: foundation, drafting, readability enforcement, revision, and export. Use this for hands-off book generation after the seed is set.",
		func(ctx context.Context, args RunPipelineArgs) (string, error) {
			p := project.New(args.ProjectDir)
			grade := loadGrade(p)
			spec := readability.GradeSpecs[grade]
			state, _ := p.LoadState()

			phases := []string{"foundation", "drafting", "revision", "export"}
			if args.Phase != "" {
				phases = []string{args.Phase}
			}

			var results []string
			for _, phase := range phases {
				state.Phase = phase
				p.SaveState(state)

				switch phase {
				case "foundation":
					seed, _ := p.Seed()
					if seed == "" {
						return "", fmt.Errorf("seed.txt required -- use from_kid_writing, from_idea, or generate_seed first")
					}
					// Generate foundation.
					if err := generateWorldForPipeline(p, rt); err != nil {
						results = append(results, fmt.Sprintf("[foundation] gen_world ERROR: %v", err))
						continue
					}
					if err := generateCharsForPipeline(p, rt); err != nil {
						results = append(results, fmt.Sprintf("[foundation] gen_characters ERROR: %v", err))
						continue
					}
					if err := generateOutlineForPipeline(p, rt); err != nil {
						results = append(results, fmt.Sprintf("[foundation] gen_outline ERROR: %v", err))
						continue
					}
					results = append(results, "[foundation] complete")

				case "drafting":
					totalCh, _ := p.GetTotalChapters(state)
					if totalCh == 0 {
						totalCh = spec.ChapterCount
					}
					for ch := 1; ch <= totalCh; ch++ {
						if err := draftChapter(p, rt, ch); err != nil {
							results = append(results, fmt.Sprintf("[drafting] ch %d ERROR: %v", ch, err))
							continue
						}
						state.ChaptersDrafted = ch
						p.SaveState(state)
					}
					wc, _ := p.CountAllWords()
					results = append(results, fmt.Sprintf("[drafting] %d chapters, %d words", state.ChaptersDrafted, wc))

				case "revision":
					// Simplify any chapters that are too hard.
					chNums, _ := p.ChapterNumbers()
					simplified := 0
					for _, ch := range chNums {
						text, _ := p.LoadChapter(ch)
						if text == "" {
							continue
						}
						a := readability.Analyze(text, grade)
						if a.GradeFit == "too-hard" {
							// Run simplification via the writer model.
							simplifyForPipeline(p, rt, ch, grade)
							simplified++
						}
					}
					results = append(results, fmt.Sprintf("[revision] simplified %d/%d chapters", simplified, len(chNums)))

				case "export":
					// Build manuscript.
					chapters, _ := p.LoadAllChapters()
					chNums, _ := p.ChapterNumbers()
					var ms strings.Builder
					for _, n := range chNums {
						if ms.Len() > 0 {
							ms.WriteString("\n\n---\n\n")
						}
						ms.WriteString(chapters[n])
					}
					p.SaveFile("manuscript.md", ms.String())
					wc, _ := p.CountAllWords()
					results = append(results, fmt.Sprintf("[export] manuscript.md (%d words)", wc))
				}
			}

			return strings.Join(results, "\n"), nil
		})
}

// Pipeline helpers that call the same logic as the tools but without args parsing.
func generateWorldForPipeline(p *project.Project, rt *runtime) error {
	grade := loadGrade(p)
	spec := readability.GradeSpecs[grade]
	seed, _ := p.Seed()

	prompt := fmt.Sprintf(`Create a concise world/setting for a %s children's book.

## Concept
%s

Include: main location, 3-5 key places, rules (if fantasy/sci-fi), sensory details, what's normal here.
Keep it vivid and concrete. A kid should be able to draw the main location.
Ages %d-%d appropriate.`, spec.Label, truncate(seed, 3000), spec.Grade+5, spec.Grade+8)

	resp, err := rt.client.Message(anthropic.Request{
		Model: rt.writerModel, System: "You build story worlds for children's books.",
		Prompt: prompt, MaxTokens: 6000, Temperature: 0.7,
	})
	if err != nil {
		return err
	}
	return p.SaveWorld(resp.Text)
}

func generateCharsForPipeline(p *project.Project, rt *runtime) error {
	grade := loadGrade(p)
	spec := readability.GradeSpecs[grade]
	seed, _ := p.Seed()
	world, _ := p.World()

	prompt := fmt.Sprintf(`Create characters for a %s children's book.

## Concept
%s

## World
%s

Create protagonist + 3-4 supporting characters. For each: who they are, what they want, their flaw, their strength, how they talk (3 sample lines). Ages %d-%d appropriate.`,
		spec.Label, truncate(seed, 2000), truncate(world, 2000), spec.Grade+5, spec.Grade+8)

	resp, err := rt.client.Message(anthropic.Request{
		Model: rt.writerModel, System: "You create characters for children's books that feel like real kids.",
		Prompt: prompt, MaxTokens: 6000, Temperature: 0.7,
	})
	if err != nil {
		return err
	}
	return p.SaveCharacters(resp.Text)
}

func generateOutlineForPipeline(p *project.Project, rt *runtime) error {
	grade := loadGrade(p)
	spec := readability.GradeSpecs[grade]
	seed, _ := p.Seed()
	world, _ := p.World()
	chars, _ := p.Characters()

	prompt := fmt.Sprintf(`Create a %d-chapter outline for a %s children's book.

## Concept
%s
## World
%s
## Characters
%s

For each chapter: title, what happens (2-3 sentences), the fun part, how it ends.
~%d words per chapter, %d-%d total.`, spec.ChapterCount, spec.Label,
		truncate(seed, 2000), truncate(world, 2000), truncate(chars, 2000),
		spec.ChapterWordTarget, spec.BookWordMin, spec.BookWordMax)

	resp, err := rt.client.Message(anthropic.Request{
		Model: rt.writerModel, System: "You outline children's chapter books. Every chapter earns its place.",
		Prompt: prompt, MaxTokens: 10000, Temperature: 0.5,
	})
	if err != nil {
		return err
	}
	return p.SaveOutline(resp.Text)
}

func simplifyForPipeline(p *project.Project, rt *runtime, ch, grade int) {
	spec := readability.GradeSpecs[grade]
	text, _ := p.LoadChapter(ch)
	if text == "" {
		return
	}

	prompt := fmt.Sprintf(`Simplify this chapter to %s reading level (FK %.1f-%.1f). Break long sentences. Replace complex words. Add dialogue. Keep the story exactly the same.

%s

Return the complete simplified chapter.`, spec.Label, spec.FKMin, spec.FKMax, text)

	resp, err := rt.client.Message(anthropic.Request{
		Model: rt.writerModel, System: fmt.Sprintf("You simplify children's prose to %s level.", spec.Label),
		Prompt: prompt, MaxTokens: 8000, Temperature: 0.5,
	})
	if err == nil {
		p.SaveChapter(ch, resp.Text)
	}
}
