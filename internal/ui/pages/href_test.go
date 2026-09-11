package pages

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestNotesHrefSendsABriefingToTheReportSurface is the address half of the
// canonical-report lock: a daily-briefing HTML must not be offered as a /notes/
// source dump. Written reports, unregistered HTML, and a nested file under
// daily-briefing/ keep the notes address, which is how an unregistered file
// still opens as source.
func TestNotesHrefSendsABriefingToTheReportSurface(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "a registered briefing opens at the report surface",
			path: "System/reports/daily-briefing/browser-boundary.html",
			want: "/reports/browser-boundary.html",
		},
		{
			name: "a written markdown report stays a note",
			path: "System/reports/Week of 2026-08-31.md",
			want: "/notes/System/reports/Week%20of%202026-08-31.md",
		},
		{
			name: "unregistered html elsewhere stays a file page",
			path: "Notes/page.html",
			want: "/notes/Notes/page.html",
		},
		{
			name: "a nested file under daily-briefing is not a briefing",
			path: "System/reports/daily-briefing/sub/x.html",
			want: "/notes/System/reports/daily-briefing/sub/x.html",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := notesHref(tt.path); got != tt.want {
				t.Errorf("notesHref(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// TestObsidianHref pins the editor hand-off URI byte-for-byte. Each expected
// string is hand-derived from the escaping rules, not read back from the
// escaper: spaces are %20 (a "+" would reach Obsidian as a literal plus),
// CJK is UTF-8 percent-escaped, and "?", "#", "%", and "&" are escaped so a
// name cannot cut the single query parameter short or start a second one.
func TestObsidianHref(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		root string
		rel  string
		want string
	}{
		{
			name: "plain ascii path",
			root: "/Users/reader/vault",
			rel:  "Writing/lessons/japanese/L05.md",
			want: "obsidian://open?path=/Users/reader/vault/Writing/lessons/japanese/L05.md",
		},
		{
			name: "cjk and spaces are escaped",
			root: "/Users/reader/vault",
			rel:  "Concepts/golang/Go 位元運算.md",
			want: "obsidian://open?path=/Users/reader/vault/Concepts/golang/Go%20%E4%BD%8D%E5%85%83%E9%81%8B%E7%AE%97.md",
		},
		{
			name: "question mark cannot end the parameter",
			root: "/Users/reader/vault",
			rel:  "Notes/what is this?.md",
			want: "obsidian://open?path=/Users/reader/vault/Notes/what%20is%20this%3F.md",
		},
		{
			name: "hash cannot start a fragment",
			root: "/Users/reader/vault",
			rel:  "Notes/C#basics.md",
			want: "obsidian://open?path=/Users/reader/vault/Notes/C%23basics.md",
		},
		{
			name: "percent cannot start a stray escape",
			root: "/Users/reader/vault",
			rel:  "Notes/50% done.md",
			want: "obsidian://open?path=/Users/reader/vault/Notes/50%25%20done.md",
		},
		{
			name: "ampersand cannot start a second parameter",
			root: "/Users/reader/vault",
			rel:  "Notes/a&b.md",
			want: "obsidian://open?path=/Users/reader/vault/Notes/a%26b.md",
		},
		{
			name: "root segments are escaped too",
			root: "/Users/my reader/obsidian vault",
			rel:  "Notes/n.md",
			want: "obsidian://open?path=/Users/my%20reader/obsidian%20vault/Notes/n.md",
		},
		{
			name: "a slash inside a name stays a separator",
			root: "/Users/reader/vault",
			rel:  "Notes/a/b.md",
			want: "obsidian://open?path=/Users/reader/vault/Notes/a/b.md",
		},
		{
			name: "NFC composed form is escaped as written",
			root: "/Users/reader/vault",
			rel:  "Notes/caf\u00e9.md",
			want: "obsidian://open?path=/Users/reader/vault/Notes/caf%C3%A9.md",
		},
		{
			name: "decomposed form is escaped as written",
			root: "/Users/reader/vault",
			rel:  "Notes/cafe\u0301.md",
			want: "obsidian://open?path=/Users/reader/vault/Notes/cafe%CC%81.md",
		},
		{name: "empty root yields no link", rel: "Notes/n.md", want: ""},
		{name: "empty rel yields no link", root: "/Users/reader/vault", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ObsidianHref(tt.root, tt.rel); got != tt.want {
				t.Errorf("ObsidianHref(%q, %q) = %q, want %q", tt.root, tt.rel, got, tt.want)
			}
		})
	}
}

// TestHitFragment pins the text directive a result link carries. Each expected
// string is hand-derived from the directive grammar rather than read back from
// the escaper: the grammar spends "-" on the marks introducing a prefix and a
// suffix and "," on the ones separating its parameters, and it reads "&" as the
// start of a second directive, so none of the three may reach it unescaped. CJK
// is UTF-8 percent-escaped, and a space is %20 — a "+" would be matched as a
// literal plus and find nothing.
func TestHitFragment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		hit  SearchResult
		want string
	}{
		{
			// The two marks carry different words, so a directive built from the
			// last one would read differently from a directive built from the
			// first. With the same word twice the two are the same string, and
			// this case would hold whichever end it was written from.
			name: "the first marked stretch is the destination",
			hit:  SearchResult{SnippetRuns: []SnippetRun{{Text: "before "}, {Text: "kafka", Hit: true}, {Text: " and "}, {Text: "streams", Hit: true}, {Text: " after"}}},
			want: "#:~:text=kafka",
		},
		{
			name: "nothing marked, nowhere to point",
			hit:  SearchResult{SnippetRuns: []SnippetRun{{Text: "before kafka after"}}},
			want: "",
		},
		{
			name: "no excerpt at all",
			want: "",
		},
		{
			name: "cjk and spaces are escaped",
			hit:  SearchResult{SnippetRuns: []SnippetRun{{Text: "位元 運算", Hit: true}}},
			want: "#:~:text=%E4%BD%8D%E5%85%83%20%E9%81%8B%E7%AE%97",
		},
		{
			name: "a hyphen cannot introduce a prefix or a suffix",
			hit:  SearchResult{SnippetRuns: []SnippetRun{{Text: "read-aloud", Hit: true}}},
			want: "#:~:text=read%2Daloud",
		},
		{
			name: "a comma cannot separate a parameter",
			hit:  SearchResult{SnippetRuns: []SnippetRun{{Text: "one, two", Hit: true}}},
			want: "#:~:text=one%2C%20two",
		},
		{
			name: "an ampersand cannot start a second directive",
			hit:  SearchResult{SnippetRuns: []SnippetRun{{Text: "this & that", Hit: true}}},
			want: "#:~:text=this%20%26%20that",
		},
		{
			name: "a percent sign cannot begin an escape of its own",
			hit:  SearchResult{SnippetRuns: []SnippetRun{{Text: "100% done", Hit: true}}},
			want: "#:~:text=100%25%20done",
		},
		{
			name: "the edges of the term are trimmed, since a term that is not a word matches none",
			hit:  SearchResult{SnippetRuns: []SnippetRun{{Text: "  kafka  ", Hit: true}}},
			want: "#:~:text=kafka",
		},
		{
			name: "a mark holding only spaces is passed over",
			hit:  SearchResult{SnippetRuns: []SnippetRun{{Text: "   ", Hit: true}, {Text: "kafka", Hit: true}}},
			want: "#:~:text=kafka",
		},
		{
			name: "a phrase that spans two blocks names both ends",
			hit: SearchResult{
				SnippetRuns:   []SnippetRun{{Text: "cobalt egret", Hit: true}},
				Landing:       "cobalt",
				LandingEnd:    "egret",
				BlockCrossing: true,
			},
			want: "#:~:text=cobalt,egret",
		},
		{
			name: "a crossing match with only a last-block stretch names that stretch",
			hit: SearchResult{
				SnippetRuns:   []SnippetRun{{Text: "ghi", Hit: true}},
				LandingEnd:    "ghi",
				BlockCrossing: true,
			},
			want: "#:~:text=ghi",
		},
		{
			name: "a crossing match with nothing locatable carries no directive",
			hit: SearchResult{
				SnippetRuns:   []SnippetRun{{Text: "cobalt egret", Hit: true}},
				BlockCrossing: true,
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := hitFragment(&tt.hit); got != tt.want {
				t.Errorf("hitFragment() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCountUnitKeepsThePhraseAfterTheDigits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		n    int
		lang wording.Lang
		want string
	}{
		{name: "zh-Hant one", n: 1, lang: wording.ZhHant, want: " 篇筆記"},
		{name: "zh-Hant many", n: 3, lang: wording.ZhHant, want: " 篇筆記"},
		{name: "en one", n: 1, lang: wording.En, want: " note"},
		{name: "en many", n: 3, lang: wording.En, want: " notes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := countUnit(tt.n, wording.NoteCountOne, wording.NoteCountMany, tt.lang)
			if got != tt.want {
				t.Errorf("countUnit(%d, %s) = %q, want %q", tt.n, tt.lang, got, tt.want)
			}
		})
	}
}

func TestCountUnitDropsDigitsThatDoNotLeadThePhrase(t *testing.T) {
	t.Parallel()

	got := countUnit(3, wording.DegradedNoticeOne, wording.DegradedNoticeMany, wording.ZhHant)
	if strings.Contains(got, "3") {
		t.Errorf("countUnit returned the digits it should have taken out: %q", got)
	}
}
