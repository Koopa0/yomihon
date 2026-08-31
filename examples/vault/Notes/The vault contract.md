---
title: The vault contract
aliases: [contract, vault-schema]
type: note
status: ready
created: 2026-01-11
updated: 2026-08-30
lang: en
---

A folder of Markdown is readable with no contract at all. The contract is what
opts a folder into the parts of yomihon that need to know what your notes mean:
which types exist, which statuses each type may carry, which directories hold
knowledge, and which never leave the machine.

It lives at `System/schemas/vault-schema.toml`. This vault's copy is the one to
start from — it is the only place in yomihon that defines these things, and
nothing in the code carries a second copy of any list it declares. ^single-source

## The shape of it

```toml
[enums]
type = ["note", "lesson", "study-path", "moc"]

[enums.status]
note = ["draft", "ready", "published", "archived"]
```

The `[[lifecycle]]` rows are the interesting part, and they are the subject of
[[The status lifecycle]].

## When it cannot be read

A contract that exists and cannot be parsed is not the same as no contract.
yomihon says so on every page and closes the write face rather than guessing —
a folder with a broken contract must never look like a folder that never had
one.
