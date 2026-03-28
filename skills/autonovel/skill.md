---
name: autonovel
description: >
  Autonomous novel writing pipeline. Generate, draft, evaluate, revise, export,
  illustrate, and narrate complete novels using a modify-evaluate-keep/discard loop.
metadata:
  requires_tools:
    - autonovel:init_project
    - autonovel:get_state
    - autonovel:run_pipeline
    - autonovel:generate_seed
    - autonovel:gen_world
    - autonovel:gen_characters
    - autonovel:gen_outline
    - autonovel:gen_canon
    - autonovel:draft_chapter
    - autonovel:evaluate_foundation
    - autonovel:evaluate_chapter
    - autonovel:evaluate_full
    - autonovel:slop_check
    - autonovel:voice_fingerprint
    - autonovel:adversarial_edit
    - autonovel:apply_cuts
    - autonovel:reader_panel
    - autonovel:gen_brief
    - autonovel:gen_revision
    - autonovel:review_manuscript
    - autonovel:compare_chapters
    - autonovel:build_arc_summary
    - autonovel:build_outline
    - autonovel:gen_art_style
    - autonovel:gen_art
    - autonovel:gen_audiobook_script
    - autonovel:gen_audiobook
---

# Autonovel: Autonomous Novel Writing Pipeline

## Overview

Autonovel writes complete novels autonomously through a four-phase pipeline:
foundation, drafting, revision, and export. It uses a modify-evaluate-keep/discard
loop: every change is evaluated, kept if it improves quality, discarded if not.

The system uses separate writer and judge models intentionally -- using a different
model as evaluator prevents self-congratulation bias.

## Architecture: The Five-Layer Stack

1. **Seed** -- the core concept (seed.txt)
2. **Voice** -- prose style rules and vocabulary wells (voice.md)
3. **Foundation** -- world, characters, outline, canon (world.md, characters.md, outline.md, canon.md)
4. **Chapters** -- the actual prose (chapters/ch_NN.md)
5. **Polish** -- revision, art, audiobook

Each layer builds on the ones below it. Never modify a lower layer without re-evaluating all layers above it.

## Two Immune Systems

The pipeline has two immune systems that detect AI-generated prose patterns:

### Immune System 1: Mechanical (No LLM)
- `autonovel:slop_check` runs regex detection for banned words, suspicious clusters, filler phrases, fiction AI tells, structural tics, and telling patterns
- `autonovel:voice_fingerprint` measures sentence rhythm, vocabulary well density, dialogue ratio, and flags statistical outliers
- These run instantly with zero API cost

### Immune System 2: Adversarial (LLM Judge)
- `autonovel:adversarial_edit` asks a ruthless editor model to identify 10-20 passages to cut or rewrite
- `autonovel:reader_panel` simulates 4 distinct reader personas who disagree with each other
- `autonovel:evaluate_chapter` and `autonovel:evaluate_full` use the judge model for multi-dimensional scoring
- `autonovel:review_manuscript` sends the full novel to Opus for dual-persona deep review

## Quick Start

### Manual Mode (step by step)
```
1. autonovel:init_project {project_dir: "/path/to/novel"}
2. autonovel:generate_seed {project_dir: "...", count: 10}
   -- User picks a seed or edits seed.txt
3. autonovel:gen_world {project_dir: "..."}
4. autonovel:gen_characters {project_dir: "..."}
5. autonovel:gen_outline {project_dir: "..."}
6. autonovel:gen_canon {project_dir: "..."}
7. autonovel:evaluate_foundation {project_dir: "..."}
   -- Iterate steps 3-7 until score >= 7.5
8. autonovel:draft_chapter {project_dir: "...", chapter: 1}
9. autonovel:evaluate_chapter {project_dir: "...", chapter: 1}
   -- Iterate 8-9 per chapter until score >= 6.0
10. autonovel:adversarial_edit {project_dir: "...", chapter: 0}
11. autonovel:apply_cuts {project_dir: "...", chapter: 0}
12. autonovel:build_arc_summary {project_dir: "..."}
13. autonovel:reader_panel {project_dir: "..."}
14. autonovel:gen_brief {project_dir: "...", chapter: N, source: "auto"}
15. autonovel:gen_revision {project_dir: "...", chapter: N, brief_path: "briefs/chNN_eval.md"}
16. autonovel:evaluate_full {project_dir: "..."}
    -- Iterate revision cycle until plateau
17. autonovel:review_manuscript {project_dir: "..."}
18. autonovel:build_outline {project_dir: "..."}
19. autonovel:gen_art_style {project_dir: "..."}
20. autonovel:gen_art {project_dir: "...", art_type: "cover"}
```

### Autonomous Mode
```
autonovel:run_pipeline {project_dir: "/path/to/novel"}
```
Runs all four phases automatically with the modify-evaluate-keep/discard loop.

## Phase Details

### Phase 1: Foundation (score target: 7.5)
Generate world bible, character registry, chapter outline, and canon database.
Evaluate with the judge. Regenerate if score is below threshold. Up to 20 iterations.

Key evaluator dimensions:
- World depth (does the magic system have real costs?)
- Character depth (does each character have a wound/want/need/lie chain?)
- Outline completeness (does every chapter have beats and try-fail cycles?)
- Foreshadowing balance (does every plant have a payoff?)
- Internal consistency (do facts contradict?)

### Phase 2: Drafting (chapter score target: 6.0)
Draft each chapter sequentially with full context (voice, world, characters, outline,
canon, adjacent chapters). Evaluate each chapter. Re-draft up to 5 times if below threshold.

Key evaluator dimensions:
- Voice adherence, beat coverage, character voice, plants seeded
- Prose quality, continuity, canon compliance, engagement
- AI pattern detection (the judge actively looks for AI tells)

### Phase 3: Revision (plateau detection)
Three revision sub-phases:

**3a: Adversarial Editing**
- Run `adversarial_edit` on each chapter to find cuts
- Apply cuts (filtered by type and fat percentage)
- Generate revision briefs from eval data, panel feedback, and cuts
- Rewrite chapters from briefs

**3b: Reader Panel**
- Build arc summary from all chapters
- Run 4-persona reader panel (editor, genre reader, writer, first reader)
- Identify disagreements (chapters flagged by some but not all readers)
- These disagreements are the most valuable feedback

**3c: Deep Review**
- Send full manuscript to Opus for dual-persona review
- Stop when: stars >= 4.5 with no major items, OR stars >= 4 with majority qualified items, OR <= 2 items total

Revision cycles continue until novel score plateaus (change < 0.3 for 3+ cycles) or max cycles reached.

### Phase 4: Export
- Rebuild outline from actual chapters (post-revision content)
- Build final arc summary
- Concatenate manuscript.md

## Evaluation Rubric

Scores are 1-10:
- 1-3: Fundamentally broken
- 4-5: Mediocre, obvious problems
- 6-7: Competent, publishable with revisions
- 8-9: Genuinely good
- 10: Reserved for masterwork prose

The mechanical slop penalty is subtracted from scores:
- Tier 1 banned words: 1.5 pts each (max 4.0)
- Tier 2 suspicious clusters: 1.0 per paragraph with 3+ (max 2.0)
- Tier 3 filler phrases: 0.3 each (max 2.0)
- Fiction AI tells: 0.3 each (max 2.0)
- Structural AI tics: 0.5 each (max 2.0)
- Telling patterns: 0.2 each (max 1.5)
- Total cap: 10.0

## Anti-Slop Reference

### Banned Words (Tier 1)
delve, utilize, leverage, facilitate, aforementioned, comprehensive, paramount,
synergy, paradigm, holistic, nuanced, multifaceted, intricate, pivotal, commences,
culminates, endeavor, myriad, plethora

### Fiction AI Tells
"a sense of [emotion]", "eyes widened", "heart pounded in", "couldn't help but",
"let out a breath", "a flicker of", "a wave of X washed", "jaw clenched",
"the weight of X settled", "something shifted", "a mixture of X and Y",
"the air [thick/heavy/charged]", "silence stretched", "darkness closed in"

### Structural Anti-Patterns
1. The Over-Explain: spelling out what the reader can infer
2. Triadic Listing: "X, Y, and Z" as rhythm crutch
3. Negative-Assertion Repetition: "He did not X. He did not Y."
4. Cataloging-by-Thinking: "She thought about X. Then about Y."
5. The Simile Crutch: simile in every paragraph
6. Section Break as Rhythm Crutch: breaking instead of transitioning
7. Paragraph Length Uniformity: all paragraphs same length
8. Predictable Emotional Arcs: every chapter ends on resolution
9. Balanced Antithesis: "Not just X, but Y"
10. Dialogue as Written Prose: characters speaking in complete paragraphs

## Craft Reference

### Save the Cat Beats
Opening Image (0-1%), Theme Stated (5%), Setup (1-10%), Catalyst (10%),
Debate (10-20%), Break into Two (20%), B Story (22%), Fun and Games (20-50%),
Midpoint (50%), Bad Guys Close In (50-75%), All Is Lost (75%), Dark Night (75-80%),
Break into Three (80%), Finale (80-99%), Final Image (99-100%)

### Character Framework
Every major character needs:
- Wound: the formative event
- Want: conscious goal
- Need: unconscious need (resisted)
- Lie: false belief they cling to
- Three Sliders: competence, likability, proactivity (1-10)

### Dialogue Distinctiveness (8 Dimensions)
1. Vocabulary level
2. Sentence length tendency
3. Question frequency
4. Interruption style
5. Emotional expression
6. Humor type
7. Power language
8. Speech tics or catchphrases

## Art Generation

Requires FAL_KEY. Uses fal.ai Nano Banana 2 model.

Workflow:
1. `gen_art_style` -- derives visual direction from world/voice
2. `gen_art` with art_type=cover -- generates cover art
3. `gen_art` with art_type=ornament, chapter=N -- generates per-chapter ornaments
4. `gen_art` with art_type=map -- generates world map
5. `gen_art` with art_type=scene_break -- generates scene break decoration

## Audiobook Generation

Requires ELEVENLABS_API_KEY and audiobook_voices.json mapping characters to voice IDs.

Workflow:
1. `gen_audiobook_script` -- parses chapters into speaker-attributed segments
2. `gen_audiobook` -- converts scripts to audio via ElevenLabs Text to Dialogue

## Project Directory Structure
```
project/
  state.json          Pipeline state
  results.tsv         Experiment log
  seed.txt            Novel seed concept
  voice.md            Voice bible
  world.md            World bible
  characters.md       Character registry
  outline.md          Chapter outline
  canon.md            Canon database
  MYSTERY.md          Mystery/subtext (optional)
  arc_summary.md      Arc summary for reader panel
  manuscript.md       Concatenated final manuscript
  chapters/
    ch_01.md ... ch_NN.md
  briefs/
    chNN_eval.md, chNN_panel.md, chNN_cuts.md
  eval_logs/
    YYYY-MM-DDTHH-MM_foundation.json
    YYYY-MM-DDTHH-MM_chNN.json
    YYYY-MM-DDTHH-MM_full.json
  edit_logs/
    chNN_cuts.json
    reader_panel.json
    review.json
    tournament_results.json
    voice_fingerprint.json
  art/
    visual_style.json
    cover.png
    ornament_chNN.png
    map.png
    scene_break.png
  audiobook/
    scripts/chNN_script.json
    chapters/ch_NN.mp3
    full_audiobook.mp3
  audiobook_voices.json
```

## Environment Variables
```
ANTHROPIC_API_KEY          Required. Claude API key.
AUTONOVEL_WRITER_MODEL     Default: claude-sonnet-4-6
AUTONOVEL_JUDGE_MODEL      Default: claude-opus-4-6
AUTONOVEL_REVIEW_MODEL     Default: claude-opus-4-6
AUTONOVEL_API_BASE_URL     Default: https://api.anthropic.com
FAL_KEY                    Optional. For art generation.
ELEVENLABS_API_KEY         Optional. For audiobook generation.
```

## Troubleshooting

- **Foundation score stuck below threshold**: Check that seed.txt has a specific, concrete concept. Generic seeds produce generic foundations.
- **Chapter scores consistently low**: Run `slop_check` to identify mechanical issues. Run `voice_fingerprint` to check for statistical outliers in sentence rhythm.
- **Revision not improving**: Check if the pipeline has plateaued. Try `adversarial_edit` on the weakest chapter (from `evaluate_full`) and revise specifically.
- **Reader panel disagreements**: These are the most valuable signal. Focus revision on chapters where readers disagree -- those are the chapters with unresolved issues.
- **Art generation fails**: Verify FAL_KEY is set. Run `gen_art_style` first to establish visual direction.
