# Semantic generation-store measurement (2026-07-20)

This report measures the implemented immutable generation store and the two
SQLite drivers that were seriously considered. This run refreshes the
2026-07-18 evidence after D60 changed the canonical query and generated sqlc
inputs; the earlier raw files remain historical but no longer certify the
current source. It also supersedes the 2026-07-13 mutable-row
JSONL/CAS/packed/SQLite bake-off. Those old timing and footprint numbers
describe a storage shape that no longer exists and are not evidence for the
active/previous/staging design.

## Outcome

Keep **SQLite through `database/sql` and `modernc.org/sqlite` v1.53.0**.

That is not a claim that one noisy local run establishes a generally faster
driver. On the full 6,496-row, 1,536-dimension workload, the current comparison
measured mattn's initial build 9.26% lower (`p=0.004`, `n=20`) and found no
proven difference in compatible one-note drift (`p=0.072`, `n=20`). Modernc's
process-cold, 256-row keyset-page load was 15.06% lower (`p=0.000`, `n=20`).
Both drivers produced identical SQLite file-plus-sidecar byte counts under the
same schema and sqlc queries.

Modernc remains the better product trade because this is an optional local CLI
distributed as a Go binary, not a continuously busy database server:

- both drivers completed the full recorded workload. Mattn's initial-build
  median was 139 ms lower; the one-chunk-drift medians differed by 111.2 ms
  without a proven difference. Process-cold paged hydration—the local SQL work
  on every semantic query—favored modernc by 6.44 ms in this run;
- modernc built and executed this workload with `CGO_ENABLED=0`, and the
  driver-comparison source cross-built for Linux and Windows using only the Go
  toolchain. Those cross-builds prove compile portability only: the product's
  v1 semantic store deliberately returns `ErrStoreUnsupportedPlatform` on
  Windows and does not claim Windows runtime support;
- mattn can cross-compile, but its own documentation requires CGO plus a target
  C compiler and describes the per-platform toolchain setup. That is a real
  release-matrix cost, not a runtime defect or a claim that cross-compilation
  is impossible; and
- the store needs ordinary transactions, foreign keys, WAL, and BLOBs. It
  receives no product capability from mattn's optional compile-time SQLite
  extensions.

The trade is explicit. The test binary was 10,787,762 bytes with modernc and
8,388,738 bytes with mattn, so modernc costs 2,399,024 bytes (28.60%) in this
harness. Modernc's own documentation also calls out its exact
`modernc.org/libc` version coupling; the nested module and root module pin the
resolved dependency rather than floating it. Mattn should be re-run, not
rejected on principle, if an end-to-end profile later shows local SQLite work
owning a meaningful part of user-visible semantic-search latency.

Badger is not a hidden third driver in these numbers. No Badger timing was run:
it was excluded before this driver bake-off because the current store is an
atomic generation catalog with relational integrity and feature-local sqlc,
not an embedded key/value workload. Badger could model that contract, but only
by replacing SQLite and reimplementing its catalog and constraints; it would
not complement modernc. If a real key/value-owned access pattern appears,
Badger requires a new measured storage decision. The percentages in this
report support only the modernc-versus-mattn driver choice; they do not claim
that SQLite beat Badger in a benchmark.

Sources: [modernc package documentation](https://pkg.go.dev/modernc.org/sqlite),
[mattn installation, feature, and cross-build documentation](https://github.com/mattn/go-sqlite3#readme),
and [SQLite's WAL concurrency and checkpoint contract](https://www.sqlite.org/wal.html).

## Current generation-store workload

The product benchmark uses only the generation package's real, package-private
reader, writer, rebuild, target-preparation, retry-ledger, completion,
activation, and active-read paths. Targets and completed rows are derived
through the same corpus-chunk transformations as the builder;
the benchmark does not manufacture store seals or write SQL.
The measured v1 schema uses `STRICT` tables, normalized note/chunk ownership,
and the shipped hash/vector integrity constraints. This is a store benchmark:
provider latency and the per-command RAM exact-index construction/top-k
observer are separate measurements.

Fixture: 6,496 chunks, 502 notes, 1,536 float32 dimensions, deterministic
non-zero vectors. Active hydration uses the production 256-row keyset-page
query; it does not retain one unbounded SQL result set. Timings are one-shot
capacity observations on Darwin/arm64, Apple M1, Go 1.26.5. They are not A/B
performance claims. `InitialBuild`
includes fixture construction, opening the writer, target persistence, one
durable reservation plus completion per row, activation, and close; provider
network time is absent. A 1 ms sampler observes database, `-wal`, `-shm`, and
`-journal` sizes through writer close and its checkpoint. Its result is an
observed high-water mark, not a proof that a shorter filesystem transient
cannot exist.

| Operation / state | Result |
|---|---:|
| Initial full local build | 2.856 s |
| Compatible one-note drift | 751.8 ms |
| Drift reuse / new completion | 6,495 / 1 chunks |
| Process-cold open + complete active hydration + close | 93.55 ms |
| One active generation, cleanly closed | 54,272,000 B (51.76 MiB) |
| One active generation, observed peak including sidecars | 58,146,464 B (55.45 MiB) |
| Active + previous + resumable staging, cleanly closed | 162,725,888 B (155.19 MiB) |
| Active + previous + staging, observed peak including sidecars | 220,969,592 B (210.73 MiB) |
| Sidecars at the three-role observed peak | WAL 58,112,632 B; SHM 131,072 B; journal 0 B |

The three-role steady file is almost exactly three complete-generation files.
The 55.42 MiB WAL at the observed peak is therefore material and must remain in
capacity arithmetic; treating the database file alone as peak usage is false.

Reproduction:

```sh
YOMIHON_GENERATION_BENCHMARK=1 \
YOMIHON_GENERATION_CHUNKS=6496 \
YOMIHON_GENERATION_NOTES=502 \
YOMIHON_GENERATION_DIMENSION=1536 \
go test ./internal/search/semantic -run '^$' -bench '^BenchmarkGenerationStore$' \
  -benchtime=1x -count=1
```

The raw output used above had SHA-256
`95064af0de8aaf2282b88ac14a2577d48c2960c9ebe9382a1cede32a921d1426`.
It is retained at
`docs/benchmarks/semantic-storage-2026-07-20/product.txt` rather than existing
only as an unverifiable digest.
The run was anchored to these SHA-256 values:

- `store.go`: `1451cfa5206e20a002e9afa197492e38dda78a546b551d98a3773ef9f0debd09`;
- `writer.go`: `a9411024e0dcd34c1bcd8b2f08f6202247869165ac4d327bf285fc41dc87b6d2`;
- `staging.go`: `95e7d3fc26ab7e2a17f8980d18958d6be30255e7cd2243c3829d500537e33ffb`;
- `manifest.go`: `2302f1f9872d176dbcb7bb61fd0dcf92b6fe8aaf595aeab2c03b91b9a1f8dd7a`;
- `schema.go`: `c37ca46584c5427776b926c0a9a72ae924f1cf6f17ac4a21c5518a1ffb1964f7`;
- `bakeoff_test.go`: `1873c63ad674c2803a0275f85a2b06eddc805dfd3dcce4c2448c5fb13acd1263`;
- SQL schema: `b8ad043ac5aee1a506847feac51d09638d95596a9f2e7bf6f13bc7a54680c623`;
- SQL query: `0181cb39d981e01845563f33c20b40da71a735a40163e2a19c46d537f918d189`;
- generated query: `cd6c7670cc0d21b0fa3ce7186685ffad72515e2660d8ff85bd4a2233066c25f8`.

## Crash, publication, and reuse correctness

Timing is downstream of correctness. The real temporary-file suite covers the
failure space rather than reimplementing a second prototype inside the
benchmark:

- a reader racing activation observes one complete old or new generation;
- failed catalog flip or prune rolls back and leaves the old active generation;
- incomplete, corrupt, or wrong-corpus staging cannot activate;
- an interrupted exact staging manifest resumes, while any mismatch is
  discarded rather than merged;
- only active, previous, and one staging generation survive pruning;
- changed note bytes reuse chunks whose exact submitted bytes are unchanged;
- retry reservations survive restart and prevent a sixth provider send; and
- explicit rebuild holds the external writer lease while removing the owned
  database, WAL, SHM, and rollback-journal files before installing a fresh
  compatible store.

These are asserted by the `TestGenerationStore*` cases in
`internal/search/semantic/store_test.go`, including the active-reader race,
activation rollback, exact staging resume, changed-note reuse, role pruning,
retry-ledger restart, corrupt-active rebuild, and owned-file reset cases.

## Driver comparison

`tools/sqlite-driver-bakeoff` is a nested module so the mattn dependency and
CGO build do not enter the product's root module. Its local `replace` binds the
tool to the checkout under test. Nested modules are not discovered by a root
`go test ./...`; verification must enter this directory explicitly. Both build
tags execute one common benchmark file, read the canonical
`internal/search/semantic/sql/schema.sql`, and call the same feature-local sqlc
package. There is no alternate handwritten domain query.
Only the registered driver name and equivalent DSN pragma spelling differ.
`TestDriverConnectionPolicies` asserts the compared writer's 5-second busy
timeout, foreign keys, WAL, and full synchronous mode, plus the process-cold
reader's read-only/query-only policy and write rejection.
The cold-load lane uses the generated `GenerationChunkPage` query with the
production page size of 256, follows the same `(rel_path, ordinal)` keyset
cursor, and asserts the final row count. It intentionally stops before domain
vector decoding, so it isolates the driver and generated-query work.

Comparable fixture: the same 6,496 chunks, 502 notes, and 1,536-dimension BLOBs
as the product measurement. Runs were adjacent on the same Darwin/arm64 M1
with Go 1.26.5 and `-benchtime=1x`; every operation has 20 samples per driver.
The run used four adjacent 10-sample blocks in the order modernc, mattn,
modernc, mattn, with all three operations in every block. This prevents one
driver from owning the whole observation window while preserving enough raw
samples to expose the workstation's substantial variance. This remains a
noisy workstation measurement, not a controlled laboratory result. Benchstat
was `golang.org/x/perf` revision
`82a0b07e230d` and used its default 0.05 alpha.

| Operation | modernc | mattn | Benchstat conclusion |
|---|---:|---:|---|
| Initial build | 1.502 s ±6% | 1.363 s ±9% | mattn −9.26% (`p=0.004`, n=20) |
| Compatible one-note drift | 414.1 ms ±5% | 302.9 ms ±60% | no proven difference (`p=0.072`, n=20) |
| Process-cold paged load | 42.77 ms ±5% | 49.21 ms ±5% | mattn +15.06% (`p=0.000`, n=20) |
| Closed SQLite bytes | 54,214,656 | 54,214,656 | identical |
| Open SQLite + sidecar bytes | 57,773,728 | 57,773,728 | identical |

The two load timings differ from the product benchmark because this harness
isolates driver and generated-query work; it does not decode and validate every
vector into the domain `Generation` type. The comparison is for driver choice,
not a substitute for the product measurement above.

Retained-output SHA-256 digests:

- combined modernc blocks: `a62624bc8724386360b46c2506f9527b1d5ae467860018b2be6fc02b7fb4c209`
- combined mattn blocks: `62db35685e3ac36e2426ca9f15e42e7443c82f16482eaf1ba577bed2bc5aa54c`
- benchstat output: `d28ea0347fc9028d14fb5dd61885f59f55dec6f11ee96119574f6b118e03a8cd`

All three files are retained under
`docs/benchmarks/semantic-storage-2026-07-20/`; the names are `modernc.txt`,
`mattn.txt`, and `benchstat.txt`.
The benchstat file canonicalizes only its machine-specific temporary directory
prefix to `/tmp/yomihon-bakeoff-20260720`; its numerical output is unchanged.

This is on-demand decision evidence, not a product CI invariant. The result is
historical and cannot support a new driver decision after any input below
changes until the comparison is re-run. The product's modernc generation store
remains covered by the ordinary root test suite; only this alternate-driver
comparison is intentionally outside that gate.

| Measured input | SHA-256 |
|---|---|
| `internal/search/semantic/sql/schema.sql` | `b8ad043ac5aee1a506847feac51d09638d95596a9f2e7bf6f13bc7a54680c623` |
| `internal/search/semantic/sql/query.sql` | `0181cb39d981e01845563f33c20b40da71a735a40163e2a19c46d537f918d189` |
| generated `internal/search/semantic/catalog/query.sql.go` | `cd6c7670cc0d21b0fa3ce7186685ffad72515e2660d8ff85bd4a2233066c25f8` |
| `sqlc.yaml` | `1f160a5271124ea0265c7cdb1a54c411040c5569615a5576e462e238052aa333` |
| nested `go.mod` | `4896509a5e1049eb2daca55783d1b0b7ae2336fbe9bc0f214139dc287d122980` |
| common benchmark | `a43bad9b7f9f46ba0d73ebac6d4e190535f1cb3d37c9679a4fc309acf46b686e` |
| modernc driver | `ecaf312490f509412b97059f597f8667f7b9c5a88cf8faa670140e8b5285def4` |
| mattn driver | `f24338cae00779c49c753b65243215a898160be263859bc701b4809dfae25f48` |

Reproduce the alternating blocks, concatenate same-driver outputs, then pass
the combined files to `benchstat`:

```sh
cd tools/sqlite-driver-bakeoff
YOMIHON_DRIVER_CHUNKS=6496 YOMIHON_DRIVER_NOTES=502 \
YOMIHON_DRIVER_DIMENSION=1536 \
go test -tags=modernc -run '^$' \
  -bench '^BenchmarkDriverGeneration/(InitialBuild|CompatibleOneNoteDrift|ColdLoad)$' \
  -benchtime=1x -count=10 > modernc-a.txt
YOMIHON_DRIVER_CHUNKS=6496 YOMIHON_DRIVER_NOTES=502 \
YOMIHON_DRIVER_DIMENSION=1536 \
go test -tags=mattn -run '^$' \
  -bench '^BenchmarkDriverGeneration/(InitialBuild|CompatibleOneNoteDrift|ColdLoad)$' \
  -benchtime=1x -count=10 > mattn-a.txt
# Repeat those two commands as modernc-b.txt and mattn-b.txt.
cat modernc-a.txt modernc-b.txt > modernc.txt
cat mattn-a.txt mattn-b.txt > mattn.txt
benchstat modernc.txt mattn.txt
```

All three operations therefore use the same order and have 20 samples per
driver. Before using the result, run `shasum -a 256` over the eight measured
inputs above and record the new values beside the new raw-output digests.

Portability checks executed against the same nested module and measured-input
hashes:

- `CGO_ENABLED=0` modernc benchmark: passed;
- `CGO_ENABLED=0 GOOS=linux GOARCH=amd64` modernc test-binary build: passed;
- `CGO_ENABLED=0 GOOS=windows GOARCH=amd64` modernc test-binary build: passed;
- `CGO_ENABLED=0` mattn benchmark: failed with its explicit CGO-disabled stub,
  as the driver's documentation predicts.

The exact portability commands were:

```sh
CGO_ENABLED=0 YOMIHON_DRIVER_CHUNKS=6496 YOMIHON_DRIVER_NOTES=502 \
  YOMIHON_DRIVER_DIMENSION=1536 go test -tags=modernc -run '^$' \
  -bench '^BenchmarkDriverGeneration$' -benchtime=1x -count=1
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go test -c -tags=modernc -o /tmp/modernc-linux-amd64.test
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go test -c -tags=modernc -o /tmp/modernc-windows-amd64.test.exe
CGO_ENABLED=0 go test -tags=mattn -run '^TestDriverConnectionPolicies$'
cd ../..
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go test -c ./internal/search/semantic -o /tmp/semantic-windows-amd64.test.exe
```

The modernc Windows command above only compiled a test binary; it did not run
the semantic store on Windows. The product package also cross-compiled, but v1
explicitly classifies Windows as unsupported and its entry points return
`ErrStoreUnsupportedPlatform` before creating files. Nothing in this report
claims Windows semantic runtime support.

## PostgreSQL, Neon, and pgvector boundary

This measurement does not open the PostgreSQL rung. The current trigger is
100,000 chunks, more than 1 GiB of raw exact-vector payload, or exact top-k p95
above about 100 ms. At 3,072 dimensions the byte guard opens the comparison at
87,382 chunks, before the count guard. Reaching a trigger opens a comparison;
it does not preselect PostgreSQL.

That future comparison must freeze one synthetic corpus, filters, and recorded
query vectors; use brute-force exact top-50 as the oracle; and report build,
bounded drift, process-cold and warm-query p50/p95/p99, peak RSS, steady/peak
disk, crash recovery, and operating cost. PostgreSQL exact search must preserve
filter/path completeness and deterministic ordering and meet the predeclared
latency target before replacing the embedded design.

Neon is the preferred managed PostgreSQL candidate if the rung opens. Direct
and pooled connections, selected region, warm and scale-to-zero latency,
outage behavior, and monthly cost are separate measurements. Until an explicit
egress ruling authorizes **any** real-vault corpus- or query-derived transfer
— including vectors, query vectors, filters, and identifiers whether or not
they are persisted — that lane uses synthetic data only. pgvector ANN is
later still: PostgreSQL exact search
must first miss its target, and a frozen ANN configuration must then achieve
top-50 recall at least 0.98 overall and 0.96 for every query, with zero filter
leakage and no regression below the recorded recall@5 floor.
