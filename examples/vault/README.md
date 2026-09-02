# An example vault

A small, complete vault you can point yomihon at to see what it does with one:

```sh
yomihon examples/vault
```

It carries a contract, so the governed surfaces are live — a status control on
each note, a study path with counts and prev/next, a map, and the diagnostics
page. Everything in it is written to be read; nothing here is a test fixture.

`Notes/中文` holds a second, shorter path over the same ideas in Traditional
Chinese, so the interface and the notes can be seen speaking one language at a
time. Nothing is translated for you: each note declares the language its author
wrote it in and keeps it.

`System/schemas/vault-schema.toml` is the contract. Copy it into your own vault
and adapt it deliberately: the directories, the enums, the privacy boundary and
the transitions are all decisions about your notes, not defaults.

This file is skipped by the contract's `scan.skip_basenames`, which is why it
carries no frontmatter and appears in no listing.
