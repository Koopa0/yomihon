package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

func fragmentVault(t *testing.T, other string) *render.Pipeline {
	t.Helper()
	return markdownRenderer(t, []graph.NoteInput{{RelPath: "Host/Other.md"}, {RelPath: "Host/here.md"}}, []string{"Host/picture.png"}, transclusions{"Host/Other.md": other})
}

func TestMarkdownLinkFragmentLandsOnTheHeadingItNames(t *testing.T) {
	t.Parallel()
	r := fragmentVault(t, "# Title\n\n## What's new?\n\n## Intro\n\n## Intro\n\n## 4. 主題清單（129 項）\n")
	for _, tt := range []struct{ name, dest, want string }{
		{"GitHub spelling of punctuation", "Other.md#whats-new", `href="/notes/Host/Other.md#what-s-new"`},
		{"GitHub spelling of a repeat", "Other.md#intro-1", `href="/notes/Host/Other.md#intro-2"`},
		{"GitHub spelling of CJK and full-width marks", "Other.md#4-主題清單129-項", `href="/notes/Host/Other.md#4-%E4%B8%BB%E9%A1%8C%E6%B8%85%E5%96%AE-129-%E9%A0%85"`},
		{"the heading's words", "Other.md#What%27s%20new%3F", `href="/notes/Host/Other.md#what-s-new"`},
		{"a name the page already stamps", "Other.md#what-s-new", `href="/notes/Host/Other.md#what-s-new"`},
		{"a query before the fragment", "Other.md?v=1#whats-new", `href="/notes/Host/Other.md?v=1#what-s-new"`},
		{"upper case folds", "Other.md#WHATS-NEW", `href="/notes/Host/Other.md#what-s-new"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("Host/here.md", "", "[go]("+tt.dest+")\n", wording.En)
			if !strings.Contains(got.HTML, tt.want) || strings.Contains(got.HTML, "wikilink-degraded") || len(got.Diagnostics) != 0 {
				t.Errorf("want %s with no fault, got %+v", tt.want, got)
			}
		})
	}
}

func TestMarkdownLinkFragmentThatReachesNothingSaysSo(t *testing.T) {
	t.Parallel()
	r := fragmentVault(t, "## Real\n")
	got := r.HTML("Host/here.md", "", "[go](Other.md?v=1#3-1-renamed)\n", wording.En)
	for _, want := range []string{`href="/notes/Host/Other.md?v=1#3-1-renamed"`, `class="wikilink wikilink-degraded"`, `No section called`} {
		if !strings.Contains(got.HTML, want) {
			t.Errorf("missing %q in %s", want, got.HTML)
		}
	}
	if len(got.Diagnostics) != 1 || got.Diagnostics[0].Kind != render.DiagLinkSectionMissing || got.Diagnostics[0].Section != "3-1-renamed" {
		t.Errorf("diagnostics = %+v, want one link-section-missing", got.Diagnostics)
	}
}

func TestMarkdownLinkFragmentLeavesWhatIsNotANoteSection(t *testing.T) {
	t.Parallel()
	r := fragmentVault(t, "## Real\n")
	for _, tt := range []struct{ name, body, want string }{
		{"a picture", "[p](picture.png#page=2)", `href="/raw/Host/picture.png#page=2"`},
		{"the top of the page", "[t](Other.md#top)", `href="/notes/Host/Other.md#top"`},
		{"no fragment", "[n](Other.md#)", `href="/notes/Host/Other.md#"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("Host/here.md", "", tt.body, wording.En)
			if !strings.Contains(got.HTML, tt.want) || strings.Contains(got.HTML, "wikilink-degraded") || len(got.Diagnostics) != 0 {
				t.Errorf("want %s with no fault, got %+v", tt.want, got)
			}
		})
	}
}

func TestMarkdownLinkToAPlaceInTheSameNote(t *testing.T) {
	t.Parallel()
	r := fragmentVault(t, "")
	body := "## What's new?\n\n[a](#whats-new) [b](#What's%20new%3F) [c](#gone)\n"
	got := r.HTML("Host/here.md", "", body, wording.En)
	if strings.Count(got.HTML, `<a href="#what-s-new">`) != 2 {
		t.Errorf("both spellings of an existing heading must land on its id:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, `<a href="#gone" class="wikilink wikilink-degraded"`) {
		t.Errorf("a heading that does not exist must be shown as missing:\n%s", got.HTML)
	}
	if len(got.Diagnostics) != 1 || got.Diagnostics[0].Section != "gone" {
		t.Errorf("diagnostics = %+v, want one for #gone", got.Diagnostics)
	}
}

func TestWikilinkFragmentAcceptsTheGitHubSpellingToo(t *testing.T) {
	t.Parallel()
	r := fragmentVault(t, "## What's new?\n")
	got := r.HTML("Host/here.md", "", "[[Other#whats-new]] [[Other#What's new?]] [[Other#nothing-here]]\n", wording.En)
	if strings.Count(got.HTML, `href="/notes/Host/Other.md#what-s-new"`) != 2 {
		t.Errorf("both spellings must reach the heading:\n%s", got.HTML)
	}
	if len(got.Diagnostics) != 1 || got.Diagnostics[0].Section != "nothing-here" {
		t.Errorf("diagnostics = %+v, want one for #nothing-here", got.Diagnostics)
	}
}

func TestBracketsInANameMakeNoWikilink(t *testing.T) {
	t.Parallel()
	r := fragmentVault(t, "")
	got := r.HTML("Host/here.md", "", "pairs [[1,2],[1,100]] and [[Other]]\n", wording.En)
	if len(got.Diagnostics) != 0 || strings.Contains(got.HTML, "wikilink-broken") {
		t.Errorf("an array of numbers was read as a link: %+v\n%s", got.Diagnostics, got.HTML)
	}
	if !strings.Contains(got.HTML, "[[1,2],[1,100]]") || !strings.Contains(got.HTML, `href="/notes/Host/Other.md"`) {
		t.Errorf("the array must stay as written beside a real link:\n%s", got.HTML)
	}
}
