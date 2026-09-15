---
name: yomihon
description: >-
   How to write Markdown that yomihon reads as a course, a map and a searchable
   note: frontmatter, filenames, the study-path marker, the dialect.
when_to_use: >-
   Before writing or revising a note for a yomihon-read vault, and when a note
   reads fine in Obsidian but is missing from a course, a count, prev/next, a
   map or search, or when a link you know is broken is not being reported.
metadata:
   version: "1.0"
   author: Koopa
---

# Authoring for yomihon

yomihon reads a folder of Markdown and projects it: a reading page, a graph, a
search index, a map, a course with counts and prev/next. Prose alone gets the
reading page, and gets it well — no marker is needed to be read, linked,
searched or rendered.

**Everything beyond that is declared, never inferred.** yomihon reads no
structure, and no claim about a note's quality or status, out of heading
wording, list punctuation, or indentation. When it cannot determine a
projection it keeps the prose, stops, and reports. It never guesses and never
flattens, and the one field it ever writes is `status` — the section below says
what that means and what it refuses.

Everything below assumes a `yomihon` on your `PATH`. There is no `--version`
to ask; this page's probe is how you find out what the one you have can see.
`yomihon serve` runs until you stop it, so the recipes that use it are two
things to do, not one line to paste.

The contract (`<vault>/System/schemas/vault-schema.toml`) owns types, fields,
statuses, the scan and privacy boundaries, and which types form courses and
maps. Every vault's is different; `examples/vault` in this repository ships a
small working one. The code owns the dialect, the study-path grammar, link
resolution and search. Where this page and `yomihon check` disagree, `check` is
right.

This file is the entry point: it names every decision a note has to make and
sends you to the file that owns the whole of each subject. Beside it,
`references/` holds one file per subject, and each is the authority for what it
covers. This page names things in a sentence each; when a rule needs more than
a sentence it links rather than explaining it a second way, because a second
explanation is what drifts.

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

Every finding carries a `rule_id`. Fix by rule id, re-run, repeat until your
file is quiet.

Exit codes: `0` nothing named by `--deny` was found · `1` a `--deny` gate hit ·
`2` the command could not run. Findings alone never fail it, so a green exit
without `--deny` is not evidence of a clean vault. A severity passed to
`--deny` is a threshold, not a match: `--deny warn` fails on warnings and
errors both.

To gate on course structure, use `--deny warn --all` on the path you touched.
`--all` restores the link and course findings that touch nothing inside
`[scan] knowledge_dirs`, which the default report drops. What it cannot do is
make the frontmatter rules judge a note filed outside that layer — those never
ran, and no flag starts them; `references/diagnostics.md` owns the difference.

That gate is not a completeness claim, and this page makes none. It misses at
least three things by design: anything your vault has declared owed rather
than missing, which sits at `info`; a side branch declared with headings
rather than nested rows, which `references/study-paths.md` shows passing in
silence; and a lesson you never listed, which `map.disk_unlisted` almost never
catches. The severity model is in `references/diagnostics.md`.

When the command cannot run at all it exits 2 with **empty stdout** and the
reason on stderr. On a vault that otherwise looks healthy the cause is almost
always a contract yomihon could not use — most often one carrying no
`[privacy]` section, but any refused contract reads the same, because a
contract it could not load has granted nothing. `check` will not say which:
naming the reason would quote the contract back out under the very policy that
is missing. Read it where reading is the point:

```bash
yomihon serve --root <vault>       # then http://127.0.0.1:9610
```

The server states the cause on the page and logs it at startup. Two of the
lines it logs there are worth knowing anyway, because a course or a map that
projects nothing shows up in them as a number, and in `check` not at all:

```
level=INFO msg="vault snapshot built" files=36 ... paths=2 maps=2 ...
level=INFO msg="yomihon serving" addr=127.0.0.1:9610 vault=…
```

The whole command surface — every rule, every flag, every refusal, both other
commands — is `references/diagnostics.md`.

### Prove your instrument

A binary too old for a rule reports nothing rather than failing, so a quiet
report is not evidence until you have watched this binary report *something*.
Prove it against a tree built to fail, never against the vault you are judging:
zero course findings on a healthy vault is exactly what a blind binary prints
on any vault.

The probe below writes its own vault — contract and all — so nothing it says
depends on your contract's spellings, scan directories or privacy policy, and a
wrong answer is about the binary rather than about your notes. It reads the
**exit code and the stderr line**, never stdout, because every refusal prints
nothing at all on stdout and a check that reads stdout scores a refusal as a
clean pass:

```bash
probe=$(mktemp -d)
mkdir -p "$probe/System/schemas"
cat > "$probe/System/schemas/vault-schema.toml" <<'TOML'
schema_version = "1"
[enums]
type = ["probe"]
[enums.status]
note = ["draft"]
[fields]
known = ["title", "type", "status"]
[navigation]
path_types = ["probe"]
map_types = []
[artifacts]
non_instance_dirs = []
[privacy]
never_egress_dirs = []
[[lifecycle]]
status = "draft"
applies_to = ["*"]
initial = true
from = []
owner = ["author"]
TOML
cat > "$probe/Probe.md" <<'MD'
---
title: Probe
type: probe
status: draft
---

## Undeclared

- [[Anything]]
MD
yomihon check --root "$probe" --all --deny path.role_missing >/dev/null 2>"$probe/err"
verdict=$?
reason=$(cat "$probe/err" 2>/dev/null)
rm -rf "$probe"
case "$verdict" in
  1) echo "proven: this binary reports path.role_missing" ;;
  0) echo "blind: it knows the rule and did not report it where one is guaranteed" ;;
  127) echo "no binary: there is no yomihon on your PATH" ;;
  *) case "$reason" in
       *"unknown --deny"*) echo "too old: this binary has never heard of path.role_missing" ;;
       *) echo "refused: $reason" ;;
     esac ;;
esac
```

Five answers, five different facts, and none of them reads as success by
accident:

| Verdict | What it tells you |
|---|---|
| **proven** | exit 1: the gate fired, so this binary sees the course rules |
| **blind** | exit 0: it accepted `path.role_missing` as a real rule id and then reported none where one is guaranteed. Trust none of its quiet course reports |
| **too old** | exit 2, `unknown --deny` on stderr: it has never heard of this rule, which is the plainest answer the probe can give you |
| **no binary** | exit 127 |
| **refused** | exit 2 with any other message: it could not read the probe vault and has said nothing about its rules. The message is printed for you |

Keeping stderr is what separates the last three, and it is the difference
between telling a reader their binary is out of date and sending them to debug
a contract. Do not run this under `set -e`: *proven* is the verdict that exits
non-zero, so `set -e` would abort on success and leave the good news silent.

**Watch it fail before you trust it.** Two mutations do that without touching
anything else:

- change `--deny path.role_missing` to `--deny collision.alias`, a real rule id
  that nothing in a one-note tree can produce — *blind*;
- delete the `- [[Anything]]` row, leaving a heading with nothing under it. A
  branch that lists no lesson is structural rather than unclassified, so no
  finding is due — *blind*.

A third shows why `--all` is in the command, and carries a side effect worth
naming. Add a `[scan]` table with `knowledge_dirs = ["Elsewhere"]` and drop
`--all`: the finding is discarded for touching nothing inside the knowledge
layer, and the verdict is *blind*; put `--all` back and it is *proven* again.
That contract also earns a `schema.unmatched_knowledge_dir`, because the probe
tree has no `Elsewhere` — which is exactly why this gate names a rule id rather
than `--deny warn`. A severity threshold would have turned that injected error
into a false *proven*.

## Frontmatter: the contract decides, and you may not invent a field

Open the vault's contract before choosing a single key. There is no default
field set, no key that is safe because it looks ordinary, and no value you can
reason your way to: `[fields] known` lists the keys any note may carry and
`[fields] lesson_only` the further ones only a lesson may, and a key on neither
list is an error even when nothing would have read it. That rule catches people
twice — once on a field they invented, and once on a field a yomihon capability
really does read, which still has to be declared before writing it does
anything.

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
autolinks — plus footnotes. A page that pulls a second body into itself with an
embed prefixes that body's footnote ids (`fn:1` in the host, `y1-fn:1` in the
embedded one), so two notes' footnotes on one page never collide. Beyond
that:

| Written | Renders as | Miss behaviour |
|---|---|---|
| `[[Note]]` | a link | unresolved → marked in place, `link.broken`; ambiguous → a span listing the candidates |
| `[[Note\|alias]]` | a link labelled `alias` | split order is `\|` first, then `#`, then `^` |
| `[[Note#Heading]]` | a link into that section | the address is kept, the note still links, `link.section_missing` |
| `[[Note#^id]]` | a link to that block | the fragment is **withdrawn**, the link leads to the whole note, `link.block_missing` |
| `![[Note]]` | the note's body, inline | one level deep only: an embed inside an embed is not expanded |
| `![[Note#Heading]]` | an excerpt | nothing is shown; the block names the address that failed and links the note |
| `> [!warning] Title` | a tinted callout | a type outside the list below → a plain blockquote with `[!type]` visible. The note page names it under the page's own diagnostics; `check` has no rule for it |
| `> [!tip]-` / `> [!tip]+` | a native `<details>`, closed / open | — |
| `text. ^my-id` | a block address a link can reach | works on a heading, an ordinary paragraph and a callout's body line; refused on a recognised callout's opening line and on a table row; the caret stays in the id |
| `==text==` | a highlight | exactly two `=` on each side. A single `=` is literal; surplus `=` also stay literal, outside the mark on the left and inside it on the right, so `===x===` gives `=<mark>x=</mark>` |
| `%%hidden%%` | nothing | unclosed runs to the end of the body, with a diagnostic naming the body line it opened on |
| `<!-- a remark -->` | **the comment, visible as text** | one word decides the fate of an HTML comment: `read-aloud`. A comment that opens with it is recognised and handled by the row below; every other comment is escaped onto the page and stays visible, so to hide a remark use `%%…%%` |
| ` ```mermaid ` | a diagram | case-insensitive, and the whole info string must be that word; the source is carried twice so it still reads without JavaScript |
| ` ```go ` | highlighted code | an unrecognised language falls back to plain text **silently, with no diagnostic** |
| `<ruby>漢<rt>かん</rt></ruby>` | ruby text | `ruby`, `rt`, `rp`, `br` and a `lang=` attribute on the first three are the allowlist; any other tag is escaped and stays visible |
| `![alt](pic.png)` | an image | a remote destination becomes an explicit link, never a request; a destination that is neither local nor http shows the alt text alone |
| `## 標題` | a heading with an anchor | CJK letters and digits survive; other characters collapse to `-`, and a repeated slug bumps `-2`, `-3` until it is free |
| `<!-- read-aloud: ja -->` | a speech control on the next paragraph | `ja` is the only value: a `read-aloud` comment naming any other language is **deleted from the page**, not escaped and not left visible, wherever it is written. `ja` raises a control only on a `type: lesson` note outside `[artifacts] non_instance_dirs`; anywhere else it is passed through as a real HTML comment, which a browser does not draw, so it is invisible and does nothing |
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
   this syntax at all, and the marker does not show up on their pages either:
   it is stripped wherever it could have declared something, so it vanishes
   rather than turning into visible text. `references/study-paths.md` owns
   where that stripping reaches and the one heading it does not; a map's own
   grammar is `references/maps.md`.

Past those gates, the grammar is three values on a branch — closed and
case-sensitive, and exact about where they may be written:

    {sequence=primary}   {sequence=local}   {sequence=none}

`primary` is the main line and the only thing the course count takes. `local`
is a side branch, with its own order and its own count and no prev/next link to
or from the main line. `none` leaves navigation entirely and still reads.
Undeclared is unclassified, and unclassified projects nothing —
`path.role_missing`, the commonest fault there is.

Four habits carry most of it: declare a role on every branch that lists
lessons; open a lesson row with its `[[link]]` and put the commentary after it;
keep a side branch one level deep, hanging off the lesson it belongs to; and do
not rely on `check` for that last one, which it catches in some shapes and not
others.

Where the marker may be written, what each state produces, which rows count as
lessons, how to mark a lesson you have not written yet, and the shapes `check`
misses are all in `references/study-paths.md`. Read it before you build a
course, not after the count comes out wrong.

## Maps: grouping without ordering

A map is the other declared collection: a note whose type is on
`[navigation] map_types`, holding links grouped under headings rather than put
in order. It has branches, not lessons — no count of lessons, no prev/next, and
no sequence grammar.

The thing to know before writing one is that **no rule judges a map's shape.**
A heading whose links all point at nothing, or sit somewhere the scanner does
not read, simply disappears from the map and is reported by nobody. You find
that out by looking at `/maps`, where such a map reads *0 branches*, not by
running `check`. What counts as a branch, what counts as an entry, what
`map_kind` is for, and what listing a note on a map changes about
`yomihon coverage` are in `references/maps.md`.

## One note into a course, end to end

Writing a lesson is not joining a course, and a course is a note. Five steps,
in this order, and only the last is about the course:

1. **The filename**, because it is the key a `[[link]]` will be typed against,
   and the title is not.
2. **The frontmatter**, every key traced to a table in the contract rather than
   chosen — `[fields] required` for what must be there, `[fields] lesson_only`
   for the extras a lesson may carry, `[enums.status]` for the value, `[rules]`
   for the shape of it.
3. **The directory**, one `[scan] knowledge_dirs` names — because outside it
   the frontmatter rules judge nothing, and silence there is not a pass.
4. **The study path's own type**, which has to be on `[navigation] path_types`.
   This is the gate that fails most quietly: get it wrong and the note is fine,
   the page renders, and the course simply does not exist.
5. **One row on a branch that already declares its role**, opening with the
   lesson's `[[link]]` and nothing before it.

Then `yomihon check --root <vault> --format json <the lesson> <the path>`, and
it should print nothing at all.

Step 5 carries the fault most worth fearing **when you are adding a lesson**.
Put a word in front of the link — `- 第四課：[[L04 …]]` — and you get
`path.entry_noncanonical`: the row still reads, the link still works, the page
looks finished, and the course count silently drops back by one. (A different
fault carries that title for broken links; `references/names-and-links.md`
owns it.)

`references/worked-example.md` is this walk done for real against the vault
this repository ships — every contract table it draws on, the counts before and
after, and the verbatim output of each way of getting it wrong.

## What goes wrong most

One mistake dominates, in four shapes: **yomihon could not place written content
in a course.** In descending order of how often it was hit in the vault this was
written against — a vault whose study paths declare a `domain`, which is what
let the fourth fire there at all — `path.role_missing`,
`path.entry_noncanonical`, `path.entry_multi_target`, `map.disk_unlisted`. Each
is explained in `references/diagnostics.md`.

What they share is the reason they go unnoticed: the note reads correctly, the
link works, the page looks finished, and only a number somewhere else is wrong.
Nothing here is loud.

The last of the four deserves a warning of its own, because its name promises
more than it delivers. `map.disk_unlisted` is the one that would catch a lesson
you wrote and forgot to list, and on most vaults it never runs: the conditions
are narrow enough that `references/diagnostics.md` spends a paragraph on them,
and a vault whose courses span subjects — the normal case — never sees it at
all. **So an unlisted lesson is usually silent, and step 3 of the checklist
below is not something `check` will do for you.**

## Before calling a note done

1. `yomihon check --root <vault> --format json <path>` is quiet for your file,
   run with a binary the probe above called *proven*. Name the path you
   touched; the whole vault will have findings that are not yours.
2. `--deny warn --all` on that path exits 0. The threshold covers errors too,
   so one run says you invented no field, wrote no value the contract does not
   declare, and left no branch undeclared — **of a note the schema rules
   reach**. If it is filed outside `[scan] knowledge_dirs` this proves none of
   that, and the repair is to move it, not to add a flag.
3. If the note is a lesson, a study path lists it — in the same change.
4. If you added a branch, it carries a `{sequence=…}` declaration.
5. Every `[[link]]` resolves, or is deliberately a planned gap under a gap
   heading.
6. **Look at the projection, because no command reports it.** Open the course
   at `/syllabus/<the path's path>` and the count is what you meant; a map you
   wrote is at `/maps` with the branches you expected. A 404 or a number one
   short is the only sign of the quietest fault there is — a note whose type
   never reached `[navigation]`.
