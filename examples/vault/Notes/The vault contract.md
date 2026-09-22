---
title: The vault contract
aliases: [contract, vault-schema]
type: note
status: ready
domain: yomihon
topics: [contract]
replaces: ["An older approach"]
created: 2026-09-22
updated: 2026-09-22
lang: en
---

A Markdown folder is readable without a contract. The contract declares note types, permitted fields and statuses, course and map types, scan scope, and command-line privacy boundaries.

These declarations belong in `System/schemas/vault-schema.toml`. Use this vault's file as a starting point. ^single-source

It replaces [[An older approach]], which inferred types from folders.

## The shape of it

This excerpt declares types and the default status vocabulary:

```toml
[enums]
type = ["note", "lesson", "study-path", "moc", "concept", "inbox"]

[enums.status]
note = ["draft", "ready", "published", "archived"]
```

`[[lifecycle]]` rows define allowed status transitions; see [[The status lifecycle]]. The excerpt is not a complete contract.

## When it cannot be read

An unreadable or invalid contract produces a page notice and disables the status control, courses and maps. Reading, folder navigation and search remain available. The `check`, `coverage` and `exists` commands refuse to run without a usable contract and privacy declaration.
