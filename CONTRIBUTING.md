# Contributing to yomihon

yomihon has one maintainer, who reviews every submission before it lands.

## What yomihon is, and what it will not become

yomihon reads a folder of Markdown as one book. It runs on your machine, for
one reader, over notes you already wrote. It is not a note editor, not a
publishing pipeline, and not a service.

Four boundaries hold. Each is enforced in the tree, so you can check the claim
rather than take it:

- **It writes one field.** The whole write face is a single frontmatter field,
  `status`, behind one `POST /status` endpoint. A transition has to be legal in
  the vault contract's state machine. The rewrite changes the status value and
  nothing else: the rest of that line, its quoting, and any comment its author
  wrote there all survive byte for byte. `internal/status` is the only package
  allowed to write vault files, and its package comment says so. One status is
  never offered. `published` records a completed publication outside the vault
  and nothing here can attest one, so a person writes it by hand and yomihon
  renders it; `Flip` refuses it with `ErrPublishedReserved`.
- **It never leaves the machine.** The listener hardcodes `127.0.0.1` and only
  the port varies, through `YOMIHON_PORT` — the one environment variable
  production code reads. A test in `cmd/yomihon` parses every file the binary is
  built from, on each supported platform, and fails on any other reach for the
  environment. yomihon makes no outbound request of any kind. Egress is a
  decision for the maintainer, not a pull request.
- **The vault contract is the only schema.** A vault's
  `System/schemas/vault-schema.toml` is the single source of what a note type
  is and which status transitions exist. `internal/schema` is the only package
  that reads it, and the only importer of a TOML decoder. A test in
  `internal/archlock` fails when a word the contract owns — `lesson`,
  `concept`, `inbox`, `draft`, `ready`, `published` — stands as a whole string
  literal in the Go source this repository ships, outside `internal/schema`. It
  reaches no further, and says so: a word inside a longer sentence, inside a
  test, or inside the Go in a `.templ` file goes past it. When you feel the urge
  to write `if status == "ready"`, the answer is already in `internal/schema`.
- **It reports; it does not repair.** The renderer reads fault-tolerantly and
  surfaces what is wrong where it is wrong: broken frontmatter, a link with no
  target, and one name two files answer to. `yomihon check` says the same thing
  on the command line. Nothing here edits a reader's prose to make it parse.

The interface speaks Traditional Chinese and English, and a reader chooses
between them with one cookie. A sentence a reader sees is therefore not written
where it is used: it lives in `internal/wording`, built from both languages at
once with `both(zhHant, en)`, and resolved with `In` by the surface that knows
who is reading. Two tests in `internal/archlock` hold that shape. One turns red
when a response carries a sentence written in its own source rather than taken
from `internal/wording`; the other when a phrase picks its language before the
request says which.

## Build and run it

You need Go 1.27 or newer, and nothing else for a plain build: the generated Go
and the stylesheet are committed.

1.  Clone the repository and build the binary.

    ```bash
    git clone https://github.com/koopa0/yomihon
    cd yomihon
    go build -o bin/yomihon ./cmd/yomihon
    ```

2.  Run it against the example vault that ships with the repository.

    ```bash
    bin/yomihon examples/vault
    ```

3.  Open `http://127.0.0.1:9610` in a browser.

`examples/vault` is a small vault with a contract, a study path, and a few
deliberate faults, so most reading and diagnostic behavior is reachable without
a vault of your own. Any folder of Markdown works the same way:
`bin/yomihon ~/notes`.

Set `YOMIHON_PORT` to serve on another port. To read two folders at once, run
two yomihon processes on two ports; the folder is fixed for the life of a
process.

The command line has four commands. `yomihon --help` lists them, and
`yomihon check --help` documents the flags, the output shapes, and the exit
codes:

```bash
bin/yomihon check --root examples/vault
```

## Run the tests

The fast loop, in the order that catches the most for the least time:

```bash
make build-check   # compile every package this repository owns
make test          # the Go tests, race-enabled, shuffled, no cached results
make lint          # golangci-lint at the version the Makefile pins
```

`make test` needs only a Go toolchain.

### The gate

`make verify` is the gate you run before you push. Continuous integration
requires four checks and `make verify` is one of them; coverage is CI-only
review evidence, and the other two build, vet, and test on macOS and Windows
runners, so a green run on your machine does not tell you those passed.

The `verify` target in the Makefile is the list of what it runs.

Beyond the Go toolchain, `make verify` needs:

- `make tools`, which installs the pinned Go analysis tools into `GOBIN`.
- Three tools that `go install` cannot provide. The Tailwind standalone
  command-line interface and ShellCheck are pinned by version at the top of the
  Makefile; Node's version lives in `.github/package.json` and the workflow,
  with the lockfile under `.github/`.
- A locally installed Google Chrome. The browser probes drive the Chrome you
  already have rather than downloading one.

templ needs no installation. It is a tool directive in `go.mod`, so
`go tool templ` is already the version this module builds with.

Three stages reach the network: `mod-check`, `vuln`, which fetches the
vulnerability database, and `frontend-check`, which runs `npm ci`. Two stages
drive a browser: `browser-check` and `mutation-check`, each of which runs
`frontend-check` first.

On a shared Linux runner the same target takes about twenty minutes.

Read its exit code, not its last screen. `make verify | tail` reports the exit
code of `tail`, and a gate whose red you piped away has told you nothing. If you
cannot run every stage — no Chrome, no network — run what you can and say in the
pull request which stages you ran and which you did not. A scoped package run is
not a verify exit code, and describing it as one is worse than reporting the
gap.

## File an issue

Use the bug form for anything that is broken. It asks what happened, what you
expected, how to reproduce it from a clean start, and which build you ran. There
is no `--version` flag yet, so give the commit you built from.

Four of its fields are optional. Three of them decide whether a
report about vault behavior can be reproduced at all:

- the section of your `System/schemas/vault-schema.toml` the bug touches;
- the smallest folder that reproduces it;
- the output of `yomihon check --root <vault>` against that folder.

Anything that is not a bug — a reading experience that is worse than it should
be, a wording that misleads, a proposal — goes in a blank issue. Say what
yomihon does now, what it should do instead, and how you would know the change
worked. Do not send vault content you would not publish: reports, issues, and
fixtures are public.

## Propose a change

Changes start from an issue, not from a branch. Open one, say what should change
and why, and wait for an answer. Scope is the maintainer's call, so the
maintainer can close a pull request that has no issue behind it, however good
the code is.

Three proposals are worth pricing before you write them:

- **A new dependency.** `go.mod` names eight direct requirements and each earned
  its place: goldmark and chroma to render, templ for the templates, a TOML
  decoder for the vault contract, a YAML decoder for frontmatter, two
  `golang.org/x` libraries, and go-cmp for the tests. The client side is vanilla
  JavaScript in flat native modules, with one exception: the Mermaid renderer,
  vendored under `assets/js/mermaid` as pre-built modules with their own license
  and checksums. That is the bar for a third-party client library, and the only
  one in the tree. There is no database, no bundler, and no JavaScript
  framework. A new dependency is a conversation, not a commit.
- **A new package.** yomihon is organized by feature, not by layer. There are no
  `services`, `models`, `handlers`, or `util` directories and there will not be.
  A package earns its name from the domain concept it owns.
- **Anything touching a boundary above.** Say so in the issue and stop there.

## What a pull request contains

One issue per pull request, branched from `main`. Four sections in the
description, and the maintainer reads the second one hardest:

- **What changed.** The behavior, in the reader's terms, not the diff's.
- **How it was verified.** Each new lock, watched failing and then passing, with
  the exact failure text. Then the exit code of `make verify` on the commit you
  pushed, and the line that produced it. If the change touches a hot path or
  what a reader sees, add a before and after collected on one machine.
- **What was left out.** Defects you saw and did not fix, and why. Naming them
  is the right move; silently widening the change is not.
- **Needs a ruling.** Anything you could not decide. If two reasonable behaviors
  are still standing, stop and write them both down rather than picking the one
  that is easier to implement, store, mock, or test.

Do not close work you did not do. If your change makes a neighboring issue
fixable, say so under **What was left out** and leave it open.

## What gets a change blocked

These are the findings that come back most often. Each one is a rule, not a
preference.

### A test you have not watched fail is not a lock

Before you claim a test protects something, break the production code it guards,
watch the test go red, read the failure message, and put it back. Three things
make that proof worthless:

- The mutation did not apply. A pattern that matched nothing produces a green
  run that means nothing, so confirm the code actually changed.
- The mutation only broke the build. A compile error is not a red test.
- The test never reached production. A test that drives a constructor instead of
  the entry point, or asserts a state production cannot produce, or compares a
  declaration against itself, passes for the wrong reason and will keep passing
  after the regression lands.

The repository already mechanizes this proof on the browser side.
`make mutation-check` runs each probe against the regression that probe exists
to catch, and demands that the probe report catching it by name. A probe that
lets the regression through fails the run, and so does an injection that matched
nothing. Read `.github/e2e/probes.sh`.

Nothing injects a regression for you on the Go side; there the proof is yours to
run and to paste into the pull request.

### Name every path into what you changed

Fix one entry point and a reviewer will find the others one at a time. Before
you patch, write down the complete set of ways the thing can be entered,
bypassed, or left early — every caller, every optional interface that routes
around you, every early return, and the case where nothing happens at all. If
you cannot name the whole set, you are guessing.

### A sentence a reader can act on has to be true after your change

That covers page copy, diagnostic text, command-line help, code comments, and
the authoring skill under `skills/`. A suggested repair the reader cannot follow
is as bad as a wrong one. Comments carry their own house style: they explain the
reason in domain language, and they do not cite issue numbers, pull requests,
planning vocabulary, or documents that a clean clone does not have.

### One owner per closed set

When you want to write a list of statuses, note types, or marks, it is already
declared somewhere — usually in `internal/schema`, from the vault contract.
Restating it in a second package compiles, passes every gate, and diverges
silently a month later.

### Delete what your change made dead

A field with no reader, a flag that is now always true, a helper the fix
retired. `make verify` fails on a function nothing reaches; fields are yours to
notice.

### Show what else your change moves

If your change alters what a reader sees or what `yomihon check` reports, run
the old and the new binary over `examples/vault` and put the difference in the
pull request. Then check that the comparison could have shown a difference at
all.

## Generated files and frozen formats

templ and Tailwind generate what this table lists, and all of it is committed.
Edit the source, regenerate, and commit the output with it:

| Generated | Source | Regenerate with |
| --- | --- | --- |
| `internal/ui/**/*_templ.go` | the matching `.templ` file | `go tool templ generate -path internal/ui` |
| `assets/css/output.css` | `assets/css/input.css` and its imports | `make css` |

Continuous integration regenerates everything in the table and fails on any
difference, so a hand edit is caught, not merged. `make fmt` formats the
templates, regenerates, and formats the Go in one step.

Some bytes are frozen because something outside this repository parses them. The
judge's output — the JSON Lines fields, their order, and the reason strings — is
pinned by the golden files under `internal/judge/testdata/`; its exit codes are
pinned by the test tables in `internal/judge` that read those goldens. The
goldens are the contract, not a description of one. Regenerating them to make a
test pass breaks every consumer downstream. If a golden has to move, it moves by
addition, and the pull request explains each changed line by the fixture and the
rule that produced it.

A new judge rule lands with three things together: the rule, a fixture that
proves it fires, and its row in [`docs/judge-rules.md`](docs/judge-rules.md). A
test in `internal/judge` reads that table and fails when it disagrees with the
rule identifiers and authorities the frozen findings carry.

## How a change reaches main

A ruleset protects `main`, and nobody can bypass it, including the maintainer:

- Every change arrives through a pull request. GitHub refuses a direct push.
- All four required checks have to be green on the head commit. An approving
  review is not required, except on a pull request carrying a change GitHub
  cannot attribute to an account.
- History stays linear, so a change lands squashed or rebased and the branch is
  deleted on merge. Merge commits are off.
- GitHub refuses a force-push to `main`, and refuses to delete it.

The maintainer presses merge, not the contributor. Your branch does not have to
be up to date with `main` to merge, but a branch that is behind can pass a gate
`main` would fail, so rebase before you ask.

Commit messages follow the conventional shape the history uses — `fix`, `feat`,
`refactor`, `test`, `docs`, `chore`, `perf` — with a lowercase imperative
description and no trailing period. The body explains why; the diff already
shows what.

Keep attribution trailers out of the commits on your branch. A change lands
squashed or rebased, and both carry your branch's commit messages into `main`,
so a trailer written on the branch becomes a trailer in the history. If your
tooling appends one you cannot remove, say so in the pull request.

## Report a vulnerability

Report privately through
[GitHub's private advisory form](https://github.com/koopa0/yomihon/security/advisories/new),
never in a public issue, and attach no vault content. Anything that writes
outside the `status` field, or leaves the machine, is worth reporting. See
[the security policy](.github/SECURITY.md) for what is supported.
