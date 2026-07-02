---
name: claude-code-advanced
description: >-
  Advanced Claude Code features reference — Agent Teams, the built-in Task
  system, /batch, /loop, Channels, LSP, Sandbox, plugin packaging, prompt
  hooks, and worktrees, with guidance on when each beats the project's
  subagent model.
when_to_use: >-
  Use when evaluating whether to adopt an advanced Claude Code feature or
  setting up a new capability — e.g. "should we use Agent Teams", enabling
  experimental features, running /batch or /loop, configuring Sandbox or LSP,
  packaging a plugin, or choosing between subagents, teams, and worktrees.
metadata:
  author: koopa
  version: "1.0"
---

# Claude Code Advanced Features

## Agent Teams (Experimental)

Multi-agent orchestration with independent context windows and inter-agent messaging.

**Enable**:
```json
// settings.json (or settings.local.json)
{
  "env": {
    "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1"
  },
  "teammateMode": "auto"  // "in-process" | "tmux" | "auto"
}
```

**When to use vs our Subagent model**:
| Scenario | Use Subagents | Use Agent Teams |
|----------|--------------|-----------------|
| L1 reviews (2-3 agents) | ✅ Lower token cost | |
| /execute-plan tasks | ✅ Crafted context | |
| 3+ independent modules, >500 LOC each | | ✅ True parallel |
| Teammates need to message each other | | ✅ Mailbox system |

**Quality gate hooks**:
```json
"TeammateIdle": [{
  "hooks": [{ "type": "command", "command": ".claude/hooks/check-teammate-output.sh" }]
}],
"TaskCompleted": [{
  "hooks": [{ "type": "command", "command": ".claude/hooks/validate-task-completion.sh" }]
}]
```

## Task System (TaskCreate/TaskUpdate/TaskList/TaskGet)

Built-in task tracking with dependency management. Complements `.claude/plans/<feature>.md`.

**Integration with /execute-plan**:
1. Read plan from `.claude/plans/<feature>.md`
2. `TaskCreate` for each Implementation Task (persistent plan is source of truth)
3. `TaskUpdate(status: "in_progress")` when starting
4. `TaskUpdate(status: "completed")` when done
5. `addBlockedBy`/`addBlocks` for task dependencies

**When to use**:
- Plans = persistent, git-tracked, cross-session design documents
- Tasks = session-scoped, real-time progress tracking with dependency blocking

## /batch — Parallel Large-Scale Changes

Three-phase: Research → Plan → Execute (each unit in isolated worktree).

**Decision tree**:
```
How many files to change?
├── < 5 → /execute-plan (sequential, precise control)
├── 5-30, homogeneous changes → /batch
└── > 30, mechanical pattern → /batch + focused prompt
```

**Usage**: `/batch update all store methods to use context.Context as first parameter`

Each worker runs in its own git worktree → no file conflicts. Workers auto-run `/simplify` before committing.

## /loop — Recurring Monitoring

Session-scoped scheduled tasks. Max 50 concurrent, auto-expire after 3 days.

```
/loop 10m go build ./...          # Build check every 10 min
/loop 5m /verify                  # Full verify every 5 min
/loop 30m git status              # Commit reminder every 30 min
/loop check the deploy every 2h   # Natural language interval
```

**Disable**: `CLAUDE_CODE_DISABLE_CRON=1`

## Autonomous Triggering (the enforcement ladder is an autonomy ladder)

Manual `/commands` decay: a feature the user must remember to invoke falls out
of use, so something that SHOULD fire never does. Counter it by placing each
trigger as far LEFT (autonomous) as its signal allows:

```
lint > hook > path-scoped rule > agent auto-delegation > skill description > /command
└──────── fires on its own, never forgotten ────────┘        └─ needs Claude ─┘  └ needs user ┘
```

- **Event-driven nudge** → Stop hook. e.g. `check-before-stop.sh` nudges
  `/reflect` only when `session-learnings.log` has captured entries.
- **Prompt-driven surfacing** → UserPromptSubmit hook emitting `additionalContext`.
  e.g. `on-user-prompt.sh` surfaces `/devil-advocate` on critique signals and
  `/design-review` on design-quality questions.
- **Caution**: do NOT auto-surface all skills — every fire costs context and
  alert fatigue trains the user to ignore them. Wire only HIGH-VALUE,
  CLEAR-SIGNAL, LOW-FREQUENCY triggers, with high-precision patterns. When
  adding a skill (see `/manage-spec`), ask: should a clear signal auto-surface
  it, or is `/command` enough?

## Prompt Hooks (Semantic Validation)

Hook type that sends a prompt to Claude for evaluation instead of running bash.

```json
{
  "type": "prompt",
  "prompt": "Does this commit message follow conventional format (<type>: <lowercase desc>)? Types: feat/fix/refactor/test/docs/chore/perf. YES or NO. Message: $ARGUMENTS"
}
```

**When to use**:
- ✅ Semantic validation (commit quality, code change intent)
- ❌ Structural checks (directory names, file paths) — use `command` hooks

## Agent Hooks (Subagent Verification)

Spawn a subagent with Read/Grep/Glob to verify conditions.

```json
{
  "type": "agent",
  "prompt": "Check that all new .go files in the diff have corresponding _test.go files. Report any missing.",
  "model": "claude-haiku-4-5-20251001"
}
```

## Channels (Push Events)

External systems push events into a running Claude Code session. Research preview.

```json
// settings.json (managed settings only)
{ "channelsEnabled": true }
```

**Use cases**: CI pushes test failures, monitoring pushes alerts, chat messages forwarded.
**Status**: Experimental — monitor for stability before integrating into workflow.

## LSP Integration

Language Server Protocol provides real-time code intelligence (go-to-definition, find-references).

**Install via plugins**:
```
/plugin install go-lsp@claude-plugins-official      # gopls
/plugin install typescript@claude-plugins-official   # tsserver
/plugin install rust@claude-plugins-official         # rust-analyzer
/plugin install python@claude-plugins-official       # pyright
/plugin install dart@claude-plugins-official         # dart-analyzer
```

**Or configure manually** (`.lsp.json` in project root):
```json
{
  "go": {
    "command": "gopls",
    "args": ["serve"],
    "extensionToLanguage": { ".go": "go" }
  }
}
```

## Sandbox Settings

Restrict filesystem and network access for security hardening.

```json
// settings.json
{
  "sandbox": {
    "permissions": {
      "disk": {
        "read": ["./", "/usr/local/go/"],
        "write": ["./"]
      },
      "network": {
        "outbound": true,
        "allowedHosts": ["pkg.go.dev", "proxy.golang.org", "github.com"]
      }
    }
  }
}
```

**Trade-off**: Increases security but may block build tools that access unexpected paths. Test thoroughly before enabling in production specs.

## Worktrees (EnterWorktree/ExitWorktree)

Built-in git worktree management. `/batch` uses this automatically.

```json
// settings.json — optimize worktree creation
{
  "worktree.symlinkDirectories": ["node_modules", ".cache", "vendor"],
  "worktree.sparsePaths": ["internal/order", "internal/user"]
}
```

**Manual use**: Say "work in a worktree" and Claude will `EnterWorktree`.

## Plugin Packaging (Personal Distribution)

Package a spec for reuse across your own projects:

```bash
./scripts/package-plugin.sh /path/to/go-spec /path/to/output
# Creates go-spec-plugin/ with plugin.json + skills/ + agents/ + hooks/

# Use in any project:
claude --plugin-dir ./go-spec-plugin

# Or symlink into project:
ln -s /path/to/go-spec-plugin .claude-plugins/go-spec
```

## SubagentStop Hook (Quality Gate)

Ensure agents complete their output before stopping:

```json
"SubagentStop": [{
  "matcher": "go-reviewer|security-reviewer|review-code",
  "hooks": [{
    "type": "command",
    "command": ".claude/hooks/check-agent-output.sh"
  }]
}]
```

## Hook Events Reference (25 Total)

| Event | Phase | Can Block | Purpose |
|-------|-------|-----------|---------|
| SessionStart | startup | yes | Verify toolchain |
| InstructionsLoaded | startup | no | Audit loaded rules |
| UserPromptSubmit | before response | yes | Validate user input |
| PreToolUse | before tool | yes | Block dangerous ops |
| PermissionRequest | before prompt | yes | Auto-approve patterns |
| PostToolUse | after tool | no | Format, validate |
| PostToolUseFailure | after failure | no | Error logging |
| SubagentStart | agent spawn | no | Inject context |
| SubagentStop | agent finish | yes | Quality gate |
| Stop | response end | yes (2=continue) | Verify before done |
| StopFailure | API error | no | Alert on failures |
| TeammateIdle | agent teams | yes | Keep working |
| TaskCompleted | agent teams | yes | Validate completion |
| ConfigChange | mid-session | yes | React to changes |
| WorktreeCreate | worktree | command only | VCS setup |
| WorktreeRemove | worktree | no | Cleanup |
| PreCompact | compaction | no | Prepare |
| PostCompact | compaction | no | Verify |
| SessionEnd | shutdown | no | Cleanup, logging |
| Elicitation | MCP input | yes | Intercept |
| ElicitationResult | MCP result | yes | Modify |
