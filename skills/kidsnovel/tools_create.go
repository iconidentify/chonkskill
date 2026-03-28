package kidsnovel

import (
	"context"
	"fmt"

	"github.com/iconidentify/chonkskill/pkg/skill"
	"github.com/iconidentify/chonkskill/pkg/anthropic"
	"github.com/iconidentify/chonkskill/pkg/project"
	"github.com/iconidentify/chonkskill/skills/kidsnovel/internal/readability"
)

func registerCreateTools(s *skill.Skill, rt *runtime) {
	skill.AddTool(s, "from_kid_writing",
		"Build a book from a kid's own writing. Takes any piece of writing -- a story start, a journal entry, a paragraph, even a few sentences -- and develops it into a full book concept that honors the kid's original ideas and voice.",
		func(ctx context.Context, args FromKidWritingArgs) (string, error) {
			p := project.New(args.ProjectDir)
			grade := loadGrade(p)
			spec := readability.GradeSpecs[grade]

			credit := ""
			if args.KidName != "" {
				credit = fmt.Sprintf("\n\nThe co-author is %s. Honor their name in the seed.", args.KidName)
			}

			prompt := fmt.Sprintf(`A young writer has shared their own writing. Your job is to find what's magical about it and develop it into a complete book concept. Do NOT rewrite their idea into something generic. The spark is THEIRS -- your job is to fan it into a flame.

## The Kid's Writing
%s
%s
## Your Task

Find the strongest elements in this writing:
- What's the most interesting idea?
- What character feels most alive?
- What moment has the most energy?
- What world detail is most specific and original?

Then develop a book concept that amplifies those elements. The kid should read the concept and think "yes, that's what I meant, but bigger!"

Write a seed concept with:
TITLE: (something the kid would be proud to see on a book cover)
HOOK: (one sentence a kid would read to their friend)
MAIN CHARACTER: (built from the kid's character, or inspired by their voice)
WORLD: (expanded from any world details in their writing)
PROBLEM: (the central challenge -- what makes the story GO)
THEME: (what the story is really about underneath)
WHAT MAKES IT SPECIAL: (the thing from the kid's writing that no AI would think of)

This book is for %s readers (~%d words, ~%d chapters).
Vocabulary and concepts must be appropriate for ages %d-%d.`,
				args.Writing, credit,
				spec.Label, spec.BookWordMin, spec.ChapterCount,
				spec.Grade+5, spec.Grade+8)

			resp, err := rt.client.Message(anthropic.Request{
				Model:       rt.writerModel,
				System:      "You are a children's book editor who finds the extraordinary in kids' writing. You never condescend. You take their ideas seriously and make them bigger, not different.",
				Prompt:      prompt,
				MaxTokens:   4000,
				Temperature: 0.9,
			})
			if err != nil {
				return "", err
			}

			if err := p.SaveSeed(resp.Text); err != nil {
				return "", err
			}

			if args.KidName != "" {
				p.SaveFile("co_author.txt", args.KidName)
			}

			return resp.Text, nil
		})

	skill.AddTool(s, "from_idea",
		"Build a book from a story idea. The idea can be anything -- a 'what if', a character description, a setting, a problem, a single sentence. Works great for brainstorming with kids or for content creators.",
		func(ctx context.Context, args FromIdeaArgs) (string, error) {
			p := project.New(args.ProjectDir)
			grade := loadGrade(p)
			spec := readability.GradeSpecs[grade]

			genreHint := ""
			if args.Genre != "" {
				genreHint = fmt.Sprintf("\nPreferred genre: %s", args.Genre)
			}

			prompt := fmt.Sprintf(`Develop this story idea into a complete children's book concept.

## The Idea
%s
%s
## Requirements

Write a seed concept with:
TITLE: (fun, memorable, something a kid would want to tell their friend about)
HOOK: (one sentence that makes a kid say "I want to read that!")
MAIN CHARACTER: (a protagonist that kids in %s will relate to -- give them a real personality, not just a role)
BEST FRIEND/SIDEKICK: (every great kids book has one)
ANTAGONIST: (not necessarily a villain -- could be a situation, a fear, a misunderstanding)
WORLD: (the setting, made vivid and specific)
THE BIG PROBLEM: (what drives the story forward)
THE TWIST: (something unexpected that makes this story different from every other kids book)
THEME: (the emotional truth the story explores -- friendship, courage, being yourself, etc.)
WHY A KID WOULD LOVE THIS: (be specific)

Book specs: %s, ~%d words, ~%d chapters.
Appropriate for ages %d-%d.`,
				args.Idea, genreHint,
				spec.Label,
				spec.Label, spec.BookWordMin, spec.ChapterCount,
				spec.Grade+5, spec.Grade+8)

			resp, err := rt.client.Message(anthropic.Request{
				Model:       rt.writerModel,
				System:      "You are a children's book author who writes stories kids actually want to read. Not preachy, not boring, not predictable. You write books that kids hide under the covers with a flashlight.",
				Prompt:      prompt,
				MaxTokens:   4000,
				Temperature: 0.9,
			})
			if err != nil {
				return "", err
			}

			if err := p.SaveSeed(resp.Text); err != nil {
				return "", err
			}
			return resp.Text, nil
		})

	skill.AddTool(s, "generate_seed",
		"Generate multiple book concepts for a kid to choose from. Optionally themed (friendship, adventure, mystery, etc.). Great for when a kid says 'I want to write a book but I don't know what about.'",
		func(ctx context.Context, args GenerateSeedArgs) (string, error) {
			p := project.New(args.ProjectDir)
			grade := loadGrade(p)
			spec := readability.GradeSpecs[grade]

			count := args.Count
			if count == 0 {
				count = 5
			}

			themeHint := ""
			if args.Theme != "" {
				themeHint = fmt.Sprintf("\nAll concepts should explore the theme of: %s", args.Theme)
			}

			prompt := fmt.Sprintf(`Generate %d children's book concepts. Each must be genuinely different and genuinely exciting for a %s reader (ages %d-%d).
%s
For each concept:
TITLE: (something a kid would remember)
HOOK: (one sentence -- would a kid tell their friend about this?)
CHARACTER: (who is the main character and why would a kid root for them?)
THE PROBLEM: (what goes wrong?)
WHY IT'S DIFFERENT: (what makes this story unlike any other?)
THEME: (what's it really about?)

RULES:
- No chosen one stories
- No "learning the power of friendship" as the plot (it can be a theme, not a plot)
- No talking animals unless the concept is genuinely original
- At least one concept with a funny premise
- At least one concept with a mystery
- At least one concept set in a world kids can relate to (school, neighborhood, family)
- At least one with a protagonist who is NOT the brave hero type
- Characters should have real flaws and real strengths

Book specs: ~%d words, ~%d chapters.`, count, spec.Label, spec.Grade+5, spec.Grade+8,
				themeHint, spec.BookWordMin, spec.ChapterCount)

			resp, err := rt.client.Message(anthropic.Request{
				Model:       rt.writerModel,
				System:      "You generate children's book concepts that make kids say 'THAT ONE.' You know that the best kids books are the ones adults want to read too.",
				Prompt:      prompt,
				MaxTokens:   4000,
				Temperature: 1.0,
			})
			if err != nil {
				return "", err
			}

			if err := p.SaveSeed(resp.Text); err != nil {
				return "", err
			}
			return resp.Text, nil
		})
}
