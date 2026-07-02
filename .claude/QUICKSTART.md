# Quickstart — Decision Tree for New Sessions

This document helps Claude Code quickly determine the correct action for any user request.

## Decision Tree

```
User Request
     │
     ├─ Research/Exploration? ("Where is X?", "How does Y work?")
     │        → Read files directly or Task(subagent_type="Explore")
     │        → Do NOT use full comprehend report (that's for Tier 2-3 code changes)
     │
     ├─ Tool/Skill Request? ("/verify", "/checkpoint", "run tests")
     │        → Execute the skill/command directly
     │
     ├─ Spec Management? ("add skill", "add rule", "add hook", "add agent", "validate config")
     │        → Skill(skill="manage-spec")
     │
     └─ Code Change? → Select Tier (see development-lifecycle.md)
              │
              ├─ Tier 1: Direct Fix
              │   ALL true: obvious cause, 1-3 files, no design, no type/API changes
              │        → Fix directly → /verify → go-reviewer → review-code
              │
              ├─ Tier 2: Existing Feature Modification
              │   ALL true: existing package, no new packages/types/APIs, no new deps
              │        → Lightweight comprehend (3-5 line scope summary)
              │        → Implement → /verify + L1 reviewers → review-code (L2)
              │
              └─ Tier 3: Full Lifecycle
                  ANY true: new feature, new package, design decisions, refactor
                       → Task(comprehend) → Task(planner) → implement → /verify + L1 reviewers → review-code (L2) + semantic check
```

## Agent Invocation

To delegate to an agent, use the **Task tool** with `subagent_type`:

```
Task(subagent_type="comprehend", prompt="User wants to add authentication...")
Task(subagent_type="planner", prompt="Based on comprehension report, design...")
Task(subagent_type="scaffold", prompt="Create internal/auth/ with planned structure...")
Task(subagent_type="go-reviewer", prompt="Review the changes in internal/auth/...")
```

## Skill Invocation

Skills are invoked via the **Skill tool** or `/command` syntax:

```
Skill(skill="verify")           → /verify
Skill(skill="checkpoint")       → /checkpoint
Skill(skill="pgx-patterns")     → /pgx-patterns
```

## Quick Reference — What Agent For What

See `CLAUDE.md` → "Available Agents" for the full agent list.

Common triggers: `go-reviewer` (code written), `db-reviewer` (SQL/store changed), `security-reviewer` (auth/crypto), `review-code` (/verify passes, deep review, pre-PR), `test-writer` (write tests), `build-resolver` (build broken), `refactor` (simplify), `perf-reviewer` (slow).

## Conflict Resolution

When rules conflict with user requests:

| Priority | Source | Action |
|----------|--------|--------|
| 1 (highest) | Hooks | Cannot override — Write/Edit will be blocked |
| 2 | CLAUDE.md / Rules | Cite the rule, explain why, suggest alternative |
| 3 | Agent instructions | Follow unless user explicitly overrides |
| 4 | User preference | Respect after noting concerns once |

If user insists on violating a rule after being informed:
1. Note the concern ONCE
2. Proceed with user's approach
3. Do NOT repeat the objection

## Example Workflow

**User**: "Add JWT authentication"

**Claude**:
1. "I'll analyze the codebase first to understand the current structure."
2. `Task(subagent_type="comprehend", prompt="User wants JWT auth. Analyze existing code, identify integration points, challenge if needed.")`
3. [Comprehend returns with analysis and questions]
4. "Before proceeding, I need to clarify: Should JWT be stored in cookies or localStorage? What endpoints need protection?"
5. [User answers]
6. `Task(subagent_type="planner", prompt="Design JWT auth based on comprehension report and user clarifications...")`
7. [Planner returns with design]
8. "Here's the plan: [summary]. Does this look correct?"
9. [User approves]
10. `Task(subagent_type="scaffold", prompt="Create internal/auth/ according to plan...")`
11. `/verify`
12. `Task(subagent_type="go-reviewer", prompt="Review internal/auth/...")`
13. `Task(subagent_type="security-reviewer", prompt="Review auth implementation...")`
14. "Implementation complete. All checks pass."

## Memory System

Agents with `memory: project` persist learnings to `.claude/agent-memory/<agent-name>/<filename>.md`.

This is used for:
- Remembering project-specific patterns discovered during review
- Caching codebase understanding across sessions
- Accumulating known issues and their resolutions

Memory files are human-readable markdown and can be edited manually.
