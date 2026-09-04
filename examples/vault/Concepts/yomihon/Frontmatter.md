---
title: Frontmatter
type: concept
status: ready
domain: yomihon
topics: [contract, metadata]
based_on: ["The vault contract"]
created: 2026-02-20
lang: en
---

The block between two `---` lines at the top of a note, written in YAML. It is where a note says what it is: its type, its status, what it is about, when it was written.

yomihon reads all of it and writes one line of it. A key the contract does not list is reported rather than ignored, so a mistyped key name is visible instead of silent.
