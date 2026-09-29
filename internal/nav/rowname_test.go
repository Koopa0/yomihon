package nav

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestACourseNamesAResolvedRowByAliasThenTitleThenLinkText holds the ruling on
// what a course prints for a lesson, and that the walk hands the same name to
// the steps on either side of it. An alias that repeats the target is still an
// alias. A row that resolved to nothing keeps its link
// text, and a title the note does not declare is not read off its file name.
func TestACourseNamesAResolvedRowByAliasThenTitleThenLinkText(t *testing.T) {
	t.Parallel()

	idx := resolver(t,
		"Writing/aliased.md", "Writing/titled.md", "Writing/untitled.md", "Writing/last.md")
	body := "## Lessons {sequence=primary}\n\n" +
		"1. [[aliased|My words]]\n" +
		"2. [[titled]]\n" +
		"3. [[Writing/untitled]]\n" +
		"4. [[not written]]\n" +
		"5. [[last|last]]\n"
	titles := map[string]string{
		"Writing/aliased.md": "Aliased: the title",
		"Writing/titled.md":  "Titled: the title",
		"Writing/last.md":    "Last: the title",
	}
	langs := map[string]string{"Writing/titled.md": "ja"}
	p := buildPath(pathNote("Maps/Course.md", "Course", body), idx, nil, langs, titles, testArtifactPolicy(t))

	var got []string
	for _, item := range p.Groups[0].Items {
		got = append(got, item.Entry.Name)
	}
	want := []string{"My words", "Titled: the title", "Writing/untitled", "not written", "last"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("row names (-want +got):\n%s", diff)
	}

	m := &Model{paths: []Path{p}}
	steps := m.PathNeighbors("Writing/untitled.md")
	if len(steps) != 1 {
		t.Fatalf("PathNeighbors(untitled) = %d answers, want 1", len(steps))
	}
	wantPrev := NoteRef{Name: "Titled: the title", RelPath: "Writing/titled.md", Language: "ja"}
	wantNext := NoteRef{Name: "last", RelPath: "Writing/last.md"}
	if diff := cmp.Diff([]NoteRef{wantPrev, wantNext}, []NoteRef{steps[0].Prev, steps[0].Next}); diff != "" {
		t.Errorf("steps beside the untitled lesson (-want +got):\n%s", diff)
	}
}
