# An example vault

A small vault with a contract. Point yomihon at it to see what a contract adds:

```sh
yomihon examples/vault
```

`Notes/中文` holds a shorter path over the same ideas in Traditional Chinese.
Nothing is translated: each note declares the language it was written in and
keeps it.

`System/schemas/vault-schema.toml` is the contract. Copy it into your own vault
and change it: the directories, the enums, the privacy boundary and the
transitions are decisions about your notes, not defaults.

`yomihon check --root examples/vault` reports three findings, and all three are
written on purpose: a status outside the list, in
`Notes/A note with a fault in its frontmatter.md`; a link with no target and a
section that is not there, both in `Notes/Wikilinks in this dialect.md`. Each of
those notes says in its own words why the fault is there. So `--deny warn` and
`--deny error` both exit 1 on this vault, and a copy of it starts out failing
that gate until you take the three out.

This file is listed in the contract's `scan.skip_basenames`, so it carries no
frontmatter and the checks skip it.
