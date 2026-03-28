---
name: skill-creator
description: >
  Create, test, and refine agent skills. Use when asked to build a new skill,
  improve an existing skill, or set up a capability that other agents should
  learn. Covers the full skill lifecycle: requirements capture, SKILL.md
  authoring, test case design, evaluation via delegate_task, grading, and
  iterative improvement.
license: MIT
metadata:
  author: chonkbase
  version: "1.0"
  tags:
    - skill
    - meta
    - authoring
    - evaluation
  requires_tools:
    - create_skill
    - update_skill
    - delegate_task
---

# Skill Creator

You are building skills for the Chonkbase agent platform. A skill is a
procedural knowledge document that teaches agents how to accomplish specific
tasks. Skills are stored in the `chonk_agent_skills` table and loaded on demand
via `get_skill`.

---

## Skill Anatomy

A skill has two layers:

1. **Frontmatter** (always in context via `list_skills`):
   - `name` -- kebab-case, max 64 chars
   - `description` -- max 1024 chars. This is the trigger: the agent reads
     descriptions to decide which skill to load. Make it specific and slightly
     "pushy" -- undertriggering is worse than overtriggering.
   - `tags` -- comma-separated, used for filtering
   - `metadata.requires_tools` -- only show this skill if the agent has these tools
   - `metadata.fallback_for_tools` -- only show if at least one listed tool is absent

2. **Body** (loaded on demand via `get_skill`):
   - Step-by-step procedural instructions
   - Decision trees for different scenarios
   - Tool usage patterns with exact tool names
   - "When to use" and "when NOT to use" sections
   - Keep under 500 lines

---

## Workflow

### Phase 1: CAPTURE

Interview the user to understand:
- What should the skill do? (concrete actions, not vague goals)
- When should it trigger? (what user messages activate it)
- What tools does the agent need? (check available tools via the platform)
- What does success look like? (expected output format, quality bar)
- Are there cases where it should NOT trigger?

Ask clarifying questions. Do not guess at requirements.

### Phase 2: DRAFT

Write the skill using `create_skill`:
- Name: kebab-case, descriptive (e.g., `git-pr-workflow`, `data-analysis`)
- Description: 1-2 sentences. Include trigger phrases the user would say.
  Good: "Create and manage GitHub pull requests -- use when the user asks to
  open a PR, review code changes, or manage branches."
  Bad: "Helps with GitHub stuff."
- Tags: 3-5 relevant keywords
- Body: Step-by-step instructions referencing actual tool names

### Phase 3: TEST

Design 2-3 test prompts:
- **Should-trigger prompts**: Messages that should cause the agent to load and
  follow this skill
- **Should-not-trigger prompts**: Messages that are superficially similar but
  should NOT activate this skill

For each test prompt, dispatch a test agent via `delegate_task`:
```
delegate_task({
  task: "<test prompt>",
  agent_slug: "chonk-general",
  max_iterations: 30
})
```

Examine the result: did the agent load the skill? Did it follow the
instructions? Was the output correct?

### Phase 4: GRADE

For each test result, evaluate:
1. **Trigger accuracy**: Did the skill activate when it should? Stay quiet when
   it shouldn't?
2. **Instruction following**: Did the agent follow the step-by-step procedure?
3. **Output quality**: Does the result meet the success criteria from Phase 1?
4. **Tool usage**: Did the agent use the right tools in the right order?

Load the `skill-grader` skill via `get_skill("skill-grader")` for structured
grading criteria. Dispatch a grader sub-agent if needed:
```
delegate_task({
  task: "Grade this agent output against these criteria: <criteria>\n\nAgent output:\n<output>",
  max_iterations: 10
})
```

### Phase 5: ITERATE

Based on grading results:
- If trigger accuracy is low: rewrite the description
- If instructions are unclear: add more detail, examples, or decision branches
- If tool usage is wrong: add explicit "use tool X with parameters Y" guidance
- If output quality is poor: add output format specifications

Update the skill via `update_skill` and re-run tests from Phase 3.
Limit to 3 improvement iterations -- if it's not working after 3 rounds,
the skill design may need fundamental rethinking.

### Phase 6: FINALIZE

When tests pass:
1. Summarize what the skill does and how it was tested
2. Report trigger accuracy (X/Y should-trigger passed, X/Y should-not passed)
3. List any known limitations
4. Call `mark_done` with the summary

---

## Description Optimization

The description is the most important part of a skill. Tips:
- Front-load action verbs: "Search", "Create", "Analyze", "Debug"
- Include the domain: "grocery shopping", "tax preparation", "code review"
- Include trigger phrases: "use when the user asks to..."
- Be slightly overinclusive -- false positives are cheaper than false negatives
- Test with edge cases: would a vague prompt like "help me with X" trigger it?

---

## Common Mistakes

- Writing skills that are too vague ("helps with coding")
- Not testing with should-NOT-trigger prompts
- Referencing tool names that don't exist in the agent's toolset
- Making the body too long (>500 lines) -- agents lose focus
- Not including a "when NOT to use" section
- Writing rigid rules instead of explaining reasoning ("always do X" vs
  "prefer X because Y, unless Z")

---

## Platform Reference

**Available tools for agents:**
- `terminal` -- run shell commands in sandbox
- `read_file`, `write_file`, `list_files`, `patch` -- file operations
- `execute_code`, `execute_python` -- code execution
- `web_search`, `web_extract` -- web research
- `delegate_task` -- spawn sub-agents
- `list_skills`, `get_skill` -- discover and load skills
- `create_skill`, `update_skill` -- create and modify skills
- `read_memory`, `write_memory` -- persistent agent memory
- `mark_done` -- signal task completion

**Skill storage:**
- Skills are stored in `chonk_agent_skills` table
- Loaded via `get_skill(name)` which returns the full SKILL.md body
- Listed via `list_skills()` which returns name + description + tags
- Created via `create_skill(name, description, content, tags)`
- Updated via `update_skill(name, content, description?)`
