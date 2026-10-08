package snapshot

import (
	"context"
	"errors"
	"flag"
	"io/fs"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/koopa0/yomihon/internal/vault"
)

// These controls change compiled benchmark guards in memory. Source files are
// never rewritten, and normal tests do not perform lifecycle measurements.
var measurementControl = flag.String("snapshot-measurement-control", "", "compiled measurement control: green or a named semantic fault")

func measurementFault(name string) bool {
	return *measurementControl == name
}

func measurementHit(tb testing.TB, name string) {
	tb.Helper()
	if *measurementControl != "" {
		tb.Log("invoked: measurement " + name)
	}
}

func registeredRepresentativePhases(tb testing.TB) []string {
	tb.Helper()
	measurementHit(tb, "registry")
	if measurementFault("missing-leaf") {
		return slices.DeleteFunc(slices.Clone(representativePhases), func(name string) bool { return name == "idle" })
	}
	return representativePhases
}

func representativeEnabled(tb testing.TB, notes int, name string) bool {
	tb.Helper()
	measurementHit(tb, "opt-in")
	if measurementFault("bypass-opt-in") && notes == 100 && name == "initial" {
		return true
	}
	return *representativeBench
}

func TestMeasurementControlState(t *testing.T) {
	if !slices.Contains([]string{"", "green", "idle-publication", "overlap-no-replacement", "visible-wrong-identity", "missing-leaf", "bypass-opt-in"}, *measurementControl) {
		t.Fatalf("not-applied: unknown measurement control %q", *measurementControl)
	}
	t.Logf("applied: measurement control %q", *measurementControl)
}

func TestMeasurementScanObservation(t *testing.T) {
	t.Parallel()
	f := newRepresentativeFixture(t, 8)
	reader, err := vault.Open(f.root)
	if err != nil {
		t.Fatal(err)
	}
	source := &measuredSource{ObservedSource: reader}
	_, scanErr := source.ScanAvailable(t.Context())
	if err := idleObservation(t, nil, nil, source.completed, source.scanErr); err != nil || scanErr != nil {
		closeReader(t, reader)
		t.Fatalf("caught: successful rooted scan observation refused: scan=%v observation=%v", scanErr, err)
	}
	closeReader(t, reader)
	source.completed = false
	_, scanErr = source.ScanAvailable(t.Context())
	if !errors.Is(scanErr, fs.ErrClosed) || !errors.Is(source.scanErr, fs.ErrClosed) || !source.completed {
		t.Fatalf("caught: rooted scan refusal was not recorded: scan=%v observation=%v completed=%v", scanErr, source.scanErr, source.completed)
	}
	if err := idleObservation(t, nil, nil, source.completed, source.scanErr); !errors.Is(err, fs.ErrClosed) {
		t.Fatalf("caught: rooted scan refusal was accepted: %v", err)
	}
}

func TestMeasurementIdleObservation(t *testing.T) {
	t.Parallel()
	f := newRepresentativeFixture(t, 8)
	store := representativeGeneration(t, f)
	before := store.Current()
	f.sources[measurementNote] += "\nAn independently changed body.\n"
	writeBenchNote(t, f.root, measurementNote, f.sources[measurementNote])
	store.rescan(t.Context())
	after := store.Current()
	if before == after {
		t.Fatal("not-applied: idle control generations are not distinct")
	}
	if err := idleObservation(t, before, after, true, nil); err == nil || err.Error() != "caught: idle published a replacement generation" {
		t.Fatalf("caught: idle publication lock accepted replacement: %v", err)
	}
	if err := idleObservation(t, before, before, true, nil); err != nil {
		t.Fatal(err)
	}
	refusal := errors.New("controlled scan refusal")
	if err := idleObservation(t, before, before, true, refusal); !errors.Is(err, refusal) {
		t.Fatalf("caught: idle scan refusal was accepted: %v", err)
	}
	if err := idleObservation(t, before, before, false, nil); err == nil || err.Error() != "caught: idle scan did not complete" {
		t.Fatalf("caught: idle missing scan was accepted: %v", err)
	}
}

func TestMeasurementOverlapObservation(t *testing.T) {
	t.Parallel()
	f := newRepresentativeFixture(t, 8)
	store := representativeGeneration(t, f)
	before := store.Current()
	reading, ok := before.Note(measurementNote)
	if !ok {
		t.Fatal("not-applied: overlap control note absent")
	}
	if err := overlapObservation(t, before, before, reading.ContentIdentity); err == nil || err.Error() != "caught: overlap did not publish a replacement generation" {
		t.Fatalf("caught: overlap publication lock accepted same generation: %v", err)
	}
	f.sources[measurementNote] += "\nAn independently changed body.\n"
	expected := vault.ContentIdentity([]byte(f.sources[measurementNote]))
	writeBenchNote(t, f.root, measurementNote, f.sources[measurementNote])
	store.rescan(t.Context())
	if err := overlapObservation(t, before, store.Current(), expected); err != nil {
		t.Fatal(err)
	}
	if err := overlapObservation(t, before, store.Current(), reading.ContentIdentity); err == nil || err.Error() != "caught: overlap did not publish the expected body identity" {
		t.Fatalf("caught: overlap wrong identity was accepted: %v", err)
	}
}

func TestMeasurementVisibleObservation(t *testing.T) {
	f := newRepresentativeFixture(t, 8)
	store := representativeGeneration(t, f)
	expected := vault.ContentIdentity([]byte(f.sources[measurementNote] + "\nUnpublished body edit.\n"))
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 3*visiblePollInterval)
		defer cancel()
		done := make(chan struct{})
		err := awaitVisible(ctx, t, store, expected, done)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("caught: visible identity lock accepted unpublished body: %v", err)
		}
		if err.Error() != "caught: visible timeout or cancellation: context deadline exceeded" {
			t.Fatalf("caught: visible timeout marker differs: %v", err)
		}
	})
	reading, ok := store.Current().Note(measurementNote)
	if !ok {
		t.Fatal("not-applied: visible control note absent")
	}
	if err := awaitVisible(t.Context(), t, store, reading.ContentIdentity, make(chan struct{})); err != nil {
		t.Fatal(err)
	}
	closed := make(chan struct{})
	close(closed)
	if err := awaitVisible(t.Context(), t, store, expected, closed); err == nil || err.Error() != "caught: visible scanner exited before observation" {
		t.Fatalf("caught: visible scanner exit was accepted: %v", err)
	}
}

func TestMeasurementVisibleCancellationJoins(t *testing.T) {
	f := newRepresentativeFixture(t, 8)
	store := representativeGeneration(t, f)
	synctest.Test(t, func(t *testing.T) {
		ctx, scanner := startMeasurementScanner(t.Context(), store)
		scanner.stop()
		select {
		case <-scanner.done:
		default:
			t.Fatal("caught: scanner owner returned without joining")
		}
		if _, err := f.reader.ScanAvailable(t.Context()); err != nil {
			t.Fatalf("caught: scanner join closed rooted reader early: %v", err)
		}
		if err := waitMeasurementDelay(ctx, time.Hour); !errors.Is(err, context.Canceled) {
			t.Fatalf("caught: pre-edit wait ignored cancellation: %v", err)
		}
	})
}
