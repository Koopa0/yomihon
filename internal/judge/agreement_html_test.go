package judge_test

import (
	"bytes"
	"cmp"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/wording"
)

type agreementCitation struct {
	Target  string
	Section string
	State   string
}

type agreementHTML struct {
	Citations       []agreementCitation
	CitationsInCode int
	Blocks          []string
	Headings        []string
}

func agreementCitationCompare(a, b agreementCitation) int {
	if n := cmp.Compare(a.Target, b.Target); n != 0 {
		return n
	}
	if n := cmp.Compare(a.Section, b.Section); n != 0 {
		return n
	}
	return cmp.Compare(a.State, b.State)
}

func agreementAttr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

func agreementClass(n *html.Node, name string) bool {
	for _, token := range strings.Fields(agreementAttr(n, "class")) {
		if token == name {
			return true
		}
	}
	return false
}

func agreementCarrier(n *html.Node) bool {
	for _, class := range strings.Fields(agreementAttr(n, "class")) {
		if class == "wikilink" || strings.HasPrefix(class, "wikilink-") {
			return true
		}
	}
	return false
}

// The wikilink factories declare five shapes. A class/href on a span is not
// navigation; broken, title-only and ambiguous notices never become controls.
func agreementCarrierState(n *html.Node) (string, error) {
	classes := strings.Fields(agreementAttr(n, "class"))
	slices.Sort(classes)
	state := strings.Join(classes, " ")
	interactive := false
	switch state {
	case "wikilink", "wikilink wikilink-degraded":
		interactive = true
	case "wikilink-broken", "wikilink-broken wikilink-title-only", "wikilink-ambiguous":
	default:
		return "", fmt.Errorf("incoherent citation classes %q", state)
	}
	counts := make(map[string]int)
	for _, attr := range n.Attr {
		counts[attr.Key]++
		allowed := attr.Key == "class" || attr.Key == "title"
		if interactive {
			allowed = allowed || attr.Key == "href" || attr.Key == "data-preview-section"
		}
		if !allowed || counts[attr.Key] != 1 {
			return "", fmt.Errorf("unexpected or duplicate citation attribute %q", attr.Key)
		}
	}
	if counts["class"] != 1 {
		return "", fmt.Errorf("citation has no single class attribute")
	}
	if !interactive {
		if n.Data != "span" || counts["title"] != 1 || agreementAttr(n, "title") == "" {
			return "", fmt.Errorf("notice is not a noninteractive explained span")
		}
		return state, nil
	}
	if n.Data != "a" || counts["href"] != 1 {
		return "", fmt.Errorf("resolved citation is not an anchor with one href")
	}
	href := agreementAttr(n, "href")
	parsed, err := url.Parse(href)
	if err != nil {
		return "", fmt.Errorf("parse first-party citation href: %w", err)
	}
	local := strings.HasPrefix(href, "#") && parsed.Path == "" && parsed.Fragment != ""
	note := strings.HasPrefix(parsed.Path, "/notes/") && len(parsed.Path) > len("/notes/")
	if parsed.Scheme != "" || parsed.Host != "" || parsed.User != nil || parsed.RawQuery != "" || (!local && !note) {
		return "", fmt.Errorf("citation href %q is not a first-party note or local fragment", href)
	}
	if state == "wikilink wikilink-degraded" && (counts["title"] != 1 || agreementAttr(n, "title") == "") {
		return "", fmt.Errorf("degraded citation has no explanation")
	}
	if state == "wikilink" && counts["title"] != 0 {
		return "", fmt.Errorf("ordinary citation has a notice title")
	}
	if state == "wikilink wikilink-degraded" && counts["data-preview-section"] != 0 {
		return "", fmt.Errorf("degraded citation has ordinary-link preview state")
	}
	return state, nil
}

// Targets come from actual notice attributes, never Diagnostic.Target. The
// unresolved corpus deliberately prevents aliases from hiding target identity.
func agreementNotice(reason string) (agreementCitation, error) {
	if target, ok := strings.CutSuffix(reason, "\" leaves the vault; the link text remains"); ok {
		target, quoted := strings.CutPrefix(target, "\"")
		if !quoted || target == "" {
			return agreementCitation{}, fmt.Errorf("invalid outside-vault notice %q", reason)
		}
		return agreementCitation{Target: target, State: "wikilink-broken"}, nil
	}
	var rest string
	for _, prefix := range []string{"There is no note called ", "There is no file called "} {
		if strings.HasPrefix(reason, prefix) {
			rest = strings.TrimPrefix(reason, prefix)
			break
		}
	}
	if rest == "" {
		return agreementCitation{}, fmt.Errorf("unknown notice %q", reason)
	}
	target, rest, err := agreementQuoted(rest)
	if err != nil {
		return agreementCitation{}, err
	}
	if !strings.HasPrefix(rest, " yet") {
		return agreementCitation{}, fmt.Errorf("missing notice suffix %q", rest)
	}
	rest = strings.TrimPrefix(rest, " yet")
	section := ""
	if rest != "" {
		prefix := "; what follows \"#\" was read as the section "
		if !strings.HasPrefix(rest, prefix) {
			return agreementCitation{}, fmt.Errorf("unknown fragment notice %q", rest)
		}
		section, rest, err = agreementQuoted(strings.TrimPrefix(rest, prefix))
		if err != nil {
			return agreementCitation{}, err
		}
		if rest != "" {
			return agreementCitation{}, fmt.Errorf("trailing fragment notice %q", rest)
		}
	}
	return agreementCitation{Target: target, Section: section, State: "wikilink-broken"}, nil
}

func agreementQuoted(text string) (string, string, error) {
	if !strings.HasPrefix(text, "\"") {
		return "", "", fmt.Errorf("expected quoted target %q", text)
	}
	for end := 1; end < len(text); end++ {
		if text[end] == '\\' {
			end++
			continue
		}
		if text[end] != '"' {
			continue
		}
		value, err := strconv.Unquote(text[:end+1])
		if err != nil {
			return "", "", fmt.Errorf("decoding notice target: %w", err)
		}
		return value, text[end+1:], nil
	}
	return "", "", fmt.Errorf("unterminated quoted target %q", text)
}

func agreementObserve(t *testing.T, text string) agreementHTML {
	t.Helper()
	return agreementObserveKnown(t, text, nil)
}

func agreementObserveKnown(t *testing.T, text string, known map[string]string) agreementHTML {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(text))
	if err != nil {
		t.Fatalf("parse actual HTML: %v", err)
	}
	var result agreementHTML
	var walk func(*html.Node, bool)
	walk = func(n *html.Node, code bool) {
		code = code || n.Type == html.ElementNode && n.Data == "code"
		if n.Type == html.ElementNode {
			id := agreementAttr(n, "id")
			if n.Data == "span" && strings.HasPrefix(id, "^") {
				result.Blocks = append(result.Blocks, id)
			}
			if len(n.Data) == 2 && n.Data[0] == 'h' && n.Data[1] >= '1' && n.Data[1] <= '6' && id != "" {
				result.Headings = append(result.Headings, id)
			}
			if agreementCarrier(n) {
				if code {
					result.CitationsInCode++
				}
				state, shapeErr := agreementCarrierState(n)
				if shapeErr != nil {
					t.Errorf("caught: P0 citation-shape tag=%q class=%q href=%q error=%v html=%q", n.Data, agreementAttr(n, "class"), agreementAttr(n, "href"), shapeErr, text)
					goto children
				}
				if agreementClass(n, "wikilink") {
					// The only resolved destination in this corpus is the host's
					// own fragment. It cites no other note, but still counts for P2.
					href := agreementAttr(n, "href")
					if strings.HasPrefix(href, "#") || strings.HasPrefix(href, "/notes/Notes/Reading.md#") || href == "/notes/Notes/Reading.md" {
						goto children
					}
					parsed, parseErr := url.Parse(href)
					if parseErr != nil {
						t.Fatalf("parse resolved citation URL: %v", parseErr)
					}
					target, held := known[strings.TrimPrefix(parsed.Path, "/notes/")]
					if !held {
						t.Fatalf("unexpected resolved citation carrier: href=%q html=%q", href, text)
					}
					result.Citations = append(result.Citations, agreementCitation{Target: target, Section: parsed.Fragment, State: state})
					goto children
				}
				citation, parseErr := agreementNotice(agreementAttr(n, "title"))
				if parseErr != nil {
					t.Fatalf("observe citation: %v; html=%q", parseErr, text)
				}
				result.Citations = append(result.Citations, citation)
			}
		}
	children:
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child, code)
		}
	}
	walk(doc, false)
	return result
}

func agreementTitleHTML(t *testing.T, c agreementCase, result render.Result) agreementHTML {
	t.Helper()
	var buf bytes.Buffer
	view := pages.NoteView{Title: c.Title, RelPath: "Notes/Reading.md", BodyHTML: result.HTML, TitleAnchor: result.TitleAnchor, TOC: result.TOC}
	if err := pages.Note(view, layouts.Chrome{Lang: wording.En}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render actual Note title: %v", err)
	}
	// Restrict the observation to the article; shell headings are not note ids.
	doc, err := html.Parse(&buf)
	if err != nil {
		t.Fatalf("parse Note: %v", err)
	}
	var article *html.Node
	var find func(*html.Node)
	find = func(n *html.Node) {
		if agreementClass(n, "y-article") {
			if article != nil {
				t.Fatal("multiple note articles")
			}
			article = n
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			find(child)
		}
	}
	find(doc)
	if article == nil {
		t.Fatal("actual Note has no y-article")
	}
	buf.Reset()
	if err := html.Render(&buf, article); err != nil {
		t.Fatalf("serialize Note article: %v", err)
	}
	return agreementObserve(t, buf.String())
}
