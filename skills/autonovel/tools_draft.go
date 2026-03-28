package autonovel

import (
	"context"
	"fmt"

	"github.com/iconidentify/chonkskill/pkg/skill"
	"github.com/iconidentify/chonkskill/pkg/anthropic"
	"github.com/iconidentify/chonkskill/pkg/project"
)

func registerDraftTools(s *skill.Skill, rt *runtime) {
	skill.AddTool(s, "draft_chapter",
		"Draft a single chapter. Loads voice, world, characters, outline, canon, and adjacent chapter context. Writes to chapters/ch_NN.md.",
		func(ctx context.Context, args DraftChapterArgs) (string, error) {
			if args.Chapter == 0 {
				return "", fmt.Errorf("chapter number is required")
			}
			p := project.New(args.ProjectDir)
			if err := draftSingleChapter(p, rt, args.Chapter); err != nil {
				return "", err
			}
			text, _ := p.LoadChapter(args.Chapter)
			return fmt.Sprintf("Chapter %d drafted (%d words)", args.Chapter, anthropic.CountWords(text)), nil
		})
}

// draftSingleChapter generates a chapter from all context.
func draftSingleChapter(p *project.Project, rt *runtime, chapterNum int) error {
	seed, _ := p.Seed()
	voice, _ := p.Voice()
	world, _ := p.World()
	chars, _ := p.Characters()
	outline, _ := p.Outline()
	canon, _ := p.Canon()

	if outline == "" {
		return fmt.Errorf("outline.md is required -- use autonovel:gen_outline first")
	}

	// Extract chapter-specific outline entry.
	chOutline := p.ExtractChapterOutline(outline, chapterNum)
	if chOutline == "" {
		return fmt.Errorf("no outline entry found for chapter %d", chapterNum)
	}

	// Extract next chapter outline (first 10 lines) for foreshadowing.
	nextOutline := p.ExtractChapterOutline(outline, chapterNum+1)
	if len(nextOutline) > 500 {
		nextOutline = nextOutline[:500]
	}

	// Load adjacent chapter tails/heads for continuity.
	prevTail := ""
	if chapterNum > 1 {
		if prev, _ := p.LoadChapter(chapterNum - 1); prev != "" {
			prevTail = project.LastNChars(prev, 2000)
		}
	}

	prompt := fmt.Sprintf(`Write Chapter %d of this novel.

## Voice Bible
%s

## World Bible (excerpt)
%s

## Character Registry (excerpt)
%s

## This Chapter's Outline
%s

## Next Chapter Preview (for foreshadowing)
%s

## Canon Database (excerpt)
%s

## Previous Chapter (last 2000 chars for continuity)
%s

## Seed Concept
%s

## INSTRUCTIONS

Write the full chapter following the outline beats above. Target approximately 3200 words.

RULES:
1. Third-person limited past tense. Stay in the POV character's head.
2. No head-hopping. We only know what the POV character perceives.
3. Show, don't tell. Never name an emotion directly. Show it through action, dialogue, sensation.
4. No AI slop words: delve, utilize, leverage, facilitate, tapestry, symphony, testament, landscape.
5. No triadic listings (not "X, Y, and Z" as a rhythm crutch).
6. No "Not just X, but Y" constructions.
7. Vary sentence length deliberately. Follow a long sentence with a short one.
8. Dialogue must be distinctive per character. Each voice should be identifiable without tags.
9. Em dashes are acceptable but limit to 3-4 per chapter.
10. No sycophantic paragraph openers (However, Furthermore, Moreover).
11. Every scene must have a micro-conflict (someone wants something and can't easily get it).
12. End the chapter on a hook -- not a cliffhanger cliche, but a question the reader needs answered.
13. Plant any foreshadowing listed in the outline. Subtly.
14. Pay off any threads listed as resolving in this chapter.
15. Maintain continuity with the previous chapter's ending.
16. Vary paragraph length. Some paragraphs should be a single sentence.
17. Sensory details from at least 3 senses per scene (not just visual).
18. No "He thought about..." or "She realized that..." -- show the thought directly.
19. No "He did not X" when you can write "He X-ed" with a negative action.
20. If a character lies, show the body language that betrays them.
21. Subtext > text. Characters should rarely say exactly what they mean.
22. No convenient interruptions to avoid difficult conversations.
23. Physical space matters. Ground every scene in a specific location.
24. Time passes. Characters eat, sleep, travel. Don't teleport between plot points.`, chapterNum,
		truncateForContext(voice, 3000),
		truncateForContext(world, 4000),
		truncateForContext(chars, 4000),
		chOutline,
		nextOutline,
		truncateForContext(canon, 3000),
		prevTail,
		truncateForContext(seed, 1000))

	resp, err := rt.client.Message(anthropic.Request{
		Model:       rt.writerModel,
		System:      "You are a literary fiction writer. You write prose that is alive with sensory detail, driven by character, and shaped by craft. You never explain what you can show. You never tell the reader what to feel.",
		Prompt:      prompt,
		MaxTokens:   16000,
		Temperature: 0.8,
	})
	if err != nil {
		return fmt.Errorf("drafting failed: %w", err)
	}

	return p.SaveChapter(chapterNum, resp.Text)
}
