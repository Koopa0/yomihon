---
name: yomihon-authoring
description: >-
   How to write Markdown that yomihon reads as a course, a map and a searchable
   note: frontmatter, filenames, the study-path marker, the dialect.
when_to_use: >-
   Before writing or revising a note for a yomihon-read vault, and when a note
   reads fine in Obsidian but is missing from a course, a count, prev/next or
   search.
user_invocable: true
metadata:
   author: koopa
   version: "2.0"
---

# Authoring for yomihon

yomihon reads a folder of Markdown and projects it: a reading page, a graph, a
search index, a map, a course with counts and prev/next. Prose alone gets the
reading page, and gets it well — no marker is needed to be read, linked,
searched or rendered.

**Everything beyond that is declared, never inferred.** yomihon reads no
structure, and no claim about a note's quality or status, out of heading
wording, list punctuation, or indentation. When it cannot determine a projection
it keeps the prose, stops, and reports. It never guesses, never flattens, and
never edits your file.

The contract (`<vault>/System/schemas/vault-schema.toml`) owns types, fields,
statuses, the scan and privacy boundaries, and which types form courses and
maps. Every vault's is different; `examples/vault/System/schemas/vault-schema.toml`
is a small working one. The code owns the dialect, the study-path grammar, link
resolution and search. Where this page and `yomihon check` disagree, `check` is
right.

## Checking your work

```bash
yomihon check --root <vault> --format json [--all] [--deny <severity|rule-id>]... [--baseline <file>] [path...]
```

Every finding carries `rule_id`, `severity`, `path`, `message`, `evidence`,
`suggested_action`, `source_rule` and `fingerprint`; `line` and `field` appear
only when the finding has one. Fix by `rule_id`, re-run, repeat until your file
is quiet.

| Thing | Behaviour |
|---|---|
| `[path...]` | narrows the run; written the way the vault spells it, relative to the root. An absolute path exits 2 with empty stdout |
| exit codes | `0` nothing named by `--deny` was found · `1` a `--deny` gate hit · `2` the command could not run. Findings alone never fail it: without `--deny` it reports and exits 0 |
| `--all` | keeps findings under `System/`, which the default output drops although the directory is fully scanned. Pass it when the note you touched lives there, or its silence means nothing |
| `--baseline <file>` | a prior run's output, subtracted by `fingerprint` so only new findings are reported and gated. A `fingerprint` carries its algorithm version as a `v1:` prefix, and a baseline written by another version stops the run at exit 2 rather than under-subtracting |
| `source_rule` | where a rule's authority lives: `vault-schema.toml`, that name with a section anchor (`#rules`, `#scan`, `#supersession`), or `yomihon` for the product's own dialect |
| `yomihon coverage` | reports concept coverage; never gates |
| `yomihon exists <name>` | exit 0 when a note for the name exists, 1 when none does, so a write-if-absent can gate on the exit code alone |
| no contract | a folder with no `System/schemas/vault-schema.toml` gets a tool error and exit 2 from all three commands. Reading and browser search are not gated; only this surface is |

`schema.` rules are errors; `path.` and `map.` rules are warnings, dropping to
`info` when a broken link is a tracked gap. To gate on course structure, use
`--deny warn` on the path you touched.

### Prove your instrument

A stale `yomihon` reports zero `path.` findings, or exits 2 with empty stdout.
Build the tree you mean to test against and call the binary by path.

```bash
go build -o ./yomihon ./cmd/yomihon
./yomihon check --root <vault> --format json | grep -c '"rule_id":"path\.'
```

## Frontmatter: the contract decides, and you may not invent a field

Read the vault's contract before choosing anything. Three of its tables gate
more than they look like they do:

- **`[fields] known`** is the complete list of keys a note may carry; a key
  outside it is `schema.unknown_key`, an error. That includes keys other yomihon
  capabilities read — `topics:` for the `topic:` filter, `domain:` for
  `domain:`. Writing the field is not the same as opening the capability.
- **`[fields] required`** must all be present; `[fields] lesson_only` names keys
  only a lesson may carry.
- **`[enums.status]` with `[fields.status_group]`** makes statuses conditional
  on type: a status legal for one type is `schema.enum` on another.

The frontmatter rules judge only files inside the contract's
`[scan] knowledge_dirs`. A file outside them is still scanned for links and
still read, but is not held to the schema.

`status` is the one field yomihon writes itself, under the contract's
`[[lifecycle]]`; `published` is set by hand only.

## Names are keys; titles are not

| | |
|---|---|
| Resolves against | four forms of the note's location — filename stem, filename, vault-relative path stem, that path — plus any `aliases` |
| Never resolves against | the frontmatter `title`. A link written against a title finds nothing; `check` names it `link.title_not_alias` |
| Two files, one name | every `[[link]]` to it is ambiguous and yomihon refuses to guess: `collision.name`, nothing links, a span lists the candidates. Two notes declaring one alias is `collision.alias` |
| Normalisation | trimmed, NFC, compared without regard to case, so `[[L01]]` and `[[l01]]` are one key and two files differing only in case collide |

Give a note a name unique in the vault; add an alias when a second spelling
should work; link by full vault-relative path when a generic name is
unavoidable.

## What the renderer treats specially

CommonMark and GFM render — tables, task lists, strikethrough, bare-URL
autolinks — plus footnotes, whose ids are prefixed per region so two bodies on
one page never collide. Beyond that:

| Written | Renders as | Miss behaviour |
|---|---|---|
| `[[Note]]` | a link | unresolved → marked in place, `link.broken`; ambiguous → a span listing the candidates |
| `[[Note\|alias]]` | a link labelled `alias` | split order is `\|` first, then `#`, then `^` |
| `[[Note#Heading]]` | a link into that section | the address is kept, the note still links, `link.section_missing` |
| `[[Note#^id]]` | a link to that block | the fragment is **withdrawn**, the link leads to the whole note, `link.block_missing` |
| `![[Note]]` | the note's body, inline | one level deep only: an embed inside an embed is not expanded |
| `![[Note#Heading]]` | an excerpt | nothing is shown; the block names the address that failed and links the note |
| `> [!warning] Title` | a tinted callout | a type outside the list below → plain blockquote with `[!type]` visible, plus a diagnostic |
| `> [!tip]-` / `> [!tip]+` | a native `<details>`, closed / open | — |
| `text. ^my-id` | a block address a link can reach | refused on a recognised callout's opening line and on a table row; the caret stays in the id |
| `==text==` | a highlight | exactly two `=` on each side. A single `=` is literal; surplus `=` also stay literal, outside the mark on the left and inside it on the right, so `===x===` gives `=<mark>x=</mark>` |
| `%%hidden%%` | nothing | unclosed runs to end of file, with a diagnostic naming the line it opened on |
| ` ```mermaid ` | a diagram | case-insensitive; the source is carried twice so it still reads without JavaScript |
| ` ```go ` | highlighted code | an unrecognised language falls back to plain text **silently, with no diagnostic** |
| `<ruby>漢<rt>かん</rt></ruby>` | ruby text | `ruby`, `rt`, `rp`, `br` and a `lang=` attribute on the first three are the whole allowlist; any other tag renders as visible text, never dropped |
| `![alt](pic.png)` | an image | a remote destination becomes an explicit link, never a request |
| `## 標題` | a heading with an anchor | CJK survives verbatim; a repeated slug bumps `-2`, `-3` until it is free |
| `<!-- read-aloud: ja -->` | a speech control on the next paragraph | `ja` is the only value, and it acts on a `type: lesson` note only — not on one held by a directory the contract's `[artifacts] non_instance_dirs` names. Anywhere else the comment is accepted and does nothing |
| `[[#Section]]` | **plain text** | a same-file anchor is not implemented, and draws no diagnostic — a silent trap |

Recognised callout types, closed; each group separated by · shares one default
title, used when the opening line names none. Markdown and wikilinks work in a
callout body. `success`, `check`, `done`, `important` and `tldr` fall through.

`info` `note` `tip` `hint` `abstract` `summary` `todo` · `question` `help` `faq`
· `example` · `quote` `cite` · `warning` `caution` `attention` · `danger`
`error` `bug` `fail` `failure` `missing`

## Being findable

Six filter keys:

`type:` · `status:` · `domain:` · `slug:` · `topic:` · `folder:`

| Property | Behaviour |
|---|---|
| Case | **lowercase only.** `Type:lesson` is not a filter — it degrades to a literal token searched as text |
| Repeated key | **AND.** Two `type:` filters both have to hold, so they are jointly unsatisfiable rather than last-wins |
| Values | not validated: exact string equality against whatever the note carries. `folder:` matches at a `/` boundary, `topic:` is membership |
| Unknown prefix | named back to the reader with all six offered, never silently searched as text |
| Quoting | `"…"`, `「…」`, `『…』` — at the start of a field, or straight after a recognised key and its colon |
| Indexed | title, aliases, body plain text and the vault-relative path are free-text searchable; the other five frontmatter values are **not**, and are reachable only through their own filter |
| CJK | no segmenter: a folded literal substring, and a newline between two Han or Kana runes is dropped |
| Ranking | none. Six fixed answer groups, then vault reading order |

## Study paths: sequence is declared, never inferred

Two gates run before any of the grammar below, and both are silent when unmet —
the body can be perfect and nothing projects.

1. **The note's `type` must be listed under `[navigation] path_types`** in the
   contract. A note whose type is not listed answers nothing at
   `/syllabus/<its path>` and enters no count, whatever it contains. Maps are
   gated the same way by `map_types`.
2. **A map is not a study path.** Maps, reports and ordinary notes do not read
   this syntax at all; the same marker inside one of them is plain text.

### The marker

Three values, closed and case-sensitive:

    {sequence=primary}   {sequence=local}   {sequence=none}

There is no other key. It is read at the end of the row's or heading's **own
first line**, trailing whitespace allowed, in exactly two places:

- **a heading, H2 through H6** — declaring the branch that heading opens;
- **a list row that has a child list beneath it** — declaring the child group
  that list forms. Its parent is the enclosing list item structurally, not the
  nearest lesson above it.

Exact form: a space before `sequence`, or a capitalised value, is not a marker
(space around the value itself is fine). A recognised marker is stripped from
the displayed name and the source bytes are untouched; a child branch does not
inherit its parent's role.

### Which rows become lessons

A row is a lesson when the first visible thing after its list marker is one
`[[link]]` (bold or italic around it is fine) and the row names no second note
outside nested lists; commentary goes after the link. Ordered and unordered rows
are the same row, and order is source order. Checkbox rows, embeds, same-file
anchors, and links inside code or `%%…%%` never count, and a lesson row before
the first level-2 heading belongs to no branch. A nested list with no marker
projects nothing and is reported; nothing is flattened.

### Five branch states

| State | Condition | Progression | Reported |
|---|---|---|---|
| `primary` | declared | main line; consecutive primary groups join end to end in declared order | no |
| `local` | declared | within its own group only | no |
| `none` | declared | none | no |
| unclassified | an undeclared nested list, or a heading that lists a lesson row and carries no readable marker | none | yes |
| structural | a **heading** that lists no lesson row and carries no marker | not applicable | no |

Declaring `none` is a legitimate authored answer and draws nothing; forgetting
to declare is not. A `local` branch may not contain another `local` branch —
that is `path.nesting_too_deep`; a `primary` branch may nest as deep as it
likes.

### What the states produce

- **The home count and the course count take `primary` only.** A `local` group
  carries its own order and its own count beside it on the path page. A `none`
  group leaves path navigation, the count, reverse placement and prev/next
  entirely, and its prose still reads on the note page.
- **A child group rolls into its parent's count when both are `primary` or
  structural, and only then.** A `local` child never does — it shows its own
  number instead, because folding a side branch into the total would print a
  figure no walk matches.
- **`primary` and `local` never link to each other through prev/next.** The
  lesson after the one a side branch hangs from is the next main-line lesson;
  the side branch does not rejoin.
- **An unresolved entry in a primary group still counts** toward the planned
  total. Openable navigation drops the unresolved stop and joins the resolved
  entries on either side of it within that component, never across a
  `primary`/`local`/`none` boundary — so a trailing unresolved entry leaves the
  lesson before it with no next lesson.

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

The course reads 8 lessons. The side branch shows as three, hanging under L03.
L04 follows L03, because the container is L03's child and L04 is its sibling.
L07 has no next lesson, because L08 is unwritten. The routine block is absent
from navigation and reads normally on the page.

### Marking a branch as a planned gap

A lesson listed but not yet written reports `map.disk_mismatch`. Put unwritten
lessons under a heading containing one of these words (still declared, e.g.
`## 缺口 {sequence=primary}`):

    缺口   待補   待寫   待整理   待建

Their broken links report as info, they still count, and they stay unopenable
until written. The softening runs from that heading down to the next heading at
the same or a higher level, and the plain-text items listed under it become
planned names for the whole vault.

A second, smaller set acts on an ordinary line rather than a heading:

    待整理   待建   下一課

Every `[[name]]` written on that same line is marked planned across the vault; a
bare name with no brackets is not collected. This softens an ordinary broken
link but **not** a lesson a course lists — what a course promises is answered
where the course says it.

Keep those words out of ordinary headings: the match is a substring with no
word boundary, so a heading that merely happens to use one turns its whole
section into a planned gap, and every broken link under it drops to `info` —
which `--deny warn` does not catch.

## What goes wrong most

Ranked by frequency in the vault this was written against, not by severity. The
first three are one problem in three shapes: yomihon could not place written
content in a course.

| `rule_id` | What went wrong |
|---|---|
| `map.disk_unlisted` | The lesson file exists but no syllabus lists it. Writing the file is half the job; a course is a list, and a lesson not on the list is not in the course |
| `path.role_missing` | A branch lists lessons but declares no role — or a nested list carries no declaration, whatever it holds. Undeclared is unclassified, and unclassified projects nothing |
| `path.entry_noncanonical` | The row's `[[link]]` is not the first visible thing after the list marker. A label, an inline code span or an embed in front of it leaves the row as prose |
| `path.entry_multi_target` | The row names two notes. Three honest rewrites, the author's choice: give each note its own row; turn the relation into an ordinary paragraph; or keep the link-first entry and move its commentary to a paragraph after the item |

## Before calling a note done

1. `yomihon check --root <vault> --format json <path>` is quiet for your file,
   run with a binary you have confirmed can see `path.` rules.
2. `--deny error` on your path is quiet: you invented no field, and copied no
   enum out of the contract into your prose.
3. If the note is a lesson, a syllabus lists it — in the same change.
4. If you added a branch, it carries a `{sequence=…}` declaration.
5. Every `[[link]]` resolves, or is deliberately a planned gap under a gap
   heading.
