package main

import (
	"fmt"
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

	"github.com/google/go-cmp/cmp"

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
	site, err := newReadingSite(t.Context(), root, t.TempDir(), slog.New(slog.DiscardHandler))
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
		{"the journal at this month", "/journal"},
		{"the journal at a named month", "/journal?month=2026-07"},
		// A month that is not one falls back to the month the reader is in, so
		// this answers rather than redirecting or refusing.
		{"the journal at a month that is not one", "/journal?month=banana"},
		{"one study path", "/syllabus/Maps/study.md"},
		{"one study path to listen to", "/listen/Maps/study.md"},
		{"one map", "/notes/Maps/reading.md"},
		{"one briefing", "/reports/2026-09-03.html"},
		{"one folder", "/folders/Concepts"},
		{"one note", "/notes/Concepts/alpha.md"},
		{"two notes side by side", "/compare/Concepts/alpha.md?with=Concepts/beta.md"},
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
		"Concepts/beta.md":                              "---\ntitle: Beta\ntype: concept\nstatus: draft\n---\n\n# Beta\n",
		"Diary/2026-07-10.md":                           "# A journal entry\n",
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
	site, err := newReadingSite(t.Context(), root, t.TempDir(), slog.New(slog.DiscardHandler))
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
		{target: "/", silent: []string{"paths", "maps"}},
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
				block := deskBlockMarkup(t, page, mode)
				if strings.Contains(block, "data-desk-item") {
					t.Errorf("the %s block lists rows built from a declaration that could not be read: %q", mode, block)
				}
				// "no courses" and "no courses declared" are both answers this
				// page does not have, so the slot that would carry either is
				// the one that must not be drawn.
				if strings.Contains(block, `data-nothing="shelf"`) {
					t.Errorf("the %s block says it holds none of something its declaration never described: %q", mode, block)
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

// TestEmptyProbeVaultsGuideFirstRunOverHTTP drives the two vault shapes the
// ruling names through the assembled reader: a bare empty directory, and the
// same directory carrying only the contract file.
func TestEmptyProbeVaultsGuideFirstRunOverHTTP(t *testing.T) {
	t.Parallel()

	contract, err := os.ReadFile(filepath.Join("..", "..", "internal", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read schema fixture: %v", err)
	}

	ungoverned := wording.JoinGuide(wording.IndexUngoverned, wording.IndexUngovernedNext, wording.ZhHant)
	for _, tt := range []struct {
		name     string
		files    map[string]string
		path     string
		mapState string
		folder   string
	}{
		{
			name:     "bare empty directory",
			files:    nil,
			path:     ungoverned,
			mapState: ungoverned,
			folder:   ungoverned,
		},
		{
			name: "contract only",
			files: map[string]string{
				schema.ContractRelPath: string(contract),
			},
			// The words are the loader fixture's own navigation declaration,
			// which is what this vault holds and all it holds.
			path:     fmt.Sprintf(wording.NoDeclaredTypeEmptyFmt.In(wording.ZhHant), "study-path"),
			mapState: fmt.Sprintf(wording.NoDeclaredTypesEmptyFmt.In(wording.ZhHant), "moc、source-map、topic-map"),
			folder:   wording.JoinGuide(wording.FolderIndexEmpty, wording.IndexDeclaredEmptyNext, wording.ZhHant),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			for rel, body := range tt.files {
				full := filepath.Join(root, filepath.FromSlash(rel))
				if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
					t.Fatalf("mkdir for %s: %v", rel, err)
				}
				if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
					t.Fatalf("write %s: %v", rel, err)
				}
			}
			site, err := newReadingSite(t.Context(), root, t.TempDir(), slog.New(slog.DiscardHandler))
			if err != nil {
				t.Fatalf("newReadingSite: %v", err)
			}
			t.Cleanup(func() {
				if closeErr := site.close(); closeErr != nil {
					t.Errorf("readingSite.close() error = %v", closeErr)
				}
			})

			for _, surface := range []struct {
				name  string
				fetch func() string
				want  string
			}{
				{
					name: "desk paths",
					fetch: func() string {
						return deskBlockMarkup(t, readingPage(t, site, "/"), "paths")
					},
					want: tt.path,
				},
				{
					name: "desk maps",
					fetch: func() string {
						return deskBlockMarkup(t, readingPage(t, site, "/"), "maps")
					},
					want: tt.mapState,
				},
				{
					name:  "path index",
					fetch: func() string { return readingPage(t, site, "/paths") },
					want:  tt.path,
				},
				{
					name:  "map index",
					fetch: func() string { return readingPage(t, site, "/maps") },
					want:  tt.mapState,
				},
				{
					name: "desk folders",
					fetch: func() string {
						return deskBlockMarkup(t, readingPage(t, site, "/"), "folders")
					},
					want: tt.folder,
				},
				{
					name:  "folder index",
					fetch: func() string { return readingPage(t, site, "/folders") },
					want:  tt.folder,
				},
			} {
				t.Run(surface.name, func(t *testing.T) {
					t.Parallel()
					assertEmptyGuide(t, surface.name, surface.fetch(), surface.want)
				})
			}
		})
	}
}

func assertEmptyGuide(t *testing.T, where, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("%s empty guide want %q; got %q", where, want, got)
	}
	if strings.Contains(got, "宣告") {
		t.Errorf("%s still uses 宣告 jargon: %q", where, got)
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
// artifact term gone. The write authority's own term is inseparable for the
// same reason, and measured the same way: dropping it fails none of the three
// cases. Nothing at the contract level tells those three apart; a check that
// did would have to reach past it.
//
// What is held instead is that some mode page did state a reason in every
// case, so a run where all three happened to say nothing is a failure rather
// than a pass.
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
			// A mode page states nothing where the claim it rests on is not
			// the one that failed, which is the point of the two narrow
			// contracts. What it must never mean is that this case compared
			// the desk's line against nothing at all.
			compared := 0
			for _, target := range []string{"/paths", "/maps", "/folders"} {
				causes := faultCauses(t, site, target)
				if len(causes) == 0 {
					continue
				}
				compared++
				for _, cause := range causes {
					if !slices.Contains(desk, cause) {
						t.Errorf("GET %s states %q and the desk does not; the desk states %q", target, cause, desk)
					}
				}
			}
			if compared == 0 {
				t.Error("no mode page stated a reason, so the desk's line was held against nothing")
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
//
// It reads inside the fault's own paragraph rather than taking the first
// diagnostic on the page. The desk carries others below the seam — a refused
// egress declaration, a snapshot that could not read every file — and a desk
// that had stopped stating its reasons would otherwise answer with one of
// those and be measured as though it had answered.
func faultCauses(t *testing.T, site http.Handler, target string) []string {
	t.Helper()
	_, fault, stated := strings.Cut(pageBody(t, site, target), "data-home-fault")
	if !stated {
		return nil
	}
	fault, _, ended := strings.Cut(fault, "</p>")
	if !ended {
		t.Fatalf("GET %s opens a fault and never closes it", target)
	}
	const opener = `<code class="y-diagdetail" lang="en">`
	_, rest, found := strings.Cut(fault, opener)
	if !found {
		t.Fatalf("GET %s states a fault carrying no reason", target)
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
	site, err := newReadingSite(t.Context(), root, t.TempDir(), slog.New(slog.DiscardHandler))
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

// TestEveryFaceRefusesAMissingNameTheSameWay holds the three refusals to one
// answer. A note, a report and a study path are looked up by different faces
// and each used to write the refusal out itself — the content type, the status,
// the page — so one of them could have started answering a missing name with a
// bare 200, or with no content type, and every page would still have looked
// right to anyone reading them.
//
// The body is checked too, and for the shared page's own shell rather than for
// a phrase: a reader walked off the end of the vault gets the folder tree and
// the search field, which is the whole reason the refusal is a page instead of
// a line of text.
func TestEveryFaceRefusesAMissingNameTheSameWay(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeDeskFixture(t, root)
	site, err := newReadingSite(t.Context(), root, t.TempDir(), slog.New(slog.DiscardHandler))
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
		{"a note nobody wrote", "/notes/Concepts/nothing-here.md"},
		{"a report never published", "/reports/1999-01-01.html"},
		{"a study path that is not there", "/syllabus/Maps/nothing-here.md"},
		{"a study path to listen to that is not there", "/listen/Maps/nothing-here.md"},
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
			if response.StatusCode != http.StatusNotFound {
				t.Errorf("GET %s = %d, want 404", tt.target, response.StatusCode)
			}
			if got := response.Header.Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Errorf("GET %s content type = %q, want the reading page's own", tt.target, got)
			}
			body := recorder.Body.String()
			if !strings.Contains(body, `class="y-recovery"`) {
				t.Errorf("GET %s does not answer with the shared not-found page", tt.target)
			}
			if !strings.Contains(body, `id="nav-rail"`) {
				t.Errorf("GET %s answers without the reading shell, so the reader has nowhere to go", tt.target)
			}
		})
	}
}

func TestFolderShelfScope(t *testing.T) {
	t.Parallel()

	contract, err := os.ReadFile(filepath.Join("..", "..", "internal", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read schema fixture: %v", err)
	}
	const declaration = `knowledge_dirs = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`
	if strings.Count(string(contract), declaration) != 1 {
		t.Fatal("schema fixture must carry exactly one knowledge declaration")
	}
	withDeclaration := func(line string) string {
		return strings.Replace(string(contract), declaration, line, 1)
	}

	tests := []struct {
		name        string
		contract    string
		rootFiles   bool
		wantCount   string
		wantShelf   []string
		wantPreview []string
	}{
		{
			name:        "declared knowledge layer",
			contract:    withDeclaration(`knowledge_dirs = ["Concepts"]`),
			rootFiles:   true,
			wantCount:   "3 篇",
			wantShelf:   []string{"/folders/Concepts", "/notes/Welcome.md", "/notes/Board.canvas"},
			wantPreview: []string{"/folders/Concepts", "/notes/Welcome.md", "/notes/Board.canvas"},
		},
		{
			name:        "omitted declaration",
			contract:    withDeclaration(""),
			rootFiles:   true,
			wantCount:   "5 篇",
			wantShelf:   []string{"/folders/Concepts", "/folders/System", "/folders/Outside", "/notes/Welcome.md", "/notes/Board.canvas"},
			wantPreview: []string{"/folders/Concepts", "/folders/System", "/folders/Outside"},
		},
		{
			name:        "empty declaration",
			contract:    withDeclaration(`knowledge_dirs = []`),
			rootFiles:   true,
			wantCount:   "5 篇",
			wantShelf:   []string{"/folders/Concepts", "/folders/System", "/folders/Outside", "/notes/Welcome.md", "/notes/Board.canvas"},
			wantPreview: []string{"/folders/Concepts", "/folders/System", "/folders/Outside"},
		},
		{
			name:        "missing contract",
			rootFiles:   true,
			wantCount:   "6 篇",
			wantShelf:   []string{"/folders/Concepts", "/folders/System", "/folders/Outside", "/notes/README.md", "/notes/Welcome.md", "/notes/Board.canvas"},
			wantPreview: []string{"/folders/Concepts", "/folders/System", "/folders/Outside"},
		},
		{
			name:        "malformed contract",
			contract:    "this is not toml [[[\n",
			rootFiles:   true,
			wantCount:   "6 篇",
			wantShelf:   []string{"/folders/Concepts", "/folders/System", "/folders/Outside", "/notes/README.md", "/notes/Welcome.md", "/notes/Board.canvas"},
			wantPreview: []string{"/folders/Concepts", "/folders/System", "/folders/Outside"},
		},
		{
			name:        "malformed knowledge declaration",
			contract:    withDeclaration(`knowledge_dirs = "Concepts"`),
			rootFiles:   true,
			wantCount:   "6 篇",
			wantShelf:   []string{"/folders/Concepts", "/folders/System", "/folders/Outside", "/notes/README.md", "/notes/Welcome.md", "/notes/Board.canvas"},
			wantPreview: []string{"/folders/Concepts", "/folders/System", "/folders/Outside"},
		},
		{
			name:        "declared folder has no captured files",
			contract:    withDeclaration(`knowledge_dirs = ["Missing"]`),
			rootFiles:   true,
			wantCount:   "1 篇",
			wantShelf:   []string{"/notes/Welcome.md", "/notes/Board.canvas"},
			wantPreview: []string{"/notes/Welcome.md", "/notes/Board.canvas"},
		},
		{
			name:      "declared folder has no captured files and no root files",
			contract:  withDeclaration(`knowledge_dirs = ["Missing"]`),
			wantCount: "0 篇",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			site := folderShelfSite(t, tt.contract, tt.rootFiles)
			t.Run("folder index", func(t *testing.T) {
				t.Parallel()
				page := readingPage(t, site, "/folders")
				if diff := cmp.Diff(tt.wantShelf, shelfRowHrefs(t, page, "data-index-row")); diff != "" {
					t.Errorf("GET /folders shelf hrefs mismatch (-want +got):\n%s", diff)
				}
				if want := `<div class="y-home__kicker">` + tt.wantCount + `</div>`; !strings.Contains(page, want) {
					t.Errorf("GET /folders shelf count is missing %q", want)
				}
				if tt.wantShelf == nil && !strings.Contains(page, "data-index-empty") {
					t.Error("GET /folders does not state the empty shelf")
				}
			})
			t.Run("home preview", func(t *testing.T) {
				t.Parallel()
				block := deskBlockMarkup(t, readingPage(t, site, "/"), "folders")
				if diff := cmp.Diff(tt.wantPreview, shelfRowHrefs(t, block, "data-desk-item")); diff != "" {
					t.Errorf("GET / folder preview hrefs mismatch (-want +got):\n%s", diff)
				}
				if want := "<p>" + tt.wantCount + "</p>"; !strings.Contains(block, want) {
					t.Errorf("GET / folder preview count is missing %q", want)
				}
				if tt.wantPreview == nil && !strings.Contains(block, `data-nothing="shelf"`) {
					t.Error("GET / folder preview does not state the empty shelf")
				}
			})
		})
	}
}

// TestEmptyFolderShelfDescribesOnlyItsListing keeps an empty selection from
// claiming that the outside files, still readable through their URLs, are gone.
func TestEmptyFolderShelfDescribesOnlyItsListing(t *testing.T) {
	t.Parallel()

	contract, err := os.ReadFile(filepath.Join("..", "..", "internal", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read schema fixture: %v", err)
	}
	const declaration = `knowledge_dirs = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`
	if strings.Count(string(contract), declaration) != 1 {
		t.Fatal("schema fixture must carry exactly one knowledge declaration")
	}
	site := folderShelfSite(t, strings.Replace(string(contract), declaration, `knowledge_dirs = ["Missing"]`, 1), false)

	for _, tt := range []struct {
		language   string
		wantEmpty  string
		falseEmpty string
		wantLede   string
		falseLede  string
	}{
		{
			language:   "zh-Hant",
			wantEmpty:  "這裡沒有列出檔案。",
			falseEmpty: "這個書庫裡沒有檔案。",
			wantLede:   "依檔案的存放位置瀏覽。",
			falseLede:  "照檔案實際存放的位置瀏覽整個書庫。",
		},
		{
			language:   "en",
			wantEmpty:  "No files are listed here.",
			falseEmpty: "There are no files in this vault.",
			wantLede:   "Browse files by where they are stored.",
			falseLede:  "Browse the whole vault by where its files actually sit.",
		},
	} {
		t.Run(tt.language, func(t *testing.T) {
			t.Parallel()
			localizedSite := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.Header.Set("Cookie", "yomihon_lang="+tt.language)
				site.ServeHTTP(w, r)
			})
			for _, target := range []string{"/folders", "/"} {
				t.Run(target, func(t *testing.T) {
					t.Parallel()
					page := readingPage(t, localizedSite, target)
					if target == "/" {
						page = deskBlockMarkup(t, page, "folders")
					}
					if !strings.Contains(page, tt.wantEmpty) {
						t.Errorf("GET %s empty shelf copy is missing %q", target, tt.wantEmpty)
					}
					if strings.Contains(page, tt.falseEmpty) {
						t.Errorf("GET %s claims the whole vault has no files: %q", target, tt.falseEmpty)
					}
					if target == "/folders" {
						if !strings.Contains(page, tt.wantLede) {
							t.Errorf("GET /folders folder lede is missing %q", tt.wantLede)
						}
						if strings.Contains(page, tt.falseLede) {
							t.Errorf("GET /folders folder lede claims whole-vault coverage: %q", tt.falseLede)
						}
					}
				})
			}
		})
	}
	for _, tt := range []struct {
		target string
		want   string
	}{
		{"/folders/Outside", `href="/notes/Outside/ordinary.md"`},
		{"/notes/Outside/ordinary.md", "Ordinary reading stays available."},
	} {
		t.Run(tt.target, func(t *testing.T) {
			t.Parallel()
			if page := readingPage(t, site, tt.target); !strings.Contains(page, tt.want) {
				t.Errorf("GET %s outside reading is missing %q", tt.target, tt.want)
			}
		})
	}
}

func TestFolderShelfKeepsDirectReading(t *testing.T) {
	t.Parallel()

	contract, err := os.ReadFile(filepath.Join("..", "..", "internal", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read schema fixture: %v", err)
	}
	site := folderShelfSite(t, string(contract), true)
	tests := []struct {
		target string
		want   string
	}{
		{"/folders/System", `href="/folders/System/nested"`},
		{"/folders/System/nested", `href="/notes/System/nested/guide.md"`},
		{"/notes/System/nested/guide.md", "System reading stays available."},
		{"/folders/Outside", `href="/notes/Outside/ordinary.md"`},
		{"/notes/Outside/ordinary.md", "Ordinary reading stays available."},
		{"/folders/Concepts", `href="/folders/Concepts/japanese"`},
		{"/folders/Concepts/japanese", `href="/notes/Concepts/japanese/chapter.md"`},
	}
	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			t.Parallel()
			if body := readingPage(t, site, tt.target); !strings.Contains(body, tt.want) {
				t.Errorf("GET %s body is missing %q", tt.target, tt.want)
			}
		})
	}
}

// folderShelfSite keeps System within the unfixed desk's first three rows.
// Its top level has no files, so direct reading must retain nested-only folders.
func folderShelfSite(t *testing.T, contract string, rootFiles bool) *readingSite {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"Concepts/lesson.md":           "# Shelf lesson\n",
		"Concepts/japanese/chapter.md": "# Nested shelf chapter\n",
		"System/nested/guide.md":       "# Outside system guide\n\nSystem reading stays available.\n",
		"Outside/ordinary.md":          "# Outside ordinary note\n\nOrdinary reading stays available.\n",
	}
	if contract != "" {
		files[schema.ContractRelPath] = contract
	}
	if rootFiles {
		files["README.md"] = "# Shelf README\n"
		files["Board.canvas"] = `{"nodes":[],"edges":[]}`
		files["Welcome.md"] = "# Shelf welcome\n"
	}
	for rel, body := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil { // #nosec G703 -- fixed fixture path under t.TempDir
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	site, err := newReadingSite(t.Context(), root, t.TempDir(), slog.New(slog.DiscardHandler))
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

// readingPage reads the committed response from one public GET.
func readingPage(t *testing.T, site http.Handler, target string) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	site.ServeHTTP(recorder, siteRequest(t, http.MethodGet, target, nil))
	response := recorder.Result()
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Errorf("close %s response: %v", target, err)
		}
	}()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200", target, response.StatusCode)
	}
	t.Logf("GET %s committed response = %d; Content-Type = %q", target, response.StatusCode, response.Header.Get("Content-Type"))
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read %s response: %v", target, err)
	}
	return string(body)
}

// shelfRowHrefs excludes headings, recent notes and other links around a shelf.
func shelfRowHrefs(t *testing.T, markup, marker string) []string {
	t.Helper()
	var hrefs []string
	for {
		_, rest, found := strings.Cut(markup, "<a ")
		if !found {
			return hrefs
		}
		attrs, rest, closed := strings.Cut(rest, ">")
		if !closed {
			t.Fatal("shelf contains an unclosed link")
		}
		markup = rest
		if !strings.Contains(attrs, " "+marker) {
			continue
		}
		_, href, found := strings.Cut(attrs, `href="`)
		if !found {
			t.Fatalf("shelf row has no href: %q", attrs)
		}
		href, _, closed = strings.Cut(href, `"`)
		if !closed {
			t.Fatalf("shelf row has an unclosed href: %q", attrs)
		}
		hrefs = append(hrefs, html.UnescapeString(href))
	}
}

// TestAnEmptyDeskNamesTheDeclarationThatFillsIt drives the two desks a
// declaration fills, over a vault holding nothing but a contract, in both
// languages the interface speaks. A note reaches Paths or Maps by carrying one
// of the declared types and by no other route, so the sentence a reader meets
// there has to name that type — and name the one this vault declared, which is
// why the fixture's vocabulary is words no source file in this repository
// spells. A word written into the interface would pass every check that
// compared the page against the interface's own spelling; here it fails,
// because the page is compared against the contract that was loaded.
func TestAnEmptyDeskNamesTheDeclarationThatFillsIt(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		pathType string
		mapTypes []string
		// named is every map type the sentence must carry, in the order a
		// reader meets them. Both vaults declare theirs jumbled, because a
		// listing that echoed the order the file happened to use would
		// rearrange a reader's sentence for an edit that changed no meaning.
		named     []string
		otherWord string
	}{
		{
			name: "one vocabulary", pathType: "trail-guide",
			mapTypes:  []string{"survey", "atlas", "chart"},
			named:     []string{"atlas", "chart", "survey"},
			otherWord: "walk-through",
		},
		{
			name: "another vocabulary", pathType: "walk-through",
			mapTypes:  []string{"sketch", "plan", "gazetteer"},
			named:     []string{"gazetteer", "plan", "sketch"},
			otherWord: "trail-guide",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeContractDeclaring(t, root, tt.pathType, tt.mapTypes)
			site, err := newReadingSite(t.Context(), root, t.TempDir(), slog.New(slog.DiscardHandler))
			if err != nil {
				t.Fatalf("newReadingSite: %v", err)
			}
			t.Cleanup(func() {
				if closeErr := site.close(); closeErr != nil {
					t.Errorf("readingSite.close() error = %v", closeErr)
				}
			})
			for _, desk := range []struct {
				target string
				// declared is the whole of what this desk is filled by, in the
				// order a reader is shown it.
				declared []string
			}{
				{target: "/paths", declared: []string{tt.pathType}},
				{target: "/maps", declared: tt.named},
			} {
				said := make(map[wording.Lang]string, 2)
				for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
					slot := emptySlot(t, readingPageIn(t, site, desk.target, lang))
					said[lang] = slot
					// The whole list, joined the way this interface joins one,
					// so a sentence naming the first declared type and
					// dropping the rest is not an answer.
					want := "type: " + strings.Join(desk.declared, wording.ListSeparator.In(lang))
					if !strings.Contains(slot, want) {
						t.Errorf("GET %s in %s does not name the declaration that fills it: want %q in %q", desk.target, lang, want, slot)
					}
					// The step a reader cannot complete: this vault's own
					// contract is what puts a note on these two desks, and a
					// markdown file that declares none of these types never
					// arrives however many are added.
					if step := wording.IndexDeclaredEmptyNext.In(lang); strings.Contains(slot, step) {
						t.Errorf("GET %s in %s still offers a step that leaves the desk empty: %q", desk.target, lang, slot)
					}
					// The vocabulary of this repository's own vault, and of the
					// other fixture: either one on the page is a word taken
					// from somewhere other than the contract that was loaded.
					for _, foreign := range []string{"study-path", "moc", "source-map", "topic-map", tt.otherWord} {
						if strings.Contains(slot, foreign) {
							t.Errorf("GET %s in %s names %q, which this vault does not declare: %q", desk.target, lang, foreign, slot)
						}
					}
				}
				if said[wording.ZhHant] == said[wording.En] {
					t.Errorf("GET %s says the same bytes to both readers, so one of them is reading the other's language: %q", desk.target, said[wording.En])
				}
			}
		})
	}
}

// writeContractDeclaring writes the loader fixture into root with its
// navigation vocabulary replaced, so the declared words are this test's and the
// rest of the contract stays the shape the product is exercised against.
func writeContractDeclaring(t *testing.T, root, pathType string, mapTypes []string) {
	t.Helper()
	if len(mapTypes) != 3 {
		t.Fatalf("the fixture declares three map types; got %d", len(mapTypes))
	}
	source, err := os.ReadFile(filepath.Join("..", "..", "internal", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read schema fixture: %v", err)
	}
	text := string(source)
	for i, replaced := range []string{"study-path", "moc", "source-map", "topic-map"} {
		with := pathType
		if i > 0 {
			with = mapTypes[i-1]
		}
		quoted := `"` + replaced + `"`
		if !strings.Contains(text, quoted) {
			t.Fatalf("the loader fixture no longer declares %s, so this fixture rewrites nothing", quoted)
		}
		text = strings.ReplaceAll(text, quoted, `"`+with+`"`)
	}
	full := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir for the contract: %v", err)
	}
	if err := os.WriteFile(full, []byte(text), 0o600); err != nil { // #nosec G703 -- the contract's fixed relative path under this test's own t.TempDir
		t.Fatalf("write contract: %v", err)
	}
}

// readingPageIn asks for one page as a reader who has chosen lang.
func readingPageIn(t *testing.T, site http.Handler, target string, lang wording.Lang) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := siteRequest(t, http.MethodGet, target, nil)
	// #nosec G124 -- the language cookie the server itself sets carries none of
	// those attributes, and a request that added them would be asking the
	// handler about a reader who does not exist.
	request.AddCookie(&http.Cookie{Name: wording.CookieName, Value: string(lang)})
	site.ServeHTTP(recorder, request)
	response := recorder.Result()
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Errorf("close %s response: %v", target, err)
		}
	}()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s in %s = %d, want 200", target, lang, response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read %s response: %v", target, err)
	}
	return string(body)
}

// emptySlot is the sentence a mode index draws in place of a listing. Reading
// the paragraph rather than the page keeps a word found anywhere else in the
// chrome from answering for the one sentence under test.
func emptySlot(t *testing.T, page string) string {
	t.Helper()
	_, rest, found := strings.Cut(page, "data-index-empty>")
	if !found {
		t.Fatal("the page draws no empty slot at all, so it says nothing to a reader whose shelf is empty")
	}
	slot, _, closed := strings.Cut(rest, "</p>")
	if !closed {
		t.Fatal("the empty slot is unclosed")
	}
	return html.UnescapeString(slot)
}
