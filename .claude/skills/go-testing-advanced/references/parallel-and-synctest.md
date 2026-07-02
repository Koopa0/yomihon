# Parallel Subtests and testing/synctest

How to structure parallel subtests and write deterministic concurrent tests.
The "When to Use t.Parallel" decision tree and the t.Setenv panic trap live
in SKILL.md.

## t.Parallel Rules

```go
func TestOrders(t *testing.T) {
    t.Parallel() // mark parent as parallel

    tests := []struct {
        name string
        // ...
    }{
        {name: "empty filter"},
        {name: "by status"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel() // each subtest runs in parallel
            // ...
        })
    }
}
```

## testing/synctest — Concurrent Tests with Fake Clock (Go 1.25+)

`synctest.Test` runs a function in an isolated "bubble" with a virtualized clock.
Replaces flaky `time.Sleep`-based concurrent tests.

```go
import "testing/synctest"

func TestDebouncer(t *testing.T) {
    synctest.Test(t, func(t *testing.T) {
        d := NewDebouncer(100 * time.Millisecond)
        d.Call(func() { /* ... */ })

        // fake clock: time.Sleep advances instantly when all goroutines block
        time.Sleep(50 * time.Millisecond)
        d.Call(func() { /* ... */ }) // resets timer

        time.Sleep(100 * time.Millisecond)
        synctest.Wait() // wait for all goroutines in bubble to block
        // assert callback was called exactly once
    })
}
```

Key points:
- `synctest.Test` runs the function in an isolated "bubble" with a fake clock
- `synctest.Wait()` blocks until all goroutines in the bubble are blocked
- Time advances only when all goroutines are idle — no real wall-clock delay
- Replaces flaky `time.Sleep`-based concurrent tests with deterministic execution
- Built-in leak detection: goroutines still blocked when the bubble ends FAIL
  the test — no third-party leak detector needed
