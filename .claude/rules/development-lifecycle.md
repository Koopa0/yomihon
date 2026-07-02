# Development Lifecycle — Mandatory Workflow

Every code change follows one of three tiers. Choose the tier FIRST. For full phase details, read `/lifecycle-phases`.

## Tier Selection

### Tier 1: Direct Fix
**ALL must be true**: obvious root cause OR typo, 1-3 files, no new files, no design/type/API changes, no new packages/deps.

**Flow**: `Root cause: X. Validate: Y.` → semantic confirm → fix → `/verify` → go-reviewer (L1) → DONE

**Semantic confirm** (one line): "I'm modifying `[pkg].[func]`, whose responsibility is: [one sentence]. Does my fix change this? Yes → Tier 2."

### Tier 2: Existing Feature Modification
**ALL must be true**: within existing package, not changing public API boundary, no new external deps.

**Flow**: scope hypothesis → implement → scope check → `/verify` + L1 → L2 (if risk-based) → DONE

**Scope hypothesis**: "Scope: package P, files F1-F3. Invalidate: if outside this → Tier 3."

**Quick MECE** (mandatory, one sentence each):
1. What I changed: [files + types + functions]
2. What I didn't change but could be affected: [sibling handlers, callers]
3. How I confirm #2 is unaffected: [tests / check / N/A → Tier 3]

**L2 trigger** (risk-based): L2 review when new handler, store logic change, or security-sensitive code. Skip for rename/docs/test-only.

### Tier 3: Full Lifecycle
**ANY is true**: new package, significant refactor, design decisions, boundary/type changes, new deps.

**Flow**: comprehend → planner → implement → `/verify` + L1 → L2 → semantic check → holistic review → DONE

**Long implementations**: `/goal <completion condition>` (e.g. `/goal go build, vet, lint, and tests all pass`) auto-loops the implement→verify cycle until the condition holds. Use for multi-turn Tier 3 work instead of manual re-prompting. Time-based alternative: `/loop`.

### Escalation Rule

Tier 1→2: fix requires understanding patterns. Tier 1/2→3 if ANY:
- New package or API surface change
- **New interface** — "How many production impls?" 1 → no interface
- **New wrapper/adapter** — "Delete it — what breaks?" Only tests → don't need it
- **New cross-package import** — "Why does A need B?" (one sentence)

### Tier Selection Reasoning (Mandatory)

> Selected Tier N because: [rationale citing criteria]

If this line contains hedging ("probably", "should be", "I think") → mandatory escalate.

## Phase Transitions (Tier 3)

| From | To | Gate |
|------|----|------|
| Request | Tier selection | Output tier + reasoning. Hedging → escalate. |
| Phase 0-1 | Phase 1.5 or 2 | Comprehension report. Research Needed → `/research` first. |
| Phase 1.5 | Phase 2 | Research Report produced. |
| Phase 2 | Phase 3 | Plan approved by user. |
| Phase 3 | Phase 4 | All files created, `go build` passes, scope checks pass. |
| Phase 4 | Done | All verification passes, results reported. |

- `needs clarification` → ask first, don't launch planner
- `recommend alternative` → present alternative, get user confirmation
- `[BLOCKING]` issues → resolve before planning

## Completion Criteria

- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes
- [ ] `golangci-lint run ./...` passes (zero issues)
- [ ] `go test ./...` passes
- [ ] L1: `go-reviewer` no BLOCKING issues
- [ ] L1: Conditional reviewers pass (if applicable)
- [ ] L2: `review-code` no CRITICAL/HIGH (Tier 2 risk-based, Tier 3 always)
- [ ] Semantic check (Tier 3)
- [ ] Holistic review (Tier 3)

## Auto-Commit

When all criteria met: `git add` specific files → commit (conventional format).

## Flow Diagram

```
User Request
     │
     ▼
 TIER SELECTION
     │
     ├─ Tier 1 → hypothesis → fix → /verify → go-reviewer (L1) → DONE
     │
     ├─ Tier 2 → scope hypothesis + MECE → implement → /verify + L1 → [L2 if risk] → DONE
     │
     └─ Tier 3 → COMPREHEND → CHALLENGE → [RESEARCH] → PLAN → IMPLEMENT → VERIFY → L1 → L2 → SEMANTIC → HOLISTIC → DONE
```

## Tier Selection Examples

| Request | Tier | Rationale |
|---|---|---|
| "Fix the typo in the error message" | 1 | Obvious, single-file, no design |
| "Add a status filter to order list" | 2 | Existing feature, existing package |
| "Add email notifications on order creation" | 3 | New package `internal/notification/` |
| "Rename this variable" | 1 | Direct command |
| "Refactor the auth package" | 3 | Restructure, design decisions |
| "Add a column to orders table" | 2 | Existing feature, existing schema |

## Anti-Patterns This Prevents

| Anti-Pattern | Prevention |
|---|---|
| AI blindly creates services/ | Phase 1 catches convention violation |
| AI guesses implementation | Phase 0 forces clarification |
| AI doesn't read existing code | Phase 0 reads packages first |
| AI implements without plan | Phase 2 requires approval |
| AI declares done without testing | Phase 4 requires all checks |
| AI says "Great idea!" to bad idea | Phase 1 forbids this |
| Plan changes → scope creep | Plan Change Protocol (see `/lifecycle-phases`) |
| Verification fails → errors suppressed | Failure Recovery ladder (see `/lifecycle-phases`) |
| Implementation drifts from design | Semantic Check catches deviations |
| 82 interfaces with 1 impl | Escalation rule + interface hook |
| Create validates X, Update doesn't | Quick MECE #2 + handler consistency |
