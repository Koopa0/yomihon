package preference_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html"

	"github.com/koopa0/yomihon/internal/wording"
)

// attribute reads one attribute off a parsed element, and says whether the
// element carries it at all: an empty value and no attribute are different
// things to a screen reader following a reference.
func attribute(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

func textOf(n *html.Node) string {
	var b strings.Builder
	for d := range n.Descendants() {
		if d.Type == html.TextNode {
			b.WriteString(d.Data)
		}
	}
	return strings.TrimSpace(b.String())
}

func parsePage(t *testing.T, body string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatalf("parse the preferences page: %v", err)
	}
	return doc
}

// The row for the buttons beside each section is worded only by what the owner
// approved: a heading and two options, and no sentence of its own. The words
// are written out here as literals, so a row that quietly grew a sentence or
// changed an option's label fails against the approved text and not against
// itself.
func TestTheSectionButtonsRowSaysOnlyWhatWasApproved(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		lang    wording.Lang
		legend  string
		options []string
	}{
		{"chinese", wording.ZhHant, "小節旁的按鈕", []string{"一直顯示", "滑鼠停留時顯示"}},
		{"english", wording.En, "Buttons beside sections", []string{"Always show", "Show on hover"}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := newServer(t)
			status, body := page(t, srv, "/preferences", "yomihon_lang="+string(tt.lang))
			if status != http.StatusOK {
				t.Fatalf("GET /preferences status = %d, want %d", status, http.StatusOK)
			}
			var row *html.Node
			for n := range parsePage(t, body).Descendants() {
				if field, ok := attribute(n, "data-pref-field"); ok && field == "sectionbuttons" {
					row = n
				}
			}
			if row == nil {
				t.Fatal("the page carries no row for the buttons beside sections")
			}
			var legend string
			var options []string
			notes, described := 0, 0
			for n := range row.Descendants() {
				if n.Type != html.ElementNode {
					continue
				}
				switch n.Data {
				case "legend":
					legend = textOf(n)
				case "label":
					options = append(options, textOf(n))
				case "p":
					if class, _ := attribute(n, "class"); strings.Contains(class, "y-preffield__note") {
						notes++
					}
				case "input":
					if _, ok := attribute(n, "aria-describedby"); ok {
						described++
					}
				}
			}
			if legend != tt.legend {
				t.Errorf("the row is headed %q, want the approved %q", legend, tt.legend)
			}
			if diff := cmp.Diff(tt.options, options); diff != "" {
				t.Errorf("the row's options (-want +got):\n%s", diff)
			}
			if notes != 0 {
				t.Errorf("the row carries %d sentences of its own, want none: no sentence for it was approved", notes)
			}
			if described != 0 {
				t.Errorf("%d of the row's options point at a description, and the row draws none", described)
			}
		})
	}
}

// A description an option points at has to be on the page. A row that draws no
// sentence must also point at none: a reference to nothing is read out as
// nothing by some screen readers and as an error by others, and it is the kind
// of fault no row-by-row check notices.
func TestEveryDescriptionAnOptionPointsAtIsOnThePage(t *testing.T) {
	t.Parallel()
	srv := newServer(t)
	status, body := page(t, srv, "/preferences")
	if status != http.StatusOK {
		t.Fatalf("GET /preferences status = %d, want %d", status, http.StatusOK)
	}
	doc := parsePage(t, body)
	ids := map[string]bool{}
	for n := range doc.Descendants() {
		if id, ok := attribute(n, "id"); ok {
			ids[id] = true
		}
	}
	pointed := 0
	for n := range doc.Descendants() {
		if n.Type != html.ElementNode {
			continue
		}
		target, ok := attribute(n, "aria-describedby")
		if !ok {
			continue
		}
		pointed++
		for id := range strings.FieldsSeq(target) {
			if !ids[id] {
				t.Errorf("a %s points at the description %q, which the page does not carry", n.Data, id)
			}
		}
	}
	if pointed == 0 {
		t.Fatal("nothing on the page points at a description, so every reference above passes for the wrong reason")
	}
	if !ids["pref-note-textsize"] {
		t.Error("the text size row lost its sentence, so this check is no longer looking at the rows that have one")
	}
}
