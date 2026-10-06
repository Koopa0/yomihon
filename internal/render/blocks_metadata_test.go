package render_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestBlocksListExactlyTheEmittedAnchors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		body      string
		want      []string
		qualified []string
	}{
		{name: "paragraph list quote and standalone", body: destBody, want: []string{"^quux", "^itm", "^qq", "^under"}, qualified: []string{"right-^quux", "right-^itm", "right-^qq", "right-^under"}},
		{name: "canonical spelling and duplicate", body: "First ^CAFE\u0301\n\nDuplicate ^CAFÉ\n\nEscaped ^Q&\"\n", want: []string{"^café", "^q&\""}, qualified: []string{"right-^café", "right-^q&\""}},
		{name: "callout body", body: "> [!note] Title ^title\n> Body ^inside\n\nAfter ^after\n", want: []string{"^inside", "^after"}, qualified: []string{"right-^inside", "right-^after"}},
		{name: "fenced inline and commented syntax", body: "```\nShown ^fenced\n```\n\n`shown ^inline`\n\n%% Hidden ^comment %%\n", want: nil},
		{name: "authored markup and tables", body: "<span id=\"^authored\">^authored</span>\n\n| A | B |\n|---|---|\n| a | b | ^table\n", want: nil},
		{name: "unused footnote is not emitted", body: "[^unused]: Hidden ^unused\n\nVisible ^visible\n", want: []string{"^visible"}, qualified: []string{"right-^visible"}},
		{name: "used footnote follows body in document order", body: "[^used]: Footnote ^foot\n\nReference[^used] ^body\n", want: []string{"^body", "^foot"}, qualified: []string{"right-^body", "right-^foot"}},
		{name: "transcluded addresses stay with their source", body: "![[Child]]\n\nOwn ^own\n", want: []string{"^own"}, qualified: []string{"right-^own"}},
		{name: "title and outline stay separate", body: "# Source\n\n## Heading\n\nBody ^body\n", want: []string{"^body"}, qualified: []string{"right-^body"}},
		{name: "a hidden first claim does not invent a later id", body: "[^unused]: Hidden ^same\n\nDuplicate ^same\n", want: nil},
		{name: "no anchors", body: "Plain words.\n", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRenderer(t, []graph.NoteInput{{RelPath: "Child.md"}}, nil, transclusions{"Child.md": "Child ^child\n"})
			for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
				got := r.HTML("Source.md", "Source", tt.body, lang)
				if diff := cmp.Diff(tt.want, got.Blocks); diff != "" {
					t.Errorf("Blocks mismatch (-want +got):\n%s", diff)
				}
				if diff := cmp.Diff(tt.want, emittedBlockIDs(t, got.HTML)); diff != "" {
					t.Errorf("emitted span ids mismatch (-want +got):\n%s", diff)
				}
				render.Qualify("right-", &got)
				if diff := cmp.Diff(tt.qualified, got.Blocks); diff != "" {
					t.Errorf("qualified Blocks mismatch (-want +got):\n%s", diff)
				}
				if diff := cmp.Diff(tt.qualified, emittedBlockIDs(t, got.HTML)); diff != "" {
					t.Errorf("qualified span ids mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

// emittedBlockIDs reads actual span attributes through the HTML tokenizer,
// independent of the renderer's private placeholders and metadata table.
func emittedBlockIDs(t *testing.T, body string) []string {
	t.Helper()
	z := html.NewTokenizer(strings.NewReader(body))
	var ids []string
	for {
		switch z.Next() {
		case html.ErrorToken:
			if err := z.Err(); !errors.Is(err, io.EOF) {
				t.Fatalf("tokenize HTML: %v", err)
			}
			return ids
		case html.TextToken, html.EndTagToken, html.SelfClosingTagToken, html.CommentToken, html.DoctypeToken:
			// These tokens declare no opening span attributes.
		case html.StartTagToken:
			token := z.Token()
			if token.Data != "span" {
				continue
			}
			for _, attr := range token.Attr {
				if attr.Key == "id" {
					ids = append(ids, attr.Val)
				}
			}
		}
	}
}

func TestQualifyCopiesBlockMetadata(t *testing.T) {
	t.Parallel()
	original := []string{"^one", "^q&\""}
	got := render.Result{Blocks: original}
	render.Qualify("right-", &got)
	if diff := cmp.Diff([]string{"right-^one", "right-^q&\""}, got.Blocks); diff != "" {
		t.Errorf("qualified metadata mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"^one", "^q&\""}, original); diff != "" {
		t.Errorf("Qualify renamed an aliased list (-want +got):\n%s", diff)
	}
	unchanged := render.Result{Blocks: original}
	render.Qualify("", &unchanged)
	if diff := cmp.Diff([]string{"^one", "^q&\""}, unchanged.Blocks); diff != "" {
		t.Errorf("empty prefix changed metadata (-want +got):\n%s", diff)
	}
	render.Qualify("right-", nil)
}
