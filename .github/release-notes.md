<!-- The text above GitHub's generated notes on the release page. Edit it for each release; do not append to it. -->

Since v0.2.0, yomihon has become a reading-first interface: the reading room has a calmer look, pages change without flashing, and the header carries only what reading needs.

## What is new

- Study paths open like a book's cover and count their parts in the noun the author chose. A side branch has a way back and on.
- Reports begin with the question they answer.
- A note shows only the statuses a reader needs to notice, and status changes are refused on read-only notes.
- "Leave off here" keeps your place on this device, and the home page links back to it.
- Code can be copied from every reading surface.
- Search indexes frontmatter tags, and a Chinese, Japanese or English match lands in the reading body.
- Wikilinks resolve from path suffixes and nested heading paths.
- `scan.exclude_dirs` keeps matching directories out of the scan.
- A frontmatter merge key, or a note that fails to parse, no longer stops the server.
- Release binaries are built with `-trimpath` and stripped of symbols, so they are smaller.

## Platform limits

- On Windows, yomihon reads a vault but does not change a note's status.
