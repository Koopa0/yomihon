package judge_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
)

// Every spelling of a selected target belongs to a container fence's info
// field in both grammars. Prose and code contents cannot lend a missing carrier.
func agreementContainerInfoZones(body string, grammar goldmark.Markdown) []graph.Span {
	source := []byte(body)
	ctx := parser.NewContext()
	ctx.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := grammar.Parser().Parse(text.NewReader(source), parser.WithContext(ctx))
	var zones []graph.Span
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		fence, ok := node.(*ast.FencedCodeBlock)
		if !entering || !ok || fence.Info == nil {
			return ast.WalkContinue, nil
		}
		for parent := fence.Parent(); parent != nil; parent = parent.Parent() {
			switch parent.(type) {
			case *ast.ListItem, *ast.Blockquote:
				zones = append(zones, graph.Span{Start: fence.Info.Segment.Start, Stop: fence.Info.Segment.Stop})
				return ast.WalkContinue, nil
			}
		}
		return ast.WalkContinue, nil
	}); err != nil {
		panic(err)
	}
	return zones
}

func agreementExclusiveContainerInfoBudget(body string) map[agreementCitation]int {
	plain := agreementContainerInfoZones(body, agreementFootnoteGrammar)
	gfm := agreementContainerInfoZones(body, agreementExclusiveCodeGrammar)
	return agreementExclusiveFieldBudget(body, plain, gfm)
}

func agreementExclusiveContainerInfoDifference(c agreementCase, f *agreementFailure, actual *agreementHTML, budget map[agreementCitation]int) (kind, authority, wrong string) {
	for _, tuple := range actual.Citations {
		if tuple.Target == f.Tuple.Target {
			return "", "", ""
		}
	}
	return agreementContainerFenceDiagnosticDifference(c, f, budget)
}

func TestAgreementExclusiveContainerInfo(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	section := agreementCitation{Target: "A", Section: "A", State: "wikilink-broken"}
	raw := agreementCitation{Target: "A\\", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body   string
		want, public map[agreementCitation]int
		oldRefuses   bool
	}{
		{name: "recorded prefixed info", body: "## <em>A</em>\n> ```` go [[A]]\n ^A\n", want: map[agreementCitation]int{a: 1}, public: map[agreementCitation]int{a: 1}, oldRefuses: true},
		{name: "all fields and container declarations", body: "> ```go [[A]][[B]] [[A#A]] [[A]]\n> ```\n\n1. ```lang [[B|alias]]\n", want: map[agreementCitation]int{a: 2, b: 2, section: 1}, public: map[agreementCitation]int{a: 2, b: 2, section: 1}, oldRefuses: true},
		{name: "prose removes whole shared target", body: "[[A]]\n\n> ```go [[A]] [[B]]\n", want: map[agreementCitation]int{b: 1}, public: map[agreementCitation]int{b: 1}},
		{name: "contents remove whole shared target", body: "> ```go [[A]] [[B]]\n> [[A]]\n> ```\n", want: map[agreementCitation]int{b: 1}, public: map[agreementCitation]int{b: 1}},
		{name: "root info cannot lend ownership", body: "> ```go [[A]]\n> ```\n\n``` [[A]]\n"},
		{name: "raw suffix and alias fields", body: "> ```go [[A\\]] [[A\\|alias]]\n", want: map[agreementCitation]int{a: 1, raw: 1}, public: map[agreementCitation]int{a: 1, raw: 1}},
		{name: "escaped embed and local fields", body: "> ```go \\[[A]] ![[B]] [[#C]]\n", want: map[agreementCitation]int{a: 1, b: 1}, public: map[agreementCitation]int{b: 1}},
		{name: "live opener can suppress the diagnostic", body: "> [!note] [[B]]\n> ```go [[A]]\n> ```\n", want: map[agreementCitation]int{a: 1}},
		{name: "independent closed comment", body: "<!--closed-->\n> ```go [[A]]\n", want: map[agreementCitation]int{a: 1}, public: map[agreementCitation]int{a: 1}},
		{name: "comment owns apparent declaration", body: "<!--\n> ```go [[A]]\n-->\n"},
		{name: "unused declaration is another owner", body: "[^n]: words\n\n    > ```go [[A]]\n"},
		{name: "used definition retains native fence", body: "ref[^n]\n\n[^n]: words\n\n    > ```go [[A]]\n", want: map[agreementCitation]int{a: 1}, public: map[agreementCitation]int{a: 1}},
		{name: "common grammar alone retains native fence", body: "https://example.invalid/ref[^n]\n\n[^n]: words\n\n    > ```go [[A]]\n"},
		{name: "page grammar alone retains native fence", body: "https://example.invalid/` words ref[^n]\nclose`\n\n[^n]: words\n\n    > ```go [[A]]\n"},
		{name: "root declaration is another owner", body: "```go [[A]]\n"},
		{name: "indented example is another owner", body: "    > ```go [[A]]\n"},
		{name: "incomplete field refuses inventory", body: "> ```go [[A]] [[B\n"},
		{name: "nested field refuses inventory", body: "> ```go [[A[[B]]]] [[A]]\n"},
		{name: "block field refuses inventory", body: "> ```go [[A#^a]] [[B]]\n"},
		{name: "ordinary quote is not info", body: "> go [[A]]\n"},
		{name: "empty info is not contents", body: "> ```\n> [[A]]\n> ```\n"},
		{name: "nested quote counts once", body: "> > ```lang [[A]]\n", want: map[agreementCitation]int{a: 1}, public: map[agreementCitation]int{a: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			budget := agreementExclusiveContainerInfoBudget(tc.body)
			if diff := cmp.Diff(tc.want, budget); diff != "" {
				t.Fatalf("caught: complete container info source inventory (-want +got):\n%s", diff)
			}
			if tc.oldRefuses && len(agreementContainerFenceDiagnostics(tc.body)) != 0 {
				t.Fatal("caught: earlier container info reader borrowed compound fields")
			}
			c := agreementCase{Body: tc.body}
			r, actual := agreementIsolatedPage(t, c)
			found := make(map[agreementCitation]int)
			for _, f := range agreementPageFailures(tc.body, &r, &actual) {
				if f.Property != "P0" {
					continue
				}
				kind, authority, wrong := agreementExclusiveContainerInfoDifference(c, &f, &actual, budget)
				if tc.public[f.Tuple] == 0 {
					if kind != "" {
						t.Fatal("caught: container info borrowed unowned public tuple")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 fence declaration ownership" || wrong != "page-diagnostic" {
					t.Fatalf("caught: container info public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				found[f.Tuple] = f.Multiplicity
				for _, other := range []agreementCase{{Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "body"}}} {
					if kind, _, _ := agreementExclusiveContainerInfoDifference(other, &f, &actual, budget); kind != "" {
						t.Fatal("caught: container info borrowed vault context")
					}
				}
				for _, change := range []func(*agreementFailure){func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Direction = "unowned" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Cut = "unowned" }, func(f *agreementFailure) { f.Fragment = "unowned" }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = true }, func(f *agreementFailure) { f.ExcerptFound = true }} {
					changed := f
					change(&changed)
					if kind, _, _ := agreementExclusiveContainerInfoDifference(c, &changed, &actual, budget); kind != "" {
						t.Fatalf("caught: container info borrowed signature %s", agreementSignature(&changed))
					}
				}
				changed := actual
				changed.Citations = append(append([]agreementCitation(nil), actual.Citations...), f.Tuple)
				if kind, _, _ := agreementExclusiveContainerInfoDifference(c, &f, &changed, budget); kind != "" {
					t.Fatal("caught: container info borrowed page carrier")
				}
			}
			if len(found) == 0 {
				found = nil
			}
			if diff := cmp.Diff(tc.public, found); diff != "" {
				t.Fatalf("caught: complete container info public receipts (-want +got):\n%s", diff)
			}
		})
	}
}
