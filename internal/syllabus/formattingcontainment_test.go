package syllabus_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html"
)

// TestAnUnderlineAroundAnEmbedStaysInsideItsParagraph reads the whole listening
// page the way a browser does. An excerpt written mid-sentence parts its
// paragraph in two, and the listening page keeps only the marked part before
// it, so an underline that opens before the excerpt and closes after it has no
// closer on this page at all. A parser reopens an underline left open in every
// later block it reaches: the next paragraph, the next lesson's title, the
// chrome after the course. The underline the author closed beside its words is
// the only one the page may carry.
func TestAnUnderlineAroundAnEmbedStaysInsideItsParagraph(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write := func(rel, body string) {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	lesson := func(title string, paragraphs ...string) string {
		b := strings.Builder{}
		b.WriteString("---\ntitle: " + title + "\ntype: lesson\ndomain: golang\nstatus: ready\n" +
			"created: 2026-06-01\nupdated: 2026-06-01\n---\n")
		for _, paragraph := range paragraphs {
			b.WriteString("\n<!-- read-aloud: ja -->\n" + paragraph + "\n")
		}
		return b.String()
	}
	write("Writing/lessons/golang/First.md", lesson("First", "<u>いち ![[Concept]] end</u>", "に<u>さん</u>。"))
	write("Writing/lessons/golang/Second.md", lesson("Second", "し。"))
	write("Writing/lessons/golang/Concept.md", "---\ntitle: Concept\n---\nFrom Other words.\n")
	write("Maps/Path.md", "---\ntitle: Go path\ntype: study-path\ndomain: golang\nstatus: evergreen\n"+
		"created: 2026-06-01\nupdated: 2026-06-01\n---\n\n"+
		"## line | Line | 線 {sequence=primary}\n\n- [[First]]\n- [[Second]]\n")

	srv := listenServer(t, root, lessonTypes("lesson"))
	code, page := get(t, srv.Client(), srv.URL+"/listen/Maps/Path.md")
	if code != http.StatusOK {
		t.Fatalf("GET the listening page status = %d, want 200", code)
	}
	if !strings.Contains(page, "data-tts=\"し。\"") {
		t.Fatalf("the second lesson's paragraph is not on the page, so nothing after the excerpt was read: %s", listenColumn(t, page))
	}
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		t.Fatal(err)
	}
	var underlines []string
	var walk func(node *html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "u" {
			underlines = append(underlines, textOf(node))
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if diff := cmp.Diff([]string{"さん"}, underlines); diff != "" {
		t.Errorf("caught: an underline holds words outside the paragraph its author closed it in (-want +got):\n%s", diff)
	}
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
