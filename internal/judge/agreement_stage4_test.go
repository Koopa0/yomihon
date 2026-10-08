package judge_test

import (
	"bytes"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

type stage4MissingFiles struct{}

func (stage4MissingFiles) MissingFile(string) bool { return true }

type stage4PageShape struct {
	Citations   []agreementCitation
	Code        []string
	Checkboxes  int
	URLs        []string
	URLLabels   []string
	LocalHrefs  []string
	Diagnostics []stage4Diagnostic
}

type stage4Diagnostic struct {
	Kind   render.DiagnosticKind
	Target string
}

// Read the actual HTML tree independently of the page's diagnostics. Code and
// task carriers are semantic elements, not highlighting class serialization.
func stage4Page(t *testing.T, body string) stage4PageShape {
	t.Helper()
	page := render.New(graph.BuildFromNotes(nil, nil), capturedBodies{}, noTitlesDeclared{}, stage4MissingFiles{})
	result := page.HTML("Notes/Subject.md", "", body, wording.En)
	actual := agreementObserve(t, result.HTML)
	if len(actual.Failures) != 0 || actual.CitationsInCode != 0 {
		t.Errorf("caught: S4 page-carriers failures=%+v code-citations=%d html=%q", actual.Failures, actual.CitationsInCode, result.HTML)
	}
	shape := stage4PageShape{Citations: actual.Citations}
	for _, diagnostic := range result.Diagnostics {
		shape.Diagnostics = append(shape.Diagnostics, stage4Diagnostic{Kind: diagnostic.Kind, Target: diagnostic.Target})
	}
	nodes, err := html.ParseFragment(strings.NewReader(result.HTML), nil)
	if err != nil {
		t.Fatalf("not-applied: page HTML parse: %v", err)
	}
	var text func(*html.Node) string
	text = func(node *html.Node) string {
		if node.Type == html.TextNode {
			return node.Data
		}
		var words strings.Builder
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			words.WriteString(text(child))
		}
		return words.String()
	}
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			switch node.Data {
			case "code":
				shape.Code = append(shape.Code, strings.TrimSuffix(text(node), "\n"))
			case "input":
				if agreementAttr(node, "type") == "checkbox" {
					shape.Checkboxes++
				}
			case "a":
				href := agreementAttr(node, "href")
				if href != "" && !strings.HasPrefix(href, "#") && !strings.HasPrefix(href, "https://") {
					shape.LocalHrefs = append(shape.LocalHrefs, href)
				}
				if strings.HasPrefix(href, "https://") {
					decoded, err := url.PathUnescape(href)
					if err != nil {
						t.Fatalf("not-applied: external href decode: %v", err)
					}
					shape.URLs = append(shape.URLs, decoded)
					var label strings.Builder
					for child := node.FirstChild; child != nil; child = child.NextSibling {
						if child.Type == html.TextNode {
							label.WriteString(child.Data)
						}
					}
					shape.URLLabels = append(shape.URLLabels, label.String())
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	for _, node := range nodes {
		walk(node)
	}
	return shape
}

func TestAgreementStage4Pages(t *testing.T) {
	cases := []struct {
		name string
		body string
		want stage4PageShape
	}{
		{name: "fence info subject", body: "``` [[Missing]]\nquoted\n```\n", want: stage4PageShape{Code: []string{"quoted"}}},
		{name: "fence info control", body: "[[ControlMissing]]\n", want: stage4WikiShape("ControlMissing")},
		{name: "footnote second paragraph", body: "ref[^n].\n\n[^n]: first paragraph.\n\n    [[Missing]]\n", want: stage4WikiShape("Missing")},
		{name: "footnote control", body: "[[ControlMissing]]\n", want: stage4WikiShape("ControlMissing")},
		{name: "ordinary indented code", body: "    [[Quoted]]\n", want: stage4PageShape{Code: []string{"[[Quoted]]"}}},
		{name: "task path subject", body: "- [ ](Missing.md)\n", want: stage4PageShape{Checkboxes: 1}},
		{name: "task path control", body: "[control](ControlMissing.md)\n", want: stage4PageShape{Citations: []agreementCitation{{Target: "ControlMissing.md", State: "wikilink-broken"}}, Diagnostics: []stage4Diagnostic{{Kind: render.DiagMarkdownBroken, Target: "ControlMissing.md"}}}},
		{name: "linkify code subject", body: "https://example.invalid/`Notes/Missing.md`\n", want: stage4PageShape{URLs: []string{"https://example.invalid/`Notes/Missing.md`"}, URLLabels: []string{"https://example.invalid/`Notes/Missing.md`"}}},
		{name: "linkify code control", body: "`Notes/ControlMissing.md`\n", want: stage4PageShape{Code: []string{"Notes/ControlMissing.md"}}},
	}
	for i := range cases {
		tc := &cases[i]
		t.Run(tc.name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want, stage4Page(t, tc.body)); diff != "" {
				t.Errorf("caught: S4 %s independent page (-want +got):\n%s", tc.name, diff)
			}
		})
	}
}

func stage4WikiShape(target string) stage4PageShape {
	return stage4PageShape{Citations: []agreementCitation{{Target: target, State: "wikilink-broken"}}, Diagnostics: []stage4Diagnostic{{Kind: render.DiagWikilinkBroken, Target: target}}}
}

func TestAgreementStage4FenceInfo(t *testing.T) {
	stage4NativeControl(t, "fence-info", "``` [[Missing]]\nquoted\n```\n", stage4PageShape{Code: []string{"quoted"}})
}

func TestAgreementStage4FootnoteParagraph(t *testing.T) {
	stage4NativeControl(t, "footnote-paragraph", "ref[^n].\n\n[^n]: first paragraph.\n\n    [[Missing]]\n", stage4WikiShape("Missing"))
}

func TestAgreementStage4TaskPath(t *testing.T) {
	stage4NativeControl(t, "task-path", "- [ ](Missing.md)\n", stage4PageShape{Checkboxes: 1})
}

func TestAgreementStage4LinkifyCode(t *testing.T) {
	stage4NativeControl(t, "linkify-code", "https://example.invalid/`Notes/Missing.md`\n", stage4PageShape{URLs: []string{"https://example.invalid/`Notes/Missing.md`"}, URLLabels: []string{"https://example.invalid/`Notes/Missing.md`"}})
}

func stage4NativeControl(t *testing.T, category, body string, want stage4PageShape) {
	t.Helper()
	if diff := cmp.Diff(want, stage4Page(t, body)); diff != "" {
		t.Errorf("caught: S4 %s independent page (-want +got):\n%s", category, diff)
	}
	root := agreementAttributionRoot(t)
	if err := os.CopyFS(root, os.DirFS("testdata/vault-page-"+category)); err != nil {
		t.Fatalf("not-applied: native fixture copy: %v", err)
	}
	wire, err := os.ReadFile("testdata/golden/page-" + category + ".jsonl")
	if err != nil {
		t.Fatalf("not-applied: native golden read: %v", err)
	}
	findings, err := judge.Check(t.Context(), root)
	if err != nil {
		t.Fatalf("not-applied: native public Check: %v", err)
	}
	var encoded bytes.Buffer
	if err := judge.WriteJSONL(&encoded, findings); err != nil {
		t.Fatalf("not-applied: native WriteJSONL: %v", err)
	}
	if !bytes.Equal(wire, encoded.Bytes()) {
		t.Errorf("caught: S4 %s public Check complete wire=%q want=%q findings=%+v", category, encoded.Bytes(), wire, findings)
	}
	controlWire, _, found := bytes.Cut(wire, []byte("\n"))
	if !found || len(controlWire) == 0 {
		t.Fatalf("not-applied: native golden has no complete control line: category=%s", category)
	}
	controlWire = append(bytes.Clone(controlWire), '\n')
	subjectWarn := 0
	if category == "footnote-paragraph" {
		subjectWarn = 1
	}
	for _, selection := range []struct {
		paths []string
		wire  []byte
		warn  int
	}{
		{paths: []string{"Notes/Subject.md"}, wire: wire[len(controlWire):], warn: subjectWarn},
		{paths: []string{"Notes/Control.md"}, wire: controlWire, warn: 1},
		{wire: wire, warn: 1},
	} {
		for _, deny := range []struct {
			tokens []string
			exit   int
		}{
			{},
			{tokens: []string{"warn"}, exit: selection.warn},
			{tokens: []string{"error"}},
		} {
			stdout, exit, err := judge.RunCheck(t.Context(), &judge.CheckOptions{Root: root, Paths: selection.paths, Format: judge.FormatJSON, Deny: deny.tokens})
			if err != nil {
				t.Fatalf("not-applied: native RunCheck: %v", err)
			}
			if exit != deny.exit || !bytes.Equal(stdout, selection.wire) {
				t.Errorf("caught: S4 %s public RunCheck paths=%q deny=%q exit=%d want=%d wire=%q want=%q", category, selection.paths, deny.tokens, exit, deny.exit, stdout, selection.wire)
			}
		}
	}
	if *agreementSource != "" {
		t.Logf("AGREEMENT-SOURCE-CONSUMED sha256=%s", agreementMutationSourceDigest(t, *agreementSource))
	}
	t.Logf("AGREEMENT-INVOKED S4/stage4-%s", category)
}

func TestAgreementStage4RetiredWitnesses(t *testing.T) {
	for _, body := range []string{"[^unused]: [[A]]\n", "``` [[A]]\n"} {
		stage4RetiredControl(t, body)
		c := agreementCase{Body: body}
		retired := agreementFailure{Property: "P1", Identity: "citation-occurrences", Tuple: agreementCitation{Target: "A"}, Direction: "judge-only", Multiplicity: 1}
		if kind, authority, wrong := agreementKnownDifference(c, &retired); kind != "" || authority != "" || wrong != "" {
			t.Errorf("caught: S4 retired-waiver body=%q kind=%q authority=%q wrong=%q", body, kind, authority, wrong)
		}
	}
}

func TestAgreementStage4UnusedFootnote(t *testing.T) {
	stage4RetiredControl(t, "[^unused]: [[A]]\n")
	if *agreementSource != "" {
		t.Logf("AGREEMENT-SOURCE-CONSUMED sha256=%s", agreementMutationSourceDigest(t, *agreementSource))
	}
	t.Log("AGREEMENT-INVOKED S4/stage4-unused-footnote")
}

func TestAgreementStage4RemainingDebt(t *testing.T) {
	for _, tc := range []struct {
		name      string
		body      string
		want      []agreementFailure
		authority string
		wrong     string
	}{
		{name: "unused footnote P0", body: "[^unused]: [[A]]\n", want: []agreementFailure{{Property: "P0", Identity: "diagnostic-html", Tuple: agreementCitation{Target: "A", State: "wikilink-broken"}, Direction: "diagnostic-only", Multiplicity: 1}}, authority: "#1011 stage 4", wrong: "page-diagnostic"},
		{name: "fence info has no debt", body: "``` [[A]]\n"},
		{name: "multiline code P1 and P2", body: "`open\n[[A]]\nclose`", want: []agreementFailure{{Property: "P1", Identity: "citation-occurrences", Tuple: agreementCitation{Target: "A"}, Direction: "page-only", Multiplicity: 1}, {Property: "P2", Identity: "wikilink-in-code", Tuple: agreementCitation{Target: "A", State: "wikilink-broken"}, Direction: "page-in-code", Multiplicity: 1}}, authority: "#1011 stage 4", wrong: "page"},
		{name: "duplicate heading P4", body: "## A\n## A\n", want: []agreementFailure{{Property: "P4", Identity: "literal-heading-id", Fragment: "a-2", Direction: "page-only", Multiplicity: 1, PagePresent: true}}, authority: "#1011 stage 8", wrong: "judge"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := agreementCase{Body: tc.body}
			result, actual := agreementIsolatedPage(t, c)
			failures := agreementPageFailures(c.Body, &result, &actual)
			fragments := agreementFragmentFailures(t, []agreementCase{c}, []agreementHTML{actual})
			failures = append(failures, fragments[0]...)
			for i := range failures {
				failure := &failures[i]
				kind, authority, wrong := agreementKnownDifference(c, failure)
				if kind != "debt" || authority != tc.authority || wrong != tc.wrong {
					t.Errorf("caught: S4 remaining-debt identity=%+v kind=%q authority=%q wrong=%q want debt/%q/%q", *failure, kind, authority, wrong, tc.authority, tc.wrong)
				}
				failure.Observation = ""
			}
			if diff := cmp.Diff(tc.want, failures); diff != "" {
				t.Errorf("caught: S4 remaining-debt complete identities (-want +got):\n%s", diff)
			}
		})
	}
}

func stage4RetiredControl(t *testing.T, body string) {
	t.Helper()
	root := agreementAttributionRoot(t)
	const frontmatter = "---\ntitle: Stage 4 subject\ntype: writing\ndomain: golang\nstatus: draft\ncreated: 2026-01-01\nupdated: 2026-01-01\n---\n"
	agreementWrite(t, root, "Notes/Subject.md", []byte(frontmatter+body))
	findings, err := judge.Check(t.Context(), root)
	if err != nil {
		t.Fatalf("not-applied: retired public Check: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("caught: S4 unused-footnote public Check body=%q complete findings=%+v want none", body, findings)
	}
	if targets := judge.LinkTargets(body); len(targets) != 0 {
		t.Errorf("caught: S4 unused-footnote LinkTargets(%q)=%q want none", body, targets)
	}
	result, actual := agreementIsolatedPage(t, agreementCase{Body: body})
	if len(actual.Citations) != 0 || len(actual.Failures) != 0 || actual.CitationsInCode != 0 {
		t.Errorf("caught: S4 unused-footnote page body=%q observed=%+v html=%q", body, actual, result.HTML)
	}
	// Diagnostics retain the separately named P0 debt; they cannot stand in
	// for the independent HTML observation of live citations above.
	for _, deny := range [][]string{nil, {"warn"}, {"error"}} {
		stdout, exit, err := judge.RunCheck(t.Context(), &judge.CheckOptions{Root: root, Format: judge.FormatJSON, Deny: deny})
		if err != nil {
			t.Fatalf("not-applied: retired RunCheck: %v", err)
		}
		if exit != 0 || !bytes.Equal(stdout, nil) {
			t.Errorf("caught: S4 unused-footnote RunCheck body=%q deny=%q exit=%d wire=%q want 0/empty", body, deny, exit, stdout)
		}
	}
}
