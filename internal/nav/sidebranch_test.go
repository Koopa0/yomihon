package nav

import (
	"testing"
)

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
