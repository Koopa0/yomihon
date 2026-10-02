package nav

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

// typeFacts is a fixture's declared note types as the per-note facts a builder
// reads. Nothing else about a note matters to which noun a path is read in.
func typeFacts(types map[string]string) map[string]noteFacts {
	facts := make(map[string]noteFacts, len(types))
	for rel, noteType := range types {
		facts[rel] = noteFacts{noteType: noteType}
	}
	return facts
}

// TestAPathIsReadInTheNounItsEntriesAre holds the one decision every surface
// asks: a path is counted and stepped in lessons until a row of it resolves to
// a note that is not one, and then in items. The table is the complete set of
// ways a path's rows can bear on that — every one of them a lesson, a note of
// another type among them, a row that resolved to nothing, a row only the
// author's reference section lists, a side branch's rows — so a rule that asked
// the wrong rows is wrong in a different case each.
func TestAPathIsReadInTheNounItsEntriesAre(t *testing.T) {
	t.Parallel()

	roles, _ := testCapabilities(t)
	types := typeFacts(map[string]string{
		"Writing/L01.md": "lesson",
		"Writing/L02.md": "lesson",
		"Writing/L03.md": "lesson",
		"Concepts/Q1.md": "concept",
		"Concepts/Q2.md": "concept",
		"Concepts/Q3.md": "concept",
	})
	idx := resolver(t,
		"Writing/L01.md", "Writing/L02.md", "Writing/L03.md",
		"Concepts/Q1.md", "Concepts/Q2.md", "Concepts/Q3.md")

	tests := []struct {
		name  string
		roles schema.NavigationRoles
		body  string
		want  Unit
	}{
		{
			name:  "every row is a lesson",
			roles: roles,
			body:  "## 主線 {sequence=primary}\n\n- [[L01]]\n- [[L02]]\n- [[L03]]\n",
			want:  UnitLesson,
		},
		{
			name:  "every row is a note that is not a lesson",
			roles: roles,
			body:  "## 主線 {sequence=primary}\n\n- [[Q1]]\n- [[Q2]]\n- [[Q3]]\n",
			want:  UnitItem,
		},
		{
			name:  "one row among the lessons is not a lesson",
			roles: roles,
			body:  "## 主線 {sequence=primary}\n\n- [[L01]]\n- [[L02]]\n- [[Q1]]\n",
			want:  UnitItem,
		},
		{
			name:  "a lesson planned and not yet written does not rename the book",
			roles: roles,
			body:  "## 主線 {sequence=primary}\n\n- [[L01]]\n- [[L02]]\n- [[Nobody wrote this one]]\n",
			want:  UnitLesson,
		},
		{
			// A course being planned is still a course: nothing it lists has
			// shown itself to be anything else.
			name:  "a path none of whose rows resolved is read in lessons",
			roles: roles,
			body:  "## 主線 {sequence=primary}\n\n- [[Nobody wrote this one]]\n",
			want:  UnitLesson,
		},
		{
			name:  "a path with no rows is read in lessons",
			roles: roles,
			body:  "Only prose.\n",
			want:  UnitLesson,
		},
		{
			name:  "a reference section the author kept out of the course is not asked",
			roles: roles,
			body:  "## 主線 {sequence=primary}\n\n- [[L01]]\n- [[L02]]\n\n## 查閱 {sequence=none}\n\n- [[Q1]]\n- [[Q2]]\n",
			want:  UnitLesson,
		},
		{
			name:  "a side branch's rows are counted too",
			roles: roles,
			body:  "## 主線 {sequence=primary}\n\n- [[L01]]\n\t- 支線 {sequence=local}\n\t\t- [[Q1]]\n- [[L02]]\n",
			want:  UnitItem,
		},
		{
			name:  "a heading nobody declared teaches nothing and is not asked",
			roles: roles,
			body:  "## 主線 {sequence=primary}\n\n- [[L01]]\n\n## 忘了宣告\n\n- [[Q1]]\n",
			want:  UnitLesson,
		},
		{
			name:  "a contract that names no lesson type has no path of lessons",
			roles: schema.NavigationRoles{},
			body:  "## 主線 {sequence=primary}\n\n- [[L01]]\n- [[L02]]\n",
			want:  UnitItem,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := buildPath(pathNote("Maps/Course.md", "Course", tt.body), idx, types, testArtifactPolicy(t))
			if got := unitOf(p.Groups, types, tt.roles); got != tt.want {
				t.Errorf("unitOf() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestTheModelDecidesEachPathsNounOnceFromItsOwnRows reads two paths through
// the whole construction: the same call a running server makes. It is the check
// that the decision is wired in at all, since the table above calls the
// decision directly, and that the answer reaches the steps a note's page
// offers, which read it off Neighbors rather than off the path.
func TestTheModelDecidesEachPathsNounOnceFromItsOwnRows(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for rel, content := range map[string]string{
		"Writing/L01.md":     "---\ntype: lesson\n---\nbody\n",
		"Writing/L02.md":     "---\ntype: lesson\n---\nbody\n",
		"Concepts/Queue.md":  "---\ntype: concept\n---\nbody\n",
		"Concepts/Worker.md": "---\ntype: concept\n---\nbody\n",
		"Maps/Lessons.md":    "---\ntype: study-path\n---\n## 主線 {sequence=primary}\n\n- [[L01]]\n- [[L02]]\n",
		"Maps/QueueKit.md":   "---\ntype: study-path\n---\n## 主線 {sequence=primary}\n\n- [[Queue]]\n- [[Worker]]\n",
	} {
		writeNavFixture(t, root, rel, content)
	}
	roles, policy := testCapabilities(t)
	contract := testContract(t)
	model := capturedModel(t, root, roles, contract.KnowledgeScope(), policy, nil)

	units := map[string]Unit{}
	for _, p := range model.Paths() {
		units[p.RelPath] = p.Unit
	}
	if diff := cmp.Diff(map[string]Unit{"Maps/Lessons.md": UnitLesson, "Maps/QueueKit.md": UnitItem}, units); diff != "" {
		t.Errorf("Path.Unit by path (-want +got):\n%s", diff)
	}
	for rel, want := range map[string]Unit{"Writing/L01.md": UnitLesson, "Concepts/Queue.md": UnitItem} {
		steps := model.PathNeighbors(rel)
		if len(steps) != 1 {
			t.Fatalf("PathNeighbors(%q) = %d answers, want 1", rel, len(steps))
		}
		if steps[0].Unit != want {
			t.Errorf("PathNeighbors(%q) unit = %v, want %v", rel, steps[0].Unit, want)
		}
	}
}

// TestASideBranchIsCountedBesideTheCourseAndNeverInIt pins the figure the
// cover prints after the course total. It counts the rows the projectable side
// branches list, the row nobody wrote included, because a branch plans the same
// way a course does; it leaves out a reference section and an undeclared
// heading, which are not branches; and it never moves Planned or the walk.
func TestASideBranchIsCountedBesideTheCourseAndNeverInIt(t *testing.T) {
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
		"\t\t- [[Nobody wrote this one]]\n" +
		"- [[L03]]\n" +
		"\n## 日常 {sequence=none}\n\n- [[R01]]\n" +
		"\n## 忘了宣告\n\n- [[U01]]\n"
	p := buildPath(pathNote("Maps/Course.md", "Course", body), idx, nil, testArtifactPolicy(t))

	if p.Branched != 3 {
		t.Errorf("Branched = %d, want 3: S01, S02 and the unwritten row, and nothing from the reference section or the undeclared heading", p.Branched)
	}
	if p.Planned != 3 {
		t.Errorf("Planned = %d, want 3: a side branch never joins the course's count", p.Planned)
	}
	if len(p.components) != 2 || len(p.components[0]) != 3 {
		t.Errorf("walk = %v, want the three main-line lessons and the branch's own sequence", p.components)
	}

	none := buildPath(pathNote("Maps/Plain.md", "Plain", "## 主線 {sequence=primary}\n\n- [[L01]]\n"), idx, nil, testArtifactPolicy(t))
	if none.Branched != 0 {
		t.Errorf("Branched = %d for a course with no side branch, want 0", none.Branched)
	}
}
