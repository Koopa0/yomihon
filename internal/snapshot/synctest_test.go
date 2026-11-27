package snapshot

import (
	"context"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

// TestRunScannerTicksAndStops pins only the scanner loop's clock, cancellation,
// and goroutine behavior. Filesystem rebuilding belongs to ordinary tests:
// system calls are not durably blocking inside a synctest bubble.
func TestRunScannerTicksAndStops(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		var scans atomic.Int32
		go func() {
			runScanner(ctx, func() {
				scans.Add(1)
			})
			close(done)
		}()

		time.Sleep(scanInterval - time.Nanosecond)
		synctest.Wait()
		if got := scans.Load(); got != 0 {
			t.Errorf("scanner calls before interval = %d, want 0", got)
		}
		select {
		case <-done:
			t.Fatal("runScanner returned before cancellation")
		default:
		}

		time.Sleep(time.Nanosecond)
		synctest.Wait()
		if got := scans.Load(); got != 1 {
			t.Errorf("scanner calls after interval = %d, want 1", got)
		}
		select {
		case <-done:
			t.Fatal("runScanner returned after a scan without cancellation")
		default:
		}

		cancel()
		synctest.Wait()
		if got := scans.Load(); got != 1 {
			t.Errorf("scanner calls after cancellation = %d, want 1", got)
		}
		select {
		case <-done:
		default:
			t.Error("runScanner did not return after cancellation")
		}
	})
}
