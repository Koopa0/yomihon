---
title: What yomihon is
type: note
status: ready
created: 2026-01-04
updated: 2026-09-04
lang: en
---

yomihon reads a folder of Markdown as one book. A study path is a course; every note has its sections and the notes that cite it beside the text; whatever is broken is said where it is. None of your words are changed.

Navigation, search, diagnostics, study paths: all of it is there so that what you organised can be read back.

## What it does not do

It does not organise your notes, suggest what to write, or repair anything. When it finds a fault it says so and leaves the file alone — see [[Diagnostics are reports]]. The one thing it writes is a note's status line, and only when you ask.

It is local and single-user: the server binds `127.0.0.1` and has no remote mode; see [[Privacy and egress]]. The status control and the study paths need to know what a note means, so they need [[The vault contract]].
