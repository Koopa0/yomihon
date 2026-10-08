# Recorded representative baselines

## Original local initial/rebuild baseline

[baseline.txt](baseline.txt) is the Go benchmark output measured from
source commit `729e0c5e0de022aa887873b1cf61416596a65698` on 2026-10-06, with the
harness's trace lines removed; no measured line was changed.
The later documentation commit adds this record without changing any measured
Go source. Regeneration and comparison instructions are in
[the benchmark guide](../BENCHMARKS.md).

- Toolchain: Go 1.27.1, darwin/arm64; Apple M1, Darwin 25.6.0.
- Settings: `GOMAXPROCS=2`, `GOFLAGS='-trimpath -p=2'`, `-benchtime=1x`, `-count=10`.
- Cache: warm local filesystem; source files were written and an initial
  generation validated before each timed operation. No cache flush.
- Each of six cases has ten samples and one operation per sample. Fixture
  creation, edit writes and complete source/generation checks are excluded.
- Original source bytes are 35,908 for 100 notes; 337,408 for 1,000 notes;
  1,677,408 for 5,000 notes. Files are N+1 and links are 2N in every sample.
- The command and benchstat both exited 0. The run took about 91 seconds locally.
  Benchstat used `golang.org/x/perf` version
  `v0.0.0-20260709024250-82a0b07e230d`.

```sh
GOMAXPROCS=2 GOFLAGS='-trimpath -p=2' go test -run='^$' \
  -bench='^BenchmarkRepresentativeSnapshot$' -benchmem -benchtime=1x -count=10 \
  ./internal/snapshot -args -snapshot-bench > /tmp/snapshot-baseline.txt
benchstat /tmp/snapshot-baseline.txt
```

This is a local baseline, with no old/new optimization comparison or capacity
claim. One-iteration results are sensitive to runtime and machine conditions;
the recorded 5,000-note rebuild samples have substantial variation. Keep the
same command, toolchain, hardware and cache conditions for a future comparison,
and inspect the distributions rather than treating a single row as a budget.

## Hosted lifecycle baseline

[lifecycle-baseline.txt](lifecycle-baseline.txt) is the complete, unmodified
50,161-byte `measurement.stdout.txt` from the `snapshot-lifecycle` artifact of
[CI run 37727506137](https://github.com/Koopa0/yomihon/actions/runs/37727506137),
[fuzz job 113149295317](https://github.com/Koopa0/yomihon/actions/runs/37727506137/job/113149295317).
Native platform/CPU headers, all benchmark rows, fixture trace lines, `PASS` and
the final package result are retained. No trace stripping or measured-line
rewriting was performed. The original `baseline.txt` remains unchanged.

The native measurement command exited **0** and its package result records
169.234 seconds. There are 150 measured rows: five cases (`initial`, `rebuild`,
`idle`, `overlap`, `visible`) at each of three sizes, ten samples per case and
one operation per sample. Every row carries the starting fixture counts:

| Notes | Files | Links | Original source bytes |
|---:|---:|---:|---:|
| 100 | 101 | 200 | 35,908 |
| 1,000 | 1,001 | 2,000 | 337,408 |
| 5,000 | 5,001 | 10,000 | 1,677,408 |

Every overlap row records a positive `reclaimed-B` and a lower
`released-heap-B` than `two-heap-B`. Every visible row completed by observing
the expected edited body identity; the benchmark refuses timeout, scanner exit
and incorrect identity. Definitions, retained ownership, GC inclusion, timed
boundaries and polling overhead are in [the benchmark guide](../BENCHMARKS.md).
These observations characterize this synthetic workload in this environment;
they establish no optimization, capacity, whole-process budget, or comparison
with the Apple M1 record.

### Immutable source and artifact binding

The runner checked out the pull-request merge commit, not the requested head.
The source bindings for this historical run are:

| Identity | Value |
|---|---|
| Measured checkout | `f060d5bde9b72dce3a20093035d03acf7674d1f7` |
| Requested pull-request head | `6a2dbccb709a006a9501ed0af9849d378f41d793` |
| Pull-request base | `0372ddf227c92f18e16c2720ec19c7278d78d66e` |
| Artifact | `snapshot-lifecycle`, ID `11529210026`, 36 entries |
| Uploaded artifact SHA-256 | `875630739664264556aa3aecb731e05826901f3b4a3d8d694fe7385e507db3b6` |

The digest above is GitHub's uploaded-artifact digest, not an asserted checksum
of a downloaded ZIP representation. The following Git blob identities were
equal in the measured checkout and requested head:

| Measured file | Git blob |
|---|---|
| [bench_test.go](https://github.com/Koopa0/yomihon/blob/f060d5bde9b72dce3a20093035d03acf7674d1f7/internal/snapshot/bench_test.go) | `688198e32feb8e8f3b6dbef346310dbf0ebcc27f` |
| [measurement_test.go](https://github.com/Koopa0/yomihon/blob/f060d5bde9b72dce3a20093035d03acf7674d1f7/internal/snapshot/measurement_test.go) | `90bbf6a9706b338afdd135e330fd1efc6051815a` |
| [measurementcontrol_test.go](https://github.com/Koopa0/yomihon/blob/f060d5bde9b72dce3a20093035d03acf7674d1f7/internal/snapshot/measurementcontrol_test.go) | `84a761e37eb304da07410a39c4bd70f38cc79c8d` |
| [representativeregistry_test.go](https://github.com/Koopa0/yomihon/blob/f060d5bde9b72dce3a20093035d03acf7674d1f7/internal/snapshot/representativeregistry_test.go) | `b52f8fddc7945471f71e7a202729c7053ec48874` |
| [ci.yml](https://github.com/Koopa0/yomihon/blob/f060d5bde9b72dce3a20093035d03acf7674d1f7/.github/workflows/ci.yml) | `91f409bf820bc95d1b7c3e935075ce48cea7ecb5` |

This record was added after the measured run and after later source lint fixes.
The temporary branch-scoped acquisition and artifact-upload steps have been
removed from the existing fuzz job. The later documentation head is separately
bound by the pull request and its final CI record; the earlier measurement does
not certify that later head, and its source identities are not inferred from
the later checkout.

### Recorded environment and command

- Toolchain: `go version go1.27.1 linux/amd64`; GOOS `linux`, GOARCH `amd64`.
- CPU: AMD EPYC 9V45 96-Core Processor; the guest exposes four logical CPUs,
  two threads per core and two cores per socket, under a Microsoft hypervisor.
- Runner: `ubuntu-latest`, `GitHub Actions 1000048914`; kernel
  `Linux runnervmmprz5 6.17.0-1022-azure #22-Ubuntu SMP Mon Jul 27 17:24:03 UTC 2026 x86_64 x86_64 x86_64 GNU/Linux`.
- Settings: `GOMAXPROCS=2`, empty `GOFLAGS`, `-benchtime=1x`, `-count=10`.
- Cache: warm after fixture construction and complete receipt; no cache flush.
- Visible: real two-second ticker, random pre-edit delay in `[0, 2s)`, 10 ms
  polling and a ten-second context from scanner startup; cancel/join precedes
  reader cleanup. The write and reader-observation overhead are included;
  construction, random wait, receipts and cancellation/join are excluded.
- Overlap: real GC before each HeapAlloc observation; old references remain
  in a non-inlined scalar-return helper until the two-generation observation.
  The released observation follows that return. KeepAlive occurs after each
  statistics read; fixture, Store and current generation remain retained.
  Only reconciliation is timed, including neither edit write nor GC/stat reads.
- Idle: synchronous unchanged reconciliation, including the delegating rooted
  scan wrapper's outcome recording; successful scan and unchanged publication
  are both required. Complete receipts are excluded from timing.

```sh
GOMAXPROCS=2 go test -run='^$' -bench='^BenchmarkRepresentativeSnapshot$' -benchmem -benchtime=1x -count=10 ./internal/snapshot -args -snapshot-bench
```

The raw artifact separately retains native measurement stdout, stderr, numeric
status, command and environment metadata. Measurement stderr was empty. No
benchstat comparison was performed for this lifecycle record.

### Semantic control receipts

The same job inventoried seven registered tests before running them. Each
controlled fault retained its selected state, an invocation marker emitted by
the actual helper or registration path, the exact caught marker, and native
status **1**. The restored `green` command ran all seven tests with native
status **0**, including the complete 15-leaf disabled inventory, bounded
100-note initial/rebuild command, actual scan success/refusal, expected-identity
polling and scanner cancellation/join controls.

| Control | Selected test | Exact caught marker |
|---|---|---|
| `idle-publication` | `TestMeasurementIdleObservation` | `caught: idle publication lock accepted replacement: <nil>` |
| `overlap-no-replacement` | `TestMeasurementOverlapObservation` | `caught: overlap publication lock accepted same generation: <nil>` |
| `visible-wrong-identity` | `TestMeasurementVisibleObservation` | `caught: visible identity lock accepted unpublished body: <nil>` |
| `missing-leaf` | `TestRepresentativeBenchmarkEntryAndOptIn` | `caught: disabled benchmark leaf inventory (-want +got):` |
| `bypass-opt-in` | `TestRepresentativeBenchmarkEntryAndOptIn` | `caught: disabled representative benchmark constructed a fixture` |

For example, the recorded idle RED invocation was:

```sh
GOMAXPROCS=2 go test -v -count=1 -run='^(TestMeasurementControlState|TestMeasurementIdleObservation)$' ./internal/snapshot -args -snapshot-measurement-control=idle-publication
```

The recorded restoration invocation was:

```sh
GOMAXPROCS=2 go test -v -count=1 -run='^(TestMeasurement(ControlState|ScanObservation|IdleObservation|OverlapObservation|VisibleObservation|VisibleCancellationJoins)|TestRepresentativeBenchmarkEntryAndOptIn)$' ./internal/snapshot -args -snapshot-measurement-control=green
```

These controls modify compiled test helpers in memory and do not rewrite source
files. A compiling invocation and exact behavioral marker distinguish an
intended RED from an unrelated build error. The acquisition harness reserves
status 2 for missing selected state or invocation. The raw receipts remain
associated with the identified artifact and job, separate from canonical
verification and the later final-head acceptance.
