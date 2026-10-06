package note_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html"
)

type comparedHeading struct {
	Tag   string
	Text  string
	ID    string
	Level string
}

func TestCompareHasOnePageHeadingAboveBothNoteOutlines(t *testing.T) {
	t.Parallel()
	body := "Opening prose.\n\n# Alpha\n\n## Beta\n\n### Gamma\n\n#### Delta\n\n##### Epsilon\n\n###### Zeta\n\nSetext\n------\n\n> ## Quoted\n\n- ## Listed\n\n`<h2>literal</h2>`\n"
	fixtures := map[string]string{
		"Writing/Original.md":    "---\ntitle: Original\ntype: lesson\nstatus: draft\nlang: en\n---\n\n# Original\n\n" + body,
		"Writing/Translation.md": "---\ntitle: Translation\ntype: lesson\nstatus: draft\nlang: zh-Hant\n---\n\n# Translation\n\n" + body,
	}
	srv := newServerWithContract(t, writeNotes(t, fixtures), loadHomeContract(t))
	for _, tt := range []struct {
		lang  string
		title string
	}{
		{lang: "zh-Hant", title: "Original 與 Translation 對照閱讀"},
		{lang: "en", title: "Original and Translation side by side"},
	} {
		t.Run(tt.lang, func(t *testing.T) {
			t.Parallel()
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/compare/Writing/Original.md?with=Writing%2FTranslation.md", http.NoBody)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Cookie", "yomihon_lang="+tt.lang)
			resp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			doc, parseErr := html.Parse(resp.Body)
			if closeErr := resp.Body.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
			if resp.StatusCode != http.StatusOK || parseErr != nil {
				t.Fatalf("compare response = %d, parse error = %v", resp.StatusCode, parseErr)
			}
			wantPage := []comparedHeading{{Tag: "h1", Text: tt.title}}
			if diff := cmp.Diff(wantPage, compareHeadings(doc, "h1")); diff != "" {
				t.Errorf("caught: page heading (-want +got):\n%s", diff)
			}
			for _, column := range []struct {
				id, prefix, title, titleID string
			}{
				{id: "compare-a", prefix: "a-", title: "Original", titleID: "a-original"},
				{id: "compare-b", prefix: "b-", title: "Translation", titleID: "b-translation"},
			} {
				section := compareElementByID(doc, column.id)
				if section == nil {
					t.Fatalf("missing column %s", column.id)
				}
				want := []comparedHeading{
					{Tag: "h2", Text: column.title, ID: column.titleID},
					{Tag: "h3", Text: "Alpha", ID: column.prefix + "alpha", Level: "1"},
					{Tag: "h4", Text: "Beta", ID: column.prefix + "beta", Level: "2"},
					{Tag: "h5", Text: "Gamma", ID: column.prefix + "gamma", Level: "3"},
					{Tag: "h6", Text: "Delta", ID: column.prefix + "delta", Level: "4"},
					{Tag: "h6", Text: "Epsilon", ID: column.prefix + "epsilon", Level: "5"},
					{Tag: "h6", Text: "Zeta", ID: column.prefix + "zeta", Level: "6"},
					{Tag: "h4", Text: "Setext", ID: column.prefix + "setext", Level: "2"},
					{Tag: "h4", Text: "Quoted", ID: column.prefix + "quoted", Level: "2"},
					{Tag: "h4", Text: "Listed", ID: column.prefix + "listed", Level: "2"},
				}
				if diff := cmp.Diff(want, compareHeadings(section, "")); diff != "" {
					t.Errorf("caught: %s outline (-want +got):\n%s", column.id, diff)
				}
			}
		})
	}
}

func compareHeadings(root *html.Node, tag string) []comparedHeading {
	var found []comparedHeading
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && len(n.Data) == 2 && n.Data[0] == 'h' && n.Data[1] >= '1' && n.Data[1] <= '6' && (tag == "" || n.Data == tag) {
			h := comparedHeading{Tag: n.Data, Text: compareHeadingText(n)}
			for _, attr := range n.Attr {
				switch attr.Key {
				case "id":
					h.ID = attr.Val
				case "data-level":
					h.Level = attr.Val
				}
			}
			found = append(found, h)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return found
}

func compareHeadingText(n *html.Node) string {
	var text strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			text.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return text.String()
}

func compareElementByID(n *html.Node, id string) *html.Node {
	for _, attr := range n.Attr {
		if attr.Key == "id" && attr.Val == id {
			return n
		}
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if found := compareElementByID(child, id); found != nil {
			return found
		}
	}
	return nil
}
