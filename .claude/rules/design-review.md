---
paths:
  - "**/*.go"
---

# Design Review — Beyond Syntax

## Core Principle

Lint catches code that breaks rules. Design review catches code that nobody would write if they understood the problem better.

A file that passes every linter can still be wrong — wrong name, wrong package, wrong abstraction, wrong concept boundary. These errors compound: they make the next reader guess, the next contributor add workarounds, and the codebase drift toward incoherence.

## The Four Questions

Every review of Go code — L1 or L2 — must ask these questions. They are not optional "nice to have" checks. They are the difference between rubber-stamping and reviewing.

### 1. Why does this package exist?

A package that cannot answer this in one sentence is wrong. "It contains order-related stuff" is not an answer — that's a directory, not a package.

A good package name IS the answer: `portfolio` tracks positions and equity. `risk` enforces limits and circuit breakers. `indicator` computes technical signals from price bars.

**Red flags**:
- Package does two unrelated things (calculate AND persist AND notify)
- Package exists because "it was getting too big" (size is not a reason)
- Package name is a noun that doesn't imply an action or responsibility (`data`, `types`, `common`)
- You can't explain what removing this package would break

### 2. What concepts do the type and function names reflect?

Names are the API's documentation. If a reader must open the file to understand what `Process()` does, the name failed.

**Ask**:
- Does `Signal` mean a trading signal, an OS signal, or a notification? In this codebase, is it unambiguous?
- Does `Handle` mean HTTP handler, event handler, or error handler? Context must be obvious from the package.
- Could a new team member guess what `Bar` is without reading the struct definition? (In a trading engine: yes, OHLCV candlestick. In a generic project: no.)
- Do the method names on a type tell you the type's ROLE? (`portfolio.Track()`, `risk.Check()`, `indicator.Compute()`)

**Red flags**:
- Generic verbs: `Process`, `Handle`, `Execute`, `Run`, `Do` — without context these say nothing
- Acronyms without prior definition in the package doc
- Same concept with two names (`Order` in one package, `Trade` in another, for the same thing)
- Type name that describes structure, not concept (`DataMap`, `InfoList`, `ResultSet`)

### 3. How would stdlib organize this?

The standard library is the gold standard for Go package design. When in doubt, look at how stdlib solves similar problems:

| Your problem | Stdlib analog | What to learn |
|---|---|---|
| Multiple exchange adapters | `database/sql` + drivers | Interface in consumer, adapters register themselves |
| Event dispatch | `net/http` handler chain | Small interface, composable middleware |
| Configuration | `flag`, `encoding/json` | Parse into concrete struct, validate after parse |
| Streaming results | `bufio.Scanner`, `iter.Seq` | Pull-based iteration, caller controls lifecycle |
| Connection pooling | `database/sql.DB` | Pool is opaque, caller gets connection via method |

**Ask**: If the Go team added this feature to stdlib, would they organize it the same way? If not, why are you organizing it differently? (There may be a good reason — but you must HAVE the reason.)

### 4. What would confuse a new reader?

Read the code as someone with Go experience but zero context about this project. What makes you stop and re-read?

**Red flags**:
- Function with 5+ parameters (the caller can't remember the order)
- Boolean parameter (`execute(order, true, false)` — true what? false what?)
- Package where you must read 3+ files to understand the happy path
- Import cycle resolved by moving types to a "shared" package (wrong decomposition)
- Comments that explain WHAT the code does (should be obvious from code) instead of WHY

## Operationalizing in Reviews

### For L1 (go-reviewer) — Quick Design Smell Check

After running grep patterns, spend 30 seconds per package asking:
- Can I state this package's purpose in one sentence?
- Are there types whose names I had to look up to understand?
- Is there a function that does something its name doesn't suggest?

Flag as `DESIGN` severity (between IMPORTANT and SUGGESTION). One-liner explanation is enough.

### For L2 (review-code) — Design Intent Lens

Full application of all 4 questions. Read the package as a whole, not file-by-file. The output must include concrete evidence: "package X exists because Y" or "type Z is confusing because its name suggests A but it does B."

### For standalone design review

Use `/design-review` skill for package-by-package deep analysis. This is the comprehend agent's mindset applied post-implementation.

## What This Is NOT

- Not a style guide (naming.md handles that)
- Not a lint rule (tools handle that)
- Not architecture planning (comprehend/planner handle that)
- Not about catching bugs (correctness lens handles that)

This is about catching **conceptual mistakes** — code that works but teaches the wrong mental model.
