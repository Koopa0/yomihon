package snapshot

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

// TestRootObservationCauseStaysInWarning keeps the scanner's original cause
// available without making the reader's public state carry operational prose.
func TestRootObservationCauseStaysInWarning(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "vault")
	writeNote(t, root, "Existing.md", "old object\n")
	reader := openReader(t, root)
	log, logged := newRecordingLogger()
	store := buildStore(t, reader, log, nil)
	if err := os.Rename(root, root+".old"); err != nil {
		t.Fatal(err)
	}
	_, observeErr := reader.ObserveRoot()
	if observeErr == nil {
		t.Fatal("missing selected path did not return its cause")
	}
	for range 3 {
		store.rescan(t.Context())
	}
	const unconfirmed = "vault root identity unconfirmed; restore access or restart"
	want := []loggedLine{{Level: slog.LevelWarn, Message: unconfirmed, Attrs: map[string]string{
		"selected_path": root, "opened_root_name": reader.Name(), "error": observeErr.Error(),
	}}}
	if diff := cmp.Diff(want, linesWith(logged(), unconfirmed)); diff != "" {
		t.Fatalf("caught: scanner root WARN cause (-want +got):\n%s", diff)
	}
	public := &RootNotice{SelectedPath: root, OpenedName: reader.Name(), Unconfirmed: true}
	if diff := cmp.Diff(public, store.Current().Freshness().Root); diff != "" {
		t.Fatalf("caught: unconfirmed root state (-want +got):\n%s", diff)
	}
}

func TestRootReplacementWarnsOnceEvenWhenPinnedFilesAreUnchanged(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "vault")
	writeNote(t, root, "Notes/Existing.md", "old object\n")
	reader := openReader(t, root)
	log, logged := newRecordingLogger()
	store := buildStore(t, reader, log, nil)
	before := store.Current()
	if err := os.Rename(root, root+".old"); err != nil {
		t.Fatal(err)
	}
	writeNote(t, root, "Notes/Replacement.md", "replacement object\n")
	for range 3 {
		store.rescan(t.Context())
	}
	if store.Current() != before {
		t.Fatal("unchanged pinned files unexpectedly rebuilt")
	}
	lines := linesWith(logged(), "vault root identity changed; restart required")
	if len(lines) != 1 || lines[0].Level != slog.LevelWarn ||
		lines[0].Attrs["selected_path"] != root || lines[0].Attrs["opened_root_name"] != reader.Name() {
		t.Fatalf("root replacement warnings = %+v, want one WARN naming %s and %s", lines, root, reader.Name())
	}
}

func TestRootNoticeTracksRecoveryAcrossRetainedAndRebuiltCaptures(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "vault")
	writeNote(t, root, "Existing.md", "old object\n")
	reader := openReader(t, root)
	log, logged := newRecordingLogger()
	store := buildStore(t, reader, log, nil)
	retained := store.Current().Capture()
	if retained.Freshness().Root != nil {
		t.Fatal("unchanged startup root reported a problem")
	}
	if err := os.Rename(root, root+".old"); err != nil {
		t.Fatal(err)
	}
	writeNote(t, root, "Replacement.md", "replacement\n")
	want := &RootNotice{SelectedPath: root, OpenedName: reader.Name()}
	store.rescan(t.Context())
	if diff := cmp.Diff(want, retained.Freshness().Root); diff != "" {
		t.Fatalf("retained root notice (-want +got):\n%s", diff)
	}
	returned := retained.Freshness().Root
	returned.SelectedPath = "caller mutation"
	if diff := cmp.Diff(want, retained.Freshness().Root); diff != "" {
		t.Fatalf("caller changed the live root notice (-want +got):\n%s", diff)
	}
	writeNote(t, root+".old", "Existing.md", "the pinned folder still rebuilds normally\n")
	store.rescan(t.Context())
	if store.Current().Freshness().BuiltAt.Equal(retained.Freshness().BuiltAt) {
		t.Fatal("pinned-folder change did not rebuild")
	}
	for _, view := range []*Generation{retained, store.Current().Capture()} {
		if diff := cmp.Diff(want, view.Freshness().Root); diff != "" {
			t.Fatalf("complete rebuild lost root notice (-want +got):\n%s", diff)
		}
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		store.rescan(t.Context())
	}
	unknown := retained.Freshness().Root
	if unknown == nil || !unknown.Unconfirmed {
		t.Fatal("missing selected path was reported as confirmed")
	}
	unknown.Unconfirmed = false
	if diff := cmp.Diff(want, unknown); diff != "" {
		t.Fatalf("unconfirmed root labels (-want +got):\n%s", diff)
	}
	if err := os.Rename(root+".old", root); err != nil {
		t.Fatal(err)
	}
	beforeRecovery := store.Current()
	store.rescan(t.Context())
	if store.Current() != beforeRecovery {
		t.Fatal("recovery should not need a content rebuild")
	}
	if retained.Freshness().Root != nil {
		t.Fatal("restored original object retained a root notice")
	}
	if err := os.Rename(root, root+".old"); err != nil {
		t.Fatal(err)
	}
	writeNote(t, root, "Replacement.md", "second replacement\n")
	store.rescan(t.Context())
	var episodes []string
	for _, line := range logged() {
		if line.Message == "vault root identity changed; restart required" || line.Message == "vault root identity unconfirmed; restore access or restart" {
			episodes = append(episodes, line.Message)
		}
	}
	if diff := cmp.Diff([]string{"vault root identity changed; restart required", "vault root identity unconfirmed; restore access or restart", "vault root identity changed; restart required"}, episodes); diff != "" {
		t.Fatalf("root warning episodes (-want +got):\n%s", diff)
	}
}

func TestRootNoticeSurvivesScanRefusalBackoffAndDegradedPublication(t *testing.T) {
	t.Parallel()
	for _, outcome := range []string{"collision", "scan error", "incomplete"} {
		t.Run(outcome, func(t *testing.T) {
			t.Parallel()
			root := filepath.Join(t.TempDir(), "vault")
			writeNote(t, root, "Existing.md", "old object\n")
			reader := openReader(t, root)
			source := &recordingSource{Source: reader, reads: make(map[string]int), fail: make(map[string]int)}
			store := buildStore(t, source, discardLogger(), nil)
			retained := store.Current().Capture()
			clock := time.Now()
			store.now = func() time.Time { return clock }
			if outcome == "incomplete" {
				writeNote(t, root, "Existing.md", "changed but unreadable\n")
				source.fail["Existing.md"] = 10
				store.rescan(t.Context())
			}
			if err := os.Rename(root, root+".old"); err != nil {
				t.Fatal(err)
			}
			writeNote(t, root, "Replacement.md", "replacement\n")
			switch outcome {
			case "collision":
				store.source = &refusingSource{Source: reader, refusal: refusedForTheCollision()}
			case "scan error":
				store.source = &refusingSource{Source: reader, refusal: errors.New("refused scan")}
			}
			want := &RootNotice{SelectedPath: root, OpenedName: reader.Name()}
			store.rescan(t.Context())
			if diff := cmp.Diff(want, retained.Freshness().Root); diff != "" {
				t.Fatalf("root notice before %s fast path (-want +got):\n%s", outcome, diff)
			}
			if outcome == "incomplete" {
				for range 2 {
					clock = clock.Add(time.Minute)
					store.rescan(t.Context())
				}
				if store.Current().Freshness().Complete {
					t.Fatal("fixture never published a degraded generation")
				}
				if diff := cmp.Diff(want, store.Current().Freshness().Root); diff != "" {
					t.Fatalf("degraded publication lost root notice (-want +got):\n%s", diff)
				}
			}
			if outcome == "collision" && len(retained.Freshness().Collision) != 2 {
				t.Fatal("root notice displaced the collision")
			}
			store.source = reader
			if err := os.RemoveAll(root); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(root+".old", root); err != nil {
				t.Fatal(err)
			}
			cancelled, cancel := context.WithCancel(t.Context())
			cancel()
			store.rescan(cancelled)
			if diff := cmp.Diff(want, retained.Freshness().Root); diff != "" {
				t.Fatalf("cancelled scan altered the live root notice (-want +got):\n%s", diff)
			}
			clock = clock.Add(time.Minute)
			store.rescan(t.Context())
			fresh := retained.Freshness()
			if fresh.Root != nil || len(fresh.Collision) != 0 {
				t.Fatalf("recovery left independent notices: %+v", fresh)
			}
		})
	}
}
