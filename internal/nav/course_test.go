package nav

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/sequence"
)

// TestTheWorkedCourseReadsAsWritten drives one tracked course end to end: the
// Markdown a person would write, through the grammar, into the model every
// surface reads. Its first two parts are the authoring contract's own worked
// example, whose result the contract states in prose; the third part is every
// row shape the grammar refuses, gathered under one undeclared heading so a
// reader can see what each looks like beside what it should have been.
func TestTheWorkedCourseReadsAsWritten(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("testdata/course.md")
	if err != nil {
		t.Fatalf("read course fixture: %v", err)
	}
	note := vaultNote(t, string(body))
	idx := resolver(t,
		"Writing/L01 器材認識.md", "Writing/L02 咖啡豆基礎.md", "Writing/L03 研磨.md",
		"Writing/L04 水溫.md", "Writing/L05 注水手法.md", "Writing/L06 比例與時間.md",
		"Writing/L07 品飲.md",
		"Writing/磨豆機校正基礎.md", "Writing/粒徑分布判讀.md", "Writing/校正實作.md",
		"Writing/注水練習.md", "Writing/沖煮記錄.md",
	)
	p := buildPath(note, idx, nil, testArtifactPolicy(t))

	// "Home reads 8 課" — the main line, the unwritten eighth lesson included.
	if p.Planned != 8 {
		t.Errorf("Planned = %d, want 8; the contract's own example says the course is eight lessons", p.Planned)
	}

	// "The side branch shows as three, hanging under L03."
	side := findLocal(p.Groups)
	if side == nil {
		t.Fatalf("the course has no side branch: %+v", p.Groups)
	}
	if side.Planned != 3 {
		t.Errorf("side branch lists %d lessons, want 3", side.Planned)
	}
	if side.AnchorTarget != "L03 研磨" {
		t.Errorf("side branch hangs from %q, want %q", side.AnchorTarget, "L03 研磨")
	}

	m := &Model{paths: []Path{p}}
	ref := func(name, rel string) NoteRef { return NoteRef{Name: name, RelPath: rel} }

	// "L04 follows L03, because the container is L03's child and L04 is its
	// sibling."
	wantL03 := []Neighbors{{
		PathTitle: "手沖咖啡", PathRelPath: "Maps/Course.md",
		Prev: ref("L02 咖啡豆基礎", "Writing/L02 咖啡豆基礎.md"),
		Next: ref("L04 水溫", "Writing/L04 水溫.md"),
	}}
	if diff := cmp.Diff(wantL03, m.PathNeighbors("Writing/L03 研磨.md")); diff != "" {
		t.Errorf("the main line steps into the side branch (-want +got):\n%s", diff)
	}

	// "L07 has no next lesson, because L08 is unwritten."
	wantL07 := []Neighbors{{
		PathTitle: "手沖咖啡", PathRelPath: "Maps/Course.md",
		Prev: ref("L06 比例與時間", "Writing/L06 比例與時間.md"),
	}}
	if diff := cmp.Diff(wantL07, m.PathNeighbors("Writing/L07 品飲.md")); diff != "" {
		t.Errorf("a trailing planned lesson gave the one before it a next (-want +got):\n%s", diff)
	}

	// "The routine block is absent from navigation and reads normally on the
	// page." Nothing it lists is a course membership.
	index := make(map[string][]Placement)
	pathPlacements(index, &p)
	for _, rel := range []string{"Writing/注水練習.md", "Writing/沖煮記錄.md"} {
		if got := index[rel]; len(got) != 0 {
			t.Errorf("the routine block placed %s in the course: %v", rel, got)
		}
	}

	// The third part is undeclared, so nothing in it projects however it is
	// written — and each row shape is refused for its own stated reason.
	doc := sequence.Parse(string(body), note.BodyLine)
	wantRules := map[sequence.Rule]int{
		sequence.RuleRoleMissing:       2, // the undeclared part, and the nested list inside it
		sequence.RuleEntryNoncanonical: 1, // "補充：[[L01]]" does not open with its link
		sequence.RuleEntryMultiTarget:  1, // a row naming two lessons
	}
	got := make(map[sequence.Rule]int)
	for _, d := range doc.Diagnostics {
		got[d.Rule]++
	}
	if diff := cmp.Diff(wantRules, got); diff != "" {
		t.Errorf("the course's diagnostics mismatch (-want +got):\n%s", diff)
	}

	// A checkbox row tracks whether something was done. It is never a lesson,
	// anywhere in the course, however the branch around it is declared.
	for _, g := range p.Groups {
		if listsCheckboxRow(g) {
			t.Error("a task row became a lesson of the course")
		}
	}
}

// listsCheckboxRow reports whether any accepted row of this branch or its
// descendants came from the fixture's task line.
func listsCheckboxRow(g *PathGroup) bool {
	for _, item := range g.Items {
		switch {
		case item.Entry != nil:
			if item.Entry.State == sequence.EntryAccepted && item.Entry.Line == 30 {
				return true
			}
		case item.Group != nil:
			if listsCheckboxRow(item.Group) {
				return true
			}
		}
	}
	return false
}

// findLocal is the course's one declared side branch, wherever it hangs.
func findLocal(groups []*PathGroup) *PathGroup {
	for _, g := range groups {
		if g.Role == sequence.RoleLocal {
			return g
		}
		for _, item := range g.Items {
			if item.Group == nil {
				continue
			}
			if found := findLocal([]*PathGroup{item.Group}); found != nil {
				return found
			}
		}
	}
	return nil
}
