package snapshot

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math/rand/v2"
	"runtime"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/vault"
)

const measurementNote = "Concepts/golang/Note-00000.md"
const visiblePollInterval = 10 * time.Millisecond
const visibleTimeout = 10 * time.Second

// measuredSource retains the real rooted capability and records whether the
// synchronous reconciliation scan succeeded. Retaining Current alone cannot
// distinguish an unchanged vault from a refused scan.
type measuredSource struct {
	ObservedSource
	completed bool
	scanErr   error
}

func (s *measuredSource) ScanAvailable(ctx context.Context) (vault.Scan, error) {
	scan, err := s.ObservedSource.ScanAvailable(ctx)
	s.completed = true
	s.scanErr = err
	return scan, err
}

func idleObservation(tb testing.TB, before, after *Generation, completed bool, scanErr error) error {
	tb.Helper()
	measurementHit(tb, "idle")
	if !completed {
		return fmt.Errorf("caught: idle scan did not complete")
	}
	if scanErr != nil {
		return fmt.Errorf("caught: idle scan failed: %w", scanErr)
	}
	if before != after && !measurementFault("idle-publication") {
		return fmt.Errorf("caught: idle published a replacement generation")
	}
	return nil
}

func overlapObservation(tb testing.TB, before, after *Generation, expected [sha256.Size]byte) error {
	tb.Helper()
	measurementHit(tb, "overlap")
	if before == after && !measurementFault("overlap-no-replacement") {
		return fmt.Errorf("caught: overlap did not publish a replacement generation")
	}
	reading, ok := after.Note(measurementNote)
	if !ok || reading.ContentIdentity != expected {
		return fmt.Errorf("caught: overlap did not publish the expected body identity")
	}
	return nil
}

func prepareMeasurementEdit(tb testing.TB, f *representativeFixture, original string, longer bool) [sha256.Size]byte {
	tb.Helper()
	suffix := "\nA measured body edit.\n"
	if longer {
		suffix = "\nA measured body edit with a different size.\n"
	}
	f.sources[measurementNote] = original + suffix
	return vault.ContentIdentity([]byte(f.sources[measurementNote]))
}

// heapSample collects first and keeps every promised owner alive through the
// stats read. The fixture and Store are shared overhead in every observation.
func heapSample(f *representativeFixture, store *Store, held *Generation) uint64 {
	current := store.Current()
	runtime.GC()
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	runtime.KeepAlive(held)
	runtime.KeepAlive(current)
	runtime.KeepAlive(store)
	runtime.KeepAlive(f)
	return stats.HeapAlloc
}

// overlapSample confines the old generation to this frame. Returning scalars
// prevents testing.B.Loop's keep-alive treatment from retaining it during the
// caller's released/current-only collection.
//
//go:noinline
func overlapSample(b *testing.B, f *representativeFixture, store *Store, original string, longer bool) (uint64, uint64) {
	b.Helper()
	old := store.Current()
	one := heapSample(f, store, old)
	expected := prepareMeasurementEdit(b, f, original, longer)
	writeBenchNote(b, f.root, measurementNote, f.sources[measurementNote])
	b.StartTimer()
	store.rescan(b.Context())
	b.StopTimer()
	if err := overlapObservation(b, old, store.Current(), expected); err != nil {
		b.Fatal(err)
	}
	representativeReceipt(b, f, store.Current())
	two := heapSample(f, store, old)
	return one, two
}

// awaitVisible observes the identity a reader can obtain, never scanner-private
// timing. The deadline is an operational failure bound, not a latency budget.
func awaitVisible(ctx context.Context, tb testing.TB, store *Store, expected [sha256.Size]byte, scannerDone <-chan struct{}) error {
	tb.Helper()
	measurementHit(tb, "visible")
	ticker := time.NewTicker(visiblePollInterval)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return fmt.Errorf("caught: visible timeout or cancellation: %w", ctx.Err())
		}
		select {
		case <-scannerDone:
			return fmt.Errorf("caught: visible scanner exited before observation")
		default:
		}
		reading, ok := store.Current().Note(measurementNote)
		if ok && (reading.ContentIdentity == expected || measurementFault("visible-wrong-identity")) {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("caught: visible timeout or cancellation: %w", ctx.Err())
		case <-scannerDone:
			return fmt.Errorf("caught: visible scanner exited before observation")
		case <-ticker.C:
		}
	}
}

func waitMeasurementDelay(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("waiting before body edit: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

type measurementScanner struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func startMeasurementScanner(ctx context.Context, store *Store) (context.Context, measurementScanner) {
	owned, cancel := context.WithTimeout(ctx, visibleTimeout)
	scanner := measurementScanner{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(scanner.done)
		store.Run(owned)
	}()
	return owned, scanner
}

func (s measurementScanner) stop() {
	s.cancel()
	<-s.done
}

func visibleSample(b *testing.B, f *representativeFixture, original string, longer bool) {
	b.Helper()
	store := representativeGeneration(b, f)
	representativeReceipt(b, f, store.Current())
	ctx, scanner := startMeasurementScanner(b.Context(), store)
	// Deferred join runs even when a write or receipt calls Fatal, before the
	// rooted reader registered with Cleanup is closed.
	defer scanner.stop()
	delay := time.Duration(rand.Int64N(int64(scanInterval))) // #nosec G404 -- samples edit timing without making a security decision
	if err := waitMeasurementDelay(ctx, delay); err != nil {
		b.Fatal(err)
	}
	previous, ok := store.Current().Note(measurementNote)
	if !ok {
		b.Fatal("caught: visible edited note missing before write")
	}
	expected := prepareMeasurementEdit(b, f, original, longer)
	if expected == previous.ContentIdentity {
		b.Fatal("caught: visible body edit has unchanged identity")
	}
	b.StartTimer()
	writeBenchNote(b, f.root, measurementNote, f.sources[measurementNote])
	err := awaitVisible(ctx, b, store, expected, scanner.done)
	b.StopTimer()
	scanner.stop()
	if err != nil {
		b.Fatal(err)
	}
	representativeReceipt(b, f, store.Current())
}

func benchmarkLifecycle(b *testing.B, f *representativeFixture, store *Store, name string) {
	b.Helper()
	original := f.sources[measurementNote]
	longer := false
	b.ReportAllocs()
	if name == "idle" {
		source := &measuredSource{ObservedSource: f.reader}
		store.source = source
		store.rootObserver = source
		for b.Loop() {
			b.StopTimer()
			before := store.Current()
			source.completed = false
			source.scanErr = nil
			b.StartTimer()
			store.rescan(b.Context())
			b.StopTimer()
			if err := idleObservation(b, before, store.Current(), source.completed, source.scanErr); err != nil {
				b.Fatal(err)
			}
			representativeReceipt(b, f, store.Current())
			b.StartTimer()
		}
		return
	}
	var oneTotal, twoTotal, releasedTotal float64
	for b.Loop() {
		b.StopTimer()
		switch name {
		case "overlap":
			one, two := overlapSample(b, f, store, original, longer)
			released := heapSample(f, store, store.Current())
			if released >= two {
				b.Fatalf("caught: overlap release did not reduce heap: two=%d released=%d", two, released)
			}
			oneTotal += float64(one)
			twoTotal += float64(two)
			releasedTotal += float64(released)
		case "visible":
			visibleSample(b, f, original, longer)
		default:
			b.Fatalf("unknown lifecycle benchmark %q", name)
		}
		longer = !longer
		b.StartTimer()
	}
	if name == "overlap" {
		count := float64(b.N)
		b.ReportMetric(oneTotal/count, "one-heap-B")
		b.ReportMetric(twoTotal/count, "two-heap-B")
		b.ReportMetric(releasedTotal/count, "released-heap-B")
		b.ReportMetric((twoTotal-releasedTotal)/count, "reclaimed-B")
	}
}
