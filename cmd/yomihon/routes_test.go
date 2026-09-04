package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestEveryReadingAddressAnswers drives the production composition over every
// address a reader can reach from the desk. Route patterns are spread across
// four packages and a mode index sits beside a subtree pattern that already
// owns its prefix ("/reports" beside "/reports/{name}", "/folders" beside
// "/folders/{path...}"), which is exactly the shape that yields a redirect or a
// not-found page instead of a page while every package's own tests stay green.
// One table over the assembled mux is the only thing that sees that.
func TestEveryReadingAddressAnswers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeDeskFixture(t, root)
	site, err := newReadingSite(t.Context(), root, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newReadingSite: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Errorf("readingSite.close() error = %v", closeErr)
		}
	})

	tests := []struct {
		name   string
		target string
	}{
		{"the desk", "/"},
		{"the study-path index", "/paths"},
		{"the map index", "/maps"},
		{"the report index", "/reports"},
		{"the folder index", "/folders"},
		{"one study path", "/syllabus/Maps/study.md"},
		{"one map", "/notes/Maps/reading.md"},
		{"one briefing", "/reports/2026-09-03.html"},
		{"one folder", "/folders/Concepts"},
		{"one note", "/notes/Concepts/alpha.md"},
		{"the health page", "/health"},
		{"search", "/search"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			site.ServeHTTP(recorder, siteRequest(t, http.MethodGet, tt.target, nil))
			response := recorder.Result()
			defer func() {
				if closeErr := response.Body.Close(); closeErr != nil {
					t.Errorf("close response body: %v", closeErr)
				}
			}()
			if response.StatusCode != http.StatusOK {
				t.Errorf("GET %s = %d %s, want 200", tt.target, response.StatusCode, response.Header.Get("Location"))
			}
		})
	}
}

// writeDeskFixture writes the smallest vault that holds one of everything the
// desk offers: a contract, a study path, a general map, a note under a
// lifecycle folder, a written report and a briefing.
func writeDeskFixture(t *testing.T, root string) {
	t.Helper()

	contract, err := os.ReadFile(filepath.Join("..", "..", "internal", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read schema fixture: %v", err)
	}
	files := map[string]string{
		schema.ContractRelPath:                          string(contract),
		"Maps/study.md":                                 "---\ntitle: Study Path\ntype: study-path\n---\n\n# Study Path\n\n## Part One\n\n- [[alpha]]\n",
		"Maps/reading.md":                               "---\ntitle: Reading Map\ntype: topic-map\n---\n\n# Reading Map\n\n## Branch\n\n- [[alpha]]\n",
		"Concepts/alpha.md":                             "---\ntitle: Alpha\ntype: concept\nstatus: draft\n---\n\n# Alpha\n",
		"System/reports/2026-09-02.md":                  "# Written report\n",
		"System/reports/daily-briefing/2026-09-03.html": "<p>briefing</p>\n",
	}
	for rel, body := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err = os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err = os.WriteFile(full, []byte(body), 0o600); err != nil { // #nosec G703 -- fixed fixture path under t.TempDir
			t.Fatalf("write %s: %v", rel, err)
		}
	}
}

// TestAWithheldDeclarationIsStatedOnTheModeIndexes closes the hole the desk
// opens. While paths and maps lived only in the rail, one lock watched them
// disappear when a contract could not be read. Giving each mode a page of its
// own is a second route to the same projection, and a page that quietly listed
// nothing would leave a reader believing the vault declares no courses when the
// truth is that its declaration could not be honoured.
func TestAWithheldDeclarationIsStatedOnTheModeIndexes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeDeskFixture(t, root)
	broken := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	if err := os.WriteFile(broken, []byte("this is not toml [[[\n"), 0o600); err != nil { // #nosec G703 -- fixed fixture path under t.TempDir
		t.Fatalf("break the contract: %v", err)
	}
	site, err := newReadingSite(t.Context(), root, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newReadingSite: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Errorf("readingSite.close() error = %v", closeErr)
		}
	})

	// The desk carries the same two projections in its blocks, so it can tell
	// the same untruth in its own markup. It says nothing there about how much
	// it holds and nothing about holding none; the fault above the blocks is
	// the whole answer.
	tests := []struct {
		target string
		absent []string
	}{
		{target: "/paths", absent: []string{"data-index-row", "data-index-empty"}},
		{target: "/maps", absent: []string{"data-index-row", "data-index-empty"}},
		{target: "/", absent: []string{
			wording.PathIndexEmpty.In(wording.ZhHant),
			wording.MapIndexEmpty.In(wording.ZhHant),
		}},
	}
	for _, tt := range tests {
		target := tt.target
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			site.ServeHTTP(recorder, siteRequest(t, http.MethodGet, target, nil))
			response := recorder.Result()
			defer func() {
				if closeErr := response.Body.Close(); closeErr != nil {
					t.Errorf("close response body: %v", closeErr)
				}
			}()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("read response body: %v", err)
			}
			html := string(body)
			if !strings.Contains(html, "data-home-fault") {
				t.Errorf("GET %s says nothing about the declaration it could not read; body = %q", target, html)
			}
			for _, absent := range tt.absent {
				if strings.Contains(html, absent) {
					t.Errorf("GET %s carries %q, which speaks for a declaration that could not be read", target, absent)
				}
			}
		})
	}
}
