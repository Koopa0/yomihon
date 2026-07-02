---
name: manage-spec
description: >-
  Meta-management for the go-spec Claude Code configuration — adds, lists, and
  validates skills, rules, hooks, and agents. Provides standard skeletons for
  each artifact type, updates the CLAUDE.md tables and .agents/skills symlinks,
  and checks configuration consistency.
when_to_use: >-
  Use when adding a new skill, rule, hook, or agent to this project, listing
  the current spec inventory, or validating .claude/ configuration consistency.
  Trigger phrases: "/manage-spec", "add a skill", "create a new rule", "new
  hook", "new agent", "list skills", "validate the spec", "is CLAUDE.md in
  sync".
metadata:
  author: koopa
  version: "1.0"
  lang: go
---

# Manage Spec — Add / List / Validate

Use this skill to manage the go-spec project's Claude Code configuration: skills, rules, hooks, and agents.

## Commands

### `/manage-spec add skill <name>`

Creates a new skill with the standard skeleton.

**Steps:**
1. Validate the name (lowercase, no spaces, no conflicts with existing skills)
2. Create `.claude/skills/<name>/SKILL.md` with the skeleton below
3. Update `CLAUDE.md` → "Available Skills" table
4. Add it to the `using-go-spec` router (Task→Skill table) — `test-skill-format.sh`
   FAILS if a non-exempt skill is missing from the router
5. Update `.agents/skills/<name>` symlink (for cross-agent compatibility)
6. **Listing-budget check**: every skill's `description`+`when_to_use` shares the
   ~1% per-turn listing budget. Keep the combined text lean (≤1536 chars per skill,
   enforced; aim far lower). A deep reference that the main session rarely needs to
   auto-discover should be set `name-only` in `settings.json` → `skillOverrides`
   (still `/`-invocable + agent-preloadable). Run `test-skill-format.sh` to see the
   aggregate advisory.
7. If the body exceeds ~400 lines, split deep material into `references/<topic>.md`
   (progressive disclosure — auto-compaction only re-attaches the first ~5,000
   tokens of a skill).
8. Report what was created

**Skill Skeleton:**

```markdown
---
name: {{name}}
description: {{1-2 dense sentences — WHAT this skill is/contains. No trigger phrases.}}
when_to_use: >-
  Use when {{trigger conditions, user-phrase examples, file/task contexts —
  the keywords a request would contain. Triggers only, never a workflow summary.}}
metadata:
  author: koopa
  version: "1.0"
  lang: go
---

# {{Name}} — {{Short Title}}

{{Description of what this skill covers.}}

## When to Use

- {{Trigger condition 1}}
- {{Trigger condition 2}}

## Patterns

### {{Pattern 1 Name}}

{{Code example or explanation}}

### {{Pattern 2 Name}}

{{Code example or explanation}}

## Decision Tree

```
{{Decision question?}}
├─ Yes → {{action}}
└─ No  → {{action}}
```

## NEVER

- {{Anti-pattern 1}}
- {{Anti-pattern 2}}

## See Also

- {{Related skill or rule}}
```

---

### `/manage-spec add rule <name>`

Creates a new rule file.

**Steps:**
1. Validate the name (lowercase-with-dashes, no conflicts)
2. Create `.claude/rules/<name>.md` with the skeleton below
3. Update `CLAUDE.md` if the rule should be referenced (ask user)
4. Report what was created

**Rule Skeleton:**

```markdown
# {{Name}} Rules

## {{Section 1}}

{{Rules as imperative statements.}}

## {{Section 2}}

| Check | Pattern | Severity |
|-------|---------|----------|
| {{what to check}} | {{code example}} | BLOCKING / IMPORTANT / SUGGESTION |

## NEVER

- NEVER {{anti-pattern}} — {{why}}

## See Also

- {{Related rule or skill}}
```

---

### `/manage-spec add hook <type> <name>`

Creates a new hook script and registers it in settings.json.

**Parameters:**
- `<type>`: `PreToolUse` or `PostToolUse`
- `<name>`: hook script name (e.g., `check-imports`)

**Steps:**
1. Validate the name (lowercase-with-dashes, no conflicts with existing hooks)
2. Create `.claude/hooks/<name>.sh` with the skeleton below
3. Make it executable (`chmod +x`)
4. Update `.claude/settings.json` to register the hook under the correct type
5. Report what was created and the registration

**Hook Skeleton:**

```bash
#!/usr/bin/env bash
# Hook: {{name}}
# Type: {{PreToolUse|PostToolUse}}
# Purpose: {{one-line description}}
#
# Exit codes:
#   0 = pass (allow operation)
#   2 = block (PreToolUse only — prevents the tool execution)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/parse-hook-input.sh"

# Parse input from stdin
INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | parse_file_path)

# Skip if not relevant
if [[ -z "$FILE_PATH" ]]; then
  exit 0
fi

# --- Hook logic below ---

# {{TODO: implement hook logic}}

exit 0
```

**PreToolUse hooks** can block operations (exit 2). **PostToolUse hooks** are non-blocking (warnings only).

---

### `/manage-spec add agent <name>`

Creates a new agent definition.

**Steps:**
1. Validate the name (lowercase-with-dashes, no conflicts)
2. Ask user for:
   - Model: `opus` or `sonnet`
   - Memory: `project` (persistent) or none
   - Tools: read-only (`Read, Grep, Glob, Bash`) or write (`Read, Grep, Glob, Bash, Write, Edit`)
   - Purpose: one-line description
3. Create `.claude/agents/<name>.md` with the skeleton below
4. If memory enabled, create `.claude/agent-memory/<name>/` directory with starter file
5. Update `CLAUDE.md` → "Available Agents" table
6. Update `.claude/rules/agents.md` → "Auto-Delegation Triggers" (ask user for trigger)
7. Report what was created

**Agent Skeleton:**

```markdown
---
name: {{name}}
description: {{ONE LINE — what this agent does. Include PROACTIVELY trigger conditions.}}
model: {{opus|sonnet}}
tools: {{Read, Grep, Glob, Bash}}
disallowedTools: {{Write, Edit, NotebookEdit (if read-only)}}
{{#if memory}}memory: project{{/if}}
metadata:
  author: koopa
  version: "1.0"
---

# {{Name}} — {{Short Title}}

You are a {{role description}}. Your job is to {{primary responsibility}}.

## Process

### Step 1: {{First Step Name}}

{{What to do first.}}

### Step 2: {{Second Step Name}}

{{What to do next.}}

## Output Format

```
## {{SEVERITY LEVEL}} ({{action required}})
- [file:line] {{Description}} — {{reference to rule}}
```

## Rules

- NEVER modify code — this is a read-only {{review|analysis}}
- {{Rule 2}}
- {{Rule 3}}

## Memory Updates

If during your task you discover any of these, include a `## Memory Update` section at the END of your output:

1. {{What kind of discovery to persist}}
2. {{What kind of discovery to persist}}

Format:
```
## Memory Update
file: .claude/agent-memory/{{name}}/{{filename}}.md
section: <section name>
entry: [YYYY-MM-DD]: <discovery> -- <where found> -- <recommendation>
```

Do NOT include Memory Update if nothing new was discovered.
The main agent will apply the update.

## Next Step

End your output with one of:
- "Next step: no issues found."
- "Next step: fix issues listed above."
```

---

### `/manage-spec list`

Lists all configured skills, rules, hooks, and agents with a one-line summary.

**Steps:**
1. Scan `.claude/skills/*/SKILL.md` — extract `name` and `description` from frontmatter
2. Scan `.claude/rules/*.md` — extract first heading
3. Scan `.claude/hooks/*.sh` — extract `# Purpose:` comment (skip `parse-hook-input.sh`)
4. Scan `.claude/agents/*.md` — extract `name`, `model`, `memory` from frontmatter
5. Present as 4 tables

**Output Format:**

```markdown
## Agents (N total)
| Name | Model | Memory | Description |
|------|-------|--------|-------------|

## Skills (N total)
| Name | Description |
|------|-------------|

## Rules (N total)
| Name | First Heading |
|------|---------------|

## Hooks (N total)
| Name | Type | Description |
|------|------|-------------|
```

---

### `/manage-spec validate`

Validates cross-reference consistency across all configuration files.

**Checks:**

1. **CLAUDE.md ↔ Agents**: Every agent in `.claude/agents/*.md` appears in CLAUDE.md "Available Agents" table, and vice versa
2. **CLAUDE.md ↔ Skills**: Every skill in `.claude/skills/*/SKILL.md` appears in CLAUDE.md "Available Skills" table, and vice versa
3. **settings.json ↔ Hooks**: Every hook referenced in `.claude/settings.json` exists as a file in `.claude/hooks/`, and vice versa (excluding `parse-hook-input.sh`)
4. **agents.md ↔ CLAUDE.md**: Agent triggers in `agents.md` reference agents that exist in CLAUDE.md
5. **Agent memory dirs**: Every agent with `memory: project` has a corresponding `.claude/agent-memory/<name>/` directory
6. **Symlinks**: Every skill has a corresponding `.agents/skills/<name>` symlink (cross-agent compatibility)
7. **QUICKSTART.md**: Agent names referenced in examples exist

**Output Format:**

```markdown
## Validation Results

### Passed (N checks)
- [OK] All agents in CLAUDE.md match .claude/agents/
- [OK] All skills in CLAUDE.md match .claude/skills/

### Failed (N checks)
- [FAIL] Agent "review-code" missing from CLAUDE.md Available Agents table
- [FAIL] Skill "manage-spec" missing from CLAUDE.md Available Skills table

### Warnings (N)
- [WARN] Agent "perf-reviewer" has no memory but has agent-memory directory
```

## Cross-Agent Compatibility

When adding a skill, also create a symlink for Codex/Gemini compatibility:

```bash
ln -sf ../../.claude/skills/<name> .agents/skills/<name>
```

This ensures skills are discoverable by all AI agents configured in the project.

## Validation is Non-Destructive

`/manage-spec validate` only reports — it never auto-fixes. The user decides what to fix.
