---
title: L02 Add a contract
type: lesson
domain: yomihon
slug: l02-add-a-contract
level: fundamental
created: 2026-09-22
updated: 2026-09-22
lang: en
---

Copy `System/schemas/vault-schema.toml` into the same path in your vault.
Adapt its vocabulary and directories to your files. yomihon reads this file
but never writes it.

If your notes already carry statuses, which section should you check first?

> [!question]- After you have decided
> 1. `[enums]` and `[fields]`: the keys and values in your notes. See [[Frontmatter]].
> 2. `[scan]`: the directories to check.
> 3. `[[lifecycle]]`: transitions among the declared statuses. See [[The status lifecycle]].
>
> The lifecycle depends on the status vocabulary. Changing that vocabulary
> first avoids revisiting transitions after each edit. If you are keeping the
> existing vocabulary, you can work on the lifecycle immediately.

An invalid contract turns off courses and maps. Reading, folders and search
remain available.

## What each section opens

| Section | What it opens |
| --- | --- |
| `[enums]`, `[[lifecycle]]` | The vocabulary, and the statuses `yomihon check` holds notes to. Without either, the contract is refused whole and the folder is read as though it carried none. |
| `[fields]` | The frontmatter keys a note may write. Leave it out and every key you have written is reported as one nothing declares. |
| `[scan]` | Which directories hold knowledge. What lies outside them is still read, and is not judged. |
| `[navigation]` | Study paths and maps. |
| `[artifacts]` | The directories holding shapes to copy rather than notes. Leave it out and a template is read and judged like any other note. |
| `[privacy]` | Directories withheld from agent-facing output. The command line refuses to report until this section exists. |
| `[rules]` | The checks on a slug, on a domain against its folder, and on a tag. |
| `[supersession]` | Fields recording which note replaced which. |

One declaration can require another. Put `lesson` in `enums.type` and
`rules.slug_pattern` becomes required, because a lesson carries a slug and
nothing else says what a slug may look like.
