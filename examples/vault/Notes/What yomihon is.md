---
title: What yomihon is
type: note
status: ready
created: 2026-01-04
updated: 2026-08-30
lang: en
---

A reading surface for a folder of Markdown you already own. It renders your
notes, resolves the links between them, reports what is broken, and changes
exactly one thing: a note's `status` line.

That last sentence is the whole design. Everything else here — the navigation,
the search, the diagnostics, the study paths — reads. One narrow act writes,
and it rewrites a single frontmatter field in place.

> [!note] This vault is the documentation
> The notes you are reading are the example vault that ships with yomihon.
> They describe yomihon using yomihon, so what they say and what you are
> looking at cannot drift apart.

## What it does not do

It does not organise your notes, suggest what to write, or repair anything.
When it finds a fault it says so and leaves the file alone — see
[[Diagnostics are reports]].

It is local and single-user. The server binds `127.0.0.1` and has no remote
mode; see [[Privacy and egress]] for what that leaves open and what it closes.
