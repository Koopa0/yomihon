package render

import "testing"

// TestAHeadingInsideASelfClosingRawTextBlockIsNotASection holds the half of the
// HTML-block table this package's other reader depends on. A self-closing
// <pre/> or <script/> opens a raw-text block that no blank line ends, so the
// lines after it are shown to a reader as the characters they typed. A section
// list naming a heading from inside one offers a place on the page that is not
// there: following it lands the reader somewhere else, and nothing tells them
// why.
//
// This scan and the callout scan read the same table for that reason. When they
// each kept their own, this one recognised no block at all for these two
// openers — neither element is in its list of the tags a blank line ends — and
// counted the heading.
func TestAHeadingInsideASelfClosingRawTextBlockIsNotASection(t *testing.T) {
	t.Parallel()

	for _, opener := range []string{"<pre/>", "<script/>"} {
		t.Run(opener, func(t *testing.T) {
			t.Parallel()

			got := scanHeadings([]string{opener, "", "# Heading after", "", "Prose after that."})
			if len(got) != 0 {
				t.Errorf("scanHeadings found %d headings after %s, which opens a block the page never closes: %+v", len(got), opener, got)
			}
		})
	}
}
