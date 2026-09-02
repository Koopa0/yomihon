<h1><img src="assets/brand/yomihon-mark.svg" width="36" height="36" alt="" aria-hidden="true"> yomihon</h1>

English | [繁體中文](README.zh-TW.md)

[![CI](https://github.com/koopa0/yomihon/actions/workflows/ci.yml/badge.svg)](https://github.com/koopa0/yomihon/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue?style=flat)](LICENSE)

**A local reading room for your Markdown notes.**

[![yomihon reading a note from the example vault: navigation and a study path on the left, the article in the centre, and on the right the note's status, its sections, and what cites it](.github/media/reading-en.png)](.github/media/reading-en.png)

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

- Reads a folder of Markdown as one book: wikilinks, callouts, ruby, Mermaid,
  footnotes, tables, and highlighted code.
- Turns a study path into a course — counts, prev and next, and where you are
  in it ([how to write one](AUTHORING.md)).
- Keeps backlinks, the note's own sections, and lexical search beside the text.
- Reports what is broken — invalid frontmatter, links with no target, one name
  two files both answer to — and never repairs it.
- Speaks English or Traditional Chinese; your notes keep the language they were
  written in.
- Writes one field, `status`, following the transitions your vault declares
  (everywhere but Windows). Nothing else is written, and nothing is sent
  anywhere: no network call, ever.

## Status

Under active development; expect product and interface changes before the first
stable release. Defects go to [Issues](https://github.com/koopa0/yomihon/issues),
security problems to
[GitHub's private advisory form](https://github.com/koopa0/yomihon/security/advisories/new).

## Licence

[MIT](LICENSE). Redistributed fonts and client assets are listed in
[`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).
