# Spec Propagation Contract

go-spec is **copied** (no symlink/submodule) into a fleet of consumer repos that
drift independently. This contract declares the **binary partition** the
`scripts/propagate-spec.sh` tool enforces: a path is either **SYSTEM** (harness-owned,
syncable from go-spec) or **USER** (local, never written by a sync). Nothing is both.
Default-deny: a path not listed SYSTEM is treated USER.

Derived from the 2026-06-17 fleet drift audit (koopa0.dev, tw-stock-trader,
resonance/backend, learning). See memory `harness-validation-gap-2026-06`.

## SYSTEM — harness-owned, sync-eligible

- `.claude/hooks/**` — the hook mechanism layer (parse-hook-input, check-*, format-go, session-start, on-*, verify-commit-message, validate-planner-output). **Excludes** consumer `*.py` hooks (product review-OS).
- `.claude/rules/**` — universal rules (testing, interfaces, package-organization, development-lifecycle, agents, git-workflow, go-version, …). **Body only** where a per-repo waiver line is embedded (see Waivers).
- `.claude/skills/**` — generic skills. **Excludes** product skills (see USER).
- `.claude/agents/**` — the 11 agent definitions.
- `.claude/workflows/**` — saved workflow scripts (skill-eval, …).
- `tests/**` + the `verify-spec` make target — the validation harness.
- `.claude/QUICKSTART.md` — verify-not-clobber where a consumer tuned it.

## USER — local, NEVER overwritten

- `**/CLAUDE.md` — product governance/memory. (Only the generic reference *tables* may be refreshed surgically, by hand.)
- `.claude/agent-memory/**`, `.claude/plans/**`, `.claude/session-learnings.log`, `.claude/build-log.md` — per-repo learned/append-only state.
- `.claude/settings.local.json` + the `permissions`/`env` block of `settings.json` — consumer-local. (Hook *wiring* entries are SYSTEM, but merged structurally, never by file replace.)
- Product MCP layer: `.claude/skills/koopa0-dev/`, `.claude/skills/build-log/`, `.claude/commands/build-log.md`, `.claude/rules/mcp-decision-policy.md`, `internal/mcp/`, `.mcp.json`.
- Adversarial-Review OS (koopa0.dev): `.claude/skills/{review-harness,adversarial-review,go-compliance-test}/`, `.claude/rules/adversarial-review.md`, `.claude/hooks/*.py`.
- Everything outside `.claude/` and `tests/` — the product (`internal/`, `frontend/`, `docs/`, domain files).
- Cross-language sibling specs' language layers — never cross-propagate.

## Waivers — generic-looking files carrying a per-repo local edit

A SYSTEM file that carries one of these markers is downgraded to **manual-merge** —
the tool refuses to overwrite it, because a blind push would destroy the local edit:

| Marker | Where seen | Meaning |
|---|---|---|
| `SA1019` (disabled) | go-version.md (koopa0.dev, resonance, learning) | deliberate brownfield deprecation waiver |
| `/frontend/` exemption | check-anti-patterns.sh (koopa0.dev) | Angular layout early-exit |
| `save_session_note` / `note_type=context` | check-before-stop.sh (koopa0.dev) | MCP context-bridge reminder tail |
| `/build-log` | development-lifecycle.md (resonance) | product build-log workflow line |

## Apply discipline

- **Dry-run is the default.** `--apply` is required to write, and even then **only files MISSING from the consumer auto-apply** (unambiguous GOSPEC-AHEAD additions). Any file that *differs* — or carries a waiver — halts for human merge. Same philosophy as `/reflect`: mechanical detection, human promotion.
- Refuses to run on a dirty consumer tree; every `--apply` is one revertable checkpoint commit.
- Long-term: extract a shared `spec-core` so the `*-spec` family stops re-forking the universal layer. Sequence: stabilize this tool first, extract `spec-core` second — never both at once.
