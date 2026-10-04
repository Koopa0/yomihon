package snapshot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

func TestRootReplacementWarnsOnceEvenWhenPinnedFilesAreUnchanged(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "vault")
	writeNote(t, root, "Notes/Existing.md", "old object\n")
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeReader(t, reader) })
	var logged bytes.Buffer
	store, err := New(t.Context(), reader, slog.New(slog.NewJSONHandler(&logged, nil)), nil, schema.Governance{})
	if err != nil {
		t.Fatal(err)
	}
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
	var warnings []map[string]string
	decoder := json.NewDecoder(&logged)
	for decoder.More() {
		var line struct {
			Message        string `json:"msg"`
			Level          string `json:"level"`
			SelectedPath   string `json:"selected_path"`
			OpenedRootName string `json:"opened_root_name"`
		}
		if err := decoder.Decode(&line); err != nil {
			t.Fatal(err)
		}
		if line.Message == "vault root identity changed; restart required" {
			warnings = append(warnings, map[string]string{
				"level":            line.Level,
				"selected_path":    line.SelectedPath,
				"opened_root_name": line.OpenedRootName,
			})
		}
	}
	want := []map[string]string{{"level": "WARN", "selected_path": root, "opened_root_name": reader.Name()}}
	if diff := cmp.Diff(want, warnings); diff != "" {
		t.Fatalf("root replacement warning (-want +got):\n%s", diff)
	}
}

func TestRootNoticeTracksRecoveryAcrossRetainedAndRebuiltCaptures(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "vault")
	writeNote(t, root, "Existing.md", "old object\n")
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeReader(t, reader) })
	log, logged := newRecordingLogger()
	store, err := New(t.Context(), reader, log, nil, schema.Governance{})
	if err != nil {
		t.Fatal(err)
	}
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
	if unknown == nil || unknown.Unconfirmed == "" {
		t.Fatal("missing selected path was reported as confirmed")
	}
	unknown.Unconfirmed = ""
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
		if line.message == "vault root identity changed; restart required" || line.message == "vault root identity unconfirmed; restore access or restart" {
			episodes = append(episodes, line.message)
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
			reader, err := vault.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { closeReader(t, reader) })
			source := &recordingSource{ObservedSource: reader, reads: make(map[string]int), fail: make(map[string]int)}
			store, err := New(t.Context(), source, discardLogger(), nil, schema.Governance{})
			if err != nil {
				t.Fatal(err)
			}
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
				store.source = &refusingSource{ObservedSource: reader, refusal: refusedForTheCollision()}
			case "scan error":
				store.source = &refusingSource{ObservedSource: reader, refusal: errors.New("refused scan")}
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
