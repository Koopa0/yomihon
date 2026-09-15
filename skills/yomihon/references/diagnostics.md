# Every diagnostic, and what to do about it

**The question this answers:** `yomihon check` reported something — what is it
telling me, whose rule is it, and what is the repair?

There is one authority for this and it is the command, not this page. Where the
two disagree, believe the command. What this page adds is the whole set in one
place: a finding you can look up is a finding you can fix, and a rule id you
cannot find here is a rule this page has fallen behind on.

## Running it

```bash
yomihon check --root <vault> [--format json|human|md] [--all]
              [--deny <severity|rule-id>]... [--baseline <file>] [path...]
```

| | |
|---|---|
| `--root` | the vault to judge. Without it, the folder you are standing in is the vault |
| `[path...]` | narrows the judging to part of that vault. Written the way the vault spells it, relative to the root — `Notes`, or `Notes/topic.md`. An absolute path is not a second way to say `--root`: it exits 2 with empty stdout |
| `--format` | `json` is one compact object per line, the machine format. `human` is a terminal summary grouped by domain. `md` is a fileable report body — it opens by saying whether this vault's contract would accept it as a note. Without the flag, a pipe gets `json` and a terminal gets `human` |
| `--deny` | a severity (`error`, `warn`, `info`) or an exact rule id, repeatable. A severity is a **threshold**, not a match: `--deny warn` fails on warnings and on errors, and `--deny info` fails on anything at all. A value that is neither a severity nor a real rule id exits 2 with `unknown --deny`, so a typo fails loudly instead of quietly disabling the gate |
| `--all` | restores findings that touch nothing inside `[scan] knowledge_dirs`. The whole vault is scanned either way; this decides what is reported |
| `--baseline` | a previous run's JSONL, subtracted by `fingerprint`, so only new findings are reported and gated |

Exit codes: **0** nothing named by `--deny` was found · **1** a `--deny` gate
hit · **2** the command could not run. Findings alone never fail it — without
`--deny`, `check` reports and exits 0, which is why a green exit is not by
itself evidence of a clean vault.

Three ways the command refuses rather than answers, all exit 2 with empty
stdout:

| Refusal | Why |
|---|---|
| no `System/schemas/vault-schema.toml` | the folder has declared no vocabulary to judge against |
| the contract has no `[privacy]` section | permission to report is positive authority, and an absent section grants none. This disables `check`, `coverage` and `exists` for the whole vault, and is the commonest cause of a refusal on a vault that otherwise looks healthy |
| a `[path...]` inside a `[privacy] never_egress_dirs` directory | that ground is scanned, but nothing from it may be reported, and an empty answer would read as a clean verdict |

The middle one prints `privacy authority unavailable; agent-facing output
disabled` and deliberately does not say more: the reason would quote the
contract back out under exactly the policy that is missing. Run the server to
read it.

`--baseline` is version-locked. A fingerprint carries its algorithm version as
a prefix, currently `v1:`, and a baseline written by a different one stops the
run at exit 2 naming the line rather than silently under-subtracting.

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
| `collision_members` | every path involved in a name collision, so one finding describes the whole of it |

`source_rule` says where the rule's authority is written, and is worth reading
before arguing with a finding:

| Value | Meaning |
|---|---|
| `vault-schema.toml` | your contract's own type, field and status declarations |
| `vault-schema.toml#rules` | its `[rules]` table |
| `vault-schema.toml#scan` | its `[scan]` table |
| `vault-schema.toml#supersession` | its `[supersession]` table |
| `yomihon` | the product's own dialect — link resolution, collisions, reference liveness, the study-path grammar. No vault artifact declares these |

A finding whose `source_rule` names the contract is one you can change by
editing the contract. One that says `yomihon` is not.

## Severity

Every `schema.` rule is an `error`; nothing else ever is. One rule is always
`info` — `callout.title_markup`, a formatting observation. Everything else is a
`warn`, except that three rules *drop* from `warn` to `info` when the vault has
declared the name owed rather than missing: `link.broken`, `link.broken.path`
and `map.disk_mismatch`.

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
| `schema.status_unreachable` | the status is in the type's enum, but no `[[lifecycle]]` row applies to that type, so nothing could ever move a note to it | contract |
| `schema.frontmatter` | the frontmatter is not valid YAML. Everything else about the note is unjudgeable until this is fixed | contract |
| `schema.language` | `lang` is not a valid BCP 47 tag | contract |
| `schema.slug` | the slug does not match `[rules] slug_pattern` | `#rules` |
| `schema.domain_folder` | the note's `domain` disagrees with the folder it sits in, under a root `[rules] domain_equals_folder_under` names | `#rules` |
| `schema.legacy_tag` | a tag carrying a slash, under `[rules] forbid_tag_with_slash` — a property written in the wrong place | `#rules` |
| `schema.provenance` | a note of the `concept` type carries none of the provenance fields `[rules] concept_requires_provenance` names. Its message is a frozen sentence naming `based_on` and `source_locator`; the fields actually demanded are whichever your contract lists, so read the contract rather than the message | `#rules` |
| `schema.unmatched_knowledge_dir` | `[scan] knowledge_dirs` names a directory this vault does not have, so the frontmatter rules reach nothing there. The fault is in the contract, not in a note | `#scan` |

## The link and name rules

All `warn` unless noted.

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

## The course and map rules

`map.*` is about a course's relationship with the files on disk; `path.*` is
the study-path grammar itself. All `warn`.

| `rule_id` | What it means |
|---|---|
| `map.disk_mismatch` | a study path lists an entry that resolves to nothing — a course promising a note that is not there. Drops to `info` under a planned-gap heading, which is how you say "owed, not missing" |
| `map.disk_unlisted` | a lesson exists on disk that no study path of its domain lists. Narrower than it sounds: it runs **only** for a study path that itself declares a `domain`, only over lessons carrying that same domain, and never over a lesson whose status is the draft one. A vault whose paths declare no domain — which is normal, since a course usually spans subjects — never sees this rule at all |
| `path.role_missing` | a branch lists lessons, or a nested list exists, and neither says what part it plays. Undeclared is unclassified, and unclassified projects nothing. Two messages, one per shape: a heading's, and a nested list's |
| `path.role_invalid` | the marker's value is none of `primary`, `local`, `none` |
| `path.role_duplicate` | one line declares more than one role |
| `path.role_conflict` | a branch declares itself part of the course while the branch above it declared itself out of it |
| `path.role_nested_primary` | a `primary` branch sits inside a `local` one, which has no place in the main line's order |
| `path.nesting_too_deep` | a side branch inside a side branch |
| `path.local_orphan` | a side branch with nothing to hang from — it was not nested under a lesson |
| `path.role_on_entry` | one row tries to be both a lesson and a branch heading. Give the branch its own row above the list it opens |
| `path.role_misplaced` | a marker written somewhere it declares nothing — in a paragraph, say. It is read only on a heading or on a row that opens a list |
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

`yomihon exists <name>` exits 0 when a note for the name exists and 1 when none
does, so a write-if-absent can gate on the exit code alone. A note inside a
withheld directory is never described — no path, no matched field — but the
exit stays 0, so gating on it does not create a second note under a withheld
note's own name.
