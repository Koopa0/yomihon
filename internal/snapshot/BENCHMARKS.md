# Representative snapshot benchmarks

The default snapshot benchmark keeps its small fixed vault. Representative runs
require `-snapshot-bench`; without it all 15 cases skip before any fixture is
built, and no ordinary test runs them.

Each representative fixture contains 100, 1,000, or 5,000 Markdown notes,
including one study path and one topic map, plus the existing test schema file.
Concepts contain CJK and Latin paragraphs, a heading, a Go code block, provenance,
and two wikilinks to concept targets. The path and map each declare two links.
This produces exactly N notes, N+1 files, and 2N links. Concepts use the fixture
schema's `seedling` lifecycle; the path and map use its valid `archived` state.
The benchmark does not change that schema or model an all-ready vault.

Every case builds a generation before it times anything, so all five read
through a warm filesystem cache; no cache flush or cold-process behavior is
claimed. After the last operation, the published generation must still count N
notes and find the measured concept by a word in its body, or the run fails.
Fixture creation and source writes are outside the timed interval, except for
the body write included in `visible` as described below.

`initial` calls the real snapshot constructor. `rebuild` calls the server's
synchronous reconciliation method after a small edit to one concept. Every case
that edits alternates two different-length suffixes, so each edit is visible to
the scan; writing the edit is not timed. Neither includes HTTP delivery or the
scanner's scheduling delay.

`idle` times the synchronous reconciliation scan of an unchanged vault.

`overlap` times reconciliation after a different-size body edit. Writes, forced
GC and statistics reads are outside `ns/op`, `B/op`, and `allocs/op`. Four
additional byte metrics describe process-wide post-GC `runtime.MemStats.HeapAlloc`:

| Metric | Retained owners at the statistics read |
|---|---|
| `one-heap-B` | Fixture, Store and original current generation |
| `two-heap-B` | Fixture, Store, replacement current generation and explicitly held old generation |
| `released-heap-B` | Fixture, Store and replacement current generation after the old-reference helper returns |
| `reclaimed-B` | `two-heap-B` minus `released-heap-B` |

Every sample performs a real GC followed by ReadMemStats. The old generation is
held in a non-inlined helper frame that returns only numeric samples, so its
release does not depend on assigning nil to a variable inside `b.Loop`. Fixture
and Store overhead remain in all observations. These are heap observations, not
allocation totals, RSS, or isolated object sizes.

`visible` starts one `Store.Run` using the real two-second reconciliation
ticker. A random delay in `[0, 2s)` samples the edit's position in the cadence
and is excluded from timing. The timed operation starts before the body write
and ends when 10 ms polling of `Current().Note(...)` observes the edit's content
identity. It includes the actual write, scanner scheduling and rebuild, and
polling resolution. Each visible iteration uses a fresh Store and starts Run
once; construction, the pre-edit wait and stopping the loop are excluded. A
ten-second context from scanner startup ends a sample that never sees its edit;
it is a failure bound, not a performance target. There is no HTTP delivery in
this measurement.

Each row reports `ns/op`, `B/op`, and `allocs/op`, plus the starting `notes`,
`files`, `links`, and `source-B`. The source byte count includes the schema and
describes the fixture before any body edit. Allocation totals describe Go
allocations during the measured boundary, not process RSS or a capacity limit.
Visible allocation totals include work performed by its scanner goroutine during
that boundary.

Run only this package with the explicit opt-in and ten independent samples:

```sh
GOMAXPROCS=2 go test -run='^$' \
  -bench='^BenchmarkRepresentativeSnapshot$' -benchmem -benchtime=1x -count=10 \
  ./internal/snapshot -args -snapshot-bench > /tmp/snapshot-baseline.txt
benchstat /tmp/snapshot-baseline.txt
```

The hosted lifecycle baseline used empty `GOFLAGS`. Each iteration is one sample:
`-benchtime=1x -count=10` supplies ten independent samples for each of the 15
cases, rather than ten samples inside each iteration. Keep the raw output,
source commit, Go version, hardware, GOMAXPROCS, iteration policy, and cache
conditions together. For a future change, repeat the same command and environment
into `/tmp/snapshot-current.txt`, then compare:

```sh
benchstat /tmp/snapshot-baseline.txt /tmp/snapshot-current.txt
```

A single baseline does not establish a speedup, latency budget, or maximum vault
size. Compare only equivalent workloads and settings; a different iteration
policy or hardware requires a separate baseline. The recorded Apple M1
initial/rebuild baseline and hosted Linux lifecycle baseline have separate source
and environment bindings in [the baseline records](benchmarks/README.md); they
are not a before/after comparison.
