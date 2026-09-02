package pages

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/sequence"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/vaultfs"
	"github.com/koopa0/yomihon/internal/wording"
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
// linked and warning rows, Ready counts only resolved entries at ready, and
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
		Ready:   2, // Slices + GC sit at ready; Arrays is draft
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
	reader, err := vaultfs.Open(root)
	if err != nil {
		t.Fatalf("vaultfs.Open() error = %v", err)
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

// TestSyllabusSaysWhyItFoundNoCourse holds the page's answer for a study-path
// note whose ordered links declare no structure this reader recognises. The
// header counted them honestly — nought parts, nought lessons — and the body
// below it was blank, so the page read as a course that exists and is empty
// rather than as prose the parser was never given a course in. The two need
// opposite repairs, and the second is the common one: the structure is
// declared by a marker on a heading, and a note written without one places
// nothing no matter how many lessons it lists.
//
// The page still states nothing about what the note should have said. It names
// where the structure comes from and stops; a human edits the file.
func TestSyllabusSaysWhyItFoundNoCourse(t *testing.T) {
	t.Parallel()

	render := func(t *testing.T, view PathView) string {
		t.Helper()
		var out bytes.Buffer
		if err := Syllabus(view, layouts.Chrome{}).Render(t.Context(), &out); err != nil {
			t.Fatalf("render syllabus: %v", err)
		}
		return out.String()
	}

	empty := render(t, PathView{Title: "Path", RelPath: "Maps/Path.md"})
	if !strings.Contains(empty, "沒有讀到任何課程結構") {
		t.Errorf("a path with no recognised structure says nothing about why:\n%s", empty)
	}
	if !strings.Contains(empty, "{sequence=") {
		t.Errorf("the empty state does not name what declares the structure:\n%s", empty)
	}

	// The control: a page that did find a course must not carry the notice,
	// or the check above would pass on every study path in the vault.
	found := render(t, PathView{
		Title:    "Path",
		RelPath:  "Maps/Path.md",
		Parts:    1,
		Entries:  1,
		Branches: []PathBranchView{{Anchor: "part-1", Ordinal: "I", Heading: "Part", Depth: 0}},
	})
	if strings.Contains(found, "沒有讀到任何課程結構") {
		t.Errorf("a path that found its course still claims it found none:\n%s", found)
	}
}

// TestSyllabusGuideClaimsNothingAboutTheNote holds the first-time invitation
// to what the page can vouch for. It used to promise the note explains the
// course's purpose, its daily rhythm, how its branches divide and what counts
// as finished — four claims about a file the page never reads, printed on
// every path including one whose structure could not be read at all. The
// invitation may say what this page is and where the author's own text lives;
// what that text contains is the author's to say.
func TestSyllabusGuideClaimsNothingAboutTheNote(t *testing.T) {
	t.Parallel()

	render := func(t *testing.T, view PathView, lang wording.Lang) string {
		t.Helper()
		var out bytes.Buffer
		if err := Syllabus(view, layouts.Chrome{Lang: lang}).Render(t.Context(), &out); err != nil {
			t.Fatalf("render syllabus: %v", err)
		}
		return out.String()
	}
	empty := PathView{Title: "Path", RelPath: "Maps/Path.md", GuideHref: "/notes/Maps/Path.md"}
	normal := PathView{
		Title: "Path", RelPath: "Maps/Path.md", GuideHref: "/notes/Maps/Path.md",
		Parts: 1, Entries: 1,
		Branches: []PathBranchView{{Anchor: "part-1", Ordinal: "I", Heading: "Part", Depth: 0}},
	}
	claims := []string{
		"課程目的", "每日節奏", "支線分工", "完成標準",
		"what the course is for", "daily rhythm", "how the branches divide", "counts as finished",
	}
	for name, view := range map[string]PathView{"empty path": empty, "normal path": normal} {
		for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
			html := render(t, view, lang)
			for _, claim := range claims {
				if strings.Contains(html, claim) {
					t.Errorf("the %s page (%s) claims the note contains %q", name, lang, claim)
				}
			}
		}
	}

	// The invitation itself survives: the block, its question, and the way to
	// the note's own page — where whatever the author wrote actually lives.
	html := render(t, normal, wording.ZhHant)
	for _, want := range []string{
		"第一次使用這條路徑？",
		`<a href="/notes/Maps/Path.md">`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("the guide invitation lost %q; html = %q", want, html)
		}
	}
}

// TestSyllabusSeparatesNoMarkerFromUnreadableMarker splits the empty course
// page's explanation into the two repairs it stands for. A note that wrote no
// marker gets the grammar: which markers declare structure, and that a
// heading without one is not read as a part. A note that wrote a marker the
// grammar could not use must not be told it wrote none — that sentence is
// false there — so it is told what a marker may say instead. Both name the
// three values the grammar accepts, spelled from the grammar's own code, and
// the two texts share no claim that is true of only one of them.
func TestSyllabusSeparatesNoMarkerFromUnreadableMarker(t *testing.T) {
	t.Parallel()

	render := func(t *testing.T, body string) string {
		t.Helper()
		path := buildTestPath(t, body)
		view := BuildPathView(&path, []nav.Path{path})
		if len(view.Branches) != 0 {
			t.Fatalf("the fixture grew a course; the empty-state page never renders")
		}
		var out bytes.Buffer
		if err := Syllabus(view, layouts.Chrome{}).Render(t.Context(), &out); err != nil {
			t.Fatalf("render syllabus: %v", err)
		}
		return out.String()
	}

	noMarker := render(t, "## Part\n\n- [[Slices]]\n")
	for _, want := range []string{
		"沒有讀到任何課程結構",
		"<code>{sequence=primary}</code>",
		"<code>{sequence=local}</code>",
		"<code>{sequence=none}</code>",
		"沒有標記的標題不會被讀成分部",
	} {
		if !strings.Contains(noMarker, want) {
			t.Errorf("the no-marker page is missing %q", want)
		}
	}
	if strings.Contains(noMarker, "寫了 sequence 標記") {
		t.Errorf("the no-marker page claims a marker was written:\n%s", noMarker)
	}

	badValue := render(t, "## Part {sequence=primry}\n\n- [[Slices]]\n")
	for _, want := range []string{
		"寫了 sequence 標記",
		"沒有一個分部能讀成課程結構",
		"<code>primary</code>",
		"<code>local</code>",
		"<code>none</code>",
	} {
		if !strings.Contains(badValue, want) {
			t.Errorf("the unreadable-marker page is missing %q", want)
		}
	}
	// The claim that no marker exists is exactly what is false here.
	if strings.Contains(badValue, "沒有標記的標題不會被讀成分部") {
		t.Errorf("a page whose note wrote a marker still claims markers are absent:\n%s", badValue)
	}
	if strings.Contains(badValue, "沒有讀到任何課程結構") {
		t.Errorf("the two empty states still share one wording:\n%s", badValue)
	}
}

// TestMarkerWrittenDividesEveryGrammarRule pins the total division behind
// the empty-course page's explanations: for every rule the grammar declares,
// whether it implies the author actually wrote a sequence marker.
//
// The expectation is keyed by rule and its keys are compared against the
// grammar's own list, so a rule declared later arrives here uncovered and this
// test says so. That is where a new rule has to be caught, because the page
// itself does not abort on one: the grammar owns the set and may grow it, and a
// course page that crashed on a rule it had not met would be the reporter
// breaking on the news. Such a rule reads as neither verdict — both of those
// are claims about what the author wrote, and one of them would be false.
func TestMarkerWrittenDividesEveryGrammarRule(t *testing.T) {
	t.Parallel()

	want := map[sequence.Rule]markerVerdict{
		sequence.RuleRoleMissing:        markerNotWritten,
		sequence.RuleEntryOutsideBranch: markerNotWritten,
		sequence.RuleEntryMultiTarget:   markerNotWritten,
		sequence.RuleEntryNoncanonical:  markerNotWritten,
		sequence.RuleRoleInvalid:        markerWritten,
		sequence.RuleRoleDuplicate:      markerWritten,
		sequence.RuleRoleMisplaced:      markerWritten,
		sequence.RuleRoleConflict:       markerWritten,
		sequence.RuleRoleOnEntry:        markerWritten,
		sequence.RuleLocalOrphan:        markerWritten,
		sequence.RuleNestingTooDeep:     markerWritten,
	}
	declared := sequence.Rules()
	if len(declared) == 0 {
		t.Fatal("the grammar declares no rules, so this division covers nothing")
	}
	for _, rule := range declared {
		expect, classified := want[rule]
		if !classified {
			t.Errorf("the grammar declares %q and this division does not classify it: decide whether it implies a written marker and add it here", rule)
			continue
		}
		if got := markerVerdictFor(rule); got != expect {
			t.Errorf("markerVerdictFor(%q) = %d, want %d", rule, got, expect)
		}
	}
	for rule := range want {
		if !slices.Contains(declared, rule) {
			t.Errorf("this division classifies %q, which the grammar no longer declares", rule)
		}
	}
	if got := markerVerdictFor("path.never_declared"); got != markerUnknownRule {
		t.Errorf("markerVerdictFor(an undeclared rule) = %d, want markerUnknownRule: neither answer about the author's file is evidenced by a rule name nobody here recognises", got)
	}
}

// TestAnUnexplainedRuleClaimsNothingAboutTheAuthor holds the third empty-course
// sentence to what it must not say. The page reaches it when the grammar
// reports a rule it has not been told about, and the two sentences it would
// otherwise choose between are both assertions about a file this page has not
// read: that markers are absent, or that markers were written and refused.
func TestAnUnexplainedRuleClaimsNothingAboutTheAuthor(t *testing.T) {
	t.Parallel()

	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		t.Run(string(lang), func(t *testing.T) {
			t.Parallel()

			var page bytes.Buffer
			view := PathView{Title: "A path", RelPath: "Maps/p.md", NoCourse: markerUnknownRule}
			if err := Syllabus(view, layouts.Chrome{Lang: lang}).Render(t.Context(), &page); err != nil {
				t.Fatalf("render: %v", err)
			}
			body := page.String()
			if !strings.Contains(body, wording.NoCourseUnexplained.In(lang)) {
				t.Errorf("the page does not say it has no explanation; want %q", wording.NoCourseUnexplained.In(lang))
			}
			for name, claim := range map[string]wording.Phrase{
				"no marker was written": wording.NoCourseIntro,
				"a marker was refused":  wording.NoCourseMarkerFaultIntro,
			} {
				if strings.Contains(body, claim.In(lang)) {
					t.Errorf("the page claims %s about a note it read no rule for", name)
				}
			}
		})
	}
}

// TestAnUnknownRuleDoesNotSilenceAWrittenMarker holds the order the two
// verdicts are read in. A path can report several rules at once, and one that
// says a marker was written is the one claim the page can make from what the
// grammar reported: an unrecognised rule beside it is not evidence against it.
func TestAnUnknownRuleDoesNotSilenceAWrittenMarker(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		rules []sequence.Rule
		want  markerVerdict
	}{
		{name: "a written marker among rules nobody here knows", rules: []sequence.Rule{"path.never_declared", sequence.RuleRoleInvalid}, want: markerWritten},
		{name: "an unknown rule beside one that implies no marker", rules: []sequence.Rule{sequence.RuleRoleMissing, "path.never_declared"}, want: markerUnknownRule},
		{name: "rules that all imply no marker", rules: []sequence.Rule{sequence.RuleRoleMissing, sequence.RuleEntryNoncanonical}, want: markerNotWritten},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := nav.Path{Title: "A path", RelPath: "Maps/p.md"}
			for _, rule := range tt.rules {
				path.Diagnostics = append(path.Diagnostics, sequence.Diagnostic{Rule: rule})
			}
			if got := BuildPathView(&path, nil).NoCourse; got != tt.want {
				t.Errorf("NoCourse = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestACourseReportsAnItemItCouldNotRead holds the page against dropping
// something silently. A branch lists rows and nested branches; a value that is
// neither is a fault, and a course page that quietly skipped it would show a
// list one item shorter than the one its author wrote, with nothing anywhere
// saying so. The page reports and never repairs, so the row keeps its place and
// says what happened.
func TestACourseReportsAnItemItCouldNotRead(t *testing.T) {
	t.Parallel()

	branch := PathBranchView{
		Heading: "Part",
		Items: []PathItemView{
			{Entry: &PathEntryView{Text: "L01", Kind: nav.EntryResolved, Href: "/notes/L01.md", Number: 1}},
			{},
			{Entry: &PathEntryView{Text: "L02", Kind: nav.EntryResolved, Href: "/notes/L02.md", Number: 2}},
		},
	}
	runs := branch.Runs(wording.ZhHant)
	faults := 0
	for _, run := range runs {
		if run.Fault != "" {
			faults++
		}
	}
	if faults != 1 {
		t.Fatalf("Runs() reported %d faults for one unreadable item, want 1: %+v", faults, runs)
	}

	view := PathView{Title: "Course", Branches: []PathBranchView{branch}}
	var buf bytes.Buffer
	if err := Syllabus(view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render syllabus: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, wording.PathItemUnreadable.In(wording.ZhHant)) {
		t.Errorf("the course page drops an item it could not read instead of saying so; html = %q", html)
	}
	for _, want := range []string{">L01<", ">L02<"} {
		if !strings.Contains(html, want) {
			t.Errorf("the rows either side of the unreadable item are missing %q; html = %q", want, html)
		}
	}
}
