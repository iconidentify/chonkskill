package kidsnovel

import (
	"context"
	"fmt"

	"github.com/iconidentify/chonkskill/pkg/skill"
	"github.com/iconidentify/chonkskill/pkg/anthropic"
	"github.com/iconidentify/chonkskill/pkg/project"
	"github.com/iconidentify/chonkskill/skills/kidsnovel/internal/readability"
)

func registerFoundationTools(s *skill.Skill, rt *runtime) {
	skill.AddTool(s, "gen_world",
		"Generate the world/setting for the book. For realistic fiction this describes the neighborhood, school, or town. For fantasy/sci-fi this builds the world rules. Calibrated to the target grade level.",
		func(ctx context.Context, args GenWorldArgs) (string, error) {
			p := project.New(args.ProjectDir)
			grade := loadGrade(p)
			spec := readability.GradeSpecs[grade]

			seed, _ := p.Seed()
			if seed == "" {
				return "", fmt.Errorf("seed.txt is required -- use from_kid_writing, from_idea, or generate_seed first")
			}

			prompt := fmt.Sprintf(`Create the world/setting for this children's book.

## Book Concept
%s

## Requirements
Write a setting description that includes:
1. **The Main Location** -- where most of the story happens. Make it specific and vivid. A kid should be able to draw it.
2. **Key Places** -- 3-5 specific locations where important scenes happen. Name them.
3. **The Rules** -- if there's magic or technology, what are the rules? Keep them simple and consistent. A %s reader should understand them without explanation.
4. **What Makes It Feel Real** -- sensory details. What does it smell like? Sound like? What's the weather?
5. **The Secret** -- every great setting has one thing that's not what it seems.
6. **Daily Life** -- what's normal here? The extraordinary only works if we understand the ordinary first.

Keep descriptions concrete. "A red barn with a rusted weather vane" not "a picturesque rural structure."
This is for %s readers. Concepts must be graspable by ages %d-%d.`,
				truncate(seed, 3000),
				spec.Label, spec.Label, spec.Grade+5, spec.Grade+8)

			resp, err := rt.client.Message(anthropic.Request{
				Model:       rt.writerModel,
				System:      "You build story worlds for children's books. Your worlds are vivid enough to draw, simple enough to remember, and deep enough to surprise.",
				Prompt:      prompt,
				MaxTokens:   8000,
				Temperature: 0.7,
			})
			if err != nil {
				return "", err
			}

			if err := p.SaveWorld(resp.Text); err != nil {
				return "", err
			}
			return fmt.Sprintf("world.md generated (%d words)", anthropic.CountWords(resp.Text)), nil
		})

	skill.AddTool(s, "gen_characters",
		"Generate the character registry. Creates relatable, grade-appropriate characters with distinct voices, real flaws, and clear wants.",
		func(ctx context.Context, args GenCharactersArgs) (string, error) {
			p := project.New(args.ProjectDir)
			grade := loadGrade(p)
			spec := readability.GradeSpecs[grade]

			seed, _ := p.Seed()
			world, _ := p.World()

			if seed == "" {
				return "", fmt.Errorf("seed.txt is required")
			}

			prompt := fmt.Sprintf(`Create the characters for this children's book.

## Book Concept
%s

## World/Setting
%s

## Requirements

Create the main character and 3-5 supporting characters. For each:

### [Character Name]
- **Who they are**: age, one-line description a kid could picture
- **What they want**: the thing they're trying to get or do
- **What they're afraid of**: their real fear (not monsters -- the emotional fear)
- **Their flaw**: the thing that gets them in trouble
- **Their strength**: the thing that will save the day (but they don't know it yet)
- **How they talk**: 3 sample lines in their voice. Each character MUST sound different.
  - Do they use slang? Big words? Short sentences? Questions?
  - Do they joke? Worry? Boss people around? Stay quiet?
- **Their quirk**: one specific habit or detail that makes them memorable

RULES:
- The main character must be someone a %s reader would want as a friend (or at least find interesting)
- At least one character should make the reader laugh
- The antagonist (if there is one) must have an understandable reason for what they do
- No perfect characters. No purely evil characters.
- Characters should be the age of the target reader (%d-%d) or close to it
- Dialogue samples must use vocabulary appropriate for %s`,
				truncate(seed, 3000),
				truncate(world, 3000),
				spec.Label, spec.Grade+5, spec.Grade+8,
				spec.Label)

			resp, err := rt.client.Message(anthropic.Request{
				Model:       rt.writerModel,
				System:      "You create characters for children's books. Your characters feel like real kids -- messy, funny, scared, brave, complicated. Not role models. Real people.",
				Prompt:      prompt,
				MaxTokens:   8000,
				Temperature: 0.7,
			})
			if err != nil {
				return "", err
			}

			if err := p.SaveCharacters(resp.Text); err != nil {
				return "", err
			}
			return fmt.Sprintf("characters.md generated (%d words)", anthropic.CountWords(resp.Text)), nil
		})

	skill.AddTool(s, "gen_outline",
		"Generate the chapter-by-chapter outline. Paced for the target grade level with clear chapter arcs, cliffhangers, and escalating stakes.",
		func(ctx context.Context, args GenOutlineArgs) (string, error) {
			p := project.New(args.ProjectDir)
			grade := loadGrade(p)
			spec := readability.GradeSpecs[grade]

			seed, _ := p.Seed()
			world, _ := p.World()
			chars, _ := p.Characters()

			if seed == "" {
				return "", fmt.Errorf("seed.txt is required")
			}

			prompt := fmt.Sprintf(`Create a chapter-by-chapter outline for this children's book.

## Book Concept
%s

## World/Setting
%s

## Characters
%s

## Requirements

Write a %d-chapter outline. For each chapter:

### Ch N: [Fun Chapter Title]
- **What happens**: 2-3 sentences describing the main events
- **The problem**: what goes wrong or what tension exists in this chapter
- **The fun part**: what moment will make a kid smile, gasp, or laugh
- **Ends with**: how the chapter ends (cliffhanger, question, surprise, or emotional moment)
- **Word target**: ~%d words

PACING RULES:
- Chapter 1: Hook the reader IMMEDIATELY. Something interesting happens on page 1.
- Chapters 2-3: Set up the normal world, introduce the problem
- Middle chapters: Escalating attempts to solve the problem, each making things worse
- Chapter before last: The lowest point. Everything seems impossible.
- Last chapter: Resolution that feels EARNED, not easy.

KIDS BOOK RULES:
- Every chapter must have at least one moment of humor, action, or emotion
- No chapter should be "the one where they just talk about their feelings"
- Subplots must connect to the main story (no filler chapters)
- The middle must not sag -- keep stakes rising
- Chapter titles should make a kid want to read that chapter

This is for %s readers (ages %d-%d). Book total: %d-%d words.`,
				truncate(seed, 3000),
				truncate(world, 3000),
				truncate(chars, 3000),
				spec.ChapterCount,
				spec.ChapterWordTarget,
				spec.Label, spec.Grade+5, spec.Grade+8,
				spec.BookWordMin, spec.BookWordMax)

			resp, err := rt.client.Message(anthropic.Request{
				Model:       rt.writerModel,
				System:      "You outline children's chapter books. You know that every chapter must earn its place. Bored kids stop reading. Your outlines never let that happen.",
				Prompt:      prompt,
				MaxTokens:   12000,
				Temperature: 0.5,
			})
			if err != nil {
				return "", err
			}

			if err := p.SaveOutline(resp.Text); err != nil {
				return "", err
			}
			return fmt.Sprintf("outline.md generated (%d words, ~%d chapters)", anthropic.CountWords(resp.Text), spec.ChapterCount), nil
		})
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n[...truncated...]"
}
