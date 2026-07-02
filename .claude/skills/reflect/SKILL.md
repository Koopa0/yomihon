---
name: reflect
description: >-
  Session learning curator — captures corrections, discoveries, and
  reusable insights to an append-only staging log during work, then
  reviews accumulated learnings for human-gated promotion to persistent
  memory, rules, or skill updates. Defines the capture format and an
  Enforcement-Ladder Gate for rule candidates; never auto-promotes.
when_to_use: >-
  Use when the user says "/reflect", "review learnings", "what did we
  learn", "promote this to memory", or at the end of a significant work
  session. Also use mid-conversation when the user corrects your
  approach ("no, use X instead", "actually..."), a non-obvious pattern
  or debugging root cause is discovered, a review agent flags a
  recurring issue, or a project convention is clarified.
metadata:
  author: koopa
  version: "1.0"
  lang: go
  security: human-gated
---

# Reflect — Session Learning Capture and Review

## Identity

You are a learning curator. You have two responsibilities:

1. **During work**: recognize corrections, discoveries, and reusable insights, and log them to a staging file
2. **When invoked via `/reflect`**: review accumulated learnings and help the user promote them to persistent storage

You NEVER auto-promote learnings. Every promotion requires explicit user approval.

---

## Part 1: Capture (During Any Conversation)

### When to Capture

Log a learning when ANY of these occur:

| Trigger | Example |
|---------|---------|
| User corrects your approach | "no, use X instead of Y", "actually...", "don't do that" |
| You discover a non-obvious pattern | pgx quirk, sqlc gotcha, Go stdlib surprise |
| A review agent finds a recurring issue | Same finding across multiple reviews |
| A debugging session reveals root cause | Non-trivial root cause worth remembering |
| A convention is clarified | "in this project, we always do X because Y" |

### When NOT to Capture

| Skip When | Why |
|-----------|-----|
| The insight is already in an existing rule | Check `.claude/rules/` first |
| The insight is already in memory | Check `MEMORY.md` first |
| The correction is situation-specific | "not this file" ≠ "never do this" |
| The learning is obvious Go knowledge | Standard Effective Go, not project-specific |

> **Rule candidates face the Enforcement-Ladder Gate.** Still capture the learning — but any `candidate-target: rule:<file>` entry must pass the mandatory Enforcement-Ladder Gate (Part 2, Step 2.5) before it may become a rule. Prose rules are the LAST resort, behind linters and hooks.

### Capture Format

Append to `.claude/session-learnings.log`:

```
---
date: YYYY-MM-DD
source: user-correction | self-discovery | review-finding | debug-finding
context: <what we were doing when this was learned>
learning: <the insight, correction, or pattern — one clear sentence>
candidate-target: memory | rule:<rule-file> | skill:<skill-name> | agent-memory:<agent>
---
```

### Capture Rules (Non-Negotiable)

1. **Append-only.** NEVER edit or delete existing entries in the log.
2. **NEVER write to CLAUDE.md, rules/, skills/, or agents/ during capture.** The log file is the ONLY write target.
3. **One entry per learning.** Keep each entry under 5 lines.
4. **Check for duplicates.** Read the log file before appending. Don't log the same thing twice.
5. **Be specific.** "pgx requires X" is good. "database stuff" is bad.

---

## Part 2: Review (Via `/reflect`)

When the user invokes `/reflect`, execute this workflow:

### Step 1: Read Accumulated Learnings

```bash
cat .claude/session-learnings.log
```

If the file is empty or doesn't exist, report: "No learnings accumulated. Nothing to review."

### Step 2: Group and Present

Group learnings by `candidate-target`. For each learning, present:

```markdown
### Learning #N

- **Date**: YYYY-MM-DD
- **Source**: user-correction
- **Context**: While implementing order creation...
- **Learning**: pgx.CollectRows returns nil slice, not empty slice, when no rows match
- **Proposed target**: skill:pgx-patterns
- **Proposed change**: Add to "Common Pitfalls" section:
  > `CollectRows` returns `nil` (not `[]T{}`). Use `emit_empty_slices` in sqlc config for list endpoints.

**Action?** [promote / defer / discard]
```

### Step 2.5: Enforcement-Ladder Gate (Mandatory for Rule Candidates)

Before ANY learning may be proposed as `rule:<file>`, walk it down this ladder. Stop at the first rung that holds and present that rung's proposal instead of a rule. For rule candidates, the Step 2 presentation MUST include a **Gate verdict** line (rung + one-sentence reasoning).

1. **Linter rung** — Can an existing golangci-lint linter catch it? → Propose a `.golangci.yml` delta (enable the linter / add its setting), NOT a rule.
2. **Hook rung** — Is it project-specific but mechanically checkable? → Propose a hook, WITH a covering case in `tests/test-hooks.sh`. (New hooks go through `/manage-spec add hook`.)
3. **Rule rung** — Only if neither rung applies may it become a rule MUST/NEVER line. Every promoted MUST/NEVER line must cite the concrete failure from `.claude/session-learnings.log` that motivated it (date + context), so the rule traces to a real incident, not a hypothetical.

The gate changes WHAT is proposed, never WHO decides: every outcome (lint delta, hook, or rule line) still requires explicit user approval before anything is written. NEVER auto-promote.

### Step 2.6: Verify the Claim Before Proposing It

A learning is only as good as the evidence under it. Before presenting any learning that rests on an external fact (a Claude Code feature, a library API, a tool flag, a version behavior):

- **Retrieve, don't recall.** Confirm the fact from a primary source (official docs via WebFetch, the actual file, a real command run) — not from memory. State the source.
- **When sources or sub-agents conflict, trust the one that quotes retrieved content over the one that defaults to doubt.** An agent that got a 404 and marked everything "SUSPECT" is weaker evidence than one that fetched the page and quoted it. Cross-check the disputed point against a third source (e.g. a JSON schema, a second doc page, a live test) before promoting.
- **Distinguish "in context" from "obeyed."** A rule being loaded does not prove code follows it. If a learning claims "the harness enforces X," verify the mechanical gate exists (lint/hook/CI) — not just that a rule file says X. Prefer promoting to the rung that actually fails the build (Step 2.5).
- **Separate skill-triggering from rule-compliance.** Skills are on-demand (may legitimately not load on a question Claude can answer directly); rules with `paths:` frontmatter auto-inject. A skill not firing is NOT evidence that governance was skipped — check the right layer for the claim.
- If a claim cannot be verified, label it UNCONFIRMED in the presentation and do not promote it to a MUST/NEVER.

### Step 3: Process User Decisions

For each learning, the user chooses:

| Decision | Action |
|----------|--------|
| **promote** | Apply the change to the target file. Show the diff before applying. |
| **defer** | Keep in the log for later review. |
| **discard** | Remove from the log. |

### Step 4: Clean Up

After all items are processed:
- Remove promoted and discarded entries from the log
- Keep deferred entries for next `/reflect` session
- Report summary: "Promoted N learnings, deferred M, discarded K."

---

## Promotion Targets

| Target | Where It Goes | When to Use |
|--------|---------------|-------------|
| `memory` | User's MEMORY.md | Project decisions, user preferences, key dates |
| `rule:<file>` | `.claude/rules/<file>.md` | Pattern that should be enforced in all future code — ONLY after passing the Enforcement-Ladder Gate (Step 2.5) |
| `lint` | `.golangci.yml` | Gate rung 1: an existing golangci-lint linter can catch it |
| `hook` | `.claude/hooks/` + `tests/test-hooks.sh` case | Gate rung 2: project-specific but mechanically checkable |
| `skill:<name>` | `.claude/skills/<name>/SKILL.md` | Deep pattern for a specific technology/framework |
| `agent-memory:<agent>` | `.claude/agent-memory/<agent>/` | Agent-specific learning (conventions, false positives) |

### Promotion Rules

1. **Show the exact diff** before applying any promotion. User must see what will change.
2. **Never create new files** during promotion. Only append to or modify existing files.
3. **If a learning suggests a NEW rule or skill**, tell the user to use `/manage-spec add rule` or `/manage-spec add skill` instead.
4. **Promoted content must match the target file's style.** Read the target file first, then match its formatting.
5. **Enforcement-Ladder Gate is mandatory for `rule:<file>` targets.** No learning becomes a rule without a recorded gate verdict (Step 2.5), and every promoted MUST/NEVER line cites the concrete failure from its motivating `.claude/session-learnings.log` entry.

---

## Security Model

This skill is designed with these security constraints:

| Constraint | Rationale |
|------------|-----------|
| Staging file only — no direct writes to CLAUDE.md/rules/skills | Prevents persistent prompt injection |
| Human gate on every promotion | Ensures a human validates each learning |
| Log file not auto-loaded into context | `.claude/session-learnings.log` is not in `rules/` or `skills/`, so agents don't read it automatically |
| Append-only capture | Prevents tampering with existing entries |
| No auto-detection of corrections | Claude decides to log based on judgment, not regex pattern matching — avoids false positives |

### What Could Go Wrong (and how we prevent it)

| Threat | Mitigation |
|--------|------------|
| Prompt injection logged as "learning" | Human reviews every entry before promotion |
| Contradicts existing rule | `/reflect` shows diff; human can see the conflict |
| Noise accumulation (too many low-value entries) | `/reflect` includes "discard" option; periodic cleanup |
| Concurrent session writes | Append-only semantics; worst case is duplicate entries |

---

## Example Session

During implementation:
```
Claude notices user said: "no, don't use fmt.Errorf here, use errors.Join for multiple errors"
Claude appends to session-learnings.log:
---
date: 2026-03-18
source: user-correction
context: implementing error aggregation in order validation
learning: use errors.Join for aggregating multiple validation errors, not repeated fmt.Errorf wrapping
candidate-target: rule:error-handling.md
---
```

Later, user runs `/reflect`:
```
### Learning #1

- **Date**: 2026-03-18
- **Source**: user-correction
- **Learning**: use errors.Join for aggregating multiple validation errors, not repeated fmt.Errorf wrapping
- **Proposed target**: rule:error-handling.md
- **Proposed change**: Add to error-handling.md under "Wrapping" section:
  > For aggregating multiple independent errors (e.g., validation), use `errors.Join(errs...)` instead of repeated `fmt.Errorf` wrapping.

**Action?** [promote / defer / discard]
```

User says "promote" → Claude shows diff → user confirms → Claude applies.
