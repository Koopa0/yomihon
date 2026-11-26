---
type: note
created: 2026-09-22
lang: en
---

yomihon opens a Markdown folder as a local reading library. Pages show section navigation, backlinks and diagnostics. Study paths arrange lessons into courses; search opens notes at the matching text.

Use [[The vault contract]] to declare note types, the fields a note may write, courses and maps.

## What it does not do

It does not organise or repair files. [[Diagnostics are reports]] describes how it presents problems. It never edits a note: a note's frontmatter, its `draft` flag included, changes only in the author's editor. The only thing it writes into the vault is the reader's own highlights, thoughts and questions, one file for each computer under `reader-notes/.yomihon/marks/`.

The server listens only on `127.0.0.1`. See [[Privacy and egress]] for network and command-line privacy boundaries.
