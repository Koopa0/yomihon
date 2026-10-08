package judge_test

import (
	"flag"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/sequence"
)

var agreementSource = flag.String("agreement-source", "", "temporary production source selected by the agreement overlay")

// Each row names one producer and one independently observable contract.
func bodyValueMutations() []agreementMutation {
	return []agreementMutation{
		{Name: "f3-order", Property: "F3", Identity: "outline-order", File: "internal/graph/bodystructure.go", Function: "collectOutline", Needle: "for c := doc.FirstChild(); c != nil; c = c.NextSibling()", Fault: "for c := doc.LastChild(); c != nil; c = c.PreviousSibling()"},
		{Name: "f3-heading-level", Property: "F3", Identity: "heading-level", File: "internal/graph/bodystructure.go", Function: "collectOutline", Needle: "Level: h.Level", Fault: "Level: h.Level + 1"},
		{Name: "f3-heading-range", Property: "F3", Identity: "heading-range", File: "internal/graph/bodystructure.go", Function: "collectOutline", Needle: "BodyOutlineHeading{Span: span,", Fault: "BodyOutlineHeading{Span: Span{Start: span.Start + 1, Stop: span.Stop},"},
		{Name: "f3-row-member", Property: "F3", Identity: "row-member", File: "internal/graph/bodystructure.go", Function: "collectListMembership", Needle: "if rowID := index.rows[c]; rowID != 0", Fault: "if rowID := index.rows[c]; rowID != 0 && c.NextSibling() != nil"},
		{Name: "f3-parent", Property: "F3", Identity: "parent-row", File: "internal/graph/bodystructure.go", Function: "collectRow", Needle: "row.ParentRowID = o.structure.lists[row.ListID-1].ParentRowID", Fault: "row.ParentRowID = 0"},
		{Name: "f3-direct-child", Property: "F3", Identity: "direct-child", File: "internal/graph/bodystructure.go", Function: "collectRow", Needle: "row.ChildListID = childID", Fault: "row.ChildListID = 0"},
		{Name: "f3-own-continuation", Property: "F3", Identity: "owned-continuation", File: "internal/graph/bodystructure.go", Function: "bodyOwnedBlocks", Needle: "spans = append(spans, span)", Fault: "if len(spans) == 0 { spans = append(spans, span) }"},
		{Name: "f3-own-nested", Property: "F3", Identity: "excluded-nested-list", File: "internal/graph/bodystructure.go", Function: "bodyOwnedBlocks", Needle: "if _, nested := c.(*ast.List); nested", Fault: "if _, nested := c.(*ast.List); nested && false"},
		{Name: "f3-stray", Property: "F3", Identity: "stray-range", File: "internal/graph/bodystructure.go", Function: "bodyStrayBlocks", Needle: "spans = append(spans, bodyStrayBlocks(c)...)", Fault: "_ = c"},
		{Name: "f3-paired", Property: "F3", Identity: "paired-opener", File: "internal/graph/bodystructure.go", Function: "collectStructureNode", Needle: "o.structure.openers = append(o.structure.openers, span)", Fault: "_ = span"},
		{Name: "f3-unpaired", Property: "F3", Identity: "unpaired-opener", File: "internal/sequence/sequence.go", Function: "openerAt", Needle: "return Span{}, false", Fault: "return Span{Start: off, Stop: off + 1}, true"},
		{Name: "f3-checkbox", Property: "F3", Identity: "checkbox-distinction", File: "internal/sequence/marker.go", Function: "isTaskRow", Needle: "case ' ', 'x', 'X':", Fault: "case ' ', 'x', 'X', '/', '-', '>':"},
		{Name: "f3-row-span", Property: "F3", Identity: "row-span", File: "internal/sequence/sequence.go", Function: "plainRow", Needle: "entry := &Candidate{Line: line, Span: spans[0]}", Fault: "entry := &Candidate{Line: line, Span: Span{Start: spans[0].Start + 1, Stop: spans[0].Stop}}"},
		{Name: "f3-target-span", Property: "F3", Identity: "target-span", File: "internal/sequence/sequence.go", Function: "span", Needle: "return Span{Start: h.start, Stop: h.stop}", Fault: "return Span{Start: h.start + 1, Stop: h.stop}"},
		{Name: "f3-anchor-span", Property: "F3", Identity: "anchor-span", File: "internal/sequence/sequence.go", Function: "anchorOwnTarget", Needle: "return entry.Target, spans[0], true", Fault: "return entry.Target, Span{Start: spans[0].Start + 1, Stop: spans[0].Stop}, true"},
		{Name: "f3-gloss-text", Property: "F3", Identity: "gloss-text", File: "internal/graph/bodystructure.go", Function: "bodyInlineParts", Needle: "case *ast.Text:", Fault: "case *ast.Text: continue"},
		{Name: "f3-gloss-code", Property: "F3", Identity: "gloss-code", File: "internal/sequence/sequence.go", Function: "gloss", Needle: "b.WriteString(p.body[part.Span.Start:part.Span.Stop])", Fault: "b.WriteString(strings.Join(strings.Fields(p.body[part.Span.Start:part.Span.Stop]), \" \"))"},
		{Name: "f3-soft-break", Property: "F3", Identity: "soft-break", File: "internal/graph/bodystructure.go", Function: "bodyInlineParts", Needle: "SoftLineBreak: node.SoftLineBreak()", Fault: "SoftLineBreak: false"},
		{Name: "f3-hard-break", Property: "F3", Identity: "hard-break", File: "internal/graph/bodystructure.go", Function: "bodyInlineParts", Needle: "HardLineBreak: node.HardLineBreak()", Fault: "HardLineBreak: false"},
		{Name: "f3-rich-raw", Property: "F3", Identity: "rich-raw", File: "internal/graph/bodystructure.go", Function: "attachRichHeadings", Needle: "Raw: raw", Fault: "Raw: raw[:0] + f.source[p.sourceSpan(h.Span).Start:p.sourceSpan(h.Span).Stop]"},
		{Name: "f3-rich-level", Property: "F3", Identity: "rich-level", File: "internal/graph/bodystructure.go", Function: "attachRichHeadings", Needle: "Level: h.Level", Fault: "Level: h.Level + 1"},
		{Name: "f3-rich-position", Property: "F3", Identity: "rich-position", File: "internal/graph/bodystructure.go", Function: "attachRichHeadings", Needle: "Span: h.Span", Fault: "Span: p.sourceSpan(h.Span)"},
		{Name: "f3-rich-origin", Property: "F3", Identity: "rich-origin", File: "internal/graph/bodystructure.go", Function: "attachRichHeadings", Needle: "origin.Original = Span{Start: start, Stop: start + right - left}", Fault: "origin.Original = Span{Start: start + 1, Stop: start + right - left}"},
		{Name: "f3-row-role", Property: "F3", Identity: "row-grammar-role", File: "internal/graph/body.go", Function: "ReadBody", Needle: "facts.structure = rows.structure", Fault: "_ = rows; facts.structure = bootstrap.structure"},
		{Name: "f3-rich-role", Property: "F3", Identity: "rich-grammar-role", File: "internal/graph/body.go", Function: "ReadBody", Needle: "facts.attachRichHeadings(admissionBody, rich.rich)", Fault: "_ = rich; facts.attachRichHeadings(expanded, authority.rich)"},
		{Name: "f3-candidate-text", Property: "F3", Identity: "candidate-text", File: "internal/sequence/sequence.go", Function: "linksIn", Needle: "display: link.Display", Fault: "display: link.Display[:0] + \"wrong\""},
		{Name: "f3-candidate-target", Property: "F3", Identity: "candidate-target", File: "internal/sequence/sequence.go", Function: "linksIn", Needle: "target: link.Target", Fault: "target: link.Target[:0] + \"wrong\""},
		{Name: "f3-candidate-aliased", Property: "F3", Identity: "candidate-aliased", File: "internal/sequence/sequence.go", Function: "linksIn", Needle: "aliased: link.Aliased", Fault: "aliased: false"},
		{Name: "f3-candidate-fragment", Property: "F3", Identity: "candidate-fragment", File: "internal/sequence/sequence.go", Function: "linksIn", Needle: "fragment: link.Heading != \"\" || link.Block != \"\"", Fault: "fragment: false"},
		{Name: "f3-candidate-state", Property: "F3", Identity: "candidate-state", File: "internal/sequence/sequence.go", Function: "plainRow", Needle: "entry.State = EntryAccepted", Fault: "entry.State = EntryUnread"},
		{Name: "f3-candidate-line", Property: "F3", Identity: "candidate-line", File: "internal/sequence/sequence.go", Function: "plainRow", Needle: "Line: line, Span: spans[0]", Fault: "Line: line + 1, Span: spans[0]"},
		{Name: "f3-diagnostic-rule", Property: "F3", Identity: "diagnostic-rule", File: "internal/sequence/sequence.go", Function: "report", Needle: "Rule:     rule", Fault: "Rule:     RuleRoleInvalid"},
		{Name: "f3-diagnostic-line", Property: "F3", Identity: "diagnostic-line", File: "internal/sequence/sequence.go", Function: "report", Needle: "Line:     line", Fault: "Line:     line + 1"},
		{Name: "f3-diagnostic-message", Property: "F3", Identity: "diagnostic-message", File: "internal/sequence/sequence.go", Function: "report", Needle: "Message:  message", Fault: "Message:  message[:0] + \"wrong\""},
		{Name: "f3-diagnostic-evidence", Property: "F3", Identity: "diagnostic-evidence", File: "internal/sequence/sequence.go", Function: "report", Needle: "Evidence: evidence", Fault: "Evidence: evidence[:0] + \"wrong\""},
		{Name: "f3-empty-outline", Property: "F3", Identity: "empty-outline", File: "internal/graph/bodystructure.go", Function: "collectOutline", Needle: "if span, present := bodyLinesRange(h); present", Fault: "if span, present := bodyLinesRange(h); present || h.Level > 0"},
		{Name: "f3-rich-original", Property: "F3", Identity: "rich-original", File: "internal/graph/bodystructure.go", Function: "attachRichHeadings", Needle: "Original: p.sourceSpan(h.Span)", Fault: "Original: h.Span"},
		{Name: "f3-rich-id", Property: "F3", Identity: "rich-id", File: "internal/graph/bodystructure.go", Function: "attachRichHeadings", Needle: "ID: len(f.richHeadings) + 1", Fault: "ID: len(f.richHeadings) + 2"},
		{Name: "f3-origin-position", Property: "F3", Identity: "origin-position", File: "internal/graph/bodystructure.go", Function: "attachRichHeadings", Needle: "Presentation: Span{Start: left, Stop: right}", Fault: "Presentation: Span{Start: left + 1, Stop: right}"},
		{Name: "f3-origin-synthetic", Property: "F3", Identity: "origin-synthetic", File: "internal/graph/bodystructure.go", Function: "attachRichHeadings", Needle: "Synthetic: piece.origin < 0", Fault: "Synthetic: true"},
		{Name: "f3-footnote-placement", Property: "F3", Identity: "footnote-placement", File: "internal/graph/bodystructure.go", Function: "collectStructureNode", Needle: "if h, ok := n.(*ast.Heading); ok", Fault: "if h, ok := n.(*ast.Heading); ok && n.Parent().Kind() == ast.KindDocument"},
		{Name: "f3-nil-zero", Property: "F3", Identity: "nil-source", File: "internal/graph/body.go", Function: "Source", Needle: "return \"\"", Fault: "return \"wrong\"", Package: "./internal/graph", ControlTest: "TestBodyFactsF3NilControl"},
		{Name: "f3-copy-handle", Property: "F3", Identity: "copied-source", File: "internal/graph/body.go", Function: "Source", Needle: "return f.data.source", Fault: "source := f.data.source; f.data.source = \"\"; return source", Package: "./internal/graph", ControlTest: "TestBodyFactsF3CopyControl"},
		{Name: "f3-early-stop", Property: "F3", Identity: "iterator-reuse", File: "internal/graph/body.go", Function: "bodyValues", Needle: "if !yield(value)", Fault: "values = values[1:]; if !yield(value)", Package: "./internal/graph", ControlTest: "TestBodyFactsF3EarlyStopControl"},
		{Name: "f3-context-invalid", Property: "F3", Identity: "invalid-context", File: "internal/graph/bodygrammar.go", Function: "bodyObservationIn", Needle: "panic(\"graph: invalid body observation context\")", Fault: "return nil", Package: "./internal/graph", ControlTest: "TestBodyFactsF3ContextControl"},
		{Name: "f3-consumer-parse", Property: "F3", Identity: "consumer-whole-body-parse", File: "internal/sequence/sequence.go", Function: "ParseFacts", Needle: "body := facts.Source()", Fault: "body := facts.Source(); var discarded strings.Builder; _ = graph.NewBodyMarkdown(nil).Convert([]byte(body), &discarded)"},
	}
}

func bodyValueMutationControl(t *testing.T, mode *agreementMutation) {
	t.Helper()
	switch mode.Name {
	case "f3-rich-raw", "f3-rich-level", "f3-rich-position", "f3-rich-origin", "f3-rich-role", "f3-rich-original", "f3-rich-id", "f3-origin-position", "f3-origin-synthetic", "f3-footnote-placement":
		bodyRichValueControl(t, mode)
		bodyRichHeadingControls(t)
	case "f3-checkbox":
		bodyCheckboxControl(t, mode)
	case "f3-consumer-parse":
		bodySimpleDocumentControl(t, mode, "## P {sequence=primary}\n\n- [[A]]\n", graph.Span{Start: 27, Stop: 32}, "")
		bodyConsumerOwnershipControl(t, *agreementSource)
	case "f3-paired", "f3-unpaired", "f3-diagnostic-rule", "f3-diagnostic-line", "f3-diagnostic-message", "f3-diagnostic-evidence":
		bodyEmphasisControl(t, mode)
	case "f3-row-span", "f3-target-span", "f3-anchor-span", "f3-candidate-text", "f3-candidate-target", "f3-candidate-aliased", "f3-candidate-fragment", "f3-candidate-state", "f3-candidate-line":
		bodyCoordinatesControl(t, mode)
	case "f3-row-role":
		bodySimpleDocumentControl(t, mode, "## P {sequence=primary}\n\n- [[A]] ^[note]\n", graph.Span{Start: 27, Stop: 40}, "^[note]")
	case "f3-gloss-text", "f3-gloss-code", "f3-soft-break", "f3-hard-break":
		bodyGlossControl(t, mode)
	case "f3-empty-outline":
		bodyEmptyOutlineControl(t, mode)
	default:
		bodyStructureValueControl(t, mode)
	}
}

func bodyValueCompare[T any](t *testing.T, mode *agreementMutation, want, got T) {
	t.Helper()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("caught: F3 %s (-want +got):\n%s", mode.Identity, diff)
	}
}

// No wanted value is obtained from another call to the producer.
func bodySimpleDocumentControl(t *testing.T, mode *agreementMutation, body string, span graph.Span, gloss string) {
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

func bodyGlossControl(t *testing.T, mode *agreementMutation) {
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

func bodyRichValueControl(t *testing.T, mode *agreementMutation) {
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

func bodyStructureValueControl(t *testing.T, mode *agreementMutation) {
	t.Helper()
	facts := graph.ReadBody("# Path\n\n- root\n  - child\n\n  tail\n\nend\n")
	bodyValueCompare(t, mode, []graph.BodyOutline{
		{Heading: graph.BodyOutlineHeading{Span: graph.Span{Start: 2, Stop: 6}, Level: 1}}, {ListID: 1}, {StrayID: 1},
	}, slices.Collect(facts.Outline()))
	bodyValueCompare(t, mode, []graph.BodyRow{{ID: 1, ListID: 1, ChildListID: 2, FirstBlockHasLines: true}}, slices.Collect(facts.ListRows(1)))
	bodyValueCompare(t, mode, []graph.BodyRow{{ID: 2, ListID: 2, ParentRowID: 1, FirstBlockHasLines: true}}, slices.Collect(facts.ListRows(2)))
	bodyValueCompare(t, mode, []graph.Span{{Start: 10, Stop: 15}, {Start: 28, Stop: 33}}, slices.Collect(facts.RowBlocks(1)))
	bodyValueCompare(t, mode, []graph.Span{{Start: 34, Stop: 38}}, slices.Collect(facts.StrayBlocks(1)))
	bodyValueCompare(t, mode, sequence.Document{}, sequence.ParseFacts(facts, 7))
	bodyCoordinatesControl(t, mode)
	bodyContinuationControl(t, mode)
	bodyNestedExclusionControl(t, mode)
	facts = graph.ReadBody("- > quoted\n  > - nested\n\n  after\n")
	bodyValueCompare(t, mode, []graph.BodyRow{{ID: 1, ListID: 1}}, slices.Collect(facts.ListRows(1)))
	bodyValueCompare(t, mode, []graph.Span{{Start: 4, Stop: 11}, {Start: 27, Stop: 33}}, slices.Collect(facts.RowBlocks(1)))
	// The stray fault must reach the recursive, container-descended branch.
	facts = graph.ReadBody("> stray {sequence=primary}\n")
	bodyValueCompare(t, mode, sequence.Document{Diagnostics: []sequence.Diagnostic{{Rule: sequence.RuleRoleMisplaced, Line: 7,
		Message: "a sequence marker here declares nothing; it is read only on a heading or on a row that opens a list", Evidence: "stray {sequence=primary}"}}}, sequence.ParseFacts(facts, 7))
}

func bodyEmphasisControl(t *testing.T, mode *agreementMutation) {
	t.Helper()
	for _, tt := range []struct {
		body        string
		candidate   sequence.Candidate
		diagnostics []sequence.Diagnostic
	}{
		{body: "## P {sequence=primary}\n\n- *[[A]]*\n", candidate: sequence.Candidate{Text: "A", Target: "A", Line: 9, Span: graph.Span{Start: 27, Stop: 34}, TargetSpan: graph.Span{Start: 28, Stop: 33}, State: sequence.EntryAccepted}},
		{body: "## P {sequence=primary}\n\n- *[[A]]\n", candidate: sequence.Candidate{Text: "A", Target: "A", Line: 9, Span: graph.Span{Start: 27, Stop: 33}, TargetSpan: graph.Span{Start: 28, Stop: 33}, State: sequence.EntryNoncanonical}, diagnostics: []sequence.Diagnostic{{Rule: sequence.RuleEntryNoncanonical, Line: 9,
			Message: "a lesson row opens with its link; move the link to the front, or take the row out of the course", Evidence: "*[[A]]"}}},
	} {
		want := sequence.Document{Groups: []*sequence.Group{{Name: "P", Level: 2, Line: 7, Role: sequence.RolePrimary, Items: []sequence.Item{{Entry: &tt.candidate}}}}, Diagnostics: tt.diagnostics}
		bodyValueCompare(t, mode, want, sequence.ParseFacts(graph.ReadBody(tt.body), 7))
	}
}

func bodyCheckboxControl(t *testing.T, mode *agreementMutation) {
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
func bodyCoordinatesControl(t *testing.T, mode *agreementMutation) {
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

func bodyEmptyOutlineControl(t *testing.T, mode *agreementMutation) {
	t.Helper()
	facts := graph.ReadBody("## P {sequence=primary}\n\n##\n\n- [[A]]\n")
	want := sequence.Document{Groups: []*sequence.Group{{Name: "P", Level: 2, Line: 7, Role: sequence.RolePrimary, Items: []sequence.Item{{Entry: &sequence.Candidate{
		Text: "A", Target: "A", Line: 11, Span: graph.Span{Start: 32, Stop: 37}, TargetSpan: graph.Span{Start: 32, Stop: 37}, State: sequence.EntryAccepted,
	}}}}}}
	bodyValueCompare(t, mode, want, sequence.ParseFacts(facts, 7))
}

func bodyContinuationControl(t *testing.T, mode *agreementMutation) {
	t.Helper()
	for _, tt := range []struct {
		body       string
		candidate  sequence.Candidate
		diagnostic sequence.Diagnostic
	}{
		{body: "## P {sequence=primary}\n\n- [[A]]\n\n  tail [[B]]\n", candidate: sequence.Candidate{Text: "[[A]]", Line: 9, Span: graph.Span{Start: 27, Stop: 33}, State: sequence.EntryMultiTarget}, diagnostic: sequence.Diagnostic{Rule: sequence.RuleEntryMultiTarget, Line: 9,
			Message: "a row naming more than one note does not say which lesson it is; give each lesson its own row", Evidence: "[[A]]"}},
		{body: "## P {sequence=primary}\n\n- [[A]]\n\n  tail {sequence=local}\n", candidate: sequence.Candidate{Text: "A", Target: "A", Line: 9, Span: graph.Span{Start: 27, Stop: 33}, TargetSpan: graph.Span{Start: 27, Stop: 32}, State: sequence.EntryAccepted}, diagnostic: sequence.Diagnostic{Rule: sequence.RuleRoleMisplaced, Line: 11,
			Message: "a sequence marker is read on the row's own line, not in its continuation", Evidence: "tail {sequence=local}"}},
	} {
		want := sequence.Document{Groups: []*sequence.Group{{Name: "P", Level: 2, Line: 7, Role: sequence.RolePrimary, Items: []sequence.Item{{Entry: &tt.candidate}}}}, Diagnostics: []sequence.Diagnostic{tt.diagnostic}}
		bodyValueCompare(t, mode, want, sequence.ParseFacts(graph.ReadBody(tt.body), 7))
	}
}

func bodyNestedExclusionControl(t *testing.T, mode *agreementMutation) {
	t.Helper()
	body := "## P {sequence=primary}\n\n- [[A]]\n\n  > - [[B]]\n\n  > tail\n"
	want := sequence.Document{Groups: []*sequence.Group{{Name: "P", Level: 2, Line: 7, Role: sequence.RolePrimary, Items: []sequence.Item{{Entry: &sequence.Candidate{
		Text: "A", Target: "A", Line: 9, Span: graph.Span{Start: 27, Stop: 33}, TargetSpan: graph.Span{Start: 27, Stop: 32}, State: sequence.EntryAccepted,
	}}}}}}
	bodyValueCompare(t, mode, want, sequence.ParseFacts(graph.ReadBody(body), 7))
}
