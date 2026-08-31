---
title: Wikilinks in this dialect
type: note
status: ready
created: 2026-02-02
updated: 2026-08-30
lang: en
---

`[[Name]]` resolves by filename and by alias, never by title. A note whose
title differs from its filename is reached by its filename or by an alias it
declares — writing the title into the link is the single most common way a link
misses, and the health page has a section for exactly that case.

## The three answers

A name resolves to one file, to several, or to none.

- One file: an ordinary link.
- Several: yomihon links nothing and says the name is ambiguous. It does not
  pick, because ==the vault gave it no way to know which one you meant==.
- None: the link is marked where it sits, and the reason travels with it.

[[A name nobody has written]] is the third case, live, in this sentence.

## Fragments

`[[Name#Section]]` points into a note. If the note resolves and the section
does not, the link still goes to the note and says the fragment did not place —
losing the whole link over a missing heading would be the wrong trade.
[[The vault contract#A section that is not there]] does that here.

`[[The vault contract#^single-source]]` links to a block instead, by the anchor
that note carries.

## Embeds

`![[Name]]` pulls the whole note in. With a fragment that does not place, the
embed widens to the whole note and says so rather than showing nothing.
