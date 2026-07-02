---
name: lifecycle-phases
description: >-
  Full phase details for the development lifecycle — Phase 0 comprehend,
  Phase 1 challenge, Phase 1.5 research, Phase 2 plan, Phase 3 implement
  (task context template, scope checks), Phase 4 verify (linter suite, L1/L2
  reviews, semantic check, holistic review), plus the plan change protocol,
  verification failure recovery, and conflict resolution.
when_to_use: >-
  Use when executing Tier 2 or Tier 3 changes that need full phase guidance —
  starting a new feature or package, running comprehend/plan/implement/verify,
  dispatching implementation subagents with task context, changing an approved
  plan mid-implementation, recovering from verification failures, or resolving
  conflicts between rules, reviewers, and the user.
metadata:
  author: koopa
  version: "1.0"
  lang: go
---

# Lifecycle Phases — Full Details

The rule `development-lifecycle.md` defines tier selection and flow. This skill provides full phase details.

---

## Phase 0: COMPREHEND (Tier 3)

**Agent**: `comprehend` (MUST delegate)

1. **Existing code** — read relevant packages, understand architecture, types, responsibilities
2. **User's intent** — what problem, what business context
3. **Go semantics** — what package name, what type names, what responsibilities
4. **Integration points** — how it connects to existing packages

Output: comprehension report mapping request to concrete Go concepts.

## Phase 1: CHALLENGE (Tier 3, within comprehend)

ACTIVELY challenge: ambiguous requirements, wrong direction, convention violations, over-engineering, missing context.

- NEVER say "Great idea!" without verification
- NEVER proceed if ambiguous — ask
- If request violates a project rule, say so IMMEDIATELY

Ends when: request is clear, direction validated, user confirmed.

## Phase 1.5: RESEARCH (Conditional)

Runs ONLY when comprehend report contains `## Research Needed`. Orchestrator invokes `/research` skill. See skill for full process.

## Phase 2: PLAN (Tier 3)

**Agent**: `planner` (after comprehend + optional research)

Design: file structure, types, functions, package boundaries, API surface, schema, out-of-scope. Plan MUST be approved before implementation.

## Phase 3: IMPLEMENT

Follow approved plan at `.claude/plans/<feature>.md`. Do not deviate.

- One logical unit at a time
- Lint gate after each unit: `go build ./... && go vet ./... && golangci-lint run ./...`
- ALL linter issues fixed before next unit
- Do not add unplanned features

### Task Context Template (subagent dispatch)

```markdown
## Task: [name from plan]
## Plan file: .claude/plans/<feature>.md — Task N
## Files to create/modify: [explicit list]
## Types/interfaces this task depends on:
[Include relevant type definitions VERBATIM]
## Scope: [what this task does]
## Out of scope: [what this task must NOT do]
## Verification: [exact command]
```

**HARD RULE**: Craft context from scratch. NEVER forward conversation history. Subagent has zero session knowledge.

**Red flags** — rewrite if prompt contains: "as we discussed", "based on the above", "continuing from where we left off", raw conversation fragments.

### Scope Check (per task, Tier 3)

After each task: did it modify only planned files? Unplanned files = evaluate Plan Change Protocol.

---

## Phase 4: VERIFY

### Step 1: Linter Suite
`go build ./... && go vet ./... && golangci-lint run ./... && go test ./...` — zero issues.

### Step 2: L1 Reviews
| Condition | Agent |
|-----------|-------|
| Any `.go` modified | `go-reviewer` (always) |
| `.sql` or `*store*.go` | `db-reviewer` |
| Handlers, auth, SQL | `security-reviewer` |

Parallel Agent Team when 2+ triggered.

### Step 2.5: L2 Review
- **Tier 1**: skip L2
- **Tier 2**: L2 when new handler, store logic change, or security-sensitive. Skip for rename/docs/test-only.
- **Tier 3**: always

### Step 3: Semantic Check (Tier 3)
Compare implementation against comprehension report + plan: package boundaries, type names, API surface, schema, responsibilities, out-of-scope.

### Step 4: Holistic Review (Tier 3 only)

Three lenses with concrete evidence (file paths, line numbers):

**Lens 1 — Coherence**: Do all modified files agree? Same names, consistent assumptions, no cross-file conflicts.

**Lens 2 — Cognitive Load**: Net lines proportional to problem? Happy path obvious? No always-N/A checklist items?

**Lens 3 — Devil's Advocate**: New interface with 1 impl? Function called from 1 place? Test-only interface? AI copy-paste pattern? Validation drift (Create validates X, Update doesn't)?

**Synthesis**: State: improved/neutral/degraded. Verdict: SHIP / SHIP WITH OBSERVATIONS / REVISE.

---

## Plan Change Protocol

**Minor** (same files, different detail): note deviation, update plan file, continue.

**Major** (new files/types/API/schema): STOP → checkpoint → describe change → update plan → user approval → resume.

Major triggers: new table/column, new package, changed endpoint, type rename, new dependency.

---

## Verification Failure Recovery

| Level | Trigger | Action |
|-------|---------|--------|
| 1: Auto-Fix | Missing imports, formatting, errcheck | Fix → re-run `/verify` |
| 2: Targeted | Test failure, type mismatch, logic error | Diagnose → fix → re-run from Step 1 |
| 3: Design | Reviewer finds plan contradiction | STOP → report → Plan Change Protocol |
| 4: Fundamental | Cascading failures, can't satisfy plan | STOP → report → return to Phase 2 |

- NEVER suppress warnings to pass. Max 3 auto-fix attempts → escalate.

---

## Conflict Resolution

- **Convention violation**: cite rule once, explain alternative once, respect user's final decision. Hook will block if hard-enforced.
- **Scope disagreement**: state recommendation, proceed with user's scope, note risks.
- **Technical disagreement**: present trade-offs, ask which approach, implement chosen one, don't revisit.
