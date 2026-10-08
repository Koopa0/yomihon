package judge_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/net/html"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

const stage5Frontmatter = "---\ntitle: Stage 5 subject\ntype: writing\ndomain: golang\nstatus: draft\ncreated: 2026-01-01\nupdated: 2026-01-01\n---\n"

const stage5Wire9 = `{"rule_id":"link.broken","severity":"warn","path":"Notes/Subject.md","line":9,"message":"[[Missing]] resolves to no note","evidence":"no filename or alias matches the target","suggested_action":"create the target note, or change the link to an existing filename/alias","source_rule":"yomihon","target":"Missing","fingerprint":"v1:a1fa3e713737c040"}` + "\n"
const stage5Wire10 = `{"rule_id":"link.broken","severity":"warn","path":"Notes/Subject.md","line":10,"message":"[[Missing]] resolves to no note","evidence":"no filename or alias matches the target","suggested_action":"create the target note, or change the link to an existing filename/alias","source_rule":"yomihon","target":"Missing","fingerprint":"v1:a1fa3e713737c040"}` + "\n"
const stage5Wire11 = `{"rule_id":"link.broken","severity":"warn","path":"Notes/Subject.md","line":11,"message":"[[Missing]] resolves to no note","evidence":"no filename or alias matches the target","suggested_action":"create the target note, or change the link to an existing filename/alias","source_rule":"yomihon","target":"Missing","fingerprint":"v1:a1fa3e713737c040"}` + "\n"
const stage5Wire12 = `{"rule_id":"link.broken","severity":"warn","path":"Notes/Subject.md","line":12,"message":"[[Missing]] resolves to no note","evidence":"no filename or alias matches the target","suggested_action":"create the target note, or change the link to an existing filename/alias","source_rule":"yomihon","target":"Missing","fingerprint":"v1:a1fa3e713737c040"}` + "\n"
const stage5TitleWire = `{"rule_id":"callout.title_markup","severity":"info","path":"Notes/Subject.md","line":9,"message":"this callout title carries markup the page escapes as visible text","evidence":"the title is written as [[N]] on a recognised callout's opening line","suggested_action":"move the markup into the callout body, or write the title as plain text","source_rule":"yomihon","target":"[[N]]","fingerprint":"v1:d474c555275a718a"}` + "\n"

type stage5Carrier struct {
	Tag, Class, Href, Title, Text string
	Explanation                 string
	InCode                      bool
}

// Shape is read from the actual HTML, independently of the diagnostic rail.
// Roles retain document order; code text retains spaces and line endings.
type stage5Shape struct {
	Text          string
	Roles         []string
	Code          []string
	IDs           []string
	Carriers      []stage5Carrier
	FootnoteIDs   []string
	FootnoteHrefs []string
	Blocks        []string
	Diagnostics   []render.Diagnostic
	Transcluded   bool
}

func stage5NodeText(node *html.Node) string {
	if node.Type == html.TextNode {
		return node.Data
	}
	if node.Type == html.ElementNode && (node.Data == "svg" || agreementClass(node, "y-offscreen")) {
		return ""
	}
	var out strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		out.WriteString(stage5NodeText(child))
	}
	return out.String()
}

// Block boundaries separate reading text even when the HTML writer puts
// adjacent paragraph tags on the same line. Code assertions use the exact
// node text instead, so this whitespace fold cannot hide a code-text defect.
func stage5DocumentText(node *html.Node) string {
	if node.Type == html.TextNode {
		return node.Data
	}
	if node.Type == html.ElementNode && (node.Data == "svg" || agreementClass(node, "y-offscreen")) {
		return ""
	}
	var out strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		out.WriteString(stage5DocumentText(child))
	}
	switch node.Data {
	case "p", "pre", "div", "ul", "ol", "li", "blockquote", "table", "tr", "th", "td", "hr", "details", "summary":
		return " " + out.String() + " "
	}
	return out.String()
}

func stage5Observe(t *testing.T, result *render.Result) stage5Shape {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(result.HTML))
	if err != nil {
		t.Fatalf("not-applied: S5 parse actual HTML: %v", err)
	}
	shape := stage5Shape{
		Text:        strings.Join(strings.Fields(stage5DocumentText(doc)), " "),
		Blocks:      result.Blocks, Diagnostics: result.Diagnostics,
		Transcluded: result.TranscludedIdentity != "",
	}
	var walk func(*html.Node, bool)
	walk = func(node *html.Node, inCode bool) {
		if node.Type == html.ElementNode {
			if node.Data == "svg" {
				return
			}
			inCode = inCode || node.Data == "code" || node.Data == "pre"
			id := agreementAttr(node, "id")
			if id != "" {
				shape.IDs = append(shape.IDs, id)
				if strings.HasPrefix(id, "fn") {
					shape.FootnoteIDs = append(shape.FootnoteIDs, id)
				}
			}
			switch node.Data {
			case "p", "pre", "code", "ul", "ol", "li", "blockquote", "table", "thead", "tbody", "tr", "th", "td", "hr", "details", "summary":
				role := node.Data
				if class := agreementAttr(node, "class"); class == "callout-title" || strings.HasPrefix(class, "embed__") {
					role += "." + class
				}
				shape.Roles = append(shape.Roles, role)
			case "div":
				shape.Roles = append(shape.Roles, "div."+agreementAttr(node, "class"))
			}
			if node.Data == "code" {
				shape.Code = append(shape.Code, stage5NodeText(node))
			}
			if agreementCarrier(node) {
				var explanation strings.Builder
				for child := node.FirstChild; child != nil; child = child.NextSibling {
					if agreementClass(child, "y-offscreen") {
						for words := child.FirstChild; words != nil; words = words.NextSibling {
							explanation.WriteString(stage5NodeText(words))
						}
					}
				}
				shape.Carriers = append(shape.Carriers, stage5Carrier{
					Tag: node.Data, Class: agreementAttr(node, "class"), Href: agreementAttr(node, "href"),
					Title: agreementAttr(node, "title"), Text: stage5NodeText(node), Explanation: explanation.String(), InCode: inCode,
				})
			}
			if href := agreementAttr(node, "href"); strings.HasPrefix(href, "#fn") {
				shape.FootnoteHrefs = append(shape.FootnoteHrefs, href)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child, inCode)
		}
	}
	walk(doc, false)
	return shape
}

func stage5MissingCarrier() stage5Carrier {
	return stage5Carrier{Tag: "span", Class: "wikilink-broken", Title: "There is no note called \"Missing\" yet", Text: "Missing", Explanation: " (There is no note called \"Missing\" yet)"}
}

func stage5MissingDiagnostic() render.Diagnostic {
	return render.Diagnostic{Kind: "wikilink-broken", Target: "Missing", Message: "wikilink \"Missing\" does not resolve to any note or file", Section: "", Block: ""}
}

func stage5MissingFinding(line int) judge.Finding {
	return judge.Finding{
		RuleID: "link.broken", Severity: judge.SeverityWarn, Path: "Notes/Subject.md", Line: new(line),
		Message: "[[Missing]] resolves to no note", Evidence: "no filename or alias matches the target",
		SuggestedAction: "create the target note, or change the link to an existing filename/alias",
		SourceRule: "yomihon", Target: new("Missing"), Fingerprint: "v1:a1fa3e713737c040",
	}
}

func stage5Live(text string, roles ...string) stage5Shape {
	return stage5Shape{Text: text, Roles: roles, Carriers: []stage5Carrier{stage5MissingCarrier()}, Diagnostics: []render.Diagnostic{stage5MissingDiagnostic()}}
}

type stage5Literal struct {
	Name     string
	Body     string
	Want     stage5Shape
	Targets  []string
	Findings []judge.Finding
	Wire     string
	WarnExit int
}

func stage5PublicCheck(t *testing.T, category, body string, bodies capturedBodies, targets []string, findings []judge.Finding, wire string, warnExit int) {
	t.Helper()
	if diff := cmp.Diff(targets, judge.LinkTargets(body), cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("caught: S5 %s LinkTargets (-want +got):\n%s", category, diff)
	}
	root := agreementAttributionRoot(t)
	agreementWrite(t, root, "Notes/Subject.md", []byte(stage5Frontmatter+body))
	for path, contents := range bodies {
		agreementWrite(t, root, path, []byte(stage5Frontmatter+contents))
	}
	actual, err := judge.Check(t.Context(), root)
	if err != nil {
		t.Fatalf("not-applied: S5 Check: %v", err)
	}
	if diff := cmp.Diff(findings, actual, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("caught: S5 %s complete Check (-want +got):\n%s", category, diff)
	}
	var encoded bytes.Buffer
	if err := judge.WriteJSONL(&encoded, actual); err != nil {
		t.Fatalf("not-applied: S5 WriteJSONL: %v", err)
	}
	if diff := cmp.Diff(wire, encoded.String()); diff != "" {
		t.Errorf("caught: S5 %s complete Check wire (-want +got):\n%s", category, diff)
	}
	for _, scope := range []struct {
		name  string
		paths []string
	}{
		{name: "whole vault"},
		{name: "subject", paths: []string{"Notes/Subject.md"}},
	} {
		for _, deny := range []struct {
			name   string
			tokens []string
			warn   bool
		}{
			{name: "no deny"},
			{name: "warn", tokens: []string{"warn"}, warn: true},
			{name: "error", tokens: []string{"error"}},
		} {
			stdout, exit, err := judge.RunCheck(t.Context(), &judge.CheckOptions{Root: root, Paths: scope.paths, Format: judge.FormatJSON, Deny: deny.tokens})
			if err != nil {
				t.Fatalf("not-applied: S5 RunCheck %s/%s: %v", scope.name, deny.name, err)
			}
			wantExit := 0
			if deny.warn {
				wantExit = warnExit
			}
			want := struct {
				Wire string
				Exit int
			}{Wire: wire, Exit: wantExit}
			got := struct {
				Wire string
				Exit int
			}{Wire: string(stdout), Exit: exit}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("caught: S5 %s complete RunCheck %s/%s (-want +got):\n%s", category, scope.name, deny.name, diff)
			}
		}
	}
}

func stage5Literals(t *testing.T, category string, cases []stage5Literal) {
	t.Helper()
	bodies := capturedBodies{"Notes/N.md": "note\n"}
	page := render.New(graph.BuildFromNotes([]graph.NoteInput{{RelPath: "Notes/N.md"}}, nil), bodies, noTitlesDeclared{}, everyFileHeld{})
	for i := range cases {
		tc := &cases[i]
		t.Run(tc.Name, func(t *testing.T) {
			result := page.HTML("Notes/Subject.md", "", tc.Body, wording.En)
			if diff := cmp.Diff(tc.Want, stage5Observe(t, &result), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("caught: S5 %s actual HTML and complete rail (-want +got):\n%s\nHTML=%q", category, diff, result.HTML)
			}
			stage5PublicCheck(t, category, tc.Body, bodies, tc.Targets, tc.Findings, tc.Wire, tc.WarnExit)
		})
	}
	if *agreementSource != "" {
		t.Logf("AGREEMENT-SOURCE-CONSUMED sha256=%s", agreementMutationSourceDigest(t, *agreementSource))
	}
	t.Logf("AGREEMENT-INVOKED S5/stage5-%s", category)
}

// Each subject catches the named production guard being dropped; its paired
// prose control prevents a converter that suppresses every citation passing.
func TestAgreementStage5(t *testing.T) {
	t.Run("stage5-percent-role", func(t *testing.T) {
		stage5Literals(t, "percent-role", []stage5Literal{
			{Name: "comment prefix cannot open fence", Body: "%%x%%````\n[[Missing]]\n", Want: stage5Live("```` Missing", "p"), Targets: []string{"Missing"}, Findings: []judge.Finding{stage5MissingFinding(10)}, WarnExit: 1, Wire: stage5Wire10},
			{Name: "ordinary prefix control", Body: "prefix````\n[[Missing]]\n", Want: stage5Live("prefix```` Missing", "p"), Targets: []string{"Missing"}, Findings: []judge.Finding{stage5MissingFinding(10)}, WarnExit: 1, Wire: stage5Wire10},
			{Name: "hidden continuation remains nonblank", Body: "para\n%%x%%\n    [[Missing]]\n", Want: stage5Live("para Missing", "p"), Targets: []string{"Missing"}, Findings: []judge.Finding{stage5MissingFinding(11)}, WarnExit: 1, Wire: stage5Wire11},
			{Name: "visible continuation control", Body: "para\nvisible\n    [[Missing]]\n", Want: stage5Live("para visible Missing", "p"), Targets: []string{"Missing"}, Findings: []judge.Finding{stage5MissingFinding(11)}, WarnExit: 1, Wire: stage5Wire11},
		})
	})
	t.Run("stage5-html-role", func(t *testing.T) {
		stage5Literals(t, "html-role", []stage5Literal{
			{Name: "HTML block suffix", Body: "<!-- x -->    [[Missing]]\n", Want: stage5Live("Missing"), Targets: []string{"Missing"}, Findings: []judge.Finding{stage5MissingFinding(9)}, WarnExit: 1, Wire: stage5Wire9},
			{Name: "ordinary paragraph control", Body: "[[Missing]]\n", Want: stage5Live("Missing", "p"), Targets: []string{"Missing"}, Findings: []judge.Finding{stage5MissingFinding(9)}, WarnExit: 1, Wire: stage5Wire9},
		})
	})
	t.Run("stage5-percent-code", func(t *testing.T) {
		stage5Literals(t, "percent-code", []stage5Literal{
			{Name: "percent opener in indented code", Body: "    %%\n\n[[Missing]]\n", Want: stage5PercentCode("%%"), Targets: []string{"Missing"}, Findings: []judge.Finding{stage5MissingFinding(11)}, WarnExit: 1, Wire: stage5Wire11},
			{Name: "literal code control", Body: "    literal\n\n[[Missing]]\n", Want: stage5PercentCode("literal"), Targets: []string{"Missing"}, Findings: []judge.Finding{stage5MissingFinding(11)}, WarnExit: 1, Wire: stage5Wire11},
		})
	})
	t.Run("stage5-html-code", func(t *testing.T) {
		stage5Literals(t, "html-code", []stage5Literal{
			{Name: "escaped ticks do not protect HTML comment", Body: "x \\`<!--\\` [[Missing]] -->\n", Want: stage5Shape{Text: "x `", Roles: []string{"p"}}},
			{Name: "actual code protects HTML opener", Body: "x `<!--` [[Missing]]\n", Want: stage5HTMLCode(), Targets: []string{"Missing"}, Findings: []judge.Finding{stage5MissingFinding(9)}, WarnExit: 1, Wire: stage5Wire9},
		})
	})
	t.Run("stage5-list-fence", func(t *testing.T) {
		stage5Literals(t, "list-fence", []stage5Literal{
			{Name: "resolved name in list fence", Body: "- a\n\n    ```\n    [[N]]\n    ```\n", Want: stage5Shape{Text: "a [[N]]", Roles: []string{"ul", "li", "p", "pre", "code"}, Code: []string{"[[N]]\n"}, Diagnostics: []render.Diagnostic{{Kind: "risky-fence", Message: "wikilink/callout/table syntax found inside a fenced code block; left untouched"}}}},
			{Name: "missing name in list fence", Body: "- a\n\n    ```\n    [[Missing]]\n    ```\n", Want: stage5Shape{Text: "a [[Missing]]", Roles: []string{"ul", "li", "p", "pre", "code"}, Code: []string{"[[Missing]]\n"}, Diagnostics: []render.Diagnostic{{Kind: "risky-fence", Message: "wikilink/callout/table syntax found inside a fenced code block; left untouched"}}}},
			{Name: "list prose control", Body: "- [[Missing]]\n", Want: stage5Live("Missing", "ul", "li"), Targets: []string{"Missing"}, Findings: []judge.Finding{stage5MissingFinding(9)}, WarnExit: 1, Wire: stage5Wire9},
		})
	})
	t.Run("stage5-wrapped-code", func(t *testing.T) {
		stage5Literals(t, "wrapped-code", []stage5Literal{
			{Name: "wrapped span", Body: "Text `a\n[[N]] b` end\n", Want: stage5Shape{Text: "Text a [[N]] b end", Roles: []string{"p", "code"}, Code: []string{"a [[N]] b"}}},
			{Name: "wrapped prose control", Body: "Text a\n[[Missing]] b end\n", Want: stage5Live("Text a Missing b end", "p"), Targets: []string{"Missing"}, Findings: []judge.Finding{stage5MissingFinding(10)}, WarnExit: 1, Wire: stage5Wire10},
			{Name: "mixed code and prose", Body: "before `[[N]]` after [[Missing]]\n", Want: stage5MixedCode(), Targets: []string{"Missing"}, Findings: []judge.Finding{stage5MissingFinding(9)}, WarnExit: 1, Wire: stage5Wire9},
			{Name: "mixed prose control", Body: "before [[N]] after [[Missing]]\n", Want: stage5MixedProse(), Targets: []string{"N", "Missing"}, Findings: []judge.Finding{stage5MissingFinding(9)}, WarnExit: 1, Wire: stage5Wire9},
			{Name: "repeated prose occurrences", Body: "[[Missing]] [[Missing]]\n", Want: stage5Repeated(), Targets: []string{"Missing", "Missing"}, Findings: []judge.Finding{stage5MissingFinding(9), stage5MissingFinding(9)}, WarnExit: 1, Wire: stage5Wire9 + stage5Wire9},
		})
	})
	t.Run("stage5-unused-definition", func(t *testing.T) {
		stage5Literals(t, "unused-definition", []stage5Literal{
			{Name: "unused definition has no rail or receipt", Body: "[^unused]: [[Missing]]\n"},
			{Name: "unused embed cannot create receipt", Body: "[^unused]: ![[N]]\n"},
			{Name: "referenced definition control", Body: "ref[^n]\n\n[^n]: [[Missing]]\n", Want: stage5Footnote("ref1"), Targets: []string{"Missing"}, Findings: []judge.Finding{stage5MissingFinding(11)}, WarnExit: 1, Wire: stage5Wire11},
			{Name: "surviving embed control", Body: "![[N]]\n", Want: stage5Shape{Text: "From N note", Roles: []string{"div.embed", "p.embed__source", "p"}, Transcluded: true}, Targets: []string{"N"}},
		})
	})
	t.Run("stage5-title-footnote", func(t *testing.T) {
		stage5Literals(t, "title-footnote", []stage5Literal{
			{Name: "escaped title cannot activate definition", Body: "> [!note] ref[^n]\n> body\n\n[^n]: [[Missing]]\n", Want: stage5Shape{Text: "ref[^n] body", Roles: []string{"div.callout callout-note", "p.callout-title", "div.callout-body", "p"}}},
			{Name: "escaped title cannot activate embed receipt", Body: "> [!note] ref[^n]\n> body\n\n[^n]: ![[N]]\n", Want: stage5Shape{Text: "ref[^n] body", Roles: []string{"div.callout callout-note", "p.callout-title", "div.callout-body", "p"}}},
			{Name: "body reference activates definition", Body: "> [!note] title\n> body ref[^n]\n\n[^n]: [[Missing]]\n", Want: stage5CalloutFootnote(), Targets: []string{"Missing"}, Findings: []judge.Finding{stage5MissingFinding(12)}, WarnExit: 1, Wire: stage5Wire12},
			{Name: "title markup retains complete info finding", Body: "> [!note] [[N]]\n> body\n", Want: stage5Shape{Text: "[[N]] body", Roles: []string{"div.callout callout-note", "p.callout-title", "div.callout-body", "p"}}, Targets: []string{"N"}, Findings: []judge.Finding{{RuleID: "callout.title_markup", Severity: judge.SeverityInfo, Path: "Notes/Subject.md", Line: new(9), Message: "this callout title carries markup the page escapes as visible text", Evidence: "the title is written as [[N]] on a recognised callout's opening line", SuggestedAction: "move the markup into the callout body, or write the title as plain text", SourceRule: "yomihon", Target: new("[[N]]"), Fingerprint: "v1:d474c555275a718a"}}, Wire: stage5TitleWire},
		})
	})
	t.Run("stage5-fence-info", func(t *testing.T) {
		stage5Literals(t, "fence-info", []stage5Literal{
			{Name: "quote fence info is not prose", Body: "> ``` [[Missing]]\n> quoted\n> ```\n", Want: stage5Shape{Text: "quoted", Roles: []string{"blockquote", "pre", "code"}, Code: []string{"quoted\n"}}},
			{Name: "quote prose control", Body: "> [[Missing]]\n", Want: stage5Live("Missing", "blockquote", "p"), Targets: []string{"Missing"}, Findings: []judge.Finding{stage5MissingFinding(9)}, WarnExit: 1, Wire: stage5Wire9},
			{Name: "list fence info is not prose", Body: "- a\n\n    ``` [[Missing]]\n    quoted\n    ```\n", Want: stage5Shape{Text: "a quoted", Roles: []string{"ul", "li", "p", "pre", "code"}, Code: []string{"quoted\n"}}},
			{Name: "list prose control", Body: "- [[Missing]]\n", Want: stage5Live("Missing", "ul", "li"), Targets: []string{"Missing"}, Findings: []judge.Finding{stage5MissingFinding(9)}, WarnExit: 1, Wire: stage5Wire9},
		})
	})
	t.Run("stage5-coordinates", func(t *testing.T) {
		stage5Literals(t, "coordinates", []stage5Literal{
			{Name: "CRLF and multibyte original line", Body: "\u7532\r\n%%hidden\r\nbody%% [[Missing]]\r\n", Want: stage5Live("\u7532 Missing", "p"), Targets: []string{"Missing"}, Findings: []judge.Finding{stage5MissingFinding(11)}, WarnExit: 1, Wire: stage5Wire11},
			{Name: "CRLF visible control", Body: "\u7532\r\nvisible\r\nbody [[Missing]]\r\n", Want: stage5Live("\u7532 visible body Missing", "p"), Targets: []string{"Missing"}, Findings: []judge.Finding{stage5MissingFinding(11)}, WarnExit: 1, Wire: stage5Wire11},
		})
	})
	t.Run("stage5-marker-authority", func(t *testing.T) {
		stage5Literals(t, "marker-authority", []stage5Literal{
			{Name: "authored inline families cannot copy citation", Body: "\ue0000\ue001 \ue0020\ue003 \ue004 [[Missing]] ^live\n", Want: stage5ForgedShape(), Targets: []string{"Missing"}, Findings: []judge.Finding{stage5MissingFinding(9)}, WarnExit: 1, Wire: stage5Wire9},
			{Name: "encoded inline families cannot copy citation", Body: "&#xE000;0&#xE001; &#xE002;0&#xE003; &#xE004; [[Missing]] ^live\n", Want: stage5ForgedShape(), Targets: []string{"Missing"}, Findings: []judge.Finding{stage5MissingFinding(9)}, WarnExit: 1, Wire: stage5Wire9},
			{Name: "ordinary numbers and caret control", Body: "0 0 [[Missing]] ^live\n", Want: stage5ForgedShape(), Targets: []string{"Missing"}, Findings: []judge.Finding{stage5MissingFinding(9)}, WarnExit: 1, Wire: stage5Wire9},
			{Name: "authored block index stays visible text", Body: "<!--yomihon-block:0-->\n\n[[Missing]]\n", Want: stage5Live("<!--yomihon-block:0--> Missing", "p", "p"), Targets: []string{"Missing"}, Findings: []judge.Finding{stage5MissingFinding(11)}, WarnExit: 1, Wire: stage5Wire11},
			{Name: "escaped block spelling control", Body: "&lt;!--yomihon-block:0-->\n\n[[Missing]]\n", Want: stage5Live("<!--yomihon-block:0--> Missing", "p", "p"), Targets: []string{"Missing"}, Findings: []judge.Finding{stage5MissingFinding(11)}, WarnExit: 1, Wire: stage5Wire11},
		})
	})
	t.Run("stage5-indent-page", stage5IndentPage)
	t.Run("stage5-indent-judge", stage5IndentJudge)
	t.Run("stage5-indent-excerpt", stage5IndentExcerpt)
}

func stage5PercentCode(code string) stage5Shape {
	want := stage5Live(code+" Missing", "pre", "code", "p")
	want.Code = []string{code + "\n"}
	return want
}

func stage5HTMLCode() stage5Shape {
	want := stage5Live("x <!-- Missing", "p", "code")
	want.Code = []string{"<!--"}
	return want
}

func stage5MixedCode() stage5Shape {
	want := stage5Live("before [[N]] after Missing", "p", "code")
	want.Code = []string{"[[N]]"}
	return want
}

func stage5MixedProse() stage5Shape {
	want := stage5Live("before N after Missing", "p")
	want.Carriers = []stage5Carrier{{Tag: "a", Class: "wikilink", Href: "/notes/Notes/N.md", Text: "N"}, stage5MissingCarrier()}
	return want
}

func stage5Repeated() stage5Shape {
	want := stage5Live("Missing Missing", "p")
	want.Carriers = []stage5Carrier{stage5MissingCarrier(), stage5MissingCarrier()}
	want.Diagnostics = []render.Diagnostic{stage5MissingDiagnostic(), stage5MissingDiagnostic()}
	return want
}

func stage5Footnote(reference string) stage5Shape {
	want := stage5Live(reference+" Missing \u21a9\ufe0e", "p", "div.footnotes", "hr", "ol", "li", "p")
	want.IDs = []string{"fnref:1", "fn:1"}
	want.FootnoteIDs = []string{"fnref:1", "fn:1"}
	want.FootnoteHrefs = []string{"#fn:1", "#fnref:1"}
	return want
}

func stage5CalloutFootnote() stage5Shape {
	want := stage5Footnote("title body ref1")
	want.Roles = []string{"div.callout callout-note", "p.callout-title", "div.callout-body", "p", "div.footnotes", "hr", "ol", "li", "p"}
	return want
}

func stage5ForgedShape() stage5Shape {
	want := stage5Live("0 0 Missing ^live", "p")
	want.IDs = []string{"^live"}
	want.Blocks = []string{"^live"}
	return want
}

// These address controls catch an indented-code guard being removed at each
// public sink. The prose control reaches the same address without code.
func stage5IndentPage(t *testing.T) {
	stage5Literals(t, "indent-page", []stage5Literal{
		{Name: "indented address is code", Body: "para\n\n    sample ^ind\n", Want: stage5Shape{Text: "para sample ^ind", Roles: []string{"p", "pre", "code"}, Code: []string{"sample ^ind\n"}}},
		{Name: "plain address remains reachable", Body: "sample ^ind\n", Want: stage5Shape{Text: "sample ^ind", Roles: []string{"p"}, IDs: []string{"^ind"}, Blocks: []string{"^ind"}}},
	})
}

const stage5LinkBlockWire = `{"rule_id":"link.block_missing","severity":"warn","path":"Notes/Subject.md","line":9,"message":"[[Target#^ind]] resolves, but no line carries the address ^ind","evidence":"the note exists and no line in it ends with the block address","suggested_action":"fix the block name after ^, or write the address at the end of the intended line","source_rule":"yomihon","target":"Target#^ind","resolved_to":"Notes/Target.md","fingerprint":"v1:6357add4bd5262f2"}` + "\n"
const stage5EmbedBlockWire = `{"rule_id":"embed.block_missing","severity":"warn","path":"Notes/Subject.md","line":9,"message":"![[Target#^ind]] resolves, but no line carries the address ^ind","evidence":"the note exists and no line in it ends with the block address, so there is no excerpt to cut","suggested_action":"fix the block name after ^, or write the address at the end of the line the excerpt should show","source_rule":"yomihon","target":"Target#^ind","resolved_to":"Notes/Target.md","fingerprint":"v1:fd698055d8b9ffed"}` + "\n"

func stage5IndentJudge(t *testing.T) {
	for _, tc := range []struct {
		name       string
		body       string
		targetBody string
		want       stage5Shape
		findings   []judge.Finding
		wire       string
		warnExit   int
	}{
		{
			name: "link rejects code address", body: "[[Target#^ind]]\n", targetBody: "para\n\n    sample ^ind\n",
			want: stage5Shape{
				Text: "Target", Roles: []string{"p"},
				Carriers: []stage5Carrier{{Tag: "a", Class: "wikilink wikilink-degraded", Href: "/notes/Notes/Target.md", Title: "That block was not found; the link now points at the whole note", Text: "Target", Explanation: " (That block was not found; the link now points at the whole note)"}},
				Diagnostics: []render.Diagnostic{{Kind: "link-fragment-missing", Target: "Target", Block: "ind", Message: "no block in \"Notes/Target.md\" matched \"^ind\"; the link leads to the note itself"}},
			},
			findings: []judge.Finding{{RuleID: "link.block_missing", Severity: judge.SeverityWarn, Path: "Notes/Subject.md", Line: new(9), Message: "[[Target#^ind]] resolves, but no line carries the address ^ind", Evidence: "the note exists and no line in it ends with the block address", SuggestedAction: "fix the block name after ^, or write the address at the end of the intended line", SourceRule: "yomihon", Target: new("Target#^ind"), ResolvedTo: new("Notes/Target.md"), Fingerprint: "v1:6357add4bd5262f2"}},
			wire: stage5LinkBlockWire, warnExit: 1,
		},
		{
			name: "embed withholds code address", body: "![[Target#^ind]]\n", targetBody: "para\n\n    sample ^ind\n",
			want: stage5Shape{Text: "From Target Unable to find \"#^ind\" in Target.", Roles: []string{"div.embed embed--withheld", "p.embed__source", "p.embed__note"}, Transcluded: true, Diagnostics: []render.Diagnostic{{Kind: "embed-fragment-missing", Target: "Target", Block: "ind", Message: "no block in \"Notes/Target.md\" matched \"^ind\"; the excerpt is withheld"}}},
			findings: []judge.Finding{{RuleID: "embed.block_missing", Severity: judge.SeverityWarn, Path: "Notes/Subject.md", Line: new(9), Message: "![[Target#^ind]] resolves, but no line carries the address ^ind", Evidence: "the note exists and no line in it ends with the block address, so there is no excerpt to cut", SuggestedAction: "fix the block name after ^, or write the address at the end of the line the excerpt should show", SourceRule: "yomihon", Target: new("Target#^ind"), ResolvedTo: new("Notes/Target.md"), Fingerprint: "v1:fd698055d8b9ffed"}},
			wire: stage5EmbedBlockWire, warnExit: 1,
		},
		{
			name: "link prose control", body: "[[Target#^ind]]\n", targetBody: "sample ^ind\n",
			want: stage5Shape{Text: "Target", Roles: []string{"p"}, Carriers: []stage5Carrier{{Tag: "a", Class: "wikilink", Href: "/notes/Notes/Target.md#^ind", Text: "Target"}}},
		},
		{
			name: "embed prose control", body: "![[Target#^ind]]\n", targetBody: "sample ^ind\n",
			want: stage5Shape{Text: "From Target sample ^ind", Roles: []string{"div.embed", "p.embed__source", "p"}, Transcluded: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bodies := capturedBodies{"Notes/Target.md": tc.targetBody}
			page := render.New(graph.BuildFromNotes([]graph.NoteInput{{RelPath: "Notes/Target.md"}}, nil), bodies, noTitlesDeclared{}, everyFileHeld{})
			result := page.HTML("Notes/Subject.md", "", tc.body, wording.En)
			if diff := cmp.Diff(tc.want, stage5Observe(t, &result), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("caught: S5 indent-judge actual fragment HTML and full rail (-want +got):\n%s\nHTML=%q", diff, result.HTML)
			}
			stage5PublicCheck(t, "indent-judge", tc.body, bodies, []string{"Target"}, tc.findings, tc.wire, tc.warnExit)
		})
	}
	if *agreementSource != "" {
		t.Logf("AGREEMENT-SOURCE-CONSUMED sha256=%s", agreementMutationSourceDigest(t, *agreementSource))
	}
	t.Log("AGREEMENT-INVOKED S5/stage5-indent-judge")
}

func stage5IndentExcerpt(t *testing.T) {
	for _, tc := range []struct {
		name, body, cut string
		found           bool
	}{
		{name: "indented code refuses", body: "para\n\n    sample ^ind\n"},
		{name: "plain address bounded cut", body: "before\n\nsample ^ind\n\nafter\n", cut: "sample ^ind", found: true},
		{name: "wrapped code refuses", body: "`first\n sample ^ind\nlast`\n"},
		{name: "fenced code refuses", body: "```\nsample ^ind\n```\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cut, found := render.Excerpt(tc.body, "^ind")
			preview, previewFound, narrowed := render.ExcerptPreview(tc.body, "^ind")
			want := struct {
				Cut, Preview                 string
				Found, PreviewFound, Narrowed bool
			}{Cut: tc.cut, Preview: tc.cut, Found: tc.found, PreviewFound: tc.found}
			got := struct {
				Cut, Preview                 string
				Found, PreviewFound, Narrowed bool
			}{Cut: cut, Preview: preview, Found: found, PreviewFound: previewFound, Narrowed: narrowed}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("caught: S5 indent-excerpt complete exact cut (-want +got):\n%s", diff)
			}
		})
	}
	if *agreementSource != "" {
		t.Logf("AGREEMENT-SOURCE-CONSUMED sha256=%s", agreementMutationSourceDigest(t, *agreementSource))
	}
	t.Log("AGREEMENT-INVOKED S5/stage5-indent-excerpt")
}
