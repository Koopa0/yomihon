package render

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestPlainProjectionDisplay(t *testing.T) {
	tests := []struct{ name, body, plain, shown, deleted string }{
		{"strike", "before ~~old~~ after", "before ~~old~~ after", "before old after", "old"},
		{"highlight", "before ==new== after", "before ==new== after", "before new after", ""},
		{"single", "~old~", "~old~", "old", "old"},
		{"triple", "before ~~~old~~~ after", "before ~~~old~~~ after", "before ~~~old~~~ after", ""},
		{"mixed open", "~~old~", "~~old~", "~old", "old"},
		{"mixed close", "~old~~", "~old~~", "old~", "old"},
		{"nested", "~~**old** `code` ==new==~~", "~~old code ==new==~~", "old code new", "old code new"},
		{"code", "`~~old~~ ==new==`", "~~old~~ ==new==", "~~old~~ ==new==", ""},
		{"fence", "```\n~~old~~ ==new==\n```", "~~old~~ ==new==", "~~old~~ ==new==", ""},
		{"escape", `\~old~ \=new==`, `\~old~ \=new==`, `\~old~ \=new==`, ""},
		{"unbalanced", "~~old ==new", "~~old ==new", "~~old ==new", ""},
		{"unicode", "  ~~旧語~~ ==é==  ", "~~旧語~~ ==é==", "旧語 é", "旧語"},
		{"heading", "## Name {sequence=primary} ###", "Name {sequence=primary}", "Name", ""},
		{"protected heading", "## *Name {sequence=primary}*", "Name {sequence=primary}", "Name {sequence=primary}", ""},
		{"code heading", "## Name `{sequence=primary}`", "Name {sequence=primary}", "Name {sequence=primary}", ""},
		{"h1", "# Name {sequence=primary}", "Name {sequence=primary}", "Name {sequence=primary}", ""},
		{"duplicate", "## Name {sequence=primary} {sequence=none}", "Name {sequence=primary} {sequence=none}", "Name {sequence=primary} {sequence=none}", ""},
		{"nameless", "## {sequence=primary}", "{sequence=primary}", "{sequence=primary}", ""},
		{"partial nested", "before ~~a~b~ after", "before ~~a~b~ after", "before ~~ab after", "b"},
		{name: "repeated opener", body: "before ~~a~ b~ after", plain: "before ~~a~ b~ after", shown: "before a b after", deleted: "a b"},
		{name: "joined rune", body: "~\xe9<A>\xa7\x80~", plain: "~\xe9\xa7\x80~", shown: "\xe9\xa7\x80", deleted: "\xe9\xa7\x80"},
		{"nested widths", "before ~a ~~b~~ c~ after", "before ~a ~~b~~ c~ after", "before a b c after", "a b c"},
		{"autolink", "~~<https://example.test/old>~~", "~~https://example.test/old~~", "https://example.test/old", "https://example.test/old"},
		{"hard break", "~~old  \nnew~~", "~~old\nnew~~", "old\nnew", "old\nnew"},
		{"soft break", "~~old\nnew~~", "~~old\nnew~~", "old\nnew", "old\nnew"},
		{"ruby", "~~<ruby>漢<rt>かん</rt></ruby>字~~", "~~漢字~~\nかん", "漢字\nかん", "漢字かん"},
		{"repeated", "old ~~old~~ old ==old==", "old ~~old~~ old ==old==", "old old old old", "old"},
		{"emphasis name", "## **Name** {sequence=local}", "Name {sequence=local}", "Name", ""},
		{"quoted heading", "> ## Name {sequence=none}", "Name {sequence=none}", "Name", ""},
		{"setext heading", "Name {sequence=none}\n---", "Name {sequence=none}", "Name", ""},
		{"format list", "- *Name {sequence=primary}*", "Name {sequence=primary}", "Name {sequence=primary}", ""},
		{"loose list", "- Name {sequence=primary}\n\n- Other", "Name {sequence=primary}\nOther", "Name\nOther", ""},
		{"unknown", "## Name {sequence=unknown}", "Name {sequence=unknown}", "Name {sequence=unknown}", ""},
		{"nonterminal", "## Name {sequence=primary} tail", "Name {sequence=primary} tail", "Name {sequence=primary} tail", ""},
		{"equals single triple", "=one= ===three===", "=one= ===three===", "=one= =three=", ""},

		{"escaped role", `## Name \{sequence=primary\}`, `Name \{sequence=primary\}`, "Name", ""},
		{"entity role", "## Name &#123;sequence&#61;primary&#125;", "Name &#123;sequence&#61;primary&#125;", "Name", ""},
		{"callout title", "> [!note] ## Name {sequence=primary}", "Name {sequence=primary}", "Name {sequence=primary}", ""},
		{"callout list title", "> [!note] - Name {sequence=primary}", "Name {sequence=primary}", "Name {sequence=primary}", ""},
		{"callout body heading", "> [!note] Title\n> ## Name {sequence=primary}", "Title\nName {sequence=primary}", "Title\nName", ""},
		{"callout literal delimiters", "> [!note] ~~old~~ ==new==", "~~old~~ ==new==", "~~old~~ ==new==", ""},
		{"wikilink literal delimiters", "## [[Foo|~~old~~ ==new==]]", "Foo ~~old~~ ==new==", "Foo ~~old~~ ==new==", ""},
		{"wikilink mixed delimiters", "before ~~[[Foo|old~~]] after", "before ~~Foo old~~ after", "before ~~Foo old~~ after", ""},
		{"wikilink outer delimiters", "~~[[Foo|Name]]~~", "~~Foo Name~~", "Foo Name", "Foo Name"},
		{"wikilink alias", "## [[Foo|Name {sequence=none}]]", "Foo Name {sequence=none}", "Foo Name {sequence=none}", ""},
		{"wikilink outside role", "## [[Foo|Name]] {sequence=none}", "Foo Name {sequence=none}", "Foo Name", ""},
		{"escaped wikilink", `## \[[Foo|Name {sequence=none}]]`, `\Foo Name {sequence=none}`, `\Foo Name {sequence=none}`, ""},
		{"url role", "## https://example.test/{sequence=primary}", "https://example.test/{sequence=primary}", "https://example.test/{sequence=primary}", ""},
		{"list", "- Parent {sequence=primary}\n  - Child {sequence=local}", "Parent {sequence=primary}\nChild {sequence=local}", "Parent\nChild", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := PlainProjection(tt.body)
			if p.Text != tt.plain {
				t.Fatalf("corpus = %q, want %q", p.Text, tt.plain)
			}
			shown, deleted := projectionDisplay(t, &p)
			if shown != tt.shown || deleted != tt.deleted {
				t.Fatalf("shown = %q, deleted = %q; want %q, %q; spans=%+v", shown, deleted, tt.shown, tt.deleted, p.DisplaySpans)
			}
		})
	}
}

func projectionDisplay(t *testing.T, p *Projection) (shown, deletedText string) {
	t.Helper()
	hiddenAt, deletedAt := make([]int, len(p.Text)+1), make([]int, len(p.Text)+1)
	for _, s := range p.DisplaySpans {
		if s.Start < 0 || s.Start >= s.End || s.End > len(p.Text) {
			t.Fatalf("invalid span %+v in %q", s, p.Text)
		}
		if s.Hidden {
			hiddenAt[s.Start]++
			hiddenAt[s.End]--
		}
		if s.Deleted {
			deletedAt[s.Start]++
			deletedAt[s.End]--
		}
	}
	var visible, deleted strings.Builder
	hide, del := 0, 0
	for i := range len(p.Text) {
		hide += hiddenAt[i]
		del += deletedAt[i]
		if hide == 0 {
			visible.WriteByte(p.Text[i])
			if del > 0 {
				deleted.WriteByte(p.Text[i])
			}
		}
	}
	return visible.String(), deleted.String()
}

func TestPlainProjectionNoteCeiling(t *testing.T) {
	count := MaxSourceBytes / len("~~x~~ ")
	body := strings.Repeat("~~x~~ ", count) + "z"
	p := PlainProjection(body)
	if p.Text != body {
		t.Fatal("independent pairs changed searchable corpus")
	}
	shown, deleted := projectionDisplay(t, &p)
	if shown != strings.Repeat("x ", count)+"z" || deleted != strings.Repeat("x", count) {
		t.Fatalf("independent pairs lost display effects: shown bytes=%d, deleted bytes=%d", len(shown), len(deleted))
	}
}

func FuzzPlainProjection(f *testing.F) {
	for _, body := range []string{"~~old~~ ==new==", "~a ~~b~~ c~", "## Name {sequence=primary} ###", "- Parent {sequence=none}\n  - Child {sequence=local}", "~~<ruby>漢<rt>かん</rt></ruby>字~~", "\n\t~~é~~\n", "```\n~~code~~\n```", "\\~escaped~", "~\xe9<A>\xa7\x80~"} {
		f.Add(body)
	}
	f.Fuzz(func(t *testing.T, body string) {
		p := PlainProjection(body)
		plain, blocks, fences := PlainBlocks(body)
		if p.Text != plain {
			t.Fatalf("projection changed corpus: got %q, want %q", p.Text, plain)
		}
		if diff := cmp.Diff(blocks, p.Blocks); diff != "" {
			t.Fatalf("projection changed blocks (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(fences, p.FenceRanges); diff != "" {
			t.Fatalf("projection changed fences (-want +got):\n%s", diff)
		}
		projectionDisplay(t, &p)
		if !utf8.ValidString(p.Text) {
			return
		}
		for _, span := range p.DisplaySpans {
			if !utf8.ValidString(p.Text[:span.Start]) || !utf8.ValidString(p.Text[:span.End]) {
				t.Fatalf("span splits a rune: %+v in %q", span, p.Text)
			}
		}
	})
}

func TestPageDisplayGrammar(t *testing.T) {
	const body = "## ~~Old~~ ==New== {sequence=primary}\n\n- [x] ~~Task~~\n- Row {sequence=local}\n\nText[^n]\n\n[^n]: ~~Foot~~ ==Bright==\n\n| Left | Right |\n|---|---|\n|~~Cell~~|==Value==|\n\n<ruby>漢<rt>かん</rt></ruby>\n\n```\n~~literal~~ ==literal==\n```\n"
	t.Run("page", func(t *testing.T) {
		t.Log("invoked: page/display grammar page consumer")
		output := New(graph.BuildFromNotes(nil, nil), noBodies{}, anyTitle{}, holdsEverything{}).HTML("Notes/Fixture.md", "", body, wording.En).HTML
		for _, markup := range []string{"<del>Old</del>", "<mark>New</mark>", "<del>Task</del>", "<del>Foot</del>", "<mark>Bright</mark>", "<del>Cell</del>", "<mark>Value</mark>", "<table>", `type="checkbox"`, "<ruby>", "~~literal~~ ==literal=="} {
			if !strings.Contains(output, markup) {
				t.Errorf("caught: page grammar missing %q: %s", markup, output)
			}
		}
	})
	t.Run("display", func(t *testing.T) {
		t.Log("invoked: page/display grammar projection consumer")
		projection := PlainProjection(body)
		plain, blocks, fences := PlainBlocks(body)
		if projection.Text != plain || !cmp.Equal(blocks, projection.Blocks) || !cmp.Equal(fences, projection.FenceRanges) {
			t.Fatal("caught: display grammar changed the independent corpus/blocks/fences")
		}
		shown, deleted := projectionDisplay(t, &projection)
		for _, hidden := range []string{"{sequence=primary}", "{sequence=local}", "~~Old~~", "==New==", "~~Task~~", "~~Foot~~", "==Bright==", "~~Cell~~", "==Value=="} {
			if strings.Contains(shown, hidden) {
				t.Errorf("caught: display grammar left %q in %q", hidden, shown)
			}
		}
		for _, text := range []string{"Old", "Task", "Foot", "Cell"} {
			if !strings.Contains(deleted, text) {
				t.Errorf("caught: display grammar lost retraction %q in %q", text, deleted)
			}
		}
		if !strings.Contains(shown, "~~literal~~ ==literal==") {
			t.Errorf("caught: display grammar changed literal code: %q", shown)
		}
	})
}
