---
title: Wikilinks in this dialect
type: note
status: ready
domain: yomihon
topics: [links, resolution]
created: 2026-02-02
updated: 2026-09-04
lang: en
---

`[[Name]]` resolves by filename and by alias, never by title. A note whose title differs from its filename is reached by the filename or by an alias it declares. Writing the title into the link is the commonest way a link misses; the health page has a section for it.

`[[Name|other words]]` keeps that target and shows the other words instead: [[The vault contract|the contract]] goes where its filename would have gone.

## The three answers

A name resolves to one file, to several, or to none.

- One file: an ordinary link.
- Several: nothing is linked, and the name is marked ambiguous. yomihon does not pick, because ==the vault gave it no way to know which one you meant==.
- None: the link is marked where it sits, with the reason.

[[A name nobody has written]] is the third case, live, in this sentence.

## Fragments

`[[Name#Section]]` points into a note. If the note resolves and the section does not, the link goes to the note and says the section was not found. [[The vault contract#A section that is not there]] does that here.

[[The vault contract#^single-source]] goes to one block rather than a whole note, named by the anchor that note carries.

## Embeds

`![[Name]]` pulls the whole note in; with a section after the name, only that section:

![[The status lifecycle#initial is written out]]

An embed whose section is not found shows nothing of the note: a notice names the address, and the note's name above it leads to the whole note.
