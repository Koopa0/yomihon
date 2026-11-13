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

// TestAMapNamesAResolvedRowByTheSameRuleAsACourse holds that a map row is named
// by the rule a course row is: alias, then the note's declared title, then the
// link text, with a heading or block link keeping its link text and an empty
// display falling back to the target. The name carries the language the note
// declared. A row that resolves to nothing is not drawn.
func TestAMapNamesAResolvedRowByTheSameRuleAsACourse(t *testing.T) {
	t.Parallel()

	idx := resolver(t, "Writing/aliased.md", "Writing/titled.md", "Writing/untitled.md")
	body := "## Notes\n\n" +
		"- [[aliased|My words]]\n" +
		"- [[titled]]\n" +
		"- [[Writing/untitled]]\n" +
		"- [[not written]]\n" +
		"- [[titled#Part A]]\n" +
		"- [[titled#^block-b]]\n" +
		"- [[titled|]]\n" +
		"- [[Writing/untitled|]]\n" +
		"- [[titled|titled]]\n"
	facts := map[string]noteFacts{
		"Writing/aliased.md": {title: "Aliased: the title"},
		"Writing/titled.md":  {title: "Titled: the title", language: "ja"},
	}
	branches := parseBranches(body, idx, facts, testArtifactPolicy(t))
	if len(branches) != 1 {
		t.Fatalf("branches = %d, want 1", len(branches))
	}
	type row struct{ Name, Language string }
	var got []row
	for _, e := range branches[0].Entries {
		got = append(got, row{e.Name, e.Language})
	}
	want := []row{
		{"My words", ""},
		{"Titled: the title", "ja"},
		{"Writing/untitled", ""},
		{"titled#Part A", "ja"},
		{"titled#^block-b", "ja"},
		{"Titled: the title", "ja"},
		{"Writing/untitled", ""},
		{"titled", "ja"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("map row names (-want +got):\n%s", diff)
	}
}

// TestRowsAndShelvesAreNamedByTheFirstHeading reads a vault end to end: a
// lesson that opens on a level-one heading is listed by it, a lesson with only
// a frontmatter title is listed by its link text, a course and a map are
// titled by their own headings, and the recent-notes summary names a note the
// same way.
func TestRowsAndShelvesAreNamedByTheFirstHeading(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNavFixture(t, root, "Concepts/headed.md", "---\ntype: concept\n---\n# Heading words\n\nbody\n")
	writeNavFixture(t, root, "Concepts/written.md", "---\ntype: concept\ntitle: Written words\n---\nbody\n")
	writeNavFixture(t, root, "Maps/course-file.md", "---\ntype: study-path\n---\n# The course\n\n## Lessons {sequence=primary}\n\n1. [[headed]]\n2. [[written]]\n")
	writeNavFixture(t, root, "Maps/map-file.md", "---\ntype: moc\n---\n# The map\n\n## Notes\n\n- [[headed]]\n- [[written]]\n")
	model := capturedModelWithDomains(t, root, nil)

	paths := model.Paths()
	if len(paths) != 1 {
		t.Fatalf("Paths() = %d, want 1", len(paths))
	}
	if paths[0].Title != "The course" {
		t.Errorf("course title = %q, want its heading", paths[0].Title)
	}
	var rows []string
	for _, item := range paths[0].Groups[0].Items {
		rows = append(rows, item.Entry.Name)
	}
	if diff := cmp.Diff([]string{"Heading words", "written"}, rows); diff != "" {
		t.Errorf("course rows (-want +got):\n%s", diff)
	}

	maps := model.Maps()
	if len(maps) != 1 {
		t.Fatalf("Maps() = %d, want 1", len(maps))
	}
	if maps[0].Title != "The map" {
		t.Errorf("map title = %q, want its heading", maps[0].Title)
	}
	var mapRows []string
	for _, entry := range maps[0].Branches[0].Entries {
		mapRows = append(mapRows, entry.Name)
	}
	if diff := cmp.Diff([]string{"Heading words", "written"}, mapRows); diff != "" {
		t.Errorf("map rows (-want +got):\n%s", diff)
	}

	summaries := map[string]string{}
	for _, n := range model.KnowledgeNotes() {
		summaries[n.RelPath] = n.Title
	}
	want := map[string]string{
		"Concepts/headed.md":  "Heading words",
		"Concepts/written.md": "written",
		"Maps/course-file.md": "The course",
		"Maps/map-file.md":    "The map",
	}
	if diff := cmp.Diff(want, summaries); diff != "" {
		t.Errorf("KnowledgeNotes titles (-want +got):\n%s", diff)
	}
}
