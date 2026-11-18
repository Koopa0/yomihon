package nav

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// The syllabus is written forward: planned lessons sit in the list as warning
// rows long before they exist. Stepping through the course must move between
// lessons the reader can open, so a warning row keeps its place on the page
// and contributes no stop — and a note two paths both teach gets one answer
// per path, never a guess about which course the reader is walking.
func TestPathNeighbors(t *testing.T) {
	t.Parallel()

	ref := func(name, rel string) NoteRef { return NoteRef{Name: name, RelPath: rel} }

	// The courses are written as an author writes them and read through the
	// declared-sequence grammar, so the walk under test is the one the product
	// builds — not a hand-assembled component list that could agree with a
	// broken projection.
	goBody := "## Data {sequence=primary}\n\n" +
		"- [[L01 Slices]]\n" +
		"- [[L02 Planned]]\n" +
		"- [[L03 Maps]]\n" +
		"\t- 支線 {sequence=local}\n" +
		"\t\t- [[S01 Side]]\n" +
		"\t\t- [[S02 Side]]\n" +
		"\n### Text {sequence=primary}\n\n" +
		"- [[Template-only]]\n" +
		"- [[L04 Strings]]\n"
	jpBody := "## 初級 {sequence=primary}\n\n" +
		"- [[L01 て形]]\n" +
		"- [[L01 Slices]]\n"

	// A planned lesson is simply absent from the index, which is what makes it
	// resolve to nothing; the template is present but sits where the artifact
	// policy keeps it out of the lifecycle.
	idx := resolver(t,
		"Writing/lessons/golang/L01 Slices.md",
		"Writing/lessons/golang/L03 Maps.md",
		"Writing/lessons/golang/L04 Strings.md",
		"Writing/lessons/golang/S01 Side.md",
		"Writing/lessons/golang/S02 Side.md",
		"Writing/lessons/japanese/L01 て形.md",
		"System/templates/Template-only.md",
	)
	policy := testArtifactPolicy(t)
	goPath := buildPath(pathNote("Maps/go.md", "Go 課綱", goBody), idx, nil, policy)
	jpPath := buildPath(pathNote("Maps/jp.md", "日本語 學習路徑", jpBody), idx, nil, policy)
	m := &Model{paths: []Course{goPath, jpPath}}

	tests := []struct {
		name    string
		relPath string
		want    []Neighbors
	}{
		{
			name:    "a planned lesson between two written ones is not a stop",
			relPath: "Writing/lessons/golang/L03 Maps.md",
			want: []Neighbors{{
				PathTitle: "Go 課綱", PathRelPath: "Maps/go.md",
				// L02 is unresolved, so the step back lands on L01, and the
				// step forward crosses into the subbranch, skipping the
				// template row the same way.
				Prev: ref("L01 Slices", "Writing/lessons/golang/L01 Slices.md"),
				Next: ref("L04 Strings", "Writing/lessons/golang/L04 Strings.md"),
			}},
		},
		{
			name:    "the opening lesson of a path has no previous",
			relPath: "Writing/lessons/japanese/L01 て形.md",
			want: []Neighbors{{
				PathTitle: "日本語 學習路徑", PathRelPath: "Maps/jp.md",
				Next: ref("L01 Slices", "Writing/lessons/golang/L01 Slices.md"),
			}},
		},
		{
			name:    "the closing lesson of a path has no next",
			relPath: "Writing/lessons/golang/L04 Strings.md",
			want: []Neighbors{{
				PathTitle: "Go 課綱", PathRelPath: "Maps/go.md",
				Prev: ref("L03 Maps", "Writing/lessons/golang/L03 Maps.md"),
			}},
		},
		{
			name:    "a lesson two paths teach answers once per path",
			relPath: "Writing/lessons/golang/L01 Slices.md",
			want: []Neighbors{
				{
					PathTitle: "Go 課綱", PathRelPath: "Maps/go.md",
					Next: ref("L03 Maps", "Writing/lessons/golang/L03 Maps.md"),
				},
				{
					PathTitle: "日本語 學習路徑", PathRelPath: "Maps/jp.md",
					Prev: ref("L01 て形", "Writing/lessons/japanese/L01 て形.md"),
				},
			},
		},
		{
			name:    "a note no path teaches has no answer",
			relPath: "Concepts/plain.md",
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := m.PathNeighbors(tt.relPath)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("PathNeighbors(%q) mismatch (-want +got):\n%s", tt.relPath, diff)
			}
		})
	}
}

// A folder of dated entries is a line, and the reader walking it wants the
// next one. Until this existed the only way to take that step was back into
// the rail: a diarist reading a week retyped the same seven-character filter
// once per entry, because opening a day cleared it.
func TestFolderStep(t *testing.T) {
	t.Parallel()

	m := &Model{dirNotes: map[string][]NoteRef{
		"Diary": {
			{Name: "2025-08-04", RelPath: "Diary/2025-08-04.md"},
			{Name: "2025-08-05", RelPath: "Diary/2025-08-05.md"},
			{Name: "2025-08-06", RelPath: "Diary/2025-08-06.md"},
		},
		"Solo": {{Name: "only", RelPath: "Solo/only.md"}},
	}}

	tests := []struct {
		name     string
		relPath  string
		wantPrev NoteRef
		wantNext NoteRef
	}{
		{
			name:     "the middle entry has both",
			relPath:  "Diary/2025-08-05.md",
			wantPrev: NoteRef{Name: "2025-08-04", RelPath: "Diary/2025-08-04.md"},
			wantNext: NoteRef{Name: "2025-08-06", RelPath: "Diary/2025-08-06.md"},
		},
		{
			name:     "the first entry has no step back",
			relPath:  "Diary/2025-08-04.md",
			wantNext: NoteRef{Name: "2025-08-05", RelPath: "Diary/2025-08-05.md"},
		},
		{
			name:     "the last entry has no step forward",
			relPath:  "Diary/2025-08-06.md",
			wantPrev: NoteRef{Name: "2025-08-05", RelPath: "Diary/2025-08-05.md"},
		},
		{name: "an only child has neither", relPath: "Solo/only.md"},
		{name: "a file no folder holds has neither", relPath: "Nowhere/gone.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			prev, next := m.FolderStep(tt.relPath)
			if diff := cmp.Diff(tt.wantPrev, prev); diff != "" {
				t.Errorf("FolderStep(%q) prev mismatch (-want +got):\n%s", tt.relPath, diff)
			}
			if diff := cmp.Diff(tt.wantNext, next); diff != "" {
				t.Errorf("FolderStep(%q) next mismatch (-want +got):\n%s", tt.relPath, diff)
			}
		})
	}
}

// A folder mixes what a reader reads with what merely sits beside it: an image
// stored between two dated entries is part of the folder, not part of the line
// being read. From a note, prev and next step over assets so the reading walk
// stays note to note; the assets keep their place in the sibling rail and on
// the folder page, and reading an asset itself still walks the whole folder.
func TestFolderStepFromANoteStepsOverAssets(t *testing.T) {
	t.Parallel()

	m := &Model{dirNotes: map[string][]NoteRef{
		"Diary": {
			{Name: "2025-08-04", RelPath: "Diary/2025-08-04.md"},
			{Name: "scan.png", RelPath: "Diary/scan.png"},
			{Name: "2025-08-05", RelPath: "Diary/2025-08-05.md"},
		},
		"Album": {
			{Name: "cover.png", RelPath: "Album/cover.png"},
			{Name: "notes", RelPath: "Album/notes.md"},
			{Name: "back.png", RelPath: "Album/back.png"},
		},
	}}

	tests := []struct {
		name     string
		relPath  string
		wantPrev NoteRef
		wantNext NoteRef
	}{
		{
			name:     "a note steps over the asset to the next note",
			relPath:  "Diary/2025-08-04.md",
			wantNext: NoteRef{Name: "2025-08-05", RelPath: "Diary/2025-08-05.md"},
		},
		{
			name:     "a note steps over the asset to the previous note",
			relPath:  "Diary/2025-08-05.md",
			wantPrev: NoteRef{Name: "2025-08-04", RelPath: "Diary/2025-08-04.md"},
		},
		{
			name:    "a note fenced by assets alone has neither",
			relPath: "Album/notes.md",
		},
		{
			name:     "reading an asset itself walks the whole folder",
			relPath:  "Diary/scan.png",
			wantPrev: NoteRef{Name: "2025-08-04", RelPath: "Diary/2025-08-04.md"},
			wantNext: NoteRef{Name: "2025-08-05", RelPath: "Diary/2025-08-05.md"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			prev, next := m.FolderStep(tt.relPath)
			if diff := cmp.Diff(tt.wantPrev, prev); diff != "" {
				t.Errorf("FolderStep(%q) prev mismatch (-want +got):\n%s", tt.relPath, diff)
			}
			if diff := cmp.Diff(tt.wantNext, next); diff != "" {
				t.Errorf("FolderStep(%q) next mismatch (-want +got):\n%s", tt.relPath, diff)
			}
		})
	}
}

// The journal drawer is the one projection that had not moved off file times,
// and it is the one where they mean least: a clone stamps every entry with one
// moment, and an entry edited today is not today's entry. It orders by the day
// each entry is for — newest first — and an entry that carries no day at all
// falls to the end rather than sorting among the days it never named.
func TestJournalOrdersByTheDayEachEntryIsFor(t *testing.T) {
	t.Parallel()

	// Times that would order them backwards if times still decided: the oldest
	// entry touched most recently is exactly what a checkout or a typo fix does.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	files := []capturedFile{
		{path: "Diary/2025-08-04.md", modified: base.AddDate(0, 0, 3)},
		{path: "Diary/2025-08-05 夜.md", modified: base.AddDate(0, 0, 1)},
		{path: "Diary/2025-08-05.md", modified: base.AddDate(0, 0, 2)},
		{path: "Diary/loose thoughts.md", modified: base.AddDate(0, 0, 4)},
		{path: "Diary/2025-08-06.md", modified: base},
		{path: "Concepts/not a journal.md", modified: base},
	}

	contract := testContract(t)
	got := buildJournal(files, contract.JournalDir(), contract.AuthoredDate())
	// The pair written on one day is here for the tie: the day cannot separate
	// them, so their own names do, and a tie nothing settles would let two
	// entries swap places between one reading and the next.
	want := []string{
		"Diary/2025-08-06.md",
		"Diary/2025-08-05 夜.md",
		"Diary/2025-08-05.md",
		"Diary/2025-08-04.md",
		"Diary/loose thoughts.md",
	}
	gotPaths := make([]string, 0, len(got))
	for _, e := range got {
		gotPaths = append(gotPaths, e.RelPath)
	}
	if diff := cmp.Diff(want, gotPaths); diff != "" {
		t.Errorf("buildJournal ordering mismatch (-want +got):\n%s", diff)
	}
}

// vaultNote parses a whole note the way the scanner does, so a fixture read
// from disk reports the lines its own file shows.
func vaultNote(tb testing.TB, content string) *vault.Note {
	tb.Helper()
	return vault.Parse("Maps/Course.md", []byte(content))
}

func pathNote(rel, title, body string) *vault.Note {
	return &vault.Note{
		RelPath:     rel,
		Frontmatter: map[string]any{"type": "study-path"},
		Body:        "# " + title + "\n\n" + body,
		BodyLine:    1,
	}
}

// TestCourseUnitsSeparatesTheMainLineFromTheBranchesAndNamesEachOnce holds the
// one list the sidebar's count, the course page's count and a route's answer
// are made from. The course is written as an author writes it, so the list is
// the walk the product builds.
func TestCourseUnitsSeparatesTheMainLineFromTheBranchesAndNamesEachOnce(t *testing.T) {
	t.Parallel()

	ref := func(name, rel string) NoteRef { return NoteRef{Name: name, RelPath: rel} }
	body := "## Data {sequence=primary}\n\n" +
		"- [[L01 Slices]]\n" +
		"- [[L02 Planned]]\n" +
		"- [[L03 Maps]]\n" +
		"\t- 支線 {sequence=local}\n" +
		"\t\t- [[S01 Side]]\n" +
		"\t\t- [[L01 Slices]]\n" +
		"\n### Text {sequence=primary}\n\n" +
		"- [[Template-only]]\n" +
		"- [[L04 Strings]]\n"
	idx := resolver(t,
		"Writing/L01 Slices.md", "Writing/L03 Maps.md", "Writing/L04 Strings.md",
		"Writing/S01 Side.md", "System/templates/Template-only.md",
	)
	// A draft is a unit: the reader can open it, and the course counts it
	// apart as the one thing its author has not finished.
	drafts := draftFacts("Writing/L03 Maps.md")
	course := buildPath(pathNote("Maps/go.md", "Go", body), idx, drafts, testArtifactPolicy(t))
	m := &Model{paths: []Course{course}}

	mainLine, branches, ok := m.CourseUnits("Maps/go.md")
	if !ok {
		t.Fatal("a course the model holds is not answered")
	}
	wantMain := []NoteRef{
		ref("L01 Slices", "Writing/L01 Slices.md"),
		ref("L03 Maps", "Writing/L03 Maps.md"),
		ref("L04 Strings", "Writing/L04 Strings.md"),
	}
	wantBranches := []NoteRef{ref("S01 Side", "Writing/S01 Side.md")}
	if diff := cmp.Diff(wantMain, mainLine); diff != "" {
		t.Errorf("main line (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantBranches, branches); diff != "" {
		t.Errorf("branches (-want +got):\n%s", diff)
	}
	if _, _, ok := m.CourseUnits("Maps/other.md"); ok {
		t.Error("a note that is no course answered as one")
	}
	// Where the rules close the courses there is no course to ask about, so what
	// reads the shell's projection cannot finish a unit of one.
	closed := m.WithoutInstanceProjections(Close(schema.Rejected("rules unavailable")))
	if _, _, ok := closed.CourseUnits("Maps/go.md"); ok {
		t.Error("a course answered while the courses are closed")
	}
	if _, _, ok := (*Model)(nil).CourseUnits("Maps/go.md"); ok {
		t.Error("no model answered as one with a course")
	}
}

// TestCourseRowsCountsEveryRowThePageNumbersAndNamesEachOpenableRowOnceForEachRow
// holds the figures a course states of itself: how many rows its main line plans,
// the ones not written yet included, as the course page prints; and the notes of
// the rows a reader can open, one for each row, so a note the course lists twice
// is two rows and what is finished can reach every row.
func TestCourseRowsCountsEveryRowThePageNumbersAndNamesEachOpenableRowOnceForEachRow(t *testing.T) {
	t.Parallel()

	ref := func(name, rel string) NoteRef { return NoteRef{Name: name, RelPath: rel} }
	body := "## Data {sequence=primary}\n\n" +
		"- [[L01 Slices]]\n" +
		"- [[L02 Planned]]\n" +
		"- [[L03 Maps]]\n" +
		"- [[L01 Slices]]\n" +
		"\t- 支線 {sequence=local}\n" +
		"\t\t- [[S01 Side]]\n"
	idx := resolver(t, "Writing/L01 Slices.md", "Writing/L03 Maps.md", "Writing/S01 Side.md")
	course := buildPath(pathNote("Maps/go.md", "Go", body), idx, nil, testArtifactPolicy(t))
	m := &Model{paths: []Course{course}}

	planned, rows, ok := m.CourseRows("Maps/go.md")
	if !ok {
		t.Fatal("a course the model holds is not answered")
	}
	if planned != course.Planned || planned != 4 {
		t.Errorf("planned = %d (the course page's own count is %d), want the four rows of its main line, the unwritten one included", planned, course.Planned)
	}
	want := []NoteRef{
		ref("L01 Slices", "Writing/L01 Slices.md"),
		ref("L03 Maps", "Writing/L03 Maps.md"),
		ref("L01 Slices", "Writing/L01 Slices.md"),
	}
	if diff := cmp.Diff(want, rows); diff != "" {
		t.Errorf("rows (-want +got):\n%s", diff)
	}
	if _, _, ok := m.CourseRows("Maps/other.md"); ok {
		t.Error("a note that is no course answered as one")
	}
	closed := m.WithoutInstanceProjections(Close(schema.Rejected("rules unavailable")))
	if _, _, ok := closed.CourseRows("Maps/go.md"); ok {
		t.Error("a course answered while the courses are closed")
	}
}
