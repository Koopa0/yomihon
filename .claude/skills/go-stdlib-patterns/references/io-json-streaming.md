# io and encoding/json — Deep Patterns

Deep material for io.Reader/Writer composition and JSON encoding/decoding.
Decision trees, the io.ReadAll rule, and the omitempty trap live in SKILL.md.

## io Composition Patterns

```go
// Hash while copying — no intermediate buffer
func copyWithHash(dst io.Writer, src io.Reader) ([]byte, error) {
    h := sha256.New()
    tee := io.TeeReader(src, h) // reads from src, writes to h
    if _, err := io.Copy(dst, tee); err != nil {
        return nil, fmt.Errorf("copying: %w", err)
    }
    return h.Sum(nil), nil
}

// Limit untrusted input — prevent OOM
const maxBodySize = 1 << 20 // 1 MB
limited := io.LimitReader(r.Body, maxBodySize)
if err := json.NewDecoder(limited).Decode(&v); err != nil {
    return fmt.Errorf("decoding: %w", err)
}

// Multi-writer — write to file and hasher simultaneously
func saveWithChecksum(path string, src io.Reader) ([]byte, error) {
    f, err := os.Create(path)
    if err != nil {
        return nil, fmt.Errorf("creating file: %w", err)
    }
    defer f.Close()

    h := sha256.New()
    if _, err := io.Copy(io.MultiWriter(f, h), src); err != nil {
        return nil, fmt.Errorf("writing: %w", err)
    }
    return h.Sum(nil), nil
}
```

## io Anti-Patterns

```go
// ❌ WRONG — reading entire file just to count lines
data, _ := io.ReadAll(file)
lines := bytes.Count(data, []byte("\n"))

// ✅ CORRECT — scan line by line, constant memory
scanner := bufio.NewScanner(file)
var lines int
for scanner.Scan() {
    lines++
}

// ❌ WRONG — intermediate buffer for copy
buf := make([]byte, 1024)
for {
    n, err := src.Read(buf)
    dst.Write(buf[:n])
    if err == io.EOF { break }
}

// ✅ CORRECT — io.Copy handles buffering
io.Copy(dst, src)
```

## json.Number vs float64

```go
// ❌ WRONG — JSON numbers decoded as float64 by default
var data map[string]any
json.Unmarshal([]byte(`{"id": 9007199254740993}`), &data)
id := data["id"].(float64) // loses precision! 9007199254740992

// ✅ CORRECT — use json.Number for arbitrary precision
dec := json.NewDecoder(r.Body)
dec.UseNumber()
var data map[string]any
dec.Decode(&data)
id := data["id"].(json.Number)
n, _ := id.Int64() // exact: 9007199254740993
```

**When to use json.Number**: any `map[string]any` decoding where integer precision
matters (IDs, timestamps, financial values).

## json.RawMessage — Defer Parsing

```go
// Parse envelope, defer inner payload based on type
type Event struct {
    Type    string          `json:"type"`
    Payload json.RawMessage `json:"payload"` // raw JSON, parsed later
}

var event Event
if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
    return fmt.Errorf("decoding event: %w", err)
}

switch event.Type {
case "order.created":
    var order Order
    if err := json.Unmarshal(event.Payload, &order); err != nil {
        return fmt.Errorf("decoding order payload: %w", err)
    }
case "user.updated":
    var user User
    if err := json.Unmarshal(event.Payload, &user); err != nil {
        return fmt.Errorf("decoding user payload: %w", err)
    }
}
```

## json Anti-Patterns

```go
// ❌ WRONG — marshaling to compare JSON
got, _ := json.Marshal(a)
want, _ := json.Marshal(b)
if string(got) != string(want) { // key order not guaranteed!
    t.Error("mismatch")
}

// ✅ CORRECT — compare structs with cmp.Diff
if diff := cmp.Diff(want, got); diff != "" {
    t.Errorf("mismatch (-want +got):\n%s", diff)
}

// ❌ WRONG — ignoring Decode error
json.NewDecoder(r.Body).Decode(&v) // silently fails

// ✅ CORRECT — always check
if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
    return fmt.Errorf("decoding: %w", err)
}

// ❌ WRONG — encoding to string for logging
slog.Info("order", "data", string(mustMarshal(order)))

// ✅ CORRECT — slog handles structured types
slog.Info("order", "id", order.ID, "status", order.Status)
```
