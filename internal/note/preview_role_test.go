package note_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html"

	"github.com/koopa0/yomihon/internal/wording"
)

// A card reads the cut's original block roles. Removing a private prefix must
// not turn its surviving words into a new fence or an indented code block.
func TestPreviewRetainsCommentBlockRoles(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body, text string
		roles            []string
	}{
		{name: "hidden continuation", body: "para\n%%hidden%%\n    [[Missing]]\n", text: "para Missing (There is no note called \"Missing\" yet)", roles: []string{"p", "span.wikilink-broken", "span.y-offscreen"}},
		{name: "hidden fence prefix", body: "%%hidden%%````\n[[Missing]]\n", text: "```` Missing (There is no note called \"Missing\" yet)", roles: []string{"p", "span.wikilink-broken", "span.y-offscreen"}},
		{name: "raw HTML suffix", body: "<!-- hidden -->    [[Missing]]\n", text: "Missing (There is no note called \"Missing\" yet)", roles: []string{"span.wikilink-broken", "span.y-offscreen"}},
		{name: "visible continuation control", body: "para\nvisible\n    [[Missing]]\n", text: "para visible Missing (There is no note called \"Missing\" yet)", roles: []string{"p", "span.wikilink-broken", "span.y-offscreen"}},
		{name: "real code control", body: "para\n\n    [[Missing]]\n", text: "para [[Missing]]", roles: []string{"p", "pre", "code"}},
	} {
		root := t.TempDir()
		writeVaultNote(t, root, "Notes/role.md", "---\ntitle: Role\n---\n"+tc.body)
		srv := newServer(t, root)
		card := askPreview(t, srv.Client(), srv.URL, "Notes/role.md", "", wording.En)
		if card.code != http.StatusOK {
			t.Fatalf("not-applied: preview status=%d body=%q", card.code, card.body)
		}
		roles, text := previewRoleShape(t, card.body)
		want := struct {
			Roles []string
			Text  string
		}{tc.roles, tc.text}
		got := struct {
			Roles []string
			Text  string
		}{roles, text}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("caught: S5 preview-role %s (-want +got):\n%s\nHTML=%q", tc.name, diff, card.body)
		}
	}
	t.Log("AGREEMENT-INVOKED S5/stage5-preview-role")
}

func previewRoleShape(t *testing.T, source string) (roles []string, words string) {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatalf("not-applied: parse card: %v", err)
	}
	var prose *html.Node
	var find func(*html.Node)
	find = func(n *html.Node) {
		for _, attr := range n.Attr {
			if n.Type == html.ElementNode && attr.Key == "class" && attr.Val == "y-prose" {
				if prose != nil {
					t.Fatal("not-applied: card contains multiple prose bodies")
				}
				prose = n
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}
	find(doc)
	if prose == nil {
		t.Fatalf("not-applied: card contains no prose body: %q", source)
	}
	var text strings.Builder
	var read func(*html.Node)
	read = func(n *html.Node) {
		if n.Type == html.ElementNode {
			var role strings.Builder
			role.WriteString(n.Data)
			for _, attr := range n.Attr {
				if attr.Key == "class" {
					role.WriteByte('.')
					role.WriteString(attr.Val)
				}
			}
			roles = append(roles, role.String())
		}
		if n.Type == html.TextNode {
			text.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			read(c)
		}
	}
	for c := prose.FirstChild; c != nil; c = c.NextSibling {
		read(c)
	}
	return roles, strings.Join(strings.Fields(text.String()), " ")
}
