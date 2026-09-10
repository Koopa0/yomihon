<!--
The description has four parts, one per heading below. Fill each one in and
delete these notes as you go — whatever you leave here stays in the pull
request body. Where a part has nothing under it, write `None.` rather than
deleting the heading.

Link every issue this change closes on its own line: `Closes #<number>`, then
`Closes #<number>`. Delete the line below if it closes none.
-->

Closes #

## What changed

<!--
What a reader can now do, or stop doing, and where. Name the packages or files
you touched.

`*_templ.go` and `assets/css/output.css` are built, not written. If you edited
a `.templ` file, a stylesheet under `assets/css/`, or a module under
`assets/js/`, run `make gen` and `make css` and commit what they produce: the
gate rebuilds both and fails on the difference. `make gen` needs only the Go
toolchain; `make css` needs the version-pinned Tailwind binary, which nothing
here installs for you. If you cannot run one of them, open the pull request
anyway and say so.
-->

## How it was verified

<!--
If your change alters behavior, name each test that now locks it and quote the
failure you watched before the fix. A test you have never seen fail is not a
lock, and a build error is not a red test. One line each:

```text
TestFlipRefusesHardLinkedNote: red on main — Flip(hard-linked note) = <nil>,
want note has more than one name; green here, and both names still hold the
pre-flip bytes.
```

If your change alters no behavior — a typo, a comment, a rename — skip the
locks.

Then name the gate you ran, precisely. `make verify` is the gate CI runs, and
it needs more than a Go toolchain: `make tools`, Node, the pinned Tailwind
binary, ShellCheck, and an installed Chrome. A scoped
`go test ./internal/status/` is worth reporting, but it is not a verify exit
code.

Do not weaken a gate, a golden file, or a test oracle to reach green.

CI runs `make verify` on every pull request, and also builds, vets, and runs
tests on macOS and Windows runners that no local target reaches. Open the pull
request even when you cannot run the whole gate yourself.
-->

## What was left out

<!--
What you noticed and did not do, and why. A defect you found but were not asked
to fix belongs here by name, not in this pull request.
-->

## Needs a ruling

<!--
If two defensible behaviors are still standing, put the choice here rather than
taking the one that is easier to implement or to test.

Four things move only on the maintainer's word: what yomihon takes a vault's
`System/schemas/vault-schema.toml` to mean, the frozen bytes under
`internal/judge/testdata/`, the single status write in `internal/status`, and
the loopback-only listener with no outbound call. If your change needs one of
them, say so here and open the pull request anyway.
-->
