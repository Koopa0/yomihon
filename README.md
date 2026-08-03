<h1><img src="assets/brand/yomihon-mark.svg" width="36" height="36" alt="" aria-hidden="true"> yomihon</h1>

[![CI](https://github.com/koopa0/yomihon/actions/workflows/ci.yml/badge.svg)](https://github.com/koopa0/yomihon/actions/workflows/ci.yml)
[![Go 1.26.5+](https://img.shields.io/badge/Go-1.26.5%2B-00ADD8?style=flat&logo=go&logoColor=white)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue?style=flat)](LICENSE)

yomihon is a local, single-user web interface for reading and curating a
personal Markdown knowledge vault. It renders notes and other browsable,
non-hidden regular files as calm, navigable pages and keeps the human decision
at the center: read a note, understand its context, then advance its lifecycle
`status` without turning the reader into another editor.

> [!WARNING]
> yomihon is under active development. Expect significant feature and
> interface changes before the first stable release.

## What it does

- Renders Markdown, wikilinks, transclusions, callouts, ruby, Mermaid, source
  files, images, PDFs, and HTML reports as a focused reading surface.
- Builds navigation, backlinks, a table of contents, and local lexical search
  from the current vault snapshot.
- Presents study paths, lesson tools, vault diagnostics, and concept coverage
  without changing the underlying notes.
- Offers diagnostic and search commands for local agents, with semantic
  retrieval available only as an explicit, user-authorized CLI action.
- Advances one note's `status` through the vault's own lifecycle rules and
  records the accepted transition as one git commit.

## Boundaries

1. **One vault write.** Yomihon may update only the frontmatter `status` field.
   Every other vault byte remains read-only.
2. **Local by default.** The web server has no authentication and listens only
   on `127.0.0.1`. Ordinary reading and search stay local; provider egress
   requires an explicit semantic or certification action and the operator's
   own credential.
3. **The vault owns its contract.** Schema, privacy, navigation, and lifecycle
   capabilities come from `System/schemas/vault-schema.toml`, never a hidden
   copy in the binary.
4. **Problems stay visible.** Invalid metadata and broken or ambiguous links
   become diagnostics. The renderer never guesses or repairs source content.

## Getting started

Requires Go 1.26.5 or newer.

Every vault needs its own contract at
`System/schemas/vault-schema.toml`. The repository includes a parser-gated
starting point at [`examples/vault-schema.toml`](examples/vault-schema.toml).
Copy and deliberately adapt it before serving real content. Yomihon diagnoses
a missing or invalid contract but never creates or edits one.

Install the command:

```sh
git clone https://github.com/Koopa0/yomihon.git && cd yomihon
go install ./cmd/yomihon
```

This repository is private, so Go's module proxy cannot fetch it and
`go install <module>@<version>` will not resolve; the clone is what supplies
the source. Once it is built, every asset the reader serves — stylesheet,
client modules, fonts, the Mermaid runtime — is compiled into the binary, and
it runs anywhere without the repository beside it.

Read a vault:

```sh
yomihon serve --root /path/to/vault
# http://127.0.0.1:9610
```

`yomihon help` lists every command, and `yomihon <command> --help` explains one.

The vault path may also come from the environment, which is what a shell alias
or a launch agent will usually set; `--root` wins when both are given.

| Variable | Purpose | Default |
|---|---|---|
| `YOMIHON_ROOT` | Vault path | `~/obsidian` |
| `YOMIHON_PORT` | Listen port | `9610` |
| `YOMIHON_EMBED_KEY` | User-owned Gemini credential for an explicit semantic CLI action | unset |

A successful start logs `yomihon serving` and Home loads at that address. An
invalid vault path exits non-zero with a `yomihon exited` error; a missing or
invalid vault contract is warned, keeps reading available, and closes the write
face.

The listener address is fixed; only the port is configurable. Reading,
navigation, diagnostics, and lexical search support macOS, Linux, and Windows.
Status writes and semantic generations currently support macOS and Linux.

Yomihon is a personal application rather than a turnkey multi-user service.
It is deliberately not distributed as a container image: the listener binds
`127.0.0.1` in the process's own network namespace, which inside a container is
a loopback the host cannot reach, and the status write runs `git` as the
operator so the commit carries their identity. Both are the boundary working,
and both would have to be loosened to make an image useful. Use
[GitHub Issues](https://github.com/Koopa0/yomihon/issues) for questions and
defects.

## License

[MIT](LICENSE). Redistributed fonts and client assets are documented in
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
