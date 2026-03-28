package kidsnovel

import (
	"context"
	"fmt"

	"github.com/iconidentify/chonkskill/pkg/skill"
	"github.com/iconidentify/chonkskill/pkg/anthropic"
	"github.com/iconidentify/chonkskill/pkg/project"
	"github.com/iconidentify/chonkskill/skills/kidsnovel/internal/readability"
)

func registerDraftTools(s *skill.Skill, rt *runtime) {
	skill.AddTool(s, "draft_chapter",
		"Draft a single chapter at the target reading level. Enforces grade-appropriate vocabulary, sentence length, and pacing.",
		func(ctx context.Context, args DraftChapterArgs) (string, error) {
			if args.Chapter == 0 {
				return "", fmt.Errorf("chapter number is required")
			}
			p := project.New(args.ProjectDir)
			if err := draftChapter(p, rt, args.Chapter); err != nil {
				return "", err
			}
			text, _ := p.LoadChapter(args.Chapter)
			grade := loadGrade(p)

			// Run readability check.
			analysis := readability.Analyze(text, grade)
			return fmt.Sprintf("Chapter %d drafted (%d words, FK %.1f, grade fit: %s)",
				args.Chapter, anthropic.CountWords(text), analysis.FleschKincaid, analysis.GradeFit), nil
		})

	skill.AddTool(s, "draft_all",
		"Draft all chapters sequentially. Each chapter is checked for reading level after drafting.",
		func(ctx context.Context, args DraftAllArgs) (string, error) {
			p := project.New(args.ProjectDir)
			grade := loadGrade(p)
			spec := readability.GradeSpecs[grade]
			state, _ := p.LoadState()

			totalCh, _ := p.GetTotalChapters(state)
			if totalCh == 0 {
				totalCh = spec.ChapterCount
			}

			var results []string
			for ch := 1; ch <= totalCh; ch++ {
				if err := draftChapter(p, rt, ch); err != nil {
					results = append(results, fmt.Sprintf("Ch %d: ERROR: %v", ch, err))
					continue
				}
				text, _ := p.LoadChapter(ch)
				analysis := readability.Analyze(text, grade)
				results = append(results, fmt.Sprintf("Ch %d: %d words, FK %.1f (%s)",
					ch, anthropic.CountWords(text), analysis.FleschKincaid, analysis.GradeFit))

				state.ChaptersDrafted = ch
				p.SaveState(state)
			}

			totalWords, _ := p.CountAllWords()
			results = append(results, fmt.Sprintf("\nTotal: %d words across %d chapters", totalWords, totalCh))
			return joinLines(results), nil
		})
}

func draftChapter(p *project.Project, rt *runtime, chapterNum int) error {
	grade := loadGrade(p)
	spec := readability.GradeSpecs[grade]
	constraints := readability.GradeConstraints(grade)

	seed, _ := p.Seed()
	world, _ := p.World()
	chars, _ := p.Characters()
	outline, _ := p.Outline()

	if outline == "" {
		return fmt.Errorf("outline.md is required -- use kidsnovel:gen_outline first")
	}

	chOutline := p.ExtractChapterOutline(outline, chapterNum)
	if chOutline == "" {
		return fmt.Errorf("no outline entry for chapter %d", chapterNum)
	}

	prevTail := ""
	if chapterNum > 1 {
		if prev, _ := p.LoadChapter(chapterNum - 1); prev != "" {
			prevTail = project.LastNChars(prev, 1500)
		}
	}

	coAuthor := ""
	if name, _ := p.LoadFile("co_author.txt"); name != "" {
		coAuthor = fmt.Sprintf("\nThis book is co-created with %s. Honor the spirit of their original idea.", name)
	}

	prompt := fmt.Sprintf(`Write Chapter %d of this children's book.

## %s
%s

## World/Setting
%s

## Characters
%s

## This Chapter's Outline
%s

## Previous Chapter Ending
%s
%s
## WRITING RULES

%s

## CRAFT RULES FOR KIDS BOOKS

1. START with action, dialogue, or a surprising detail. Never "It was a beautiful day."
2. Show, don't tell -- but simpler than adult fiction. "His stomach did a flip" not "He experienced anxiety."
3. Dialogue should sound like real kids talking. Incomplete sentences are fine. Kids interrupt.
4. Use the five senses. Kids notice smells, textures, sounds that adults ignore.
5. One paragraph = one idea. Short paragraphs. White space is a kids book's friend.
6. Every page should have a reason to turn to the next page.
7. Humor matters. Even in serious moments, kids appreciate a light touch.
8. Action verbs over state verbs. "She sprinted" not "She was running."
9. Specific details over general ones. "A grape popsicle" not "a frozen treat."
10. End the chapter with a hook -- a question, a cliffhanger, a surprise, a feeling.
11. No preaching. The theme comes through the story, never through a character speech.
12. Characters make mistakes. Perfect characters are boring.
13. Vary the pace. Fast scenes (action, chase, argument) and slow scenes (discovery, friendship, wonder).
14. Chapter title should be fun and hint at what happens.

Target: ~%d words.`,
		chapterNum,
		spec.Label,
		truncate(seed, 2000),
		truncate(world, 2000),
		truncate(chars, 2000),
		chOutline,
		prevTail,
		coAuthor,
		constraints,
		spec.ChapterWordTarget)

	resp, err := rt.client.Message(anthropic.Request{
		Model:       rt.writerModel,
		System:      fmt.Sprintf("You write children's chapter books at the %s reading level. Your writing is vivid, fun, and impossible to put down. You never write down to kids. You write UP to them.", spec.Label),
		Prompt:      prompt,
		MaxTokens:   8000,
		Temperature: 0.8,
	})
	if err != nil {
		return fmt.Errorf("drafting failed: %w", err)
	}

	return p.SaveChapter(chapterNum, resp.Text)
}

func joinLines(lines []string) string {
	result := ""
	for i, l := range lines {
		if i > 0 {
			result += "\n"
		}
		result += l
	}
	return result
}
