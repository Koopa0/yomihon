package note_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	write("Maps/Go path.md", "---\ntitle: Go path\ntype: study-path\ndomain: golang\n---\n\n## data | Data | 資料 {sequence=primary}\n\n- [[Slices]]\n- [[Planned lesson]]\n- [[Maps lesson]]\n")
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
			code, body := get(t, srv.Client(), srv.URL+tt.url)
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

	code, body := get(t, srv.Client(), srv.URL+"/notes/Concepts/cited.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	cited := citedByBlock(t, body)
	if !strings.Contains(cited, `href="/notes/Concepts/citer.md"`) {
		t.Errorf("the cited note does not list what cites it; block = %q", cited)
	}

	code, body = get(t, srv.Client(), srv.URL+"/notes/Concepts/island.md")
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

	code, body := get(t, srv.Client(), srv.URL+"/health")
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
	unwritten := healthSectionBody(t, body, "連到不存在的目標")
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
	code, body := get(t, srv.Client(), srv.URL+"/folders/%E6%95%99%E6%A1%88")
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
	code, body = get(t, srv.Client(), srv.URL+"/folders/%E6%95%99%E6%A1%88/%E5%9C%8B%E4%B8%80%E4%B8%8A")
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
	code, _ = get(t, srv.Client(), srv.URL+"/folders/no-such-folder")
	if code != http.StatusNotFound {
		t.Errorf("GET a folder that does not exist status = %d, want 404", code)
	}

	// The breadcrumb on a note now leads somewhere.
	code, body = get(t, srv.Client(), srv.URL+"/notes/%E6%95%99%E6%A1%88/%E5%9C%8B%E4%B8%80%E4%B8%8A/%E7%AC%AC%E4%BA%8C%E8%AA%B2%20%E7%A9%BA%E6%B0%A3.md")
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
	code, body := get(t, srv.Client(), srv.URL+"/notes/Diary/2025-08-38.md")
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

// A fresh clone stamps every file with the checkout moment, so the times that
// order the landing block separate nothing and its tie-break — path order —
// decides. The heading then promises recency and leads with whatever name
// sorts first. A doctor met this on his own vault and read it correctly: four
// notes, one timestamp, no ordering.
func TestHomeSaysWhenItsTimesCannotOrderAnything(t *testing.T) {
	t.Parallel()

	stamp := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	write := func(t *testing.T, root, rel string, at time.Time) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		body := "---\ntitle: " + filepath.Base(rel) + "\ntype: concept\ndomain: golang\nstatus: draft\n---\n\nbody\n"
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		if err := os.Chtimes(full, at, at); err != nil {
			t.Fatalf("chtimes: %v", err)
		}
	}

	t.Run("one shared timestamp is not recency", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		for _, name := range []string{"a.md", "b.md", "c.md"} {
			write(t, root, "Concepts/"+name, stamp)
		}
		srv := newServerWithContract(t, root, loadHomeContract(t))
		code, body := get(t, srv.Client(), srv.URL+"/folders")
		if code != http.StatusOK {
			t.Fatalf("status = %d, want 200", code)
		}
		if strings.Contains(body, "最近改動過的筆記") {
			t.Error("Home promises recency over notes its timestamps cannot order")
		}
		if !strings.Contains(body, "排不出先後") {
			t.Errorf("Home does not say its timestamps separate nothing; body = %q", body)
		}
	})

	t.Run("timestamps that differ are recency", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		for i, name := range []string{"a.md", "b.md", "c.md"} {
			write(t, root, "Concepts/"+name, stamp.AddDate(0, 0, i))
		}
		srv := newServerWithContract(t, root, loadHomeContract(t))
		code, body := get(t, srv.Client(), srv.URL+"/folders")
		if code != http.StatusOK {
			t.Fatalf("status = %d, want 200", code)
		}
		if !strings.Contains(body, "最近改動過的筆記") {
			t.Errorf("Home hedges on notes its timestamps do order; body = %q", body)
		}
	})
}

// What links here has to survive the widths where the right rail is gone. It
// did not: the narrow projection carried the contents and the diagnostics and
// nothing else, and a note whose only aid was its backlinks lost the rail
// entirely — so at every width, wide included, the answer disappeared for
// exactly the notes where it was the whole of what the page had to add.
func TestCitedBySurvivesWithoutTheRail(t *testing.T) {
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
	// No headings anywhere, so neither note has a table of contents and the
	// backlink is the only reading aid the page can offer.
	write("Concepts/target.md", "---\ntitle: Target\ntype: concept\ndomain: golang\nstatus: draft\n---\n\nplain prose\n")
	write("Concepts/source.md", "---\ntitle: Source\ntype: concept\ndomain: golang\nstatus: draft\n---\n\nsee [[target]]\n")

	srv := newServerWithContract(t, root, loadHomeContract(t))
	code, body := get(t, srv.Client(), srv.URL+"/notes/Concepts/target.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}

	// The control: the block exists at all, and names the note that cites this
	// one — otherwise the projection assertion below could pass over an empty
	// page.
	if !strings.Contains(citedByBlock(t, body), `href="/notes/Concepts/source.md"`) {
		t.Fatalf("the cited-by block does not name the citing note; body = %q", body)
	}
	if strings.Contains(body, "y-shell--rail-empty") {
		t.Error("the rail is called empty while it holds the only aid this note has")
	}
	aids := strings.Index(body, `class="y-inlineaids"`)
	if aids < 0 {
		t.Fatalf("the narrow projection is absent; body = %q", body)
	}
	end := strings.Index(body[aids:], "</main>")
	if end < 0 {
		t.Fatal("the narrow projection is not followed by the end of main")
	}
	if !strings.Contains(body[aids:aids+end], "連到這篇") {
		t.Errorf("the narrow projection drops what links here; block = %q", body[aids:aids+end])
	}
}

// TestTheArrowWalksTheCourseThatTeachesTheNote asserts the step at the foot of
// a lesson follows the order its course declares, and says so.
//
// The foot of the article was built for a folder of dated entries, where the
// folder's own order is the line the reader is walking. A course is not that:
// it declares its order in a syllabus, printed in the rail of the very same
// page, and the folder's alphabetical order agrees with it only by luck. One
// course whose lessons happened to sort into teaching order taught the reader
// to trust the arrow; the next course sent them somewhere else entirely.
//
// The fixture is built so the two orders disagree: alphabetically Basics comes
// before Setup, while the syllabus teaches Setup first.
func TestTheArrowWalksTheCourseThatTeachesTheNote(t *testing.T) {
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
	lesson := func(title string) string {
		return "---\ntitle: " + title + "\ntype: lesson\ndomain: golang\nstatus: draft\n---\n\nbody\n"
	}
	write("Writing/lessons/golang/Setup.md", lesson("Setup"))
	write("Writing/lessons/golang/Basics.md", lesson("Basics"))
	write("Writing/lessons/golang/Wrapping up.md", lesson("Wrapping up"))
	write("Maps/Go course.md", "---\ntitle: Go course\ntype: study-path\ndomain: golang\n---\n\n## start | Start | 開始 {sequence=primary}\n\n- [[Setup]]\n- [[Basics]]\n- [[Wrapping up]]\n")

	srv := newServerWithContract(t, root, loadHomeContract(t))

	code, body := get(t, srv.Client(), srv.URL+"/notes/Writing/lessons/golang/Setup.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	// Basics is what the course teaches next. It is also what the folder puts
	// before Setup, so an arrow reading the folder would have offered it as the
	// step back instead.
	if want := `href="/notes/Writing/lessons/golang/Basics.md" rel="next"`; !strings.Contains(body, want) {
		t.Errorf("the first lesson does not step forward through its course: missing %q", want)
	}
	if unwanted := `href="/notes/Writing/lessons/golang/Basics.md" rel="prev"`; strings.Contains(body, unwanted) {
		t.Errorf("the first lesson steps back into the folder's order: found %q", unwanted)
	}
	// The course begins here, so there is nowhere behind — even though the
	// folder holds a file that sorts earlier.
	if strings.Contains(body, `rel="prev"`) {
		t.Errorf("the first lesson of the course offers a step back")
	}
	// The order's name is printed for every reader, not spoken only to
	// assistive technology, and each link says it hands over a lesson.
	for _, want := range []string{
		`aria-label="Go course 課程順序"`,
		`<p class="y-steps__source">Go course 課程順序</p>`,
		`<span class="y-steps__role">下一課</span>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the step does not name the order it walks: missing %q", want)
		}
	}

	// A note no course teaches keeps the folder, and keeps saying so — even
	// when the author wrote their own recommendation into the prose. The foot
	// still calls its links folder adjacency, never a next lesson: the folder's
	// neighbour is not the author's suggested step, and the words must not
	// claim otherwise.
	write("Diary/2026-08-01.md", "today\n\n下一步：[[2026-08-02課程筆記]]\n")
	// The neighbouring file's displayed name legitimately contains 課 — an
	// author may call a note 課程筆記, and the folder line shows file names —
	// so the guard below must refuse the course words only in the foot's
	// structured chrome, never inside a note's own name.
	write("Diary/2026-08-02課程筆記.md", "tomorrow\n")
	srv2 := newServerWithContract(t, root, loadHomeContract(t))
	code, body = get(t, srv2.Client(), srv2.URL+"/notes/Diary/2026-08-01.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	for _, want := range []string{
		`href="/notes/Diary/2026-08-02%E8%AA%B2%E7%A8%8B%E7%AD%86%E8%A8%98.md" rel="next"`,
		`<p class="y-steps__source">同資料夾的前後檔案</p>`,
		`<span class="y-steps__role">下一份</span>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("an entry no course teaches lost its folder line: missing %q", want)
		}
	}
	// The refusal is scoped to the foot's structured chrome — the step-role
	// words and the course's source line, matched with their markup — because
	// a neighbouring file's own name may honestly say 課, and a folder note
	// called 課程筆記 must not read as the footer borrowing course words.
	foot := stepsBlock(t, body)
	for _, forbidden := range []string{
		`<span class="y-steps__role">上一課</span>`,
		`<span class="y-steps__role">下一課</span>`,
		`<p class="y-steps__source">Go course 課程順序</p>`,
		`aria-label="Go course 課程順序"`,
	} {
		if strings.Contains(foot, forbidden) {
			t.Errorf("a folder foot borrows the course's chrome %q; foot = %q", forbidden, foot)
		}
	}
}

// stepsBlock cuts the article-foot navigation out of a page, so an assertion
// about its words cannot pass on the same words appearing elsewhere on the
// page — the rail prints course steps too.
func stepsBlock(t *testing.T, body string) string {
	t.Helper()
	start := strings.Index(body, `<nav class="y-steps"`)
	if start < 0 {
		t.Fatal("the page has no article-foot steps")
	}
	block, _, closed := strings.Cut(body[start:], "</nav>")
	if !closed {
		t.Fatal("the article-foot steps are not closed")
	}
	return block
}
