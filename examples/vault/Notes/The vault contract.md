---
aliases: [contract, vault-schema, An older approach]
type: note
created: 2026-09-22
lang: en
---

A Markdown folder is readable without a contract. The contract declares the folders it checks, the note types and the role each plays in courses and maps, the fields a note may write, the values some of them may take, and command-line privacy boundaries.

These declarations belong in `System/schemas/vault-schema.toml`. Use this vault's file as a starting point. ^single-source

It replaced an older approach that inferred types from folders. That note was deleted rather than kept beside this one, and its name is among this note's `aliases`, so a link written to it still arrives here.

## The shape of it

This excerpt declares where the notes are, two types and the fields a note may write:

```toml
schema_version = "2"

[folders]
checked = ["Concepts", "Lessons", "Notes"]
domain_from = ["Concepts", "Lessons"]

[enums]
domain = ["yomihon", "japanese", "go"]

[fields]
common = ["type", "aliases", "lang", "created"]
required = ["type"]

[types.note]

[types.lesson]
role = "lesson"
fields = ["slug", "level"]
```

The field list is closed: a note writes the fields yomihon defines that the contract gives it, and Obsidian's own `tags`, `aliases` and `cssclasses`. `title` and `domain` are not written at all — see [[Frontmatter]]. A note still being written says so with `draft: true` — see [[Drafts]]. The excerpt is not a complete contract.

## When it cannot be read

An unreadable or invalid contract produces a page notice and turns off courses and maps. Reading, folder navigation and search remain available. The `check`, `coverage` and `exists` commands refuse to run without a usable contract and privacy declaration.
