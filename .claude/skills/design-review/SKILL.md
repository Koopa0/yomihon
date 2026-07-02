---
name: design-review
description: >-
  Deep design review — package-by-package comprehension of why each package
  exists, what concepts its names model, how stdlib would organize it, and
  what would confuse a new reader. Produces a markdown report with
  severity-ranked findings (optional HTML artifact for complex reviews).
when_to_use: >-
  Use when the user asks for a design review, architecture review, naming
  review, or package organization critique; questions whether the code's
  mental model is right; or wants more than lint/test surface checks. Trigger
  phrases: "design review", "review the design", "does this package structure
  make sense", "is the naming right".
user_invocable: true
---

# Design Review

You are performing a deep design review of the codebase. This is NOT a code review — you are not looking for bugs, lint violations, or missing tests. You are looking at whether the code's **organization and naming reflects the right mental model**.

## Scope

If the user specifies packages or files, review those. Otherwise, review all packages under `internal/`.

```bash
ls internal/
```

## Process: Package by Package

For each package, read ALL `.go` files in it (excluding `_test.go` and `internal/db/`). Then answer these four questions.

### Question 1: Why does this package exist?

State the answer in one sentence. If you can't, that's a finding.

**Good**: "`portfolio` tracks open positions, computes equity and unrealized PnL."
**Bad**: "`utils` contains helper functions used by other packages."

Check:
- Does the package do ONE thing? Or has it accumulated unrelated responsibilities?
- Is there a clear reason this isn't part of an adjacent package?
- Would the codebase break in a specific, nameable way if you deleted this package?

### Question 2: What concepts do the names reflect?

Read the exported types and functions. For each one:
- Does the name tell you WHAT it is and WHAT it does without reading the implementation?
- Is the naming consistent with how the same concept is named elsewhere in the codebase?
- Are there generic names (`Data`, `Info`, `Result`, `Item`, `Entry`, `Record`, `Processor`, `Manager`, `Handler`) that could be replaced with domain-specific names?

```bash
# List all exported types and functions per package
for pkg in internal/*/; do
  echo "=== $(basename $pkg) ==="
  grep -n "^type [A-Z]" "$pkg"*.go 2>/dev/null | grep -v _test.go
  grep -n "^func [A-Z]\|^func (.*) [A-Z]" "$pkg"*.go 2>/dev/null | grep -v _test.go
  echo
done
```

### Question 3: How would stdlib organize this?

For each package, identify the closest stdlib analog:

| Pattern | Stdlib analog | What to check |
|---|---|---|
| Multiple backends/adapters | `database/sql`, `hash` | Interface in consumer, adapter registers itself |
| Request→Response processing | `net/http` | Handler interface, middleware composition |
| Stream of items | `bufio.Scanner`, `iter.Seq` | Pull-based, caller controls lifecycle |
| Configuration | `flag`, `encoding/json` | Parse into struct, validate after, no global state |
| Pool of resources | `database/sql.DB`, `sync.Pool` | Opaque pool, get/release via methods |
| Registry of named things | `image`, `encoding` | `Register()` at init, `Lookup()` at runtime |

If the code diverges from the stdlib pattern, is there a GOOD reason? Document it.

### Question 4: What would confuse a new reader?

Read the package as someone with 5 years of Go experience but zero context about this project.

- Can you follow the happy path (main use case) by reading ONE file?
- Are there types you had to look up in another file/package to understand?
- Are there unexported functions that do heavy lifting but have no doc comment explaining WHY?
- Are there boolean or string parameters where the call site is unreadable without checking the signature?
- Is the package's `doc.go` or main file's package comment missing or misleading?

## Output Contract

The report body is **always Markdown**. HTML is an optional supplementary artifact, never the contract.

### Markdown summary (always required, top of report)

Open every design-review report with this concise summary block:

```markdown
## Summary
[one-paragraph design health overview]

## Key findings
- [finding]

## Recommended changes
- [change]

## Open questions
- [question]

## HTML artifact
[path if generated, or "none"]
```

Then follow with the per-package detail described in **Markdown Report Format** below.

### HTML artifact (optional, complex reviews only)

For reviews where visual structure adds genuine value (multi-package dependency maps, collapsible per-package sections, glossary, annotated code) you MAY additionally produce a self-contained HTML file at:

```
tmp/design-review-<topic>-<YYYY-MM-DD>.html
```

Use SVG diagrams, color-coded severity, collapsible sections, hover-linked glossary. Inline all CSS/JS so the file opens standalone in a browser.

The Markdown summary always remains the source of truth. If HTML disagrees with Markdown, Markdown wins. Reference the HTML path in the Markdown summary's "HTML artifact" field.

### Guardrails (scope of this skill)

- HTML MUST NOT become the only output. The Markdown summary is the contract.
- Do NOT change planner, execute-plan, review-code, or SubagentStop report formats — those are parsed by other skills/hooks that expect Markdown.
- Do NOT create a generic HTML-output skill or add a global rule telling Claude to prefer HTML. This dual-format choice is scoped to design-review only.
- Do NOT propagate this pattern into `.claude/rules/`.

## Markdown Report Format

```markdown
## Design Review: `<package>`

**Purpose**: [one sentence — or "UNCLEAR" with explanation]
**Stdlib analog**: [closest match]
**Files**: [count]

### Findings

| # | Question | Finding | Severity |
|---|----------|---------|----------|
| 1 | Q1: Purpose | [finding] | HIGH/MEDIUM/LOW |
| 2 | Q2: Names | [finding] | HIGH/MEDIUM/LOW |

### What's Good

- [specific thing this package does well — don't skip this]

---
```

Repeat for each package, then end with:

```markdown
## Cross-Package Findings

- [concept inconsistency across packages]
- [dependency direction that doesn't make sense]
- [responsibility that's split across two packages but shouldn't be]

## Summary

| Metric | Value |
|---|---|
| Packages reviewed | N |
| HIGH findings | N |
| MEDIUM findings | N |
| Packages with unclear purpose | N |
| Cross-package inconsistencies | N |
```

## Severity

| Level | Meaning |
|---|---|
| **HIGH** | Package has no clear single responsibility, OR a type name actively misleads, OR same concept has conflicting names across packages |
| **MEDIUM** | Generic name that could be more specific, minor confusion for new reader, slight stdlib divergence without documented reason |
| **LOW** | Nitpick — name could be marginally better, doc comment missing but name is self-explanatory |

## Rules

- NEVER suggest code changes — this is a design assessment, not a refactor
- NEVER flag lint/style issues — that's go-reviewer's job
- ALWAYS state what's GOOD about each package before findings
- ALWAYS provide the stdlib analog, even if the code matches it perfectly
- If a package is well-designed, say so. Don't manufacture problems.
- Read FULL files. Don't skim exports and guess.
