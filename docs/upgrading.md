# Upgrade guide: move a vault from the older format

English | [繁體中文](upgrading.zh-TW.md)

Use this page when the first line of `System/schemas/vault-schema.toml` is `schema_version = "1"`, or when yomihon says "This vault's rules use an older format." The move has two parts: change the rules file, then change the notes.

yomihon only reads your vault. Every edit below is yours to make, in your editor or with a script of your own. Work in a Git repository or on a copy, so you can see what changed before you keep it.

## Until you migrate

Reading and search work as they do for a folder with no rules file. `yomihon check`, `coverage`, `exists` and `marks` refuse the vault with exit 2 and "privacy authority unavailable". A yomihon built for the older format cannot read the migrated rules file either, so migrate on the day you install the new binary. Restart yomihon when you are done: it reads the rules file once, when it starts.

## 1. The rules file

Set the first line to `schema_version = "2"`, then move each key as the table says.

| Older format | Now |
| --- | --- |
| `[scan] knowledge_dirs` | `[folders] checked` |
| `[scan] skip_basenames` | `[folders] skipped_files` |
| `[scan] exclude_dirs` | `[folders] ignored` |
| `[scan] no_frontmatter_is_legal` | `[folders] frontmatter_optional` |
| `[rules] domain_equals_folder_under` | `[folders] domain_from` |
| `[artifacts] non_instance_dirs` | `[folders] templates` |
| `[navigation] journal_dir` | `[folders] journal` |
| `[navigation] path_types` | `role = "course"` in a `[types.<name>]` table for each type listed |
| `[navigation] map_types` | `role = "map"` in a `[types.<name>]` table for each type listed; an empty list is simply deleted |
| `[navigation] answer_type` | `role = "thought"` in that type's table, with `based_on` in its `fields` |
| the type named `lesson` | a `[types.lesson]` table with `role = "lesson"` and `slug` in its `fields` |
| `[rules] slug_pattern` | `slug_pattern` in the lesson type's table |
| `[fields] known` | `[fields] common`, keeping only the fields in "The notes" below |
| `[fields] lesson_only` | `fields` in the lesson type's table, with the same limit |
| `[fields] required` | stays, but may name only a field every note may carry (see "The notes" below). Take out `title`, `status` and `domain` |
| `[enums] type`, `domain`, `level` | unchanged. A domain is now checked against the folder a note sits in |
| `[rules] concept_requires_provenance`, `planned_gap_marks`, `planned_inline_marks` | unchanged |
| `[privacy] never_egress_dirs` | unchanged |

A type needs a `[types.<name>]` table only to give it a role, fields of its own, required fields or a slug pattern. A name in `[enums] type` is a type already.

Delete what nothing replaces: `[enums.status]`, every `[[lifecycle]]`, `[supersession]`, `[navigation]` once its keys have moved, `[fields] required_inbox`, `domain_exempt_types` and `status_group`, `[rules] forbid_tag_with_slash`, the enums `source_kind`, `source_provider` and `map_kind`, and the top-level `aligned_with` and `generated_at_must_match`. This yomihon refuses a rules file that still carries any of them.

A small file, before and after:

```toml
schema_version = "1"

[enums]
type = ["lesson", "study-path"]
domain = ["go", "sql"]

[enums.status]
note = ["draft", "ready"]

[fields]
required = ["title", "type", "status", "domain"]
known = ["title", "aliases", "type", "status", "domain", "tags", "lang"]
lesson_only = ["slug", "level", "sources", "go_version"]

[rules]
slug_pattern = '^[a-z0-9-]+$'
domain_equals_folder_under = ["courses"]

[scan]
knowledge_dirs = ["courses"]
no_frontmatter_is_legal = true

[navigation]
path_types = ["study-path"]
map_types = []

[artifacts]
non_instance_dirs = ["templates"]

[privacy]
never_egress_dirs = []

[[lifecycle]]
status = "draft"
applies_to = ["*"]
initial = true
from = ["ready"]
owner = ["author"]
```

```toml
schema_version = "2"

[folders]
checked = ["courses"]
templates = ["templates"]
domain_from = ["courses"]
frontmatter_optional = true

[enums]
type = ["lesson", "study-path"]
domain = ["go", "sql"]

[fields]
common = ["type", "aliases", "draft", "lang"]
required = ["type"]

[types.lesson]
role = "lesson"
fields = ["slug", "level", "verified_with"]
slug_pattern = '^[a-z0-9-]+$'

[types.study-path]
role = "course"

[privacy]
never_egress_dirs = []
```

`sources` and `go_version` left the lesson's field list because notes no longer write them; "The notes" says where their content goes. `verified_with` took the place of `go_version`.

## 2. The notes

A note may carry these keys and no others: `type`, `aliases`, `draft`, `based_on`, `lang`, `created`, `slug`, `level`, `prerequisites`, `verified_on` and `verified_with`, plus Obsidian's own `tags` and `cssclasses`. `type`, `draft`, `aliases`, `tags` and `cssclasses` need no listing. Every other one has to be listed in `[fields] common`, or in the `fields` of its type, before a note may write it. `yomihon check` reports any other key as `schema.unknown_key`.

For each note in a folder named in `folders.checked`:

| Key the note carries | What to do |
| --- | --- |
| `title` | The title is the first `#` heading. Make sure the note has one with these words, adding it if the note has none, then delete the key. Without a `#` heading a note is titled by its file name |
| `domain` | Delete it. The domain is the first folder below a root named in `[folders] domain_from`. Where the old value and the folder disagree, the folder wins: move the note, or rename the folder |
| `status: draft`, or any status that means the note is not finished | Write `draft: true` instead |
| `status: ready`, or any other `status` | Delete the line. A note with no `draft` is finished |
| `status: archived` | There is no archived state. If another note replaced this one, delete this note and add its file name to the replacement's `aliases`; [Replace a note](authoring.md#replace-a-note) says when that is safe. Otherwise delete the line |
| `sources` | Move what is not already in the note's text into the text, for example a `## Sources` section at the end, then delete the key. A reference to another note of the vault goes into `based_on: ["[[Note]]"]`, quoted |
| the version of a tool the note was checked against, such as `go_version` | Write it in `verified_with`, for example `verified_with: ["Go 1.26"]`, and list `verified_with` in the type's `fields` |
| `replaces`, `supersedes`, `superseded_by`, `archived`, `published`, `topics`, `updated`, `source_kind`, `source_provider`, `map_kind` | Delete them. Anything a reader should still see goes into the text |
| any other key not in the list above, a counter or an hours estimate or a tool's version stamp | Delete it, or write it into the text if a reader should see it |
| a key in the list above | Keep it |

Scripts and templates that write the older keys put them back into new notes. Change them in the same pass: the folders named in `[folders] templates` first.

## 3. Check it

```bash
yomihon check --root <vault> --format human --deny error
```

Read the exit status.

- **0**: no finding of severity error. The vault is migrated.
- **1**: errors remain. Each `schema.unknown_key` names the key and the note that still carries it. Fix those and run it again.
- **2**: yomihon still cannot use the rules file. Run `yomihon serve --root <vault>`: at start it logs `rules file unavailable` with the reason, which names the key it could not read, or `rules file is for another schema version` when the first line is not `schema_version = "2"`.

Findings that are not `schema.*`, such as a link with no target, belong to the vault and were there before the migration.
