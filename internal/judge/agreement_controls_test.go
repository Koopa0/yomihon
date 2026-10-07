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
		{name: "inline footnote is no address", body: "paragraph ^[literal]\n", fragment: "^[literal]"},
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
		{name: "outside path", reason: `"../../../etc/passwd.md" leaves the vault; the link text remains`, want: agreementCitation{Target: "../../../etc/passwd.md", State: "wikilink-broken"}},
		{name: "outside raw quotation", reason: `"../a"b\c.md" leaves the vault; the link text remains`, want: agreementCitation{Target: `../a"b\c.md`, State: "wikilink-broken"}},
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
	for _, tc := range []struct {
		name string
		body string
		want agreementControlElement
	}{
		{name: "resolved", body: "[[A]]", want: agreementControlElement{Tag: "a", Class: "wikilink", Href: "/notes/Notes/A.md"}},
		{name: "local", body: "## Present\n[[#Present]]", want: agreementControlElement{Tag: "a", Class: "wikilink", Href: "#present"}},
		{name: "degraded", body: "[[A#Missing]]", want: agreementControlElement{Tag: "a", Class: "wikilink wikilink-degraded", Href: "/notes/Notes/A.md#missing", Title: "No section called \"Missing\" was found; the link lands at the top of the note"}},
		{name: "broken", body: "[[Absent]]", want: agreementControlElement{Tag: "span", Class: "wikilink-broken", Title: "There is no note called \"Absent\" yet"}},
	} {
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
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && agreementClass(n, "embed") {
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

func agreementAttributionRoot(t *testing.T) string {
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
	for _, finding := range findings {
		if finding.RuleID == "scan.unreadable" || finding.RuleID == "scan.skipped" {
			t.Fatalf("attribution Check incomplete: %+v", finding)
		}
		isProbe := finding.Path == probe && strings.HasPrefix(string(finding.RuleID), "link.")
		isCollision := collision != "" && finding.RuleID == "collision.name" &&
			finding.Target != nil && *finding.Target == collision
		if !isProbe && !isCollision {
			continue
		}
		if finding.Target == nil {
			t.Fatalf("attribution finding has no target: %+v", finding)
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
