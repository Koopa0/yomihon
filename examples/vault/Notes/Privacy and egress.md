---
title: Privacy and egress
type: note
status: ready
domain: yomihon
created: 2026-02-16
updated: 2026-09-04
lang: en
---

yomihon makes no network call. It opens your files and a loopback socket, and holds no client and no credential with which to do anything else.

An address pointing off the machine is shown, never fetched. This one is written as a picture and arrives as a link:

![a photograph on somebody else's server](https://example.invalid/moonrise.jpg)

`[privacy] never_egress_dirs` in the contract names directories an agent's output may not draw from. `Diary` is named there, so `check`, `coverage` and `exists` leave those notes out of what they report.

The reading pages are not bound by it. Open `Diary` in the sidebar and the note is there, the same as in Obsidian. What the declaration governs is what yomihon tells a program about this vault, not what you can read on your own machine.

> [!warning] Naming a directory is the whole protection
> A directory left off `never_egress_dirs` is one an agent's output may draw from. The list is not a judgement about what looks private — it is the list, and a directory added tomorrow is outside it until somebody puts it in.
