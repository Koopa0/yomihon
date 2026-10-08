# Representative snapshot benchmarks

The default snapshot benchmark retains its small fixed vault. Representative
runs require `-snapshot-bench`; ordinary tests exercise an eight-note fixture and
one bounded 100-note command check limited to `initial|rebuild`. Lightweight
controls use eight-note fixtures without timing or GC heap measurements. Disabled
execution registers and skips all 15 leaves before benchmark fixture setup;
ordinary tests never run `idle`, `overlap`, or `visible` measurements.

Each representative fixture contains 100, 1,000, or 5,000 Markdown notes,
including one study path and one topic map, plus the existing test schema file.
Concepts contain CJK and Latin paragraphs, a heading, a Go code block, provenance,
and two wikilinks to declared concept targets. The path and map each declare two
links. This produces exactly N notes, N+1 files, and 2N links. Concepts use the
fixture schema's `seedling` lifecycle; the path and map use its valid `archived`
state. The benchmark does not change that schema or model an all-ready vault.

Before timing and after each operation, the benchmark reads the complete source
set and verifies the published generation: file bytes and membership, note bodies,
frontmatter/schema health, resolved link targets, search membership, path items,
topic-map items, and complete scan state. Fixture creation, source writes, and
these checks are outside the timed interval, except for the body write included
in `visible` as described below.

`initial` calls the real snapshot constructor. `rebuild` calls the server's
synchronous reconciliation method after a small edit to one concept and requires
a replacement published generation. The edit alternates two different-length
suffixes to make every iteration observable; writing the edit is not timed.
Neither includes HTTP delivery or the scanner's scheduling delay.

`idle` times the real synchronous reconciliation scan of an unchanged vault and
requires the same published generation afterward. A benchmark-local wrapper of
the existing rooted source delegates all operations and records whether
`ScanAvailable` completed and whether it returned an error. A refused scan cannot
count as unchanged success. The timed boundary includes this wrapper's scan
dispatch and outcome recording; the complete post-operation receipt is excluded.

`overlap` times reconciliation after a different-size body edit and requires a
replacement generation with the expected content identity. Writes, identity
preparation, receipts, forced GC, and statistics reads are outside `ns/op`,
`B/op`, and `allocs/op`. Four additional byte metrics describe process-wide
post-GC `runtime.MemStats.HeapAlloc`:

| Metric | Retained owners at the statistics read |
|---|---|
| `one-heap-B` | Fixture, Store and original current generation |
| `two-heap-B` | Fixture, Store, replacement current generation and explicitly held old generation |
| `released-heap-B` | Fixture, Store and replacement current generation after the old-reference helper returns |
| `reclaimed-B` | `two-heap-B` minus `released-heap-B` |

Every sample performs a real GC followed by ReadMemStats and then KeepAlive for
every promised owner. The old generation exists in a non-inlined helper frame
that returns only numeric samples; release does not depend on assigning nil to a
variable inside `b.Loop`. Fixture and Store overhead remain in all observations.
These are heap observations, not allocation totals, RSS, or isolated object
sizes. Release must reduce the observed heap; a contradictory observation fails
with its actual two-generation and released byte values. No equality with the
original heap, percentage tolerance, or capacity ceiling is imposed.

`visible` starts one owned `Store.Run` using the real two-second reconciliation
ticker. A cancellable random delay in `[0, 2s)` samples the edit's position in
the cadence and is excluded from timing. The timed operation starts before the
body-write helper and ends when sparse 10 ms polling of `Current().Note(...)`
observes the exact prepared content identity. It includes directory checking,
the actual write, scanner scheduling and rebuild, polling resolution and
observation overhead. The edit changes body content and size; a status-only edit
would not change the content identity. An arbitrary replacement pointer or other
changed identity does not satisfy the observation.

Each visible iteration uses a fresh Store and starts Run once. Fixture/Store
construction, complete pre/post receipts, identity preparation, random pre-edit
wait, and cancellation/join are excluded. A ten-second context from scanner
startup is an operational failure bound, not a performance target. Timeout,
cancellation and unexpected scanner exit refuse the sample. Deferred cancellation
and joining run before rooted-reader cleanup, including write failures; the
successful path also joins before the complete final receipt. No synchronous
rescan races with this Run owner. There is no HTTP delivery in this measurement.

Setup constructs and validates a generation first, so all five cases use warm
filesystem-cache measurements; no cache flush or cold-process behavior is
claimed.

Each row reports `ns/op`, `B/op`, and `allocs/op`, plus the observed starting
`notes`, `files`, `links`, and `source-B`. The source byte count includes the
schema and describes the original fixture before any body edit. Allocation totals
describe Go allocations during the measured boundary, not process RSS or a
capacity limit. Visible allocation totals include work performed by its scanner
goroutine during that boundary.

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
