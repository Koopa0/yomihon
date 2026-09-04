---
title: The vault contract
aliases: [contract, vault-schema]
type: note
status: ready
domain: yomihon
topics: [contract]
replaces: ["An older approach"]
created: 2026-01-11
updated: 2026-09-04
lang: en
---

A folder of Markdown is readable with no contract. The contract tells yomihon what your notes mean: which types exist, which statuses each type may carry, which directories hold knowledge, and which never leave the machine.

It lives at `System/schemas/vault-schema.toml`, and it is the only place these things are defined. Start from this vault's copy. ^single-source

Before it there was [[An older approach]], where the folder a note sat in decided what it was.

## The shape of it

```toml
[enums]
type = ["note", "lesson", "study-path", "moc"]

[enums.status]
note = ["draft", "ready", "published", "archived"]
```

The `[[lifecycle]]` rows are the part that matters; see [[The status lifecycle]].

## When it cannot be read

A contract that cannot be parsed is not the same as no contract. yomihon says so on every page and switches the status control off rather than guess.
