package judge

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

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

			_, _, blockLines := anchorSurface(tt.body)
			if got := blockAddressed(blockLines, "^probe"); got != tt.addressed {
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
