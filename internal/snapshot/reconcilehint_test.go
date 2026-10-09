package snapshot

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/vault"
)

const hintNote = "Concepts/Alpha.md"
const hintOld = "# Alpha\n\nOLD_BODY\n"
const hintNew = "# Alpha\n\nNEW_BODY\n"

func hintFixture(t *testing.T) *warningFixture {
	t.Helper()
	root := t.TempDir()
	writeNote(t, root, hintNote, hintOld)
	return newWarningFixture(t, root, nil)
}

func stealthHintEdit(t *testing.T, f *warningFixture) {
	t.Helper()
	entry, ok := f.store.Current().Entry(hintNote)
	if !ok {
		t.Fatal("note has no captured entry")
	}
	if len(hintOld) != len(hintNew) {
		t.Fatal("stealth edit changed size")
	}
	file := filepath.Join(f.root, filepath.FromSlash(hintNote))
	if err := os.WriteFile(file, []byte(hintNew), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(file, entry.ModTime(), entry.ModTime()); err != nil {
		t.Fatal(err)
	}
	before := f.store.Current()
	f.store.rescan(t.Context())
	if f.store.Current() != before {
		t.Fatal("edit was metadata-visible")
	}
}

func TestReconcileHintPublishesStealthEdit(t *testing.T) {
	t.Parallel()
	f := hintFixture(t)
	stealthHintEdit(t, f)
	f.store.RequestReconcile()
	f.clock = f.clock.Add(time.Minute)
	f.store.rescan(t.Context())
	n, ok := f.store.Current().Note(hintNote)
	if !ok || !strings.Contains(n.Body, "NEW_BODY") {
		t.Fatalf("hinted note body = %q, want NEW_BODY", n.Body)
	}
}

func TestReconcileHintsRespectMinuteFloor(t *testing.T) {
	t.Parallel()
	f := hintFixture(t)
	before := f.source.reads[hintNote]
	for range 29 {
		f.store.RequestReconcile()
		f.clock = f.clock.Add(2 * time.Second)
		f.store.rescan(t.Context())
	}
	if got := f.source.reads[hintNote] - before; got != 0 {
		t.Fatalf("within-floor reads = %d, want 0", got)
	}
	f.clock = f.clock.Add(2 * time.Second)
	f.store.rescan(t.Context())
	if got := f.source.reads[hintNote] - before; got != 1 {
		t.Fatalf("coalesced floor reads = %d, want 1", got)
	}
}

func TestReconcileHintsRespectBackoff(t *testing.T) {
	t.Parallel()
	f := hintFixture(t)
	writeNote(t, f.root, hintNote, hintOld+"changed size\n")
	f.source.refuse[hintNote] = errors.New("temporary refusal")
	f.store.rescan(t.Context())
	if !f.store.retry {
		t.Fatal("fixture did not start retry backoff")
	}
	f.clock = f.store.nextRetry.Add(-time.Nanosecond)
	before := f.source.reads[hintNote]
	for range 10 {
		f.store.RequestReconcile()
		f.store.rescan(t.Context())
	}
	if got := f.source.reads[hintNote] - before; got != 0 {
		t.Fatalf("backoff reads = %d, want 0", got)
	}
}

func TestReconcileHintDuringBuildSurvives(t *testing.T) {
	t.Parallel()
	f := hintFixture(t)
	writeNote(t, f.root, hintNote, hintOld+"visible edit\n")
	f.source.beforeRead = func(_ context.Context, entry vault.Entry) {
		if entry.Path() == hintNote {
			f.store.RequestReconcile()
		}
	}
	f.store.rescan(t.Context())
	f.source.beforeRead = nil
	before := f.source.reads[hintNote]
	f.clock = f.clock.Add(time.Minute)
	f.store.rescan(t.Context())
	if got := f.source.reads[hintNote] - before; got != 1 {
		t.Fatalf("mid-build hint reads = %d, want 1", got)
	}
	f.clock = f.clock.Add(time.Minute)
	f.store.rescan(t.Context())
	if got := f.source.reads[hintNote] - before; got != 1 {
		t.Fatalf("consumed hint repeated reads = %d, want 1", got)
	}
}
