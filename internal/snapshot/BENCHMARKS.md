# Representative snapshot benchmarks

The default snapshot benchmark retains its small fixed vault. Representative
runs require `-snapshot-bench`; ordinary tests exercise an eight-note fixture and
one bounded 100-note command check. They never construct the 1,000- or 5,000-note
fixtures without explicitly enabling the benchmark.

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
these checks are outside the timed interval.

`initial` calls the real snapshot constructor. `rebuild` calls the server's
synchronous reconciliation method after a small edit to one concept and requires
a replacement published generation. The edit alternates two different-length
suffixes to make every iteration observable; writing the edit is not timed.
Neither phase includes HTTP delivery or the scanner's scheduling delay. Setup
constructs and validates a generation first, so these are warm filesystem-cache
measurements; no cache flush or cold-process behavior is claimed.

Each row reports `ns/op`, `B/op`, and `allocs/op`, plus the observed starting
`notes`, `files`, `links`, and `source-B`. The source byte count includes the
schema and describes the original fixture before the small rebuild edit.
Allocation totals describe Go allocations, not process RSS or a capacity limit.

Run only this package with the explicit opt-in and ten independent samples:

```sh
GOMAXPROCS=2 GOFLAGS='-trimpath -p=2' go test -run='^$' \
  -bench='^BenchmarkRepresentativeSnapshot$' -benchmem -benchtime=1x -count=10 \
  ./internal/snapshot -args -snapshot-bench > /tmp/snapshot-baseline.txt
benchstat /tmp/snapshot-baseline.txt
```

The one-iteration samples keep this local baseline bounded. Keep the raw output,
source commit, Go version, hardware, GOMAXPROCS, iteration policy, and cache
conditions together. For a future change, repeat the same command and environment
into `/tmp/snapshot-current.txt`, then compare:

```sh
benchstat /tmp/snapshot-baseline.txt /tmp/snapshot-current.txt
```

A single baseline does not establish a speedup, latency budget, or maximum vault
size. Compare only equivalent workloads and settings; a different iteration
policy or hardware requires a separate baseline.
