---
title: L02 Add a contract
type: lesson
status: ready
domain: yomihon
slug: l02-add-a-contract
level: fundamental
created: 2026-01-09
updated: 2026-09-04
lang: en
---

Copy this vault's `System/schemas/vault-schema.toml` to the same path inside
your own vault, then change it until it describes your notes. That path is the
only one it is read from, and it is never written back.

Change it in this order:

1. `[enums]` and `[fields]`, together. Start from the frontmatter your notes
   already carry — [[Frontmatter]] says where to look — and cut both down to
   the keys and the values you really write.
2. `[scan]`, naming the directories that hold knowledge rather than machinery.
3. `[[lifecycle]]` last. It is the part that needs the most thought, and it is
   easier to judge once the vocabulary above it has stopped moving:
   [[The status lifecycle]].

It is validated whole, so a half-finished edit closes the status control until
the edit is finished. Reading, folders and search do not depend on any of it
and carry on.

## What each section opens

| Section | What it opens |
| --- | --- |
| `[enums]`, `[[lifecycle]]` | The vocabulary, and the status control that moves a note through it. Without either, the contract is refused whole and the folder is read as though it carried none. |
| `[fields]` | The frontmatter keys a note may write. Leave it out and every key you have written is reported as one nothing declares. |
| `[scan]` | Which directories hold knowledge. What lies outside them is still read, and is not judged. |
| `[navigation]` | Study paths and maps. |
| `[artifacts]` | The directories holding shapes to copy rather than notes under a lifecycle. Leave it out and no note anywhere gets a status control. |
| `[privacy]` | The directories nothing may quote back out — and with them the command line, which refuses to report at all until the section is there. |
| `[rules]` | The checks on a slug, on a domain against its folder, and on a tag. |
| `[supersession]` | The two fields that record which note replaced which. |

One declaration can require another. Put `lesson` in `enums.type` and
`rules.slug_pattern` becomes required, because a lesson carries a slug and
nothing else says what a slug may look like.
