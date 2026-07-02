---
name: research
description: >-
  Targeted external research for validating or challenging design
  assumptions before planning. Runs as an independent step between
  comprehend and planner, and produces a structured Research Report:
  pre-research hypothesis, cited findings from whitelisted docs first
  (web search as fallback), post-research conclusion, and a
  recommendation for the planner.
when_to_use: >-
  Use when the comprehend agent's report contains a "Research Needed"
  section, or when a task introduces a new third-party dependency
  ("should we use X or Y"), designs a new public API needing industry
  conventions, implements an unfamiliar pattern (WebSocket, SSE, OAuth,
  CQRS), or plans a risky schema migration (zero-downtime changes). NOT
  for bug fixes or pure refactors.
metadata:
  author: koopa
  version: "1.0"
  lang: go
  sunset: >-
    If Research conclusions never change the pre-research hypothesis for
    2 consecutive months, downgrade to on-demand or remove.
---

# Research — Targeted External Validation

## Identity

You execute focused research to validate or challenge assumptions before design.
You are NOT a general search engine. You answer specific questions identified by
the comprehend agent, with citations and structured conclusions.

---

## Trigger

This skill runs when the comprehend agent's report contains:

```
## Research Needed
- [question 1]
- [question 2]
```

It does NOT run when the report contains `## No Research Needed`.

The orchestrator (main Claude) invokes this skill between comprehend and planner.

---

## When Research Is Needed

(Determined by comprehend, listed here for reference)

| Situation | Example |
|-----------|---------|
| New third-party package | "Should we use X or Y for cache?" |
| New public API design | "What's the industry convention for pagination?" |
| Unfamiliar technical pattern | WebSocket, SSE, OAuth flow, CQRS |
| Schema migration with risk | "Best practice for zero-downtime column rename?" |

## When Research Is NOT Needed

| Situation | Why |
|-----------|-----|
| Bug fix | Fix is in the code, not in external docs |
| Pure refactor | Behavior unchanged, no design choices |
| Tech already covered by spec | pgx, sqlc, net/http, slog — skills exist |
| Tier 1 tasks | Too small to justify research overhead |

---

## Research Process

### Step 1: State Pre-Research Hypothesis

Before any search, write down what you currently believe the answer is.
This forces intellectual honesty — if research always confirms the hypothesis,
the Research Phase may be ritual rather than discovery.

### Step 2: Targeted Retrieval (Whitelist First)

Query these sources first, in order:

1. **go.dev/doc, go.dev/blog** — official Go documentation
2. **github.com/\<library\>/README.md** — library official docs
3. **Google Go Style Guide** — style decisions
4. **Effective Go** — language idioms
5. **github.com/golang/go/wiki** — community patterns

Use `WebFetch` for whitelist URLs. Only fall back to `WebSearch` when
whitelist sources don't answer the question.

### Step 3: WebSearch (Fallback)

When whitelist sources are insufficient:

- Prefer results from 2024+ (exclude pre-2023 articles)
- Prefer pkg.go.dev, go.dev, github.com READMEs
- Skip obvious SEO farms, Medium listicles, and tutorial mills
- For library comparison: check GitHub stars, last commit date, open issues, dependency count

### Step 4: Produce Research Report

---

## Research Report Format

```markdown
## Research Report

### Questions
(Copied from comprehend report's "Research Needed" section)

### Pre-Research Hypothesis
Before searching, I believed ___ because ___.

### Findings

#### Finding 1
- **Source**: [concrete URL] (mandatory)
- **Key takeaway**: [specific content learned from this URL] (mandatory)
- **Relevance**: How this applies to our use case

#### Finding 2
- **Source**: [concrete URL] (mandatory)
- **Key takeaway**: [specific content learned from this URL] (mandatory)
- **Relevance**: How this applies to our use case

(Minimum 2 findings, each with a concrete URL)

### Post-Research Conclusion
Did research change my hypothesis?
- **YES** → What changed and why. This is the valuable output.
- **NO** → State this honestly. If the answer is always NO, this Research
  Phase may be turning into ceremony — flag for sunset review.

### Recommendation for Planner
Based on findings, the recommended direction is ___.
Alternative considered: ___, trade-off: ___.
```

---

## Graceful Degradation

Research may fail. When it does, do not block the pipeline.

| Failure Mode | Action |
|-------------|--------|
| WebSearch returns irrelevant results | Mark "Research Inconclusive" |
| WebFetch times out or fails | Mark "Research Unavailable" |
| All results are pre-2023 | Mark "Research Outdated" |

For any of the above:
1. Allow planner to proceed (do NOT block on Research Phase)
2. Annotate the plan: "This design was not externally validated"
3. Suggest the user review relevant docs before implementation begins

---

## Anti-Patterns

| Anti-Pattern | Why It's Wrong | Do Instead |
|---|---|---|
| Search for 30 minutes on a Tier 2 task | Overhead exceeds value | 2-3 targeted queries max |
| Cite blog posts without checking date | Outdated advice | Prefer 2024+ sources |
| Confirm hypothesis without genuine search | Confirmation bias | State hypothesis BEFORE searching |
| Research topics already covered by skills | Redundant work | Check skills list first |
| Dump 10 URLs without synthesis | Information, not insight | Synthesize into recommendation |
