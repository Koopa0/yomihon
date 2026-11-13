---
type: concept
based_on: ["The vault contract"]
created: 2026-09-22
lang: en
---

Frontmatter is the YAML block between two `---` lines at the start of a note. It holds fields such as `type`, `aliases`, `created` and `draft`. A note does not write its title or its domain: the title is its first `#` heading, or the file name when it has none, and the domain is the folder it sits in below a root the contract names.

The fields come from a closed list yomihon defines, and the contract says which of them each type may write and which values some may take. Within the folders it checks, any other field is a diagnostic, with Obsidian's own `tags`, `aliases` and `cssclasses` let through. yomihon never writes frontmatter: `draft: true`, the one mark of an unfinished note, is the author's to write and remove in their editor.
