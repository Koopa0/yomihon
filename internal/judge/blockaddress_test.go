package judge

import (
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

type blockAddressFinding struct {
	Rule     RuleID
	Path     string
	Target   string
	Line     int
	Resolved string
}

// checkBlockAddressFindings exercises the public scan and retains every link
// and embed diagnostic, so a failed resolver cannot masquerade as a good block.
func checkBlockAddressFindings(t *testing.T, files map[string]string) []blockAddressFinding {
	t.Helper()
	root := t.TempDir()
	for path, body := range files {
		write(t, root, path, body)
	}
	writeTestContract(t, root, nil)
	findings, err := runCheckAction(t.Context(), root, nil, false)
	if err != nil {
		t.Fatalf("Check(block address vault) error = %v", err)
	}
	var got []blockAddressFinding
	for i := range findings {
		f := &findings[i]
		if !strings.HasPrefix(string(f.RuleID), "link.") && !strings.HasPrefix(string(f.RuleID), "embed.") {
			continue
		}
		if f.Target == nil || f.Line == nil {
			t.Fatalf("Check() link/embed finding lacks target or line: %+v", *f)
		}
		resolved := ""
		if f.ResolvedTo != nil {
			resolved = *f.ResolvedTo
		}
		got = append(got, blockAddressFinding{Rule: f.RuleID, Path: f.Path, Target: *f.Target, Line: *f.Line, Resolved: resolved})
	}
	return got
}

// TestBlockAddressesReachTheCheck holds the check's side of a block address
// through a whole run: a written address answers both a link and an embed, and
// an address the note does not carry is missing on both.
func TestBlockAddressesReachTheCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		citer string
		want  []blockAddressFinding
	}{
		{name: "a written address", citer: "[[Target#^abc-1]]\n![[Target#^abc-1]]\n"},
		{
			name: "an address the note does not carry", citer: "[[Target#^abc1]]\n![[Target#^abc1]]\n",
			want: []blockAddressFinding{
				{Rule: "link.block_missing", Path: "Notes/Citer.md", Target: "Target#^abc1", Line: 1, Resolved: "Notes/Target.md"},
				{Rule: "embed.block_missing", Path: "Notes/Citer.md", Target: "Target#^abc1", Line: 2, Resolved: "Notes/Target.md"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := checkBlockAddressFindings(t, map[string]string{
				"Notes/Target.md": "Para ^abc-1\n",
				"Notes/Citer.md":  tt.citer,
			})
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Check() block address findings (-want +got):\n%s", diff)
			}
		})
	}
}

// A block address is read by three faces — the page that stamps its anchor, the
// excerpt that cuts to it, and this one, which reports it missing — and each
// builds its lines with a different comment strip. The page blanks a comment
// where it stands; this face cuts the bytes out and reads code the way goldmark
// does. The bodies below are the ones where a strip can change the line
// geometry the code-span question reads, so the three must still answer alike.

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
		// Cutting the comment's bytes out must keep its line endings, or the
		// words on either side join and the closing backtick lands in the
		// address's own run.
		name:      "a comment written across a blank line",
		body:      "A lone ` in a sentence opens nothing.\nThis paragraph is the one a link addresses ^probe\nand the sentence runs on. %%Check this claim against\n\nthe source before publishing.%% It appears in `fmt` too.\n",
		addressed: true,
	},
	{
		// The mirror of it: a join would put two backticks side by side, and a
		// run of two closes on a run of two, hiding the span the page draws.
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
		// drops any the vault wrote, which can empty a line the same way.
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
		// page shows the opener as written and stamps the address at its end.
		// A known type's opener is refused an address only where it is a
		// callout, whose opening line is consumed as the title.
		name:      "a callout opener an indented code block shows as written",
		body:      "para\n\n    > [!success] X ^probe\n",
		addressed: true,
	},
	{
		name:      "a callout opener an indented code block shows as written, older type",
		body:      "para\n\n    > [!note] X ^probe\n",
		addressed: true,
	},
	{
		name:      "a table row an indented code block shows as written",
		body:      "para\n\n    | a | b ^probe\n",
		addressed: true,
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
		// A heading whose whole text is the token has nothing to address: the
		// token is its words, on every face.
		name:      "a heading of only the token",
		body:      "para\n\n## ^probe\n",
		addressed: false,
	},
	{
		name:      "a heading with words before the token",
		body:      "para\n\n## Words ^probe\n",
		addressed: true,
	},
	{
		// On a line of its own the token addresses the block above it.
		name:      "a paragraph of only the token",
		body:      "para\n\n^probe\n",
		addressed: true,
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
			if got := slices.Contains(blockAddresses, "^probe"); got != tt.addressed {
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
// with one per line is, is a single run for the code-span question, and the
// check reads it once. Four times the lines make about four times the
// allocations when the run is read once, and about sixteen times as many when
// each address reads it again, so a ratio of six separates them without a
// clock.
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
	return render.New(graph.BuildFromNotes(nil, nil), fragmentNotes(nil), fragmentNotes(nil), fragmentNotes(nil))
}
