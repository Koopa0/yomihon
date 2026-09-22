---
title: L01 Point yomihon at a folder
type: lesson
status: ready
domain: yomihon
slug: l01-point-at-a-folder
level: fundamental
supersedes: ["Building yomihon from source"]
created: 2026-09-22
updated: 2026-09-22
lang: en
---

Start the reader with a Markdown folder:

```sh
yomihon /path/to/notes
```

Open `http://127.0.0.1:9610` to read, browse folders and search. Starting the
server leaves the notes unchanged. Keep the command running while you read;
press `Ctrl+C` in the terminal to stop it.

[[The vault contract]] enables features that depend on declared types and
rules. [[L02 Add a contract]] shows the setup.

| What works with no contract | What needs one |
| --- | --- |
| Reading, rendering, folders | Status control |
| Wikilinks and backlinks | Study paths and maps |
| Full-text search | Contract-based diagnostics and CLI reports |
