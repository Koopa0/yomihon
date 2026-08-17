package pages

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/vault"
)

func TestSyllabusUsesTheBranchTreeAsItsHeadingOutline(t *testing.T) {
	t.Parallel()

	view := PathView{
		Title: "Path",
		Branches: []PathBranchView{{
			Anchor: "part-1", Ordinal: "I", Heading: "Part", Depth: 0,
			Items: []PathItemView{{Branch: &PathBranchView{
				Num: 1, Heading: "Module", Depth: 1,
				Items: []PathItemView{{Branch: &PathBranchView{
					Num: 2, Heading: "Unit", Depth: 2,
					Items: []PathItemView{{Branch: &PathBranchView{Num: 3, Heading: "Topic", Depth: 3}}},
				}}},
			}}},
		}},
	}

	var out bytes.Buffer
	if err := Syllabus(view, layouts.Chrome{}).Render(t.Context(), &out); err != nil {
		t.Fatalf("render syllabus: %v", err)
	}
	html := out.String()
	for _, want := range []string{
		`<h1 class="y-title">Path</h1>`,
		`<h2 class="y-part" id="part-1">`,
		`<h3 class="y-module__label">`,
		`<h4 class="y-module__label">`,
		`<h5 class="y-module__label">`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("syllabus heading outline missing %q; html = %q", want, html)
		}
	}
	if strings.Contains(html, `<div class="y-part"`) || strings.Contains(html, `<div class="y-module__label"`) {
		t.Errorf("syllabus renders visual-only branch headings; html = %q", html)
	}
}

// TestBuildPathView pins the pure transform's contract: a branch's total counts
// linked and warning rows, Ready counts only sealed resolved entries, and
// document order is preserved at every level. The fixture is two parts, one
// with a module and one with entries attached directly (no module), so the
// tallies and the modules count are hand-derivable and non-tautological.
func TestBuildPathView(t *testing.T) {
	t.Parallel()

	// The course is written the way an author writes one and read through the
	// declared-sequence grammar, so the page is asserted against the same
	// interpretation navigation uses — not a hand-built tree that could agree
	// with a broken reading.
	body := "## Data {sequence=primary}\n\n" +
		"### Text {sequence=primary}\n\n" +
		"- [[Slices]]\n" +
		"- [[Arrays]]\n" +
		"\n## Memory {sequence=primary}\n\n" +
		"- [[GC]]\n" +
		"\t- 選修 {sequence=local}\n" +
		"\t\t- [[Tuning]]\n" +
		"- [[Template]]\n" +
		"- [[Unwritten]]\n"
	path := buildTestPath(t, body)

	got := BuildPathView(&path, []nav.Path{path})

	want := PathView{
		Title:      "Go path",
		RelPath:    "Maps/Go path.md",
		GuideHref:  "/notes/Maps/Go%20path.md",
		SealTarget: schema.SealStatus,
		Paths: []PathLink{
			// The side branch is not part of the main line, so the planned
			// total is five, not six.
			{Title: "Go path", RelPath: "Maps/Go path.md", Entries: 5, Active: true},
		},
		Parts:   2,
		Modules: 2, // the "Text" module under Data, and the side branch under GC
		Entries: 5,
		Ready:   2, // Slices + GC sit at the seal; Arrays is draft
		Branches: []PathBranchView{
			{
				Anchor: "part-1", Ordinal: "I", Num: 1, Heading: "Data", Depth: 0,
				// The part holds no rows of its own; its count is the module's,
				// so the part heading and Home agree.
				Total: 2,
				Items: []PathItemView{{Branch: &PathBranchView{
					Num: 1, Heading: "Text", Depth: 1, Total: 2,
					Items: []PathItemView{
						{Entry: &PathEntryView{Text: "Slices", Href: "/notes/Writing/Slices.md", Status: schema.SealStatus, Sealed: true, Number: 1}},
						{Entry: &PathEntryView{Text: "Arrays", Href: "/notes/Writing/Arrays.md", Status: "draft", Number: 2}},
					},
				}}},
			},
			{
				Anchor: "part-2", Ordinal: "II", Num: 2, Heading: "Memory", Depth: 0,
				Total: 3,
				Items: []PathItemView{
					// The main line's numbering continues from the first part:
					// the course has one declared order across its parts.
					{Entry: &PathEntryView{Text: "GC", Href: "/notes/Writing/GC.md", Status: schema.SealStatus, Sealed: true, Number: 3}},
					// The side branch is drawn where the author put it: under
					// the lesson it hangs from, before the next main lesson —
					// and it numbers its own rows from one, never sharing the
					// main line's count.
					{Branch: &PathBranchView{
						Num: 1, Heading: "選修", Depth: 1, Local: true, Total: 1,
						Items: []PathItemView{
							{Entry: &PathEntryView{Text: "Tuning", Href: "/notes/Writing/Tuning.md", Status: "draft", Number: 1}},
						},
					}},
					// Warning rows keep their place and their number: a planned
					// lesson is still one of the course's lessons.
					{Entry: &PathEntryView{Text: "Template", Kind: nav.EntryNonInstance, Number: 4}},
					{Entry: &PathEntryView{Text: "Unwritten", Kind: nav.EntryUnresolved, Number: 5}},
				},
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("BuildPathView() mismatch (-want +got):\n%s", diff)
	}
}

// TestLessonsAreAnOrderedList locks the course page's list semantics: an
// author-declared run of lessons is a native ordered list whose item values
// are the walk's own numbering, not look-alike sibling links whose order lives
// only in the pixels. It would catch a revert to a neutral container, a main
// line whose numbering restarts at each part, a side branch borrowing the main
// line's count, a warning row losing its place or number, and a branch
// declared out of the course leaking back in.
func TestLessonsAreAnOrderedList(t *testing.T) {
	t.Parallel()

	body := "## Data {sequence=primary}\n\n" +
		"### Text {sequence=primary}\n\n" +
		"- [[Slices]]\n" +
		"- [[Arrays]]\n" +
		"\n## Memory {sequence=primary}\n\n" +
		"- [[GC]]\n" +
		"\t- 選修 {sequence=local}\n" +
		"\t\t- [[Tuning]]\n" +
		"- [[Template]]\n" +
		"- [[Unwritten]]\n" +
		"\n## 日常 {sequence=none}\n\n" +
		"- [[Routine]]\n"
	// Routine is a real, resolvable lesson: its absence below is then the none
	// declaration at work, not a resolution failure.
	path := buildTestPath(t, body, map[string]string{
		"Writing/Routine.md": "---\ntitle: Routine\ntype: lesson\nstatus: draft\n---\nbody\n",
	})
	view := BuildPathView(&path, []nav.Path{path})

	if view.Entries != 5 {
		t.Errorf("BuildPathView() Entries = %d, want 5: a none block adds nothing and a planned row still counts", view.Entries)
	}

	var out bytes.Buffer
	if err := Syllabus(view, layouts.Chrome{}).Render(t.Context(), &out); err != nil {
		t.Fatalf("render syllabus: %v", err)
	}
	html := out.String()

	if got := strings.Count(html, `<ol class="y-lessons"`); got != 4 {
		t.Errorf("the course renders %d ordered lists, want 4: one per uninterrupted run of lessons", got)
	}
	for _, want := range []string{
		// Every fragment says which component it belongs to: the first
		// main-line list opens the order, later ones resume it, and a side
		// branch is named as one.
		`<ol class="y-lessons" aria-label="主線">`,
		`<ol class="y-lessons" aria-label="支線：選修">`,
		// The main line numbers on across parts: one course, one order.
		`<li value="1"><a class="y-lesson" href="/notes/Writing/Slices.md"`,
		`<li value="3"><a class="y-lesson" href="/notes/Writing/GC.md"`,
		// The side branch counts its own rows from one.
		`<li value="1"><a class="y-lesson" href="/notes/Writing/Tuning.md"`,
		// Warning rows keep their sequence position, their number, and their
		// non-interactive form.
		`<li value="4"><span class="y-lesson y-lesson--broken" data-resolution="non-instance"`,
		`<li value="5"><span class="y-lesson y-lesson--broken" data-resolution="unresolved"`,
		// The side branch is a sibling after its run closes, never a list item:
		// its heading stays in the document outline.
		`</ol><div class="y-module y-module--local">`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("ordered course list is missing %q; html = %q", want, html)
		}
	}
	// Both interrupted resumptions of the main line — after the second part's
	// heading and after the side branch — carry the resuming name.
	if got := strings.Count(html, `<ol class="y-lessons" aria-label="主線（接續）">`); got != 2 {
		t.Errorf("the course renders %d resuming main-line lists, want 2; html = %q", got, html)
	}
	if strings.Contains(html, "Routine") {
		t.Errorf("a branch declared out of the course reached its page; html = %q", html)
	}
}

// TestASideBranchHandsOutNoMainLineNumbers pins the one shape where the drawn
// tree and navigation's walk disagree: a branch nested inside a side branch is
// drawn, but the walk never descends into a side branch, so no order ever
// reaches its rows. Numbering them would print ordinals no arrow can follow —
// they render as plain rows outside any list, without a number and without a
// component name. The side branch's own row after the interruption resumes
// the side branch's order and says so.
func TestASideBranchHandsOutNoMainLineNumbers(t *testing.T) {
	t.Parallel()

	body := "## Main {sequence=primary}\n\n" +
		"- [[Slices]]\n" +
		"\t- 選修 {sequence=local}\n" +
		"\t\t- [[Tuning]]\n" +
		"\t\t\t- 深入 {sequence=primary}\n" +
		"\t\t\t\t- [[GC]]\n" +
		"\t\t- [[Arrays]]\n"
	path := buildTestPath(t, body)
	view := BuildPathView(&path, []nav.Path{path})

	var walkless *PathEntryView
	var find func(items []PathItemView)
	find = func(items []PathItemView) {
		for _, item := range items {
			switch {
			case item.Entry != nil && item.Entry.Text == "GC":
				walkless = item.Entry
			case item.Branch != nil:
				find(item.Branch.Items)
			}
		}
	}
	for _, b := range view.Branches {
		find(b.Items)
	}
	if walkless == nil {
		t.Fatal("the nested primary branch's row is not drawn at all")
	}
	if walkless.Number != 0 {
		t.Errorf("a row navigation never walks carries number %d, want 0", walkless.Number)
	}

	var out bytes.Buffer
	if err := Syllabus(view, layouts.Chrome{}).Render(t.Context(), &out); err != nil {
		t.Fatalf("render syllabus: %v", err)
	}
	html := out.String()
	if got := strings.Count(html, "<li value="); got != 3 {
		t.Errorf("the page numbers %d rows, want 3: the main line's one and the side branch's two", got)
	}
	if got := strings.Count(html, `<ol class="y-lessons"`); got != 3 {
		t.Errorf("the page renders %d ordered lists, want 3: the walkless branch must not open one", got)
	}
	for _, want := range []string{
		`<ol class="y-lessons" aria-label="主線">`,
		`<ol class="y-lessons" aria-label="支線：選修">`,
		// The side branch's row after the interruption resumes the branch's
		// own count and name.
		`<ol class="y-lessons" aria-label="支線：選修（接續）">`,
		`<li value="2"><a class="y-lesson" href="/notes/Writing/Arrays.md"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("side-branch fragments are missing %q; html = %q", want, html)
		}
	}
	if !strings.Contains(html, `<a class="y-lesson" href="/notes/Writing/GC.md"`) {
		t.Errorf("the walkless row disappeared instead of rendering unnumbered; html = %q", html)
	}
	if strings.Contains(html, `<li value="0">`) {
		t.Errorf("a walkless row was numbered onto an order that never reaches it; html = %q", html)
	}
}

func TestRoman(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   int
		want string
	}{
		{name: "one", in: 1, want: "I"},
		{name: "four", in: 4, want: "IV"},
		{name: "eight", in: 8, want: "VIII"},
		{name: "nine", in: 9, want: "IX"},
		{name: "twelve", in: 12, want: "XII"},
		{name: "non-positive falls back to decimal", in: 0, want: "0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := roman(tt.in); got != tt.want {
				t.Errorf("roman(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// buildTestPath writes a small vault holding one study path and the lessons it
// names, then reads it back through the real navigation build. The page is
// asserted against the interpretation the product actually produces, so a test
// cannot agree with a projection the product would never make. extra files are
// written beside the fixed fixture set.
func buildTestPath(t *testing.T, body string, extra ...map[string]string) nav.Path {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"Maps/Go path.md":              "---\ntitle: Go path\ntype: study-path\ndomain: golang\n---\n\n" + body,
		"Writing/Slices.md":            "---\ntitle: Slices\ntype: lesson\nstatus: " + schema.SealStatus + "\n---\nbody\n",
		"Writing/Arrays.md":            "---\ntitle: Arrays\ntype: lesson\nstatus: draft\n---\nbody\n",
		"Writing/GC.md":                "---\ntitle: GC\ntype: lesson\nstatus: " + schema.SealStatus + "\n---\nbody\n",
		"Writing/Tuning.md":            "---\ntitle: Tuning\ntype: lesson\nstatus: draft\n---\nbody\n",
		"System/templates/Template.md": "---\ntitle: Template\ntype: lesson\nstatus: draft\n---\nbody\n",
	}
	for _, m := range extra {
		maps.Copy(files, m)
	}
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	scan, err := reader.ScanComplete(t.Context())
	if err != nil {
		t.Fatalf("ScanComplete() error = %v", err)
	}
	notes := make(map[string]*vault.Note)
	noteList := make([]*vault.Note, 0, len(scan.Files()))
	for _, entry := range scan.Files() {
		data, readErr := reader.ReadFile(t.Context(), entry)
		if readErr != nil {
			t.Fatalf("ReadFile() error = %v", readErr)
		}
		note := vault.Parse(entry.Path(), data)
		notes[entry.Path()] = note
		noteList = append(noteList, note)
	}
	contract, err := schema.LoadFile(filepath.Join("..", "..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("schema.LoadFile = %v", err)
	}
	model := nav.New(
		scan.Files(), notes, graph.New(noteList, nil),
		contract.NavigationRoles(), contract.KnowledgeScope(), contract.ArtifactPolicy(),
	)
	paths := model.Paths()
	if len(paths) != 1 {
		t.Fatalf("fixture produced %d study paths, want 1", len(paths))
	}
	return paths[0]
}
