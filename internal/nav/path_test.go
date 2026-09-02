package nav

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/sequence"
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
			if diff := cmp.Diff(tt.want, m.PathNeighbors(tt.at)); diff != "" {
				t.Errorf("PathNeighbors(%q) mismatch (-want +got):\n%s", tt.at, diff)
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

// TestTheWalkNumbersWhatItPlans holds the walk's numbering, on the walk itself
// rather than on any consumer: the main line counts on through every primary
// branch a structural wrapper carries, in document order; a side branch counts
// its own rows from one without displacing the main line; a planned row that
// resolves to nothing keeps its place on the line; and a row in a branch the
// walk never reaches — declared out, never declared, structurally broken, or
// nested beneath a side branch — keeps number zero. It would catch a
// structural wrapper dropping its children off the line, a side branch seeded
// from the main count, and a walkless branch handing out numbers.
func TestTheWalkNumbersWhatItPlans(t *testing.T) {
	t.Parallel()

	// L03 is deliberately absent from the resolver: a planned, unwritten
	// lesson. X01 sits in a branch nested beneath the side branch, which the
	// walk never enters; V01 sits in a branch whose duplicate declaration is
	// a structural error.
	idx := resolver(t,
		"Writing/L01.md", "Writing/L02.md", "Writing/L04.md",
		"Writing/S01.md", "Writing/S02.md", "Writing/X01.md",
		"Writing/R01.md", "Writing/U01.md", "Writing/V01.md")
	body := "## Part\n\n" +
		"### A {sequence=primary}\n\n" +
		"- [[L01]]\n" +
		"- [[L02]]\n" +
		"\t- 支線 {sequence=local}\n" +
		"\t\t- [[S01]]\n" +
		"\t\t\t- 深入 {sequence=primary}\n" +
		"\t\t\t\t- [[X01]]\n" +
		"\t\t- [[S02]]\n" +
		"\n### B {sequence=primary}\n\n" +
		"- [[L03]]\n" +
		"- [[L04]]\n" +
		"\n## 日常 {sequence=none}\n\n- [[R01]]\n" +
		"\n## 忘了宣告\n\n- [[U01]]\n" +
		"\n## 壞 {sequence=primary} {sequence=local}\n\n- [[V01]]\n"
	p := buildPath(pathNote("Maps/Course.md", "Course", body), idx, nil, testArtifactPolicy(t))

	want := map[string]int{
		"L01": 1, "L02": 2, "L03": 3, "L04": 4,
		"S01": 1, "S02": 2,
		"X01": 0, "R01": 0, "U01": 0, "V01": 0,
	}
	got := map[string]int{}
	var collect func(groups []*PathGroup)
	collect = func(groups []*PathGroup) {
		for _, g := range groups {
			for _, item := range g.Items {
				switch {
				case item.Entry != nil:
					got[item.Entry.Text] = item.Entry.Number
				case item.Group != nil:
					collect([]*PathGroup{item.Group})
				}
			}
		}
	}
	collect(p.Groups)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("walk numbering mismatch (-want +got):\n%s", diff)
	}

	// The numbering rides the same walk the reader's arrow follows, so the
	// openable projection must be unchanged beside it: the unwritten L03 is
	// counted but not walkable.
	if p.Planned != 4 {
		t.Errorf("Planned = %d, want 4: the planned line counts its unwritten lesson", p.Planned)
	}
	wantComponents := [][]NoteRef{
		{
			{Name: "L01", RelPath: "Writing/L01.md"},
			{Name: "L02", RelPath: "Writing/L02.md"},
			{Name: "L04", RelPath: "Writing/L04.md"},
		},
		{
			{Name: "S01", RelPath: "Writing/S01.md"},
			{Name: "S02", RelPath: "Writing/S02.md"},
		},
	}
	if diff := cmp.Diff(wantComponents, p.components); diff != "" {
		t.Errorf("components mismatch (-want +got):\n%s", diff)
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

// TestAPathCarriesTheGrammarsDiagnostics holds the parse's report on the
// path it came from. Home tells a course that plans nothing from one whose
// structure could not be read by exactly this, and the course page decides
// its empty-state wording by it — dropped here, both surfaces would call
// every broken course an empty one. The clone hands out its own copy, so a
// holder cannot reach what other requests are reading.
func TestAPathCarriesTheGrammarsDiagnostics(t *testing.T) {
	t.Parallel()

	idx := resolver(t, "Writing/L01.md")
	body := "## Part\n\n- [[L01]]\n"
	p := buildPath(pathNote("Maps/Course.md", "Course", body), idx, nil, testArtifactPolicy(t))

	if len(p.Diagnostics) == 0 {
		t.Fatal("an undeclared branch with rows left no diagnostic on the path")
	}
	if p.Diagnostics[0].Rule != sequence.RuleRoleMissing {
		t.Errorf("Diagnostics[0].Rule = %q, want %q", p.Diagnostics[0].Rule, sequence.RuleRoleMissing)
	}

	cloned := p.clone()
	cloned.Diagnostics[0].Rule = "written-by-a-holder"
	if p.Diagnostics[0].Rule != sequence.RuleRoleMissing {
		t.Error("a clone's diagnostics alias the original's")
	}
}
