# The contract, and the frontmatter it lets you write

**The question this answers:** which frontmatter keys may this note carry, which
values may they hold, and who decided. yomihon decides what the keys are, and
the list is closed. The vault's contract decides which of them each type of note
may write and which values some may take, and it is different in every vault.

One file holds the contract: `<vault>/System/schemas/vault-schema.toml`. yomihon
reads it and never writes it. Read it before choosing a single key. What follows
is the list of fields, then what each of the contract's tables does to a note
you are writing, so you can read a contract you have not seen before and know
what it is asking of you.

`examples/vault/System/schemas/vault-schema.toml` in this repository is a small
working one, written to be copied and then cut down.

Before any table: the file's first line is `schema_version = "2"`, and it is
required. A contract without it is refused whole, and `"2"` is the only value
this yomihon accepts. A contract written for another format is refused the same
way, and so is a `yomihon` built before this one when it reads a `"2"` contract.
`check` says only that it could not use the contract; `yomihon serve` logs the
reason:

```
level=WARN msg="vault contract unavailable"
  error="decode vault contract: unsupported schema_version \"1\"; supported version is \"2\""
```

## The fields yomihon defines

The list is closed. A key outside it is `schema.unknown_key`, an error, even
when nothing would have read it, and there is no harmless extra key.

| Field | What it is, and what yomihon does with it |
|---|---|
| `type` | which of the contract's note types this is. A type the contract does not declare is `schema.enum`. Always allowed |
| `aliases` | further names a `[[link]]` resolves against. Always allowed, and a name two notes share is `collision.alias` |
| `draft` | the boolean `true`, which says this note is still being written. Always allowed; see below |
| `based_on` | the note or notes this one is written from, quoted: `["[[Note]]"]`. A reference that reaches nothing is `provenance.unresolved` |
| `lang` | the language of the note's own text, a BCP 47 tag such as `zh-Hant` or `ja`; a malformed one is `schema.language` |
| `created` | the day the note was made, such as `2026-11-12`. It dates a report and orders the thoughts `left-open` lists |
| `slug` | a lesson's stable key. The type that holds the `lesson` role has to write it; the type's `slug_pattern`, if it states one, shapes it |
| `level` | how much a lesson assumes. When `[enums] level` lists values, it is one of them |
| `prerequisites` | allowed in the list; no page shows it and no command checks it yet |
| `verified_on` | the author's own date of checking the note; allowed, and read by nothing yet |
| `verified_with` | what the note was checked against; allowed, and read by nothing yet |

Obsidian's `tags` and `cssclasses` are let through and never judged, and
`aliases` is one of its names too. `type`, `draft` and those three need no
listing. Every other field has to be listed by the contract before a note may
write it: in `[fields] common` for any note, or in a type's own `fields` for
that type alone. `created` and `lang` also have to be in `[fields] common`
before yomihon reads them at all, because a declaration on one type cannot
describe every note.

A contract that lists a name outside this table in a field list is refused whole
and says so in the log:

```
level=WARN msg="vault contract unavailable"
  error="decode vault contract: fields.common: \"nosuchkey\" is not a field yomihon defines (the fields are \"type\", \"aliases\", \"draft\", \"based_on\", \"lang\", \"created\", \"slug\", \"level\", \"prerequisites\", \"verified_on\", \"verified_with\"; title and domain are derived)"
```

### Two things a note does not write

A note writes no `title` and no `domain`; writing either is
`schema.unknown_key`.

- **The title** is the note's first `#` heading, or its file name without the
  extension when it has none. A `#` line inside a code fence, a comment
  (`%% … %%`, `<!-- … -->`) or a `$$` math block is not a heading.
- **The domain** is the first folder below a root that `[folders] domain_from`
  names: `Concepts/go/Channel.md` is in `go` when `Concepts` is a root. A note
  directly under a root, or outside every root, has no domain and nothing is
  reported about it. `[enums] domain` may list the folder names a root's
  folders are allowed to have.

### `draft`

`draft: true` is the author's statement that the note is not finished. The
boolean is the whole of it: a note that writes no `draft` is finished, and
`draft: false` says the same, so a finished note never needs the field. It is
not a type; an unfinished lesson is `type: lesson` with `draft: true`.

- The note's own page, the sidebar and the course row that reach it carry a
  quiet word, `Draft`, and a finished note carries nothing. No button changes it.
- Search finds the note as usual, and `draft:true` or `draft:false` filters.
- Links and backlinks work as for any note.
- `check` judges only the type of value: anything but `true` or `false`, such as
  `"yes"`, is `schema.draft`.
- `yomihon left-open` lists a thought that still carries it.
- `map.disk_unlisted` never reports a draft lesson.

yomihon never writes a note, `draft` included. Taking the line out, or putting
it back, is done in an editor. Nothing else records how far along a note is.

## The tables, and what each one costs you

### `[folders]` — where the notes are

| Key | What it does |
|---|---|
| `checked` | the directories that hold knowledge rather than machinery, the **knowledge layer**, a phrase the diagnostics and the `--all` flag both use. The frontmatter rules judge only files inside it, and `check` drops findings that touch nothing inside it unless you pass `--all`. An omitted or empty list is not the empty set: it means every file is inside. Each entry is one top-level directory name |
| `ignored` | directories the scan does not enter at all, as vault-relative patterns in `path.Match` syntax: `["courses/*/lab"]` excludes every course's `lab` directory, and a `*` never crosses a `/`. An ignored directory is treated like a dot-directory such as `.git`; see below |
| `templates` | directories of shapes to copy rather than notes. A note there is rendered like any other and is never a lesson, a course, a map or a reader's thought |
| `journal` | one directory whose files fill the sidebar's journal rail. It reads no frontmatter, so an untyped file below it is eligible |
| `domain_from` | the roots a note's domain is read below. The roots may not overlap |
| `skipped_files` | file names that are never read as notes, such as `README.md` |
| `frontmatter_optional` | whether a note with no frontmatter block is legal |

A directory named in `checked` that the vault does not actually have is
`schema.unmatched_knowledge_dir`, an error against the contract rather than
against any note: the guard against a typo silently switching the rules off.

`skipped_files` is an exact match, and unlike almost everything else in this
dialect it is not case-folded. Such a file still has a page of its own —
`/notes/<path>` and `/raw/<path>` both answer 200 — but it is not a note: it
answers no `[[link]]` (`[[README]]` in a vault that skips `README.md` reports
`link.broken`), it is not in the folder listing, its words are not in the search
index, and `exists` does not know it.

An ignored directory has nothing below it as a note or a resource: it is not in
the folder listing, not searched, not judged and cannot collide on a name, and
`/notes/` and `/raw/` answer 404 for it. A Markdown path link into it is left as
written rather than reported broken; a `[[wikilink]]` to a name that only exists
there is broken, as it is for a dot-directory. Use it for generated or copied
trees whose file names repeat. A pattern must be relative and clean (no leading
`/`, no `..`, no trailing `/`) and must parse; anything else makes the contract
fail to load.

A template directory changes what the server projects for the notes in it: no
lesson affordance, such as read-aloud, is added; they stay out of the courses,
maps and shelf of what the reader left open; the `type:`, `draft:`, `domain:`
and `slug:` search filters skip them, though their words are searched; and a map
entry pointing into one is dropped from the map.

`frontmatter_optional` has a third state that matters. Omitted, a note without
frontmatter is fine. Written `true`, fine. Only writing it `false` makes a
missing frontmatter block a fault. A block that is present and unparseable is
`schema.frontmatter` either way, and nothing else about that note can be judged
until it parses.

`check` does not apply YAML merge keys: a top-level `<<` is
`schema.unknown_key`, and fields supplied only through a merge are not seen. A
required field supplied only by a merge therefore gets `schema.required`. The
reading page may still show merged values, or say that the frontmatter uses a
form it cannot read. Write every field out directly.

A block that never closes is not a block to any reader: its opening `---` is
read as body text and its keys as prose. When line 1 is `---`, line 2 reads as
`key:`, and no closing `---` or `...` follows, `check` says so under
`schema.frontmatter` with the message `frontmatter opens on line 1 and never
closes`, whatever `frontmatter_optional` says. Written `false`, this finding
takes the place of `frontmatter is missing` for such a note, because the block
is there and only its closing line is not. A note that opens with a thematic
break followed by anything else is not reported.

### `[enums]` — the vocabulary

Flat lists of legal values, one per field: `domain` and `level` are the ones the
example uses, and any field in the table above may have one. None of them has a
built-in vocabulary — there is no scale `level` is measured against; the words
your contract writes are the whole answer, and a value outside the declared list
is `schema.enum`, an error. For `domain` there is no frontmatter to fix, because
the note's domain is its folder's name: move the note into a folder named for a
declared domain, or add the name to the list. An omitted or empty list
constrains nothing at all, so a contract that declares no `domain` list accepts
any domain, and silence there is permission rather than absence.

A note type needs no entry here: every `[types.<name>]` table declares one, and
`[enums] type` may add types that need no table.

### `[fields]` — which keys may appear at all

| Key | What it does to your note |
|---|---|
| `common` | the fields any note may carry, chosen from the table above. A note carrying a field that neither this list, nor its type's own `fields`, nor the always-allowed names cover is `schema.unknown_key`. **An omitted `common` is the empty set,** not permission: a contract with no `[fields]` table lets a note carry only `type`, `draft`, Obsidian's names and whatever its type's own `fields` adds |
| `required` | every key here must be present on every note. "Present" means a non-empty scalar or a non-empty list, so a required field written as a one-item list counts. A key named here that `common` does not list refuses the contract |

### `[types.<name>]` — what a type of note is

One table per type, in the order a reader meets them. A type with nothing to add
needs only its name.

| Key | What it does |
|---|---|
| `role` | what the type does in the reader: `lesson`, `course`, `map` or `thought`. A type with no role is a plain note |
| `fields` | the further fields only this type may carry. This is permission, not obligation. While a non-empty scalar `type` is undeclared, these keys are left unjudged and reported once as `schema.type_dependent` until the type is valid |
| `required` | the fields every note of this type must carry, beside the vault-wide `required`. Each must be one the type may carry |
| `slug_pattern` | a regular expression a slug must match, for a type that lists `slug`. Optional; a note that breaks it is `schema.slug` |

The roles:

- `course` — the type's notes are study paths, read with the `{sequence=…}`
  grammar. This is the declaration behind the first silent gate in front of the
  grammar, which [`study-paths.md`](study-paths.md) explains: a note whose type
  holds no `course` role projects no course and reports nothing.
- `map` — the type's notes are maps, which [`maps.md`](maps.md) explains. The
  same silent gate stands in front of them.
- `lesson` — the notes a course lists. At most one type holds it, and the type
  must list `slug`, which each of its notes then has to write: a lesson without
  one is `schema.required` with the message `slug is required for a lesson`.
- `thought` — a reader's own notes that cite a source, the ones the "What you
  left open" shelf and `left-open` list when they carry `draft: true`. At most
  one type holds it, and the type must list `based_on`.

A role that is none of the four, or a second type holding `lesson` or `thought`,
does not refuse the contract. It closes the courses and the maps: `check`
reports `schema.navigation_unavailable` and the startup line reads `paths=0
maps=0`. A type name `concept` is yomihon's, with the concept sheets and the
coverage report; your contract decides whether such a type exists, never what it
is called.

### `[rules]` — the checks with their own rule ids

| Key | What it checks |
|---|---|
| `concept_requires_provenance` | the fields a note of the `concept` type — an idea distilled from somewhere else — must have at least one of, so it says where it came from. A concept may write these fields without listing them, and the list may not be empty when a `concept` type exists. The finding's message is a frozen sentence naming two particular fields; the list actually demanded is whichever your contract wrote here |
| `planned_gap_marks` | the heading words that mark a section as owed rather than missing; a word must begin the heading or follow a space or punctuation mark, and what comes after it does not matter (`## 缺口帳` matches, `## 產品化缺口與事故` does not) |
| `planned_inline_marks` | the same for a single line |

The list under `concept_requires_provenance` may name fields beside the ones
yomihon defines, but not one that is built in or derived (`type`, `draft`,
`tags`, `aliases`, `cssclasses`, `title`, `domain`) or that yomihon no longer
defines: the contract is refused at load, because a concept would be let write
it with no finding.

The last two keys are worth knowing exist, because a contract that omits them is
loaded with defaults and they then look like product behaviour.
[`names-and-links.md`](names-and-links.md) has the default words and what they
do to a broken link; what this table adds is that they are yours to change, and
that writing an empty list turns the mechanism off.

### `[privacy]` — the one omission that stops the tooling dead

`never_egress_dirs` names directories nothing may quote back out. A note under
one of them is never reported by `check` or `coverage`, not even with `--all`,
and naming a withheld path on the command line is refused rather than answered.
`exists` is the exception and the one worth reading before you script against
it — [`diagnostics.md`](diagnostics.md) owns exactly what it answers there. None
of this binds the reading pages: a person at this machine opens such a note the
way they open any other.

The behaviour to know before you debug anything: **this section is fail-closed
and it is not optional in practice.** A contract with no `[privacy]` section has
granted no permission, so `check`, `coverage` and `exists` all refuse to run at
all, and exit 2. If your commands are refusing on a vault that otherwise looks
healthy, look here first — and then at the rest of the contract, because every
other way a contract can fail to load prints the same line. The startup log
names this one outright:

```
level=WARN msg="vault contract policy unavailable" capability=privacy
  reason="contract declares no privacy policy; agent-facing output disabled until it does"
```

## Keys the example never shows

A contract carrying a key outside these tables is refused whole, `[privacy]`
excepted: an unknown key there leaves only the privacy policy unusable. Several
optional keys are not in the example contract, and they are the ones a reader of
it would never learn exist: `[folders] ignored`, `skipped_files`,
`frontmatter_optional` and `journal`; a type's `slug_pattern`; and the two
`planned_*_marks` keys above, which change behaviour even when omitted.
