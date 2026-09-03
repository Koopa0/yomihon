package nav

import (
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/sequence"
)

// TestACourseCountsTheReadyLessonsItDraws holds the figure the course page's
// header prints. It counts across every branch a surface draws — the main line
// and the side branches beside it — and nothing from a block declared out of
// the course or one nobody declared at all, because neither is drawn and
// neither is part of the course to be counted into.
//
// The fixture puts a lesson at the reviewed status in each of the four places
// so the answer is wrong in a different way for each rule that could be
// dropped, and one lesson at that status is unwritten so resolution is
// required rather than assumed.
func TestACourseCountsTheReadyLessonsItDraws(t *testing.T) {
	t.Parallel()

	idx := resolver(t,
		"Writing/L01.md", "Writing/L02.md", "Writing/L03.md",
		"Writing/S01.md", "Writing/S02.md", "Writing/R01.md", "Writing/U01.md")
	status := map[string]string{
		"Writing/L01.md": schema.SealStatus,
		"Writing/L02.md": "draft",
		"Writing/L03.md": schema.SealStatus,
		"Writing/S01.md": schema.SealStatus,
		"Writing/S02.md": "draft",
		"Writing/R01.md": schema.SealStatus,
		"Writing/U01.md": schema.SealStatus,
	}
	body := "## 主線 {sequence=primary}\n\n" +
		"- [[L01]]\n" +
		"- [[L02]]\n" +
		"\t- 支線 {sequence=local}\n" +
		"\t\t- [[S01]]\n" +
		"\t\t- [[S02]]\n" +
		"- [[L03]]\n" +
		"- [[Nobody wrote this one]]\n" +
		"\n## 日常 {sequence=none}\n\n- [[R01]]\n" +
		"\n## 忘了宣告\n\n- [[U01]]\n"
	p := buildPath(pathNote("Maps/Course.md", "Course", body), idx, status, testArtifactPolicy(t))

	if p.Ready != 3 {
		t.Errorf("Ready = %d, want 3: L01 and L03 on the main line plus S01 on the side branch, and neither the block declared out of the course nor the branch nobody declared", p.Ready)
	}
	if p.Planned != 4 {
		t.Fatalf("Planned = %d, want 4; the fixture is not the course this test describes, so its Ready figure proves nothing", p.Planned)
	}
}

// TestABranchSaysWhetherASurfaceDrawsIt and its companion below pin the two
// answers the course page and the navigation rail both read, so neither has to
// re-derive one from Role and Projectable.
func TestABranchSaysWhetherASurfaceDrawsIt(t *testing.T) {
	t.Parallel()

	idx := resolver(t, "Writing/L01.md")
	body := "## 結構\n\n" +
		"### 主線 {sequence=primary}\n\n- [[L01]]\n" +
		"\n## 日常 {sequence=none}\n\n- [[L01]]\n"
	p := buildPath(pathNote("Maps/Course.md", "Course", body), idx, nil, testArtifactPolicy(t))

	if len(p.Groups) != 2 {
		t.Fatalf("groups = %d, want the structural heading and the block declared out of the course", len(p.Groups))
	}
	structural, declaredOut := p.Groups[0], p.Groups[1]
	if !structural.Drawn() {
		t.Error("a heading that carries a declared branch is not drawn, so its parts would be orphaned")
	}
	if declaredOut.Drawn() {
		t.Error("a block the author declared out of the course is drawn as part of it")
	}
	if len(structural.Items) != 1 || structural.Items[0].Group == nil {
		t.Fatalf("structural branch holds %d items, want one nested branch", len(structural.Items))
	}
	if !structural.Items[0].Group.Drawn() {
		t.Error("the declared branch under the heading is not drawn")
	}
}

func TestABranchSaysWhichRowsItTeaches(t *testing.T) {
	t.Parallel()

	idx := resolver(t, "Writing/L01.md")
	body := "## 主線 {sequence=primary}\n\n- [[L01]]\n" +
		"\n## 日常 {sequence=none}\n\n- [[L01]]\n"
	p := buildPath(pathNote("Maps/Course.md", "Course", body), idx, nil, testArtifactPolicy(t))

	taught := p.Groups[0]
	untaught := p.Groups[1]
	if taught.Items[0].Entry.State != sequence.EntryAccepted {
		t.Fatalf("the fixture's main-line row is %v, not an accepted one, so this test asserts nothing", taught.Items[0].Entry.State)
	}
	if !taught.Teaches(taught.Items[0].Entry) {
		t.Error("a row the grammar accepted on the main line is not one of the course's lessons")
	}
	if untaught.Teaches(untaught.Items[0].Entry) {
		t.Error("a row inside a block declared out of the course reads as one of its lessons")
	}
	if taught.Teaches(nil) {
		t.Error("a branch teaches a row that is not there")
	}
}

// TestAnUnresolvedLessonIsNeverReady states why the count asks about
// resolution as well as about the status. A status is a fact read off a note's
// own frontmatter, so today only a row that resolved to a note carries one and
// the two questions cannot come apart. The tree here is built by hand rather
// than parsed, because that is the only way to hand the count a row that
// carries a status and no note: a course still plans that lesson, and nothing
// has reviewed it.
func TestAnUnresolvedLessonIsNeverReady(t *testing.T) {
	t.Parallel()

	group := &PathGroup{
		Name:        "主線",
		Role:        sequence.RolePrimary,
		Projectable: true,
		Items: []PathItem{
			{Entry: &PathEntry{Text: "Written", State: sequence.EntryAccepted, Kind: EntryResolved, RelPath: "Writing/L01.md", Status: schema.SealStatus}},
			{Entry: &PathEntry{Text: "Planned", State: sequence.EntryAccepted, Kind: EntryUnresolved, Status: schema.SealStatus}},
		},
	}
	if got := readyLessons([]*PathGroup{group}); got != 1 {
		t.Errorf("readyLessons = %d, want 1; a row that reaches no note has no note to have been reviewed", got)
	}
}
