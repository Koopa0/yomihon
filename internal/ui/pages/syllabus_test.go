package pages

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/kurodo/internal/graph"
	"github.com/koopa0/kurodo/internal/nav"
)

// TestBuildSyllabusView pins the pure transform's contract: a section's
// lesson count equals the number of resolved lesson
// list-items beneath it, document order is preserved at every level, and an
// unresolved lesson is STILL present, carrying its mark instead of a link
// — never dropped. The fixture is two parts, one with a module and a
// broken lesson, one with a lesson attached directly (no module), so the
// tallies and the modules count are hand-derivable and non-tautological.
func TestBuildSyllabusView(t *testing.T) {
	t.Parallel()

	syl := nav.Syllabus{
		Title:   "Go path",
		RelPath: "Maps/Go path.md",
		Sections: []nav.Section{
			{
				Heading: "Data", Level: 2,
				Sub: []nav.Section{
					{
						Heading: "Text", Level: 3,
						Lessons: []nav.Lesson{
							{Text: "Slices", RelPath: "Writing/Slices.md", Status: "ready", Resolution: graph.Unique},
							{Text: "Arrays", RelPath: "Writing/Arrays.md", Status: "draft", Resolution: graph.Unique},
							{Text: "Ghost", Target: "Ghost", Resolution: graph.Unresolved},
						},
					},
				},
			},
			{
				Heading: "Memory", Level: 2,
				Lessons: []nav.Lesson{
					{Text: "GC", RelPath: "Writing/GC.md", Status: "ready", Resolution: graph.Unique},
				},
			},
		},
	}

	got := BuildSyllabusView(syl, []nav.Syllabus{syl})

	want := SyllabusView{
		Title:   "Go path",
		RelPath: "Maps/Go path.md",
		Paths: []SyllabusLink{
			{Title: "Go path", RelPath: "Maps/Go path.md", Lessons: 4, Active: true},
		},
		Parts:   2,
		Modules: 1, // only "Data" has a sub-section; "Memory" holds a lesson directly
		Lessons: 4,
		Ready:   2, // Slices + GC; Arrays is draft, Ghost is unresolved
		Sections: []SectionView{
			{
				Anchor: "part-1", Ordinal: "I", Num: 1, Heading: "Data", Depth: 0,
				Ready: 1, Total: 3,
				Sub: []SectionView{
					{
						Num: 1, Heading: "Text", Depth: 1, Ready: 1, Total: 3,
						Lessons: []LessonView{
							{Text: "Slices", Href: "/notes/Writing/Slices.md", Status: "ready"},
							{Text: "Arrays", Href: "/notes/Writing/Arrays.md", Status: "draft"},
							{Text: "Ghost", Mark: "unresolved"},
						},
					},
				},
			},
			{
				Anchor: "part-2", Ordinal: "II", Num: 2, Heading: "Memory", Depth: 0,
				Ready: 1, Total: 1,
				Lessons: []LessonView{
					{Text: "GC", Href: "/notes/Writing/GC.md", Status: "ready"},
				},
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("BuildSyllabusView() mismatch (-want +got):\n%s", diff)
	}
}

// TestBuildSyllabusViewAmbiguous pins the other never-drop branch: an ambiguous
// lesson is listed too, marked "ambiguous" and unlinked (a link would silently
// pick one of the collision's targets).
func TestBuildSyllabusViewAmbiguous(t *testing.T) {
	t.Parallel()

	syl := nav.Syllabus{
		Title: "P", RelPath: "Maps/P.md",
		Sections: []nav.Section{{
			Heading: "Only", Level: 2,
			Lessons: []nav.Lesson{
				{Text: "Dup", Target: "Dup", Resolution: graph.Ambiguous, Candidates: []string{"a/Dup.md", "b/Dup.md"}},
			},
		}},
	}

	got := BuildSyllabusView(syl, nil)
	want := []LessonView{{Text: "Dup", Mark: "ambiguous"}}
	if diff := cmp.Diff(want, got.Sections[0].Lessons); diff != "" {
		t.Errorf("ambiguous lesson mismatch (-want +got):\n%s", diff)
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

func TestFillBucket(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		ready, total int
		want         int
	}{
		{name: "empty path is zero", ready: 0, total: 0, want: 0},
		{name: "complete is one hundred", ready: 12, total: 12, want: 100},
		{name: "rounds 68.75pct to 70", ready: 33, total: 48, want: 70},
		{name: "rounds 53.8pct to 55", ready: 7, total: 13, want: 55},
		{name: "rounds 33.3pct to 35", ready: 1, total: 3, want: 35},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := fillBucket(tt.ready, tt.total); got != tt.want {
				t.Errorf("fillBucket(%d, %d) = %d, want %d", tt.ready, tt.total, got, tt.want)
			}
		})
	}
}
