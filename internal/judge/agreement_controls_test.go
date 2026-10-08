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
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/wording"
)

// The corpus's existence comparison cannot detect an excerpt that silently
// widens while still returning true. These independent literals lock its cut.
func agreementExcerptCuts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		body     string
		fragment string
		want     string
		found    bool
	}{
		{name: "multiline bounded paragraph", body: "first line\ncontinued ^a\n\nsecond ^b\n", fragment: "^a", want: "first line\ncontinued ^a", found: true},
		{name: "first duplicate", body: "first ^a\n\nsecond ^a\n", fragment: "^a", want: "first ^a", found: true},
		{name: "missing does not widen", body: "first ^a\n\nsecond\n", fragment: "^absent"},
		// An inline footnote is prose, so it grants no block address or excerpt.
		{name: "inline footnote grants no block cut", body: "paragraph ^[literal]\n", fragment: "^[literal]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cut, found := render.Excerpt(tc.body, tc.fragment)
			got := struct {
				Cut   string
				Found bool
			}{Cut: cut, Found: found}
			want := struct {
				Cut   string
				Found bool
			}{Cut: tc.want, Found: tc.found}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("caught: P3 bounded-excerpt body=%q fragment=%q (-want +got):\n%s", tc.body, tc.fragment, diff)
			}
		})
	}
}

func agreementOrderedOccurrences(t *testing.T) {
	t.Parallel()
	body := "[[B|alias]] [[A]] [[A]] ![[B]]\n"
	page := render.New(graph.BuildFromNotes(nil, nil), capturedBodies{}, noTitlesDeclared{}, everyFileHeld{})
	result := page.HTML("Notes/Reading.md", "", body, wording.En)
	actual := agreementObserve(t, result.HTML)
	var targets []string
	for _, citation := range actual.Citations {
		targets = append(targets, citation.Target)
	}
	want := []string{"B", "A", "A", "B"}
	if diff := cmp.Diff(want, targets); diff != "" {
		t.Errorf("caught: P1 citation-order page (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(want, judge.LinkTargets(body)); diff != "" {
		t.Errorf("caught: P1 citation-order check (-want +got):\n%s", diff)
	}
}

func agreementNoticeProjection(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		reason string
		want   agreementCitation
	}{
		{name: "missing note", reason: `There is no note called "A" yet`, want: agreementCitation{Target: "A", State: "wikilink-broken"}},
		{name: "missing file", reason: `There is no file called "A.pdf" yet`, want: agreementCitation{Target: "A.pdf", State: "wikilink-broken"}},
		{name: "missing heading", reason: `There is no note called "A" yet; what follows "#" was read as the section "B"`, want: agreementCitation{Target: "A", Section: "B", State: "wikilink-broken"}},
		{name: "outside path", reason: `"../../../etc/passwd.md" leaves the vault; the link text remains`, want: agreementCitation{SourceRole: agreementOutsideMarkdown, Target: "../../../etc/passwd.md", State: "wikilink-broken"}},
		{name: "outside raw quotation", reason: `"../a"b\c.md" leaves the vault; the link text remains`, want: agreementCitation{SourceRole: agreementOutsideMarkdown, Target: `../a"b\c.md`, State: "wikilink-broken"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := agreementNotice(tc.reason)
			if err != nil {
				t.Fatalf("notice setup: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("caught: notice identity (-want +got):\n%s", diff)
			}
		})
	}
	for _, reason := range []string{
		`unknown notice`,
		`There is no note called "A"`,
		`There is no note called "A" yet extra`,
		`../../../etc/passwd.md" leaves the vault; the link text remains`,
		`"" leaves the vault; the link text remains`,
		`"../../../etc/passwd.md" leaves the vault; the link text remains extra`,
	} {
		if got, err := agreementNotice(reason); err == nil {
			t.Errorf("caught: unsupported notice %q accepted as %+v", reason, got)
		}
	}
}

type agreementProjectionCase struct {
	Name        string
	Body        string
	Citations   []agreementCitation
	Diagnostics []render.Diagnostic
	Elements    []agreementControlElement
	WikiTargets []string
}

// Literal expectations cover the whole carrier partition, diagnostic identity,
// actual noninteractive shape and ordered wiki occurrences independently.
func agreementProjectionCases() []agreementProjectionCase {
	return []agreementProjectionCase{
		{
			Name:        "outside-markdown",
			Body:        "[out](../../../etc/passwd.md)\n",
			Citations:   []agreementCitation{{SourceRole: agreementOutsideMarkdown, Target: "../../../etc/passwd.md", State: "wikilink-broken"}},
			Diagnostics: []render.Diagnostic{{Kind: render.DiagMarkdownBroken, Target: "../../../etc/passwd.md", Message: `Markdown path "../../../etc/passwd.md" leaves the vault`}},
			Elements:    []agreementControlElement{{Tag: "span", Class: "wikilink-broken", Title: `"../../../etc/passwd.md" leaves the vault; the link text remains`}},
			WikiTargets: []string{},
		},
		{
			Name: "outside-markdown-with-wiki",
			Body: "[out](../../../etc/passwd.md) [[A]] [[A]]\n",
			Citations: []agreementCitation{
				{SourceRole: agreementOutsideMarkdown, Target: "../../../etc/passwd.md", State: "wikilink-broken"},
				{Target: "A", State: "wikilink-broken"},
				{Target: "A", State: "wikilink-broken"},
			},
			Diagnostics: []render.Diagnostic{
				{Kind: render.DiagWikilinkBroken, Target: "A", Message: `wikilink "A" does not resolve to any note or file`},
				{Kind: render.DiagWikilinkBroken, Target: "A", Message: `wikilink "A" does not resolve to any note or file`},
				{Kind: render.DiagMarkdownBroken, Target: "../../../etc/passwd.md", Message: `Markdown path "../../../etc/passwd.md" leaves the vault`},
			},
			Elements: []agreementControlElement{
				{Tag: "span", Class: "wikilink-broken", Title: `"../../../etc/passwd.md" leaves the vault; the link text remains`},
				{Tag: "span", Class: "wikilink-broken", Title: `There is no note called "A" yet`},
				{Tag: "span", Class: "wikilink-broken", Title: `There is no note called "A" yet`},
			},
			WikiTargets: []string{"A", "A"},
		},
		{
			Name: "outside-markdown-refuter",
			Body: "[[../../../etc/passwd.md]] [[A]]\n",
			Citations: []agreementCitation{
				{Target: "../../../etc/passwd.md", State: "wikilink-broken"},
				{Target: "A", State: "wikilink-broken"},
			},
			Diagnostics: []render.Diagnostic{
				{Kind: render.DiagWikilinkBroken, Target: "../../../etc/passwd.md", Message: `wikilink "../../../etc/passwd.md" does not resolve to any note or file`},
				{Kind: render.DiagWikilinkBroken, Target: "A", Message: `wikilink "A" does not resolve to any note or file`},
			},
			Elements: []agreementControlElement{
				{Tag: "span", Class: "wikilink-broken", Title: `There is no note called "../../../etc/passwd.md" yet`},
				{Tag: "span", Class: "wikilink-broken", Title: `There is no note called "A" yet`},
			},
			WikiTargets: []string{"../../../etc/passwd.md", "A"},
		},
		{
			Name:        "outside-envelope-identity",
			Body:        "[out](<../../../a\"b.md?raw=\\c>)\n",
			Citations:   []agreementCitation{{SourceRole: agreementOutsideMarkdown, Target: `../../../a"b.md?raw=\c`, State: "wikilink-broken"}},
			Diagnostics: []render.Diagnostic{{Kind: render.DiagMarkdownBroken, Target: `../../../a"b.md?raw=\c`, Message: `Markdown path "../../../a\"b.md?raw=\\c" leaves the vault`}},
			Elements:    []agreementControlElement{{Tag: "span", Class: "wikilink-broken", Title: `"../../../a"b.md?raw=\c" leaves the vault; the link text remains`}},
			WikiTargets: []string{},
		},
	}
}

func TestAgreementProjectionControl(t *testing.T) {
	cases := agreementProjectionCases()
	for i := range cases {
		tc := &cases[i]
		t.Run(tc.Name, func(t *testing.T) {
			agreementProjectionControl(t, tc)
		})
	}
}

func agreementProjectionControl(t *testing.T, tc *agreementProjectionCase) {
	t.Helper()
	page := render.New(graph.BuildFromNotes(nil, nil), capturedBodies{}, noTitlesDeclared{}, everyFileHeld{})
	result := page.HTML("Notes/Reading.md", "", tc.Body, wording.En)
	actual := agreementObserve(t, result.HTML)
	if diff := cmp.Diff(tc.Citations, actual.Citations); diff != "" {
		t.Errorf("caught: P0 carrier-partition case=%s (-want +got):\n%s", tc.Name, diff)
	}
	if diff := cmp.Diff(tc.Elements, agreementControlElements(t, result.HTML)); diff != "" {
		t.Errorf("caught: P0 citation-shape case=%s (-want +got):\n%s", tc.Name, diff)
	}
	if diff := cmp.Diff(tc.Diagnostics, result.Diagnostics); diff != "" {
		t.Errorf("caught: P0 projection-diagnostics case=%s (-want +got):\n%s", tc.Name, diff)
	}
	pageTargets := make([]string, 0, len(actual.Citations))
	for _, citation := range actual.Citations {
		if citation.SourceRole == "" {
			pageTargets = append(pageTargets, citation.Target)
		}
	}
	if diff := cmp.Diff(tc.WikiTargets, pageTargets); diff != "" {
		t.Errorf("caught: P1 citation-occurrences literal page case=%s (-want +got):\n%s", tc.Name, diff)
	}
	if diff := cmp.Diff(tc.WikiTargets, judge.LinkTargets(tc.Body)); diff != "" {
		t.Errorf("caught: P1 citation-occurrences literal check case=%s (-want +got):\n%s", tc.Name, diff)
	}
	failures := agreementPageFailures(tc.Body, &result, &actual)
	for failureIndex := range failures {
		failure := &failures[failureIndex]
		t.Errorf("caught: %s %s case=%s body=%q observations=%s", failure.Property, failure.Identity, tc.Name, tc.Body, failure.Observation)
	}
}

func agreementKnownControls(t *testing.T) {
	t.Parallel()
	t.Run("title is no resolution key", agreementTitleControl)
	t.Run("ambiguous names never guess", agreementAmbiguityControl)
	t.Run("media has resource authority", agreementMediaControl)
	t.Run("factory carrier shapes", agreementFactoryShapes)
	bodies := capturedBodies{"Notes/A.md": "## Present\n\nfirst ^a\n", "Notes/Child.md": "## Child Heading\n\n[[A|child alias]] [[A]] `[[A]]`\n\nchild ^child\n"}
	idx := graph.BuildFromNotes([]graph.NoteInput{{RelPath: "Notes/A.md"}, {RelPath: "Notes/Child.md"}}, nil)
	page := render.New(idx, bodies, noTitlesDeclared{}, everyFileHeld{})
	t.Run("quoted known citation", func(t *testing.T) {
		result := page.HTML("Notes/Reading.md", "", "`[[A]]`\n\n``` go\n[[A]]\n```\n\n[[A]]\n", wording.En)
		actual := agreementObserveKnown(t, result.HTML, map[string]string{"Notes/A.md": "A"})
		if actual.CitationsInCode != 0 {
			t.Errorf("caught: P2 wikilink-in-code html=%q", result.HTML)
		}
		want := []agreementCitation{{Target: "A", State: "wikilink"}}
		if diff := cmp.Diff(want, actual.Citations); diff != "" {
			t.Errorf("caught: P1 citation-occurrences known control (-want +got):\n%s", diff)
		}
	})
	for _, tc := range []struct {
		name string
		body string
		kind render.DiagnosticKind
		href string
	}{
		{name: "missing section retains address", body: "[[A#Missing]]", kind: render.DiagLinkSectionMissing, href: "/notes/Notes/A.md#missing"},
		{name: "missing block withdraws address", body: "[[A#^missing]]", kind: render.DiagLinkFragmentMissing, href: "/notes/Notes/A.md"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := page.HTML("Notes/Reading.md", "", tc.body, wording.En)
			agreementObserveKnown(t, result.HTML, map[string]string{"Notes/A.md": "A"})
			if len(result.Diagnostics) != 1 || result.Diagnostics[0].Kind != tc.kind || result.Diagnostics[0].Target != "A" {
				t.Errorf("caught: P0 diagnostic-html diagnostics=%+v", result.Diagnostics)
			}
			doc, err := html.Parse(strings.NewReader(result.HTML))
			if err != nil {
				t.Fatalf("parse degraded HTML: %v", err)
			}
			var hrefs []string
			var walk func(*html.Node)
			walk = func(n *html.Node) {
				if agreementClass(n, "wikilink-degraded") {
					if n.Data != "a" || !agreementClass(n, "wikilink") || agreementAttr(n, "title") == "" {
						t.Errorf("caught: P0 diagnostic-html malformed degraded control: %q", result.HTML)
					}
					hrefs = append(hrefs, agreementAttr(n, "href"))
				}
				for child := n.FirstChild; child != nil; child = child.NextSibling {
					walk(child)
				}
			}
			walk(doc)
			if diff := cmp.Diff([]string{tc.href}, hrefs); diff != "" {
				t.Errorf("caught: P0 diagnostic-html degraded address (-want +got):\n%s", diff)
			}
		})
	}
	t.Run("embedded literal heading belongs to host", func(t *testing.T) {
		c := agreementCase{Name: "control/transcluded-heading", Body: "![[Notes/Child]]\n", Companions: bodies}
		result, actual := agreementTransclusionControl(t, c.Body, bodies)
		if len(actual.Headings) == 0 {
			t.Fatal("transclusion control emitted no heading")
		}
		if len(result.Diagnostics) != 0 {
			t.Errorf("caught: P0 transclusion-diagnostics diagnostics=%+v", result.Diagnostics)
		}
		agreementFragments(t, []agreementCase{c}, []agreementHTML{actual})
	})
}

type agreementDeclaredTitles map[string][]string

func (titles agreementDeclaredTitles) TitledBy(target string) []string {
	return titles[target]
}

func agreementFactoryShapes(t *testing.T) {
	idx := graph.BuildFromNotes([]graph.NoteInput{{RelPath: "Notes/A.md"}}, nil)
	page := render.New(idx, capturedBodies{"Notes/A.md": "## Present\n"}, noTitlesDeclared{}, everyFileHeld{})
	cases := []struct {
		name string
		body string
		want agreementControlElement
	}{
		{name: "resolved", body: "[[A]]", want: agreementControlElement{Tag: "a", Class: "wikilink", Href: "/notes/Notes/A.md"}},
		{name: "local", body: "## Present\n[[#Present]]", want: agreementControlElement{Tag: "a", Class: "wikilink", Href: "#present"}},
		{name: "degraded", body: "[[A#Missing]]", want: agreementControlElement{Tag: "a", Class: "wikilink wikilink-degraded", Href: "/notes/Notes/A.md#missing", Title: "No section called \"Missing\" was found; the link lands at the top of the note"}},
		{name: "broken", body: "[[Absent]]", want: agreementControlElement{Tag: "span", Class: "wikilink-broken", Title: "There is no note called \"Absent\" yet"}},
	}
	for i := range cases {
		tc := &cases[i]
		t.Run(tc.name, func(t *testing.T) {
			result := page.HTML("Notes/Reading.md", "", tc.body, wording.En)
			if diff := cmp.Diff([]agreementControlElement{tc.want}, agreementControlElements(t, result.HTML)); diff != "" {
				t.Errorf("caught: P0 citation-shape factory=%s (-want +got):\n%s", tc.name, diff)
			}
			agreementObserveKnown(t, result.HTML, map[string]string{"Notes/A.md": "A"})
		})
	}
	// Title-only and ambiguous factories have independent literal ledger
	// controls above; their shapes go through the same carrier guard there.
}

type agreementProvenance struct {
	SourceTag   string
	SourceClass string
	AnchorTag   string
	AnchorClass string
	Href        string
	Path        string
	Attributes  []html.Attribute
}

// An expanded embed contributes its provenance once to the host. Live child
// citations belong to the child, so they are compared with its separate render
// and LinkTargets, never appended to the host's source citation list.
func agreementTransclusionControl(t *testing.T, hostBody string, bodies capturedBodies) (render.Result, agreementHTML) {
	t.Helper()
	idx := graph.BuildFromNotes([]graph.NoteInput{{RelPath: "Notes/A.md"}, {RelPath: "Notes/Child.md"}}, nil)
	page := render.New(idx, bodies, noTitlesDeclared{}, everyFileHeld{})
	result := page.HTML("Notes/Reading.md", "", hostBody, wording.En)
	doc, err := html.Parse(strings.NewReader(result.HTML))
	if err != nil {
		t.Fatalf("parse host transclusion HTML: %v", err)
	}
	var provenance []agreementProvenance
	var containers []agreementControlElement
	var sources []agreementControlElement
	var childHTML bytes.Buffer
	observeEmbed := func(n *html.Node) {
		containers = append(containers, agreementControlElement{Tag: n.Data, Class: agreementAttr(n, "class")})
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if !agreementClass(child, "embed__source") {
				if renderErr := html.Render(&childHTML, child); renderErr != nil {
					t.Fatalf("serialize owned child HTML: %v", renderErr)
				}
				continue
			}
			sources = append(sources, agreementControlElement{Tag: child.Data, Class: agreementAttr(child, "class")})
			for anchor := child.FirstChild; anchor != nil; anchor = anchor.NextSibling {
				if anchor.Type != html.ElementNode {
					continue
				}
				href := agreementAttr(anchor, "href")
				parsed, parseErr := url.Parse(href)
				if parseErr != nil {
					t.Errorf("caught: P1 provenance-identity malformed href=%q: %v", href, parseErr)
					continue
				}
				provenance = append(provenance, agreementProvenance{
					SourceTag:   child.Data,
					SourceClass: agreementAttr(child, "class"),
					AnchorTag:   anchor.Data,
					AnchorClass: agreementAttr(anchor, "class"),
					Href:        href,
					Path:        strings.TrimPrefix(parsed.Path, "/notes/"),
					Attributes:  anchor.Attr,
				})
			}
		}
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && agreementClass(n, "embed") {
			observeEmbed(n)
			return
		}
		if n.Type == html.ElementNode && agreementCarrier(n) {
			t.Errorf("caught: P1 provenance-identity unexpected host carrier outside child ownership: tag=%q href=%q", n.Data, agreementAttr(n, "href"))
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	wantContainers := []agreementControlElement{{Tag: "div", Class: "embed"}}
	if diff := cmp.Diff(wantContainers, containers); diff != "" {
		t.Errorf("caught: P1 provenance-identity embed ownership (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]agreementControlElement{{Tag: "p", Class: "embed__source"}}, sources); diff != "" {
		t.Errorf("caught: P1 provenance-identity source count/ownership (-want +got):\n%s", diff)
	}
	wantProvenance := []agreementProvenance{{
		SourceTag:   "p",
		SourceClass: "embed__source",
		AnchorTag:   "a",
		Href:        "/notes/Notes/Child.md",
		Path:        "Notes/Child.md",
		Attributes:  []html.Attribute{{Key: "href", Val: "/notes/Notes/Child.md"}},
	}}
	if diff := cmp.Diff(wantProvenance, provenance); diff != "" {
		t.Errorf("caught: P1 provenance-identity host source (-want +got):\n%s", diff)
	}
	var hostTargets []string
	for _, citation := range provenance {
		hostTargets = append(hostTargets, strings.TrimSuffix(citation.Path, ".md"))
	}
	if diff := cmp.Diff(judge.LinkTargets(hostBody), hostTargets); diff != "" {
		t.Errorf("caught: P1 provenance-identity host occurrences (-judge +page):\n%s", diff)
	}
	childBody, held := bodies["Notes/Child.md"]
	if !held {
		t.Fatal("transclusion control has no independent child body")
	}
	known := map[string]string{"Notes/A.md": "A"}
	embedded := agreementObserveKnown(t, childHTML.String(), known)
	childResult := page.HTML("Notes/Child.md", "", childBody, wording.En)
	separate := agreementObserveKnown(t, childResult.HTML, known)
	wantChild := []agreementCitation{{Target: "A", State: "wikilink"}, {Target: "A", State: "wikilink"}}
	if diff := cmp.Diff(wantChild, separate.Citations); diff != "" {
		t.Errorf("caught: P1 child-citation-occurrences independent child (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(separate.Citations, embedded.Citations); diff != "" {
		t.Errorf("caught: P1 child-citation-occurrences owned subtree (-separate +embedded):\n%s", diff)
	}
	var childTargets []string
	for _, citation := range embedded.Citations {
		childTargets = append(childTargets, citation.Target)
	}
	if diff := cmp.Diff(judge.LinkTargets(childBody), childTargets); diff != "" {
		t.Errorf("caught: P1 child-citation-occurrences (-judge +embedded):\n%s", diff)
	}
	if embedded.CitationsInCode != 0 || separate.CitationsInCode != 0 {
		t.Errorf("caught: P2 wikilink-in-code embedded=%d separate=%d", embedded.CitationsInCode, separate.CitationsInCode)
	}
	if len(childResult.Diagnostics) != 0 {
		t.Errorf("caught: P0 child-diagnostics diagnostics=%+v", childResult.Diagnostics)
	}
	return result, agreementObserveKnown(t, result.HTML, known)
}

type agreementControlElement struct {
	Tag   string
	Class string
	Href  string
	Title string
	Src   string
	Alt   string
}

// These controls observe their actual declared classes/attributes directly.
// An ambiguity explanation is never passed to the missing-notice decoder.
func agreementControlElements(t *testing.T, text string) []agreementControlElement {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(text))
	if err != nil {
		t.Fatalf("parse attribution control HTML: %v", err)
	}
	var elements []agreementControlElement
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && agreementCarrier(n) {
			if _, shapeErr := agreementCarrierState(n); shapeErr != nil {
				t.Errorf("caught: P0 citation-shape error=%v html=%q", shapeErr, text)
			}
		}
		if n.Type == html.ElementNode && (n.Data == "a" || n.Data == "img" ||
			agreementClass(n, "wikilink-broken") || agreementClass(n, "wikilink-ambiguous") ||
			agreementClass(n, "embed-media")) {
			elements = append(elements, agreementControlElement{
				Tag:   n.Data,
				Class: agreementAttr(n, "class"),
				Href:  agreementAttr(n, "href"),
				Title: agreementAttr(n, "title"),
				Src:   agreementAttr(n, "src"),
				Alt:   agreementAttr(n, "alt"),
			})
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return elements
}

func agreementAttributionRoot(t agreementTB) string {
	t.Helper()
	root := t.TempDir()
	data, err := os.ReadFile("../schema/testdata/contract.toml")
	if err != nil {
		t.Fatalf("read attribution contract: %v", err)
	}
	needle := `knowledge_dirs = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`
	if strings.Count(string(data), needle) != 1 {
		t.Fatal("attribution contract knowledge declaration is not unique")
	}
	text := strings.Replace(string(data), needle, `knowledge_dirs = ["Notes"]`, 1) +
		"\n[privacy]\nnever_egress_dirs = []\n"
	agreementWrite(t, root, schema.ContractRelPath, []byte(text))
	return root
}

type agreementCheckEvidence struct {
	Rule       judge.RuleID
	Path       string
	Target     string
	ResolvedTo string
	Members    []string
}

func agreementAttributionFindings(t *testing.T, root, probe, collision string) []agreementCheckEvidence {
	t.Helper()
	findings, err := judge.Check(t.Context(), root)
	if err != nil {
		t.Fatalf("public attribution Check: %v", err)
	}
	var observed []agreementCheckEvidence
	for i := range findings {
		finding := &findings[i]
		if finding.RuleID == "scan.unreadable" || finding.RuleID == "scan.skipped" {
			t.Fatalf("attribution Check incomplete: %+v", *finding)
		}
		isProbe := finding.Path == probe && strings.HasPrefix(string(finding.RuleID), "link.")
		isCollision := collision != "" && finding.RuleID == "collision.name" &&
			finding.Target != nil && *finding.Target == collision
		if !isProbe && !isCollision {
			continue
		}
		if finding.Target == nil {
			t.Fatalf("attribution finding has no target: %+v", *finding)
		}
		evidence := agreementCheckEvidence{
			Rule:    finding.RuleID,
			Path:    finding.Path,
			Target:  *finding.Target,
			Members: finding.CollisionMembers,
		}
		if finding.ResolvedTo != nil {
			evidence.ResolvedTo = *finding.ResolvedTo
		}
		observed = append(observed, evidence)
	}
	return observed
}

func agreementTitleControl(t *testing.T) {
	const probe = "Notes/title-probe.md"
	const body = "[[Declared Title|reader words]]\n"
	root := agreementAttributionRoot(t)
	agreementWrite(t, root, probe, agreementEnvelope(t, body))
	wantMissing := []agreementCheckEvidence{{Rule: "link.broken", Path: probe, Target: "Declared Title"}}
	if diff := cmp.Diff(wantMissing, agreementAttributionFindings(t, root, probe, "")); diff != "" {
		t.Fatalf("title probe missing-target receipt (-want +got):\n%s", diff)
	}
	agreementWrite(t, root, "Notes/holder.md", []byte("---\ntitle: Declared Title\n---\nbody\n"))
	idx := graph.BuildFromNotes([]graph.NoteInput{{RelPath: "Notes/holder.md"}}, nil)
	if got := idx.Resolve("Declared Title"); got.Kind != graph.KindUnresolved {
		t.Fatalf("declared title gained resolution authority: %+v", got)
	}
	page := render.New(idx, capturedBodies{},
		agreementDeclaredTitles{"Declared Title": {"Notes/holder.md"}}, everyFileHeld{})
	result := page.HTML("Notes/Reading.md", "", body, wording.En)
	wantElements := []agreementControlElement{{
		Tag:   "span",
		Class: "wikilink-broken wikilink-title-only",
		Title: "\"Declared Title\" is the title of \"Notes/holder.md\", and a title is not a name a link finds; an alias on that note makes this link work",
	}}
	if diff := cmp.Diff(wantElements, agreementControlElements(t, result.HTML)); diff != "" {
		t.Errorf("caught: P0 title-only-attribution actual DOM (-want +got):\n%s", diff)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Kind != render.DiagWikilinkTitleOnly ||
		result.Diagnostics[0].Target != "Declared Title" {
		t.Errorf("caught: P0 title-only-attribution diagnostics=%+v", result.Diagnostics)
	}
	wantHeld := []agreementCheckEvidence{{Rule: "link.title_not_alias", Path: probe, Target: "Declared Title"}}
	if diff := cmp.Diff(wantHeld, agreementAttributionFindings(t, root, probe, "")); diff != "" {
		t.Errorf("caught: P0 title-only-attribution public Check (-want +got):\n%s", diff)
	}
}

func agreementAmbiguityControl(t *testing.T) {
	const probe = "Notes/ambiguity-probe.md"
	const body = "[[A|reader words]]\n"
	root := agreementAttributionRoot(t)
	agreementWrite(t, root, probe, agreementEnvelope(t, body))
	wantMissing := []agreementCheckEvidence{{Rule: "link.broken", Path: probe, Target: "A"}}
	if diff := cmp.Diff(wantMissing, agreementAttributionFindings(t, root, probe, "a")); diff != "" {
		t.Fatalf("ambiguity probe missing-target receipt (-want +got):\n%s", diff)
	}
	members := []string{"Notes/one/A.md", "Notes/two/A.md"}
	for _, member := range members {
		agreementWrite(t, root, member, agreementEnvelope(t, "body\n"))
	}
	idx := graph.BuildFromNotes([]graph.NoteInput{{RelPath: members[0]}, {RelPath: members[1]}}, nil)
	resolution := idx.Resolve("A")
	wantResolution := graph.Resolution{Kind: graph.KindAmbiguous, Candidates: members}
	if diff := cmp.Diff(wantResolution, resolution); diff != "" {
		t.Fatalf("ambiguity precondition (-want +got):\n%s", diff)
	}
	page := render.New(idx, capturedBodies{}, noTitlesDeclared{}, everyFileHeld{})
	result := page.HTML("Notes/Reading.md", "", body, wording.En)
	wantElements := []agreementControlElement{{
		Tag:   "span",
		Class: "wikilink-ambiguous",
		Title: "\"A\" points at more than one file: Notes/one/A.md, Notes/two/A.md. yomihon does not guess which",
	}}
	if diff := cmp.Diff(wantElements, agreementControlElements(t, result.HTML)); diff != "" {
		t.Errorf("caught: P0 ambiguity-attribution actual DOM (-want +got):\n%s", diff)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Kind != render.DiagWikilinkAmbiguous ||
		result.Diagnostics[0].Target != "A" {
		t.Errorf("caught: P0 ambiguity-attribution diagnostics=%+v", result.Diagnostics)
	}
	wantCollision := []agreementCheckEvidence{{Rule: "collision.name", Path: members[0], Target: "a", Members: members}}
	if diff := cmp.Diff(wantCollision, agreementAttributionFindings(t, root, probe, "a")); diff != "" {
		t.Errorf("caught: P0 ambiguity-attribution public Check (-want +got):\n%s", diff)
	}
}

func agreementMediaControl(t *testing.T) {
	const probe = "Notes/media-probe.md"
	const body = "![[picture.png]]\n\n![[manual.pdf]]\n"
	root := agreementAttributionRoot(t)
	agreementWrite(t, root, probe, agreementEnvelope(t, body))
	wantMissing := []agreementCheckEvidence{
		{Rule: "link.broken", Path: probe, Target: "picture.png"},
		{Rule: "link.broken", Path: probe, Target: "manual.pdf"},
	}
	if diff := cmp.Diff(wantMissing, agreementAttributionFindings(t, root, probe, "")); diff != "" {
		t.Fatalf("media probe missing-resource receipt (-want +got):\n%s", diff)
	}
	resources := []string{"Notes/picture.png", "Notes/manual.pdf"}
	for _, resource := range resources {
		// Resource identity is its captured path/extension, not Markdown bytes.
		agreementWrite(t, root, resource, []byte("public synthetic resource\n"))
	}
	idx := graph.BuildFromNotes(nil, resources)
	page := render.New(idx, capturedBodies{}, noTitlesDeclared{}, everyFileHeld{})
	result := page.HTML("Notes/Reading.md", "", body, wording.En)
	wantElements := []agreementControlElement{
		{Tag: "img", Src: "/raw/Notes/picture.png", Alt: "picture.png"},
		{Tag: "div", Class: "embed-media"},
		{Tag: "a", Href: "/notes/Notes/manual.pdf"},
	}
	if diff := cmp.Diff(wantElements, agreementControlElements(t, result.HTML)); diff != "" {
		t.Errorf("caught: P0 media-attribution actual DOM (-want +got):\n%s", diff)
	}
	if len(result.Diagnostics) != 0 {
		t.Errorf("caught: P0 media-attribution resource gained note diagnostic: %+v", result.Diagnostics)
	}
	if got := agreementAttributionFindings(t, root, probe, ""); len(got) != 0 {
		t.Errorf("caught: P0 media-attribution public Check resource gained note finding: %+v", got)
	}
}

func agreementEnvelopeIdentity(t *testing.T) {
	t.Parallel()
	for _, body := range []string{"", "\ufeffbody\n", "---\nunterminated\n", "---\r\n---", "body\r\n", "\n---\n"} {
		t.Run(body, func(t *testing.T) {
			agreementEnvelope(t, body)
		})
	}
}

// These rows offer real renderer/LinkTargets/public-Check observations to the
// classifier. The expected eligibility is independently written per row.
func TestAgreementDifferenceControls(t *testing.T) {
	for _, tc := range []struct {
		name    string
		body    string
		allowed int
	}{
		{name: "one title owns one occurrence", body: "> [!note] [[A]]\n", allowed: 1},
		{name: "title and unrelated code delta", body: "> [!note] [[A]]\n> body\n\n`open\n[[B]]\nclose`", allowed: 1},
		{name: "repeated target title and body", body: "> [!note] [[A]]\n> [[A]]\n", allowed: 1},
		{name: "code is no title owner", body: "```\n> [!note] [[A]]\n```\n"},
		{name: "ambiguous duplicate title ownership", body: "> [!note] [[A]]\n> [!note] [[A]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := agreementCase{Name: tc.name, Body: tc.body}
			page := render.New(graph.BuildFromNotes(nil, nil), capturedBodies{}, noTitlesDeclared{}, everyFileHeld{})
			result := page.HTML("Notes/Reading.md", "", c.Body, wording.En)
			actual := agreementObserve(t, result.HTML)
			failures := agreementPageFailures(c.Body, &result, &actual)
			allowed := agreementDesignedDifferences(t, c, &actual, failures)
			if len(allowed) != tc.allowed {
				t.Errorf("caught: designed-occurrence-ownership allowed=%d want=%d failures=%+v", len(allowed), tc.allowed, failures)
			}
			for failureIndex := range failures {
				failure := &failures[failureIndex]
				if !allowed[agreementSignature(failure)] {
					continue
				}
				changes := []agreementFailure{
					{Property: "P0", Identity: failure.Identity, Tuple: failure.Tuple, Direction: failure.Direction, Multiplicity: failure.Multiplicity},
					{Property: failure.Property, Identity: failure.Identity, Tuple: failure.Tuple, Direction: "page-only", Multiplicity: failure.Multiplicity},
					{Property: failure.Property, Identity: failure.Identity, Tuple: failure.Tuple, Direction: failure.Direction, Multiplicity: failure.Multiplicity + 1},
					{Property: failure.Property, Identity: failure.Identity, Tuple: failure.Tuple, Fragment: "^different", Direction: failure.Direction, Multiplicity: failure.Multiplicity},
				}
				for changeIndex := range changes {
					changed := &changes[changeIndex]
					if got := agreementDesignedDifferences(t, c, &actual, []agreementFailure{*changed}); len(got) != 0 {
						t.Errorf("caught: designed-signature-drift accepted=%+v", *changed)
					}
				}
			}
		})
	}
	t.Run("retired inline footnote literal", func(t *testing.T) {
		body := "paragraph ^[literal]\n"
		page := render.New(graph.BuildFromNotes(nil, nil), capturedBodies{}, noTitlesDeclared{}, everyFileHeld{})
		result := page.HTML("Notes/Reading.md", "", body, wording.En)
		actual := agreementObserve(t, result.HTML)
		cut, found := render.Excerpt(body, "^[literal]")
		// Inline footnote spelling is not a block name. Refuse the raw cut
		// as well as the page anchor instead of inventing an address candidate.
		if len(actual.Blocks) != 0 || found || cut != "" {
			t.Errorf("caught: retired-inline-footnote-defect blocks=%q found=%t cut=%q", actual.Blocks, found, cut)
		}
		failures := agreementFragmentFailures(t, []agreementCase{{Name: "retired-inline-footnote", Body: body}}, []agreementHTML{actual})
		if len(failures[0]) != 0 {
			t.Errorf("caught: manufactured-raw-candidate-debt failures=%+v", failures[0])
		}
	})
	t.Run("setup is distinct from behavior", func(t *testing.T) {
		setup := agreementCapture(t, func(observer agreementTB) {
			observer.Fatalf("controlled observation setup refusal")
		})
		if setup != "controlled observation setup refusal" {
			t.Errorf("caught: setup-channel got=%q", setup)
		}
		c := agreementCounterexample{Case: agreementCase{Body: "[[A]]"}, Failure: agreementFailure{Property: "setup", Observation: setup}}
		body, candidates, checks, stop := agreementMinimize(t, &c)
		if body != c.Case.Body || candidates != 0 || checks != 0 || stop != "not-attempted setup-failure" {
			t.Errorf("caught: setup-minimization body=%q candidates=%d checks=%d stop=%q", body, candidates, checks, stop)
		}
	})
}

func TestAgreementWitnessControls(t *testing.T) {
	for _, tc := range []struct {
		name  string
		body  string
		known int
	}{
		{name: "frozen bare backslash", body: "[[Trail\\]]\n", known: 2},
		{name: "frozen list fence", body: "- a list item\n\n    ```\n    [[Nested]]\n    ```\n\n[[Outside List]]\n", known: 2},
		{name: "unused-footnote-2-0332", body: "[^unused]: [[A]]\n", known: 2},
		{name: "fence-info-2-0984", body: "``` [[A]]\n", known: 1},
		{name: "multiline-code-1-1357", body: "`open\n[[A]]\nclose`", known: 2},
		{name: "duplicate-heading-3-1181", body: "## A\n## A\n", known: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := agreementCase{Name: tc.name, Body: tc.body}
			result, actual := agreementIsolatedPage(t, c)
			failures := agreementPageFailures(c.Body, &result, &actual)
			fragments := agreementFragmentFailures(t, []agreementCase{c}, []agreementHTML{actual})
			failures = append(failures, fragments[0]...)
			known := 0
			for failureIndex := range failures {
				failure := &failures[failureIndex]
				kind, authority, wrong := agreementKnownDifference(c, failure)
				if kind == "" {
					continue
				}
				known++
				if kind != "debt" || (authority != "#1011 stage 3" && authority != "#1011 stage 4" && authority != "#1011 stage 5" && authority != "#1011 stage 8") || wrong == "" {
					t.Errorf("caught: debt-witness-provenance kind=%q authority=%q wrong=%q", kind, authority, wrong)
				}
				changed := *failure
				changed.Direction = "different-direction"
				if kind, _, _ := agreementKnownDifference(c, &changed); kind != "" {
					t.Errorf("caught: debt-wrong-direction accepted=%+v", changed)
				}
				changed = *failure
				changed.Multiplicity++
				if kind, _, _ := agreementKnownDifference(c, &changed); kind != "" {
					t.Errorf("caught: debt-occurrence-budget accepted=%+v", changed)
				}
				companion := c
				companion.Companions = capturedBodies{"Notes/Other.md": "other"}
				if kind, _, _ := agreementKnownDifference(companion, failure); kind != "" {
					t.Errorf("caught: debt-companion-drift accepted=%+v", *failure)
				}
				budget := agreementReplayBudget{Candidates: 128, Checks: 64}
				body, candidates, checks, stop := agreementMinimizeBudget(t, &agreementCounterexample{Case: c, Failure: *failure}, &budget)
				if body != c.Body || candidates != 0 || checks != 0 || stop != "not-attempted public-check-budget" {
					t.Errorf("caught: minimizer-budget body=%q candidates=%d checks=%d stop=%q", body, candidates, checks, stop)
				}
			}
			if known != tc.known {
				t.Errorf("caught: active-debt-witness-membership known=%d want=%d failures=%+v", known, tc.known, failures)
			}
		})
	}
}

func TestAgreementMinimizerControls(t *testing.T) {
	c := agreementCase{
		Name:       "reducer-selected-code-occurrence",
		Body:       "unrelated prefix\n\n`open\n[[A]]\nclose`\n\nunrelated suffix\n",
		Title:      "Preserved Title",
		Companions: capturedBodies{"Notes/Other.md": "## Protected companion\n"},
	}
	want := agreementFailure{
		Property: "P1", Identity: "citation-occurrences",
		Tuple: agreementCitation{Target: "A"}, Direction: "page-only", Multiplicity: 1,
	}
	result, actual := agreementIsolatedPage(t, c)
	failures := agreementPageFailures(c.Body, &result, &actual)
	selected := agreementFailure{}
	matches := 0
	for failureIndex := range failures {
		failure := &failures[failureIndex]
		if agreementSignature(failure) == agreementSignature(&want) {
			selected = *failure
			matches++
		}
	}
	if matches != 1 || actual.CitationsInCode != 1 {
		t.Fatalf("not-applied: minimizer actual selected-code boundary matches=%d code=%d failures=%+v", matches, actual.CitationsInCode, failures)
	}
	t.Run("retains selected producer difference", func(t *testing.T) {
		body, candidates, checks, stop := agreementMinimize(t, &agreementCounterexample{Case: c, Failure: selected})
		if body == c.Body || body == "" || candidates == 0 || checks <= 2 {
			t.Fatalf("caught: minimizer-no-preserved-reduction original=%q body=%q candidates=%d checks=%d stop=%q", c.Body, body, candidates, checks, stop)
		}
		reduced := c
		reduced.Body = body
		result, actual := agreementIsolatedPage(t, reduced)
		observed := agreementPageFailures(body, &result, &actual)
		fragments := agreementFragmentFailures(t, []agreementCase{reduced}, []agreementHTML{actual})
		observed = append(observed, fragments[0]...)
		matches := 0
		for failureIndex := range observed {
			failure := &observed[failureIndex]
			if agreementSignature(failure) == agreementSignature(&want) {
				matches++
			}
		}
		if matches != 1 || actual.CitationsInCode != 1 {
			t.Errorf("caught: minimizer-selected-signature-lost body=%q code=%d failures=%+v", body, actual.CitationsInCode, observed)
		}
		wantContext := agreementCase{
			Name: "reducer-selected-code-occurrence", Body: c.Body,
			Title: "Preserved Title", Companions: capturedBodies{"Notes/Other.md": "## Protected companion\n"},
		}
		if diff := cmp.Diff(wantContext, c); diff != "" {
			t.Errorf("caught: minimizer-original-context-mutated (-want +got):\n%s", diff)
		}
		if reduced.Title != "Preserved Title" || len(reduced.Companions) != 1 || reduced.Companions["Notes/Other.md"] != "## Protected companion\n" {
			t.Errorf("caught: minimizer-replay-context-drift reduced=%+v", reduced)
		}
	})
	t.Run("refuses original defect switch", func(t *testing.T) {
		switched := selected
		switched.Tuple.Target = "B"
		body, candidates, checks, stop := agreementMinimize(t, &agreementCounterexample{Case: c, Failure: switched})
		if body != c.Body || candidates != 0 || checks != 2 || stop != "not-attempted isolated-different-signature" {
			t.Errorf("caught: minimizer-original-defect-switch body=%q candidates=%d checks=%d stop=%q", body, candidates, checks, stop)
		}
	})
	t.Log("AGREEMENT-MINIMIZER-CONTROL preserved-target-and-context")
}

func TestAgreementMinimizerFragmentContext(t *testing.T) {
	t.Parallel()
	c := agreementCase{
		Name:       "reducer-fragment-with-owned-context",
		Body:       "# A\n\nunrelated prefix\n\n![[Notes/Child]]\n\nunrelated suffix\n",
		Title:      "A",
		Companions: capturedBodies{"Notes/Child.md": "## A\n"},
	}
	want := agreementFailure{
		Property: "P4", Identity: "literal-heading-id", Fragment: "a-2",
		Direction: "page-only", Multiplicity: 1, PagePresent: true,
	}
	observe := func(c agreementCase) []agreementFailure {
		result, actual := agreementIsolatedPage(t, c)
		failures := agreementPageFailures(c.Body, &result, &actual)
		fragments := agreementFragmentFailures(t, []agreementCase{c}, []agreementHTML{actual})
		return append(failures, fragments[0]...)
	}
	matches := func(failures []agreementFailure) int {
		count := 0
		for failureIndex := range failures {
			failure := &failures[failureIndex]
			if agreementSignature(failure) == agreementSignature(&want) {
				count++
			}
		}
		return count
	}
	original := observe(c)
	if matches(original) != 1 {
		t.Fatalf("not-applied: actual title/companion fragment context lacks selected a-2: %+v", original)
	}
	// Only a matching leading H1 lends its address to the page title. The
	// companion's A heading then has to move to a-2 in the assembled page.
	result, _ := agreementIsolatedPage(t, c)
	if result.TitleAnchor != "a" {
		t.Fatalf("not-applied: matching leading H1 did not give the title its address: %q", result.TitleAnchor)
	}
	withoutCompanion := c
	withoutCompanion.Companions = capturedBodies{}
	if matches(observe(withoutCompanion)) != 0 {
		t.Fatal("not-applied: selected fragment does not depend on the companion heading")
	}
	body, candidates, checks, stop := agreementMinimize(t, &agreementCounterexample{Case: c, Failure: want})
	if body == c.Body || body == "" || candidates == 0 || checks <= 2 {
		t.Fatalf("caught: minimizer-context-reduction original=%q reduced=%q candidates=%d checks=%d stop=%q", c.Body, body, candidates, checks, stop)
	}
	reduced := c
	reduced.Body = body
	if got := observe(reduced); matches(got) != 1 {
		t.Errorf("caught: minimizer-fragment-context-lost body=%q failures=%+v", body, got)
	}
	result, _ = agreementIsolatedPage(t, reduced)
	if result.TitleAnchor != "a" {
		t.Errorf("caught: minimizer-title-address-lost body=%q anchor=%q", body, result.TitleAnchor)
	}
	wantOriginal := agreementCase{
		Name:  "reducer-fragment-with-owned-context",
		Body:  "# A\n\nunrelated prefix\n\n![[Notes/Child]]\n\nunrelated suffix\n",
		Title: "A", Companions: capturedBodies{"Notes/Child.md": "## A\n"},
	}
	if diff := cmp.Diff(wantOriginal, c); diff != "" {
		t.Errorf("caught: minimizer-material-context-mutated (-want +got):\n%s", diff)
	}
}
