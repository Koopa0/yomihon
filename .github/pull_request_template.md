<!--
The description has three parts, one per heading below. Fill each one in and
delete these notes as you go — whatever you leave here stays in the pull
request body. Where a part has nothing under it, write `None.` rather than
deleting the heading.

Link every issue this change closes on its own line: `Closes #<number>`, then
`Closes #<number>`.
-->

Closes #

## What changed

<!--
What a reader can now do, or stop doing, and where. Name the packages or files
you touched. Name every gate you removed and every test you deleted, with a
line on why.

`*_templ.go` is built, not written. If you edited a `.templ` file, run
`make gen` and commit what it produces: the gate rebuilds it and fails on the
difference. If the judge's output changed on purpose, regenerate its golden
files and say so here.
-->

## How it was verified

<!--
Name the Go test for the behavior you changed. If you changed a promise test,
or the code it guards, quote the failure you watched when you broke the
production code, then say you restored it. The four promises: nothing writes
into the folder being read; loopback only, no network call; what a rules file
withholds stays out of agent-facing output; the browser never receives
unsanitised Markdown.

Then report `make verify`, run unpiped, on the commit you pushed: its exit
status, and which stages you ran if you could not run them all. CI runs the
rest.

If the change touches what a reader sees, add screenshots before and after, at
desktop and phone width, in light and dark.
-->

## What was left out

<!--
What you noticed and did not do, and why. A defect you found but were not asked
to fix belongs here by name, not in this pull request.

Only if two defensible behaviors are still standing, add a fourth heading,
"Needs a ruling", and write both down rather than taking the one that is easier
to implement or to test. A change that crosses one of the four promises needs
that ruling.
-->
