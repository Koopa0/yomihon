<h1><img src="assets/brand/yomihon-mark.svg" width="36" height="36" alt="" aria-hidden="true"> yomihon</h1>

[![CI](https://github.com/koopa0/yomihon/actions/workflows/ci.yml/badge.svg)](https://github.com/koopa0/yomihon/actions/workflows/ci.yml)
[![Go 1.26.6+](https://img.shields.io/badge/Go-1.26.6%2B-00ADD8?style=flat&logo=go&logoColor=white)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue?style=flat)](LICENSE)

yomihon is a local, single-user web interface for reading a personal Markdown
vault and deciding what to do about what you read. It renders your notes as
calm, navigable pages, and it writes exactly one thing back: a note's lifecycle
`status`, as one git commit under your own name.

[![Yomihon's Traditional Chinese browser interface, with a note open for
reading: the vault's notes down the left, the article in the middle in a serif
face, and the note's status, contents, and relations down the
right](.github/media/reading.png)](.github/media/reading.png)

*The browser chrome is intentionally in Traditional Chinese. Vault notes keep
their own declared language; this screenshot was captured from a demonstration
vault.*

> [!WARNING]
> yomihon is under active development. Expect significant feature and
> interface changes before the first stable release.

## Features

- Renders Markdown, wikilinks, transclusions, callouts, ruby, Mermaid, source
  files, images, PDFs, and HTML reports as one reading surface.
- Builds navigation, backlinks, a table of contents, and lexical search from
  the current vault snapshot.
- Presents study paths, lesson tools, vault diagnostics, and concept coverage
  without changing your notes.
- Answers diagnostic and search commands for local agents.
- Advances one note's `status` through your vault's own lifecycle rules and
  records the accepted transition as one git commit.

## Boundaries

1. **One vault write.** yomihon updates only the frontmatter `status` field.
   Every other vault byte is read-only.
2. **Local by default.** The server has no authentication and listens only on
   `127.0.0.1`. Reading and lexical search stay on your machine; provider
   egress requires an explicit semantic action and your own credential.
3. **The vault owns its contract.** The legal types and fields, the lifecycle,
   the privacy boundary, and which document types may take part in path and
   map behaviour all come from `System/schemas/vault-schema.toml`, never a
   copy compiled into the binary.
4. **Ordinary Markdown needs no changes.** A folder of notes reads, links,
   searches and renders as it is. Structured presentation — a course with an
   order, a side branch, a block that stays out of navigation — is opt-in, and
   the body syntax for it belongs to yomihon rather than to your vault:
   see [`AUTHORING.md`](AUTHORING.md). A note that declares nothing keeps
   reading as prose; a course that declares its structure gets one.
5. **Problems stay visible.** Invalid metadata and broken or ambiguous links
   become diagnostics. The renderer never guesses or repairs your notes.

## Requirements

- Go 1.26.6 or newer.
- A vault contract at `System/schemas/vault-schema.toml`. Start from
  [`examples/vault-schema.toml`](examples/vault-schema.toml) and adapt it to
  your own directories and note types. yomihon diagnoses a missing or invalid
  contract, but never creates or edits one.
- To change a `status`: the vault must be a git repository, and the note must
  have no uncommitted changes. yomihon refuses rather than mix its one-line
  edit into work you have not committed. Reading, search, and diagnostics need
  no git.

## Install

```sh
git clone https://github.com/Koopa0/yomihon.git && cd yomihon
go install ./cmd/yomihon
```

This repository is private, so `go install <module>@<version>` cannot resolve
it — the clone supplies the source. The built binary carries every asset it
serves and runs without the repository beside it.

## Run

Read the folder you are standing in, or one you name:

```sh
cd /path/to/vault && yomihon
```

```sh
yomihon /path/to/vault
```

Home loads at `http://127.0.0.1:9610` once the log says `yomihon serving`. An
invalid vault path exits non-zero. A missing or invalid contract is reported on
the page, leaves reading available, and closes the write face.

Every command reads the current folder unless told otherwise, so `yomihon
check` and `yomihon search <query>` answer about the folder `yomihon` serves.
Run `yomihon help` for the list, or `yomihon <command> --help` for one.

Each process holds one folder for its lifetime, so reading a second vault means
a second `yomihon` on a second port.

## Configure

Which folder to read is not configuration: it is the one you are standing in,
or the one you name on the line.

| Variable | Purpose | Default |
|---|---|---|
| `YOMIHON_PORT` | Listen port. The address itself is always `127.0.0.1`. | `9610` |

### Semantic search

Semantic retrieval is an optional CLI action and the one place yomihon talks to
a network service. It is off unless you set `YOMIHON_EMBED_KEY` to your own
Gemini credential; the vectors it computes are stored locally, and the server
never uses it.

## Platform support

| | macOS | Linux | Windows |
|---|---|---|---|
| Reading, navigation, diagnostics, lexical search | yes | yes | yes |
| `status` writes, semantic generations | yes | yes | no |

yomihon is a personal application, not a multi-user service, and is
deliberately not published as a container image: the listener binds `127.0.0.1`
in its own network namespace, which a host cannot reach from inside a
container, and the `status` commit runs `git` as you so it carries your
identity. Use [GitHub Issues](https://github.com/Koopa0/yomihon/issues) for
questions and defects.

## License

[MIT](LICENSE). Redistributed fonts and client assets are documented in
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
