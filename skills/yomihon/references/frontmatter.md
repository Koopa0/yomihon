# The contract, and the frontmatter it lets you write

**The question this answers:** which frontmatter keys may this note carry, which
values may they hold, and who decided — because the answer is never "yomihon",
and it is different in every vault.

One file decides: `<vault>/System/schemas/vault-schema.toml`. yomihon reads it
and never writes it. Read it before choosing a single key. What follows is what
each of its tables does to a note you are writing, so you can read a contract
you have not seen before and know what it is asking of you.

`examples/vault/System/schemas/vault-schema.toml` in this repository is a small
working one, written to be copied and then cut down.

## The tables, and what each one costs you

### `[enums]` — the vocabulary

Flat lists of legal values: `type`, `domain`, `source_kind`, `source_provider`,
`level`, `map_kind`. None of them has a built-in vocabulary — there is no scale
`level` is measured against and no set of kinds a map has to be one of; the
words your contract writes are the whole answer, and a value outside the
declared list is `schema.enum`, an error. An omitted or empty list constrains
nothing at all, so a contract that declares no `domain` list accepts any domain,
and silence there is permission rather than absence.

`[enums.status]` is different in shape: a map of *group name* to list of
statuses. Every contract has to declare a group called `note`; a contract
without one is refused whole, and a refused contract means the folder is read
as though it carried none.

### `[fields.status_group]` — which status list applies to your type

This is the table that catches people. It maps a group name to the types it
governs. A type it does not name is judged against the `note` group. So a
status can be perfectly legal for one type and an error on another, and the
message says which: `status "published" is not a valid lesson status`, not
`is not a valid status`.

### `[fields]` — which keys may appear at all

| Key | What it does to your note |
|---|---|
| `known` | the keys any note may carry. A key on neither this list nor `lesson_only` is `schema.unknown_key`, an error. There is no harmless extra key — including keys other yomihon capabilities read, like `topics` for the `topic:` filter or `domain` for `domain:`. Writing a field is not the same as opening the capability; the contract has to declare it too |
| `required` | every key here must be present. "Present" means a non-empty scalar or a non-empty list, so a required field written as a one-item list counts |
| `lesson_only` | keys only a lesson **may** carry. This is permission, not obligation: nothing here becomes mandatory by being listed, and any non-lesson carrying one gets `schema.unknown_key` |
| `required_inbox` | for a note whose type is `inbox`, this list **replaces** `required` entirely — it is not a delta on top of it |
| `domain_exempt_types` | types excused from carrying `domain`, and only `domain`. A course and a map usually span subjects, which is what this is for |

Three consequences worth holding on to.

First, **`known` is not the whole permitted vocabulary** — the keys a lesson may
carry are `known` plus `lesson_only`. In `examples/vault`, `slug` and `level`
appear only under `lesson_only`, and a lesson carrying both passes `check`
cleanly.

Second, `required` must be a subset of `known`, and `known` and `lesson_only`
may not overlap. Both are enforced by refusing the contract outright, which is
worth knowing because a refused contract does not say so in `check` — it
reports the generic refusal [`diagnostics.md`](diagnostics.md) describes, and
you get the real reason from `yomihon serve`.

Third, a lesson is required to carry `slug` regardless of any of this: that
demand is yomihon's, not the contract's, and it fires as `schema.required` with
the message `slug is required for a lesson`. A contract that forgets to list
`slug` under `lesson_only` therefore makes every lesson wrong twice — missing
it is an error, and writing it is an unknown key.

### `[rules]` — the checks with their own rule ids

| Key | What it checks |
|---|---|
| `slug_pattern` | a regular expression a lesson's `slug` must match. Required as soon as `[enums] type` contains `lesson`, since nothing else would say what a slug may look like |
| `domain_equals_folder_under` | directory roots under which a note's `domain` must equal the folder it sits in |
| `concept_requires_provenance` | the fields a note of the `concept` type — an idea distilled from somewhere else — must have at least one of, so it says where it came from. The finding's message is a frozen sentence naming two particular fields; the list actually demanded is whichever your contract wrote here |
| `forbid_tag_with_slash` | when true, a tag containing `/` is a property written in the wrong place |
| `planned_gap_marks` | the heading words that mark a section as owed rather than missing |
| `planned_inline_marks` | the same for a single line |

The last two are worth knowing exist, because a contract that omits them is
loaded with defaults and they then look like product behaviour.
[`names-and-links.md`](names-and-links.md) has the default words and what they
do to a broken link; what this table adds is that they are yours to change, and
that writing an empty list turns the mechanism off.

### `[scan]` — which files are held to all of the above

`knowledge_dirs` names the directories that hold knowledge rather than
machinery — the **knowledge layer**, a phrase the diagnostics and the `--all`
flag both use for it. The frontmatter rules judge only files inside it, and
`check` drops findings that touch nothing inside it unless you pass `--all`.
An omitted or empty list is not the empty set: it means every file is inside.

A directory named here that the vault does not actually have is
`schema.unmatched_knowledge_dir`, an error against the contract rather than
against any note — the guard against a typo silently switching the rules off.

`skip_basenames` names filenames that are never read as notes. The match is
exact, and unlike almost everything else in this dialect it is not case-folded.
Such a file still has a page of its own — `/notes/<path>` and `/raw/<path>` both
answer 200 — but it is not a note: it answers no `[[link]]` (`[[README]]` in a
vault that skips `README.md` reports `link.broken`), it is not in the folder
listing, its words are not in the search index, and `exists` does not know it.

`no_frontmatter_is_legal` has a third state that matters. Omitted, a note
without frontmatter is fine. Written `true`, fine. Only writing it `false`
makes a missing frontmatter block a fault. A block that is present and
unparseable is `schema.frontmatter` either way, and nothing else about that
note can be judged until it parses.

### `[navigation]` — what can be a course or a map

`path_types` and `map_types` name the types that become study paths and maps.
This is the declaration behind the first silent gate in front of the study-path
grammar, which [`study-paths.md`](study-paths.md) explains, and in front of maps,
which [`maps.md`](maps.md) does: a type that is not listed here projects nothing
and reports nothing.

If the table is present, both keys are required. A fault in either closes both,
and so does naming a type on both lists, or naming a type `[enums] type` does
not declare, or omitting the table. Each of those leaves the vault with no
study paths and no maps at all, and the way to see it is the startup line
reading `paths=0 maps=0`.

`journal_dir` names a single directory whose files fill the sidebar's journal
rail. It reads no frontmatter, so an untyped file below it is eligible.

### `[artifacts]` — shapes to copy, rather than notes under a lifecycle

`non_instance_dirs` names the directories holding templates — shapes to copy
rather than notes under a lifecycle. A note there is rendered like any other,
but where an ordinary note's page offers status buttons its page says it is
outside lifecycle governance, a `read-aloud` marker in it does nothing, and a
map entry pointing into one of these directories is dropped from the map.

A contract with no `[artifacts]` section still loads, and this is the second
omission that closes a capability rather than failing. It says so at startup:

```
level=WARN msg="vault contract policy unavailable" capability=artifact
  reason="contract declares no artifact policy; instance projections disabled until it does"
```

and the projections it disables include the study paths and the maps — the same
startup line that read `paths=2 maps=5` with the section present reads
`paths=0 maps=0` without it, on an otherwise identical vault.

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
other way a contract can fail to load prints the same line.

### `[supersession]` — the replacement ledger

`predecessor_field`, `successor_field`, `general_link_field` and
`archived_status`. yomihon reports on the pair and moves nothing. The first two
must be lesson-only and all three must differ, so a lesson keeps both sides of
its own history while any other note carries only the field naming what it
replaced. A reference in any of them that resolves to nothing is
`provenance.unresolved`.

### `[[lifecycle]]` — how a status may move

One row per status, each carrying `status`, `applies_to`, `initial`, `from` and
`owner`. `*` means every declared type in `applies_to`, and any current status
in `from`; where it appears it must be the only value.

`initial` is all-or-nothing across the rows: write it on every row or on none.
Written on none, it is inferred — a status is initial when its `from` list is
empty or wildcard. Written on some, the contract is refused, because the rows
that stayed silent are the ones a reader would have to guess about.

`owner` lists are declarative data. They gate nothing, and no code outside the
contract reader consults them; they record, for your own tooling's doctrine,
whose step a status's onward move is.

A note carrying a status that is legal in its type's enum but has no lifecycle
row applying to that type gets `schema.status_unreachable`: nothing could ever
have moved it there. The finding needs a note that actually carries the value —
a status declared and never used is not reported.

## The one field yomihon writes, and the one it refuses

`status` is the only frontmatter yomihon writes, and it writes only the value —
every other byte of the note, including the rest of that line, stays identical,
so a comment you left beside it and the quoting you chose both survive. A
transition has to be legal in `[[lifecycle]]`.

`published` is the exception it will not make. The value records a publication
that happened outside the vault, and nothing here can attest to one, so the
control is never offered and a request for it is refused. A note carrying the
value renders like any other; it is set by hand.

## Keys the parser accepts that the example never shows

`aligned_with` and `generated_at_must_match` are accepted at the top level so a
contract carrying them decodes, and are read by nothing. The two
`planned_*_marks` keys above are the ones that actually change behaviour, and
the ones a reader of the example contract would never learn are configurable at
all.
