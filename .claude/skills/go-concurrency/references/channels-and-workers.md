# Channels, Worker Pools, and Brokers

Implementation reference for channel-based concurrency. Decision trees and
common mistakes live in SKILL.md.

## Worker Pool

```go
func workerPool(ctx context.Context, jobs <-chan Job, results chan<- Result, workers int) {
    var wg sync.WaitGroup

    for range workers {
        wg.Go(func() { // Go 1.25+; use Add(1)/go/Done on older versions
            for {
                select {
                case <-ctx.Done():
                    return
                case job, ok := <-jobs:
                    if !ok {
                        return // channel closed, no more jobs
                    }
                    result, err := process(ctx, job)
                    if err != nil {
                        // Log or send error result
                        continue
                    }
                    select {
                    case results <- result:
                    case <-ctx.Done():
                        return
                    }
                }
            }
        })
    }

    wg.Wait()
    close(results)
}
```

### Usage

```go
// Buffer size is DERIVED, never guessed (concurrency.md): the bound here is
// the worker count — at most one prefetched job / uncollected result per worker.
const workers = 5
jobs := make(chan Job, workers)
results := make(chan Result, workers)

// Start workers
go workerPool(ctx, jobs, results, workers)

// Send jobs
go func() {
    defer close(jobs)
    for _, item := range items {
        select {
        case jobs <- Job{Item: item}:
        case <-ctx.Done():
            return
        }
    }
}()

// Collect results
for result := range results {
    // process result
}
```

## Channel Patterns

### Fan-Out / Fan-In

```go
// Fan-out: one source, multiple workers
func fanOut(ctx context.Context, source <-chan Item, workers int) []<-chan Result {
    channels := make([]<-chan Result, workers)
    for i := range workers {
        channels[i] = worker(ctx, source)
    }
    return channels
}

func worker(ctx context.Context, source <-chan Item) <-chan Result {
    out := make(chan Result)
    go func() {
        defer close(out)
        for item := range source {
            select {
            case <-ctx.Done():
                return
            case out <- process(item):
            }
        }
    }()
    return out
}

// Fan-in: multiple sources, one destination
// IMPORTANT: each input channel MUST be closed by its producer (defer close(out) in worker).
// If a producer exits without closing, fan-in goroutines will hang.
func fanIn(ctx context.Context, channels ...<-chan Result) <-chan Result {
    merged := make(chan Result)
    var wg sync.WaitGroup

    for _, ch := range channels {
        wg.Go(func() { // Go 1.25+
            for result := range ch {
                select {
                case <-ctx.Done():
                    return
                case merged <- result:
                }
            }
        })
    }

    go func() {
        wg.Wait()
        close(merged)
    }()

    return merged
}
```

### Pipeline

```go
// Stage 1: Generate
func generate(ctx context.Context, items []Item) <-chan Item {
    out := make(chan Item)
    go func() {
        defer close(out)
        for _, item := range items {
            select {
            case <-ctx.Done():
                return
            case out <- item:
            }
        }
    }()
    return out
}

// Stage 2: Transform
func transform(ctx context.Context, in <-chan Item) <-chan Result {
    out := make(chan Result)
    go func() {
        defer close(out)
        for item := range in {
            result := process(item)
            select {
            case <-ctx.Done():
                return
            case out <- result:
            }
        }
    }()
    return out
}

// Wire pipeline
items := generate(ctx, data)
results := transform(ctx, items)
for r := range results {
    // consume
}
```

## SSE Broker: In-Memory Pub/Sub

Real-world concurrent design with buffered channels and non-blocking publish:

```go
type Broker struct {
    mu         sync.RWMutex
    clients    map[chan string]struct{}
    bufferSize int
}

func NewBroker(bufferSize int) *Broker {
    return &Broker{
        clients:    make(map[chan string]struct{}),
        bufferSize: bufferSize,
    }
}

func (b *Broker) Subscribe() chan string {
    ch := make(chan string, b.bufferSize)
    b.mu.Lock()
    b.clients[ch] = struct{}{}
    b.mu.Unlock()
    return ch
}

func (b *Broker) Unsubscribe(ch chan string) {
    b.mu.Lock()
    delete(b.clients, ch)
    b.mu.Unlock()
    // Don't close ch here — subscriber might still be reading
}

// Non-blocking publish — slow clients miss events
func (b *Broker) Publish(event string) {
    b.mu.RLock()
    defer b.mu.RUnlock()
    for ch := range b.clients {
        select {
        case ch <- event:
        default:
            // Client buffer full — drop event
        }
    }
}
```

### Why Non-Blocking Publish

- **Blocking send**: one slow client blocks ALL clients
- **Non-blocking send with default**: slow clients drop events, fast clients unaffected
- **Buffered channel**: absorbs burst traffic, drops only if consistently slow
