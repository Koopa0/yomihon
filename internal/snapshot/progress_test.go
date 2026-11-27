package snapshot

import (
	"context"
	"path"
	"slices"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// readWatcher says how far the build has got each time it reads a file, which
// is the moment another goroutine would be asking.
type readWatcher struct {
	Source

	progress *Progress
	seen     [][2]int
	paths    []string
	folders  []string
}

func (w *readWatcher) ReadFile(ctx context.Context, entry vault.Entry) ([]byte, error) {
	done, total := w.progress.Files()
	w.seen = append(w.seen, [2]int{done, total})
	w.paths = append(w.paths, entry.Path())
	w.folders = append(w.folders, w.progress.Folder())
	return w.Source.ReadFile(ctx, entry)
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
	reader := openReader(t, root)

	progress := new(Progress)
	if done, total := progress.Files(); done != 0 || total != 0 {
		t.Fatalf("before the build, Files() = %d, %d, want 0, 0", done, total)
	}
	watcher := &readWatcher{Source: reader, progress: progress}
	if _, err := New(t.Context(), watcher, discardLogger(), nil, schema.Ungoverned(), WithProgress(progress)); err != nil {
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

// TestProgressNamesTheFolderBeingRead checks the other thing a waiting page is
// told: which folder of the vault the build is in, as a path within it, so that
// the reader sees the read move through the folders and a file at the top of the
// vault is read in no folder at all.
func TestProgressNamesTheFolderBeingRead(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range []string{"Top.md", "courses/go/One.md", "courses/go/Two.md", "courses/design/deep/Three.md"} {
		writeNote(t, root, name, "# "+name+"\n")
	}
	reader := openReader(t, root)

	progress := new(Progress)
	if folder := progress.Folder(); folder != "" {
		t.Fatalf("before the build, Folder() = %q, want none", folder)
	}
	watcher := &readWatcher{Source: reader, progress: progress}
	if _, err := New(t.Context(), watcher, discardLogger(), nil, schema.Ungoverned(), WithProgress(progress)); err != nil {
		t.Fatalf("New: %v", err)
	}

	if len(watcher.paths) != 4 {
		t.Fatalf("the build read %d files, want 4: %v", len(watcher.paths), watcher.paths)
	}
	for i, file := range watcher.paths {
		want := path.Dir(file)
		if want == "." {
			want = ""
		}
		if watcher.folders[i] != want {
			t.Errorf("reading %q, Folder() = %q, want %q", file, watcher.folders[i], want)
		}
	}
	if folder := progress.Folder(); folder != "" {
		t.Errorf("after the build, Folder() = %q, want none: nothing is being read", folder)
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
	if folder := progress.Folder(); folder != "" {
		t.Errorf("a nil Progress names the folder %q", folder)
	}
	progress.begin(5)
	progress.reached(2, "")
}
