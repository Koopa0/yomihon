package report

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/vault"
)

// TestTheShelfListsTheWrittenReportsAndNothingElse pins what the shelf at
// /reports offers: each Markdown report written directly in System/reports/,
// at its reading address. An HTML file left in the folder a retired generator
// wrote its briefings to is a vault file like any other, and the shelf neither
// lists it nor offers an address of its own for it.
func TestTheShelfListsTheWrittenReportsAndNothingElse(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for rel, body := range map[string]string{
		"System/reports/2026-09-22 review.md":       "# Weekly review\n\nThe week in the vault.\n",
		"System/reports/daily-briefing/latest.html": "<!doctype html><title>Stray briefing</title>\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	source, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open(%q) error: %v", root, err)
	}
	t.Cleanup(func() {
		if closeErr := source.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error: %v", closeErr)
		}
	})
	store, err := snapshot.New(t.Context(), source, slog.New(slog.DiscardHandler), nil, schema.Ungoverned())
	if err != nil {
		t.Fatalf("snapshot.New(%q) error: %v", root, err)
	}

	calls := 0
	mux := http.NewServeMux()
	New(func() *snapshot.Generation {
		calls++
		return store.Current()
	}, slog.New(slog.DiscardHandler)).Register(mux)

	for _, tt := range []struct {
		target string
		want   int
	}{
		{target: "/reports", want: http.StatusOK},
		{target: "/reports/latest.html", want: http.StatusNotFound},
		{target: "/reports/latest.html/raw", want: http.StatusNotFound},
	} {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.target, http.NoBody))
		if rr.Code != tt.want {
			t.Errorf("GET %s = %d, want %d", tt.target, rr.Code, tt.want)
		}
		if tt.want != http.StatusOK {
			continue
		}
		body := rr.Body.String()
		if want := `href="/notes/System/reports/2026-09-22%20review.md"`; !strings.Contains(body, want) {
			t.Errorf("the shelf does not offer the written report at %s; body = %q", want, body)
		}
		for _, stray := range []string{"Stray briefing", "latest.html", "daily-briefing"} {
			if strings.Contains(body, stray) {
				t.Errorf("the shelf mentions %q, which is not a report", stray)
			}
		}
	}
	if calls != 1 {
		t.Errorf("snapshot reads for one shelf request = %d, want 1", calls)
	}
}
