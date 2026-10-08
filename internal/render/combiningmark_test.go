package render_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestCombiningMarkLinksNameTheSecondActualHeading(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, first, second string }{
		{"Hindi vowel", "कि", "की"},
		{"Thai tone", "ปู่", "ปู"},
		{"variation selector", "葛城", "葛\U000e0100城"},
		{"Kana mark", "か行", "か\u309a行"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body := "## " + tt.first + "\n\nFirst passage.\n\n## " + tt.second + "\n\nSecond passage.\n"
			r := newRenderer(t, []graph.NoteInput{{RelPath: "Target.md"}}, nil, transclusions{"Target.md": body})
			destination := r.HTML("Target.md", "", body, wording.En)
			want := []string{tt.first, tt.second}
			if diff := cmp.Diff(want, allMatches(elementID, destination.HTML)); diff != "" {
				t.Errorf("HTML(mark headings) ids mismatch (-want +got):\n%s", diff)
			}
			source := r.HTML("Source.md", "", "[[Target#"+tt.second+"|second]]", wording.En)
			if !strings.Contains(source.HTML, `href="/notes/Target.md#`+tt.second+`"`) || len(source.Diagnostics) != 0 {
				t.Errorf("HTML(second mark link) = %+v, want the second actual heading id without diagnostics", source)
			}
		})
	}
}

func TestFragmentNormalizationRenderedPlaces(t *testing.T) {
	t.Parallel()
	for _, authored := range []string{"Y\u030a", "y\u030a", "\u1e99"} {
		for _, requested := range []string{"Y\u030a", "y\u030a", "\u1e99"} {
			t.Run(authored+"/"+requested, func(t *testing.T) {
				t.Parallel()
				body := "[[#" + requested + "|local]]\n\n## " + authored + "\n\nPASSAGE\n"
				r := newRenderer(t, []graph.NoteInput{{RelPath: "Target.md"}}, nil, transclusions{"Target.md": body})
				got := r.HTML("Target.md", "", body, wording.En)
				for _, want := range []string{" id=\"\u1e99\"", "href=\"#\u1e99\""} {
					if !strings.Contains(got.HTML, want) {
						t.Errorf("caught: fragment-normalization rendered places missing %q: %s", want, got.HTML)
					}
				}
				wantTOC := []render.TOCEntry{{Level: 2, Text: authored, ID: "\u1e99"}}
				if diff := cmp.Diff(wantTOC, got.TOC); diff != "" {
					t.Errorf("caught: fragment-normalization TOC (-want +got):\n%s", diff)
				}
				link := r.HTML("Source.md", "", "[[Target#"+requested+"|cross]]", wording.En)
				if !strings.Contains(link.HTML, "href=\"/notes/Target.md#\u1e99\"") || len(link.Diagnostics) != 0 || len(got.Diagnostics) != 0 {
					t.Errorf("caught: fragment-normalization heading link = %+v; destination = %+v", link, got)
				}
			})
		}
	}
	t.Run("title reservation and qualification", func(t *testing.T) {
		t.Parallel()
		body := "# Y\u030a\n\n[[#\u1e99|title]]\n\n## y\u030a\n\nQuote. ^QUOTE-1\n\n[[Target#^quote-1|block]]\n"
		r := newRenderer(t, []graph.NoteInput{{RelPath: "Target.md"}}, nil, transclusions{"Target.md": body})
		got := r.HTML("Target.md", "Y\u030a", body, wording.En)
		if got.TitleAnchor != "\u1e99" {
			t.Errorf("caught: fragment-normalization title = %q, want %q", got.TitleAnchor, "\u1e99")
		}
		render.Qualify("right-", &got)
		if got.TitleAnchor != "right-\u1e99" {
			t.Errorf("caught: fragment-normalization qualified title = %q", got.TitleAnchor)
		}
		if diff := cmp.Diff([]render.TOCEntry{{Level: 2, Text: "y\u030a", ID: "right-\u1e99-2"}}, got.TOC); diff != "" {
			t.Errorf("caught: fragment-normalization qualified TOC (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff([]string{"right-^quote-1"}, got.Blocks); diff != "" {
			t.Errorf("caught: fragment-normalization qualified blocks (-want +got):\n%s", diff)
		}
		for _, want := range []string{" id=\"right-\u1e99-2\"", "href=\"#right-\u1e99\"", `id="right-^quote-1"`, `href="/notes/Target.md#%5Equote-1"`} {
			if !strings.Contains(got.HTML, want) {
				t.Errorf("caught: fragment-normalization qualified HTML missing %q: %s", want, got.HTML)
			}
		}
		if len(got.Diagnostics) != 0 {
			t.Errorf("caught: fragment-normalization qualified diagnostics = %+v", got.Diagnostics)
		}
	})
	t.Run("duplicates and occupied path suffix", func(t *testing.T) {
		t.Parallel()
		body := "## First\n### Y\u030a\n### \u1e99 2\n## Y\u030a\n### y\u030a\n"
		r := newRenderer(t, []graph.NoteInput{{RelPath: "Target.md"}}, nil, transclusions{"Target.md": body})
		got := r.HTML("Target.md", "", body, wording.En)
		want := []string{"first", "\u1e99", "\u1e99-2", "\u1e99-3", "\u1e99-4"}
		if diff := cmp.Diff(want, allMatches(elementID, got.HTML)); diff != "" {
			t.Errorf("caught: fragment-normalization duplicate IDs (-want +got):\n%s", diff)
		}
		wantTOC := []render.TOCEntry{
			{Level: 2, Text: "First", ID: "first"},
			{Level: 3, Text: "Y\u030a", ID: "\u1e99"},
			{Level: 3, Text: "\u1e99 2", ID: "\u1e99-2"},
			{Level: 2, Text: "Y\u030a", ID: "\u1e99-3"},
			{Level: 3, Text: "y\u030a", ID: "\u1e99-4"},
		}
		if diff := cmp.Diff(wantTOC, got.TOC); diff != "" {
			t.Errorf("caught: fragment-normalization duplicate TOC (-want +got):\n%s", diff)
		}
		id, found := r.HeadingPath("Target.md", "\u1e99#\u1e99")
		if !found {
			t.Fatal("caught: fragment-normalization canonical parent path not found")
		}
		if id != "\u1e99-4" {
			t.Errorf("caught: fragment-normalization path ID = %q, want %q", id, "\u1e99-4")
		}
		link := r.HTML("Source.md", "", "[[Target#\u1e99#\u1e99|leaf]]", wording.En)
		if !strings.Contains(link.HTML, "href=\"/notes/Target.md#\u1e99-4\"") || len(link.Diagnostics) != 0 {
			t.Errorf("caught: fragment-normalization path link = %+v", link)
		}
	})
	t.Run("setext and embedded address ownership", func(t *testing.T) {
		t.Parallel()
		body := "Y\u030a\n---\n\n[[#\u1e99|inside]]\n"
		r := newRenderer(t, []graph.NoteInput{{RelPath: "Target.md"}}, nil, transclusions{"Target.md": body})
		page := r.HTML("Target.md", "", body, wording.En)
		if diff := cmp.Diff([]render.TOCEntry{{Level: 2, Text: "Y\u030a", ID: "\u1e99"}}, page.TOC); diff != "" {
			t.Errorf("caught: fragment-normalization setext TOC (-want +got):\n%s", diff)
		}
		got := r.HTML("Source.md", "", "![[Target]]\n\n[[Target#\u1e99|outside]]", wording.En)
		if strings.Count(got.HTML, "href=\"/notes/Target.md#\u1e99\"") != 2 || !strings.Contains(got.HTML, " id=\"\u1e99\"") || len(got.Diagnostics) != 0 {
			t.Errorf("caught: fragment-normalization embedded ownership = %+v", got)
		}
	})
}

func TestFragmentNormalizationExcerptsAndBlocks(t *testing.T) {
	t.Parallel()
	body := "## Y\u030a\n\nFIRST\n\n### Child\n\nDEEP\n\n## Stop\n\nOUTSIDE\n\nQuote. ^QUOTE-1\n\nOther. ^quote1\n"
	for _, tt := range []struct{ name, fragment, want string }{
		{"heading", "\u1e99", "## Y\u030a\n\nFIRST\n\n### Child\n\nDEEP\n"},
		{"path", "\u1e99#Child", "### Child\n\nDEEP\n"},
		{"ASCII block", "^quote-1", "Quote. ^QUOTE-1"},
		{"distinct ASCII block", "^quote1", "Other. ^quote1"},
		{"missing", "Absent", ""},
		{"closed path", "\u1e99#Stop", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, found := render.Excerpt(body, tt.fragment)
			if found != (tt.want != "") {
				t.Fatalf("caught: fragment-normalization Excerpt(%q).found = %t", tt.fragment, found)
			}
			if got != tt.want {
				t.Errorf("caught: fragment-normalization Excerpt(%q) = %q, want %q", tt.fragment, got, tt.want)
			}
			preview, previewFound, narrowed := render.ExcerptPreview(body, tt.fragment)
			if previewFound != found || narrowed {
				t.Fatalf("caught: fragment-normalization preview found/narrowed = %t/%t", previewFound, narrowed)
			}
			if preview != tt.want {
				t.Errorf("caught: fragment-normalization preview = %q, want %q", preview, tt.want)
			}
		})
	}
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Target.md"}}, nil, transclusions{"Target.md": body})
	page := r.HTML("Target.md", "", body, wording.En)
	if diff := cmp.Diff([]string{"^quote-1", "^quote1"}, page.Blocks); diff != "" {
		t.Errorf("caught: fragment-normalization ASCII blocks (-want +got):\n%s", diff)
	}
	if !strings.Contains(page.HTML, `<span id="^quote-1">^QUOTE-1</span>`) {
		t.Errorf("caught: fragment-normalization ASCII block HTML = %s", page.HTML)
	}
	link := r.HTML("Source.md", "", "[[Target#^quote-1|block]]", wording.En)
	if !strings.Contains(link.HTML, `href="/notes/Target.md#%5Equote-1"`) || len(link.Diagnostics) != 0 {
		t.Errorf("caught: fragment-normalization ASCII block href = %+v", link)
	}
	embed := r.HTML("Source.md", "", "![[Target#\u1e99]]", wording.En)
	if !strings.Contains(embed.HTML, "FIRST") || !strings.Contains(embed.HTML, "DEEP") || strings.Contains(embed.HTML, "OUTSIDE") || len(embed.Diagnostics) != 0 {
		t.Errorf("caught: fragment-normalization heading embed cut = %+v", embed)
	}
	missing := r.HTML("Source.md", "", "![[Target#Absent]]", wording.En)
	if strings.Contains(missing.HTML, "FIRST") || strings.Contains(missing.HTML, "OUTSIDE") {
		t.Errorf("caught: fragment-normalization missing excerpt widened to body: %s", missing.HTML)
	}
	var kinds []render.DiagnosticKind
	for _, diagnostic := range missing.Diagnostics {
		kinds = append(kinds, diagnostic.Kind)
	}
	if diff := cmp.Diff([]render.DiagnosticKind{render.DiagEmbedFragmentMissing}, kinds); diff != "" {
		t.Errorf("caught: fragment-normalization missing embed diagnostics (-want +got):\n%s", diff)
	}
}
