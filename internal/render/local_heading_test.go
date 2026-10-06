package render_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestSameNoteHeadingLinkUsesTheRenderedBody(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ heading, fragment, id, label string }{
		{"Second section", "Second section", "second-section", "#Second section"},
		{"第三節：失約的燈", "第三節：失約的燈", "第三節-失約的燈", "#第三節：失約的燈"},
		{"がん", "か\u3099ん", "がん", "#か\u3099ん"},
		{"A & B!", "a & b!", "a-b", "#a & b!"},
	} {
		for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
			t.Run(tt.heading+" "+string(lang), func(t *testing.T) {
				t.Parallel()
				// The current body is authoritative even if no graph entry or captured
				// transclusion describes it, or the captured body is a different one.
				r := newRenderer(t, nil, nil, transclusions{"note.md": "## Stale section\n"})
				body := "[[#" + tt.fragment + "]] and [[#" + tt.fragment + "|go <script> & **plain**]]\n\n## " + tt.heading + "\n\nTarget body.\n"
				got := r.HTML("note.md", "", body, lang)
				if id := headingID(t, &got, tt.heading); id != tt.id {
					t.Errorf("actual heading ID = %q, want %q", id, tt.id)
				}
				want := `<a href="#` + tt.id + `" class="wikilink">` + strings.ReplaceAll(tt.label, "&", "&amp;") + `</a>`
				if !strings.Contains(got.HTML, want) || !strings.Contains(got.HTML, `<a href="#`+tt.id+`" class="wikilink">go &lt;script&gt; &amp; **plain**</a>`) {
					t.Errorf("same-note heading links missing:\n%s", got.HTML)
				}
				if len(got.Diagnostics) != 0 {
					t.Errorf("local heading diagnostics = %+v", got.Diagnostics)
				}
			})
		}
	}
}

func TestSameNoteMissingHeadingMatchesCrossNoteDiagnostic(t *testing.T) {
	t.Parallel()
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		t.Run(string(lang), func(t *testing.T) {
			t.Parallel()
			body := "[[#Missing local|where]]\n\n## Other\n\nBody.\n"
			r := newRenderer(t, []graph.NoteInput{{RelPath: "other.md"}}, nil, transclusions{"other.md": body, "note.md": "## Missing local\n"})
			local := r.HTML("note.md", "", body, lang)
			cross := r.HTML("source.md", "", "[[other#Missing local|where]]\n", lang)
			if len(local.Diagnostics) != 1 {
				t.Fatalf("local missing-heading diagnostics = %+v, want one", local.Diagnostics)
			}
			if len(cross.Diagnostics) != 1 {
				t.Fatalf("cross-note control diagnostics = %+v", cross.Diagnostics)
			}
			want := render.Diagnostic{Kind: render.DiagLinkSectionMissing, Section: "Missing local", Message: `no heading in "note.md" matched "Missing local"; the address is left as written and may land at the top of the note`}
			if diff := cmp.Diff(want, local.Diagnostics[0]); diff != "" {
				t.Errorf("local diagnostic (-want +got):\n%s", diff)
			}
			if local.Diagnostics[0].Kind != cross.Diagnostics[0].Kind {
				t.Error("local/cross-note diagnostic kind drift")
			}
			wantHTML := strings.ReplaceAll(cross.HTML, `/notes/other.md#missing-local`, `#missing-local`)
			if diff := cmp.Diff(wantHTML, strings.Split(local.HTML, "\n")[0]+"\n"); diff != "" {
				t.Errorf("local degraded link (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSameNoteHeadingLinkKeepsLiteralAndFragmentControls(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)
	for _, body := range []string{"`[[#Section]]`\n", "\\[[#Section]]\n", "    [[#Section]]\n", "%% [[#Section]] %%\n", "![[#Section]]\n", "[[#^address]]\n", "[[^address]]\n", "[[#]]\n"} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("note.md", "", body, wording.En)
			if strings.Contains(got.HTML, `class="wikilink`) || len(got.Diagnostics) != 0 {
				t.Errorf("literal/unsupported local control became a link: %+v", got)
			}
		})
	}
}

func TestSameNoteHeadingLinkReachesTheRemovedTitle(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)
	got := r.HTML("note.md", "Own title", "# Own title\n\n[[#Own title|up]]\n", wording.En)
	if got.TitleAnchor != "own-title" || !strings.Contains(got.HTML, `<a href="#own-title" class="wikilink">up</a>`) || len(got.Diagnostics) != 0 {
		t.Errorf("local title link lost its actual page anchor: %+v", got)
	}
}

func TestSameNoteHeadingLinksInAnEmbedKeepSourceOwnership(t *testing.T) {
	t.Parallel()
	target := "## Inside\n\n[[#Inside|inside]] and [[#Outside|outside]]\n\nContent.\n\n## Outside\n\nOther.\n"
	r := newRenderer(t, []graph.NoteInput{{RelPath: "target.md"}}, nil, transclusions{"target.md": target})
	got := r.HTML("host.md", "", "## Host\n\n![[target#Inside]]\n", wording.En)
	// An excerpt belongs to its source note. Its local references must not
	// become jumps to unrelated or absent headings on the enclosing host page.
	if !strings.Contains(got.HTML, `href="/notes/target.md#inside" class="wikilink">inside</a>`) || !strings.Contains(got.HTML, `href="/notes/target.md#outside" class="wikilink">outside</a>`) || len(got.Diagnostics) != 0 {
		t.Errorf("embedded same-note link lost source ownership: %+v", got)
	}
}

func TestSameNoteHeadingLinksInSeparateRegionsUseTheSourcePage(t *testing.T) {
	t.Parallel()
	full := "## Inside\n\n[[#Outside|outside]]\n\n## Outside\n\nOther.\n"
	r := newRenderer(t, []graph.NoteInput{{RelPath: "note.md"}}, nil, transclusions{"note.md": full})
	for _, tt := range []struct{ name, body string }{
		{"preview", "## Inside\n\n[[#Outside|outside]]\n"},
		{"self embed", "![[note#Inside]]\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got render.Result
			if tt.name == "preview" {
				got = r.HTMLIn("preview", "note.md", "", tt.body, wording.En)
			} else {
				got = r.HTML("note.md", "", tt.body, wording.En)
			}
			if !strings.Contains(got.HTML, `href="/notes/note.md#outside" class="wikilink">outside</a>`) || len(got.Diagnostics) != 0 {
				t.Errorf("separate region resolved against an excerpt: %+v", got)
			}
		})
	}
}

func TestSameNoteHeadingLinkWithEmptyAliasUsesItsFragmentName(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)
	got := r.HTML("note.md", "", "[[#Section| ]]\n\n## Section\n", wording.En)
	if !strings.Contains(got.HTML, `<a href="#section" class="wikilink">#Section</a>`) || len(got.Diagnostics) != 0 {
		t.Errorf("empty local alias left an unnamed control: %+v", got)
	}
}
