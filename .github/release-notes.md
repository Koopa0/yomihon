<!-- Goes above GitHub's generated notes when a release is published (see CONTRIBUTING). Edit it for each release; do not append to it. -->

Since v0.2.0, yomihon has become a reading-first interface: the reading room has a calmer look, pages change without flashing, the header carries only what reading needs, and the screen says one thing in one word.

## What is new

- The screen text uses one word for one idea, in English and in Chinese: a vault (書庫), a course (課程), the Health page (整體狀況), a finding (發現事項 in the Health table). The header shows only the name yomihon, and the search box says "Search…" (搜尋…).
- Courses open like a book's cover and count their parts in the noun the author chose. A side branch has a way back and on.
- Reports are the Markdown reports written in the reports directory, and each begins with the question it answers.
- A note shows only the statuses a reader needs to notice, and status changes are refused on read-only notes.
- "Leave off here" keeps your place on this device, and the reading desk links back to it. A location you mark "Not sure yet" and a thought you leave unfinished come back on the desk under "What you left open".
- `yomihon left-open` lists the same items for another program to read: each location marked "Not sure yet" and each thought still in a status the contract declares initial, newest first, as JSON Lines or as text. It reads nothing from a directory the contract withholds, and refuses when the contract declares no `[privacy]` section.
- A note can embed an interactive HTML figure with `![[name.html]]`. A figure is shown in a sandboxed frame and runs no script. `yomihon approve-figure <path>` shows a figure's scripts and lets it run them only while its bytes stay exactly as approved; `yomihon revoke-figure <path>` takes that back. An approved figure's scripts run in your browser and can send data anywhere.
- The Japanese cells of a lesson table can be read aloud.
- Code can be copied from every reading surface.
- Search indexes frontmatter tags, and a Chinese, Japanese or English match lands in the reading body.
- Wikilinks resolve from path suffixes and nested heading paths, and a Markdown link to a folder opens the folder page.
- `yomihon check` reports a link reference definition that nothing uses, and no longer reads a command line in a code span as a vault path.
- `scan.exclude_dirs` keeps matching directories out of the scan.
- A frontmatter merge key, or a note that fails to parse, no longer stops the server. Return addresses, links in vault markup and the depth of the contract are bounded.
- Release binaries are built with Go 1.27.4 and `-trimpath`, and stripped of symbols, so they are smaller. Dependencies are upgraded, templ to v0.3.1100 among them.

## Platform limits

- On Windows, yomihon reads a vault but does not change a note's status.
