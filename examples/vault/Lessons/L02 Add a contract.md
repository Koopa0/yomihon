---
type: lesson
slug: l02-add-a-contract
level: fundamental
created: 2026-09-22
lang: en
---

Copy `System/schemas/vault-schema.toml` into the same path in your vault.
Adapt its vocabulary and directories to your files. yomihon reads this file
but never writes it.

If your notes already carry frontmatter, which section should you check first?

> [!question]- After you have decided
> 1. `[fields]` and `[types.*]`: the keys in your notes. See [[Frontmatter]].
> 2. `[folders]`: the directories to check.
> 3. `[enums]`: the values a field may take.
>
> The field list is closed. A key your notes write that yomihon does not
> define is reported wherever it appears, so settling the fields first avoids
> reading the same report after each edit. If your notes already write only
> yomihon's fields, you can go straight to the folders.

An invalid contract turns off courses and maps. Reading, folders and search
remain available.

## What each section opens

| Section | What it opens |
| --- | --- |
| `schema_version` | The format the file is written in. A contract without the version yomihon reads is refused whole, and the folder is read as though it carried none. |
| `[folders]` | Which directories hold knowledge, which hold templates to copy rather than notes, the journal, and the roots a note's domain is read from. What lies outside the checked directories is still read, and is not judged. |
| `[types.<name>]` | A note type, the fields it adds, and its role: `lesson`, `course`, `map` or `thought`. Study paths and maps come from the roles. |
| `[fields]` | The fields every type may write and the ones every note must. Leave it out and every key but `type`, `draft` and Obsidian's own is reported as one nothing declares. |
| `[enums]` | The values a field, or the domain a folder gives, may take. |
| `[privacy]` | Directories withheld from agent-facing output. The command line refuses to report until this section exists. |
| `[rules]` | The provenance a `concept` must carry, and the marks that keep a planned link from counting as broken. |

One declaration can require another. Give a type the `lesson` role and its
fields must include `slug`, because a lesson carries a slug; give one the
`thought` role and they must include `based_on`, because a thought is about a
source.
