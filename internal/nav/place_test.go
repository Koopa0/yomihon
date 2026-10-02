package nav

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// placeModel builds one path from a body written the way an author writes it
// and wraps it in the model the page asks.
func placeModel(t *testing.T, body string, written ...string) (*Model, *Path) {
	t.Helper()
	paths := make([]string, 0, len(written))
	for _, name := range written {
		paths = append(paths, "Writing/"+name+".md")
	}
	p := buildPath(pathNote("Maps/Course.md", "Course", body), resolver(t, paths...), nil, testArtifactPolicy(t))
	return &Model{paths: []Path{p}}, &p
}

func lesson(name string) NoteRef {
	return NoteRef{Name: name, RelPath: "Writing/" + name + ".md"}
}

const coursePath = "Maps/Course.md"

// TestABranchLessonKnowsItsWayBackAndItsWayOn holds what a side branch hands a
// reader at its two ends, over the shapes it can be written in. Each case is a
// different way the answer could be wrong: a branch of one lesson owes both
// ends, a longer one owes its first lesson the way back and its last the way on
// and its middle lessons neither, a branch hung from the course's last lesson
// has nothing to go on to but the contents, and a lesson planned and not yet
// written is neither a place to go back to nor one to go on to.
func TestABranchLessonKnowsItsWayBackAndItsWayOn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		written []string
		at      string
		want    Place
	}{
		{
			name: "a branch of one lesson owes both ends",
			body: "## 啟動 {sequence=primary}\n\n- [[G01]]\n- [[G02]]\n\n## 取消 {sequence=primary}\n\n" +
				"- [[G04]]\n\t- 要限制等待時間時 {sequence=local}\n\t\t- [[G06]]\n- [[G05]]\n",
			written: []string{"G01", "G02", "G04", "G05", "G06"},
			at:      "G06",
			want:    Place{Part: "取消", OnBranch: true, Back: lesson("G04"), Onward: lesson("G05")},
		},
		{
			name:    "the first lesson of a longer branch goes back only",
			body:    "## 主線 {sequence=primary}\n\n- [[G01]]\n- [[G02]]\n\t- 支線 {sequence=local}\n\t\t- [[S1]]\n\t\t- [[S2]]\n\t\t- [[S3]]\n- [[G03]]\n",
			written: []string{"G01", "G02", "G03", "S1", "S2", "S3"},
			at:      "S1",
			want:    Place{Part: "主線", OnBranch: true, Back: lesson("G02")},
		},
		{
			name:    "a middle lesson of a longer branch goes neither way",
			body:    "## 主線 {sequence=primary}\n\n- [[G01]]\n- [[G02]]\n\t- 支線 {sequence=local}\n\t\t- [[S1]]\n\t\t- [[S2]]\n\t\t- [[S3]]\n- [[G03]]\n",
			written: []string{"G01", "G02", "G03", "S1", "S2", "S3"},
			at:      "S2",
			want:    Place{Part: "主線", OnBranch: true},
		},
		{
			name:    "the last lesson of a longer branch goes on only",
			body:    "## 主線 {sequence=primary}\n\n- [[G01]]\n- [[G02]]\n\t- 支線 {sequence=local}\n\t\t- [[S1]]\n\t\t- [[S2]]\n\t\t- [[S3]]\n- [[G03]]\n",
			written: []string{"G01", "G02", "G03", "S1", "S2", "S3"},
			at:      "S3",
			want:    Place{Part: "主線", OnBranch: true, Onward: lesson("G03")},
		},
		{
			name:    "a branch hung from the last lesson goes on to the contents",
			body:    "## 主線 {sequence=primary}\n\n- [[G01]]\n- [[G02]]\n\t- 支線 {sequence=local}\n\t\t- [[S1]]\n",
			written: []string{"G01", "G02", "S1"},
			at:      "S1",
			want:    Place{Part: "主線", OnBranch: true, Back: lesson("G02"), ToContents: true},
		},
		{
			name:    "a lesson planned and not written is not a place to go back to, and the way on passes it",
			body:    "## 主線 {sequence=primary}\n\n- [[G01]]\n- [[Nobody wrote this]]\n\t- 支線 {sequence=local}\n\t\t- [[S1]]\n- [[G03]]\n",
			written: []string{"G01", "G03", "S1"},
			at:      "S1",
			want:    Place{Part: "主線", OnBranch: true, Onward: lesson("G03")},
		},
		{
			name:    "the way on is the next lesson that can be opened and not the next row",
			body:    "## 主線 {sequence=primary}\n\n- [[G01]]\n- [[G02]]\n\t- 支線 {sequence=local}\n\t\t- [[S1]]\n- [[Nobody wrote this]]\n- [[G03]]\n",
			written: []string{"G01", "G02", "G03", "S1"},
			at:      "S1",
			want:    Place{Part: "主線", OnBranch: true, Back: lesson("G02"), Onward: lesson("G03")},
		},
		{
			name:    "a branch hung from an unwritten last lesson has neither a place back nor one on",
			body:    "## 主線 {sequence=primary}\n\n- [[G01]]\n- [[Nobody wrote this]]\n\t- 支線 {sequence=local}\n\t\t- [[S1]]\n",
			written: []string{"G01", "S1"},
			at:      "S1",
			want:    Place{Part: "主線", OnBranch: true, ToContents: true},
		},
		{
			name:    "a branch a heading opened hangs from nothing and goes on to the contents",
			body:    "## 主線 {sequence=primary}\n\n- [[G01]]\n\n### 選修 {sequence=local}\n\n- [[S1]]\n",
			written: []string{"G01", "S1"},
			at:      "S1",
			want:    Place{Part: "主線", OnBranch: true, ToContents: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m, _ := placeModel(t, tt.body, tt.written...)
			got, found := m.PathPlace("Writing/"+tt.at+".md", coursePath)
			if !found {
				t.Fatalf("PathPlace(%s) did not find the note, so the answer below proves nothing", tt.at)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("PathPlace(%s) (-want +got):\n%s", tt.at, diff)
			}
		})
	}
}

// TestALessonKnowsTheBranchesHangingFromIt holds the aside a foot offers beside
// its own steps: the first lesson of each branch nested under the row, in the
// order they were written, found for the row it is under and not for a row that
// happens to name the same note.
func TestALessonKnowsTheBranchesHangingFromIt(t *testing.T) {
	t.Parallel()

	m, _ := placeModel(t, "## 主線 {sequence=primary}\n\n- [[G01]]\n- [[G02]]\n"+
		"\t- 一 {sequence=local}\n\t\t- [[S1]]\n\t\t- [[S2]]\n"+
		"\t- 二 {sequence=local}\n\t\t- [[T1]]\n"+
		"- [[G03]]\n- [[G03]]\n\t- 三 {sequence=local}\n\t\t- [[U1]]\n",
		"G01", "G02", "G03", "S1", "S2", "T1", "U1")

	tests := []struct {
		at   string
		want []NoteRef
	}{
		{"G01", nil},
		{"G02", []NoteRef{lesson("S1"), lesson("T1")}},
		// Listed twice, hung from the second row: the note carries the aside
		// whichever of its rows the reader came by.
		{"G03", []NoteRef{lesson("U1")}},
		{"S1", nil},
	}
	for _, tt := range tests {
		got, found := m.PathPlace("Writing/"+tt.at+".md", coursePath)
		if !found {
			t.Fatalf("PathPlace(%s) did not find the note", tt.at)
		}
		if diff := cmp.Diff(tt.want, got.Asides); diff != "" {
			t.Errorf("Asides of %s (-want +got):\n%s", tt.at, diff)
		}
	}
	for _, branch := range []string{"S1", "T1", "U1"} {
		got, _ := m.PathPlace("Writing/"+branch+".md", coursePath)
		if len(got.Asides) != 0 {
			t.Errorf("%s, a lesson of a side branch, carries asides %v", branch, got.Asides)
		}
	}
}

// TestABranchBuildsNothingIntoTheMainLinesStepsOrCounts holds the owner's
// invariant by comparing the same course written with and without the branch:
// the main line's own previous and next, its total, and where it stands in the
// path's membership are the same, so what a branch offers cannot have become a
// step or a count.
func TestABranchBuildsNothingIntoTheMainLinesStepsOrCounts(t *testing.T) {
	t.Parallel()

	const without = "## 啟動 {sequence=primary}\n\n- [[G01]]\n- [[G02]]\n- [[G03]]\n\n## 取消 {sequence=primary}\n\n- [[G04]]\n- [[G05]]\n"
	const with = "## 啟動 {sequence=primary}\n\n- [[G01]]\n- [[G02]]\n- [[G03]]\n\n## 取消 {sequence=primary}\n\n" +
		"- [[G04]]\n\t- 要限制等待時間時 {sequence=local}\n\t\t- [[G06]]\n- [[G05]]\n"
	bare, barePath := placeModel(t, without, "G01", "G02", "G03", "G04", "G05")
	branched, branchedPath := placeModel(t, with, "G01", "G02", "G03", "G04", "G05", "G06")

	for _, name := range []string{"G01", "G02", "G03", "G04", "G05"} {
		rel := "Writing/" + name + ".md"
		if diff := cmp.Diff(bare.PathNeighbors(rel), branched.PathNeighbors(rel)); diff != "" {
			t.Errorf("a branch moved the steps of %s (-without +with):\n%s", name, diff)
		}
	}
	if next := branched.PathNeighbors("Writing/G04.md")[0].Next; next != lesson("G05") {
		t.Errorf("the lesson the branch hangs from steps on to %v, want G05", next)
	}
	if prev := branched.PathNeighbors("Writing/G05.md")[0].Prev; prev != lesson("G04") {
		t.Errorf("the lesson after the branch steps back to %v, want G04", prev)
	}
	if barePath.Planned != branchedPath.Planned || barePath.Unsettled != branchedPath.Unsettled {
		t.Errorf("a branch moved the course's counts: planned %d and %d, unsettled %d and %d",
			barePath.Planned, branchedPath.Planned, barePath.Unsettled, branchedPath.Unsettled)
	}
	if branchedPath.Branched != 1 || barePath.Branched != 0 {
		t.Errorf("Branched = %d with the branch and %d without, want 1 and 0", branchedPath.Branched, barePath.Branched)
	}

	// The branch lesson steps nowhere in the course, and the path still teaches
	// the notes it taught before and one more.
	steps := branched.PathNeighbors("Writing/G06.md")
	if len(steps) != 1 || steps[0].Prev.RelPath != "" || steps[0].Next.RelPath != "" {
		t.Errorf("the branch's only lesson steps to %+v, want nowhere", steps)
	}
}

// TestPlacesOfNotesThePathDoesNotWalk keeps the question honest about what it
// is asked: a note the path does not walk, a path that does not exist, and no
// model at all.
func TestPlacesOfNotesThePathDoesNotWalk(t *testing.T) {
	t.Parallel()

	m, _ := placeModel(t, "## 主線 {sequence=primary}\n\n- [[G01]]\n", "G01", "Other")
	if got, found := m.PathPlace("Writing/Other.md", coursePath); found || got.Part != "" || got.OnBranch {
		t.Errorf("a note the path does not list has a place: %+v, found = %t", got, found)
	}
	if _, found := m.PathPlace("Writing/G01.md", "Maps/Nowhere.md"); found {
		t.Error("a path that does not exist placed a note")
	}
	if _, found := m.PathPlace("", coursePath); found {
		t.Error("an empty address was placed")
	}
	var none *Model
	if _, found := none.PathPlace("Writing/G01.md", coursePath); found {
		t.Error("a nil model placed a note")
	}
}

// TestAPartIsTheTopLevelBranchAStopSitsUnder holds the name a running head
// prints: the top-level branch, whatever it is nested in beneath it, and the
// same part for a side branch's lessons as for the lesson they hang from.
func TestAPartIsTheTopLevelBranchAStopSitsUnder(t *testing.T) {
	t.Parallel()

	m, _ := placeModel(t, "## 第一章\n\n### 模組甲 {sequence=primary}\n\n- [[G01]]\n- [[G02]]\n\t- 支線 {sequence=local}\n\t\t- [[S1]]\n"+
		"\n## 第二章 {sequence=primary}\n\n- [[G03]]\n", "G01", "G02", "G03", "S1")
	for name, want := range map[string]string{"G01": "第一章", "G02": "第一章", "S1": "第一章", "G03": "第二章"} {
		got, found := m.PathPlace("Writing/"+name+".md", coursePath)
		if !found || got.Part != want {
			t.Errorf("Part of %s = %q (found %t), want %q", name, got.Part, found, want)
		}
	}
}
