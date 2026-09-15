# Turning a list of notes into a course

**The question this answers:** how does a list of links become a course with
counts, a main line and prev/next — and why is mine projecting nothing?

One tangle of words first, because it is unavoidable and nothing else
reconciles it. The thing is a **study path**: that is the note's `type`, and
`path_types` declares it. The page that renders it is served under
`/syllabus/`, its rules are named `path.*`, and the prose here calls the result
a **course**. Confusingly, two rules named `map.*` — `map.disk_mismatch` and
`map.disk_unlisted` — also judge study paths rather than maps; they are about
the course's relationship with the files on disk. One thing, four names.

The principle under all of it: **sequence is declared, never inferred.** yomihon
reads no ordering out of heading wording, list punctuation or indentation. When
it cannot determine a projection it keeps the prose, stops, and reports.
Nothing is flattened as a guess.

## Before the grammar: the two silent gates

The entry point states them — the note's type has to be listed under
`[navigation] path_types`, and no other kind of note reads this syntax. Two
things about them are worth seeing concretely, because they decide what you
observe when a course does not appear.

**The failure is a 404, not a bad page.** A note whose type is not listed
answers nothing at `/syllabus/<its path>`. Omit the `[navigation]` table
altogether and the whole vault has no study paths and no maps — the server
reports `paths=0 maps=0` at startup and `check` says nothing at all, because
none of this is a fault in any note.

**"Does not read it" does not mean "shows it".** Put `## A part
{sequence=primary}` in a map and the page renders the heading *A part*. The
marker is stripped from any heading or list row in any note, so it has not
become plain text — it has vanished, which is a quieter failure than being
visible would be, and it is why a marker in the wrong kind of note leaves no
trace to notice.

## The marker

Three values, closed and case-sensitive:

```
{sequence=primary}   {sequence=local}   {sequence=none}
```

There is no other key. It is read at the end of the row's or heading's **own
first line**, trailing whitespace allowed, in exactly two places:

- **a heading, H2 through H6** — declaring the branch that heading opens;
- **a list row that has a child list beneath it** — declaring the child group
  that list forms. Its parent is the enclosing list item structurally, not the
  nearest lesson above it.

Written anywhere else in a study path — a paragraph, a blockquote, a table, a
row's continuation paragraph — it is reported `path.role_misplaced` rather than
ignored. To write *about* the syntax inside a vault note, put it in a code
span, a fence, or `%%…%%`, where it is quoted rather than written.

The exact form matters, and the failures differ:

| Written | Outcome |
|---|---|
| `{sequence=primary}` | the marker |
| `{sequence= primary }` | the marker — space around the value is fine |
| `{sequence=primary}` + trailing spaces or a tab | the marker |
| `{sequence=Primary}` | not a value: `path.role_invalid`, and the branch is then undeclared, so `path.role_missing` too |
| `{ sequence=primary}` | not a marker at all — `path.role_missing` only, with no hint that you nearly wrote one |
| `{sequence=primary} {sequence=local}` | `path.role_duplicate` |
| the marker on the line *below* the heading | `path.role_misplaced` |
| `# Heading {sequence=primary}` | H1 opens no branch: `path.role_misplaced`, and the marker stays visible in the title |

A recognised marker is stripped from the displayed name and the source bytes
are untouched. A child branch does not inherit its parent's role.

Indentation for a nested branch may be tabs or spaces; two spaces and four both
nest as you would expect.

## Which rows become lessons

A row is a lesson when the first visible thing after its list marker is one
`[[link]]` — bold or italic around it is fine — and the row names no second
note outside its nested lists. Commentary goes **after** the link. A second
link in a paragraph below the row, still inside the same list item, counts as a
second target and refuses the row.

Ordered and unordered rows are the same row, and order is source order.

Never a lesson, and silently so: a checkbox row, an embed `![[…]]`, a bare
same-file anchor `[[#Section]]`, and any link inside code or `%%…%%`. Note that
`[[Note#Heading]]` and `[[Note#^id]]` **do** count — they resolve to the note.

A lesson row that appears before any heading has opened a branch belongs to no
part of the course: `path.entry_outside_branch`. Any heading from H2 to H6
opens a branch, so a path built entirely out of H3s is perfectly valid — it is
the first branch-opening heading that matters, not the first H2.

## The five branch states

| State | Condition | Progression | Reported |
|---|---|---|---|
| `primary` | declared | main line; consecutive primary groups join end to end in declared order | no |
| `local` | declared | within its own group only | no |
| `none` | declared | none | no |
| unclassified | an undeclared nested list, or a heading that lists a lesson row and carries no readable marker | none | yes — `path.role_missing` |
| structural | a heading that lists no lesson row and carries no marker | not applicable | no |

Declaring `none` is a legitimate authored answer and draws nothing; forgetting
to declare is not.

Under a branch declared `none`, an undeclared child is quiet rather than
reported — and a child that declares itself `primary` or `local` there is
`path.role_conflict`, because it cannot take part in a course its parent left.

## What the states produce

- **The course count takes `primary` only.** Every place the extent is shown —
  the paths index, the reading rail, the syllabus header — reads that one
  number.
- **A `local` group carries its own order and its own count beside it.** It
  numbers from one.
- **A `none` group leaves the course entirely** — the count, prev/next, and the
  note's own sense of which course it belongs to. Its prose still reads on the
  note page.
- **A child group rolls into its parent's count when both are `primary` or
  structural, and only then.** A `local` child never does — it shows its own
  number instead, because folding a side branch into the total would print a
  figure no walk matches.
- **`primary` and `local` never link to each other through prev/next.** The
  lesson after the one a side branch hangs from is the next main-line lesson;
  the side branch does not rejoin.
- **An unresolved entry in a primary group still counts** toward the planned
  total, so a course can promise eight lessons while seven exist. Prev/next
  simply steps over it and joins the resolved entries on either side. That
  joining never crosses from one role to another, so a trailing unresolved entry
  leaves the lesson before it with no next lesson at all.

## Nesting, and the shape the check will not catch

A `primary` branch may nest under another `primary`. A `primary` inside a
`local`, and a `local` inside a `local`, have no place in the main line's
order.

Those last two are reported — `path.role_nested_primary` and
`path.nesting_too_deep` — **when the outer side branch is a nested list row**.
Declared with headings instead, the same shapes pass `check` in silence and
project badly: a path whose H2 is `{sequence=local}` and whose H3 inside it is
also `{sequence=local}` reports nothing, shows a count beside each branch, and
reads **0 lessons** overall, because nothing in it is on the main line.

So the working rule is not "the check will tell me". It is: **a study path has
one main line declared with `primary`, and a side branch is one level deep,
hanging off the lesson it belongs to.** If your course count is 0 or is smaller
than the lessons you can see on the page, this is the first thing to look at.

## A complete example

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

The course reads 8 lessons. The side branch shows as three, hanging under L03.
L04 follows L03, because the container is L03's child and L04 is its sibling.
L07 has no next lesson, because L08 is unwritten. The routine block is absent
from navigation and reads normally on the page.

## Listing a lesson you have not written yet

A lesson a path lists and the vault does not have is `map.disk_mismatch`. To
say it is owed rather than missing, put it under a heading carrying one of your
contract's gap words — the defaults are in
[`names-and-links.md`](names-and-links.md), along with the substring trap that
makes this the most expensive accident in the dialect. The heading still needs
its own `{sequence=…}` declaration, for example `## 缺口 {sequence=primary}`.

Softening a course entry works by **position** — the entry physically inside
the gap section — and not by name. Listing a name elsewhere as planned does not
excuse a course from answering what it promised.

## The rules this grammar reports

Twelve, all warnings, and [`diagnostics.md`](diagnostics.md) carries what each
one means. They are `path.role_missing`, `path.role_invalid`,
`path.role_duplicate`, `path.role_conflict`, `path.role_nested_primary`,
`path.role_misplaced`, `path.role_on_entry`, `path.nesting_too_deep`,
`path.local_orphan`, `path.entry_noncanonical`, `path.entry_multi_target` and
`path.entry_outside_branch`.

Two of those describe shapes not covered above. `path.role_on_entry` is one row
trying to be both a lesson and a branch heading — a row that opens with its
link and also carries a marker; give the branch its own row above the list it
opens. `path.local_orphan` is a side branch with nothing to hang from: the row
it nests under has to be an accepted lesson row, so hanging one under a
checkbox row, a noncanonical row or a multi-target row orphans it while the
parent row still looks fine to a reader.

To gate on course structure, run `check` with `--deny warn` on the path you
touched.
