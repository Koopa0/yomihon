---
title: Privacy and egress
type: note
status: ready
domain: yomihon
created: 2026-09-22
updated: 2026-09-22
lang: en
---

yomihon serves local files on `127.0.0.1` and makes no outbound network calls.

Remote Markdown images become links, so opening a note does not fetch them:

![a photograph on somebody else's server](https://example.invalid/moonrise.jpg)

The contract's `[privacy] never_egress_dirs` excludes directory trees from command-line reports. This vault lists `Diary`: `check` and `coverage` omit its notes. If `exists` matches a private note, it returns exit 0 with `"withheld": true`, without the note's path, matched field or value.

The privacy list does not restrict local reading. Notes under `Diary` remain available in the sidebar and reading pages.

> [!warning] Naming a directory is the whole protection
> Only listed directory trees are withheld. A new folder outside those trees remains eligible for command-line output until added to the list.
