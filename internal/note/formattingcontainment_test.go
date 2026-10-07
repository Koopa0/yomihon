package note_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/koopa0/yomihon/internal/ui/pages"
)

// TestAnUnpairedFormattingTagStaysInsideItsNote reads the whole reading page
// the way a browser does. An underline is a formatting element: a parser
// reopens one left open in every later block it reaches, past the end of the
// note and into the page around it. Only a tag whose closer the author wrote in
// the same container may become markup, so the one paired underline is the only
// underline on the page and the unpaired openers are shown as the text they are.
// The unwritten link gives the reading rail after the note words of its own,
// which is where a reopened underline would land.
func TestAnUnpairedFormattingTagStaysInsideItsNote(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeVaultNote(t, root, "Notes/Formatting.md", "---\ntitle: Formatting\n---\n"+
		"Paired <u>kept</u> words, see [[Unwritten]].\n\n"+
		"para <u>open\n\n"+
		"Later prose.\n\n"+
		"self <u/>closing\n\n"+
		"- item <u>listopen\n- next item\n\n"+
		"<u>\nblock opener\n\n"+
		"Closing prose.\n")
	srv := newServer(t, root)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+pages.VaultHref("/notes/", "Notes/Formatting.md"), http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	response, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil {
		t.Fatalf("read/close response: %v / %v", readErr, closeErr)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.StatusCode, body)
	}
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}

	var underlines []string
	var walk func(node *html.Node, inProse bool)
	walk = func(node *html.Node, inProse bool) {
		element := node.Type == html.ElementNode
		inProse = inProse || element && node.Data == "div" && hasClass(node, "y-prose")
		if element && node.Data == "u" {
			where := "the note body"
			if !inProse {
				where = "outside the note body"
			}
			underlines = append(underlines, where+": "+textOf(node))
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child, inProse)
		}
	}
	walk(doc, false)
	if len(underlines) != 1 || underlines[0] != "the note body: kept" {
		t.Errorf("caught: authored underline escaped its container; underlines on the page = %q, want only the paired one", underlines)
	}
	for _, shown := range []string{"para &lt;u&gt;open", "self &lt;u/&gt;closing", "item &lt;u&gt;listopen", "&lt;u&gt;\nblock opener"} {
		if !strings.Contains(string(body), shown) {
			t.Errorf("caught: unpaired opener not shown as text: %q missing", shown)
		}
	}
}

func hasClass(node *html.Node, class string) bool {
	for _, attr := range node.Attr {
		if attr.Key == "class" && strings.Contains(" "+attr.Val+" ", " "+class+" ") {
			return true
		}
	}
	return false
}

func textOf(node *html.Node) string {
	var b strings.Builder
	var collect func(*html.Node)
	collect = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			collect(child)
		}
	}
	collect(node)
	return b.String()
}
