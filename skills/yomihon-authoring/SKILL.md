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
   version: "3.0"
---

# Authoring for yomihon

yomihon reads a folder of Markdown and projects it: a reading page, a graph, a
search index, a map, a course with counts and prev/next. Prose alone gets the
reading page, and gets it well — no marker is needed to be read, linked,
searched or rendered.

**Everything beyond that is declared, never inferred.** yomihon reads no
structure, and no claim about a note's quality or status, out of heading
wording, list punctuation, or indentation. When it cannot determine a
projection it keeps the prose, stops, and reports. It never guesses, never
flattens, and never edits your file.

The contract (`<vault>/System/schemas/vault-schema.toml`) owns types, fields,
statuses, the scan and privacy boundaries, and which types form courses and
maps. Every vault's is different; `examples/vault` in this repository ships a
small working one. The code owns the dialect, the study-path grammar, link
resolution and search. Where this page and `yomihon check` disagree, `check` is
right.

This file is the entry point and is enough to write a correct note. Beside it,
`references/` holds one file per subject for when you need the whole of one;
each is named below where you would want it.

## What carrying this skill changes

Four habits, and they are the whole of it:

1. **You settle the filename before the title,** because the filename is what a
   `[[link]]` resolves against and the title is not.
2. **You open the vault's contract before choosing frontmatter,** because the
   legal keys and values are declared per vault and there is no default set.
3. **You declare structure rather than implying it** — a course is a list with
   a role written on each branch, not an arrangement a reader can see.
4. **You run `yomihon check` before you call a note done,** because the ways
   this goes wrong are mostly silent: the page still reads, and the count is
   quietly different from what you meant.

## Checking your work

```bash
yomihon check --root <vault> --format json [--all] [--deny <severity|rule-id>]... [--baseline <file>] [path...]
```

Every finding carries `rule_id`, `severity`, `path`, `message`, `evidence`,
`suggested_action`, `source_rule` and `fingerprint`; `line`, `field`, `target`,
`resolved_to` and `collision_members` appear only when the finding has them, so
their absence tells you something too. Fix by `rule_id`, re-run, repeat until
your file is quiet.

Exit codes: `0` nothing named by `--deny` was found · `1` a `--deny` gate hit ·
`2` the command could not run. Findings alone never fail it, so a green exit
without `--deny` is not evidence of a clean vault. A severity passed to
`--deny` is a threshold, not a match: `--deny warn` fails on warnings and
errors both.

Every `schema.` rule is an error. Everything else is a warning, except that
three link rules drop to `info` when the vault has declared the name owed
rather than missing — so `--deny warn` deliberately does not catch those. To
gate on course structure, use `--deny warn` on the path you touched.

Three ways the command refuses instead of answering, all exit 2 with empty
stdout: a folder with no contract; a `[path...]` inside a directory the
contract withholds; and — the commonest, and the one that looks like a broken
install — a contract carrying no privacy section at all, which disables this
whole command surface for the vault.

The whole surface — every rule, every flag, the refusals, both other commands —
is `references/diagnostics.md`.

### Prove your instrument

A stale `yomihon` reports zero `path.` findings rather than failing, so a clean
report is not evidence until you have seen the binary report something. Prove
it against a tree that *must* fail, never against the vault you are judging: a
correct binary reports zero `path.` findings on a healthy vault, which is
exactly what a stale one reports on any vault.

Two files are enough — a copy of the contract, and a study path whose branch
declares no role:

```bash
go build -o ./yomihon ./cmd/yomihon
mkdir -p /tmp/probe/System/schemas /tmp/probe/Notes
cp <vault>/System/schemas/vault-schema.toml /tmp/probe/System/schemas/
printf -- '---\ntitle: Probe\ntype: study-path\nstatus: draft\n---\n\n## Undeclared\n\n- [[Anything]]\n' \
  > /tmp/probe/Notes/Probe.md
./yomihon check --root /tmp/probe --format json | grep -c '"rule_id":"path\.'
```

That prints `1`. A `0` means the binary cannot see `path.` rules, and every
quiet course report it has given you is worthless.

The `type` in that probe has to be one your contract lists under
`[navigation] path_types` — `study-path` above is the example vault's spelling.
Get that wrong and the probe prints `0` for the same reason a stale binary
does, which is the trap this whole section exists to avoid.

## Frontmatter: the contract decides, and you may not invent a field

Open the vault's contract before choosing a single key. There is no default
field set, no key that is safe because it looks ordinary, and no value you can
reason your way to: `[fields] known` is the complete list of keys a note may
carry, and everything outside it is an error even when nothing would have read
it. That rule catches people twice — once on a field they invented, and once on
a field a yomihon capability really does read, which still has to be declared
before writing it does anything.

Three things about a contract you have not seen before are worth knowing in
advance, because each makes a legal-looking note wrong:

- a status can be legal for one type and an error on another;
- a key a lesson may carry may be forbidden to every other type;
- a note outside the directories the contract calls knowledge is read and
  linked and searched, but is not judged — so silence over it is not a verdict.

`references/frontmatter.md` is every table, what each one does to a note you are
writing, and the two sections whose *absence* quietly closes a capability
rather than failing the load.

`status` is the one field yomihon writes itself, under the contract's
`[[lifecycle]]`, and it rewrites the value alone — the rest of that line, the
comment you left on it and the quoting you chose all survive. `published` is
the exception it will not make: it records a publication outside the vault that
nothing here can attest, so the control is never offered and a request for it
is refused. It is set by hand.

## Names are keys; titles are not

| | |
|---|---|
| Resolves against | four forms of the note's location — filename stem, filename, vault-relative path stem, that path — plus any `aliases` |
| Never resolves against | the frontmatter `title`. A link written against a title finds nothing; `check` names it `link.title_not_alias` and tells you which note you meant |
| Two files, one name | every `[[link]]` to it is ambiguous and yomihon refuses to guess: `collision.name`, nothing links, a span lists the candidates. Two notes declaring one alias is `collision.alias` |
| Normalisation | trimmed, NFC, compared without regard to case, so `[[L01]]` and `[[l01]]` are one key and two files differing only in case collide |

Give a note a name unique in the vault; add an alias when a second spelling
should work; link by full vault-relative path when a generic name is
unavoidable. Fragments, embeds, and how to mark a link as owed rather than
broken are in `references/names-and-links.md`.

## What the renderer treats specially

CommonMark and GFM render — tables, task lists, strikethrough, bare-URL
autolinks — plus footnotes, whose ids are prefixed per additional region so two
bodies on one page never collide. Beyond that:

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
| `text. ^my-id` | a block address a link can reach | works on a heading, an ordinary paragraph and a callout's body line; refused on a recognised callout's opening line and on a table row; the caret stays in the id |
| `==text==` | a highlight | exactly two `=` on each side. A single `=` is literal; surplus `=` also stay literal, outside the mark on the left and inside it on the right, so `===x===` gives `=<mark>x=</mark>` |
| `%%hidden%%` | nothing | unclosed runs to the end of the body, with a diagnostic naming the body line it opened on |
| `<!-- a remark -->` | **the comment, visible as text** | an ordinary HTML comment is escaped onto the page, not hidden. To hide a remark use `%%…%%` |
| ` ```mermaid ` | a diagram | case-insensitive, and the whole info string must be that word; the source is carried twice so it still reads without JavaScript |
| ` ```go ` | highlighted code | an unrecognised language falls back to plain text **silently, with no diagnostic** |
| `<ruby>漢<rt>かん</rt></ruby>` | ruby text | `ruby`, `rt`, `rp`, `br` and a `lang=` attribute on the first three are the allowlist; any other tag is escaped and stays visible — except a `read-aloud` comment naming anything but `ja`, which is removed outright |
| `![alt](pic.png)` | an image | a remote destination becomes an explicit link, never a request; a destination that is neither local nor http shows the alt text alone |
| `## 標題` | a heading with an anchor | CJK letters and digits survive; other characters collapse to `-`, and a repeated slug bumps `-2`, `-3` until it is free |
| `<!-- read-aloud: ja -->` | a speech control on the next paragraph | `ja` is the only value, and it acts on a `type: lesson` note only — not on one held by a directory the contract's `[artifacts] non_instance_dirs` names. Anywhere else the comment does nothing |
| `[[#Section]]` | **plain text** | a same-file anchor is not implemented and draws no diagnostic. What is left is the display half — `[[#Section]]` leaves `#Section`, and `[[#Section\|see below]]` leaves only `see below` |
| `> [!quote] [[Note]]` | **plain text** | a recognised callout's title is escaped, not parsed; a wikilink, an HTML tag, emphasis, a code span, a markdown link or an image there draws `callout.title_markup` — move the markup into the body |

Recognised callout types, closed; each group separated by · shares one default
title, used when the opening line names none. The title is plain text. Markdown
and wikilinks work in a callout body. `success`, `check`, `done`, `important`
and `tldr` fall through.

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
| Values | folded the same way matching folds text (NFC, fullwidth ASCII narrowed, lowercase). Values are not validated against the contract. `folder:` matches at a `/` boundary after that fold; `topic:` is membership of the folded topics |
| Unknown prefix | named back to the reader with all six offered, and the term is searched as text rather than dropped |
| Quoting | `"…"`, `「…」`, `『…』` — at the start of a field, or straight after a recognised key and its colon |
| Indexed | title, aliases, declared topics, body plain text and the vault-relative path are free-text searchable; type, status, domain and slug are reachable only through their own filter. An alias hit is filed with the title hits |
| CJK | no segmenter: a folded literal substring, and a newline between two Han or Kana runes is dropped |
| Ranking | eight fixed groups. Where the query was found decides the group — title, then body, then topics — and a note outranks a non-markdown file at each; last come the two groups matched on their path alone. Inside a group, notes in the directories the contract calls knowledge come before those outside, then vault reading order. A title that is exactly the query under that fold leads. There is no score |

## Study paths: sequence is declared, never inferred

Two gates run before any of the grammar below, and both are silent when unmet —
the body can be perfect and nothing projects.

1. **The note's `type` must be listed under `[navigation] path_types`** in the
   contract. A note whose type is not listed answers nothing at
   `/syllabus/<its path>` and enters no count, whatever it contains. Maps are
   gated the same way by `map_types`.
2. **A map is not a study path.** Maps, reports and ordinary notes do not read
   this syntax at all — and the marker does not show up on their pages either.
   The renderer strips it from any heading or list row in any note, so it
   vanishes while declaring nothing.

Past those gates, the grammar is three values on a branch — closed and
case-sensitive, and exact about where they may be written:

    {sequence=primary}   {sequence=local}   {sequence=none}

`primary` is the main line and the only thing the course count takes. `local`
is a side branch, with its own order and its own count and no prev/next link to
or from the main line. `none` leaves navigation entirely and still reads.
Undeclared is unclassified, and unclassified projects nothing —
`path.role_missing`, the second commonest fault there is.

Four habits carry most of it: declare a role on every branch that lists
lessons; open a lesson row with its `[[link]]` and put the commentary after it;
keep a side branch one level deep, hanging off the lesson it belongs to; and do
not rely on `check` for that last one, which it catches in some shapes and not
others.

Where the marker may be written, what each state produces, which rows count as
lessons, how to mark a lesson you have not written yet, and the shapes `check`
misses are all in `references/study-paths.md`. Read it before you build a
course, not after the count comes out wrong.

## One note into a course, end to end

Take a capture in `examples/vault` — ordinary prose, `type: inbox`, no domain.
It has a reading page and is in the search index. It is in no course. Four
steps put it in one, and only the last is about the course:

**The filename first**, because it is the key: `Lessons/L04 Say which language
a note is in.md`. **Then frontmatter, every key traced to the contract:**

```yaml
---
title: Say which language a note is in
type: lesson
status: draft
domain: yomihon
slug: l04-note-language
level: intermediate
---
```

`title` `type` `status` `domain` because `[fields] required` names them;
`slug` and `level` because `[fields] lesson_only` permits them to a lesson;
`draft` because `[enums.status]` gives a lesson `draft`, `ready`, `archived`
and not `published`; the slug in that shape because `[rules] slug_pattern` says
so. **Then the file goes under a directory `[scan] knowledge_dirs` names.**
**Then one row in the path**, on a branch that already declares its role:

```markdown
## Doing it {sequence=primary}

- [[L01 Point yomihon at a folder]]
- [[L02 Add a contract]]
- [[L04 Say which language a note is in]]
```

Now `yomihon check --root <vault> --format json <the lesson> <the path>` is
quiet, and the course counts one more.

Get any of it wrong and the report is specific. `status: published` gives
`schema.enum` saying *not a valid lesson status*. A missing `domain` gives
`schema.required`. An invented key gives `schema.unknown_key`. Writing the row
as `- 第四課：[[L04 Say which language a note is in]]` gives
`path.entry_noncanonical` — and this is the one to fear, because the page still
reads perfectly while the course count silently drops back.

That is the short version. `references/worked-example.md` walks the same note
at full length against the vault this repository ships: every contract table it
draws on, the study path's own frontmatter and the gate hiding in it, the
counts before and after, and the verbatim output of each way of getting it
wrong.

## What goes wrong most

One mistake dominates, in four shapes: **yomihon could not place written content
in a course.** In descending order of how often it was hit in the vault this was
written against — `path.role_missing`, `path.entry_noncanonical`,
`path.entry_multi_target`, `map.disk_unlisted`. Each is explained in
`references/diagnostics.md`.

What they share is the reason they go unnoticed: the note reads correctly, the
link works, the page looks finished, and only a number somewhere else is wrong.
Nothing here is loud.

The last of the four deserves a warning of its own, because its name promises
more than it delivers. `map.disk_unlisted` is the one that would catch a lesson
you wrote and forgot to list — but it only runs for a study path that itself
declares a `domain`, only over lessons carrying that same domain, and never
over a draft. A vault whose courses span subjects, which is the normal case,
never sees it at all. **So an unlisted lesson is usually silent, and step 3 of
the checklist below is not something `check` will do for you.**

## Before calling a note done

1. `yomihon check --root <vault> --format json <path>` is quiet for your file,
   run with a binary you have confirmed can see `path.` rules.
2. `--deny error` on your path is quiet: you invented no field, and copied no
   enum out of the contract into your prose.
3. If the note is a lesson, a syllabus lists it — in the same change.
4. If you added a branch, it carries a `{sequence=…}` declaration.
5. Every `[[link]]` resolves, or is deliberately a planned gap under a gap
   heading.
