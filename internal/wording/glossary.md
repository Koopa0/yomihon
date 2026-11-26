# Glossary

One word for one idea. This page decides the words the product writes on screen, in its documentation, in its commands and in its code. Where a word is retired, the word beside it is the one to write.

A vault's own words belong to that vault, not to this page: the note types and domains its `System/schemas/vault-schema.toml` declares. The example vault calls a course a `study-path`; that is the vault's word, and the reader still accepts it.

## Decided

| Idea | Screen word | 畫面用語 | Means | Retired | In code |
| --- | --- | --- | --- | --- | --- |
| The folder of Markdown yomihon reads | vault | 書庫 | The folder yomihon is pointed at. A directory inside it is a *folder*. | library, 知識庫, folder or root for the vault | `vault`, `internal/vault` |
| A Markdown file yomihon reads as one | note | 筆記 (counted 篇) | A Markdown file in the index: rendered, searched, linked to and checked. | | |
| An ordered run of units | course | 課程 | Units grouped in parts along a main line, with optional side branches of extra reading. | syllabus, study path, 學習路徑 | `nav.Course`, `internal/course`, `Course*`. The page is at `/courses/…` and the list at `/courses`; the retired `/syllabus/…` and `/paths` answer with a permanent redirect. |
| One note of a course | unit | 單元 | Counted "N 單元" everywhere, in every course, so a page never mixes two nouns. | lesson, 課, 課文, 課次 | `UnitWords` |
| A Japanese sentence-pattern exercise | drill | 句型練習 | | the `lesson` package | `internal/drill` |
| A note that lists notes by subject | index | 索引 | The page is at `/indexes`; the retired `/maps` answers with a permanent redirect. | map, moc, 地圖 | |
| The optional rule file | rules | 規則 | `System/schemas/vault-schema.toml`. A vault needs none to be read. | contract, 契約 | `schema.Contract` and `Contract*` still carry the old word in code. |
| A note its author says is unfinished | draft | 草稿 | A note whose frontmatter says `draft: true`. A finished note carries no word at all. | | `draft`, `schema.DraftField`, `wording.Draft` |
| Notes by date | journal | 日記 | | 日誌 | `Journal*` |
| The page that lists findings | Health page | 整體狀況 | The page at `/health`. | health report, 健康報告, 健康頁 | `Health` |
| A note `based_on` points at | source | 出自 | | based on, 來源 | `BasedOn` |
| Anything the reader leaves | mark | 標記 | The page that lists them is "My marks", 我的標記, at `/marks`; the command is `yomihon marks`. The retired `/open-thoughts` answers with a permanent redirect and `left-open` is an alias of `marks`. | left open, 還沒想通的 | `mark`, `internal/mark` |
| A chosen stretch of text, marked | highlight | 劃線 | | | |
| A spot the reader does not understand | question | 疑問 | The control is "Mark as question", 標為疑問. The questions a reader marks are kept with the other marks, in the reader's file for this computer under `reader-notes/.yomihon/marks/`, so git carries them. | uncertainty, Not sure yet, 還不確定 | `mark.Question`, `/questions` (the retired `/uncertainties` answers with a permanent redirect that keeps the method) |
| A question the reader no longer has | resolved | 已解決 | The control is 標為已解決. | | |
| The reader's own writing | thought | 想法 | The control is "Write a note", 寫想法. A note of the reader's own that cites a source. | 留下自己的想法 | `thought`, `/thought/…` |
| Where the reader stopped | bookmark | 書籤 | The control is "Add bookmark", 加書籤. One for each course and one for reading outside any course, kept on this device. The Home block that returns to it is "Continue reading", 繼續閱讀. | continuation, kept place, 待續位置 | `mark.Bookmark`, `/bookmark` (the retired `POST /marks` answers with a permanent redirect that keeps the method) |
| Where the reader last was in a note | reading position | 閱讀位置 | Kept for each note without the reader pressing anything, in this browser's storage for this address; a note opens at it. The setting is "Reading position", 自動記閱讀位置. It is not the bookmark, which the reader adds and the server keeps. | place, resume, kept place, 記位置 | `yomihon_position`, `yomihon.positions`, `positions.js` |
| The ground the light theme is set on | paper | 紙色 | One of three, stone (石紙), washi (和紙) or white (白); a daytime choice, since the dark theme is one paper of its own, the night lamp (夜燈). | background, 底色 | `yomihon_paper`, `data-paper` |
| One problem the check reports | finding | 發現 | A problem that `yomihon check` and the Health page report about a note or the vault. | diagnostic, 診斷 | `judge.Finding` |
| A written report | report | 報告 (counted 份) | A Markdown report written in the vault's `System/reports/`, listed at `/reports`, newest first. | adjudication report, 判讀報告 | |
| A note's change on disk | change | 變更 | When a file last changed, as the desk's band of recent changes (最近變更, "Recently changed") and the order by latest change (依最近變更) name it. Modifying something, as a verb in a sentence, is still 改動 there. | 改動, for a note's change | `RecentlyChanged`, `SortByRecent`, `SearchSortRecent` |
| A note new notes start from | template | 範本 | A note in a folder the rules hold apart as templates. Its fields are the ones a note made from it starts with, so a template that writes `draft: true` is not a draft. | | `KindTemplate`, `wording.Template` |
| Looking for findings | check | 檢查 | The act, and the command `yomihon check`. | adjudication | `internal/judge`, `adjudicate` |
| The home page | reading desk | 閱讀桌 | The page at `/`, where the reader returns. | | `Desk`, `HomeView` |
| The column beside the page | sidebar | 側欄 | The navigation column. | rail | `ReadingRail`, `Rail*` |
| A list of rows on the desk | shelf | 書架 | One organisation's listing of documents. | | `Shelf` |
| A heading's part of a note | section | 小節 | The part of a note under one heading, as `#Heading` addresses it. | | |
| A paragraph | paragraph | 段 | One paragraph; the unit read-aloud moves by. | | |
| A `^id` target | block | 區塊 | A paragraph or item named with `^id`. | | |
| A picture shown inline | picture | 圖片 | | image | `IsPicture` |
| Placing a file inside a note | embed | 嵌入 | | 內嵌 | |
| An interactive HTML file a note embeds | figure | 圖解 | One self-contained `.html` file in the vault, embedded with `![[name.html]]` and shown in a frame of its own. Without approval it runs no script. | | `figure`, `internal/figure`; the frame's document is served at `/embed/…` |
| Letting one figure run its scripts | approve | 核可 | The owner's word, given at a terminal, that a figure's exact bytes may run their inline scripts. Taking it back is to revoke it (撤銷核可); a change to the file lapses it (核可已失效). | | `approve-figure`, `revoke-figure`, `mark.FigureApproval` |
| Any file in the vault | file | 檔案 (counted 個) | A file of any kind: a note, a picture, a PDF. | | |
| A Markdown file the rules leave out | document | 文件 (counted 份) | A Markdown file the rules leave out of the index. It still opens and renders; it is not searched or checked. | | `FileDocument`, `SkipsNote` |
| A directory | folder | 資料夾 | A directory inside the vault. | | |
| Where a file sits in the vault | path | 路徑 | The route from the vault's root to a file, such as `Notes/Books/Go.md`. It never names a course. | | `path` |

Not counting by the right word reads as careless in Chinese: 篇 for a note, 份 for a document or a report, 個 for a file, and 單元 for a unit of a course.

## On disk and on the wire

These keep their names so a vault and a reader's own files written by an earlier version still read and write as they did. Their format belongs to its own change.

- `reader.json`, `uncertainty.json` and `figures.json`, and the `yomihon/<digest>` directory they sit in, with every key inside them (the bookmark is still stored under `continuation`).
- A rules file's `role` values and a note's `type` values `study-path`, `lesson`, `moc` and `map`, which are accepted beside the new words. Documents teach the new words only.
- The words the command line writes: `kind` is `location` or `thought`, and `lost` is `note` or `location`. Their names are decided with the command that needs them.

## The command line's words

The command line speaks English only. These are the words `yomihon marks` writes, one JSON object per item. A field with no value is left out, except a lost location's `anchor`. `yomihon left-open` is the same command under its earlier name.

| Word | Means | In code |
| --- | --- | --- |
| `marks` | The command that lists what the reader marked: the same items, in the same order, as the My marks page. | `openCommand` |
| `kind` | `location` for a question the reader marked, `thought` for a thought that declares `draft: true`. | `openKindLocation`, `openKindThought` |
| `path` | The item's path. | |
| `anchor` | The id the page gives the location inside its note. Left out for a mark on the whole note. Present and empty when `lost` is set: the id is made from a heading's words, and with the heading gone they cannot be checked against withheld ground. | |
| `anchor_text` | The words the page shows for that location: a heading, the title, or a practice card's pattern. | `MarkedLocation.AnchorText` |
| `title` | The note's first `#` heading. Left out when it has none; the file name never stands in. | `DeclaredTitle` |
| `wikilink` | The vault's own link to the item, brackets included, as a thought's `based_on` writes it. Left out where it could reach somewhere else. | `NoteWikilink`, `MarkedLocation.Wikilink` |
| `lost` | What a location no longer finds: `note`, or `location` when the note is there and the location is not. | `note.Loss`, `openLostNote`, `openLostLocation` |
| `create_time` | When the location was marked, in RFC 3339 and in UTC. | |
| `create_date` | The date a thought's `created` field declares. Thoughts sort among the marks by it, newest first, and a thought with none sorts after every dated item. | |

## Retired words

Each line below is `retired => replacement`, for the bilingual strings in `internal/wording`. A word is retired together with its plural and `'s`; a word that is ordinary English elsewhere, such as the verb *judge* in a sentence, is not retired there. Nothing reads these blocks mechanically: a reviewer checks new screen text against them.

```retired-en
library => vault
libraries => vaults
study path => course
syllabus => course
lesson => unit
map => index
contract => rules
diagnostic => finding
health report => Health page
adjudication => check
rail => sidebar
uncertainty => question
Not sure yet => Mark as question
left open => marks
continuation => bookmark
kept place => bookmark
based on => source
```

```retired-zh
學習路徑 => 課程
課文 => 單元
課次 => 單元
地圖 => 索引
契約 => 規則
日誌 => 日記
診斷 => 發現
健康報告 => 整體狀況
判讀報告 => 檢查結果
知識庫 => 書庫
內嵌 => 嵌入
來源 => 出自
最近改動 => 最近變更
待續位置 => 書籤
還不確定 => 標為疑問
還沒想通的 => 我的標記
留下自己的想法 => 寫想法
```
