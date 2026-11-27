package lexical

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/vault"
)

func TestLandingContextInsideChangedBlocks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		body   string
		query  string
		suffix string
	}{
		{name: "struck sentence", body: "~~加一格 buffer 就能取消工作。~~ 它只能暫存一個值。\n", query: "取消", suffix: "工作"},
		{name: "after a struck sentence", body: "~~撤回。~~取消工作。\n", query: "取消", suffix: "工作"},
		{name: "nfd before the struck sentence", body: "cafe\u0301cafe\u0301cafe\u0301。\n\n~~取消工作。~~\n", query: "取消", suffix: "工作"},
		{name: "nfd inside the context", body: "~~取消cafe\u0301工作。~~\n", query: "取消", suffix: "café工作"},
		{name: "trimmed leading space", body: "\n\n~~取消工作。~~\n\n", query: "取消", suffix: "工作"},
		{name: "ruby reading in the same block", body: "<ruby>今日<rt>きょう</rt></ruby>取消工作。\n", query: "取消"},
		{name: "rewritten wikilink in the same block", body: "[[來源]]取消工作。\n", query: "取消"},
		{name: "a consumed highlight", body: "==取消工作。==\n", query: "取消"},
		{name: "no certain boundary before a removed delimiter", body: "~~取消工作~~仍然阻塞。\n", query: "取消"},
		{name: "a suffix crossing a removed delimiter", body: "取消~~工作。~~\n", query: "取消"},
		{name: "nfd contraction cannot cover following markup", body: "~~e\u0301e\u0301e\u0301e\u0301e\u0301e\u0301取消工作~~。\n", query: "取消"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := DocumentFromNote(vault.Parse("Notes/context.md", []byte(tt.body)))
			idx := NewIndex([]Document{doc}, validArtifactPolicy(t))
			got := soleResult(t, idx, tt.query)
			if diff := cmp.Diff(tt.suffix, got.LandingSuffix); diff != "" {
				t.Errorf("Search(%q).LandingSuffix (-want +got):\n%s", tt.query, diff)
			}
		})
	}
}

// TestHeadingHitNamesTheSectionOpening pins what a hit inside a heading
// names. The contents list repeats an unchanged heading's words, so a run
// beside the match comes from outside the heading: the section's first words
// after it, which the list's copy is never followed by, or else the previous
// block's last words, which it is never preceded by. A heading hit given
// neither keeps the directive any other heading hit has, and is not pinned
// here; TestHeadingHitsRespectDecodedReadingContext pins that directive
// where the gate is what withheld the context.
func TestHeadingHitNamesTheSectionOpening(t *testing.T) {
	t.Parallel()

	type landing struct{ Prefix, Landing, Bare, Suffix string }
	tests := []struct {
		name, body, query string
		want              landing
	}{
		{name: "cjk heading", body: "## 等待者應該放在哪裡？\n\n若把 `wg.Wait()` 與 `close(results)` 移回 `main`。\n", query: "等待", want: landing{Landing: "等待者應該放在哪裡？", Bare: "等待者應該放在哪裡？", Suffix: "若把 wg.Wait() 與"}},
		{name: "english heading", body: "Intro.\n\n## Where the inkwell waits\n\nThe shelf keeps it dry all winter.\n", query: "inkwell", want: landing{Landing: "inkwell waits", Bare: "inkwell waits", Suffix: "The shelf keeps"}},
		{name: "unspaced opening stops at a certain boundary", body: "## 等待者\n\n若把等待移回主程式之前會發生什麼事情呢如果一直寫下去，就這樣。\n", query: "等待者", want: landing{Landing: "等待者", Bare: "等待者", Suffix: "若把等待移回主程式之前會發生什麼事情呢如果一直寫下去"}},
		{name: "a heading follows, prose before", body: "## First\n\nIntro words here.\n\n## Where the inkwell waits\n\n## Next\n", query: "inkwell", want: landing{Prefix: "Intro words here.", Landing: "Where the inkwell waits", Bare: "Where the inkwell waits"}},
		{name: "decoded opening is preferred", body: "## Earlier\n\nPrior words here.\n\n## Where the inkwell waits\n\nTom &amp; Jerry run.\n", query: "inkwell", want: landing{Landing: "inkwell waits", Bare: "inkwell waits", Suffix: "Tom & Jerry"}},
		{name: "opening before link insertion is preferred", body: "## Earlier\n\nPrior words here.\n\n## Where the inkwell waits\n\n[Go docs](https://go.dev) explains it.\n", query: "inkwell", want: landing{Landing: "inkwell waits", Bare: "inkwell waits", Suffix: "Go docs"}},
		{name: "a cjk heading follows, prose before", body: "## 前言\n\n先讀這段。\n\n## 等待者\n\n### 下一節\n", query: "等待", want: landing{Prefix: "先讀這段。", Landing: "等待者", Bare: "等待者"}},
		{name: "the title heading the page drops", body: "# heading\n\nOpening words here.\n", query: "heading", want: landing{Landing: "heading", Bare: "heading"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := DocumentFromNote(vault.Parse("Notes/heading.md", []byte(tt.body)))
			idx := NewIndex([]Document{doc}, validArtifactPolicy(t))
			got := soleResult(t, idx, tt.query)
			if diff := cmp.Diff(tt.want, landing{got.LandingPrefix, got.Landing, got.LandingBare, got.LandingSuffix}); diff != "" {
				t.Errorf("Search(%q) heading landing (-want +got):\n%s", tt.query, diff)
			}
		})
	}
}

func TestDroppedTitleKeepsMainLanding(t *testing.T) {
	t.Parallel()

	const path = "Notes/comment-before-title.md"
	const title = "comment-before-title"
	const body = "%% prefatory comment %%\n# comment-before-title\n\nOpening words here.\n"
	doc := DocumentFromNote(vault.Parse(path, []byte(body)))
	if doc.Title != title {
		t.Fatalf("DocumentFromNote().Title = %q, want %q", doc.Title, title)
	}
	got := soleResult(t, NewIndex([]Document{doc}, validArtifactPolicy(t)), title)
	want := Result{
		RelPath:     path,
		Title:       title,
		Landing:     title,
		LandingBare: title,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Search(%q) mismatch (-want +got):\n%s", title, diff)
	}
}

func TestLandingCutsAtPageInsertions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, body, query, prefix, landing, bare, suffix string
	}{
		{name: "cjk after link", body: "甲[乙](https://go.dev)丙丁搜尋戊己。", query: "搜尋", prefix: "丙丁", landing: "搜尋", bare: "搜尋", suffix: "戊己"},
		{name: "cjk before link", body: "甲搜尋乙[丙](https://go.dev)丁。", query: "搜尋", prefix: "甲", landing: "搜尋", bare: "搜尋", suffix: "乙丙"},
		{name: "word after link", body: "[alpha](https://go.dev)betaword", query: "beta", landing: "betaword", bare: "betaword"},
		{name: "word before link", body: "[alphaword](https://go.dev)beta", query: "alpha", landing: "alphaword", bare: "alphaword"},
		{name: "match crosses link", body: "[alpha](https://go.dev)beta", query: "alphabeta"},
		{name: "nfd before link", body: "cafe\u0301 [乙](https://go.dev)搜尋尾。", query: "搜尋", landing: "搜尋", bare: "搜尋", suffix: "尾"},
		{name: "composed link edge", body: "[e](https://go.dev)\u0301 tail", query: "é"},
		{name: "reordered link edge", body: "[a\u0315](https://go.dev)\u0300 tail", query: "à"},
		{name: "multiple links", body: "[甲](https://a.test)乙搜尋丙[丁](https://b.test)戊。", query: "搜尋", prefix: "乙", landing: "搜尋", bare: "搜尋", suffix: "丙丁"},
		{name: "body ends at link", body: "搜尋[尾](https://go.dev)", query: "搜尋", landing: "搜尋", bare: "搜尋", suffix: "尾"},
		{name: "code is literal", body: "`snake\\_case &amp;`", query: "&amp;", prefix: "snake\\_case", landing: "&amp;", bare: "&amp;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			document := DocumentFromNote(vault.Parse("Notes/fixture.md", []byte(tt.body)))
			results := searchResults(t, NewIndex([]Document{document}, validArtifactPolicy(t)), Parse(tt.query))
			if len(results) != 1 {
				t.Fatalf("Search(%q) = %v, want one result", tt.query, results)
			}
			got := []string{results[0].LandingPrefix, results[0].Landing, results[0].LandingBare, results[0].LandingSuffix}
			want := []string{tt.prefix, tt.landing, tt.bare, tt.suffix}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("Search(%q) insertion context (-want +got):\n%s", tt.query, diff)
			}
		})
	}
}

func TestInsertionNFCMapping(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, text    string
		offsets, want []int
		barriers      [][2]int
	}{
		{name: "nfd before boundary", text: "e\u0301x", offsets: []int{3}, want: []int{2}},
		{name: "composition across boundary", text: "e\u0301x", offsets: []int{1}, want: []int{0, 2}, barriers: [][2]int{{0, 2}}},
		{name: "body edges", text: "ab", offsets: []int{0, 1, 2}, want: []int{0, 1, 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, barriers := remapInsertions(tt.text, tt.offsets)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("remapInsertions(%q) offsets (-want +got):\n%s", tt.text, diff)
			}
			if diff := cmp.Diff(tt.barriers, barriers); diff != "" {
				t.Errorf("remapInsertions(%q) barriers (-want +got):\n%s", tt.text, diff)
			}
		})
	}
}

// TestALandingIsTheWholeMarkAtTheEarliestMatch holds a landing to the stretch
// the page marks as one at the earliest match: every match of any term that
// overlaps or touches it is part of it, the note's own spacing stands in it,
// and neither a later copy of the same words, a term overlapping itself, nor a
// match beyond a line break is taken for it.
func TestALandingIsTheWholeMarkAtTheEarliestMatch(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, text, query, want string
	}{
		{"overlap", "The ledger holds lanthanum here.", `nthanu "nthanum here"`, "nthanum here"},
		{"collapsed whitespace", "before\tlanthanum\n\t here after", `"nthanum here"`, "nthanum here"},
		{"unicode space", "before\u3000lanthanum\u3000here after", `" lanthanum here "`, "lanthanum here"},
		{"folded character", "\u017f\u017f before lanthanum here", "nthanum", "nthanum"},
		{"repeated text", "unmatched before lanthanum here and lanthanum later", "nthanum", "nthanum"},
		{"a term never overlaps itself", "before ### after", "##", "##"},
		{"a line break parts two matches", "\u4e8b\u4ef6\n\u4e8b\u4ef6 after", "\u4e8b\u4ef6", "\u4e8b\u4ef6"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			idx := NewIndex([]Document{{RelPath: "Notes/n.md", Title: "n", PlainText: tt.text,
				Blocks: []render.Block{{End: len(tt.text), Verbatim: true}}}}, validArtifactPolicy(t))
			got := soleResult(t, idx, tt.query)
			if got.Landing != tt.want {
				t.Errorf("Landing = %q, want %q", got.Landing, tt.want)
			}
		})
	}
}

// TestALandingStopsWhereAFenceBegins holds the run of marks a prose landing
// takes to the prose: a phrase that carries the match on into the fence that
// follows it marks across the break, and the landing is still the prose's own
// run rather than one ending inside the code.
func TestALandingStopsWhereAFenceBegins(t *testing.T) {
	t.Parallel()

	doc := DocumentFromNote(vault.Parse("Notes/fence-after.md", []byte("Prose ends with alpha\n\n```\nbeta gamma\n```\n")))
	if len(doc.FenceRanges) != 1 {
		t.Fatalf("DocumentFromNote().FenceRanges = %v, want the one fence", doc.FenceRanges)
	}
	got := soleResult(t, NewIndex([]Document{doc}, validArtifactPolicy(t)), `alpha "alpha beta"`)
	if got.Landing != "alpha" || got.BlockCrossing || got.LandingEnd != "" || got.FromFence {
		t.Errorf("Landing = %q, crossing %t to %q, from fence %t; want the prose run alpha alone",
			got.Landing, got.BlockCrossing, got.LandingEnd, got.FromFence)
	}
}

// TestALandingHoldsTheMatchWhenTheFoldChangesLength holds the landing on the
// hit when case folding before it does not preserve length: Ⱥ grows from two
// bytes to three and ſ shrinks from two to one, so an offset found on the
// folded copy and used as an index into the note's own bytes drifts one byte
// per such character, into the middle of a character or clear of the term.
func TestALandingHoldsTheMatchWhenTheFoldChangesLength(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, text, query string
	}{
		{"a fold that grows", strings.Repeat("\u023a", 120) + "needle " + strings.Repeat("x", 400), "needle"},
		{"a fold that shrinks", strings.Repeat("\u017f", 400) + "needle " + strings.Repeat("x", 400), "needle"},
		{"a shrinking fold before Han", "\u017f" + strings.Repeat("今天出門散步看見一隻貓在牆上睡覺，", 6) + "天氣很好，回家以後泡了一壺茶。", "天氣"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			idx := NewIndex([]Document{{RelPath: "Notes/n.md", Title: "n", PlainText: tt.text,
				Blocks: []render.Block{{End: len(tt.text), Verbatim: true}}}}, validArtifactPolicy(t))
			if got := soleResult(t, idx, tt.query); got.Landing != tt.query {
				t.Errorf("Landing = %q, want %q", got.Landing, tt.query)
			}
		})
	}
}

// TestDocumentFromNoteDropsTheTitleHeadingWhereverItSits holds that the search
// corpus stops vouching for the heading the page drops as its title, whether it
// opens the body or follows other text, and keeps a heading the page keeps.
func TestDocumentFromNoteDropsTheTitleHeadingWhereverItSits(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]string{
		"opening":       "# Quorum reads\n\nbody text\n",
		"after prose":   "An opening line.\n\n# Quorum reads\n\nbody text\n",
		"after heading": "## Background\n\nan opening line\n\n# Quorum reads\n\nbody text\n",
		"setext, later": "An opening line.\n\nQuorum reads\n============\n\nbody text\n",
	} {
		doc := DocumentFromNote(&vault.Note{RelPath: "Notes/q.md", Body: body})
		start := 0
		for _, block := range doc.Blocks {
			if strings.TrimSpace(doc.PlainText[start:block.End]) == "Quorum reads" && block.Heading {
				t.Errorf("%s: the title heading is still a heading block in the corpus", name)
			}
			start = block.End
		}
	}
	kept := DocumentFromNote(&vault.Note{RelPath: "Notes/q.md", Body: "# Title\n\n## Quorum reads\n\nbody\n"})
	headings := 0
	for _, block := range kept.Blocks {
		if block.Heading {
			headings++
		}
	}
	if headings != 1 {
		t.Errorf("a section heading the page keeps: %d heading blocks, want 1", headings)
	}
}
