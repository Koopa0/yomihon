# Glossary

One word for one idea. This page decides the words the product writes on screen, in its documentation and on its command line. Where a word is retired, the word beside it is the one to write. Code may keep its own names, but each is listed here next to the word a reader sees, so nobody has to guess which is which.

A vault's own words belong to that vault, not to this page: the note types and statuses its `System/schemas/vault-schema.toml` declares. The example vault calls a course a `study-path`; that is the vault's word.

## Decided

| Idea | Screen word | 畫面用語 | Means | Retired | In code |
| --- | --- | --- | --- | --- | --- |
| The folder of Markdown yomihon reads | vault | 書庫 | The folder yomihon is pointed at. A directory inside it is a *folder*. | library | `vault`, `internal/vault` |
| An ordered run of lessons | course | 課程 | Lessons grouped in parts along a main line, with optional side branches of extra reading. | study path, syllabus, 學習路徑 | `Course`. The course page is `Syllabus` and its address `/syllabus/…`; both are to be renamed, with a redirect. |
| Where a file lives | path | 路徑 | A file's location in the vault, such as `Notes/Books/Go.md`. It never names a course. | | `path` |
| Where the reader stopped | place | 待續位置 | What "Leave off here" keeps. | | `Continuation` |
| A spot the reader is unsure of | location | 位置 | A spot in a note marked "Not sure yet". The thing marked is a location; the act is to *mark* it. | place, in this sense | `Uncertainty`. The code that resolves one calls it `Place`; to be renamed `Location`. |
| One problem the check reports | finding | 發現 | A problem that `yomihon check` and the Health page report about a note or the vault. | diagnostic, 診斷 | `Diagnostic` |
| A written report | report | 報告 | A Markdown report the vault keeps under its reports directory, listed at `/reports`. | health report, 健康報告 | |
| The page that lists findings | Health page | 整體狀況 | The page at `/health`. It is a page, not a report. | | `Health` |
| Looking for findings | check | 檢查 | The act, and the command `yomihon check`. | judge, as the name of this act | `internal/judge` |
| The reader's own writing | thought | 想法 | A note of the reader's own that cites a source, of the type the contract names as its answer type. | | `thought`, `/thought/…` |
| What the reader has not settled | What you left open | 還沒說清楚的 | The shelf of locations marked "Not sure yet" and thoughts not yet finished, newest first. | | `open-thoughts`, `/open-thoughts` |

Two things are deliberately not here yet. The words *note*, *document*, *file* and *folder* mean different things on purpose and are kept apart; their definitions are still to be written. The command line's own words, its command and field names, are decided when the command that needs them is.

## Enforcement

The two blocks below are read by `internal/archlock/glossary_test.go`. It fails when a bilingual string in `internal/wording` uses a retired word beyond the allowance in `internal/archlock/testdata/glossary-debt.txt`, and it fails when an allowance is higher than the uses left, so the allowance only ever goes down. Each line is `retired => replacement`. A word matches whole, case-insensitively for English, together with its plural (`-s`, `-es`) and `'s`; an irregular plural needs a line of its own. Only the wording is read: a word that is ordinary English elsewhere, such as the verb *judge* in a sentence, is not retired there.

```retired-en
library => vault
libraries => vaults
study path => course
syllabus => course
diagnostic => finding
health report => Health page
```

```retired-zh
學習路徑 => 課程
診斷 => 發現
健康報告 => 整體狀況
```
