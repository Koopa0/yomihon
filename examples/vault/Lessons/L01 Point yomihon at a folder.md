---
title: L01 Point yomihon at a folder
type: lesson
status: ready
domain: yomihon
slug: l01-point-at-a-folder
level: fundamental
supersedes: ["Building yomihon from source"]
created: 2026-01-06
updated: 2026-09-04
lang: en
---

Run it against any folder of Markdown. Nothing is written or created, and the
folder is not changed.

```sh
yomihon /path/to/notes
```

Open `http://127.0.0.1:9610`: the reading page, navigation built from the
folders, search, and the links between your notes resolved.

What is not there yet is anything that needs to know what your notes mean: no
status control, no study paths, no type-aware diagnostics. Those need
[[The vault contract]], which is [[L02 Add a contract]].

| What works with no contract | What needs one |
| --- | --- |
| Reading, rendering, folders | Status control |
| Wikilinks and backlinks | Study paths and maps |
| Lexical search | Statuses outside the list |
