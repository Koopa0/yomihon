# Claude Design Brief — kurodo

> Purpose: a self-contained brief Koopa brings to Claude for a design pass. The deliverables the designer returns are listed at the end.
> Product context, in one line: a single-user, local, never-public reading and adjudication workbench for a knowledge vault (Go + templ, server-rendered).

## Product and users

kurodo is where an engineer reads their own vault every day: long technical pieces (zh-TW), Japanese lessons (with heavy `<ruby>` annotations), concept notes, syllabi, and system reports. A single user, desktop-first, with reading sessions from 20 minutes to 2 hours. The core action: **finish a piece, then certify it in place with a key press (a status transition)**. Koopa will describe the tonal direction in person — he has a precise aesthetic vocabulary, so talk to him directly rather than presuming it for him.

## Pages (three key pages)

1. **Reading page** (the most important): a three-column docs shell — a left sidebar (lifecycle folder tree + syllabus tree + reports area, collapsible); a center prose content column; a right column = TOC + frontmatter/status panel + diagnostics rail.
2. **Syllabus page**: a tree navigation of one study path (part → module → lesson), showing each lesson's status.
3. **Search panel**: an overlay invoked with ⌘K (an input box + a results list + type/status filter chips).

## Component list

- prose typography: H1–H3, paragraphs, lists, GFM tables, code blocks (server-side highlighting), blockquotes
- callouts in two buckets: note (tip/question/example) and warning (caution/warning), each with a title row + a collapsible variant (native `<details>`)
- status panel: a current-status badge + a set of transition buttons; **`ready` is the only primary button** — it should carry a sense of ceremony (the feel of pressing to adjudicate) without being ostentatious
- diagnostics rail: warning cards for bad YAML / broken links / name collisions (amber palette), read-only with no repair actions
- corrections ledger: how a lesson's corrections record is presented (a three-part claim → fix → source layout)
- badges: status (9 values), type, domain
- header: the wordmark (kurodo), search trigger, theme toggle

## Typography hard requirements (CJK is a first-class citizen)

- body text is mostly zh-TW, interleaved with Japanese lesson text and English terms; line-height ≥ 1.8
- **ruby (furigana) spec**: `<ruby>漢字<rt>かな</rt></ruby>` appears heavily; rt font-size around 0.5em; **rt is shown and hidden by toggling `visibility`, and hiding it must not change line-height or line width (zero reflow)** — this is a requirement an existing product paid for in blood
- katakana words always carry ruby and never participate in the show/hide toggle
- the Latin typeface is settled: Geist (the variable woff2 is already licensed); **please propose CJK (Chinese/Japanese) typefaces** (with a fallback stack; prefer ones that work offline on the local machine)
- dark mode: a class strategy (`.dark`), with light and dark color tokens for every component

## Technical constraints

- **deliverables target Tailwind CSS v4**: `@theme` design tokens (colors, a type-size scale, line-heights, border-radii, shadows) + HTML mocks of 2–3 key pages (written directly in Tailwind classes is best; high-resolution images are also acceptable)
- no JS framework: interactive components must look presentable using native `<details>` and `<dialog>`
- color contrast passes WCAG AA (both body text and badges)
- the skeleton may reference the information architecture of the Tailwind Plus "Syntax" docs template, but the visual identity must be its own — it must not read like a documentation-site template

## Deliverables

1. `@theme` tokens (one CSS file)
2. mocks of the reading page + syllabus page + search panel (HTML with Tailwind classes, or high-resolution images)
3. close-up specs for three components: ruby, callout, and the status panel
