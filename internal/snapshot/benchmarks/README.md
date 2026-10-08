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

[lifecycle-baseline.txt](lifecycle-baseline.txt) records the lifecycle measurement
on 2026-10-07, using production code at main
`0372ddf227c92f18e16c2720ec19c7278d78d66e` and the measurement harness at
`6a2dbccb709a006a9501ed0af9849d378f41d793` on this branch. This PR changes no
production code.
The fixture trace lines and their benchmark-log headers have been removed;
no measured line was changed. The original `baseline.txt` remains unchanged.

- Toolchain: Go 1.27.1, linux/amd64; AMD EPYC 9V45 96-Core Processor.
- Settings: `GOMAXPROCS=2`, empty `GOFLAGS`, `-benchtime=1x`, `-count=10`.
- Sample policy: five cases (`initial`, `rebuild`, `idle`, `overlap`, `visible`)
  at 100, 1,000 and 5,000 notes; ten samples per case, one operation per sample.
- Cache: warm after fixture construction and complete receipt; no cache flush.
- Original source bytes: 35,908 / 337,408 / 1,677,408 for the three sizes.
  Each fixture has N+1 files and 2N links.
- Exit status: 0. Package duration: 169.234 seconds.

```sh
GOMAXPROCS=2 go test -run='^$' -bench='^BenchmarkRepresentativeSnapshot$' -benchmem -benchtime=1x -count=10 ./internal/snapshot -args -snapshot-bench
```

Timed boundaries, retained owners and observation definitions are in
[the benchmark guide](../BENCHMARKS.md). These historical measurements describe
one synthetic workload and environment; they do not certify later harness
changes, an optimization, a capacity limit or a comparison with the Apple M1
record. Source/artifact binding and mutation receipts belong to the pull request.
