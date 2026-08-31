# An example vault

A small, complete vault you can point yomihon at to see what it does with one:

```sh
yomihon examples/vault
```

It carries a contract, so the governed surfaces are live — a status control on
each note, a study path with counts and prev/next, a map, and the diagnostics
page. Everything in it is written to be read; nothing here is a test fixture.

`System/schemas/vault-schema.toml` is the contract. Copy it into your own vault
and adapt it deliberately: the directories, the enums, the privacy boundary and
the transitions are all decisions about your notes, not defaults.

This file is skipped by the contract's `scan.skip_basenames`, which is why it
carries no frontmatter and appears in no listing.
