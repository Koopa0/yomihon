package snapshot

import (
	"context"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/koopa0/yomihon/internal/search"
)

// TestRunScannerTicksAndStops pins only the scanner loop's clock, cancellation,
// and goroutine behavior. Filesystem rebuilding belongs to ordinary tests:
// system calls are not durably blocking inside a synctest bubble.
func TestRunScannerTicksAndStops(t *testing.T) {
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

func TestViewConcurrentReadersCannotMutateGeneration(t *testing.T) {
	t.Parallel()
	view, _ := immutableViewFixture(t)

	synctest.Test(t, func(t *testing.T) {
		for range 32 {
			go func() {
				for range 100 {
					request := view.Capture()
					resolution := request.Graph().Resolve("Foo")
					resolution.Candidates[0] = "mutated"

					folders := request.Navigation().Folders()
					folders[0].Name = "mutated"

					results, err := request.Search().Search(search.Parse("Foo"))
					if err != nil {
						t.Errorf("Search(Foo) error = %v", err)
						continue
					}
					results[0].Title = "mutated"
					counts, err := request.Search().CountByStatus()
					if err != nil {
						t.Errorf("CountByStatus() error = %v", err)
						continue
					}
					counts["draft"] = 0

					slot, ok := request.Slots().Lookup("lesson-l01")
					if !ok {
						t.Error("Slots().Lookup(lesson-l01) = false")
						continue
					}
					position := slot.Patterns[0].Slots["A"]
					position.Fills[0].JP = "mutated"
					slot.Patterns[0].Slots["A"] = position

					_ = request.ArtifactPolicy().ValidateSource()
				}
			}()
		}
		synctest.Wait()
	})

	if got := view.Graph().Resolve("Foo").Candidates[0]; got != "A/Foo.md" {
		t.Errorf("Resolve(Foo) after concurrent mutation starts with %q, want %q", got, "A/Foo.md")
	}
	if got := view.Navigation().Folders()[0].Name; got == "mutated" {
		t.Error("Navigation().Folders() retained a concurrent caller mutation")
	}
	if got := snapshotSearch(t, view.Search(), "Foo")[0].Title; got != "Foo" {
		t.Errorf("Search(Foo) after concurrent mutation starts with title %q, want %q", got, "Foo")
	}
	slot, ok := view.Slots().Lookup("lesson-l01")
	if !ok {
		t.Fatal("Slots().Lookup(lesson-l01) = false")
	}
	if got := slot.Patterns[0].Slots["A"].Fills[0].JP; got != "私" {
		t.Errorf("Slots().Lookup() after concurrent mutation fill = %q, want %q", got, "私")
	}
}
