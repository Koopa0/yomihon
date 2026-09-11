# yomihon — agent entry point

This file governs any agent that opens a pull request here.

## Commit and GitHub text — no agent identity

Do not put any of the following in commit messages, PR titles/bodies, or
Issue/PR comments:

- `Co-authored-by: Cursor Agent <…>` or any Cursor Co-authored-by trailer
- Self-identification as Cursor / Cursor Agent / app/cursor / “written by
  Cursor”

Finish the change. Do not advertise that an agent wrote it.

## What you may work on

- Work from an issue that carries the `grok` label. An issue labeled
  `needs-owner`, `needs-repro`, or `blocked` is not yours until that label
  changes.
- An issue that an open pull request already references is taken. Stop
  rather than open a second one.
- Treat the issue body as the specification. When the issue also carries a
  ruling comment, that comment wins: it names the behavior today, the behavior
  wanted, the test that locks it, and the scope you stay inside. Build that and
  nothing else.
- Check the premise before you write code. If the behavior the issue describes
  no longer occurs on current `main`, say so on the issue and stop. Do not
  redefine the issue into a change you can make.

## The pull request

1. Start from `main`.
2. Keep one issue per pull request, plus any issue the ruling bundles with it.
   Write `Closes #<number>` for each, one per line.
3. Give the description four headings, in this order: What changed, How it was
   verified, What was left out, and Needs a ruling. Keep a heading you have
   nothing for, and write `None.` under it.
4. Prove each behavior change under "How it was verified". Break the production
   code the change relies on — revert one branch, drop one guard — run the test
   that locks it, quote the failure you watched, then restore it. A lock you
   have never seen go red against your own production code proves nothing, and
   a build error is not a red test.
5. Name the gate you ran under the same heading. Run `make verify` unpiped and
   report its exit status: a pipe reports the exit status of the last command in
   it, which has read a red gate as green here before. A scoped
   `go test ./internal/status/` is worth reporting, so long as you call it what
   it is.
6. When a review asks for a change, push it and reply once, in English, naming
   the commit. Do not restate the gate: CI runs it on every push, and a claim
   that it is green is not evidence a reviewer can use.

Never merge a pull request; opening it ends your work.

## The gate

- `make verify` passes on the exact commit you push, not on an earlier one.
- Never weaken a gate, a golden file, or a test oracle to reach green. If you
  believe a gate is wrong, leave it red and say so under "Needs a ruling".
- `make verify` needs more than a Go toolchain. `make tools` installs the Go
  analysis tools; the `Makefile` header pins the three that are not
  go-installable — the Tailwind standalone command-line interface, ShellCheck,
  and Node driven from the lockfile under `.github/` — and the browser probes
  drive an installed Google Chrome. The agent environment's bootstrap prepares
  the build, not the gate, so install the gate's own tools before you call it
  green.
- Open the pull request even when your environment cannot run every stage, and
  say which you ran. Continuous integration runs the gate on every pull request,
  and `main` accepts nothing that fails it.
- Do not hand-edit generated files. `make gen` regenerates `*_templ.go` from its
  `.templ` source, and `make css` regenerates `assets/css/output.css` from
  `assets/css/input.css`. The gate compares both against a fresh generation.

## What you may not touch

- The meaning of a field or a state machine in
  `System/schemas/vault-schema.toml`. `internal/schema` is the only package that
  reads it, and no enum, field list, or lifecycle rule is copied anywhere else.
- The bytes of any file under `internal/judge/testdata/`. External pipelines
  parse the judge's lines and exit codes byte for byte.
- The status write path in `internal/status`, which changes the status value and
  no other byte of the note.
- The loopback-only listener and the guarantee that yomihon makes no network
  call. `YOMIHON_PORT` is the only environment variable the product reads, and a
  test fails any new one.

Stop if your change needs to cross one of these. If the ruling authorizes the
crossing for this issue, do what it authorizes and nothing more. Otherwise write
`NEEDS-KOOPA` under "Needs a ruling", name the boundary and why the change needs
it, and open the pull request anyway.

Do not flip a note's status through a running server. The notes under
`examples/vault/` are tracked and the agent environment serves that vault, so a
flip lands in your diff.
