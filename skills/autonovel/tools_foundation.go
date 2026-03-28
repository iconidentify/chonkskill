package autonovel

import (
	"context"
	"fmt"
	"strings"

	"github.com/iconidentify/chonkskill/pkg/skill"
	"github.com/iconidentify/chonkskill/pkg/anthropic"
	"github.com/iconidentify/chonkskill/pkg/project"
)

func registerFoundationTools(s *skill.Skill, rt *runtime) {
	skill.AddTool(s, "generate_seed",
		"Generate novel seed concepts or riff on an existing idea. Seeds define the core concept, hook, world, magic system, tension, and theme.",
		func(ctx context.Context, args GenerateSeedArgs) (string, error) {
			p := project.New(args.ProjectDir)
			count := args.Count
			if count == 0 {
				count = 10
			}

			var prompt string
			if args.Riff != "" {
				prompt = fmt.Sprintf(`Generate 5 radically different variations on this novel idea:

"%s"

For each variation:
TITLE: (evocative, not generic)
HOOK: (one sentence that makes a reader pick up the book)
WORLD: (what makes this world impossible to forget)
MAGIC-COST: (what does power cost? not just "energy" -- real cost)
TENSION: (the central dramatic question)
THEME: (what this novel is really about, underneath the plot)
WHY-NOT-GENERIC: (what prevents this from being a standard genre entry)

Push each variation into a different emotional register. At least one should be uncomfortable.`, args.Riff)
			} else {
				prompt = fmt.Sprintf(`Generate %d novel seed concepts for fantasy fiction. Each must be genuinely original.

For each concept:
TITLE: (evocative, not generic)
HOOK: (one sentence that makes a reader pick up the book)
WORLD: (what makes this world impossible to forget)
MAGIC-COST: (what does power cost? not just "energy" -- real cost)
TENSION: (the central dramatic question)
THEME: (what this novel is really about, underneath the plot)
WHY-NOT-GENERIC: (what prevents this from being a standard genre entry)

DO NOT GENERATE:
- Chosen one narratives
- Dark lord antagonists
- Medieval European defaults
- Magic academy settings
- Love triangles as central conflict
- Prophecy-driven plots

At least 3 concepts must use non-Western cultural foundations. At least 2 should have protagonists over 40.`, count)
			}

			resp, err := rt.client.Message(anthropic.Request{
				Model:       rt.writerModel,
				System:      "You are a fantasy novelist with deep genre knowledge and a hatred of cliche. You generate concepts that a jaded reader would still pick up.",
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

	skill.AddTool(s, "gen_world",
		"Generate the world bible from seed.txt and voice.md. Creates world.md with cosmology, magic system, geography, factions, bestiary, cultural details, and consistency rules.",
		func(ctx context.Context, args GenWorldArgs) (string, error) {
			p := project.New(args.ProjectDir)
			if err := generateWorld(p, rt); err != nil {
				return "", err
			}
			world, _ := p.World()
			return fmt.Sprintf("world.md generated (%d words)", anthropic.CountWords(world)), nil
		})

	skill.AddTool(s, "gen_characters",
		"Generate the character registry from seed, world, and voice. Creates characters.md with wound/want/need/lie chains, three sliders, dialogue distinctiveness, and speech patterns for each character.",
		func(ctx context.Context, args GenCharactersArgs) (string, error) {
			p := project.New(args.ProjectDir)
			if err := generateCharacters(p, rt); err != nil {
				return "", err
			}
			chars, _ := p.Characters()
			return fmt.Sprintf("characters.md generated (%d words)", anthropic.CountWords(chars)), nil
		})

	skill.AddTool(s, "gen_outline",
		"Generate the full chapter outline from seed, world, characters, and voice. Creates outline.md with per-chapter beats, foreshadowing ledger, and try-fail cycles.",
		func(ctx context.Context, args GenOutlineArgs) (string, error) {
			p := project.New(args.ProjectDir)
			if err := generateOutline(p, rt); err != nil {
				return "", err
			}
			outline, _ := p.Outline()
			return fmt.Sprintf("outline.md generated (%d words)", anthropic.CountWords(outline)), nil
		})

	skill.AddTool(s, "gen_canon",
		"Extract all hard facts from world.md, characters.md, and seed.txt into canon.md. Low temperature for factual precision. Target 80-120 canon entries.",
		func(ctx context.Context, args GenCanonArgs) (string, error) {
			p := project.New(args.ProjectDir)
			if err := generateCanon(p, rt); err != nil {
				return "", err
			}
			canon, _ := p.Canon()
			return fmt.Sprintf("canon.md generated (%d words)", anthropic.CountWords(canon)), nil
		})
}

// generateWorld creates world.md.
func generateWorld(p *project.Project, rt *runtime) error {
	seed, err := p.Seed()
	if err != nil || seed == "" {
		return fmt.Errorf("seed.txt is required")
	}
	voice, _ := p.Voice()

	prompt := fmt.Sprintf(`Generate a complete world bible for this novel concept.

## Seed
%s

## Voice Guidelines (if available)
%s

Write the world bible with these sections:
1. **Cosmology & History** -- origin, timeline, era, what happened to shape this world
2. **Magic System** -- Hard Rules (limits, costs, consistency), Soft Magic (unexplained wonders), Societal Implications (who benefits, who suffers)
3. **Geography** -- major locations, climate, travel routes, distances
4. **Factions & Politics** -- power structures, alliances, tensions, economies
5. **Bestiary & Flora** -- creatures, plants, materials unique to this world
6. **Cultural Details** -- food, clothing, festivals, funeral rites, insults, terms of endearment
7. **Internal Consistency Rules** -- hard constraints the narrative must never violate

Target 3000-4000 words. Prioritize specificity over breadth. Every detail should create story potential.`, seed, voice)

	resp, err := rt.client.Message(anthropic.Request{
		Model:       rt.writerModel,
		System:      "You are a fantasy worldbuilder who combines Sanderson's systematic magic design, Le Guin's cultural depth, and TTRPG-quality lore. Every element you create is a loaded gun for the narrative.",
		Prompt:      prompt,
		MaxTokens:   16000,
		Temperature: 0.7,
	})
	if err != nil {
		return err
	}
	return p.SaveWorld(resp.Text)
}

// generateCharacters creates characters.md.
func generateCharacters(p *project.Project, rt *runtime) error {
	seed, _ := p.Seed()
	world, _ := p.World()
	voice, _ := p.Voice()

	if seed == "" {
		return fmt.Errorf("seed.txt is required")
	}

	prompt := fmt.Sprintf(`Generate a complete character registry for this novel.

## Seed
%s

## World Bible
%s

## Voice Guidelines
%s

For the protagonist and each major character, provide:

### Character Name
- **Role**: protagonist / antagonist / mentor / etc.
- **Wound**: the formative event that shaped them
- **Want**: what they consciously pursue
- **Need**: what they actually need (which they resist)
- **Lie**: the false belief they cling to
- **Three Sliders**: (1) competence 1-10, (2) likability 1-10, (3) proactivity 1-10
- **Arc**: how they change from chapter 1 to final chapter
- **Dialogue Distinctiveness** (8 dimensions):
  1. Vocabulary level (formal/informal/technical/slang)
  2. Sentence length tendency (terse/average/verbose)
  3. Question frequency (rarely/sometimes/often)
  4. Interruption style (lets others finish/cuts in/talks over)
  5. Emotional expression (guarded/measured/open/effusive)
  6. Humor type (dry/dark/warm/none)
  7. Power language (deferential/equal/commanding)
  8. Speech tics or catchphrases
- **Speech Pattern Examples**: 3 sample lines in their voice
- **Physical**: key distinguishing features only (not a police sketch)
- **Relationships**: their dynamic with each other major character

Create at least 5 characters. The antagonist must have a valid worldview, not just be evil.`, seed, truncateForContext(world, 6000), voice)

	resp, err := rt.client.Message(anthropic.Request{
		Model:       rt.writerModel,
		System:      "You are a character designer who creates people, not archetypes. Every character should feel like they existed before page one and will continue after the last page.",
		Prompt:      prompt,
		MaxTokens:   16000,
		Temperature: 0.7,
	})
	if err != nil {
		return err
	}
	return p.SaveCharacters(resp.Text)
}

// generateOutline creates outline.md.
func generateOutline(p *project.Project, rt *runtime) error {
	seed, _ := p.Seed()
	world, _ := p.World()
	chars, _ := p.Characters()
	mystery, _ := p.Mystery()
	voice, _ := p.Voice()

	if seed == "" {
		return fmt.Errorf("seed.txt is required")
	}

	prompt := fmt.Sprintf(`Generate a complete chapter outline for a 22-26 chapter novel.

## Seed
%s

## World Bible
%s

## Characters
%s

## Mystery/Subtext (if available)
%s

## Voice Guidelines
%s

For each chapter, provide:
### Ch N: [Title]
- **POV**: character name
- **Location**: where
- **Save the Cat Beat**: which beat this chapter fulfills
- **Percentage Mark**: ~N%% of the story
- **Emotional Arc**: character's emotional state start -> end
- **Try-Fail Cycle**: what the character attempts and how it goes wrong (or right)
- **Beats**: 3-5 specific scene beats
- **Plants**: foreshadowing elements planted in this chapter
- **Payoffs**: foreshadowing elements resolved from earlier chapters
- **Character Movement**: how the character changes by chapter end
- **The Lie**: status of the character's central lie
- **Target Word Count**: ~3000-4000

After all chapters, include:
## Foreshadowing Ledger
| Thread | Planted (Ch) | Reinforced (Ch) | Payoff (Ch) | Type |
|--------|-------------|-----------------|-------------|------|

Ensure every plant has a payoff. Ensure the Save the Cat beats fall at correct percentage marks.`, seed, truncateForContext(world, 6000), truncateForContext(chars, 6000), mystery, voice)

	resp, err := rt.client.Message(anthropic.Request{
		Model:       rt.writerModel,
		System:      "You are a plot architect who designs stories with the structural precision of a Swiss watch and the emotional resonance of a folk tale. Every chapter earns its place.",
		Prompt:      prompt,
		MaxTokens:   16000,
		Temperature: 0.5,
	})
	if err != nil {
		return err
	}

	// If the response seems to cut off (no foreshadowing ledger), generate continuation.
	if !strings.Contains(resp.Text, "Foreshadowing Ledger") {
		part2Prompt := fmt.Sprintf(`Continue the outline from where you left off. Complete all remaining chapters and add the Foreshadowing Ledger.

## Outline So Far
%s

## Mystery/Subtext
%s

Complete all remaining chapters following the same format. Then add the Foreshadowing Ledger table.`, resp.Text, mystery)

		resp2, err := rt.client.Message(anthropic.Request{
			Model:       rt.writerModel,
			System:      "You are continuing a chapter outline. Match the exact format of the previous entries.",
			Prompt:      part2Prompt,
			MaxTokens:   16000,
			Temperature: 0.5,
		})
		if err == nil {
			resp.Text += "\n\n" + resp2.Text
		}
	}

	return p.SaveOutline(resp.Text)
}

// generateCanon creates canon.md.
func generateCanon(p *project.Project, rt *runtime) error {
	seed, _ := p.Seed()
	world, _ := p.World()
	chars, _ := p.Characters()

	if seed == "" {
		return fmt.Errorf("seed.txt is required")
	}

	prompt := fmt.Sprintf(`Extract ALL hard facts from these documents into a structured canon database.

## Seed
%s

## World Bible
%s

## Characters
%s

Organize the canon into sections:
1. **Geography** -- place names, distances, directions, landmarks
2. **Timeline** -- dated or sequenced events
3. **Magic System Rules** -- hard constraints that must never be violated
4. **Character Facts** -- ages, relationships, physical traits, skills
5. **Political/Factional** -- alliances, hierarchies, territories
6. **Cultural** -- customs, food, clothing, language, rituals
7. **Established In-Story** -- facts established by narrative events

Format each entry as a single line:
- [Category] Fact statement. (Source: world/characters/seed)

Target 80-120 entries minimum. Include EVERY specific number, name, distance, rule, and relationship. If two facts could contradict, flag the tension.`, seed, world, chars)

	resp, err := rt.client.Message(anthropic.Request{
		Model:       rt.writerModel,
		System:      "You are a continuity editor. You extract facts with the precision of a legal deposition. Nothing escapes your notice. If it could be checked, it goes in the canon.",
		Prompt:      prompt,
		MaxTokens:   16000,
		Temperature: 0.2,
	})
	if err != nil {
		return err
	}
	return p.SaveCanon(resp.Text)
}

func truncateForContext(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "\n[...truncated...]"
}
