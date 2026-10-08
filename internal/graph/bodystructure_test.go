package graph_test

import (
	"iter"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
)

func TestBodyStructureOutlineAndRows(t *testing.T) {
	facts := graph.ReadBody("# Path\n\n- root\n  - child\n\n  tail\n\nend\n")
	wantOutline := []graph.BodyOutline{
		{Heading: graph.BodyOutlineHeading{Span: graph.Span{Start: 2, Stop: 6}, Level: 1}},
		{ListID: 1}, {StrayID: 1},
	}
	if diff := cmp.Diff(wantOutline, slices.Collect(facts.Outline())); diff != "" {
		t.Fatalf("caught: original outline differs (-want +got):\n%s", diff)
	}
	rows := slices.Collect(facts.ListRows(1))
	if len(rows) != 1 {
		t.Fatalf("caught: direct list rows = %d, want 1", len(rows))
	}
	root := rows[0]
	if root.ID != 1 || root.ListID != 1 || root.ParentRowID != 0 || root.ChildListID != 2 || !root.FirstBlockHasLines {
		t.Fatalf("caught: root relationships differ: %+v", root)
	}
	children := slices.Collect(facts.ListRows(root.ChildListID))
	if len(children) != 1 || children[0].ID != 2 || children[0].ListID != 2 || children[0].ParentRowID != 1 || children[0].ChildListID != 0 {
		t.Fatalf("caught: child relationships differ: %+v", children)
	}
	if diff := cmp.Diff([]graph.Span{{Start: 10, Stop: 14}, {Start: 28, Stop: 32}}, slices.Collect(facts.RowBlocks(root.ID))); diff != "" {
		t.Errorf("caught: owned continuation differs (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]graph.Span{{Start: 34, Stop: 37}}, slices.Collect(facts.StrayBlocks(1))); diff != "" {
		t.Errorf("caught: stray block differs (-want +got):\n%s", diff)
	}
	root.ChildListID = 99
	again, ok := facts.Row(1)
	if !ok || again.ChildListID != 2 {
		t.Fatal("caught: copied row changed immutable facts")
	}
	for range facts.Outline() {
		break
	}
	if diff := cmp.Diff(wantOutline, slices.Collect(facts.Outline())); diff != "" {
		t.Fatal("caught: early iterator stop consumed facts")
	}
	if _, ok := facts.Row(0); ok {
		t.Fatal("caught: absent row ID accepted")
	}
	if _, ok := facts.List(99); ok {
		t.Fatal("caught: unknown list ID accepted")
	}
	if len(slices.Collect(facts.ListRows(-1))) != 0 || len(slices.Collect(facts.RowBlocks(99))) != 0 || len(slices.Collect(facts.RowInlineParts(0))) != 0 || len(slices.Collect(facts.StrayBlocks(0))) != 0 {
		t.Fatal("caught: invalid ID yielded values")
	}
}

func TestBodyStructureContainerOwnership(t *testing.T) {
	facts := graph.ReadBody("- > quoted\n  > - nested\n\n  after\n")
	rows := slices.Collect(facts.ListRows(1))
	if len(rows) != 1 || rows[0].FirstBlockHasLines || rows[0].ChildListID != 0 {
		t.Fatalf("caught: first container or direct child selection differs: %+v", rows)
	}
	if len(slices.Collect(facts.RowInlineParts(rows[0].ID))) != 0 {
		t.Fatal("caught: line-less first container contributed gloss parts")
	}
	nested, ok := facts.List(2)
	if !ok || nested.ParentRowID != 0 {
		t.Fatalf("caught: quote ancestry invented an owning row: %+v", nested)
	}
	if diff := cmp.Diff([]graph.Span{{Start: 4, Stop: 10}, {Start: 27, Stop: 32}}, slices.Collect(facts.RowBlocks(rows[0].ID))); diff != "" {
		t.Errorf("caught: nested list escaped owned-block exclusion (-want +got):\n%s", diff)
	}
}

func TestBodyStructureRawCodeAndBreaks(t *testing.T) {
	facts := graph.ReadBody("- a ` code\n  words ` b  \n  c\n")
	parts := slices.Collect(facts.RowInlineParts(1))
	want := []graph.BodyInlinePart{
		{Span: graph.Span{Start: 2, Stop: 4}},
		{Span: graph.Span{Start: 6, Stop: 18}, Code: true},
		{Span: graph.Span{Start: 20, Stop: 22}, HardLineBreak: true},
		{Span: graph.Span{Start: 27, Stop: 28}},
	}
	if diff := cmp.Diff(want, parts); diff != "" {
		t.Fatalf("caught: raw inline parts differ (-want +got):\n%s", diff)
	}
	if len(parts) > 1 && facts.Source()[parts[1].Span.Start:parts[1].Span.Stop] != "code\n  words" {
		t.Fatal("caught: code content normalized original bytes")
	}
}

func TestBodyStructureExpandedOriginalGrammar(t *testing.T) {
	// Inline notes stay literal in sequence's original-byte grammar. Authored
	// admission would hide this emphasis inside an opaque node.
	facts := graph.ReadBody("- ^[*words*]\n")
	if diff := cmp.Diff([]graph.Span{{Start: 4, Stop: 5}}, slices.Collect(facts.PairedEmphasisOpeners())); diff != "" {
		t.Errorf("caught: original grammar lost literal-note emphasis (-want +got):\n%s", diff)
	}
	parts := slices.Collect(facts.RowInlineParts(1))
	var words strings.Builder
	for _, part := range parts {
		words.WriteString(facts.Source()[part.Span.Start:part.Span.Stop])
	}
	if got := words.String(); got != "^[words]" {
		t.Errorf("caught: original grammar words = %q, want literal note", got)
	}
}

func TestBodyRichHeadingPresentationOrigins(t *testing.T) {
	facts := graph.ReadBody("## A%%hide%%B `x`\n")
	want := []graph.RichBodyHeading{{ID: 1, Level: 2, Raw: "AB `x`", Span: graph.Span{Start: 3, Stop: 9}, Original: graph.Span{Start: 3, Stop: 17}}}
	if diff := cmp.Diff(want, slices.Collect(facts.RichHeadings())); diff != "" {
		t.Fatalf("caught: rich raw heading differs (-want +got):\n%s", diff)
	}
	wantOrigins := []graph.BodyOrigin{
		{Presentation: graph.Span{Start: 3, Stop: 4}, Original: graph.Span{Start: 3, Stop: 4}},
		{Presentation: graph.Span{Start: 4, Stop: 9}, Original: graph.Span{Start: 12, Stop: 17}},
	}
	if diff := cmp.Diff(wantOrigins, slices.Collect(facts.RichHeadingOrigins(1))); diff != "" {
		t.Errorf("caught: rich heading attribution differs (-want +got):\n%s", diff)
	}
	if len(slices.Collect(facts.RichHeadingOrigins(0))) != 0 {
		t.Fatal("caught: absent heading ID yielded origins")
	}
}

func TestBodyRichHeadingFootnotePlacement(t *testing.T) {
	facts := graph.ReadBody("## Main\n\nref[^used]\n\n[^unused]:\n    ## Hidden\n\n[^used]:\n    ## Shown\n")
	var raw []string
	for h := range facts.RichHeadings() {
		raw = append(raw, h.Raw)
	}
	if diff := cmp.Diff([]string{"Main", "Shown"}, raw); diff != "" {
		t.Errorf("caught: post-placement heading set differs (-want +got):\n%s", diff)
	}
}

func TestBodyStructureEmptyHeadingHasNoOutlineEvent(t *testing.T) {
	for _, tt := range []struct {
		body  string
		level int
	}{{"#\n", 1}, {"##   \n", 2}, {"### ###\n", 3}} {
		facts := graph.ReadBody(tt.body)
		if len(slices.Collect(facts.Outline())) != 0 {
			t.Errorf("caught: empty heading fabricated outline event for %q", tt.body)
		}
		// Rich headings intentionally retain the empty heading for the
		// judge's fallback section naming; outline ownership is separate.
		want := []graph.RichBodyHeading{{ID: 1, Level: tt.level}}
		if diff := cmp.Diff(want, slices.Collect(facts.RichHeadings())); diff != "" {
			t.Errorf("caught: empty rich heading differs for %q (-want +got):\n%s", tt.body, diff)
		}
	}
}

func TestBodyFactsZeroValue(t *testing.T) {
	var facts graph.BodyFacts
	if facts.Source() != "" || facts.CommentFree() != "" || facts.UnclosedComment() != (graph.BodyComment{}) {
		t.Fatal("caught: zero facts contain source or comment state")
	}
	for _, offset := range []int{-1, 0, 1} {
		if facts.CodeAt(offset) || facts.CommentAt(offset) || facts.EmittedAt(offset) {
			t.Fatalf("caught: zero facts recognize offset %d", offset)
		}
	}
	for _, id := range []int{-1, 0, 1} {
		if value, ok := facts.List(id); ok || value != (graph.BodyList{}) {
			t.Fatalf("caught: zero facts contain list %d", id)
		}
		if value, ok := facts.Row(id); ok || value != (graph.BodyRow{}) {
			t.Fatalf("caught: zero facts contain row %d", id)
		}
		assertNoBodyValues(t, facts.ListRows(id))
		assertNoBodyValues(t, facts.RowBlocks(id))
		assertNoBodyValues(t, facts.RowInlineParts(id))
		assertNoBodyValues(t, facts.StrayBlocks(id))
		assertNoBodyValues(t, facts.RichHeadingOrigins(id))
	}
	for range 2 {
		assertNoBodyValues(t, facts.Codes())
		assertNoBodyValues(t, facts.Comments())
		assertNoBodyValues(t, facts.Footnotes())
		assertNoBodyValues(t, facts.InlineFootnotes())
		assertNoBodyValues(t, facts.Headings())
		assertNoBodyValues(t, facts.Destinations())
		assertNoBodyValues(t, facts.CodeLiterals())
		assertNoBodyValues(t, facts.HTMLCommentLimits())
		assertNoBodyValues(t, facts.Outline())
		assertNoBodyValues(t, facts.PairedEmphasisOpeners())
		assertNoBodyValues(t, facts.RichHeadings())
	}
}

func TestBodyFactsHandleCopyIsolation(t *testing.T) {
	original := graph.ReadBody("## A%%hide%%B\n\n- row\n")
	borrowed := original
	originValues := slices.Collect(original.RichHeadingOrigins(1))
	rowValues := slices.Collect(original.ListRows(1))
	headingValues := slices.Collect(borrowed.RichHeadings())
	if len(headingValues) != 1 {
		t.Fatal("caught: copied handle lost rich heading")
	}
	headingValues[0].Raw = "changed"
	for origin := range borrowed.RichHeadingOrigins(1) {
		origin.Original = graph.Span{}
		if origin.Original != (graph.Span{}) {
			t.Fatal("caught: local origin copy did not accept the mutation stimulus")
		}
	}
	for row := range borrowed.ListRows(1) {
		row.ChildListID = 99
		if row.ChildListID != 99 {
			t.Fatal("caught: local row copy did not accept the mutation stimulus")
		}
	}
	want := slices.Collect(original.RichHeadings())
	if len(want) != 1 || want[0].Raw != "AB" {
		t.Fatal("caught: copied handle exposed mutable heading backing")
	}
	if diff := cmp.Diff(originValues, slices.Collect(borrowed.RichHeadingOrigins(1))); diff != "" {
		t.Fatal("caught: copied handle exposed mutable origin backing")
	}
	if diff := cmp.Diff(rowValues, slices.Collect(borrowed.ListRows(1))); diff != "" {
		t.Fatal("caught: copied handle exposed mutable row backing")
	}
	for range borrowed.Outline() {
		break
	}
	if diff := cmp.Diff(slices.Collect(original.Outline()), slices.Collect(borrowed.Outline())); diff != "" {
		t.Fatal("caught: copied handle iterator was consumed")
	}
	borrowed = graph.BodyFacts{}
	if borrowed.Source() != "" || original.Source() != "## A%%hide%%B\n\n- row\n" || len(slices.Collect(original.ListRows(1))) != 1 {
		t.Fatal("caught: zeroing copied handle changed original")
	}
}

func assertNoBodyValues[T any](t *testing.T, values iter.Seq[T]) {
	t.Helper()
	if slices.Collect(values) != nil {
		t.Fatal("caught: zero facts yielded recognition values")
	}
}
