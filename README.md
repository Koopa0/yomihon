<h1><img src="assets/brand/yomihon-mark.svg" width="36" height="36" alt="" aria-hidden="true"> yomihon</h1>

English | [繁體中文](README.zh-TW.md)

[![CI](https://github.com/koopa0/yomihon/actions/workflows/ci.yml/badge.svg)](https://github.com/koopa0/yomihon/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue?style=flat)](LICENSE)

**Turn the Markdown you have already organised into a book worth reading.**

[![yomihon reading the first chapter of a course from the example vault: the course and where you are in it on the left, the article set for long-form reading in the centre, and on the right its sections and the notes that cite it](.github/media/reading-en.png)](.github/media/reading-en.png)

Notes accumulate in folders; understanding happens when they are read back.
yomihon serves a folder of Markdown as one book. A study path opens as a
course with chapters and a marker for where you are; a note opens with its
sections and everything that cites it beside the text; whatever is broken in
the collection is said where it happens. It runs on your machine and reads the
notes where they already are.

## Install

```sh
go install github.com/koopa0/yomihon/cmd/yomihon@latest
```

Needs Go 1.27 or newer.

## Use

```sh
yomihon serve ~/notes
```

Then open <http://127.0.0.1:9610>. Any folder works as it is;
`yomihon serve examples/vault` shows what a vault contract adds.

## What it does

- **Read.** Wikilinks, callouts, footnotes, tables, Mermaid, highlighted code
  and ruby render as the author meant them. The page is set for long-form
  reading, Chinese and Japanese first, with a light and a dark desk and three
  text sizes.
- **Learn.** A study path becomes a course: chapter counts, previous and next,
  and where you are. Furigana switches on and off; a passage marked for reading
  aloud is spoken in its own language (the example vault carries
  [a path to copy](examples/vault/Notes/Reading%20yomihon.md)).
- **Find.** Lexical search with folder filters and a way back from zero
  results; backlinks and the note's own sections stay beside the text.
- **Keep the collection honest.** A health page lists links with no target,
  notes nothing cites, and one name two files answer to; each note carries its
  own diagnostics; the same checks run as `yomihon check` on the command line.
  Nothing is repaired for you — you edit the file.
- **Reports.** Daily briefings kept under the vault's
  `System/reports/daily-briefing/` folder open inside the same room, sandboxed.
- **Two languages.** The interface speaks English or Traditional Chinese; every
  note keeps the language it was written in.
- **Yours.** It binds to `127.0.0.1` only, never makes a network call, and
  never edits your prose.

## Status

Under active development; expect product and interface changes before the first
stable release. Defects go to [Issues](https://github.com/koopa0/yomihon/issues),
security problems to
[GitHub's private advisory form](https://github.com/koopa0/yomihon/security/advisories/new).

## Licence

[MIT](LICENSE). Redistributed fonts and client assets carry their own licences:
[`assets/fonts/LICENSE.txt`](assets/fonts/LICENSE.txt) and
[`assets/js/mermaid/LICENSE`](assets/js/mermaid/LICENSE).
