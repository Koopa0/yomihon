# time.Time and strings/bytes — Deep Patterns

Deep material for time handling and string/bytes building.
The time.Equal vs == rule and strings.Builder pattern live in SKILL.md.

## time: Storage and Transmission

```go
// ✅ ALWAYS store and transmit as UTC
createdAt := time.Now().UTC()

// ✅ Use predefined format constants
formatted := t.Format(time.RFC3339)          // "2024-01-15T09:30:00Z"
formatted := t.Format(time.DateOnly)         // "2024-01-15"
formatted := t.Format(time.DateTime)         // "2024-01-15 09:30:00"

// ❌ WRONG — custom format string for standard formats
t.Format("2006-01-02T15:04:05Z07:00") // just use time.RFC3339
t.Format("2006-01-02")                 // just use time.DateOnly
```

## time.Tick Leaks

```go
// ❌ WRONG — time.Tick leaks the ticker (never garbage collected)
for range time.Tick(5 * time.Second) {
    doWork()
}

// ✅ CORRECT — time.NewTicker with explicit Stop
ticker := time.NewTicker(5 * time.Second)
defer ticker.Stop()
for {
    select {
    case <-ctx.Done():
        return ctx.Err()
    case <-ticker.C:
        doWork()
    }
}
```

## Duration Construction

```go
// ❌ WRONG — arithmetic on raw int
timeout := 30 * 1000000000 // nanoseconds? unclear

// ✅ CORRECT — named constants
timeout := 30 * time.Second
interval := 500 * time.Millisecond
```

## time Anti-Patterns

```go
// ❌ WRONG — time.Now() in pure function (untestable)
func (o *Order) IsExpired() bool {
    return time.Now().After(o.ExpiresAt) // can't test!
}

// ✅ CORRECT — accept time as parameter
func (o *Order) IsExpired(now time.Time) bool {
    return now.After(o.ExpiresAt)
}

// ❌ WRONG — time.Sleep for testing
time.Sleep(100 * time.Millisecond) // flaky, slow

// ✅ CORRECT — use channels, tickers, or fake clocks
```

## bytes.Buffer.Peek (Go 1.26+)

```go
// Peek at buffer contents without consuming — useful for sniffing content type
buf := bytes.NewBufferString("hello world")
peek := buf.Peek(5) // returns "hello" without advancing read position
```

## strings.Cut (Go 1.18+)

```go
// ❌ WRONG — index gymnastics
i := strings.Index(header, ":")
if i == -1 {
    return "", "", false
}
key := header[:i]
value := strings.TrimSpace(header[i+1:])

// ✅ CORRECT — strings.Cut
key, value, found := strings.Cut(header, ":")
if !found {
    return "", "", false
}
value = strings.TrimSpace(value)
```

## strings.EqualFold for Case-Insensitive Comparison

```go
// ❌ WRONG — allocates lowercased copies
if strings.ToLower(a) == strings.ToLower(b) { ... }

// ✅ CORRECT — no allocation
if strings.EqualFold(a, b) { ... }
```

## strings Anti-Patterns

```go
// ❌ WRONG — fmt.Sprintf for simple concatenation
path := fmt.Sprintf("%s/%s", dir, file)

// ✅ CORRECT — path.Join for file paths
path := filepath.Join(dir, file)

// ❌ WRONG — regexp for simple contains/prefix/suffix check
matched, _ := regexp.MatchString(`^prefix`, s)

// ✅ CORRECT — strings functions
strings.HasPrefix(s, "prefix")
```
