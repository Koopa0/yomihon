# Authoring contract

How Markdown becomes structure in yomihon.

## Status of this document

| | |
|---|---|
| The sequence contract below | **decided** by the vault owner |
| Candidate rows, task checkboxes, undeclared nesting | **decided** by the vault owner |
| The parser that reads it | **implemented** |
| Map and report syntax | **out of scope here** |

A study path is read through this grammar: it decides what the course lists,
what Home counts, where a side branch hangs, what prev and next offer, and
which notes a course places. Where a document does not meet the contract, the
prose still reads and the projection stops; the diagnostics below reach the
author through `yomihon check`.

Maps and reports are unaffected — they do not read this syntax, and the same
marker in one of them is plain text.

## Who owns what

The vault owns the truth about its content. Yomihon owns the contract that
says how those bytes become a course, a path, a map, a reader or a report on
screen. An author who wants that structured presentation writes to yomihon's
contract.

1. Markdown and Obsidian own the literal meaning of the text — what a
   wikilink, an embed, a code fence, a comment and a tag actually do.
   Yomihon does not redefine those; they are external facts.
2. `System/schemas/vault-schema.toml` owns the legal types and fields, the
   lifecycle, the privacy boundary, and which document types may take part
   in path and map behaviour at all.
3. This document owns how the bytes of an opted-in document project into a
   yomihon surface.
4. Templates and skills teach humans and agents to write to this contract.
   They never hold a second schema, a second enum, or a second set of rules.
5. Existing notes are adjustable content, not the design authority for a
   parser. Their shapes are a migration plan; they are never an argument for
   or against a rule, and no special case is added so that an existing file
   can stay untouched.
6. Where a document does not meet the contract, yomihon keeps the prose,
   stops the projection it cannot determine, and tells the author. It never
   guesses quietly, never flattens, and never edits the file.

## Adoption

Nothing here is a migration requirement.

- **Ordinary Markdown needs no changes.** Any folder of notes reads, links,
  searches and renders without a single marker.
- **Structured presentation is opt-in.** A note takes part in path or map
  behaviour only if its type is listed for that capability in the vault
  contract; everything else is prose.
- **Yomihon accepts all content and does not claim to understand everyone's
  organising sense.** What it will not do is infer your structure from
  heading wording, list punctuation, or indentation alone.
- **A large existing vault can start with one path.** Declare a single
  course, point it at the notes you already have, and leave the other
  thousand files alone. There is no sweep to perform.

## Study path: sequence is declared, never inferred

Applies only to notes whose type the vault contract lists under
`[navigation] path_types`. Maps, reports and ordinary notes do not read this
syntax; the same marker in one of those is plain text.

That gate is silent, and it is upstream of everything on this page: a document
whose type is not listed reads as ordinary prose no matter how well its body
is written, and nothing reports the omission, because nothing was claimed. A
note yomihon reads as a course answers at `/syllabus/<its vault-relative
path>` and enters the course count on the home page. A 404 there means the
type is not one the contract lists, and every marker below is plain text.
Check that before reading further, because a page that cannot answer 404 or
200 for a document has not been told whether the rest of this applies.

### The marker

Three values, closed:

```
{sequence=primary}
{sequence=local}
{sequence=none}
```

It declares a branch's role in the course order and nothing else. There is no
other key and no room to grow into a general directive syntax.

### Where it may appear

At end of the row's or heading's **own first line**, trailing whitespace
allowed, in exactly two places. A list row may run to several lines; a marker
written further down is read by nobody and is reported rather than obeyed.

- **a heading line, H2 through H6** — declaring the branch that heading opens;
- **a nested group-container line** — a list item that carries the marker and
  has a child list beneath it, declaring the child group that list forms. Its
  parent is the enclosing list item structurally, not "the nearest lesson
  above it".

Heading text and container text take no part in the decision. A branch titled
「每日練習」 that declares `primary` is primary. A child branch does not
inherit its parent's role.

### A local container is a boundary

A valid `{sequence=local}` nested group-container marks a structural edge:

- the container and its child-list subtree are **not** a continuation of the
  enclosing entry and **not** inside its target scope;
- the container is **not** itself an entry candidate;
- the child rows belong to the child group **alone**.

Nesting that carries no valid declaration is settled below, under
"Undeclared nesting"; this boundary rule is about declared containers only.

### The candidate grammar

A candidate is a list row — ordered or unordered, either is the same row —
holding at least one live wikilink in the item's own target scope. Row
punctuation declares nothing; order is source order and nothing else.

A **live** wikilink addresses another note. An embed (`![[…]]`) shows a
note rather than listing it, a same-file anchor (`[[#…]]`, `[[^…]]`) names
no note, and a link inside code or an Obsidian comment is quoted or
switched off. None of those is live.

An item's **target scope** is its own text: everything the item says from
its list marker to its end, including continuation content that follows a
nested list, and excluding every nested list subtree — a declared local
container's subtree by the boundary rule above, an undeclared one by the
nesting rule below. A second live wikilink anywhere in that scope makes
the row `path.entry_multi_target`.

A candidate is **canonical** when both hold:

- the first visible inline after the list marker is a live, non-embed
  wikilink — it may be wrapped in bold or italic, and nothing else may
  come before it. "Visible" is what renders: an Obsidian comment before the
  link shows nothing and does not count, while a run of `*` or `_` that does
  not open emphasis prints as itself and does;
- the item's whole target scope holds no second live wikilink.

A canonical candidate is an entry. Text after the link is read as prose,
never as a second declaration: a link-first action sentence is an entry
and gains no further guessed meaning. A row whose single live link comes
after anything else — a label, an embed, a same-file anchor, inline code —
is still a candidate, but it is never accepted: it reports
`path.entry_noncanonical` with its line, and the author either moves the
link to the front or takes the row out of the course.

### Task checkboxes

A task checkbox row (`- [ ]`, `- [x]`) is never a candidate and never an
entry: it tracks whether something was done, which is a different question
from what the course lists. It also cannot anchor a side branch — a local
container nested under a checkbox row has nothing to hang from and
reports `path.local_orphan`.

A side branch hangs from a **lesson**, so the same is true of every row the
grammar refused: a non-canonical row, a row naming two notes, and a row naming
none anchor nothing. A `{sequence=local}` container beneath one of them reports
`path.local_orphan` and projects nothing.

### Before the first branch

There is no implicit root. A candidate before the first level-2 heading
belongs to no part of the course and reports `path.entry_outside_branch`;
the course starts where its first branch is declared.

### Undeclared nesting

A nested list whose row declares nothing keeps its source exactly as
written, projects nothing, and is reported. Yomihon never flattens it into
the enclosing branch and never guesses what it was meant to be: the author
declares it, or it stays prose.

### Two words that must not be confused

- a **direct candidate** is a source row the candidate grammar recognizes;
- an **accepted entry** is a candidate that passed canonical validation.

Branch state depends on the first. Counting and navigation depend on the
second. The order is fixed:

1. parse headings, containers and role declarations;
2. content under a declared `none` is body-only for navigation — only an
   explicitly declared child role is checked for conflict beneath it;
3. run the candidate grammar over every group, `none` included — a declared
   `none` settles the branch's role and takes it out of navigation, not out of
   the grammar, so a malformed row there is still reported to its author;
4. a candidate becomes an accepted entry only after canonical validation;
5. an undeclared nested list is `unclassified`, whatever it holds — nesting is
   itself a claim about structure, and one nobody explained is reported;
6. otherwise, no declaration and at least one direct candidate →
   `unclassified`;
7. otherwise, no declaration and no direct candidate → `structural`;
8. course counts, reverse placement and prev/next read accepted entries only.

Fixing a malformed entry therefore never produces a second round of "this
branch has no role": the branch state was settled at step 5, 6 or 7.

### Five branch states

Declaring `none` is a legitimate authored answer and draws no diagnostic.
Forgetting to declare is not the same thing. A heading that merely groups
other headings is a third case again.

The quiet case is a **heading** only. A heading that lists nothing has said
everything it needs to: it holds other headings, and that is visible from the
document. A nested list is not that case — nesting one list under a row is
itself a claim about structure, and a claim nobody explained is reported
whether or not the rows beneath it name any notes.

| state | condition | progression | diagnostic |
|---|---|---|---|
| `primary` | declared | main line; consecutive primary groups join end to end in declared order | none |
| `local` | declared | within its own group only | none |
| `none` | declared | none | none |
| `unclassified` | an undeclared nested list, or a heading with a direct candidate and no marker the parser could read | none | yes |
| `structural` | a **heading** with no direct candidate and no marker | not applicable | none |

### What the states produce

- **Home counts `primary` only.**
- A **`local`** group carries its own order and shows its own count beside it
  on the path page.
- **`primary` and `local` never link to each other through prev/next.** The
  lesson after the one a side branch hangs from is the next main-line lesson;
  the side branch does not rejoin.
- A **`none`** group leaves the path navigation, the course count, reverse
  placement and prev/next entirely. Its prose still reads normally on the
  note page.
- An **unresolved entry in a primary group still counts** toward the planned
  course total: a lesson that is planned but unwritten is still one of the
  course's lessons. Openable navigation is the resolved projection of each
  sequence component — it drops the unresolved stop and joins the resolved
  entries on either side of it *within that component*, never across a
  `primary`/`local`/`none` boundary. A trailing unresolved entry therefore
  leaves the lesson before it with no next lesson.
- A recognized `{sequence=…}` is **stripped from the name yomihon displays**
  for a group. The source bytes are untouched.

### Marking a branch as a planned gap

A lesson that is listed but not yet written is reported: `map.disk_mismatch`
says the course links a note that resolves to nothing, and suggests creating
the note, fixing the entry, or marking it a planned gap. This is how the last
one is written.

Put the unwritten lessons under a heading whose own text contains one of

    缺口   待補   待寫   待整理   待建

and every unresolved link from that heading down to the next heading at the
same or a higher level reports as information rather than a warning. The mark
is part of the heading's text and sits alongside the declaration, which the
branch still needs:

```markdown
## 缺口 {sequence=primary}

- [[尚未寫的一課]]
```

The mark changes how loud the report is, and nothing else. The branch keeps
its declared role, the entry still counts toward the planned course total, and
the lesson stays unopenable until the note exists — an author who plans a
lesson and writes it later has nothing to undo.

The heading is the only thing that softens a course's own row. Naming a
concept as planned elsewhere in the vault softens an ordinary broken link to
it, but not a lesson a course lists: what a course promises is answered where
the course says it.

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
follows L03, because the container is L03's child and L04 is its sibling.
L07 has no next lesson, because L08 is unwritten. The routine block is
absent from navigation and reads normally on the page.

### When a document does not meet the contract

| rule | condition |
|---|---|
| `path.role_missing` | a heading has a direct candidate but no declaration, or a nested list carries no declaration at all — nesting is itself a claim about structure, so it is reported whether or not the rows beneath it name any notes |
| `path.role_duplicate` | one branch declares two roles |
| `path.role_conflict` | a `primary` or `local` branch sits under a declared `none` (a purely structural branch never triggers it) |
| `path.local_orphan` | a `local` container has no entry to hang from |
| `path.nesting_too_deep` | a `local` container inside a `local` container — rewrite to at most one level while keeping what it hangs from |
| `path.role_on_entry` | a container that is itself a candidate: it cannot be both a lesson and a branch heading |
| `path.role_invalid` | a value outside the three, or no value at all |
| `path.role_misplaced` | a well-formed marker somewhere it cannot be read |
| `path.entry_noncanonical` | a candidate whose single live link is not the first visible inline — the row stays a row, and no entry is accepted from it |
| `path.entry_outside_branch` | a candidate before the first level-2 heading — there is no implicit root for it to join |
| `path.entry_multi_target` | a row whose target scope names more than one note |

`path.entry_multi_target` has three honest rewrites, and which one is right
is the author's call, not the parser's:

- parallel members — give each lesson its own row;
- a relation between notes — turn the row into an ordinary paragraph;
- one entry with commentary — keep the link-first entry and move the
  commentary to an ordinary paragraph after the item.

The rows above are conditions, not a partition: one mistake can meet more than
one, and each is reported. A marker the parser cannot read is the common case —
it declares nothing, so the branch is still undeclared and draws
`path.role_missing` alongside the rule naming what is wrong with the marker
itself. Fixing the marker settles both.

Every one carries the source line and is addressed to the author: these reach
the author through the judge and never enter a reader's page. Every `path.*`
finding is a warning in its first version — reading always proceeds; what
waits on the author is the structured projection. A malformed or misplaced
declaration never fails silently. Yomihon does not guess, does not flatten,
and does not edit the file.

## Not decided

None of the following is settled by anything above, and none of it may be
inferred from the rules on this page:

- the map grammar;
- the report format.
