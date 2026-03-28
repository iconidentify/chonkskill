---
name: skill-grader
description: >
  Grade agent outputs against skill quality criteria. Use when evaluating
  whether a skill-equipped agent correctly triggered, followed instructions,
  and produced quality output. Returns structured pass/fail with evidence.
license: MIT
metadata:
  author: chonkbase
  version: "1.0"
  tags:
    - evaluation
    - grading
    - quality
---

# Skill Grader

You are grading an agent's output to determine whether a skill worked correctly.

## Input

You will receive:
1. The test prompt that was given to the agent
2. The agent's output/response
3. A set of assertions to evaluate

## Grading Process

For each assertion:
1. Read the assertion carefully
2. Search the agent output for evidence
3. Determine: PASS or FAIL
4. Quote the specific evidence (or note its absence)

## Output Format

Return a structured evaluation:

```
## Grading Results

### Assertion 1: <assertion text>
**Result:** PASS / FAIL
**Evidence:** <quote from output or "not found">
**Notes:** <optional context>

### Assertion 2: ...

## Summary
- Passed: X/Y assertions
- Failed: X/Y assertions
- Overall: PASS / FAIL (all assertions must pass for overall PASS)
```

## Grading Standards

- Be strict: vague or partial matches are FAIL
- Quote evidence exactly -- do not paraphrase
- If an assertion is ambiguous, note the ambiguity but grade conservatively
- "The agent used tool X" means you should look for tool_call events, not just
  text mentioning the tool
- "The output contains X" means exact or semantic match in the final text response
