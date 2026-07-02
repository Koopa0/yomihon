# Agent Memory

This directory stores persistent learnings from agents with `memory: project` enabled.

## Memory-Enabled Agents

| Agent | Memory File | Purpose |
|-------|-------------|---------|
| comprehend | `comprehend/patterns.md` | Codebase structure insights, recurring patterns |
| planner | `planner/decisions.md` | Architectural decisions, plan templates |
| go-reviewer | `go-reviewer/conventions.md` | Project-specific conventions, recurring issues |
| db-reviewer | `db-reviewer/schema-knowledge.md` | Schema knowledge, query patterns |
| security-reviewer | `security-reviewer/threat-model.md` | Threat model, security decisions |
| review-code | `review-code/findings.md` | Recurring bug patterns, convention gaps |

## How Memory Updates Work (Direct Write)

Memory-enabled agents have `Write` tool access and write directly to their own memory file. No delegation needed.

1. Agent completes its task (review, comprehension, planning)
2. If the agent discovers something worth persisting, it reads its memory file to check for duplicates
3. The agent appends the new entry directly to its memory file

### Entry Format

```
[YYYY-MM-DD]: <discovery> -- <where found> -- <recommendation>
```

### Agent Constraints

- Each agent can ONLY write to its own memory file (instruction-enforced)
- Agents cannot use Edit tool — append-only via Write
- Max 200 lines per file; agents remove least useful entries when near limit

## What Agents Should Persist

- A pattern that isn't in the rules but should be followed
- A project-specific convention (e.g., "this project uses X for Y")
- A recurring issue and its resolution
- Architectural context that future sessions should know
- False positive patterns in grep-based checks

## What Agents Should NOT Persist

- Session-specific context (current task, in-progress work)
- Information already covered by existing rules
- Speculative or unverified conclusions
- Temporary state or debugging notes

## Maintenance

- Memory files are committed to git (project knowledge, version-controlled)
- Max 200 lines per file; archive old entries if exceeded
- Entries contradicted by rule changes should be removed
- Review when project architecture changes significantly

## Clearing Memory

To reset a specific agent's memory, delete its `.md` file. The agent will start fresh.

To reset all memory:
```bash
rm -r .claude/agent-memory/*/
```
