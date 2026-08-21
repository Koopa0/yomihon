<h1><img src="assets/brand/yomihon-mark.svg" width="36" height="36" alt="" aria-hidden="true"> yomihon</h1>

English | [繁體中文](README.zh-TW.md)

[![CI](https://github.com/koopa0/yomihon/actions/workflows/ci.yml/badge.svg)](https://github.com/koopa0/yomihon/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue?style=flat)](LICENSE)

**A focused reading room for your Markdown knowledge base.**

yomihon turns a folder of notes into a calm, local reading experience. Read
long-form Markdown, keep its links and context close, follow learning
structures when they exist, and decide when a note is ready—without moving
your knowledge into another database.

[![An English note open in yomihon, with a Study Path and Map on the left, the article in the centre, and its status, contents, and backlinks on the right](.github/media/reading-en.png)](.github/media/reading-en.png)

*The browser chrome is intentionally in Traditional Chinese; each note keeps
its authored language. This screenshot uses a synthetic demonstration vault.*

> [!WARNING]
> yomihon is under active development. Expect meaningful product and interface
> changes before the first stable release.

## Why yomihon

- **Read without migration.** Ordinary Markdown remains ordinary Markdown.
  Notes, links, images, callouts, Mermaid diagrams, PDFs, and reports are
  presented as a coherent reading surface. Math notation is displayed as
  written, not typeset; characters inside `$…$` spans follow ordinary
  Markdown emphasis rules.
- **See the right structure at the right time.** Navigation, headings,
  backlinks, and lexical search stay close to the text. Opt-in Study Paths
  express ordered progress; Maps express relationships; Reports remain
  reports. They are distinct surfaces, not one universal markup system.
- **Keep the final decision human.** yomihon is a reader, not an editor. It can
  advance one note's approved `status`; prose revisions stay in the writing
  tool you already use.

## Start reading

You need Git and Go 1.26.6 or newer. Install from source:

```sh
git clone https://github.com/Koopa0/yomihon.git
cd yomihon
go install ./cmd/yomihon
```

Then point yomihon at a Markdown folder:

```sh
yomihon /path/to/vault
```

Open `http://127.0.0.1:9610`. You can also run `yomihon` from inside the
folder you want to read.

Two environment variables configure yomihon: `YOMIHON_PORT` picks the local
port (default `9610`; the listener always stays on `127.0.0.1`), and
`YOMIHON_EMBED_KEY` holds your embedding provider credential for the explicit
semantic actions — it is used only to build local search vectors from note
content the vault's privacy contract allows, and to embed the query text of a
semantic search you request.

A plain folder is readable without changing its files. To opt in to governed
metadata, structured navigation, and lifecycle actions, add a vault contract
at `System/schemas/vault-schema.toml`; start with
[`examples/vault-schema.toml`](examples/vault-schema.toml).
[`AUTHORING.md`](AUTHORING.md) documents yomihon's opt-in Study Path body
syntax; Maps and Reports keep their own document roles.

## Trust by design

- **Local and single-user.** The server listens only on `127.0.0.1` and has no
  remote or multi-user mode.
- **One narrow write.** Reading, rendering, search, and diagnostics do not
  change vault content. An authorized lifecycle action changes only `status` —
  a single frontmatter line, rewritten in place.
- **Visible failure.** Invalid metadata and broken or ambiguous links become
  diagnostics. yomihon does not guess at or silently repair your notes.
- **Optional network use.** Ordinary reading and lexical search stay local.
  Semantic actions are explicit, use your own provider credential, and respect
  the vault's privacy contract.

## Platform support

| Capability | macOS | Linux | Windows |
|---|---:|---:|---:|
| Reading, navigation, diagnostics, lexical search | Yes | Yes | Yes |
| `status` writes and semantic generations | Yes | Yes | No |

## Project

Questions and defects belong in [GitHub Issues](https://github.com/Koopa0/yomihon/issues).
Report security problems privately through
[GitHub's vulnerability reporting](https://github.com/Koopa0/yomihon/security/advisories/new)
rather than a public issue.

Contributions gate on `make verify`. `make tools` installs the pinned Go
analysis tools it expects; a tool built outside `go install` (a Homebrew
build, for example) carries no module version stamp and cannot pass the
exact-version check. The Makefile and CI workflow pin the remaining
prerequisites: the Tailwind CSS standalone CLI, ShellCheck, Node.js with npm
for the development-only frontend lint, and Chrome for the browser probes.

yomihon is released under the [MIT License](LICENSE). Redistributed fonts and
client assets are listed in
[`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).
