# Contributing to yomihon

yomihon has one maintainer, who reviews every submission before it lands.

## What yomihon is

yomihon is a local reading room for a folder of Markdown. The reader comes
first: reading, marking what they read, and handing those marks to an AI. It
runs on your machine, for one reader, over notes you already wrote. It is not a
note editor, not a publishing pipeline, and not a service.

`check`, `coverage` and `exists` are a supporting tool for an author who chose
to write a rules file; they are not the product. Any folder must read well with
no rules file and no configuration. A rules file
(`System/schemas/vault-schema.toml`) adds validation and privacy rules. It is
never the price of a reading feature.

yomihon reports; it does not repair. Broken frontmatter, a link with no target,
and one name two files answer to are shown where they are, and
`yomihon check` says the same thing on the command line. Nothing here edits a
reader's prose to make it parse.

## Four promises

These are what a person trusts yomihon for. None of them moves without an
explicit ruling from the maintainer on the issue.

1. **Nothing writes into the folder being read.** No byte of a note, the rules
   file or anything else under the vault directory is written, renamed or
   removed. What an author claims about a note — `draft: true` included — stays
   in the author's editor. The reader's own marks live in files of yomihon's own
   under the platform's configuration directory, and `internal/mark` is the only
   package allowed to write at all; a test in `internal/archlock` reads every
   shipped file and fails on a write anywhere else.
2. **Loopback only, no network call.** The listener binds `127.0.0.1` and only
   the port varies, through `YOMIHON_PORT` — the one environment variable the
   product reads. Adding another needs a ruling. A test in `cmd/yomihon` parses
   every file the binary is built from, on each supported platform, and fails on
   any other reach for the environment. The process opens no outbound
   connection of any kind. A figure the owner approved runs scripts in the
   browser that can reach the network; that is the one browser-side exception,
   decided on 2026-11-12, and the threat model bounds it.
3. **Withheld ground stays withheld.** What a rules file's `[privacy]` section
   withholds never reaches the output of a command an AI reads: `check`,
   `coverage`, `exists` and `marks`.
4. **The browser never receives unsanitised Markdown.** Go renders and
   sanitises; the browser shows the result.

The interface speaks Traditional Chinese and English, and a reader chooses
between them with one cookie. A sentence a reader sees is therefore not written
where it is used: it lives in `internal/wording`, built from both languages at
once with `both(zhHant, en)`, and resolved with `In` by the surface that knows
who is reading. Three tests in `internal/archlock` hold that shape. One turns
red when a response carries a sentence written in its own source rather than
taken from `internal/wording`; one when a phrase picks its language before the
request says which; and one when a script carries Chinese or Japanese text of
its own.

## Build and run it

You need Go 1.27 or newer, and nothing else for a plain build: the generated Go
and the stylesheet are committed.

For tests and the full gate, bring the following in addition to Go:

- **Race-enabled tests:** On Linux and Windows, `make test` needs cgo enabled
  and a C compiler for the race detector; macOS needs neither for that detector.

- **Test user:** Run the tests as an unprivileged user: some permission fixtures fail
  rather than skip when the process can bypass file permissions.

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

`examples/vault` is a small vault with a rules file, a course, and a few
deliberate faults, so most reading and diagnostic behavior is reachable without
a vault of your own. Any folder of Markdown works the same way:
`bin/yomihon ~/notes`.

Set `YOMIHON_PORT` to change the port yomihon holds, which a folder new to the
list is served on when no vault is. Each folder you read joins a list of
vaults, every one on a port of its own that it keeps; while a yomihon is
running, `yomihon <dir>` hands it the folder and prints the address, and
`yomihon vaults` lists them.

The command line has eight commands. `yomihon --help` lists them, and
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

`make test` uses the race detector and permission fixtures, so the test
prerequisites above also apply to this fast loop.

### The gate

`make verify` is the gate you run before you push. Continuous integration runs
it as one of its required checks; two of the others build, vet, and test on
macOS and Windows runners, so a green run on your machine does not tell you
those passed.

The `verify` target in the Makefile is the list of what it runs.

The test prerequisites above also apply to `make verify`.

Beyond those, the gate needs:

- `make tools`, which installs the pinned Go analysis tools into `GOBIN`.
- Two tools that `go install` cannot provide. ShellCheck is pinned by version
  at the top of the Makefile; Node's version lives in `.github/package.json` and
  the workflow, with the lockfile under `.github/`.
- A locally installed Google Chrome. The browser probes drive the Chrome you
  already have rather than downloading one.

templ needs no installation. It is a tool directive in `go.mod`, so
`go tool templ` is already the version this module builds with.

Three stages reach the network: `mod-check`, `vuln`, which fetches the
vulnerability database, and `frontend-check`, which runs `npm ci`. One stage
drives a browser: `browser-check`, which installs those npm dependencies first
and leaves the lint to `frontend-check`.

Read its exit code, not its last screen. `make verify | tail` reports the exit
code of `tail`, and a gate whose red you piped away has told you nothing. If you
cannot run every stage — no Chrome, no network — run what you can and say in the
pull request which stages you ran and which you did not; CI runs the rest. A
scoped package run is not a verify exit code, and describing it as one is worse
than reporting the gap.

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

When you work from an issue:

- Its body is the specification. A ruling comment from the maintainer
  overrides the body.
- An issue labelled `needs-owner`, `needs-repro` or `blocked` is not open for
  taking, and neither is one an open pull request already references.
- Check the premise first. If the behavior it describes no longer happens on
  `main`, say so on the issue and stop.

Three proposals are worth pricing before you write them:

- **A new dependency.** `go.mod` names eleven direct requirements and each
  earned its place: goldmark and chroma to render, a goldmark extension that
  lets emphasis close beside CJK punctuation, templ for the templates, a TOML
  decoder for the rules file, a YAML decoder for frontmatter, two
  `golang.org/x` libraries for runtime behavior, and go-cmp, `golang.org/x/mod`
  and `golang.org/x/tools` for the tests. The client side is vanilla
  JavaScript in flat native modules, with one exception: the Mermaid renderer,
  vendored under `assets/js/mermaid` as pre-built modules with their own license
  and checksums. That is the bar for a third-party client library, and the only
  one in the tree. There is no database, no bundler, and no JavaScript
  framework. A new dependency is a conversation, not a commit.
- **A new package.** yomihon is organized by feature, not by layer. There are no
  `services`, `models`, `handlers`, or `util` directories and there will not be.
  A package earns its name from the domain concept it owns.
- **Anything that crosses one of the four promises.** Say so in the issue and
  stop there.

## What a pull request contains

One issue per pull request, plus any the maintainer's ruling bundles with it,
branched from `main`. Write `Closes #<number>` for each issue, one per line.

The description has three headings, and the maintainer reads the second one
hardest:

- **What changed.** The behavior, in the reader's terms, not the diff's. Name
  every gate you removed and every test you deleted, with a line on why.
- **How it was verified.** The test for the behavior you changed and, for a
  promise test, the failure you watched (see below). Then the exit status of
  `make verify` on the commit you pushed, and which stages you ran if you could
  not run them all. If the change touches a hot path, add a before and after
  collected on one machine. If it touches what a reader sees, add screenshots
  before and after, at desktop and phone width, in light and dark.
- **What was left out.** Defects you saw and did not fix, and why. Naming them
  is the right move; silently widening the change is not.

Add a fourth heading, **Needs a ruling**, only when you have something for it:
two reasonable behaviors that are still standing. Write them both down rather
than picking the one that is easier to implement, store, mock, or test.

Do not close work you did not do. If your change makes a neighboring issue
fixable, say so under **What was left out** and leave it open.

When a review asks for a change, push it and reply once, naming the commit.

## Tests

Tests come in two levels. Spend effort accordingly.

**Promise tests** guard the four promises above. Keep them strict. When you
change one, or the code it guards, break the production code, watch the test go
red, quote the failure in the pull request, then restore it. Three things make
that proof worthless:

- The break did not apply. A pattern that matched nothing produces a green run
  that means nothing, so confirm the code actually changed.
- The break only broke the build. A compile error is not a red test.
- The test never reached production. A test that drives a constructor instead
  of the entry point, or asserts a state production cannot produce, or compares
  a declaration against itself, passes for the wrong reason and will keep
  passing after the regression lands.

**Everything else** gets an ordinary Go test for the behavior you changed. That
is enough:

- No proof that the test goes red.
- No browser probe unless the behavior exists only in a browser and a Go test
  cannot reach it. A new probe needs no self-test mode.
- No test for wording, comments, documents or tooling.

When a redesign breaks a test that is not a promise test, and the old behavior
is no longer wanted, delete the test and list it in the pull request. Do not
rewrite a test to keep an obsolete behavior alive. Where a gate still enforces
an older, stricter rule than this guide and blocks a change the guide allows,
remove that gate in the same pull request and name it under **What changed**.

## What gets a change blocked

These are the findings that come back most often. Each one is a rule, not a
preference.

### Name every path into what you changed

Fix one entry point and a reviewer will find the others one at a time. Before
you patch, write down the complete set of ways the thing can be entered,
bypassed, or left early — every caller, every optional interface that routes
around you, every early return, and the case where nothing happens at all. If
you cannot name the whole set, you are guessing.

### A sentence a reader can act on has to be true after your change

That covers page copy, diagnostic text, command-line help, code comments, and
the authoring skill under `skills/`. A suggested repair the reader cannot follow
is as bad as a wrong one.

A comment says why, in a line or two. It does not narrate history, argue with a
reviewer, restate what the code says, or cite issue numbers, pull requests, or
documents a clean clone does not have.

### Interface text does the thing it names

A control's label is the verb for what it does: "Write a note", not "Leave your
own thought". Pressing it does that thing in place.

No sentence on a screen explains what yomihon will not do, how the mechanism
works, or what the reader should go and do elsewhere. If a screen needs such a
sentence, the flow is wrong; fix the flow.

### One name per concept

One concept has one name, in the code, the interface, the commands and the
documents. Before adding a type, package, route, command or label, find the
existing name for the concept and use it. If two names already exist for one
concept, say so on the issue rather than adding a third.

The vocabulary (English / 中文): vault 書庫, note 筆記, course 課程, unit 單元,
drill 句型練習, index 索引, rules 規則, draft 草稿, journal 日記, mark 標記,
highlight 劃線, question 疑問, resolved 已解決, thought 想法, bookmark 書籤.
Older names (syllabus, study path, lesson, map, moc, contract, uncertainty,
continuation, kept place, left open) are being retired; do not introduce them
anywhere new.

### One owner per closed set

When you want to write a list of note types, domains, or marks, it is already
declared somewhere. `internal/schema` is the only package that reads
`System/schemas/vault-schema.toml`, and the only importer of a TOML decoder; do
not copy its enums or field lists anywhere else. A test in `internal/archlock`
fails when a word the rules file owns — `lesson`, `concept`, `inbox`, `draft`,
`ready`, `published` — stands as a whole string literal in the Go or templates
this repository ships, outside `internal/schema`. When you feel the urge to
write `if noteType == "lesson"`, the answer is already in `internal/schema`.

### Delete what your change made dead

A field with no reader, a flag that is now always true, a helper the fix
retired. `make verify` fails on a function nothing reaches; fields are yours to
notice.

### Show what else your change moves

If your change alters what a reader sees or what `yomihon check` reports, run
the old and the new binary over `examples/vault` and put the difference in the
pull request. Then check that the comparison could have shown a difference at
all.

## What is not a rule

None of these is a rule. Do not defend them, and do not add work to preserve
them:

- Pure full-page navigation. Use client-side JavaScript wherever it makes the
  reading better.
- A no-JavaScript path for reader interactions such as marks and highlights.
- Byte-exact output from `check`. See below.
- Any test that pins prose, comment wording, or the test runner itself.

## Generated files and golden output

templ generates what this table lists, and all of it is committed. Do not edit
it by hand: edit the source, regenerate, and commit the output with it:

| Generated | Source | Regenerate with |
| --- | --- | --- |
| `internal/ui/**/*_templ.go` | the matching `.templ` file | `make gen` |

The stylesheets under `assets/css/` are not generated, except `art.css`:
`internal/asset` joins them as they are, so editing one needs no regeneration.

The pictures under `assets/icons` and `assets/art`, and `assets/css/art.css`
that names them, are generated too but sit outside the table, because CI cannot
regenerate them: they are cut from the design team's PNGs, which are not in
version control. `go run .github/art-import/main.go <art-in directory>` makes
them, and `.github/art-import/inputs.sha256` names the bytes it made them from.
`go test ./assets` holds the committed files to the `SHA256SUMS` list beside
them, so a hand edit is caught there. Rerunning on the same inputs writes the
same bytes.

Continuous integration regenerates everything in the table and fails on any
difference, so a hand edit is caught, not merged. `make fmt` formats the
templates, regenerates, and formats the Go in one step.

The judge's output — the JSON Lines fields, their order, and the reason
strings — is recorded in the golden files under `internal/judge/testdata/`, and
its exit codes in the test tables in `internal/judge` that read those goldens.
That output may change when the change is intended: regenerate the goldens, and
in the pull request explain each changed line by the fixture and the rule that
produced it. An unintended difference is a defect, not a golden to regenerate.

A new judge rule lands with three things together: the rule, a fixture that
proves it fires, and its row in [`docs/judge-rules.md`](docs/judge-rules.md).

## How a change reaches main

A ruleset protects `main`, and nobody can bypass it, including the maintainer:

- Every change arrives through a pull request. GitHub refuses a direct push.
- Every required check has to be green on the head commit. An approving review
  is not required, except on a pull request carrying a change GitHub cannot
  attribute to an account.
- History stays linear, so a change lands squashed or rebased and the branch is
  deleted on merge. Merge commits are off.
- GitHub refuses a force-push to `main`, and refuses to delete it.

The maintainer presses merge, not the contributor. Opening the pull request
ends your part. Your branch does not have to be up to date with `main` to merge,
but a branch that is behind can pass a gate `main` would fail, so rebase before
you ask.

Commit messages follow the conventional shape the history uses — `fix`, `feat`,
`refactor`, `test`, `docs`, `chore`, `perf` — with a lowercase imperative
description and no trailing period. The body explains why; the diff already
shows what.

Keep attribution trailers out of the commits on your branch, and do not say
which tool wrote a change in a commit, a pull request, or a comment. A change
lands squashed or rebased, and both carry your branch's commit messages into
`main`, so a trailer written on the branch becomes a trailer in the history. If
your tooling appends one you cannot remove, say so in the pull request.

## Before you tag a release

Also whenever the public screenshots or the README's reading claims change.
Check that:

- the version, `README.md` and `README.zh-TW.md` agree, and both READMEs give
  the same install path, linking the latest-release page rather than one
  version's asset;
- the platforms the READMEs name match the assets `release.yml` builds, and
  `.github/release-notes.md` states each platform's limits and lists what is
  new since the last release: edit it for this release rather than append;
- `go install github.com/koopa0/yomihon/cmd/yomihon@<the commit>` in a
  disposable `GOBIN`, `GOMODCACHE` and `GOCACHE` installs the module you expect,
  without touching your own binary;
- the example vault comes from the same revision as the binary;
- every behaviour a screenshot shows still happens at that commit, and the
  images were regenerated with `make screenshots`.

When the release is published, put `.github/release-notes.md` above GitHub's
generated list:

```sh
gh release view <tag> --json body -q .body > /tmp/generated.md
cat .github/release-notes.md /tmp/generated.md | gh release edit <tag> --notes-file -
```

Then download one asset and check it against `SHA256SUMS`
and `go version -m`, and confirm `go list -m github.com/koopa0/yomihon@latest`
reports the new version.

## Report a vulnerability

Report privately through
[GitHub's private advisory form](https://github.com/koopa0/yomihon/security/advisories/new),
never in a public issue, and attach no vault content. Anything that writes
into the vault, or leaves the machine, is worth reporting. See
[the security policy](.github/SECURITY.md) for what is supported.
