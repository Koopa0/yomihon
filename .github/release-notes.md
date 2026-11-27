<!-- Goes above GitHub's generated notes when a release is published (see CONTRIBUTING). Edit it for each release; do not append to it. -->

Since v0.3.0, yomihon edits no note: the only thing it writes into a vault is the reader's own marks, and a note's fields are a closed list the contract declares. The reading room is redrawn around reading, with a new mark.

## What is new

- **No note is written.** The status panel and `POST /status` are gone. Your highlights, thoughts and questions are kept in one file for each computer under `reader-notes/.yomihon/marks/` in the vault, so that git carries them to your other computers; your bookmarks, the ledger of the marks and your figure approvals stay in its configuration directory.
- **A note has a title, a domain and an optional `draft` flag.** The title is the first H1 (then the file name) and the domain is the folder the note sits in; neither is written in frontmatter. A draft is marked quietly in the sidebar, the course page, the shelf and search.
- **A closed list of fields.** `type`, `aliases`, `draft`, `based_on`, `lang`, `created`, `slug`, `level`, `prerequisites`, `verified_on` and `verified_with`. Any other key is reported as `schema.unknown_key`.
- **A margin beside every section** of a lesson or note: "Not sure yet", "Leave a thought", the thoughts you already wrote there, and the place you left off at. A setting hides the buttons until the pointer rests on a section.
- **The end of a note** is one block: what to read beside it, its sources, the notes based on it, findings and the facts, including the day the author wrote as `verified_on` and the prerequisites as links.
- **Callouts** name their kind, and a closed question gets a quiet "try answering before you look" with a link that writes the thought.
- **The frame** has the course name leading the sidebar with the note's own contents under it. The right rail is gone. Courses live at `/courses` (the old `/paths` and `/syllabus/…` addresses redirect). A page change plays no motion, the sidebar keeps its scroll and a kept place lands once.
- **The desk, the shelf, search and the health page** are redrawn: the place you kept is the largest text on the desk, the shelf has three orders and no count, and the health page keeps its exceptions visible.
- **Drawings sit on a mat.** An SVG alone in its paragraph and a figure's frame scroll sideways on a narrow screen instead of shrinking until the words cannot be read.
- **A new mark.** The favicon and the README image stay visible on dark browser tabs.
- **Copy path.** Beside the bookmark, wherever the left column is not a drawer, a note offers its full path with the line of the section you are reading (`…/mvcc.md:42`), for an editor that opens `path:line`. The Obsidian link stays for a folder Obsidian opened.
- **The section Copy path names** is the one at a line just under the header, where the text you read starts, not at the top of the window the header covers. Over the last screen of a note the line moves down to the bottom of the window, so a short section at the end is named in its turn. The bookmark still keeps its place at the top of the window.

## Upgrading

Restart every yomihon that is running, the server included, once this binary is installed. An older yomihon and this one running over the same configuration directory do not keep each other's changes apart, so one can write away a mark the other just kept.

The rules file no longer has a `schema_version`. One that still writes it is refused as an unknown key, like any other key this yomihon does not read.

For tools that read yomihon's output:

- Removed judge rules: `schema.status_unreachable`, `schema.domain_folder`, `schema.legacy_tag`, `supersession.predecessor_not_archived`, `supersession.archived_navigation_target`. New: `schema.draft`, `provenance.alias_resolved`, and `schema.enum` on the derived domain.
- `yomihon left-open` JSON: a thought item has no `status`; `update_date` is now `create_date` and items sort by `created`.
- Search: the `topic:` filter is removed and `status:` became `draft:`; `domain:` reads the derived domain.
- `exists` no longer compares `title_en`; the markdown report of `check` begins with frontmatter that holds only `type: report`.

## Platform limits

- Obsidian's handling of a boolean `draft` property and of `aliases` on a note that was deleted has not been checked.
