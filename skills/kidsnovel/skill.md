---
name: kidsnovel
description: >
  Create children's chapter books at grade 3-6 reading levels. Collaborative
  with kids, reading-level enforced, illustrated. Build books from a kid's
  writing, an idea, or from scratch.
metadata:
  requires_tools:
    - kidsnovel:init_project
    - kidsnovel:get_state
    - kidsnovel:from_kid_writing
    - kidsnovel:from_idea
    - kidsnovel:generate_seed
    - kidsnovel:gen_world
    - kidsnovel:gen_characters
    - kidsnovel:gen_outline
    - kidsnovel:draft_chapter
    - kidsnovel:draft_all
    - kidsnovel:readability_check
    - kidsnovel:evaluate_chapter
    - kidsnovel:evaluate_book
    - kidsnovel:simplify_chapter
    - kidsnovel:revise_chapter
    - kidsnovel:build_book
    - kidsnovel:gen_illustration
    - kidsnovel:run_pipeline
---

# Kids Novel: Children's Chapter Book Creator

## Overview

Create high-quality children's chapter books at grade 3-6 reading levels. Three
ways to start:

1. **From a kid's writing** -- a child's story, journal entry, or idea becomes a full book
2. **From an idea** -- anyone describes a concept and gets a complete book
3. **From scratch** -- generate seed concepts for a kid to choose from

Every chapter is mechanically checked for reading level (Flesch-Kincaid, vocabulary
complexity, sentence length) and evaluated for quality by an LLM judge.

## Reading Level Specs

| Grade | FK Range | Avg Sentence | Max Sentence | Words/Chapter | Book Length | Chapters |
|-------|----------|-------------|-------------|---------------|-------------|----------|
| 3 | 2.0-3.9 | ~9 words | 18 words | ~800 | 5K-12K | ~10 |
| 4 | 3.0-4.9 | ~11 words | 22 words | ~1,200 | 8K-18K | ~12 |
| 5 | 4.0-5.9 | ~13 words | 28 words | ~1,800 | 15K-30K | ~15 |
| 6 | 5.0-6.9 | ~15 words | 32 words | ~2,200 | 20K-45K | ~18 |

## Collaborative Workflow (with a kid)

This is the recommended flow when working with a child:

```
1. Kid shares their idea or writing
   -> kidsnovel:from_kid_writing or kidsnovel:from_idea

2. Read the seed back to the kid. Ask: "Is this your story? What would you change?"
   -> Edit seed.txt based on feedback

3. Generate the world, characters, and outline
   -> kidsnovel:gen_world, gen_characters, gen_outline
   -> Show the kid the characters. "Do you like them? Want to add anyone?"

4. Draft chapters one by one
   -> kidsnovel:draft_chapter
   -> Read each chapter to/with the kid
   -> kidsnovel:revise_chapter with their feedback

5. Check reading level
   -> kidsnovel:readability_check
   -> kidsnovel:simplify_chapter if too hard

6. Build the final book
   -> kidsnovel:build_book

7. Generate illustrations
   -> kidsnovel:gen_illustration (cover + per-chapter)
```

The key: the kid's input drives every decision. The system amplifies their creativity.

## Content Creator Workflow

For creators producing grade-leveled content at scale:

```
1. kidsnovel:init_project {grade: 4}
2. kidsnovel:from_idea {idea: "...", genre: "mystery"}
3. kidsnovel:run_pipeline  (runs everything automatically)
4. kidsnovel:evaluate_book  (check quality)
5. kidsnovel:gen_illustration  (cover + chapters)
```

## Reading Level Enforcement

Every chapter goes through two checks:

### Mechanical Check (readability_check, no LLM)
- Flesch-Kincaid Grade Level
- Coleman-Liau Index
- Automated Readability Index
- Syllable count per word
- Sentence length distribution
- Complex word percentage
- Dialogue percentage
- Specific flagged sentences and vocabulary

### LLM Check (evaluate_chapter)
- Hook power (would a kid keep reading?)
- Engagement (any boring stretches?)
- Character voice (natural kid dialogue?)
- Age appropriateness
- Fun factor
- Emotional resonance
- Show don't tell quality

### Simplification

When readability_check shows "too-hard", use simplify_chapter. It:
- Breaks long sentences into shorter ones
- Replaces complex vocabulary with simpler words
- Adds more dialogue (turning narration into conversation)
- Cuts unnecessary adjectives and adverbs
- Preserves the exact same story, characters, and emotions

## Craft Principles for Kids Books

### What Makes Kids Stop Reading
- Boring first page (always start with action or dialogue)
- Long paragraphs of description
- Characters who are perfect or preachy
- Predictable plots
- Writing that talks down to them
- Chapters where "nothing happens"
- Vocabulary they have to look up every sentence

### What Makes Kids Keep Reading
- Characters who feel like real kids
- Humor (even in serious stories)
- Secrets and mysteries
- Clear stakes (what happens if the hero fails?)
- Short chapters that end with hooks
- Dialogue that sounds like real people talking
- Vivid sensory details (smells, textures, sounds)
- The feeling that the author respects them

### Chapter Structure
- Start: action, dialogue, or surprise (never weather or waking up)
- Middle: escalating problem with try-fail cycle
- End: hook (question, cliffhanger, revelation, emotional beat)

### Dialogue
- Most natural way to convey information at this level
- Kids love reading dialogue
- Each character must sound different
- Tags: mostly "said" and "asked"
- Interruptions, incomplete sentences, and reactions make it real

## Illustration

Requires FAL_KEY. Supports styles:
- **cartoon**: Bright, bold, expressive
- **watercolor**: Soft, dreamy, atmospheric
- **pencil**: Detailed, classic, textured
- **comic**: Dynamic, action-oriented, panel-style
- **whimsical**: Playful, slightly surreal, imaginative

For chapter illustrations, the system automatically picks the most visual moment.
For covers, it generates from the book concept.

## Environment Variables

```
ANTHROPIC_API_KEY          Required.
KIDSNOVEL_WRITER_MODEL     Default: claude-sonnet-4-6
KIDSNOVEL_JUDGE_MODEL      Default: claude-opus-4-6
FAL_KEY                    Optional (illustrations)
```
