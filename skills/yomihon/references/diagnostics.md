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
| `--all` | restores findings that touch nothing inside `[folders] checked`. It changes what is **reported**, never what is judged — see below |
| `--baseline` | a previous run's JSONL, subtracted by `fingerprint`, so only new findings are reported and gated. You write one by keeping a `--format json` run: `yomihon check --root <vault> --format json > baseline.jsonl`. It is version-locked; see the refusals below |

Exit codes: **0** nothing named by `--deny` was found · **1** a `--deny` gate
hit · **2** the command could not run. Findings alone never fail it — without
`--deny`, `check` reports and exits 0, which is why a green exit is not by
itself evidence of a clean vault. A file `check` could not read is one of those
findings and not a reason it could not run, so it exits 0 too unless a `--deny`
covers it — its own id, or any of the three severities, since it is an `error`.
`coverage` and `exists` still refuse with 2, because their whole answer is a
verdict about the vault entire.

### What `--all` does, and the thing it cannot do

The knowledge layer cuts twice, and the two cuts are not the same:

- **The frontmatter rules — every `schema.` rule — judge only files inside
  `[folders] checked.`** A note outside it is never judged by them, and
  `--all` restores nothing, because nothing ran. Put an invented key, a `level`
  its contract does not list and a missing `slug` on one lesson, file it under
  `Archive/`, and `check` prints nothing about it with or without `--all`; move
  the same bytes into `Notes/` and three errors appear. There is no flag for
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
| a vault no note at all could be read from | one unreadable file is reported and the rest judged, but where every read failed there is nothing in hand to judge, and a page whose every line said so under a passing exit code would read as a verdict. The refusal names the first file the reads stopped on |

The second row is the one to expect on a vault that otherwise looks healthy,
and it is wider than it looks. It prints

```
yomihon: privacy authority unavailable; agent-facing output disabled
```

for **any** contract that did not load — a missing `[privacy]` section is
merely the commonest, and a `[fields] required` naming a key that
`[fields] common` does not list reads exactly the same. It deliberately says no more, because
the reason would quote the contract back out under the very policy that is
missing. One command tells you which:

```bash
yomihon serve --root <vault>
```

The reason is on the page, and in a startup log line that names it outright:

```
level=WARN msg="vault contract unavailable"
  error="decode vault contract: fields.required: \"slug\" is not listed in fields.common"
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
| `vault-schema.toml#folders` | its `[folders]` table |
| `vault-schema.toml#types` | its `[types]` tables |
| `yomihon` | the product's own dialect — link resolution, collisions, reference liveness, the study-path grammar. No vault artifact declares these |

A finding that says `yomihon` is never one you can argue with by editing the
contract. The reverse does not hold as neatly, and two rules are the reason:
`schema.frontmatter` and `schema.language` both carry `vault-schema.toml`, and
neither is a value your contract chose — one says the YAML did not parse and
the other that `lang` is not a well-formed BCP 47 tag. What they share with the
rest of the table is where they were reached from, not who set the bar.

## Severity

Every `schema.` rule is an `error`; nothing else ever is. Two rules are always
`info` — `callout.title_markup`, a formatting observation, and
`link.unused.definition`, a tidy-up signal. Everything else is a `warn`, except that three rules *drop* from `warn` to `info` when the vault has
declared the name owed rather than missing: `link.broken`, `link.broken.path`
and `map.disk_mismatch`. `link.broken.path` has a second reason to sit at
`info`: a path that climbs out of the vault root is reported rather than judged,
because what is there depends on the machine.

The consequence for gating: `--deny warn` catches a broken link and does not
catch one marked as a planned gap. That is intended, and it is also the
dialect's sharpest trap; [`names-and-links.md`](names-and-links.md) owns how a
gap is declared and how a heading can declare one by accident.

## The frontmatter rules

All frontmatter rules are `error`. They judge only files inside
`[folders] checked`; a file outside them is still read and still linked,
so silence there is not a verdict.

| `rule_id` | What it means | `source_rule` |
|---|---|---|
| `schema.required` | a key `[fields] required`, or the `required` list of the note's own `[types.<name>]`, names is absent. `field` says which | contract |
| `schema.unknown_key` | a key outside the closed field list: the fields yomihon defines that this note's type allows (`[fields] common` and its `[types.<name>] fields`), `type`, `draft`, and the names Obsidian reserves (`tags`, `aliases`, `cssclasses`). `title` and `domain` are derived, not fields, so writing either is this finding | contract |
| `schema.enum` | a value outside the list `[enums]` declares for the field, or a `type` the contract does not declare, or, for `field` `domain`, a folder under a `[folders] domain_from` root that the `[enums] domain` list does not name. For `domain` there is no frontmatter to fix: the note's domain is its folder's name, so move the note into a folder named for a declared domain, or add the name to `[enums] domain`. Read the message's own wording | contract |
| `schema.type_dependent` | a present, non-empty scalar `type` is undeclared and the note carries keys that some type lists in its `[types.<name>] fields`. Those keys are unjudged until `type` is valid; this one explanation replaces their false `schema.unknown_key` findings. The type's `schema.enum`, genuinely unknown keys, and independent checks still apply | contract |
| `schema.draft` | `draft` is anything but the boolean `true` or `false` — text such as `"yes"`, a list, or an empty value. yomihon defines the flag itself, so the check holds whether or not the contract lists it | contract |
| `schema.frontmatter` | the frontmatter is not valid YAML, or it opens on line 1 with a `key:` line and never closes (a lost closing `---`). Everything else about the note is unjudgeable until this is fixed | contract |
| `schema.language` | `lang` is not a valid BCP 47 tag | contract |
| `schema.slug` | the slug does not match the `slug_pattern` of the note's type | `#types` |
| `schema.provenance` | a note of the `concept` type carries none of the provenance fields `[rules] concept_requires_provenance` names. Its message is a frozen sentence, `frontmatter concept has neither based_on nor source_locator`, and those two words are in the message rather than in your contract: the fields actually demanded are whichever your contract lists, and `source_locator` is not even a legal key in the vault this repository ships. The list may name fields beside the ones yomihon defines, but not a built-in, derived or removed one (`type`, `draft`, `tags`, `aliases`, `cssclasses`, `title`, `domain`, or a name earlier contracts allowed that yomihon has dropped): the contract is refused at load. Read the contract, not the message | `#rules` |
| `schema.reference_nested_sequence` | an item in a recognized reference list is itself a YAML list, including an alias to a list. One finding names each affected `field` | `yomihon` |
| `schema.navigation_unavailable` | a type role in the contract could not be used (a role that is none of `lesson`, `course`, `map`, `thought`, or a second type holding the `lesson` or the `thought` role), so no note type is a course or a map and the course and map rules, including every study-path rule, skipped every note. Its `path` is the contract file, and it does not quote the contract; denying any of the rules it silenced fails the gate through it | contract |
| `schema.unmatched_knowledge_dir` | `[folders] checked` names a directory this vault does not have, so the frontmatter rules reach nothing there. The fault is in the contract, not in a note | `#folders` |

For `schema.reference_nested_sequence`, quote the reference: write
`based_on: ["[[Note]]"]` or `based_on: ["Note"]`. YAML reads the unquoted
`based_on: [[Note]]` as nested lists. The rule shares reference selection with
`provenance.unresolved`: the reference field is `based_on`, and a `related`
key is read the same way even though it is not on the closed field list.
`source_locator` and additional provenance requirements do not declare
reference fields. The reading page explains the same finding; neither surface
rewrites the note.

## The link and name rules

All `warn` unless noted. One exception is worth knowing before you go looking
for a finding that never comes: a `[[link]]` yomihon accepted as a **study
path's lesson row** is judged by `map.disk_mismatch` below instead of by any of
these, even when the target is exactly some note's title. Every other link in
that same note is judged here as usual.

| `rule_id` | What it means |
|---|---|
| `link.broken` | the target matches no filename, path or alias anywhere. Drops to `info` under a planned gap |
| `link.title_not_alias` | the target is some note's title, the first heading of its body, and titles are not keys. The message names the note you meant |
| `link.section_missing` | the note resolved; no heading answers the text after `#`. `resolved_to` says which note |
| `link.block_missing` | the note resolved; no line carries the `^address` |
| `link.broken.image` | a picture the page would show from the vault names no file there: a Markdown image `![](path)`, resolved from the note's own folder, or an `![[file]]` embed of a picture or PDF. `target` is the path or name the page reports. A missing `![[file]]` is reported here and never as `link.broken` |
| `link.broken.path` | a plain path reference to a file that is not in the vault. Drops to `info` under a planned gap |
| `link.unused.definition` | a link reference definition `[label]: address` that no link or image in the same note resolves through, so the page shows its address nowhere. It is always `info`: it breaks nothing the reader sees, so `--deny warn` and `--deny link.unused.definition` do not catch it and `--deny info` does. `target` is the label. The usual causes are a label renamed on one side only, and two markers written together as `[E1][E2]`, which reads as one link with the text E1 and the label E2: write `[E1] [E2]` |
| `embed.section_missing` | an `![[…#Heading]]` embed whose heading is not there, so there is no excerpt to cut. Nothing is shown; the block names the address that failed |
| `embed.block_missing` | the same for `![[…#^id]]` |
| `collision.name` | two or more files share one resolution name that some `[[link]]` or `![[embed]]` in the vault is written against, so that link is ambiguous and nothing is linked. A shared name no wikilink uses is not reported: a Markdown path link such as `[run](lab/a/main.go)` resolves by path, not by name. `collision_members` lists them all. Rename one, or link each by its full vault-relative path |
| `collision.alias` | two notes declare the same alias. Give the alias one owner |
| `provenance.unresolved` | a frontmatter reference field points at nothing — `based_on`, and a `related` key, which is not on the closed field list and so draws `schema.unknown_key` as well (`source_rule` `yomihon`). A lesson slug counts as resolving |
| `provenance.alias_resolved` | a reader's thought (the type holding the `thought` role) cites a note in `based_on` by a name that is only an alias of another note, so the note it reaches may have replaced the one the thought was written about. `resolved_to` names the note the alias leads to. Read it, then cite it by its own name or rewrite the thought. A note a thought cites by its file name is not reported, and neither is any note that is not a thought |

## The course rules

Every rule below judges a **study path**, the two whose names open with *map*
included; [`study-paths.md`](study-paths.md) explains that naming. All `warn`.

| `rule_id` | What it means |
|---|---|
| `map.disk_mismatch` | a study path lists an entry that resolves to nothing — a course promising a note that is not there. Drops to `info` under a planned-gap heading, which is how you say "owed, not missing" |
| `map.disk_unlisted` | a lesson exists on disk that no study path of its domain lists. Narrower than it sounds: it runs **only** for a study path whose folder gives it a domain (it sits under a `[folders] domain_from` root, inside a folder of its own), only over lessons in that same domain, and never over a lesson that declares `draft: true`. Nobody has offered that lesson yet, so there is nothing for a path to have listed. A vault whose paths sit outside the domain folders never sees this rule at all |
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
| `scan.skipped` | warn | the scan saw a path and read nothing from it, because a note is read only out of a regular file. A symbolic link is the usual cause: it holds no note, answers no link, and appears in no listing |
| `scan.unreadable` | error | the scan saw the file and the read could not open it — a permission taken away is the usual cause — so nothing was judged from it. The rest of the vault is still judged, but the five rules that conclude something is *nowhere* say nothing for that run: see below. Restore read access and judge again. A file under a `[privacy] never_egress_dirs` directory carries no path here and one fixed sentence in place of the reason, because both would describe ground the contract closed |
| `callout.title_markup` | info | a recognised callout's title carries markup — a wikilink, an HTML tag, emphasis, a code span, a markdown link or an image. Titles are escaped, not parsed, so it would render as visible text. Move it into the body |

### What a file nobody could read takes with it

A finding about one note stays true when another note could not be opened. A
finding about something being **absent** does not: the file nobody read may hold
the name, the alias or the listing. So a run carrying `scan.unreadable` reports
none of these five, and the notice is the only account you get of why:

`link.broken` · `link.title_not_alias` · `provenance.unresolved` ·
`map.disk_mismatch` · `map.disk_unlisted`

One link is narrowed rather than silenced: an address whose `#section` or
`#^block` lands in the file that could not be opened says nothing, while every
other fragment in the run is still judged.

`link.broken.image` is split the same way: a missing `![[file]]` embed says
nothing for that run, since the name could be the unread file's alias, while a
Markdown image is still judged, because it names a path and the scan already
saw every path.

A run over a healthy vault is unaffected — there is no notice, so nothing is
withheld. What this costs is the run you make **while** a permission is wrong:
fix the permission before trusting a clean answer about dead links.

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
$ yomihon exists --root <vault> --format json 2026-09-22
{"query":"2026-09-22","matches":[],"withheld":true}
$ echo $?
0
```

Run that against a name under one of your own withheld directories before you
write any script that depends on it. `check` and `coverage` make the opposite
trade — they never report a withheld note at all, not even with `--all` — which
is why this one command has to be described separately rather than lumped in
with them.

A name containing a `/` is also read as a vault path or path suffix, through
the same resolver a link uses. A readable public note it reaches, and that no
other field row already reports, answers with a `path` row whose
`path` and `value` both hold the note's full vault path:

```
$ yomihon exists --root <vault> --format json Atlas/Postgres
{"query":"Atlas/Postgres","matches":[{"path":"Notes/Projects/Atlas/Postgres.md","field":"path","value":"Notes/Projects/Atlas/Postgres.md"}]}
$ echo $?
0
```

Several notes ending with the same suffix each get a `path` row, and a
withheld one among them adds only `"withheld":true`. A path query like this
one, which earlier versions answered with exit 1, now exits 0 when a note
answers it, so a script that gated a write on it no longer writes a second
note at a path that already exists.
