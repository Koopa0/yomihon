---
title: Two languages
type: note
status: ready
created: 2026-08-29
lang: en
---

Everything yomihon says in its own voice exists in English and Traditional
Chinese, and the control in the header switches between them.

What does not switch is your notes. The words in this sentence are the author's
and stay exactly as written whichever language the interface is speaking — see
[[芭蕉の句]], which declares its own language and keeps it while the frame around
it changes.

## How it is built

There is no catalogue file and no lookup that can miss. A sentence is built from
two strings at once, so one written in a single language does not compile. The
cost is that the sentences live in one package instead of beside the markup that
shows them; what it buys is that no sentence can be half-written.
