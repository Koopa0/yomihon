package nav

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestHomeCountsTheMainLineOnly holds the ruling Home's number rests on: the
// course total is the declared main line, side branches excluded and blocks
// declared out of the course excluded. A branch nobody declared counts nothing
// at all — the course is declared, never inferred.
func TestHomeCountsTheMainLineOnly(t *testing.T) {
	t.Parallel()

	idx := resolver(t,
		"Writing/L01.md", "Writing/L02.md", "Writing/L03.md",
		"Writing/S01.md", "Writing/S02.md", "Writing/R01.md", "Writing/U01.md")
	body := "## 主線 {sequence=primary}\n\n" +
		"- [[L01]]\n" +
		"- [[L02]]\n" +
		"\t- 支線 {sequence=local}\n" +
		"\t\t- [[S01]]\n" +
		"\t\t- [[S02]]\n" +
		"- [[L03]]\n" +
		"\n## 日常 {sequence=none}\n\n- [[R01]]\n" +
		"\n## 忘了宣告\n\n- [[U01]]\n"
	p := buildPath(pathNote("Maps/Course.md", "Course", body), idx, nil, testArtifactPolicy(t))

	if p.Planned != 3 {
		t.Errorf("Planned = %d, want 3: the main line's three lessons, without the side branch, the routine block, or the branch nobody declared", p.Planned)
	}
}

// TestAnUndeclaredCourseProjectsNothing is the other half of "declared, never
// inferred". A path written without a single marker reads as prose: it plans no
// lessons and offers no walk, and the author hears about it from the judge.
func TestAnUndeclaredCourseProjectsNothing(t *testing.T) {
	t.Parallel()

	idx := resolver(t, "Writing/L01.md", "Writing/L02.md")
	body := "## Part\n\n- [[L01]]\n- [[L02]]\n"
	p := buildPath(pathNote("Maps/Course.md", "Course", body), idx, nil, testArtifactPolicy(t))

	if p.Planned != 0 {
		t.Errorf("Planned = %d, want 0; nothing here says these rows are a course", p.Planned)
	}
	if len(p.components) != 0 {
		t.Errorf("components = %v, want none", p.components)
	}
	if len(p.Groups) != 1 || p.Groups[0].Projectable {
		t.Errorf("branch projectable = %+v, want an undeclared branch that projects nothing", p.Groups)
	}
}

// TestPrimaryAndLocalNeverLinkToEachOther holds the walk's boundary. The lesson
// after the one a side branch hangs from is the next main-line lesson; the side
// branch has its own first and last and never rejoins.
func TestPrimaryAndLocalNeverLinkToEachOther(t *testing.T) {
	t.Parallel()

	idx := resolver(t,
		"Writing/L01.md", "Writing/L02.md", "Writing/L03.md",
		"Writing/S01.md", "Writing/S02.md")
	body := "## 主線 {sequence=primary}\n\n" +
		"- [[L01]]\n" +
		"- [[L02]]\n" +
		"\t- 支線 {sequence=local}\n" +
		"\t\t- [[S01]]\n" +
		"\t\t- [[S02]]\n" +
		"- [[L03]]\n"
	p := buildPath(pathNote("Maps/Course.md", "Course", body), idx, nil, testArtifactPolicy(t))
	m := &Model{paths: []Path{p}}

	ref := func(name, rel string) NoteRef { return NoteRef{Name: name, RelPath: rel} }
	tests := []struct {
		name string
		at   string
		want []Neighbors
	}{
		{
			name: "the main line steps over the side branch",
			at:   "Writing/L02.md",
			want: []Neighbors{{
				PathTitle: "Course", PathRelPath: "Maps/Course.md",
				Prev: ref("L01", "Writing/L01.md"),
				Next: ref("L03", "Writing/L03.md"),
			}},
		},
		{
			name: "the side branch opens on its own",
			at:   "Writing/S01.md",
			want: []Neighbors{{
				PathTitle: "Course", PathRelPath: "Maps/Course.md",
				Next: ref("S02", "Writing/S02.md"),
			}},
		},
		{
			name: "the side branch closes without rejoining",
			at:   "Writing/S02.md",
			want: []Neighbors{{
				PathTitle: "Course", PathRelPath: "Maps/Course.md",
				Prev: ref("S01", "Writing/S01.md"),
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tt.want, m.Neighbors(tt.at)); diff != "" {
				t.Errorf("Neighbors(%q) mismatch (-want +got):\n%s", tt.at, diff)
			}
		})
	}
}

// TestAStructuralHeadingStillCarriesItsParts is the reductio a depth-scoped
// implementation fails: both of this vault's real courses put every lesson in a
// level-3 branch under a level-2 part. Counting has to follow the declaration,
// never the nesting depth.
func TestAStructuralHeadingStillCarriesItsParts(t *testing.T) {
	t.Parallel()

	idx := resolver(t, "Writing/L01.md", "Writing/L02.md")
	body := "## Part\n\n### Module {sequence=primary}\n\n- [[L01]]\n- [[L02]]\n"
	p := buildPath(pathNote("Maps/Course.md", "Course", body), idx, nil, testArtifactPolicy(t))

	if p.Planned != 2 {
		t.Errorf("Planned = %d, want 2; a part that only groups modules still carries their lessons", p.Planned)
	}
	if len(p.components) != 1 || len(p.components[0]) != 2 {
		t.Errorf("walk = %v, want both lessons on the main line", p.components)
	}
}

// TestReversePlacementTakesOnlyCourseMembership holds what "this note is in
// that course" means. A row in a block declared out of the course, or in one
// nobody declared, is not a membership and must not appear as one.
func TestReversePlacementTakesOnlyCourseMembership(t *testing.T) {
	t.Parallel()

	idx := resolver(t, "Writing/L01.md", "Writing/R01.md", "Writing/U01.md")
	body := "## 主線 {sequence=primary}\n\n- [[L01]]\n" +
		"\n## 日常 {sequence=none}\n\n- [[R01]]\n" +
		"\n## 忘了宣告\n\n- [[U01]]\n"
	p := buildPath(pathNote("Maps/Course.md", "Course", body), idx, nil, testArtifactPolicy(t))

	index := make(map[string][]Placement)
	pathPlacements(index, &p)

	if len(index["Writing/L01.md"]) != 1 {
		t.Errorf("placements for the declared lesson = %v, want one", index["Writing/L01.md"])
	}
	for _, rel := range []string{"Writing/R01.md", "Writing/U01.md"} {
		if got := index[rel]; len(got) != 0 {
			t.Errorf("placements for %s = %v, want none: it is listed, but not as part of the course", rel, got)
		}
	}
}
