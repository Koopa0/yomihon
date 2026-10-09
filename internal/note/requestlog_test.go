package note_test

import (
	"context"
	"encoding/json"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/shell"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestMissingNoteGETWritesNoLogRecord(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "readable.md"), []byte("# Readable\n\nReadable body.\n"), 0o600); err != nil {
		t.Fatalf("write readable note: %v", err)
	}
	srv, logPath := requestLogServer(t, root)
	code, body := get(t, srv.Client(), srv.URL+"/notes/readable.md")
	if code != http.StatusOK || !strings.Contains(body, "Readable body.") {
		t.Fatalf("readable GET status/body = %d/%q", code, body)
	}
	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/notes/missing.md", nil)
		rec := httptest.NewRecorder()
		srv.Config.Handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("cancelled missing-note GET status = %d, want 404", rec.Code)
		}
		t.Log("invoked: cancelled missing-note GET returned 404")
		if logged := requestLogBytes(t, logPath); len(logged) != 0 {
			t.Fatalf("caught: cancelled missing-note GET wrote %d log bytes", len(logged))
		}
		t.Log("invoked: cancelled missing-note GET returned 404 with zero log bytes")
	})
	for _, lang := range []string{"zh-Hant", "en"} {
		for _, rel := range []string{"missing.md", strings.Repeat("x", 10000) + ".md", "missing<private>.md", "e\u0301.md"} {
			name := rel
			if len(name) > 40 {
				name = "long"
			}
			t.Run(lang+"/"+name, func(t *testing.T) {
				t.Parallel()
				for range 2 {
					req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/notes/"+url.PathEscape(rel), http.NoBody)
					if err != nil {
						t.Fatalf("create missing-note GET: %v", err)
					}
					req.Header.Set("Origin", "https://unrelated.example")
					req.Header.Set("Sec-Fetch-Site", "cross-site")
					req.Header.Set("Cookie", wording.CookieName+"="+lang)
					resp, err := srv.Client().Do(req)
					if err != nil {
						t.Fatalf("missing-note GET: %v", err)
					}
					raw, readErr := io.ReadAll(resp.Body)
					closeErr := resp.Body.Close()
					if readErr != nil || closeErr != nil {
						t.Fatalf("read/close missing-note response: %v/%v", readErr, closeErr)
					}
					body := string(raw)
					if resp.StatusCode != http.StatusNotFound || !strings.Contains(body, `<html lang="`+lang+`"`) || !strings.Contains(body, html.EscapeString(req.URL.Path)) {
						t.Fatalf("caught: missing-note GET lost its localized 404/requested address: status=%d", resp.StatusCode)
					}
					t.Log("invoked: missing-note GET returned its localized 404/requested address")
					if logged := requestLogBytes(t, logPath); len(logged) != 0 {
						t.Fatalf("caught: missing-note GET wrote %d log bytes", len(logged))
					}
				}
				t.Log("invoked: missing-note GET returned 404 twice with zero log bytes")
			})
		}
	}
}

func TestUnreadableNoteGETKeepsItsWarning(t *testing.T) {
	t.Parallel()
	srv, logPath := requestLogServer(t, writeDegradedFixture(t))
	code, body := get(t, srv.Client(), srv.URL+"/notes/note-locked.md")
	if code != http.StatusNotFound || !strings.Contains(body, "檔案存在") {
		t.Fatalf("caught: unreadable-note GET lost its distinct 404: status=%d", code)
	}
	t.Log("invoked: unreadable-note GET returned its distinct 404")
	var record struct {
		Level string `json:"level"`
		Msg   string `json:"msg"`
		Path  string `json:"path"`
	}
	if err := json.Unmarshal(requestLogBytes(t, logPath), &record); err != nil {
		t.Fatalf("caught: unreadable-note GET did not write exactly one JSON record: %v", err)
	}
	if record.Level != "WARN" || record.Msg != "note captured in scan but unreadable in this generation" || record.Path != "note-locked.md" {
		t.Fatalf("caught: unreadable-note warning = %+v", record)
	}
	t.Log("invoked: unreadable-note GET retained its WARN/source path")
}

func requestLogServer(t *testing.T, root string) (srv *httptest.Server, logPath string) {
	t.Helper()
	store, source := newSnapshotStore(t, root, slog.New(slog.DiscardHandler), nil, schema.Ungoverned())
	writer := openStatusWriter(t, source, nil, schema.Ungoverned())
	file, err := os.CreateTemp(t.TempDir(), "request-*.log")
	if err != nil {
		t.Fatalf("create request log: %v", err)
	}
	logPath = file.Name()
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Errorf("close request log: %v", err)
		}
	})
	mux := http.NewServeMux()
	note.New(&note.Sources{
		Source: source, VaultName: shell.VaultName(source.Name()), Status: writer.Authority,
		Snapshot: store.Current, RequestReconcile: func() {}, ObservedStatus: writer.ObservedStatus, ConsumeReceipt: writer.ConsumeReceipt,
		Continuation: noMark, Log: slog.New(slog.NewJSONHandler(file, nil)),
	}).Register(mux)
	srv = httptest.NewServer(origin.LoopbackOnly(origin.Protect(http.NewCrossOriginProtection().Handler(mux))))
	t.Cleanup(srv.Close)
	return srv, logPath
}

func requestLogBytes(t *testing.T, path string) []byte {
	t.Helper()
	logged, err := os.ReadFile(path) // #nosec G304 -- reading only the log file this test created inside its own TempDir
	if err != nil {
		t.Fatalf("read request log: %v", err)
	}
	return logged
}
