package nav

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestACourseNamesAResolvedRowByAliasThenTitleThenLinkText holds the ruling on
// what a course prints for a lesson, and that the walk hands the same name to
// the steps on either side of it. An alias that repeats the target is still an
// alias. A link to a heading or block keeps its link text, so two rows to one
// note name different places. A row that resolved to nothing keeps its link
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
		"5. [[last|last]]\n" +
		"6. [[titled#Part A]]\n" +
		"7. [[titled#^block-b]]\n" +
		"8. [[titled|]]\n" +
		"9. [[Writing/untitled|]]\n"
	facts := map[string]noteFacts{
		"Writing/aliased.md": {title: "Aliased: the title"},
		"Writing/titled.md":  {title: "Titled: the title", language: "ja"},
		"Writing/last.md":    {title: "Last: the title"},
	}
	p := buildPath(pathNote("Maps/Course.md", "Course", body), idx, facts, testArtifactPolicy(t))

	var got []string
	for _, item := range p.Groups[0].Items {
		got = append(got, item.Entry.Name)
	}
	want := []string{
		"My words", "Titled: the title", "Writing/untitled", "not written", "last",
		// Two rows to one note name two places, so neither takes the title.
		"titled#Part A", "titled#^block-b",
		// A separator with nothing after it wrote no alias: the note's title,
		// else the target.
		"Titled: the title", "Writing/untitled",
	}
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
