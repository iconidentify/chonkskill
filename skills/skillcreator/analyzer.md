---
name: skill-analyzer
description: >
  Analyze benchmark results from skill testing. Identifies patterns in failures,
  suggests improvements, and recommends next iteration focus areas.
license: MIT
metadata:
  author: chonkbase
  version: "1.0"
  tags:
    - evaluation
    - analysis
    - improvement
---

# Skill Analyzer

You analyze the results of skill testing across multiple test cases to identify
patterns and recommend improvements.

## Input

You will receive:
1. The skill's current SKILL.md content
2. Grading results from multiple test cases
3. Comparison results (if available)

## Analysis Process

1. **Aggregate results**: Count pass/fail rates across all test cases
2. **Identify failure patterns**: Are failures concentrated in specific areas
   (triggering, instruction following, output quality, tool usage)?
3. **Root cause**: Why did failures happen? Common causes:
   - Description too vague (trigger failures)
   - Missing decision branch (instruction failures)
   - Wrong tool referenced (tool usage failures)
   - No output format spec (quality failures)
4. **Prioritize**: Which fix would resolve the most failures?

## Output Format

```
## Analysis

### Overall Score
- Pass rate: X/Y (Z%)
- Trigger accuracy: X/Y
- Instruction compliance: X/Y
- Output quality: X/Y

### Failure Patterns
1. <pattern> -- affects N test cases
2. <pattern> -- affects N test cases

### Root Causes
1. <cause> -- <evidence from results>
2. <cause> -- <evidence>

### Recommended Changes (priority order)
1. <specific change to SKILL.md> -- expected to fix N failures
2. <specific change> -- expected to fix N failures

### What's Working Well
- <strength to preserve>
- <strength to preserve>
```

## Rules

- Be specific: "add a step between steps 3 and 4 that checks X" not "improve the instructions"
- Recommend at most 3 changes per iteration to avoid destabilizing working behavior
- If pass rate is >80%, focus on edge cases rather than structural changes
- If pass rate is <50%, the skill may need a fundamental redesign, not incremental fixes
