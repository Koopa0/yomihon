package render

import (
	"testing"

	"github.com/yuin/goldmark/util"
)

// TestAttributeURLRoundTrip holds the two directions to being each other's
// inverse. A URL that comes back from the trip changed is a file the reader
// asks for by a name nothing on disk has.
func TestAttributeURLRoundTrip(t *testing.T) {
	t.Parallel()

	for _, u := range []string{
		"/raw/Notes/a&b.png",
		"/raw/Notes/a&amp;b.png",
		"/raw/Notes/a&copy.png",
		"/raw/Notes/a%3Bb.png",
		`/raw/Notes/quote".png`,
		"/raw/Notes/angle<>.png",
		"/notes/Notes/Q&A.md",
		"/raw/Notes/plain.png",
	} {
		if got := attributeUnescaper.Replace(attributeEscaper.Replace(u)); got != u {
			t.Errorf("round trip of %q came back as %q", u, got)
		}
	}
}

// TestAttributeEscaperAgreesWithTheMarkupEscaper keeps this package to one
// spelling of an attribute value. The asset pass reads a source the markup
// renderer wrote, so a value escaped here by different rules is one that pass
// would read back as a different URL. The apostrophe is the case that separates
// the two candidate escapers: a URL may carry it, and an attribute in double
// quotes does not need it touched.
func TestAttributeEscaperAgreesWithTheMarkupEscaper(t *testing.T) {
	t.Parallel()

	for _, u := range []string{
		"a&b.png",
		"a&copy.png",
		"Q&A.md",
		"it's.png",
		`quote".png`,
		"angle<>.png",
		"plain.png",
	} {
		got := attributeEscaper.Replace(u)
		want := string(util.EscapeHTML([]byte(u)))
		if got != want {
			t.Errorf("escaping %q gives %q, the markup renderer writes %q", u, got, want)
		}
	}
}
