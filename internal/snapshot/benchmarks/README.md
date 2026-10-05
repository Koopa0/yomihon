# Recorded representative baseline

[baseline.txt](baseline.txt) is the unedited Go benchmark output measured from
source commit `729e0c5e0de022aa887873b1cf61416596a65698` on 2026-10-06.
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
