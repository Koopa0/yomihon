# yomihon — agent entry point

Authority: `ENGINEERING_STANDARD.md` version 2.0 and `PROJECT_PROFILE.md`.
Product walls and repo-specific facts: `CLAUDE.md` — read it first.
Canonical complete automated verification: `make verify`.

The tracked English `ENGINEERING_STANDARD.md` is the repository's only
normative engineering-standard body. `PROJECT_PROFILE.md` resolves its
applicability for yomihon. `docs/standards.md` and an optional maintainer-local
go-spec checkout may add stricter procedures, but neither may weaken the
tracked standard or become a clean-clone prerequisite.

Before touching renderer / graph / search, `docs/vault-model.md` is required reading.

## Semantic and acceptance contract

Treat semantic correctness as the primary gate. Before non-trivial work,
identify the user need and non-goals; concept and non-concept; semantic owner
and authority; source of truth, capability, projection, cache, transport, and
diagnostics; legal and impossible states; failure ownership, partial-result
validity, retry safety, and recovery; final irreversible boundary; concurrency
timeline; and load-bearing invariant with a watched-red mutation.

If two reasonable product behaviors remain, mark `NEEDS-KOOPA`. Continue only
reversible investigation; do not choose a disputed behavior because it is
easier to implement, store, mock, or test.

Use the authority order in `ENGINEERING_STANDARD.md` §3. In particular, Go
code follows the selected Go specification and memory model, the standard
library, repository decisions, Effective Go, Go Code Review Comments, and
Google Go Style in that order. Tools and reference projects inform review but
do not outrank product authority.

Every formal review reports three separate gates:

1. architecture and open-source engineering quality;
2. real-user and independent-agent usability through supported public surfaces;
3. test and evidence-system quality, including watched-red proof.

Formal verdicts bind to an immutable commit or artifact digest. `UNVERIFIED`,
`BLOCKED`, `UNRESOLVED`, or an unapproved exception is never PASS. Merge-ready,
release-ready, and production-ready are distinct claims as defined by
`PROJECT_PROFILE.md`; a moving worktree can satisfy none of them.

# Codex operating contract

- Maintainers with the optional go-spec harness read
  `.agents/skills/using-go-spec/SKILL.md` before implementation and
  `.agents/skills/self-review/SKILL.md` before completion and after each review
  repair. A clean clone instead follows this file, the normative standard, the
  profile, and tracked canon; absent local skills never excuse a missing gate.
- Local `.claude/rules/` and `.codex/hooks.json` are mechanical accelerators,
  not repository evidence. No formal claim may depend on an ignored hook or
  instruction file.
- Do not auto-delegate. Use subagents only when the user or the active instructions explicitly request delegation or parallel agent work.

## Repository commands and traps

- Run `make verify`; any mandatory stage failure or drift is non-zero. Exact
  conditional and release-only commands live in `PROJECT_PROFILE.md` §13–§14.
- Do not edit generated `*_templ.go`, sqlc catalog files, or
  `assets/css/output.css`; edit their sources and regenerate.
- JSON/JSONL goldens, exit codes, field order, reason strings, vault dialect
  fixtures, and semantic-generation formats are compatibility contracts, not
  convenient expected output.
- The final side-effect owners are narrow: `internal/status` for the one status
  write and git commit; `internal/schema` for the vault contract;
  `internal/search/semantic` for approved provider egress and the disposable
  generation store. Revalidate authority, freshness, privacy, and consent at
  the final send, write, or publication boundary.
- Never commit vault content, credentials, semantic generations, query-bearing
  logs, environment dumps, or review artifacts containing private paths.
- Never weaken lint, parsing, validation, privacy, or a test oracle to obtain a
  green result. A check not executed successfully on the named snapshot is not
  PASS.

# Web Platform Guidance

For all frontend work, follow the current official guidance from:

- Chrome Modern Web Guidance
- web.dev Baseline
- web.dev Learn HTML, CSS, JavaScript
- web.dev Learn Accessibility
- web.dev Core Web Vitals / Learn Performance
- web.dev Security and Privacy guidance
- developer.chrome.com CSS/UI docs for specific modern platform APIs

## Baseline target

This project's Web Platform Baseline target is Baseline 2026.

Prefer Baseline Widely available features for core UX.
Baseline Newly available features are allowed with progressive enhancement.
Limited availability features require explicit justification, feature detection, and fallback.

## Implementation principles

Prefer native Web Platform features over JavaScript-heavy abstractions:

- semantic HTML before ARIA
- native form controls before custom controls
- CSS layout before JS layout
- CSS transitions/view transitions before JS animation loops
- dialog/popover/details/summary when appropriate
- progressive enhancement over hard browser assumptions

## Accessibility

Every UI change must preserve:

- semantic structure
- keyboard navigation
- visible focus states
- correct labels and accessible names
- reduced-motion support where motion is introduced
- no color-only state communication

## Performance

Every UI change must consider:

- LCP
- CLS
- INP
- JS bundle impact
- image/font loading
- unnecessary hydration or client-side JS

Do not add a dependency for behavior already available in the native platform unless there is a documented reason.

## Security and privacy

For user-generated or dynamic content:

- avoid unsafe HTML injection
- sanitize untrusted input
- prefer strict CSP patterns
- minimize collected data
- avoid unnecessary third-party scripts
