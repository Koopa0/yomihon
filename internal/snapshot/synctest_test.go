package snapshot

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/koopa0/yomihon/internal/search"
)

// TestRunSwapsOnlyOnChange pins the scanner's timing behavior on a fake clock:
// a tick over an unchanged vault leaves the published snapshot untouched, a
// tick after a note is added reflects the new note, and a cancelled context
// returns the loop promptly. Only time is virtual — the vault is a real
// directory and the scans do real file I/O — so the whole test finishes in
// milliseconds of wall clock rather than waiting out the scan cadence. The
// synctest bubble fails the test if the scanner goroutine is still alive at the
// end, which is how the prompt-return guarantee is enforced.
func TestRunSwapsOnlyOnChange(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		root := t.TempDir()
		writeNote(t, root, "Concepts/Alpha.md", "---\ntitle: Alpha\ntype: concept\n---\n\nalpha body\n")

		store := New(root, discardLogger())
		first := store.Current()

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		go func() {
			store.Run(ctx)
			close(done)
		}()

		// A tick with nothing changed on disk must not swap the pointer.
		time.Sleep(scanInterval + time.Second)
		synctest.Wait()
		if store.Current() != first {
			t.Error("snapshot pointer swapped after a tick with no vault change")
		}

		// A new note, then a tick: the live snapshot reflects it. The freshness
		// is checked by observable content rather than an internal counter.
		writeNote(t, root, "Concepts/Beta.md", "---\ntitle: Beta\ntype: concept\n---\n\nbeta mentions widgets\n")
		time.Sleep(scanInterval + time.Second)
		synctest.Wait()
		if store.Current() == first {
			t.Error("snapshot pointer did not swap after a vault change")
		}
		if got := store.Current().Search.Search(search.Parse("widgets")); len(got) == 0 {
			t.Error("rescan did not reflect the added note")
		}

		// A cancelled context returns the loop; the bubble's leak check would
		// fail the test if it did not.
		cancel()
		synctest.Wait()
		select {
		case <-done:
		default:
			t.Error("Run did not return after the context was cancelled")
		}
	})
}
