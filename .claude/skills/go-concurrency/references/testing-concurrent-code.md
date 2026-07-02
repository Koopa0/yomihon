# Testing Concurrent Code

How to test goroutine lifecycle and time-dependent behavior. The goroutine
leak prevention pattern itself lives in SKILL.md.

## Leak Detection in Tests

PRIMARY mechanism: the `testing/synctest.Test` bubble (Go 1.25+). Goroutines
still blocked when the bubble ends FAIL the test — built-in leak detection,
stdlib-only, no third-party detector needed (see testing.md rule).

```go
import "testing/synctest"

func TestWorkerStopsOnCancel(t *testing.T) {
    synctest.Test(t, func(t *testing.T) {
        ctx, cancel := context.WithCancel(t.Context())
        go worker(ctx)
        cancel()
        // If worker's goroutine is still blocked when the bubble ends,
        // the test FAILS — that's the leak check.
    })
}
```

Caveats:
- Only goroutines started INSIDE the bubble are tracked
- The bubble virtualizes time — `time.Sleep`, timers, tickers use the fake clock
- Real I/O (network, database, files) does not belong inside a bubble

`go.uber.org/goleak` is a third-party dependency — REJECTED by default.
Adopting it is a Tier 3 dependency decision (go-philosophy.md § Dependencies),
justifiable only for leak-checking code that cannot run inside a synctest
bubble.

## Testing Concurrent Code: synctest (Go 1.25+)

Use `testing/synctest` to test concurrent code with a virtualized clock.
The fake clock advances only when all goroutines in the "bubble" are blocked,
making time-dependent tests deterministic and fast.

```go
import "testing/synctest"

func TestDebounce(t *testing.T) {
    synctest.Test(t, func(t *testing.T) {
        var count atomic.Int64
        go func() {
            time.Sleep(100 * time.Millisecond) // fake clock — no real delay
            count.Add(1)
        }()

        synctest.Wait() // wait for all goroutines in bubble to block
        if count.Load() != 1 {
            t.Fatal("expected count 1")
        }
    })
}
```

Key points:
- `synctest.Wait()` blocks until all goroutines in the bubble are idle
- `time.Sleep`, `time.After`, `time.NewTimer` all use the fake clock
- No flaky sleeps — the clock only advances when all goroutines block
- Use for testing timeouts, debounce, rate limiters, and ticker-based loops
