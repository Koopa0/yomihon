# Example library

Run `yomihon examples/vault`. The home page offers two books:

- [Go concurrency, first steps](vault/Notes/Books/Go%20並行入門.md): a five-lesson main line, from starting a goroutine to a fixed number of workers; timeout handling is optional reading.
- [Reading Japanese at the library](vault/Notes/Books/在圖書館讀日文.md): two dialogues, readings, read-aloud and sentence-pattern substitution.

The books teach their subjects directly. The table below is for maintainers checking features; none of it appears in the lessons.

## Feature map

| Feature and implementation | What the library holds | What to check |
| --- | --- | --- |
| Courses and side branches: `internal/sequence`, `internal/nav`, `internal/syllabus` | The five Go lessons; G06 as optional reading under G04; two Japanese lessons | The lesson after G04 is G05; G06 is not inserted into the main line |
| Concepts and maps: `internal/lesson/concept.go`, `internal/render/concept.go`, `internal/nav/map.go` | Goroutine lifecycle, channel handoff, cancellation and cleanup; Japanese location particles | Links in the lessons open concept pages; all five concepts are mounted on a map |
| Aliases and search: `internal/graph`, `internal/lexical`, `internal/search` | Titles, aliases and topics of the Go lessons; two programs, one blocking and one fixed | `取消 domain:go` finds the material; adding `status:draft` finds the blocking version |
| Sources and comparison: `internal/snapshot/basedon.go`, `internal/note/compare.go` | [提早返回的管線](vault/Notes/go/提早返回的管線.md) declares the fixed version as its source | Open the side-by-side comparison from the draft and check how send exits |
| Sections, blocks and excerpts: `internal/render` | G04 quotes a specific passage of the cancellation note | Heading and block links and embeds are valid; a quoted body is not counted as precise declared-source support |
| Diagrams, code and footnotes: `internal/render`, `internal/asset` | The local [Go pipeline diagram](vault/System/assets/go-pipeline.svg), complete programs, official documentation as sources | The local diagram needs no network; the complete programs run |
| Japanese read-aloud: `internal/render/tts.go`, `internal/syllabus/listen.go` | Four Japanese passages in J01/J02, with ruby | Readings toggle; speech uses ja; actual sound depends on the browser's voices |
| Sentence-pattern practice: `internal/lesson/slot.go`, `internal/note/handler.go` | Nine combinations for finding a book, six for reading places | Matched by slug; the Japanese and its translation update together |
| Diary and reports: `internal/nav/nav.go`, `internal/report` | The 9/22 cancellation diary, a Markdown review, an HTML case | That day's content opens; the HTML is self-contained CSS with no script or external resource |
| Resume, freshness and preferences: `internal/mark`, `internal/note/freshness.go`, `internal/preference` | Complete lessons and long programs that can be returned to | Save a position and return from the home page; an outside edit to the copy shows a notice; narrow widths and large text stay readable |
| Contract, diagnostics, supersession and privacy: `internal/schema`, `internal/judge`, `internal/status` | Handbooks in both languages, deliberate faults, old and new startup notes | The diagnostics list, the supersession relation and the Diary privacy scope are kept |

The Go books add `go` to the example contract's domain list. Fields, statuses and lifecycle rules are unchanged.

## Deliberate diagnostics

`yomihon check --root examples/vault --format json --all` lists three:

| Rule | File |
| --- | --- |
| `schema.enum` | `Notes/A note with a fault in its frontmatter.md` |
| `link.broken` | `Notes/Wikilinks in this dialect.md` |
| `link.section_missing` | `Notes/Wikilinks in this dialect.md` |

Without `--deny` it exits 0; both `--deny warn` and `--deny error` exit 1. Ordinary lessons should add no diagnostics. `coverage --format json` should show all five concepts mounted: three for go, one for japanese, one for yomihon.

## Checking in a copy

```sh
lab=$(mktemp -d)
cp -R examples/vault/. "$lab/"
yomihon check --root "$lab" --format json --all
yomihon coverage --root "$lab" --format json
yomihon "$lab"
```

Open G04 and leave a resume position, then edit the lesson in the copy with an editor. The original page should show an update notice, and the home page's resume entry should say the content has changed. Do not change the shared demo's state or leave personal records behind.
