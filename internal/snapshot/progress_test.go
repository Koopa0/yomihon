package snapshot

import (
	"context"
	"slices"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// readWatcher says how far the build has got each time it reads a file, which
// is the moment another goroutine would be asking.
type readWatcher struct {
	*vault.Reader

	progress *Progress
	seen     [][2]int
}

func (w *readWatcher) ReadFile(ctx context.Context, entry vault.Entry) ([]byte, error) {
	done, total := w.progress.Files()
	w.seen = append(w.seen, [2]int{done, total})
	return w.Reader.ReadFile(ctx, entry)
}

// TestProgressCountsTheFilesOfTheFirstBuild checks what a page that is waiting
// on the first read is told: the total once the folder is walked, and a count
// that rises with every file and ends at the total.
func TestProgressCountsTheFilesOfTheFirstBuild(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range []string{"A.md", "B.md", "C.md"} {
		writeNote(t, root, name, "# "+name+"\n")
	}
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	t.Cleanup(func() { closeReader(t, reader) })

	progress := new(Progress)
	if done, total := progress.Files(); done != 0 || total != 0 {
		t.Fatalf("before the build, Files() = %d, %d, want 0, 0", done, total)
	}
	watcher := &readWatcher{Reader: reader, progress: progress}
	if _, err = New(t.Context(), watcher, discardLogger(), nil, schema.Ungoverned(), WithProgress(progress)); err != nil {
		t.Fatalf("New: %v", err)
	}

	want := [][2]int{{0, 3}, {1, 3}, {2, 3}}
	if !slices.Equal(watcher.seen, want) {
		t.Errorf("progress seen at each read = %v, want %v", watcher.seen, want)
	}
	if done, total := progress.Files(); done != 3 || total != 3 {
		t.Errorf("after the build, Files() = %d, %d, want 3, 3", done, total)
	}
}

// TestNilProgressIsAnUnwatchedBuild keeps the commands and every later rescan,
// which have no page waiting on them, from needing one.
func TestNilProgressIsAnUnwatchedBuild(t *testing.T) {
	t.Parallel()
	var progress *Progress
	if done, total := progress.Files(); done != 0 || total != 0 {
		t.Errorf("a nil Progress reports %d, %d, want 0, 0", done, total)
	}
	progress.begin(5)
	progress.reached(2)
}
