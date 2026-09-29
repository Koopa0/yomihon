---
title: Frontmatter
type: concept
status: ready
domain: yomihon
topics: [contract, metadata]
based_on: ["The vault contract"]
created: 2026-09-22
updated: 2026-09-22
lang: en
---

Frontmatter is the YAML block between two `---` lines at the start of a note. It holds fields such as `title`, `type`, `status` and `domain`.

The contract declares permitted fields and values. Within its knowledge scope, an undeclared field is a diagnostic. The status control rewrites only the `status` value, preserving the rest of the file.
