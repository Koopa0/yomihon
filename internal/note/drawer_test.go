package note_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The rail's type drawers open to meet the page the reader is on — the journal
// drawer on a journal page, the report drawer on a report, the study-path
// drawer on a note some path has placed — and render closed everywhere else.
// The rail's shape never changes; only which drawer stands open does. Both
// directions are locked per drawer, because a drawer that is always open is
// exactly the state this replaced: the study-path drawer used to greet every
// page, including the reports, fully expanded.
func TestSidebarDrawersOpenForThePageAtHand(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	write("Writing/lessons/golang/Slices.md", "---\ntitle: Slices\ntype: lesson\ndomain: golang\nstatus: draft\n---\n\nbody\n")
	write("Writing/lessons/golang/Maps lesson.md", "---\ntitle: Maps lesson\ntype: lesson\ndomain: golang\nstatus: draft\n---\n\nbody\n")
	write("Maps/Go path.md", "---\ntitle: Go path\ntype: study-path\ndomain: golang\n---\n\n## data | Data | 資料\n\n- [[Slices]]\n- [[Planned lesson]]\n- [[Maps lesson]]\n")
	write("Diary/2026-07-31.md", "today\n")
	write("Concepts/plain.md", "---\ntitle: Plain\ntype: concept\ndomain: golang\nstatus: draft\n---\n\nprose\n")
	write("System/reports/weekly.md", "---\ntitle: Weekly\ntype: system\ndomain: meta\n---\n\nreport\n")
	write("Concepts/linked.md", "---\ntitle: Linked\ntype: concept\ndomain: golang\nstatus: draft\n---\n\nprose\n")
	write("Maps/atlas.md", "---\ntitle: Atlas\ntype: topic-map\ndomain: golang\n---\n\n## Themes\n\n- [[linked]]\n")

	srv := newServerWithContract(t, root, loadHomeContract(t))

	const (
		pathsOpen     = `<details open data-sidebar-group="paths"`
		pathsClosed   = `<details data-sidebar-group="paths"`
		journalOpen   = `<details open data-sidebar-group="journal"`
		journalShut   = `<details data-sidebar-group="journal"`
		reportsOpen   = `<details open data-sidebar-group="reports"`
		reportsClosed = `<details data-sidebar-group="reports"`
		mapsOpen      = `<details open data-sidebar-group="maps"`
		mapsClosed    = `<details data-sidebar-group="maps"`
	)
	tests := []struct {
		name string
		url  string
		want []string
		ban  []string
	}{
		{
			name: "a placed lesson opens the study-path drawer alone",
			url:  "/notes/Writing/lessons/golang/Slices.md",
			// The planned lesson between the two written ones is a warning
			// row, not a stop: the step forward from the first lesson lands
			// on the third, and the first lesson has no step back.
			want: []string{
				pathsOpen, journalShut, reportsClosed, mapsClosed,
				`下一課：Maps lesson →`,
			},
		},
		{
			name: "the closing lesson steps back across the planned row",
			url:  "/notes/Writing/lessons/golang/Maps%20lesson.md",
			want: []string{pathsOpen, `← 上一課：Slices`},
		},
		{
			name: "a journal entry opens the journal drawer alone",
			url:  "/notes/Diary/2026-07-31.md",
			want: []string{journalOpen, pathsClosed, reportsClosed, mapsClosed},
		},
		{
			name: "a report opens the report drawer alone",
			url:  "/notes/System/reports/weekly.md",
			want: []string{reportsOpen, pathsClosed, journalShut, mapsClosed},
		},
		{
			name: "an unplaced note opens none of them",
			url:  "/notes/Concepts/plain.md",
			want: []string{pathsClosed, journalShut, reportsClosed, mapsClosed},
			ban:  []string{"上一課", "下一課"},
		},
		{
			name: "a note some map places opens the map drawer alone",
			url:  "/notes/Concepts/linked.md",
			want: []string{mapsOpen, pathsClosed, journalShut, reportsClosed},
		},
		{
			name: "the map itself opens its own drawer",
			url:  "/notes/Maps/atlas.md",
			want: []string{mapsOpen, `<details open data-map-tree="Maps/atlas.md"`, pathsClosed},
		},
		{
			name: "the study path itself opens its own drawer",
			url:  "/notes/Maps/Go path.md",
			want: []string{pathsOpen, `<details open data-map-tree="Maps/Go path.md"`, mapsClosed},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code, body := get(t, srv.URL+tt.url)
			if code != http.StatusOK {
				t.Fatalf("GET %s status = %d, want 200", tt.url, code)
			}
			for _, want := range tt.want {
				if !strings.Contains(body, want) {
					t.Errorf("GET %s sidebar missing %q", tt.url, want)
				}
			}
			for _, ban := range tt.ban {
				if strings.Contains(body, ban) {
					t.Errorf("GET %s sidebar carries %q, want absent", tt.url, ban)
				}
			}
		})
	}
}

// Five trial readers asked the same question standing on a note — who cites
// this — and two of them pointed out the resolver already knew, because it
// could report the citations that were broken. The answer is on the page now,
// and the empty answer is said out loud: a note nothing cites is an island,
// which is a finding, not a blank.
func TestCitedByAnswersOnThePage(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	write("Concepts/cited.md", "---\ntitle: Cited\ntype: concept\ndomain: golang\nstatus: draft\n---\n\nthe target\n")
	write("Concepts/citer.md", "---\ntitle: Citer\ntype: concept\ndomain: golang\nstatus: draft\n---\n\nsee [[cited]]\n")
	write("Concepts/island.md", "---\ntitle: Island\ntype: concept\ndomain: golang\nstatus: draft\n---\n\nalone\n")

	srv := newServerWithContract(t, root, loadHomeContract(t))

	code, body := get(t, srv.URL+"/notes/Concepts/cited.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	cited := citedByBlock(t, body)
	if !strings.Contains(cited, `href="/notes/Concepts/citer.md"`) {
		t.Errorf("the cited note does not list what cites it; block = %q", cited)
	}

	code, body = get(t, srv.URL+"/notes/Concepts/island.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	island := citedByBlock(t, body)
	if !strings.Contains(island, "目前沒有其他筆記連到這篇") {
		t.Errorf("the island does not say it is one; block = %q", island)
	}
	// Scoped to the block on purpose: the rail lists every note in the folder,
	// so a whole-page search for the citer's href would pass whatever the
	// cited-by block said.
	if strings.Contains(island, `href="/notes/Concepts/citer.md"`) {
		t.Errorf("the island lists a citation it does not have; block = %q", island)
	}
}

// citedByBlock returns just the cited-by region of a reading page, and fails
// when it is absent — a page that stopped rendering the block entirely must
// not read as a page whose block is correctly empty.
func citedByBlock(t *testing.T, body string) string {
	t.Helper()
	const open = `<nav class="y-citedby"`
	start := strings.Index(body, open)
	if start < 0 {
		t.Fatal("the reading page has no cited-by block at all")
	}
	end := strings.Index(body[start:], "</nav>")
	if end < 0 {
		t.Fatal("the cited-by block is not closed")
	}
	return body[start : start+end]
}

// The whole-folder page gathers what the single-note pages each know. It is
// the one thing this reader was told beats keeping the editor open, and it is
// worthless if it reports a problem the note's own page would not.
func TestHealthPageGathersWhatTheNotesKnow(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	write("Concepts/broken.md", "---\ntitle: Broken\ntype: concept\ndomain: golang\nstatus: draft\n---\n\ncites [[nothing at all]] and [[A Written Note]]\n")
	write("Concepts/written.md", "---\ntitle: A Written Note\ntype: concept\ndomain: golang\nstatus: draft\n---\n\nI exist\n")
	write("Maps/ledger.md", "---\ntitle: Ledger\ntype: topic-map\ndomain: golang\n---\n\n## 缺口帳\n\n- Planned concept\n")
	write("Concepts/tracked.md", "---\ntitle: Tracked\ntype: concept\ndomain: golang\nstatus: draft\n---\n\ncites [[Planned concept]]\n")

	srv := newServerWithContract(t, root, loadHomeContract(t))

	code, body := get(t, srv.URL+"/health")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if !strings.Contains(body, "nothing at all") {
		t.Errorf("the health page omits a citation with nowhere to land; body = %q", body)
	}
	// A citation naming a written note's title is a different finding with the
	// opposite repair — an alias on that note, not a new note — and calling it
	// unwritten sends the reader to write a note that already exists.
	if !strings.Contains(body, "連結寫的是筆記的標題") || !strings.Contains(body, `href="/notes/Concepts/written.md"`) {
		t.Errorf("the health page does not separate a title-named citation from an unwritten one; body = %q", body)
	}
	unwritten := healthSectionBody(t, body, "連到還沒寫的筆記")
	if strings.Contains(unwritten, "A Written Note") {
		t.Errorf("a written note is listed as unwritten; section = %q", unwritten)
	}
	// A name the ledger declared is owed, not wrong — the same classifier the
	// note page uses, so the two faces cannot disagree about this citation.
	if strings.Contains(body, "Planned concept") {
		t.Errorf("the health page reports a planned name as a fault; body = %q", body)
	}
}

// healthSectionBody returns one section of the health page, failing when the
// section is absent — a page that stopped rendering a section must not read as
// a page whose section is correctly empty.
func healthSectionBody(t *testing.T, body, title string) string {
	t.Helper()
	start := strings.Index(body, title)
	if start < 0 {
		t.Fatalf("the health page has no %q section", title)
	}
	section, _, closed := strings.Cut(body[start:], "</section>")
	if !closed {
		t.Fatalf("the %q section is not closed", title)
	}
	return section
}

// Three trial readers clicked the breadcrumb before anything else — a teacher
// wanting a term's lessons in order, a cellist wanting her practice log in one
// list, a student wanting to read a course straight through. It read as a
// trail out of where they were, which is what a trail of folder names says it
// is, and it led nowhere.
func TestFolderPageShowsAFolderWhole(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	write("教案/國一上/第二課 空氣.md", "---\ntitle: 空氣\ntype: concept\ndomain: golang\nstatus: draft\n---\n\nbody\n")
	write("教案/國一上/第三課 水.md", "---\ntitle: 水\ntype: concept\ndomain: golang\nstatus: draft\n---\n\nbody\n")
	write("教案/研習/心得.md", "---\ntitle: 心得\ntype: concept\ndomain: golang\nstatus: draft\n---\n\nbody\n")

	srv := newServerWithContract(t, root, loadHomeContract(t))

	// The folder holding only subfolders lists them, and says how many.
	code, body := get(t, srv.URL+"/folders/%E6%95%99%E6%A1%88")
	if code != http.StatusOK {
		t.Fatalf("GET the parent folder status = %d, want 200", code)
	}
	for _, want := range []string{"國一上", "研習", "2 個資料夾"} {
		if !strings.Contains(body, want) {
			t.Errorf("the parent folder page is missing %q", want)
		}
	}

	// The folder holding files lists them in the captured order, which is the
	// lesson order the naming fix established.
	code, body = get(t, srv.URL+"/folders/%E6%95%99%E6%A1%88/%E5%9C%8B%E4%B8%80%E4%B8%8A")
	if code != http.StatusOK {
		t.Fatalf("GET the leaf folder status = %d, want 200", code)
	}
	second := strings.Index(body, "第二課 空氣")
	third := strings.Index(body, "第三課 水")
	if second < 0 || third < 0 {
		t.Fatalf("the leaf folder page does not list both lessons; body = %q", body)
	}
	if second > third {
		t.Error("the leaf folder page lists the third lesson before the second")
	}

	// A folder nothing is under is not a folder, and answers like any other
	// path the vault has nothing at.
	code, _ = get(t, srv.URL+"/folders/no-such-folder")
	if code != http.StatusNotFound {
		t.Errorf("GET a folder that does not exist status = %d, want 404", code)
	}

	// The breadcrumb on a note now leads somewhere.
	code, body = get(t, srv.URL+"/notes/%E6%95%99%E6%A1%88/%E5%9C%8B%E4%B8%80%E4%B8%8A/%E7%AC%AC%E4%BA%8C%E8%AA%B2%20%E7%A9%BA%E6%B0%A3.md")
	if code != http.StatusOK {
		t.Fatalf("GET the note status = %d, want 200", code)
	}
	// The href must be the folder the crumb names, not merely folder-shaped:
	// a crumb carrying no path still renders "/folders/" and would pass a
	// prefix check while leading the reader nowhere.
	for _, want := range []string{
		`href="/folders/%E6%95%99%E6%A1%88"`,
		`href="/folders/%E6%95%99%E6%A1%88/%E5%9C%8B%E4%B8%80%E4%B8%8A"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the note's breadcrumb does not link to %s; body = %q", want, body)
		}
	}
}

// A diarist with three years of entries met every wall this closes at once:
// no way onward from the entry she had just read, seven hundred rows of rail
// on every page with her own day far below the fold, and a cited-by block that
// said "nothing cites this" seven hundred times and was right every time.
func TestASequenceOfEntriesReadsAsALine(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for day := 1; day <= 40; day++ {
		rel := filepath.Join(root, "Diary", fmt.Sprintf("2025-08-%02d.md", day))
		if err := os.MkdirAll(filepath.Dir(rel), 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(rel, []byte("普通的一天。\n"), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	srv := newServerWithContract(t, root, loadHomeContract(t))
	// Read near the end of the folder on purpose: an entry in the first
	// twenty-four would sit inside a window that never moved, and the
	// assertion below could not tell a neighbourhood from a truncated list.
	code, body := get(t, srv.URL+"/notes/Diary/2025-08-38.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}

	// The way onward is under the prose, naming where it leads.
	for _, want := range []string{
		`href="/notes/Diary/2025-08-37.md" rel="prev"`,
		`href="/notes/Diary/2025-08-39.md" rel="next"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the entry offers no step: missing %q", want)
		}
	}

	// The rail carries a neighbourhood, not the folder, and says what it left.
	here := hereBlock(t, body)
	rows := strings.Count(here, `href="/notes/Diary/`)
	if rows > 30 {
		t.Errorf("the siblings block carries %d rows of a 40-entry folder; it is the folder again, not a neighbourhood", rows)
	}
	if !strings.Contains(here, "另外") || !strings.Contains(here, `href="/folders/Diary"`) {
		t.Errorf("the siblings block does not say what it left out or where the rest is; block = %q", here)
	}
	// The entry being read must be inside the window, or the block is a
	// neighbourhood of somewhere else.
	if !strings.Contains(here, `href="/notes/Diary/2025-08-38.md"`) {
		t.Errorf("the siblings block does not contain the entry being read; block = %q", here)
	}

	// Nothing in this folder cites anything, so the cited-by block is absent
	// rather than answering the same way on every one of forty pages.
	if strings.Contains(body, "y-citedby") {
		t.Error("a folder that uses no links still carries a cited-by block")
	}

	// The rail's tree carries the same folder, and windowing only the siblings
	// block left the tree reciting all of it — the fix was half a fix, and the
	// whole-page count is what showed that. The count covers every list on the
	// page at once, so no single one of them can quietly go back to reciting.
	if rows := strings.Count(body, `href="/notes/Diary/`); rows > 60 {
		t.Errorf("the page carries %d links into a 40-entry folder; some list is reciting the folder rather than showing a neighbourhood", rows)
	}
	// And the entry being read must survive every window on the page.
	if !strings.Contains(body, `href="/notes/Diary/2025-08-38.md"`) {
		t.Error("the page lost the entry being read from its own navigation")
	}

	// Trimming the rail took the filter's reach with it: the box passes over
	// what was rendered, so in this folder it searches a couple of dozen rows
	// while looking like it searched the folder. The server owes the runtime
	// the two facts it needs to say so — how many rows are missing, and which
	// folder they belong to — and a place to say it.
	if !strings.Contains(body, "data-rail-trimmed=") || !strings.Contains(body, "data-rail-dir=") {
		t.Error("a trimmed rail does not carry how much it left or which folder it belongs to, so the filter cannot say what it could not reach")
	}
	if !strings.Contains(body, "data-filter-partial") {
		t.Error("the rail has nowhere to say the filter's reach fell short")
	}
}

func hereBlock(t *testing.T, body string) string {
	t.Helper()
	start := strings.Index(body, `<nav class="y-here"`)
	if start < 0 {
		t.Fatal("the page has no siblings block")
	}
	block, _, closed := strings.Cut(body[start:], "</nav>")
	if !closed {
		t.Fatal("the siblings block is not closed")
	}
	return block
}
