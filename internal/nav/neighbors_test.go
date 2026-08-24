package nav

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/vault"
)

// The syllabus is written forward: planned lessons sit in the list as warning
// rows long before they exist. Stepping through the course must move between
// lessons the reader can open, so a warning row keeps its place on the page
// and contributes no stop — and a note two paths both teach gets one answer
// per path, never a guess about which course the reader is walking.
func TestNeighbors(t *testing.T) {
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

	idx := stubResolver{
		"L01 Slices":  {Kind: graph.Unique, Path: "Writing/lessons/golang/Slices.md"},
		"L03 Maps":    {Kind: graph.Unique, Path: "Writing/lessons/golang/Maps.md"},
		"L04 Strings": {Kind: graph.Unique, Path: "Writing/lessons/golang/Strings.md"},
		"S01 Side":    {Kind: graph.Unique, Path: "Writing/lessons/golang/Side1.md"},
		"S02 Side":    {Kind: graph.Unique, Path: "Writing/lessons/golang/Side2.md"},
		"L01 て形":      {Kind: graph.Unique, Path: "Writing/lessons/japanese/Te.md"},
		// A planned lesson resolves to nothing, and a template resolves to a
		// file the artifact policy keeps out of the lifecycle.
		"Template-only": {Kind: graph.Unique, Path: "System/templates/Card.md"},
	}
	policy := testArtifactPolicy(t)
	goPath := buildPath(pathNote("Maps/go.md", "Go 課綱", goBody), idx, nil, policy)
	jpPath := buildPath(pathNote("Maps/jp.md", "日本語 學習路徑", jpBody), idx, nil, policy)
	m := &Model{paths: []Path{goPath, jpPath}}

	tests := []struct {
		name    string
		relPath string
		want    []Neighbors
	}{
		{
			name:    "a planned lesson between two written ones is not a stop",
			relPath: "Writing/lessons/golang/Maps.md",
			want: []Neighbors{{
				PathTitle: "Go 課綱", PathRelPath: "Maps/go.md",
				// L02 is unresolved, so the step back lands on L01, and the
				// step forward crosses into the subbranch, skipping the
				// template row the same way.
				Prev: ref("L01 Slices", "Writing/lessons/golang/Slices.md"),
				Next: ref("L04 Strings", "Writing/lessons/golang/Strings.md"),
			}},
		},
		{
			name:    "the opening lesson of a path has no previous",
			relPath: "Writing/lessons/japanese/Te.md",
			want: []Neighbors{{
				PathTitle: "日本語 學習路徑", PathRelPath: "Maps/jp.md",
				Next: ref("L01 Slices", "Writing/lessons/golang/Slices.md"),
			}},
		},
		{
			name:    "the closing lesson of a path has no next",
			relPath: "Writing/lessons/golang/Strings.md",
			want: []Neighbors{{
				PathTitle: "Go 課綱", PathRelPath: "Maps/go.md",
				Prev: ref("L03 Maps", "Writing/lessons/golang/Maps.md"),
			}},
		},
		{
			name:    "a lesson two paths teach answers once per path",
			relPath: "Writing/lessons/golang/Slices.md",
			want: []Neighbors{
				{
					PathTitle: "Go 課綱", PathRelPath: "Maps/go.md",
					Next: ref("L03 Maps", "Writing/lessons/golang/Maps.md"),
				},
				{
					PathTitle: "日本語 學習路徑", PathRelPath: "Maps/jp.md",
					Prev: ref("L01 て形", "Writing/lessons/japanese/Te.md"),
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
			got := m.Neighbors(tt.relPath)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Neighbors(%q) mismatch (-want +got):\n%s", tt.relPath, diff)
			}
		})
	}
}

// A folder of dated entries is a line, and the reader walking it wants the
// next one. Until this existed the only way to take that step was back into
// the rail: a diarist reading a week retyped the same seven-character filter
// once per entry, because opening a day cleared it.
func TestAdjacent(t *testing.T) {
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
			prev, next := m.Adjacent(tt.relPath)
			if diff := cmp.Diff(tt.wantPrev, prev); diff != "" {
				t.Errorf("Adjacent(%q) prev mismatch (-want +got):\n%s", tt.relPath, diff)
			}
			if diff := cmp.Diff(tt.wantNext, next); diff != "" {
				t.Errorf("Adjacent(%q) next mismatch (-want +got):\n%s", tt.relPath, diff)
			}
		})
	}
}

// A folder mixes what a reader reads with what merely sits beside it: an image
// stored between two dated entries is part of the folder, not part of the line
// being read. From a note, prev and next step over assets so the reading walk
// stays note to note; the assets keep their place in the sibling rail and on
// the folder page, and reading an asset itself still walks the whole folder.
func TestAdjacentFromANoteStepsOverAssets(t *testing.T) {
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
			prev, next := m.Adjacent(tt.relPath)
			if diff := cmp.Diff(tt.wantPrev, prev); diff != "" {
				t.Errorf("Adjacent(%q) prev mismatch (-want +got):\n%s", tt.relPath, diff)
			}
			if diff := cmp.Diff(tt.wantNext, next); diff != "" {
				t.Errorf("Adjacent(%q) next mismatch (-want +got):\n%s", tt.relPath, diff)
			}
		})
	}
}

// The journal drawer is the one projection that had not moved off file times,
// and it is the one where they mean least: a clone stamps every entry with one
// moment, and an entry edited today is not today's entry. It orders by the
// entries' own names now — newest first — which is the same reading order the
// folder tree beside it uses.
func TestJournalOrdersByTheEntriesOwnNames(t *testing.T) {
	t.Parallel()

	paths := []string{
		"Diary/2025-08-04.md",
		"Diary/2025-08-05.md",
		"Diary/2025-08-06.md",
		"Concepts/not a journal.md",
	}
	// Times that would order them backwards if times still decided: the oldest
	// entry touched most recently is exactly what a checkout or a typo fix does.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mtimes := map[string]time.Time{
		"Diary/2025-08-04.md": base.AddDate(0, 0, 3),
		"Diary/2025-08-05.md": base.AddDate(0, 0, 2),
		"Diary/2025-08-06.md": base,
	}

	got := buildJournal(paths, mtimes)
	want := []string{"Diary/2025-08-06.md", "Diary/2025-08-05.md", "Diary/2025-08-04.md"}
	gotPaths := make([]string, 0, len(got))
	for _, e := range got {
		gotPaths = append(gotPaths, e.RelPath)
	}
	if diff := cmp.Diff(want, gotPaths); diff != "" {
		t.Errorf("buildJournal ordering mismatch (-want +got):\n%s", diff)
	}
}

// stubResolver answers the wikilinks these courses name and reports every other
// name as unresolved, which is what a planned lesson is.
type stubResolver map[string]graph.Resolution

func (r stubResolver) Resolve(name string) graph.Resolution {
	if res, ok := r[name]; ok {
		return res
	}
	return graph.Resolution{Kind: graph.Unresolved}
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
		Frontmatter: map[string]any{"title": title, "type": "study-path"},
		Body:        body,
		BodyLine:    1,
	}
}
