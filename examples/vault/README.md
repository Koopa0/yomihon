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

This file is listed in the contract's `scan.skip_basenames`, so it carries no
frontmatter and the checks skip it.
