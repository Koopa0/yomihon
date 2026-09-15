# Every diagnostic, and what to do about it

**The question this answers:** `yomihon check` reported something — what is it
telling me, whose rule is it, and what is the repair?

There is one authority for this and it is the command, not this page. Where the
two disagree, believe the command. What this page adds is the whole set in one
place: a finding you can look up is a finding you can fix. A rule id the
command emits that you cannot find here should be impossible — a test in the
repository holds this folder's ids to the emitted set in both directions — so
if it happens, this page is stale and the test is not running.

## Running it

```bash
yomihon check --root <vault> [--format json|human|md] [--all]
              [--deny <severity|rule-id>]... [--baseline <file>] [path...]
```

| | |
|---|---|
| `--root` | the vault to judge. Without it, the folder you are standing in is the vault |
| `[path...]` | narrows the judging to part of that vault. Written the way the vault spells it, relative to the root — `Notes`, or `Notes/topic.md`. An absolute path is not a second way to say `--root`, and is refused |
| `--format` | `json` is one compact object per line, the machine format. `human` is a terminal summary grouped by domain. `md` is a fileable report body — it opens by saying whether this vault's contract would accept it as a note. Without the flag, a pipe gets `json` and a terminal gets `human` |
| `--deny` | a severity (`error`, `warn`, `info`) or an exact rule id, repeatable. A severity is a **threshold**, not a match: `--deny warn` fails on warnings and on errors, and `--deny info` fails on anything at all. A value that is neither is refused rather than ignored |
| `--all` | restores findings that touch nothing inside `[scan] knowledge_dirs`. It changes what is **reported**, never what is judged — see below |
| `--baseline` | a previous run's JSONL, subtracted by `fingerprint`, so only new findings are reported and gated. You write one by keeping a `--format json` run: `yomihon check --root <vault> --format json > baseline.jsonl`. It is version-locked; see the refusals below |

Exit codes: **0** nothing named by `--deny` was found · **1** a `--deny` gate
hit · **2** the command could not run. Findings alone never fail it — without
`--deny`, `check` reports and exits 0, which is why a green exit is not by
itself evidence of a clean vault.

### What `--all` does, and the thing it cannot do

The knowledge layer cuts twice, and the two cuts are not the same:

- **The frontmatter rules — every `schema.` rule — judge only files inside
  `[scan] knowledge_dirs.`** A note outside it is never judged by them, and
  `--all` restores nothing, because nothing ran. Put an invented key, a status
  its type cannot hold and a missing `slug` on one lesson, file it under
  `Archive/`, and `check` prints nothing about it with or without `--all`; move
  the same bytes into `Notes/` and four errors appear. There is no flag for
  this. The repair is to file the note where the rules reach.
- **The link, collision and course rules run over the whole vault**, and it is
  only their *reporting* that the layer gates. A dead `[[link]]` or an
  undeclared branch in that same `Archive/` note is found either way, dropped
  from the default report, and restored by `--all`.

So `--all` on a gating run is worth passing — it is the difference between
seeing a course fault outside the layer and not — but it is never a substitute
for the note being somewhere the schema rules reach. Silence over a file
outside the layer is not a pass, and no flag makes it one.

Every way the command refuses rather than answers exits 2 and writes **nothing
at all on stdout**, with the reason on stderr. A check that reads the output
instead of the exit code therefore scores each of these as a clean pass, which
is why the entry point's probe reads `$?`:

| Refusal | Why |
|---|---|
| no `System/schemas/vault-schema.toml` | the folder has declared no vocabulary to judge against |
| a contract yomihon could not use | permission to report is positive authority, and a contract it could not load has granted none. This disables `check`, `coverage` and `exists` for the whole vault |
| a `[path...]` inside a `[privacy] never_egress_dirs` directory | that ground is scanned, but nothing from it may be reported, and an empty answer would read as a clean verdict |
| a `[path...]` written as an absolute path | a filter names part of the vault from the vault's own root; the vault itself goes after `--root` |
| a `--deny` value that is neither a severity nor a real rule id | a typo fails loudly instead of quietly disabling the gate |
| a `--baseline` file written by another fingerprint version | a fingerprint carries its algorithm version as a prefix, currently `v1:`, and subtracting across versions would silently under-subtract. The message names the offending line |

The second row is the one to expect on a vault that otherwise looks healthy,
and it is wider than it looks. It prints

```
yomihon: privacy authority unavailable; agent-facing output disabled
```

for **any** contract that did not load — a missing `[privacy]` section is
merely the commonest, and a `[fields] required` naming a key outside
`[fields] known` reads exactly the same. It deliberately says no more, because
the reason would quote the contract back out under the very policy that is
missing. One command tells you which:

```bash
yomihon serve --root <vault>
```

The reason is on the page, and in a startup log line that names it outright:

```
level=WARN msg="vault contract unavailable; write face is closed (fail-closed)"
  error="decode vault contract: fields.required: value \"nosuchkey\" is not listed in fields.known"
```

## The shape of one finding

Eight keys are on every finding:

`rule_id` · `severity` · `path` · `message` · `evidence` ·
`suggested_action` · `source_rule` · `fingerprint`

Five more appear only when the finding has them, which means their absence
carries information:

| Key | When it is there |
|---|---|
| `line` | the finding points at a body line, 1-based as an editor counts |
| `field` | a frontmatter key is at fault |
| `target` | the original link or value text, kept structured so nothing has to parse the prose |
| `resolved_to` | the target did resolve, and this is the path it reached — a fragment or listing fault rather than a dead name |
| `collision_members` | every path involved in a name or alias collision, so one finding describes the whole of it |

`source_rule` says which artifact the rule was read out of, and is worth
reading before arguing with a finding:

| Value | Meaning |
|---|---|
| `vault-schema.toml` | the finding came out of reading the contract's frontmatter declarations |
| `vault-schema.toml#rules` | its `[rules]` table |
| `vault-schema.toml#scan` | its `[scan]` table |
| `vault-schema.toml#supersession` | its `[supersession]` table |
| `yomihon` | the product's own dialect — link resolution, collisions, reference liveness, the study-path grammar. No vault artifact declares these |

A finding that says `yomihon` is never one you can argue with by editing the
contract. The reverse does not hold as neatly, and two rules are the reason:
`schema.frontmatter` and `schema.language` both carry `vault-schema.toml`, and
neither is a value your contract chose — one says the YAML did not parse and
the other that `lang` is not a well-formed BCP 47 tag. What they share with the
rest of the table is where they were reached from, not who set the bar.

## Severity

Every `schema.` rule is an `error`; nothing else ever is. One rule is always
`info` — `callout.title_markup`, a formatting observation. Everything else is a
`warn`, except that three rules *drop* from `warn` to `info` when the vault has
declared the name owed rather than missing: `link.broken`, `link.broken.path`
and `map.disk_mismatch`. `link.broken.path` has a second reason to sit at
`info`: a path that climbs out of the vault root is reported rather than judged,
because what is there depends on the machine.

The consequence for gating: `--deny warn` catches a broken link and does not
catch one marked as a planned gap. That is intended, and it is also the
dialect's sharpest trap; [`names-and-links.md`](names-and-links.md) owns how a
gap is declared and how a heading can declare one by accident.

## The frontmatter rules

All eleven are `error`. They judge only files inside `[scan] knowledge_dirs`; a
file outside them is still read and still linked, so silence there is not a
verdict.

| `rule_id` | What it means | `source_rule` |
|---|---|---|
| `schema.required` | a key `[fields] required` names is absent. `field` says which | contract |
| `schema.unknown_key` | a key outside `[fields] known` | contract |
| `schema.enum` | a value outside the list declared for this note's type. Read the message's own wording: it names the list it judged against | contract |
| `schema.status_unreachable` | this note's status is in its type's enum, but no `[[lifecycle]]` row carrying that status applies to its type, so nothing could ever have moved it there. It is a finding about the note that carries the value, not about the contract that declares it: a status no note uses is never reported | contract |
| `schema.frontmatter` | the frontmatter is not valid YAML. Everything else about the note is unjudgeable until this is fixed | contract |
| `schema.language` | `lang` is not a valid BCP 47 tag | contract |
| `schema.slug` | the slug does not match `[rules] slug_pattern` | `#rules` |
| `schema.domain_folder` | the note's `domain` disagrees with the folder it sits in, under a root `[rules] domain_equals_folder_under` names | `#rules` |
| `schema.legacy_tag` | a tag carrying a slash, under `[rules] forbid_tag_with_slash` — a property written in the wrong place | `#rules` |
| `schema.provenance` | a note of the `concept` type carries none of the provenance fields `[rules] concept_requires_provenance` names. Its message is a frozen sentence, `frontmatter concept has neither based_on nor source_locator`, and those two words are in the message rather than in your contract: the fields actually demanded are whichever your contract lists, and `source_locator` is not even a legal key in the vault this repository ships. Read the contract, not the message | `#rules` |
| `schema.unmatched_knowledge_dir` | `[scan] knowledge_dirs` names a directory this vault does not have, so the frontmatter rules reach nothing there. The fault is in the contract, not in a note | `#scan` |

## The link and name rules

All `warn` unless noted. One exception is worth knowing before you go looking
for a finding that never comes: a `[[link]]` yomihon accepted as a **study
path's lesson row** is judged by `map.disk_mismatch` below instead of by any of
these, even when the target is exactly some note's title. Every other link in
that same note is judged here as usual.

| `rule_id` | What it means |
|---|---|
| `link.broken` | the target matches no filename, path or alias anywhere. Drops to `info` under a planned gap |
| `link.title_not_alias` | the target is some note's frontmatter `title`, and titles are not keys. The message names the note you meant |
| `link.section_missing` | the note resolved; no heading answers the text after `#`. `resolved_to` says which note |
| `link.block_missing` | the note resolved; no line carries the `^address` |
| `link.broken.path` | a plain path reference to a file that is not in the vault. Drops to `info` under a planned gap |
| `embed.section_missing` | an `![[…#Heading]]` embed whose heading is not there, so there is no excerpt to cut. Nothing is shown; the block names the address that failed |
| `embed.block_missing` | the same for `![[…#^id]]` |
| `collision.name` | two or more files share one resolution name, so every `[[link]]` to it is ambiguous and nothing is linked. `collision_members` lists them all. Rename one, or link each by its full vault-relative path |
| `collision.alias` | two notes declare the same alias. Give the alias one owner |
| `provenance.unresolved` | a frontmatter reference field points at nothing — `based_on` or `related` (`source_rule` `yomihon`), or one of the fields `[supersession]` configures (`#supersession`). A lesson slug counts as resolving |

## The course rules

Every rule below judges a **study path**, the two whose names open with *map*
included; [`study-paths.md`](study-paths.md) explains that naming. All `warn`.

| `rule_id` | What it means |
|---|---|
| `map.disk_mismatch` | a study path lists an entry that resolves to nothing — a course promising a note that is not there. Drops to `info` under a planned-gap heading, which is how you say "owed, not missing" |
| `map.disk_unlisted` | a lesson exists on disk that no study path of its domain lists. Narrower than it sounds: it runs **only** for a study path that itself declares a `domain`, only over lessons carrying that same domain, and never over a lesson still carrying the status your lifecycle starts a lesson at — whichever word that is, written as `initial = true` or inferred from the row whose `from` is empty or wildcard, which [`frontmatter.md`](frontmatter.md) explains. Nobody has offered that lesson yet, so there is nothing for a path to have listed. A lesson whose status has no starting row for its type — a word your contract never declares included — started nowhere and is reported like any other. A vault whose paths declare no domain — which is normal, since a course usually spans subjects — never sees this rule at all |
| `path.role_missing` | a branch lists lessons, or a nested list exists, and neither says what part it plays. Undeclared is unclassified, and unclassified projects nothing. Two messages, one per shape: a heading's, and a nested list's |
| `path.role_invalid` | the marker's value is none of `primary`, `local`, `none` |
| `path.role_duplicate` | one line declares more than one role |
| `path.role_conflict` | a branch declares itself part of the course while the branch above it declared itself out of it |
| `path.role_nested_primary` | a `primary` branch sits inside a `local` one, which has no place in the main line's order |
| `path.nesting_too_deep` | a side branch inside a side branch |
| `path.local_orphan` | a side branch with nothing to hang from — it was not nested under a lesson |
| `path.role_on_entry` | one row tries to be both a lesson and a branch heading. Give the branch its own row above the list it opens |
| `path.role_misplaced` | a marker written somewhere it declares nothing — in a paragraph, say, or on an H1, which opens no branch. It is read only on a heading from H2 to H6, or on a row that opens a list |
| `path.entry_noncanonical` | the row's `[[link]]` is not the first visible thing after the list marker. The row still reads and the link still works; it is simply no longer a lesson row, and the course count drops without anything looking broken |
| `path.entry_multi_target` | one row names two notes, so it does not say which lesson it is |
| `path.entry_outside_branch` | a lesson row sitting above every branch-opening heading, so it belongs to no part of the course |

## The rest

| `rule_id` | Severity | What it means |
|---|---|---|
| `supersession.predecessor_not_archived` | warn | the successor ledger says this note was replaced, but its status is not the archived one. Archive it, or clear the field until the replacement is authoritative |
| `supersession.archived_navigation_target` | warn | a live path or map links a note whose status is archived |
| `scan.skipped` | warn | the scan saw a path and read nothing from it, because a note is read only out of a regular file. A symbolic link is the usual cause: it holds no note, answers no link, and appears in no listing |
| `callout.title_markup` | info | a recognised callout's title carries markup — a wikilink, an HTML tag, emphasis, a code span, a markdown link or an image. Titles are escaped, not parsed, so it would render as visible text. Move it into the body |

## Two other commands, for completeness

`yomihon coverage` reports how much of the vault's `concept` material is
reachable from its maps, counting only inside the knowledge layer, and never
gates: it exits 0 whatever it finds, and 2 only if it could not run. A map
filed outside that layer does not count towards it.
[`maps.md`](maps.md) owns what the three mount states mean and how to move one.

`yomihon exists [--root <vault>] [--format json|human|md] <name>` exits 0 when a
note for the name exists and 1 when none does, so a write-if-absent can gate on the exit code — but on **0 against 1**,
not on zero against everything else. `exists` refuses like the other two, and a
refusal exits **2**:

```sh
yomihon exists --root <vault> "$name"; case $? in
  0) : ;;                       # it is there, write nothing
  1) write_the_note ;;          # it is not there
  *) exit 2 ;;                  # the command could not answer; do not decide
esac
```

`exists "$name" || write_the_note` is the shape to avoid: it takes the write
branch on a refusal, so a vault whose contract merely stopped loading grows a
second note under a name that already exists.

A note inside a `[privacy] never_egress_dirs` directory is the case to get
right, and it is the one place a withheld note still answers. It is never
*described* — no path, no matched field — but the **exit stays 0** and the JSON
carries `"withheld":true` instead of a match, so a script gated on the exit
code does not go on to create a second note under a withheld note's own name:

```
$ yomihon exists --root <vault> --format json 2026-08-30
{"query":"2026-08-30","matches":[],"withheld":true}
$ echo $?
0
```

Run that against a name under one of your own withheld directories before you
write any script that depends on it. `check` and `coverage` make the opposite
trade — they never report a withheld note at all, not even with `--all` — which
is why this one command has to be described separately rather than lumped in
with them.
