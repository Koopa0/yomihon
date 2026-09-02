# Authoring contract

How Markdown becomes structure in yomihon. It settles one thing, the
study-path grammar; map and report syntax are not settled here.

Ordinary Markdown needs no markers — any folder of notes reads, links,
searches and renders without one. Structured presentation is opt-in, and
`System/schemas/vault-schema.toml` grants it, owning the legal types, fields,
lifecycle and privacy boundary. Where a document claims a structure and misses
this contract, yomihon keeps the prose, stops the projection it cannot
determine, and reports through `yomihon check`; it never guesses, never
flattens, and never edits the file.

## Study path: sequence is declared, never inferred

Applies only to notes whose type the contract lists under
`[navigation] path_types`. Maps, reports and ordinary notes do not read this
syntax; the same marker in one of them is plain text. That gate is silent and
upstream of everything below: a note yomihon reads as a course answers at
`/syllabus/<its vault-relative path>` and enters the home page's course count.
When it does not answer there, nothing below applies until that is settled.

### The marker

Three values, closed:

    {sequence=primary}   {sequence=local}   {sequence=none}

It declares a branch's role in the course order and nothing else; there is no
other key. It is read at the end of the row's or heading's **own first line**,
trailing whitespace allowed, in exactly two places:

- **a heading, H2 through H6** — declaring the branch that heading opens;
- **a nested group-container line** — a list item carrying the marker with a
  child list beneath it, declaring the child group that list forms. Its parent
  is the enclosing list item structurally, not "the nearest lesson above it".

A list row may run to several lines; a marker written further down is reported
rather than obeyed. Heading and container text take no part in the decision: a
branch titled 「每日練習」 that declares `primary` is primary, and a child
branch does not inherit its parent's role. A recognized marker is stripped
from the displayed name, and the source bytes are untouched.

A declared `{sequence=local}` container is a structural edge: it is not a
continuation of the enclosing entry, not inside that entry's target scope, and
not itself an entry candidate. Its child rows belong to the child group alone.

### The candidate grammar

A candidate is a list row — ordered or unordered, either is the same row —
holding at least one live wikilink in the item's own **target scope**:
everything the item says from its list marker to its end, including
continuation content after a nested list, and excluding every nested-list
subtree. Row punctuation declares nothing; order is source order.

A **live** wikilink addresses another note. An embed (`![[…]]`) shows a note
rather than listing it, a same-file anchor (`[[#…]]`, `[[^…]]`) names no note,
and a link inside code or an Obsidian comment is quoted or switched off. A
second live wikilink anywhere in the target scope makes the row
`path.entry_multi_target`.

A candidate is **canonical** when its target scope holds no second live
wikilink and the first visible inline after the list marker is a live,
non-embed wikilink — bold or italic around it is allowed, nothing else may
come before it. "Visible" is what renders: an Obsidian comment before the link
shows nothing and does not count, while a run of `*` or `_` that does not open
emphasis prints as itself and does. Text after the link is prose, never a
second declaration. A row whose single live link comes after anything else — a
label, an embed, a same-file anchor, inline code — is a candidate that is
never accepted: it reports `path.entry_noncanonical` with its line, and the
author either moves the link to the front or takes the row out of the course.

A task checkbox row (`- [ ]`, `- [x]`) is never a candidate and never an
entry: it tracks whether something was done, a different question from what
the course lists. A side branch hangs from a **lesson**, so every row the
grammar refused — a checkbox, a non-canonical row, a row naming two notes, a
row naming none — anchors nothing, and a `{sequence=local}` container beneath
one reports `path.local_orphan`.

There is no implicit root: a candidate before the first level-2 heading
reports `path.entry_outside_branch`. A nested list whose row declares nothing
keeps its source exactly as written, projects nothing, and is reported;
yomihon never flattens it into the enclosing branch.

### Five branch states

A **direct candidate** is a source row the candidate grammar recognizes; an
**accepted entry** is a candidate that passed canonical validation. Branch
state depends on the first and is settled before canonical validation runs, so
fixing a malformed entry never produces a second round of "this branch has no
role"; counting and navigation depend on the second. Declaring `none` is a
legitimate authored answer and draws no diagnostic, forgetting to declare is
not, and a declared `none` leaves navigation rather than the grammar: a
malformed row underneath one is still reported to its author.

| state | condition | progression | diagnostic |
|---|---|---|---|
| `primary` | declared | main line; consecutive primary groups join end to end in declared order | none |
| `local` | declared | within its own group only | none |
| `none` | declared | none | none |
| `unclassified` | an undeclared nested list, or a heading with a direct candidate and no marker the parser could read | none | yes |
| `structural` | a **heading** with no direct candidate and no marker | not applicable | none |

What the states produce:

- **Home counts `primary` only.** A `local` group carries its own order and
  its own count beside it on the path page. A `none` group leaves path
  navigation, the course count, reverse placement and prev/next entirely, and
  its prose still reads on the note page.
- **`primary` and `local` never link to each other through prev/next.** The
  lesson after the one a side branch hangs from is the next main-line lesson;
  the side branch does not rejoin.
- An **unresolved entry in a primary group still counts** toward the planned
  course total. Openable navigation is the resolved projection of each
  sequence component: it drops the unresolved stop and joins the resolved
  entries on either side of it *within that component*, never across a
  `primary`/`local`/`none` boundary, so a trailing unresolved entry leaves the
  lesson before it with no next lesson.

### Marking a branch as a planned gap

A lesson listed but not yet written reports `map.disk_mismatch`. To mark it
planned instead, put the unwritten lessons under a heading whose own text
contains one of

    缺口   待補   待寫   待整理   待建

alongside the declaration the branch still needs (`## 缺口 {sequence=primary}`),
and every unresolved link from that heading down to the next heading at the
same or a higher level reports as information rather than a warning. The
branch keeps its declared role, the entry still counts toward the planned
total, and the lesson stays unopenable until the note exists. This heading is
the only thing that softens a course's own row: naming a concept as planned
elsewhere in the vault softens an ordinary broken link to it, never a lesson a
course lists.

### A complete example

Eight lessons, a side branch hanging from the third, and a routine block that
stays out of navigation:

```markdown
## 主線 {sequence=primary}

1. [[L01 器材認識]]
2. [[L02 咖啡豆基礎]]
3. [[L03 研磨]]
	- 進階選修(卡住才讀) {sequence=local}
		1. [[磨豆機校正基礎]]
		2. [[粒徑分布判讀]]
		3. [[校正實作]]
4. [[L04 水溫]]
5. [[L05 注水手法]]
6. [[L06 比例與時間]]
7. [[L07 品飲]]
8. [[L08 常見問題排除]] *(尚未撰寫)*

## 日常練習 {sequence=none}

- [[注水練習]]:空壺練 10 分鐘
- [[沖煮記錄]]:每天沖一杯,把參數記下來
```

Home reads `8 課`. The side branch shows as three, hanging under L03. L04
follows L03, because the container is L03's child and L04 is its sibling. L07
has no next lesson, because L08 is unwritten. The routine block is absent from
navigation and reads normally on the page.

### When a document does not meet the contract

| rule | condition |
|---|---|
| `path.role_missing` | a heading has a direct candidate but no declaration, or a nested list carries no declaration at all |
| `path.role_duplicate` | one branch declares two roles |
| `path.role_conflict` | a `primary` or `local` branch sits under a declared `none` (a purely structural branch never triggers it) |
| `path.local_orphan` | a `local` container has no entry to hang from |
| `path.nesting_too_deep` | a `local` container inside a `local` container — rewrite to at most one level while keeping what it hangs from |
| `path.role_on_entry` | a container that is itself a candidate: it cannot be both a lesson and a branch heading |
| `path.role_invalid` | a value outside the three, or no value at all |
| `path.role_misplaced` | a well-formed marker somewhere it cannot be read |
| `path.entry_noncanonical` | a candidate whose single live link is not the first visible inline |
| `path.entry_outside_branch` | a candidate before the first level-2 heading |
| `path.entry_multi_target` | a row whose target scope names more than one note |

The rows are conditions, not a partition: one mistake can meet more than one,
and each is reported — an unreadable marker declares nothing, so it draws
`path.role_missing` too. Every `path.*` finding is a warning carrying its
source line; reading always proceeds, and what waits on the author is the
structured projection.

`path.entry_multi_target` has three honest rewrites and the choice is the
author's: give each parallel member its own row; turn a relation between notes
into an ordinary paragraph; or keep the link-first entry and move its
commentary to a paragraph after the item.
