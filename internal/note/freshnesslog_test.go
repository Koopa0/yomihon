package note_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/vault"
)

type freshnessLogFixture struct {
	mux       *http.ServeMux
	source    *vault.Reader
	published *snapshot.Generation
	written   bytes.Buffer
}

// newFreshnessLogFixture separates startup diagnostics from the log a poll can
// reach, so an empty response log means no requested name was disclosed.
func newFreshnessLogFixture(t *testing.T, root string) *freshnessLogFixture {
	t.Helper()
	quiet := slog.New(slog.DiscardHandler)
	governance := schema.Ungoverned()
	store, source := newSnapshotStore(t, root, quiet, nil, governance)
	writer := openStatusWriter(t, source, nil, governance)
	f := &freshnessLogFixture{
		mux:       http.NewServeMux(),
		source:    source,
		published: store.Current(),
	}
	note.New(&note.Sources{
		Source:         source,
		Status:         writer.Authority,
		Snapshot:       store.Current,
		ObservedStatus: writer.ObservedStatus,
		ConsumeReceipt: writer.ConsumeReceipt,
		Continuation:   noMark,
		Log:            slog.New(slog.NewJSONHandler(&f.written, nil)),
	}).Register(f.mux)
	return f
}

func (f *freshnessLogFixture) ask(t *testing.T, rel, want string) {
	t.Helper()
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/freshness/"+rel+"?identity="+strings.Repeat("0", 64), http.NoBody)
	f.mux.ServeHTTP(response, request)
	t.Logf("producer-hit: registered freshness request for %q", rel)
	if response.Code != http.StatusOK || response.Body.String() != want {
		t.Errorf("caught: freshness of %q = (%d, %q), want (200, %q)",
			rel, response.Code, response.Body.String(), want)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	if got := response.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/plain; charset=utf-8", got)
	}
}

func TestFreshnessNeverLogsUnpublishedLookupFailures(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFreshNote(t, root, "# Readable\n")
	f := newFreshnessLogFixture(t, root)
	paths := []string{strings.Repeat("x", 3000) + ".md", freshRel + "/x.md"}
	for _, rel := range paths {
		if _, ok := f.published.Entry(rel); ok {
			t.Fatalf("fabricated name %q exists in the generation", rel)
		}
		if _, err := f.source.Lookup(rel); err == nil || errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("Lookup(%q) error = %v, want a non-absence lookup refusal", rel, err)
		}
	}
	for range 3 {
		for _, rel := range paths {
			f.ask(t, rel, "gone")
		}
	}
	f.ask(t, "Writing/missing.md", "gone")
	if f.written.Len() != 0 {
		t.Errorf("caught: unpublished freshness names reached the log: %s", &f.written)
	}
}

func TestFreshnessStillWarnsForPublishedLookupFailures(t *testing.T) {
	t.Parallel()
	for _, oversized := range []bool{false, true} {
		name := "with body"
		if oversized {
			name = "captured entry without body"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			body := "# Watched\n"
			if oversized {
				body = strings.Repeat("x", 1024*1024+1)
			}
			writeFreshNote(t, root, body)
			f := newFreshnessLogFixture(t, root)
			if _, ok := f.published.Entry(freshRel); !ok {
				t.Fatal("the generation did not capture the watched file")
			}
			if _, ok := f.published.Note(freshRel); ok == oversized {
				t.Fatalf("body projection exists = %t, oversized = %t", ok, oversized)
			}
			folder := filepath.Join(root, "Writing")
			if err := os.Rename(folder, filepath.Join(root, "saved-writing")); err != nil {
				t.Fatalf("move captured parent: %v", err)
			}
			if err := os.WriteFile(folder, []byte("a file now occupies the parent"), 0o600); err != nil {
				t.Fatalf("replace parent with a file: %v", err)
			}
			if _, err := f.source.Lookup(freshRel); !errors.Is(err, vault.ErrNotDirectory) {
				t.Fatalf("Lookup captured name error = %v, want ErrNotDirectory", err)
			}
			f.ask(t, freshRel, "unreadable")
			var record struct {
				Level     string `json:"level"`
				Path      string `json:"path"`
				Operation string `json:"operation"`
			}
			if err := json.Unmarshal(f.written.Bytes(), &record); err != nil {
				t.Fatalf("read one freshness log record: %v; log = %s", err, &f.written)
			}
			if record.Level != "WARN" || record.Path != freshRel || record.Operation != "lookup" {
				t.Errorf("captured failure log = %+v, want WARN for %s lookup", record, freshRel)
			}
			before := f.written.String()
			f.ask(t, freshRel, "unreadable")
			if got := f.written.String(); got != before {
				t.Errorf("repeated captured failure was logged twice: %s", got)
			}
		})
	}
}

func TestFreshnessStillReadsUnpublishedArrivals(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFreshNote(t, root, "# Existing\n")
	f := newFreshnessLogFixture(t, root)
	const rel = "Writing/arrived.md"
	if _, ok := f.published.Entry(rel); ok {
		t.Fatal("the arrival was captured before it was written")
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte("# Just saved\n"), 0o600); err != nil {
		t.Fatalf("write unpublished arrival: %v", err)
	}
	if _, err := f.source.Lookup(rel); err != nil {
		t.Fatalf("Lookup unpublished arrival: %v", err)
	}
	f.ask(t, rel, "preparing")
	if f.written.Len() != 0 {
		t.Errorf("a readable unpublished arrival reached the log: %s", &f.written)
	}
}
