package judge

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestSupportedBlockAddressesReachTheJudge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		body  string
		citer string
		want  []blockAddressFinding
	}{
		{name: "paragraph", body: "Para ^abc-1\n", citer: "[[Target#^abc-1]]\n![[Target#^abc-1]]\n"},
		{name: "marker only", body: "^abc-1\n", citer: "[[Target#^abc-1]]\n![[Target#^abc-1]]\n"},
		{name: "tab boundary", body: "Para\t^abc-1\n", citer: "[[Target#^abc-1]]\n![[Target#^abc-1]]\n"},
		{name: "trailing spaces and tabs", body: "Para ^abc-1 \t\n", citer: "[[Target#^abc-1]]\n![[Target#^abc-1]]\n"},
		{name: "ASCII case", body: "Para ^AbC-1\n", citer: "[[Target#^abc-1]]\n![[Target#^abc-1]]\n"},
		{
			name: "hyphen is significant", body: "Para ^abc-1\n", citer: "[[Target#^abc1]]\n![[Target#^abc1]]\n",
			want: []blockAddressFinding{
				{Rule: "link.block_missing", Path: "Notes/Citer.md", Target: "Target#^abc1", Line: 1, Resolved: "Notes/Target.md"},
				{Rule: "embed.block_missing", Path: "Notes/Citer.md", Target: "Target#^abc1", Line: 2, Resolved: "Notes/Target.md"},
			},
		},
		{
			name: "underscore", body: "Para ^a_b\n", citer: "[[Target#^a_b]]\n![[Target#^a_b]]\n",
			want: []blockAddressFinding{
				{Rule: "link.block_missing", Path: "Notes/Citer.md", Target: "Target#^a_b", Line: 1, Resolved: "Notes/Target.md"},
				{Rule: "embed.block_missing", Path: "Notes/Citer.md", Target: "Target#^a_b", Line: 2, Resolved: "Notes/Target.md"},
			},
		},
		{
			name: "dot", body: "Para ^a.b\n", citer: "[[Target#^a.b]]\n![[Target#^a.b]]\n",
			want: []blockAddressFinding{
				{Rule: "link.block_missing", Path: "Notes/Citer.md", Target: "Target#^a.b", Line: 1, Resolved: "Notes/Target.md"},
				{Rule: "embed.block_missing", Path: "Notes/Citer.md", Target: "Target#^a.b", Line: 2, Resolved: "Notes/Target.md"},
			},
		},
		{
			name: "Unicode", body: "Para ^\u304c\n", citer: "[[Target#^\u304c]]\n![[Target#^\u304c]]\n",
			want: []blockAddressFinding{
				{Rule: "link.block_missing", Path: "Notes/Citer.md", Target: "Target#^\u304c", Line: 1, Resolved: "Notes/Target.md"},
				{Rule: "embed.block_missing", Path: "Notes/Citer.md", Target: "Target#^\u304c", Line: 2, Resolved: "Notes/Target.md"},
			},
		},
		{
			name: "punctuation", body: "Para ^abc!\n", citer: "[[Target#^abc!]]\n![[Target#^abc!]]\n",
			want: []blockAddressFinding{
				{Rule: "link.block_missing", Path: "Notes/Citer.md", Target: "Target#^abc!", Line: 1, Resolved: "Notes/Target.md"},
				{Rule: "embed.block_missing", Path: "Notes/Citer.md", Target: "Target#^abc!", Line: 2, Resolved: "Notes/Target.md"},
			},
		},
		{
			name: "glued caret", body: "Para^abc-1\n", citer: "[[Target#^abc-1]]\n![[Target#^abc-1]]\n",
			want: []blockAddressFinding{
				{Rule: "link.block_missing", Path: "Notes/Citer.md", Target: "Target#^abc-1", Line: 1, Resolved: "Notes/Target.md"},
				{Rule: "embed.block_missing", Path: "Notes/Citer.md", Target: "Target#^abc-1", Line: 2, Resolved: "Notes/Target.md"},
			},
		},
		{
			name: "empty token", body: "Para ^\n", citer: "[[Target#^abc-1]]\n![[Target#^abc-1]]\n",
			want: []blockAddressFinding{
				{Rule: "link.block_missing", Path: "Notes/Citer.md", Target: "Target#^abc-1", Line: 1, Resolved: "Notes/Target.md"},
				{Rule: "embed.block_missing", Path: "Notes/Citer.md", Target: "Target#^abc-1", Line: 2, Resolved: "Notes/Target.md"},
			},
		},
		{
			name: "inline footnote", body: "Para ^[note]\n", citer: "[[Target#^[note]|shown]]\n![[Target#^[note]|shown]]\n",
			want: []blockAddressFinding{
				{Rule: "link.block_missing", Path: "Notes/Citer.md", Target: "Target#^[note]", Line: 1, Resolved: "Notes/Target.md"},
				{Rule: "embed.block_missing", Path: "Notes/Citer.md", Target: "Target#^[note]", Line: 2, Resolved: "Notes/Target.md"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			t.Logf("hit: judge block-address case %s", tt.name)
			got := checkBlockAddressFindings(t, map[string]string{
				"Notes/Target.md": tt.body,
				"Notes/Citer.md":  tt.citer,
			})
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("caught: unsupported block address accepted by judge or supported address rejected: Check() (-want +got):\n%s", diff)
			}
		})
	}
}

// A block address is read by three faces — the page that stamps its anchor, the
// excerpt that cuts to it, and this one, which reports it missing — and each
// builds its lines with a different comment strip. The page blanks a comment
// where it stands; this face cuts the bytes out and reads code the way goldmark
// does. Where those two hide the same text the three have to answer alike, and
// the bodies below are the ones where they did not: a strip that empties a line
// its author filled, or takes the line endings out of a comment written over
// several lines, hands the code-span question a run edge nobody typed, and each
// face gets a different one.

// blockAddressBodies are bodies whose answer about "^probe" turns on what a
// strip did to the line geometry around it.
var blockAddressBodies = []struct {
	name string
	// title is the page's, which decides whether a leading H1 is dropped there.
	title string
	body  string
	// addressed is the one answer all three faces have to give: whether a
	// reader can be sent to ^probe.
	addressed bool
}{
	{
		// Cutting the comment's bytes out used to take its line endings with
		// them, joining the words on either side; the closing backtick then
		// landed in the address's own run and a span appeared to hold the
		// caret. The page, which blanks a comment where it stands, kept the
		// blank line and the address, and reported nothing wrong.
		name:      "a comment written across a blank line",
		body:      "A lone ` in a sentence opens nothing.\nThis paragraph is the one a link addresses ^probe\nand the sentence runs on. %%Check this claim against\n\nthe source before publishing.%% It appears in `fmt` too.\n",
		addressed: true,
	},
	{
		// The mirror of it. Here the join put two backticks side by side, and
		// a run of two closes on a run of two, so the span the page draws
		// disappeared and this face alone thought the address was fine.
		name:      "a comment holding a single backtick",
		body:      "A lone ` in a sentence opens nothing.\nThis paragraph is the one a link addresses ^probe\nand the pair `%%` is how a comment marker is shown.\n\n%%` needs a section of its own%%\n",
		addressed: false,
	},
	{
		// A line whose every visible byte was inside a comment is empty after
		// the strip, and a blank line is the one thing that ends the run the
		// span is looked for over. The author wrote no blank line here, so the
		// two stray backticks are one span and the caret is inside it.
		name:      "whole-line comments on both sides of the address",
		body:      "Before `\n%%an aside%%\na paragraph ^probe\n%%another aside%%\nAfter `\n",
		addressed: false,
	},
	{
		// The renderer reserves four private-use runes for its own markers and
		// drops any the vault wrote, which can empty a line the same way. Only
		// the page does that, so this is the strip whose run edge appeared at
		// one door and nowhere else.
		name:      "placeholder runes on both sides of the address",
		body:      "Before `\n\ue000\na paragraph ^probe\n\ue001\nAfter `\n",
		addressed: false,
	},
	{
		// The page drops a leading H1 that repeats its title, so its lines are
		// numbered one lower than the other two faces'. A heading ends a run
		// wherever it stands, so the run holding the caret is the same text at
		// all three doors with or without it.
		name:      "a leading H1 repeating the title, above a span the author wrapped",
		title:     "Target",
		body:      "# Target\nThe XOR is `left ^probe\nright` ends it\n",
		addressed: false,
	},
	{
		// Four spaces after a blank line make an indented code block, so the
		// page shows the opener as written and keeps its caret as code.
		// Neither quoted code nor a consumed callout title carries an address.
		name:      "a callout opener an indented code block shows as written",
		body:      "para\n\n    > [!success] X ^probe\n",
		addressed: false,
	},
	{
		name:      "a callout opener an indented code block shows as written, older type",
		body:      "para\n\n    > [!note] X ^probe\n",
		addressed: false,
	},
	{
		name:      "a table row an indented code block shows as written",
		body:      "para\n\n    | a | b ^probe\n",
		addressed: false,
	},
	{
		// The same opener at the top level is a callout, whose opening line
		// is its title and carries no address on any face.
		name:      "a callout opener outside code",
		body:      "para\n\n> [!success] X ^probe\n",
		addressed: false,
	},
	{
		// Four spaces deep in a nested list item is that item's content
		// column, not code, so the opener there is a callout too.
		name:      "a callout opener nested in a list item",
		body:      "- a\n  - b\n    > [!success] X ^probe\n",
		addressed: false,
	},
	{
		name:      "a leading H1 repeating the title, above an ordinary address",
		title:     "Target",
		body:      "# Target\n\nan ordinary paragraph ^probe\n",
		addressed: true,
	},
}

func TestTheThreeBlockAddressFacesAgreeOverAStrippedBody(t *testing.T) {
	t.Parallel()

	for _, tt := range blockAddressBodies {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, blockAddresses := anchorSurface(tt.body)
			if got := blockAddressed(blockAddresses, "^probe"); got != tt.addressed {
				t.Errorf("the check finds the address = %v, want %v", got, tt.addressed)
			}
			if _, got := render.Excerpt(tt.body, "^probe"); got != tt.addressed {
				t.Errorf("the excerpt cuts to the address = %v, want %v", got, tt.addressed)
			}
			page := blockAddressPipeline().HTML("Notes/Target.md", tt.title, tt.body, wording.ZhHant)
			if got := strings.Contains(page.HTML, `<span id="^probe">`); got != tt.addressed {
				t.Errorf("the page stamps the anchor = %v, want %v:\n%s", got, tt.addressed, page.HTML)
			}
		})
	}
}

// A paragraph whose every line ends in an address, which is what a transcript
// with one per line is, is a single run for the code-span question. The check
// reads it once for the body; read once per line it cost the square of the
// lines, six seconds for 2,000 of them. Four times the lines make about four
// times the allocations when the run is read once, and about sixteen times as
// many when each address reads it again, so a ratio of six separates them
// without a clock.
func TestBlockAddressCostGrowsLinearlyWithTheLinesOfARun(t *testing.T) {
	// Not parallel: testing.AllocsPerRun pins GOMAXPROCS for its measurement
	// and must not run alongside other parallel tests.
	const lines = 200
	small := strings.Repeat("これは字幕の行です ^t\n", lines)
	large := strings.Repeat("これは字幕の行です ^t\n", 4*lines)

	if _, _, kept := anchorSurface(small); len(kept) != lines {
		t.Fatalf("the check kept %d address lines of %d, so the cost below is of a different pass", len(kept), lines)
	}
	smallAllocs := testing.AllocsPerRun(1, func() { anchorSurface(small) })
	largeAllocs := testing.AllocsPerRun(1, func() { anchorSurface(large) })
	if growth := largeAllocs / smallAllocs; growth > 6 {
		t.Errorf("%d addressed lines made %.0f allocations and %d made %.0f, %.1fx for 4x the lines; want at most 6x",
			lines, smallAllocs, 4*lines, largeAllocs, growth)
	}
}

// blockAddressPipeline renders a body with nothing else in the vault: these
// notes cite nothing, so what the page has to answer is its own addresses.
func blockAddressPipeline() *render.Pipeline {
	return render.New(graph.BuildFromNotes(nil, nil), noBodies{}, noTitles{}, everyFileHeld{})
}

type noBodies struct{}

func (noBodies) Transclusion(string) (string, bool) { return "", false }

type noTitles struct{}

func (noTitles) TitledBy(string) []string { return nil }

type everyFileHeld struct{}

func (everyFileHeld) MissingFile(string) bool { return false }
