# Glossary

One word for one idea. This page decides the words the product writes on screen, in its documentation and on its command line. Where a word is retired, the word beside it is the one to write. Code may keep its own names, but each is listed here next to the word a reader sees, so nobody has to guess which is which.

A vault's own words belong to that vault, not to this page: the note types and domains its `System/schemas/vault-schema.toml` declares. The example vault calls a course a `study-path`; that is the vault's word.

## Decided

| Idea | Screen word | 畫面用語 | Means | Retired | In code |
| --- | --- | --- | --- | --- | --- |
| The folder of Markdown yomihon reads | vault | 書庫 | The folder yomihon is pointed at. A directory inside it is a *folder*; the vault's own root is a folder only when the subject is the disk. | library, 知識庫 | `vault`, `internal/vault` |
| An ordered run of lessons | course | 課程 | Lessons grouped in parts along a main line, with optional side branches of extra reading. | study path, syllabus, 學習路徑 | `nav.Path` (with `PathView`, `PathGroup`, `IsPathType` and the contract's `course` role), `Syllabus*`, `internal/syllabus`, `Course*`. They keep their names. The course page is at `/courses/…` and the list of courses at `/courses`; the retired `/syllabus/…` and `/paths` answer with a permanent redirect to them. |
| Where a file sits in the vault | path | 路徑 | The route from the vault's root to a file, such as `Notes/Books/Go.md`. It never names a course. | | `path`. In code `nav.Path` and `IsPathType` name a course; this is the one place the code says path for a course. |
| Where the reader stopped | place | 待續位置 | What "Leave off here" keeps. Written with 待續 in full, not as 位置 alone. | | `Continuation`. `nav.Place` is a third, unrelated sense (where a note sits in a course) and is to be renamed; `isContinuation` in `sequence` is a course row's continuation paragraph, not this. |
| A spot the reader is unsure of | location | 位置 | A spot in a note marked "Not sure yet". The thing marked is a location; the act is to *mark* it. | place, in this sense | `Uncertainty`. Only the wording names `UncertaintyPlace*` move to `Location`: `mark.Places` and `HasPlace` serve both marks and name an anchor a note renders, so they keep their names. |
| One problem the check reports | finding | 發現 | A problem that `yomihon check` and the Health page report about a note or the vault, and what the note page's Note health panel shows about one note. | diagnostic, 診斷 | `judge.Finding`. `Diagnostic` in code means four other things (render, sequence, vault, schema); the Note health panel shows `render.Diagnostic`. |
| A written report | report | 報告 | A Markdown report written directly in the vault's `System/reports/`, listed at `/reports`, newest first. | health report, 健康報告, adjudication report, 判讀報告 | |
| The page that lists findings | Health page | 整體狀況 | The page at `/health`. It is a page, not a report. | | `Health` |
| Looking for findings | check | 檢查 | The act, and the command `yomihon check`. | judge, as the name of this act; adjudication | `internal/judge`, `adjudicate` |
| The reader's own writing | thought | 想法 | A note of the reader's own that cites a source, of the type that holds the contract's `thought` role. | | `thought`, `/thought/…` |
| What the reader has not settled | What you left open | 還沒想通的 | The shelf of locations marked "Not sure yet" and thoughts that still declare `draft: true`, newest first. The command `yomihon left-open` lists the same things. | | `open-thoughts`, `/open-thoughts` |
| A note its author says is unfinished | draft | 草稿 | A note whose frontmatter says `draft: true`. A finished note carries no word at all. | | `draft`, `schema.DraftField`, `wording.Draft` |
| The home page | reading desk | 閱讀桌 | The page at `/`, where the reader returns. | | `Desk`, `HomeView` |
| The column beside the page | sidebar | 側欄 | The navigation column. | rail | `ReadingRail`, `Rail*` |
| A list of rows on the desk | shelf | 書架 | One organisation's listing of documents. | | `Shelf` |
| A heading's part of a note | section | 小節 | The part of a note under one heading, as `#Heading` addresses it. | | |
| A paragraph | paragraph | 段 | One paragraph; the unit read-aloud moves by. | | |
| A `^id` target | block | 區塊 | A paragraph or item named with `^id`. | | |
| One unit of a course | lesson | 課 | | 課文, 課次 | |
| The vault's rule file | contract | 契約 | `System/schemas/vault-schema.toml`. The Chinese screen text does not write `vault contract`. | | |
| The contract's field rules | schema | schema | | | |
| The note page's own findings panel | Note health | 筆記狀況 | | | |
| A picture shown inline | picture | 圖片 | | | `IsPicture` |
| Placing a file inside a note | embed | 嵌入 | | 內嵌 | |
| An interactive HTML file a note embeds | figure | 圖解 | One self-contained `.html` file in the vault, embedded with `![[name.html]]` and shown in a frame of its own. Without approval it runs no script. | | `figure`, `internal/figure`; the frame's document is served at `/embed/…` |
| Letting one figure run its scripts | approve | 核可 | The owner's word, given at a terminal, that a figure's exact bytes may run their inline scripts. Taking it back is to revoke it (撤銷核可); a change to the file lapses it (核可已失效). | | `approve-figure`, `revoke-figure`, `mark.FigureApproval` |
| Any file in the vault | file | 檔案 (counted 個) | A file of any kind: a note, a picture, a PDF. | | |
| A Markdown file yomihon reads as one | note | 筆記 (counted 篇) | A Markdown file in the index: rendered, searched, linked to and checked. | | |
| A Markdown file the contract leaves out | document | 文件 (counted 份) | A Markdown file the contract leaves out of the index. It still opens and renders; it is not searched or checked. | | `FileDocument`, `SkipsNote` |
| A directory | folder | 資料夾 | A directory inside the vault. | | |

A written report is counted 份 as well. Not counting by the right word reads as careless in Chinese: 篇 for a note, 份 for a document or a report, 個 for a file.

The command line's own words are decided with the command that needs them; those decided so far are below.

## The command line's words

The command line speaks English only. These are the words `yomihon left-open` writes, one JSON object per item. A field with no value is left out, except a lost location's `anchor`.

| Word | Means | In code |
| --- | --- | --- |
| `left-open` | The command that lists what the reader left open: the same items, in the same order, as the What you left open shelf. | `openCommand` |
| `kind` | `location` for a location marked "Not sure yet", `thought` for a thought that declares `draft: true`. | `openKindLocation`, `openKindThought` |
| `path` | The item's path. | |
| `anchor` | The id the page gives the location inside its note. Left out for a mark on the whole note. Present and empty when `lost` is set: the id is made from a heading's words, and with the heading gone they cannot be checked against withheld ground. | |
| `anchor_text` | The words the page shows for that location: a heading, the title, or a practice card's pattern. | `MarkedLocation.AnchorText` |
| `title` | The note's first `#` heading. Left out when it has none; the file name never stands in. | `DeclaredTitle` |
| `wikilink` | The vault's own link to the item, brackets included, as a thought's `based_on` writes it. Left out where it could reach somewhere else. | `NoteWikilink`, `MarkedLocation.Wikilink` |
| `lost` | What a location no longer finds: `note`, or `location` when the note is there and the location is not. | `note.Loss`, `openLostNote`, `openLostLocation` |
| `create_time` | When the location was marked, in RFC 3339 and in UTC. | |
| `create_date` | The date a thought's `created` field declares. Thoughts sort among the marks by it, newest first, and a thought with none sorts after every dated item. | |

## Frozen outputs

`yomihon check` keeps some of the retired words (*syllabus*, *study-path*, *judge*) in its rule messages and in the golden files under `internal/judge/testdata/`. They are not debt. External pipelines read them byte for byte, and they change only with the owner's ruling. A vocabulary change does not touch the strings in `internal/judge`.

## Known debt the test cannot see

The test below compares whole words, so it cannot tell a retired sense of a word from a sound one. These are known, found by reading, and replaced in the redesign. Confirm each before changing it.

- 位置 for something other than a location or a place: a storage place at `desk.go:72` and `freshness.go:18`.
- Section written three ways in Chinese (段落, 小節, 章節): `diagnostic.go:17,18,23,41,42,44,45`; `render.go:15,20,135`; `note.go:97,99`.
- Lesson written three ways (課, 課文, 課次): `syllabus.go:124,127`; `preferences.go:84`.
- *Image* where *picture* is meant: `slotmachine.go:23`.
- The Note health panel called "the note's conditions" in English: `render.go:113`.

## Enforcement

The two blocks below are read by `internal/archlock/glossary_test.go`. It fails when a bilingual string in `internal/wording` uses a retired word beyond the allowance in `internal/archlock/testdata/glossary-debt.txt`, and it fails when an allowance is higher than the uses left, so the allowance only ever goes down. Each line is `retired => replacement`. A word matches whole, case-insensitively for English, together with its plural (`-s`, `-es`) and `'s`; an irregular plural needs a line of its own. Only the wording is read: a word that is ordinary English elsewhere, such as the verb *judge* in a sentence, is not retired there.

```retired-en
library => vault
libraries => vaults
study path => course
syllabus => course
diagnostic => finding
health report => Health page
adjudication => check
rail => sidebar
```

```retired-zh
學習路徑 => 課程
診斷 => 發現
健康報告 => 整體狀況
判讀報告 => 檢查結果
知識庫 => 書庫
內嵌 => 嵌入
```
