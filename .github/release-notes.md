<!-- Goes above GitHub's generated notes when a release is published (see CONTRIBUTING). Edit it for each release; do not append to it. -->

Since v0.3.0, yomihon only reads: nothing writes into a vault any more, and a note's fields are a closed list the contract declares. The reading room is redrawn around reading, with a new mark.

**Upgrading needs the contract of your vault changed on the same day as the binary.** This release reads `schema_version = "2"` only, and v0.3.0 refuses it. See "Upgrading" below.

## What is new

- **Read-only.** The status panel and `POST /status` are gone. The only things yomihon writes are your reader marks and figure approvals, under its configuration directory.
- **A note has a title, a domain and an optional `draft` flag.** The title is the first H1 (then the file name) and the domain is the folder the note sits in; neither is written in frontmatter. A draft is marked quietly in the sidebar, the course page, the shelf and search.
- **A closed list of fields.** `type`, `aliases`, `draft`, `based_on`, `lang`, `created`, `slug`, `level`, `prerequisites`, `verified_on` and `verified_with`. Any other key is reported as `schema.unknown_key`.
- **A margin beside every section** of a lesson or note: "Not sure yet", "Leave a thought", the thoughts you already wrote there, and the place you left off at. A setting hides the buttons until the pointer rests on a section.
- **The end of a note** is one block: what to read beside it, its sources, the notes based on it, findings and the facts, including the day the author wrote as `verified_on` and the prerequisites as links.
- **Callouts** name their kind, and a closed question gets a quiet "try answering before you look" with a link that writes the thought.
- **The frame** has the course name leading the sidebar with the note's own contents under it. The right rail is gone. Courses live at `/courses` (the old `/paths` and `/syllabus/…` addresses redirect). A page change plays no motion, the sidebar keeps its scroll and a kept place lands once.
- **The desk, the shelf, search and the health page** are redrawn: the place you kept is the largest text on the desk, the shelf has three orders and no count, and the health page keeps its exceptions visible.
- **Drawings sit on a mat.** An SVG alone in its paragraph and a figure's frame scroll sideways on a narrow screen instead of shrinking until the words cannot be read.
- **A new mark.** The favicon and the README image stay visible on dark browser tabs.

## Upgrading

1. Install this binary and the vault's migration on the same day. The old and the new loader refuse each other's contract on purpose: `check`, `coverage`, `exists` and `left-open` exit 2 with "privacy authority unavailable" when the contract is not the one they read.
2. In the contract, remove `[[lifecycle]]`, `[enums.status]`, `[supersession]`, `[navigation]`, `[artifacts]`, `[scan]`, `known`, `lesson_only` and the other keys the closed contract no longer reads, and set `schema_version = "2"`. `docs/` and the yomihon skill describe the contract.
3. In notes, move what frontmatter used to carry into the text: `title` becomes the first H1, `domain` the folder, `status` is dropped or becomes `draft: true`, `sources` and `tags` become text. A note that is replaced is deleted and its old name goes into the successor's `aliases`.

For tools that read yomihon's output:

- Removed judge rules: `schema.status_unreachable`, `schema.domain_folder`, `schema.legacy_tag`, `supersession.predecessor_not_archived`, `supersession.archived_navigation_target`. New: `schema.draft`, `provenance.alias_resolved`, and `schema.enum` on the derived domain.
- `yomihon left-open` JSON: a thought item has no `status`; `update_date` is now `create_date` and items sort by `created`.
- Search: the `topic:` filter is removed and `status:` became `draft:`; `domain:` reads the derived domain.
- `exists` no longer compares `title_en`; the markdown report of `check` begins with frontmatter that holds only `type: report`.

## Platform limits

- Obsidian's handling of a boolean `draft` property and of `aliases` on a note that was deleted has not been checked.
