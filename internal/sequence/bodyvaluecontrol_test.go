package sequence_test

import (
	"flag"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/sequence"
)

var bodyMutationMode = flag.String("body-mode", "", "body product control selected by its compiling fault")

type bodyValueCase struct {
	Name, Identity string
	Run            func(*testing.T, *bodyValueCase)
}

// Each product is compared with literal values at the layer that publishes it.
var bodyValueCases = []bodyValueCase{
	{Name: "f3-order", Identity: "outline-order", Run: bodyStructureValueControl},
	{Name: "f3-heading-level", Identity: "heading-level", Run: bodyStructureValueControl},
	{Name: "f3-heading-range", Identity: "heading-range", Run: bodyStructureValueControl},
	{Name: "f3-row-member", Identity: "row-member", Run: bodyStructureValueControl},
	{Name: "f3-parent", Identity: "parent-row", Run: bodyStructureValueControl},
	{Name: "f3-direct-child", Identity: "direct-child", Run: bodyStructureValueControl},
	{Name: "f3-own-continuation", Identity: "owned-continuation", Run: bodyStructureValueControl},
	{Name: "f3-own-nested", Identity: "excluded-nested-list", Run: bodyStructureValueControl},
	{Name: "f3-stray", Identity: "stray-range", Run: bodyStructureValueControl},
	{Name: "f3-paired", Identity: "paired-opener", Run: bodyEmphasisControl},
	{Name: "f3-unpaired", Identity: "unpaired-opener", Run: bodyEmphasisControl},
	{Name: "f3-checkbox", Identity: "checkbox-distinction", Run: bodyCheckboxControl},
	{Name: "f3-row-span", Identity: "row-span", Run: bodyCoordinatesControl},
	{Name: "f3-target-span", Identity: "target-span", Run: bodyCoordinatesControl},
	{Name: "f3-anchor-span", Identity: "anchor-span", Run: bodyCoordinatesControl},
	{Name: "f3-gloss-text", Identity: "gloss-text", Run: bodyGlossControl},
	{Name: "f3-gloss-code", Identity: "gloss-code", Run: bodyGlossControl},
	{Name: "f3-soft-break", Identity: "soft-break", Run: bodyGlossControl},
	{Name: "f3-hard-break", Identity: "hard-break", Run: bodyGlossControl},
	{Name: "f3-row-role", Identity: "row-grammar-role", Run: bodyRowRoleControl},
	{Name: "f3-candidate-text", Identity: "candidate-text", Run: bodyCoordinatesControl},
	{Name: "f3-candidate-target", Identity: "candidate-target", Run: bodyCoordinatesControl},
	{Name: "f3-candidate-aliased", Identity: "candidate-aliased", Run: bodyCoordinatesControl},
	{Name: "f3-candidate-fragment", Identity: "candidate-fragment", Run: bodyCoordinatesControl},
	{Name: "f3-candidate-state", Identity: "candidate-state", Run: bodyCoordinatesControl},
	{Name: "f3-candidate-line", Identity: "candidate-line", Run: bodyCoordinatesControl},
	{Name: "f3-diagnostic-rule", Identity: "diagnostic-rule", Run: bodyEmphasisControl},
	{Name: "f3-diagnostic-line", Identity: "diagnostic-line", Run: bodyEmphasisControl},
	{Name: "f3-diagnostic-message", Identity: "diagnostic-message", Run: bodyEmphasisControl},
	{Name: "f3-diagnostic-evidence", Identity: "diagnostic-evidence", Run: bodyEmphasisControl},
	{Name: "f3-empty-outline", Identity: "empty-outline", Run: bodyEmptyOutlineControl},
	{Name: "f3-list-id", Identity: "list-id", Run: bodyFieldControl},
	{Name: "f3-list-parent", Identity: "list-parent", Run: bodyFieldControl},
	{Name: "f3-row-id", Identity: "row-id", Run: bodyFieldControl},
	{Name: "f3-row-list", Identity: "row-list", Run: bodyFieldControl},
	{Name: "f3-row-fallback", Identity: "row-fallback", Run: bodyFieldControl},
	{Name: "f3-row-presence", Identity: "row-presence", Run: bodyFieldControl},
	{Name: "f3-row-first-child", Identity: "row-first-child", Run: bodyFieldControl},
	{Name: "f3-outline-list", Identity: "outline-list", Run: bodyFieldControl},
	{Name: "f3-outline-stray", Identity: "outline-stray", Run: bodyFieldControl},
	{Name: "f3-inline-text-span", Identity: "inline-text-span", Run: bodyFieldControl},
	{Name: "f3-inline-code-span", Identity: "inline-code-span", Run: bodyFieldControl},
	{Name: "f3-inline-code-kind", Identity: "inline-code-kind", Run: bodyFieldControl},
	{Name: "f3-accessor-outline", Identity: "accessor-outline", Run: bodyFieldControl},
	{Name: "f3-accessor-list", Identity: "accessor-list", Run: bodyFieldControl},
	{Name: "f3-accessor-row", Identity: "accessor-row", Run: bodyFieldControl},
	{Name: "f3-accessor-list-rows", Identity: "accessor-list-rows", Run: bodyFieldControl},
	{Name: "f3-accessor-row-blocks", Identity: "accessor-row-blocks", Run: bodyFieldControl},
	{Name: "f3-accessor-row-parts", Identity: "accessor-row-parts", Run: bodyFieldControl},
	{Name: "f3-accessor-stray", Identity: "accessor-stray", Run: bodyFieldControl},
	{Name: "f3-accessor-rich", Identity: "accessor-rich", Run: bodyFieldControl},
	{Name: "f3-accessor-rich-origin", Identity: "accessor-rich-origin", Run: bodyFieldControl},
	{Name: "f3-accessor-paired", Identity: "accessor-paired", Run: bodyFieldControl},
	{Name: "f3-group-heading-name", Identity: "group-heading-name", Run: bodyFieldControl},
	{Name: "f3-group-heading-level", Identity: "group-heading-level", Run: bodyFieldControl},
	{Name: "f3-group-heading-line", Identity: "group-heading-line", Run: bodyFieldControl},
	{Name: "f3-group-heading-role", Identity: "group-heading-role", Run: bodyFieldControl},
	{Name: "f3-container-name", Identity: "container-name", Run: bodyFieldControl},
	{Name: "f3-container-line", Identity: "container-line", Run: bodyFieldControl},
	{Name: "f3-container-role", Identity: "container-role", Run: bodyFieldControl},
	{Name: "f3-container-container", Identity: "container-container", Run: bodyFieldControl},
	{Name: "f3-container-anchortarget", Identity: "container-anchortarget", Run: bodyFieldControl},
	{Name: "f3-container-anchorspan", Identity: "container-anchorspan", Run: bodyFieldControl},
	{Name: "f3-container-invalid", Identity: "container-invalid", Run: bodyFieldControl},
	{Name: "f3-entry-item", Identity: "entry-item", Run: bodyFieldControl},
	{Name: "f3-branch-item", Identity: "branch-item", Run: bodyFieldControl},
	{Name: "f3-document-groups", Identity: "document-groups", Run: bodyFieldControl},
	{Name: "f3-document-diagnostics", Identity: "document-diagnostics", Run: bodyFieldControl},
	{Name: "f3-link-target", Identity: "link-target", Run: bodyFieldControl},
	{Name: "f3-link-display", Identity: "link-display", Run: bodyFieldControl},
	{Name: "f3-link-aliased", Identity: "link-aliased", Run: bodyFieldControl},
	{Name: "f3-link-fragment", Identity: "link-fragment", Run: bodyFieldControl},
	{Name: "f3-link-span", Identity: "link-span", Run: bodyFieldControl},
}

func TestBodyValueMutationControl(t *testing.T) {
	for i := range bodyValueCases {
		mode := &bodyValueCases[i]
		if *bodyMutationMode != "" {
			if mode.Name != *bodyMutationMode {
				continue
			}
			mode.Run(t, mode)
			t.Logf("AGREEMENT-INVOKED F3/%s", mode.Name)
			return
		}
		t.Run(mode.Name, func(t *testing.T) {
			mode.Run(t, mode)
			t.Logf("AGREEMENT-INVOKED F3/%s", mode.Name)
		})
	}
	if *bodyMutationMode != "" {
		t.Fatalf("not-applied: unknown body control %q", *bodyMutationMode)
	}
}

func bodyRowRoleControl(t *testing.T, mode *bodyValueCase) {
	t.Helper()
	bodySimpleDocumentControl(t, mode, "## P {sequence=primary}\n\n- [[A]] ^[note]\n", graph.Span{Start: 27, Stop: 40}, "^[note]")
}

func bodyValueCompare[T any](t *testing.T, mode *bodyValueCase, want, got T) {
	t.Helper()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("caught: F3 %s (-want +got):\n%s", mode.Identity, diff)
	}
}

// No wanted value is obtained from another call to the producer.
func bodySimpleDocumentControl(t *testing.T, mode *bodyValueCase, body string, span graph.Span, gloss string) {
	t.Helper()
	want := sequence.Document{Groups: []*sequence.Group{{
		Name: "P", Level: 2, Line: 7, Role: sequence.RolePrimary,
		Items: []sequence.Item{{Entry: &sequence.Candidate{
			Text: "A", Target: "A", Line: 9, Span: span, Gloss: gloss,
			TargetSpan: graph.Span{Start: 27, Stop: 32}, State: sequence.EntryAccepted,
		}}},
	}}}
	bodyValueCompare(t, mode, want, sequence.ParseFacts(graph.ReadBody(body), 7))
}

func bodyGlossControl(t *testing.T, mode *bodyValueCase) {
	t.Helper()
	for _, tt := range []struct {
		body  string
		span  graph.Span
		gloss string
	}{
		{body: "## P {sequence=primary}\n\n- [[A]] one\n  two\n", span: graph.Span{Start: 27, Stop: 42}, gloss: "one two"},
		{body: "## P {sequence=primary}\n\n- [[A]] one  \n  two\n", span: graph.Span{Start: 27, Stop: 44}, gloss: "one two"},
		{body: "## P {sequence=primary}\n\n- [[A]] `one\n  two`\n", span: graph.Span{Start: 27, Stop: 44}, gloss: "one\n  two"},
		{body: "## P {sequence=primary}\n\n- [[A]] one %%hidden%% two\n", span: graph.Span{Start: 27, Stop: 51}, gloss: "one  two"},
	} {
		bodySimpleDocumentControl(t, mode, tt.body, tt.span, tt.gloss)
	}
}

func bodyRichValueControl(t *testing.T, mode *bodyValueCase) {
	t.Helper()
	facts := graph.ReadBody("## A%%hide%%B `x`\n")
	bodyValueCompare(t, mode, []graph.RichBodyHeading{{ID: 1, Level: 2, Raw: "AB `x`", Span: graph.Span{Start: 3, Stop: 9}, Original: graph.Span{Start: 3, Stop: 17}}}, slices.Collect(facts.RichHeadings()))
	bodyValueCompare(t, mode, []graph.BodyOrigin{
		{Presentation: graph.Span{Start: 3, Stop: 4}, Original: graph.Span{Start: 3, Stop: 4}},
		{Presentation: graph.Span{Start: 4, Stop: 9}, Original: graph.Span{Start: 12, Stop: 17}},
	}, slices.Collect(facts.RichHeadingOrigins(1)))
	// Expanded authority grows the inline note into a reference. Rich raw
	// ownership keeps the literal pre-expansion bytes at their own positions.
	facts = graph.ReadBody("## A ^[note]\n")
	bodyValueCompare(t, mode, []graph.RichBodyHeading{{ID: 1, Level: 2, Raw: "A ^[note]", Span: graph.Span{Start: 3, Stop: 12}, Original: graph.Span{Start: 3, Stop: 12}}}, slices.Collect(facts.RichHeadings()))
	facts = graph.ReadBody("## Main\n\nref[^used]\n\n[^unused]:\n    ## Hidden\n\n[^used]:\n    ## Shown\n")
	bodyValueCompare(t, mode, []graph.RichBodyHeading{
		{ID: 1, Level: 2, Raw: "Main", Span: graph.Span{Start: 3, Stop: 7}, Original: graph.Span{Start: 3, Stop: 7}},
		{ID: 2, Level: 2, Raw: "Shown", Span: graph.Span{Start: 63, Stop: 68}, Original: graph.Span{Start: 63, Stop: 68}},
	}, slices.Collect(facts.RichHeadings()))
}

func bodyStructureValueControl(t *testing.T, mode *bodyValueCase) {
	t.Helper()
	facts := graph.ReadBody("# Path\n\n- root\n  - child\n\n  tail\n\nend\n")
	bodyValueCompare(t, mode, []graph.BodyOutline{
		{Heading: graph.BodyOutlineHeading{Span: graph.Span{Start: 2, Stop: 6}, Level: 1}}, {ListID: 1}, {StrayID: 1},
	}, slices.Collect(facts.Outline()))
	bodyValueCompare(t, mode, []graph.BodyRow{{ID: 1, ListID: 1, ChildListID: 2, FirstBlockHasLines: true}}, slices.Collect(facts.ListRows(1)))
	bodyValueCompare(t, mode, []graph.BodyRow{{ID: 2, ListID: 2, ParentRowID: 1, FirstBlockHasLines: true}}, slices.Collect(facts.ListRows(2)))
	bodyValueCompare(t, mode, []graph.Span{{Start: 10, Stop: 14}, {Start: 28, Stop: 32}}, slices.Collect(facts.RowBlocks(1)))
	bodyValueCompare(t, mode, []graph.Span{{Start: 34, Stop: 37}}, slices.Collect(facts.StrayBlocks(1)))
	bodyValueCompare(t, mode, sequence.Document{Diagnostics: []sequence.Diagnostic{{
		Rule: sequence.RuleEntryOutsideBranch, Line: 10,
		Message: "rows nested before the first level-2 heading belong to no part of the course", Evidence: "",
	}}}, sequence.ParseFacts(facts, 7))
	bodyCoordinatesControl(t, mode)
	bodyContinuationControl(t, mode)
	bodyNestedExclusionControl(t, mode)
	facts = graph.ReadBody("- > quoted\n  > - nested\n\n  after\n")
	bodyValueCompare(t, mode, []graph.BodyRow{{ID: 1, ListID: 1}}, slices.Collect(facts.ListRows(1)))
	bodyValueCompare(t, mode, []graph.Span{{Start: 4, Stop: 10}, {Start: 27, Stop: 32}}, slices.Collect(facts.RowBlocks(1)))
	// The stray fault must reach the recursive, container-descended branch.
	facts = graph.ReadBody("> stray {sequence=primary}\n")
	bodyValueCompare(t, mode, sequence.Document{Diagnostics: []sequence.Diagnostic{{Rule: sequence.RuleRoleMisplaced, Line: 7,
		Message: "a sequence marker here declares nothing; it is read only on a heading or on a row that opens a list", Evidence: "stray {sequence=primary}"}}}, sequence.ParseFacts(facts, 7))
}

func bodyEmphasisControl(t *testing.T, mode *bodyValueCase) {
	t.Helper()
	cases := []struct {
		body        string
		candidate   sequence.Candidate
		diagnostics []sequence.Diagnostic
	}{
		{body: "## P {sequence=primary}\n\n- *[[A]]*\n", candidate: sequence.Candidate{Text: "A", Target: "A", Line: 9, Span: graph.Span{Start: 27, Stop: 34}, TargetSpan: graph.Span{Start: 28, Stop: 33}, State: sequence.EntryAccepted}},
		{body: "## P {sequence=primary}\n\n- *[[A]]\n", candidate: sequence.Candidate{Text: "A", Target: "A", Line: 9, Span: graph.Span{Start: 27, Stop: 33}, TargetSpan: graph.Span{Start: 28, Stop: 33}, State: sequence.EntryNoncanonical}, diagnostics: []sequence.Diagnostic{{Rule: sequence.RuleEntryNoncanonical, Line: 9,
			Message: "a lesson row opens with its link; move the link to the front, or take the row out of the course", Evidence: "*[[A]]"}}},
	}
	for i := range cases {
		tt := &cases[i]
		want := sequence.Document{Groups: []*sequence.Group{{Name: "P", Level: 2, Line: 7, Role: sequence.RolePrimary, Items: []sequence.Item{{Entry: &tt.candidate}}}}, Diagnostics: tt.diagnostics}
		bodyValueCompare(t, mode, want, sequence.ParseFacts(graph.ReadBody(tt.body), 7))
	}
}

func bodyCheckboxControl(t *testing.T, mode *bodyValueCase) {
	t.Helper()
	for _, marker := range []string{" ", "x", "X", "/", "-", ">"} {
		own := "[" + marker + "] [[A]]"
		want := sequence.Document{Groups: []*sequence.Group{{Name: "P", Level: 2, Line: 7, Role: sequence.RolePrimary}}}
		if marker == "/" || marker == "-" || marker == ">" {
			want.Groups[0].Items = []sequence.Item{{Entry: &sequence.Candidate{Text: "A", Target: "A", Line: 9, Span: graph.Span{Start: 27, Stop: 36}, TargetSpan: graph.Span{Start: 31, Stop: 36}, State: sequence.EntryNoncanonical}}}
			want.Diagnostics = []sequence.Diagnostic{{Rule: sequence.RuleEntryNoncanonical, Line: 9,
				Message: "a lesson row opens with its link; move the link to the front, or take the row out of the course", Evidence: own}}
		}
		bodyValueCompare(t, mode, want, sequence.ParseFacts(graph.ReadBody("## P {sequence=primary}\n\n- "+own+"\n"), 7))
	}
	for _, marker := range []string{" ", "x", "X"} {
		body := "## P {sequence=primary}\n\n- [" + marker + "] [[A]]\n  - Side {sequence=local}\n    - [[B]]\n"
		want := sequence.Document{Groups: []*sequence.Group{{Name: "P", Level: 2, Line: 7, Role: sequence.RolePrimary, Items: []sequence.Item{{Branch: &sequence.Group{
			Name: "Side", Line: 10, Role: sequence.RoleLocal, Container: true, Invalid: true,
			Items: []sequence.Item{{Entry: &sequence.Candidate{Text: "B", Target: "B", Line: 11, Span: graph.Span{Start: 69, Stop: 74}, TargetSpan: graph.Span{Start: 69, Stop: 74}, State: sequence.EntryAccepted}}},
		}}}}}, Diagnostics: []sequence.Diagnostic{{Rule: sequence.RuleLocalOrphan, Line: 10,
			Message: "a side branch has nothing to hang from; nest it under the lesson it belongs to", Evidence: "Side {sequence=local}"}}}
		bodyValueCompare(t, mode, want, sequence.ParseFacts(graph.ReadBody(body), 7))
	}
}

// All coordinates jointly select the second occurrence, independently of its
// shared target, alias and fragment, and retain the file's CRLF line numbers.
func bodyCoordinatesControl(t *testing.T, mode *bodyValueCase) {
	t.Helper()
	body := "## P {sequence=primary}\r\n\r\n- [[A|Alias]]\r\n- [[A#Part|Again]]\r\n  - Side {sequence=local}\r\n    - [[B]]\r\n"
	want := sequence.Document{Groups: []*sequence.Group{{Name: "P", Level: 2, Line: 7, Role: sequence.RolePrimary, Items: []sequence.Item{
		{Entry: &sequence.Candidate{Text: "Alias", Target: "A", Aliased: true, Line: 9, Span: graph.Span{Start: 29, Stop: 40}, TargetSpan: graph.Span{Start: 29, Stop: 40}, State: sequence.EntryAccepted}},
		{Entry: &sequence.Candidate{Text: "Again", Target: "A", Aliased: true, Fragment: true, Line: 10, Span: graph.Span{Start: 44, Stop: 60}, TargetSpan: graph.Span{Start: 44, Stop: 60}, State: sequence.EntryAccepted}},
		{Branch: &sequence.Group{Name: "Side", Line: 11, Role: sequence.RoleLocal, Container: true, AnchorTarget: "A", AnchorSpan: graph.Span{Start: 44, Stop: 60}, Items: []sequence.Item{
			{Entry: &sequence.Candidate{Text: "B", Target: "B", Line: 12, Span: graph.Span{Start: 95, Stop: 100}, TargetSpan: graph.Span{Start: 95, Stop: 100}, State: sequence.EntryAccepted}},
		}}},
	}}}}
	bodyValueCompare(t, mode, want, sequence.ParseFacts(graph.ReadBody(body), 7))
}

func bodyEmptyOutlineControl(t *testing.T, mode *bodyValueCase) {
	t.Helper()
	facts := graph.ReadBody("## P {sequence=primary}\n\n##\n\n- [[A]]\n")
	want := sequence.Document{Groups: []*sequence.Group{{Name: "P", Level: 2, Line: 7, Role: sequence.RolePrimary, Items: []sequence.Item{{Entry: &sequence.Candidate{
		Text: "A", Target: "A", Line: 11, Span: graph.Span{Start: 31, Stop: 36}, TargetSpan: graph.Span{Start: 31, Stop: 36}, State: sequence.EntryAccepted,
	}}}}}}
	bodyValueCompare(t, mode, want, sequence.ParseFacts(facts, 7))
}

func bodyContinuationControl(t *testing.T, mode *bodyValueCase) {
	t.Helper()
	cases := []struct {
		body       string
		candidate  sequence.Candidate
		diagnostic sequence.Diagnostic
	}{
		{body: "## P {sequence=primary}\n\n- [[A]]\n\n  tail [[B]]\n", candidate: sequence.Candidate{Text: "[[A]]", Line: 9, Span: graph.Span{Start: 27, Stop: 32}, State: sequence.EntryMultiTarget}, diagnostic: sequence.Diagnostic{Rule: sequence.RuleEntryMultiTarget, Line: 9,
			Message: "a row naming more than one note does not say which lesson it is; give each lesson its own row", Evidence: "[[A]]"}},
		{body: "## P {sequence=primary}\n\n- [[A]]\n\n  tail {sequence=local}\n", candidate: sequence.Candidate{Text: "A", Target: "A", Line: 9, Span: graph.Span{Start: 27, Stop: 32}, TargetSpan: graph.Span{Start: 27, Stop: 32}, State: sequence.EntryAccepted}, diagnostic: sequence.Diagnostic{Rule: sequence.RuleRoleMisplaced, Line: 11,
			Message: "a sequence marker is read on the row's own line, not in its continuation", Evidence: "tail {sequence=local}"}},
	}
	for i := range cases {
		tt := &cases[i]
		want := sequence.Document{Groups: []*sequence.Group{{Name: "P", Level: 2, Line: 7, Role: sequence.RolePrimary, Items: []sequence.Item{{Entry: &tt.candidate}}}}, Diagnostics: []sequence.Diagnostic{tt.diagnostic}}
		bodyValueCompare(t, mode, want, sequence.ParseFacts(graph.ReadBody(tt.body), 7))
	}
}

func bodyNestedExclusionControl(t *testing.T, mode *bodyValueCase) {
	t.Helper()
	body := "## P {sequence=primary}\n\n- [[A]]\n\n  > - [[B]]\n\n  > tail\n"
	want := sequence.Document{Groups: []*sequence.Group{{Name: "P", Level: 2, Line: 7, Role: sequence.RolePrimary, Items: []sequence.Item{{Entry: &sequence.Candidate{
		Text: "A", Target: "A", Line: 9, Span: graph.Span{Start: 27, Stop: 32}, TargetSpan: graph.Span{Start: 27, Stop: 32}, State: sequence.EntryAccepted,
	}}}}}}
	bodyValueCompare(t, mode, want, sequence.ParseFacts(graph.ReadBody(body), 7))
}

func bodyFieldControl(t *testing.T, mode *bodyValueCase) {
	t.Helper()
	bodyStructureValueControl(t, mode)
	bodyRichValueControl(t, mode)
	bodyGlossControl(t, mode)
	bodyCheckboxControl(t, mode)
	bodyEmphasisControl(t, mode)
	facts := graph.ReadBody("# Path\n\n- root\n  - child\n\n  tail\n\nend\n")
	for _, want := range []graph.BodyList{{ID: 1}, {ID: 2, ParentRowID: 1}} {
		got, found := facts.List(want.ID)
		bodyValueCompare(t, mode, want, got)
		bodyValueCompare(t, mode, true, found)
	}
	for _, want := range []graph.BodyRow{{ID: 1, ListID: 1, ChildListID: 2, FirstBlockHasLines: true}, {ID: 2, ListID: 2, ParentRowID: 1, FirstBlockHasLines: true}} {
		got, found := facts.Row(want.ID)
		bodyValueCompare(t, mode, want, got)
		bodyValueCompare(t, mode, true, found)
	}
	for _, id := range []int{-1, 0, 3} {
		list, found := facts.List(id)
		bodyValueCompare(t, mode, graph.BodyList{}, list)
		bodyValueCompare(t, mode, false, found)
		row, found := facts.Row(id)
		bodyValueCompare(t, mode, graph.BodyRow{}, row)
		bodyValueCompare(t, mode, false, found)
		bodyValueCompare(t, mode, []graph.BodyRow(nil), slices.Collect(facts.ListRows(id)))
		bodyValueCompare(t, mode, []graph.Span(nil), slices.Collect(facts.RowBlocks(id)))
		bodyValueCompare(t, mode, []graph.BodyInlinePart(nil), slices.Collect(facts.RowInlineParts(id)))
		bodyValueCompare(t, mode, []graph.Span(nil), slices.Collect(facts.StrayBlocks(id)))
		bodyValueCompare(t, mode, []graph.BodyOrigin(nil), slices.Collect(facts.RichHeadingOrigins(id)))
	}
	bodyValueCompare(t, mode, []graph.BodyInlinePart{{Span: graph.Span{Start: 10, Stop: 14}}}, slices.Collect(facts.RowInlineParts(1)))
	bodyValueCompare(t, mode, []graph.BodyInlinePart{{Span: graph.Span{Start: 19, Stop: 24}}}, slices.Collect(facts.RowInlineParts(2)))
	facts = graph.ReadBody("- `b`\n")
	bodyValueCompare(t, mode, []graph.BodyInlinePart{{Span: graph.Span{Start: 3, Stop: 4}, Code: true}}, slices.Collect(facts.RowInlineParts(1)))
	facts = graph.ReadBody("- b\n")
	bodyValueCompare(t, mode, []graph.BodyInlinePart{{Span: graph.Span{Start: 2, Stop: 3}}}, slices.Collect(facts.RowInlineParts(1)))
	links, zones := sequence.LiveScanFacts(graph.ReadBody("[[A#Part|Alias]] `[[B]]`\n"))
	bodyValueCompare(t, mode, []sequence.Link{{Target: "A", Display: "Alias", Aliased: true, Fragment: true, Span: graph.Span{Start: 0, Stop: 16}}}, links)
	bodyValueCompare(t, mode, []graph.Span{{Start: 18, Stop: 24}}, zones)
}
