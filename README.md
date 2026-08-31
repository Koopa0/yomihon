<h1><img src="assets/brand/yomihon-mark.svg" width="36" height="36" alt="" aria-hidden="true"> yomihon</h1>

English | [繁體中文](README.zh-TW.md)

[![CI](https://github.com/koopa0/yomihon/actions/workflows/ci.yml/badge.svg)](https://github.com/koopa0/yomihon/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue?style=flat)](LICENSE)

**A focused reading room for your Markdown knowledge base.**

yomihon turns a folder of notes into a calm, local reading experience. Read
long-form Markdown, keep its links and context close, follow learning
structures when they exist, and decide when a note is ready—without moving
your knowledge into another database.

[![yomihon reading a note from the example vault with its interface in English: the navigation rail and study path on the left, the article in the centre, and on the right its status with the one transition it can take, the sections of the page, and what cites it](.github/media/reading-en.png)](.github/media/reading-en.png)

*The interface follows the language you choose — English here, Traditional
Chinese by default. Your notes are not translated: each keeps the language its
author wrote it in. The screenshot is the example vault this repository ships,
retaken with `make screenshots`.*

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
- **Two languages, one set of notes.** Everything yomihon says in its own voice
  reads in English or Traditional Chinese, switched from the header. Your notes
  are not translated: a note that declares its own language keeps it whichever
  language the interface is speaking around it.

## Start reading

You need Git and Go 1.27.0 or newer. Install from source:

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

One environment variable configures yomihon: `YOMIHON_PORT` picks the local
port (default `9610`). The listener always stays on `127.0.0.1`.

A plain folder is readable without changing its files. To opt in to governed
metadata, structured navigation, and lifecycle actions, add a vault contract at
`System/schemas/vault-schema.toml`.

[`examples/vault`](examples/vault) is a complete small vault you can read right
now — `yomihon examples/vault` — with a contract, a study path, a map, lessons,
and one deliberately unwritten link so you can see what a diagnostic looks like.
Its [contract](examples/vault/System/schemas/vault-schema.toml) is the one to
copy. A test scans that vault on every run, so the example cannot quietly stop
working: it is the file nobody with a working vault ever opens again.

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
- **Bound to what you read.** A status change carries the identity of the bytes
  the page showed you. If the note changed on disk in between, the write is
  refused rather than applied to a version you never saw — and the reading page
  tells you the note has moved on before you press anything.
- **No network use at all.** yomihon opens your files and a loopback socket.
  It has no client, no credential, and nothing to send: there is no request it
  could make even if you wanted one.

## Platform support

| Capability | macOS | Linux | Windows |
|---|---:|---:|---:|
| Reading, navigation, diagnostics, lexical search | Yes | Yes | Yes |
| `status` writes | Yes | Yes | No |

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
