package main

import (
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
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
		// silent names the desk blocks that must list nothing. The reports and
		// the folders are listings no declaration can close, so their blocks
		// fill in every contract state and the desk as a whole is never
		// rowless; only the two a declaration gates can be asked for silence.
		//
		// This stands for the property rather than for the branch that keeps
		// it. Every contract state that closes the declaration also empties the
		// projections behind those two blocks, so the guard that skips building
		// them cannot be caught by removing it: measured on a contract that
		// cannot be parsed and on one whose artifact policy alone is rejected,
		// the model answers with no courses and no maps either way.
		silent []string
	}{
		{target: "/paths", absent: []string{"data-index-row", "data-index-empty"}},
		{target: "/maps", absent: []string{"data-index-row", "data-index-empty"}},
		{target: "/", absent: []string{
			wording.PathIndexEmpty.In(wording.ZhHant),
			wording.MapIndexEmpty.In(wording.ZhHant),
		}, silent: []string{"paths", "maps"}},
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
			page := string(body)
			if !strings.Contains(page, "data-home-fault") {
				t.Errorf("GET %s says nothing about the declaration it could not read; body = %q", target, page)
			}
			for _, absent := range tt.absent {
				if strings.Contains(page, absent) {
					t.Errorf("GET %s carries %q, which speaks for a declaration that could not be read", target, absent)
				}
			}
			for _, mode := range tt.silent {
				if block := deskBlockMarkup(t, page, mode); strings.Contains(block, "data-desk-item") {
					t.Errorf("the %s block lists rows built from a declaration that could not be read: %q", mode, block)
				}
			}
			if len(tt.silent) > 0 && !strings.Contains(deskBlockMarkup(t, page, "folders"), "data-desk-item") {
				// A slice that reached no markup would pass the checks above in
				// silence, so one block that must carry a row is read the same
				// way and required to.
				t.Fatal("the folders block lists nothing either, so the checks above cannot tell silence from a bad slice")
			}
		})
	}
}

// deskBlockMarkup slices one way in out of the desk, from its own marker to the
// end of the section it opens.
func deskBlockMarkup(t *testing.T, page, mode string) string {
	t.Helper()
	_, rest, found := strings.Cut(page, `data-home-block="`+mode+`"`)
	if !found {
		t.Fatalf("the desk carries no %s block", mode)
	}
	block, _, closed := strings.Cut(rest, "</section>")
	if !closed {
		t.Fatalf("the %s block is never closed", mode)
	}
	return block
}

// TestTheDeskStatesWhatEveryModePageStates holds the desk to the rule the fault
// line is built on: a page states the reasons that could empty something it
// draws, and the desk, which draws all four ways in at once, states the union
// of what those four pages state. A mode added later whose reason never reached
// the desk would empty a block there and never say why, which is the shape of
// the fault this locks against.
//
// One broken contract cannot show that. A contract that will not parse fails
// every claim with one sentence, so all four pages state the same cause and
// dropping any one of them from the desk leaves the others supplying the same
// words. The two contracts below each parse and then fail one claim alone.
//
// That separates the navigation declaration, and it is measured: dropping it
// from the desk's line fails the third case here and nothing else. It does not
// separate the artifact policy, and that is measured too — a contract whose
// artifact section is unusable is a contract the write authority rejects as
// well, in the same words, so the desk keeps saying the right thing with the
// artifact term gone. Nothing at the contract level tells those two apart; a
// check that did would have to reach past it.
func TestTheDeskStatesWhatEveryModePageStates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		contract func(string) string
	}{
		{
			name:     "a contract that will not parse",
			contract: func(string) string { return "this is not toml [[[\n" },
		},
		{
			// The artifact policy alone: the section is there and the key it
			// must carry is not.
			name: "an artifact policy missing its required key",
			contract: func(contract string) string {
				return strings.Replace(contract, "non_instance_dirs = [\"System/templates\"]\n", "", 1)
			},
		},
		{
			// The navigation roles alone: one type declared as both a path and
			// a map, which the roles reader rejects and nothing else reads.
			name: "a type declared as both a path and a map",
			contract: func(contract string) string {
				return strings.Replace(contract,
					`map_types = ["moc", "source-map", "topic-map"]`,
					`map_types = ["moc", "source-map", "topic-map", "study-path"]`, 1)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			site := siteOverAContract(t, tt.contract)
			desk := faultCauses(t, site, "/")
			if len(desk) == 0 {
				t.Fatal("the desk states no reason, so this compares nothing")
			}
			for _, target := range []string{"/paths", "/maps", "/folders"} {
				causes := faultCauses(t, site, target)
				if len(causes) == 0 {
					continue
				}
				for _, cause := range causes {
					if !slices.Contains(desk, cause) {
						t.Errorf("GET %s states %q and the desk does not; the desk states %q", target, cause, desk)
					}
				}
			}
			// The reports are a listing of one directory. No declaration can
			// close it, so an empty one is the answer rather than a withheld
			// projection, and a reason printed there would be about some other
			// page.
			if strings.Contains(pageBody(t, site, "/reports"), "data-home-fault") {
				t.Error("GET /reports states a reason for something no declaration can withhold")
			}
		})
	}
}

// faultCauses reads the reasons one page states, as the page states them: the
// detail the browser diagnostic carries, split back into the causes joined into
// it. A page with no fault yields none.
func faultCauses(t *testing.T, site http.Handler, target string) []string {
	t.Helper()
	const opener = `<code class="y-diagdetail" lang="en">`
	_, rest, found := strings.Cut(pageBody(t, site, target), opener)
	if !found {
		return nil
	}
	detail, _, closed := strings.Cut(rest, "</code>")
	if !closed {
		t.Fatalf("GET %s opens a diagnostic detail and never closes it", target)
	}
	// The detail reaches the page escaped, and one of the entities it is
	// written with ends in the same byte the causes are joined on, so the text
	// has to come back out of its markup before it can be split into them.
	return strings.Split(html.UnescapeString(detail), "; ")
}

// pageBody renders one page of the running site.
func pageBody(t *testing.T, site http.Handler, target string) string {
	t.Helper()
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
		t.Fatalf("read %s: %v", target, err)
	}
	return string(body)
}

// siteOverAContract is the desk fixture served over a contract the caller has
// had its way with, so a check can name the one claim it wants failed.
func siteOverAContract(t *testing.T, mutate func(string) string) *readingSite {
	t.Helper()
	root := t.TempDir()
	writeDeskFixture(t, root)
	contractPath := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	original, err := os.ReadFile(contractPath) // #nosec G304 -- a fixture path under t.TempDir
	if err != nil {
		t.Fatalf("read the fixture contract: %v", err)
	}
	written := mutate(string(original))
	if written == string(original) {
		t.Fatal("the contract came back unchanged, so this fixture fails no claim at all")
	}
	if err = os.WriteFile(contractPath, []byte(written), 0o600); err != nil { // #nosec G703 -- a fixture path under t.TempDir
		t.Fatalf("write the contract: %v", err)
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
	return site
}
