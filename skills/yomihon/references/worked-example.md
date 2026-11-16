# One note, from prose to a lesson in a course

**The question this answers:** what does it actually take to turn a piece of
writing into something a course lists, counts and walks — and what does
`yomihon check` say for each way of getting it wrong?

Everything below is a real walk through `examples/vault`, the vault the
yomihon repository ships, with the real output of every command.

If you have that repository, run along:

```sh
cp -R examples/vault /tmp/lab
yomihon check --root /tmp/lab --format json
```

That last command prints three findings and exits 0. They are deliberate — the
example vault keeps a faulty note type and two dead link targets on display so
a reader can see what a diagnostic looks like. Everything added below leaves
those three and adds nothing.

If you do not have the repository — you installed the skill beside your own
vault, which is where it belongs — read it instead as the shape of the work.
Every frontmatter key, every contract table and every finding below is quoted
literally, so you can map each one onto your own contract without running
anything.

## Where we start

`Inbox/A speak control for Chinese.md` is ordinary prose that yomihon reads
perfectly well:

```markdown
---
type: inbox
draft: true
created: 2026-09-22
---

Read-aloud accepts any well-formed BCP 47 language tag on a lesson paragraph,
including `zh-Hant`, `en` and `ja`. The paragraph keeps its declared language;
if speech is unavailable on this device, the control reports that it cannot
play.
```

It has a reading page. It is in the search index. Its words are findable.
It is in no course, and nothing about it is wrong: a capture is a capture.

It writes no `title` and no `domain`, because no note does: its title is its
file name, since it has no `#` heading, and it sits directly under `Inbox/`,
which is not a domain root, so it has no domain. `draft: true` says it is not
finished. [`frontmatter.md`](frontmatter.md) owns the fields; the point here is
that the note below, once it becomes a lesson, is held to more.
Turning prose into the fourth lesson of the `Reading yomihon` study path is five
decisions, and only the last one is about the course.

## 1. What is it? The contract says which answers exist

`examples/vault/System/schemas/vault-schema.toml` declares:

```toml
[types.lesson]
role = "lesson"
required = ["slug", "level"]
fields = ["slug", "level"]
slug_pattern = "^[a-z0-9]+(-[a-z0-9]+)*$"
```

`lesson` has a table, and it holds the `lesson` role, so `lesson` is available.
Had it not been, the honest move is to edit the contract deliberately or pick
another type — not to write the value and hope. A type the contract does not
declare is `schema.enum`, an error.

## 2. The filename, before the title

The filename stem is what a `[[link]]` resolves against. The title — the
note's first `#` heading — never is. So the filename is chosen first and chosen
to be the thing other notes will want to type:

```
Lessons/L04 Say which language a note is in.md
```

The name has to be unique in the whole vault, compared after trimming, NFC
normalisation and case folding — so a second `l04 say which language a note is
in.md` anywhere would collide and neither would resolve. See
[`names-and-links.md`](names-and-links.md).

## 3. The frontmatter, every key traced to the contract

```markdown
---
type: lesson
slug: l04-note-language
level: intermediate
created: 2026-09-22
lang: en
---

# Say which language a note is in
```

The `#` heading is the lesson's title; there is no `title` key to write. Where
each key comes from, in this vault's contract:

| Key | Why it is here |
|---|---|
| `type` | `[fields] required` names it, and `[types.lesson]` declares the value |
| `slug` `level` | `[types.lesson] fields` lists them, so a lesson may carry them and another type may not, and its `required` makes both mandatory |
| `created` `lang` | on `[fields] common`, so any note may carry them, and neither is required. Between that list and the type's own `fields`, and the always-allowed `type`, `draft`, `aliases`, `tags` and `cssclasses`, they are the whole permitted vocabulary, which is also how you know the `aliases` repair for a missed link is available in this vault at all |
| no `draft` | yomihon defines it for every note, so no list names it. Leaving it out says the lesson is finished; `draft: true` would say it is not |
| `slug: l04-note-language` | `[types.lesson] slug_pattern` is `^[a-z0-9]+(-[a-z0-9]+)*$` |
| `level: intermediate` | `[enums] level` offers `fundamental` and `intermediate`. There is no built-in scale — a vault that declares no `level` list accepts any word, and one that declares another list accepts only those |

The file is directly under `Lessons/`, a domain root, so it has no domain, and
nothing asks for one. The heading is deliberately not the filename here, which
is realistic and is also the commonest way a link later misses. Nothing about
it is wrong; it just means other notes link `[[L04 Say which language a note is
in]]`, not `[[Say which language a note is in]]`.

## 4. Where the file goes

`[folders] checked` is `["Concepts", "Inbox", "Lessons", "Maps", "Notes"]`.
`Lessons` is on the list, so the frontmatter rules will judge this file. Had it
gone somewhere else — `Archive`, say — every check below would have passed by
saying nothing, which is not the same as passing.

## 5. Listing it — the only step that is about the course

Writing the file is not joining the course. A course is a list, and the list is
a note — here `Notes/Reading yomihon.md`, which opens:

```yaml
---
type: study-path
created: 2026-09-22
lang: en
---
```

`type: study-path` is doing the load-bearing work, and it is worth stopping on,
because this is the gate that fails most quietly. The contract says:

```toml
[types.study-path]
role = "course"
```

Only a type holding the `course` role reads the `{sequence=…}` grammar. Had
this note said `type: note`, everything below would still be valid Markdown,
the page would still render, and the course would simply not exist —
`/courses/` would answer 404 and `check` would report nothing, because nothing
is wrong with the note. There is no diagnostic for this. You find it by the
course being absent.

One row, added to the branch that already declares itself the main line:

```markdown
## Doing it {sequence=primary}

- [[L01 Point yomihon at a folder]]
- [[L02 Add a contract]]
	- If your notes are not all in English {sequence=local}
		- [[L03 Mark a paragraph to be read aloud]]
- [[L04 Say which language a note is in]]
```

Now check what you touched, rather than the whole vault:

```sh
yomihon check --root /tmp/lab --format json \
  "Lessons/L04 Say which language a note is in.md" "Notes/Reading yomihon.md"
```

It prints nothing and exits 0.

## What that bought

The check cannot show you a projection. The server can:

```sh
yomihon serve --root /tmp/lab       # then http://127.0.0.1:9610/paths
```

That command does not return; stop it when you are done looking. Opening
*Reading yomihon* there:

- the course reads **7 items**, up from 6 — four in *The idea*, three in
  *Doing it*;
- *1 in a branch* sits beside that count, not in it: `L03` is in a
  `{sequence=local}` side branch, which has its own count and does not roll
  into the main line's;
- `L02`'s **next lesson is `L04`**, not `L03` — a side branch does not join the
  main line's prev/next, and the lesson after the one it hangs from is the next
  main-line lesson. The course lists the lesson under its heading, *Say which
  language a note is in*, because that is its title.

## Every way this goes wrong, and what check says

Each block below is one change to the finished state above, and the finding is
the real output of `check` against that change. All but one use the command
from step 5 unchanged; the exception says so, because its fault is in a third
file and a path filter cannot report a file it was not given.

**A value the contract does not allow.** `level: expert` on the lesson:

```json
{"rule_id":"schema.enum","severity":"error","path":"Lessons/L04 Say which language a note is in.md","field":"level","message":"level \"expert\" is not an allowed value", ...}
```

The list it was judged against is the contract's `[enums] level`, and the
message names the field and the value, not the list.

**A required field left out.** Delete `slug:`:

```json
{"rule_id":"schema.required","severity":"error","path":"Lessons/L04 Say which language a note is in.md","field":"slug","message":"slug is required for a lesson", ...}
```

**A field nobody declared.** Add `difficulty: medium`:

```json
{"rule_id":"schema.unknown_key","severity":"error","path":"Lessons/L04 Say which language a note is in.md","message":"frontmatter \"difficulty\" is not a known field","target":"difficulty", ...}
```

The key does not have to conflict with anything or be misspelled; it simply is
not on the list. The same finding answers `title: …` and `domain: …`, which no
note writes because yomihon derives them from the heading and the folder.

**A flag written as text.** `draft: yes`:

```json
{"rule_id":"schema.draft","severity":"error","path":"Lessons/L04 Say which language a note is in.md","field":"draft","message":"draft \"yes\" must be true or false", ...}
```

Without the finding, a flag spelled as a word would read as a finished note.

**A slug in the wrong shape.** `slug: L04 Note Language`:

```json
{"rule_id":"schema.slug","severity":"error","path":"Lessons/L04 Say which language a note is in.md","field":"slug","message":"slug \"L04 Note Language\" is not a valid slug","source_rule":"vault-schema.toml#types", ...}
```

Note `source_rule`: the anchor says the authority is the contract's `[types]`
tables, so the pattern to satisfy is written in the vault, not in yomihon.

**The row names the title instead of the filename.** Change the path's row to
`- [[Say which language a note is in]]`:

```json
{"rule_id":"map.disk_mismatch","severity":"warn","path":"Notes/Reading yomihon.md","line":24,"message":"syllabus links [[Say which language a note is in]] but it resolves to nothing", ...}
```

This is the course rule speaking, not the link rule, and that is the general
case rather than a quirk of this example: [`study-paths.md`](study-paths.md)
owns it. The same mistake in ordinary prose reports differently. Add a sentence
carrying `[[Say which language a note is in]]` to the end of
`Notes/Wikilinks in this dialect.md`, and name that third file on the command
line — the step-5 filter names two files and would report nothing about it:

```sh
yomihon check --root /tmp/lab --format json "Notes/Wikilinks in this dialect.md"
```

```json
{"rule_id":"link.title_not_alias","severity":"warn","path":"Notes/Wikilinks in this dialect.md","line":35,"message":"[[Say which language a note is in]] resolves to no filename or alias","evidence":"the target is the title of Lessons/L04 Say which language a note is in.md but not one of its aliases", ...}
```

which is a different rule for the same typing mistake, decided by where you
made it.

**A word in front of the link.** Write the row as
`- 第四課：[[L04 Say which language a note is in]]`:

```json
{"rule_id":"path.entry_noncanonical","severity":"warn","path":"Notes/Reading yomihon.md","line":24,"message":"a lesson row opens with its link; move the link to the front, or take the row out of the course","evidence":"第四課：[[L04 Say which language a note is in]]", ...}
```

This is the one worth dwelling on, because the page still looks right. The row
still reads. The link still works. But the row is no longer a lesson row, and
the course silently drops to **6 items**, and the lesson is no longer in it.
Commentary belongs after the link, or in a paragraph below the item.

**A nested list that declares nothing.** Hang an undeclared child list under
the new row:

```json
{"rule_id":"path.role_missing","severity":"warn","path":"Notes/Reading yomihon.md","line":25,"message":"this nested list never says what part it plays; declare {sequence=local} on the row that opens it, or unnest it","evidence":"a nested list carrying no declaration", ...}
```

Undeclared is unclassified, and unclassified projects nothing. Nothing is
flattened into the parent as a guess.

## The check that would have caught each of these

All of them are `warn` or `error`, so one command covers the two files you
touched:

```sh
yomihon check --root /tmp/lab --format json --deny warn --all \
  "Lessons/L04 Say which language a note is in.md" "Notes/Reading yomihon.md"
```

`--all` restores the link and course findings that touch nothing inside
`[folders] checked`; [`diagnostics.md`](diagnostics.md) says what it
restores and what no flag can. Exit 1 means
something above is true of your change; exit 0 means the note is where you
think it is, in the two files you named — a fault you introduced in a third
file is a run you have not made. Run it before you say the note is written, not
after someone notices the count is wrong.
