---
title: Frontmatter
type: concept
domain: yomihon
topics: [contract, metadata]
based_on: ["The vault contract"]
created: 2026-09-22
updated: 2026-09-22
lang: en
---

Frontmatter is the YAML block between two `---` lines at the start of a note. It holds fields such as `title`, `type`, `domain` and `draft`.

The contract declares permitted fields and values. Within its knowledge scope, an undeclared field is a diagnostic. yomihon never writes it: `draft: true`, the one mark of an unfinished note, is the author's to write and remove in their editor.
