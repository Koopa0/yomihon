package render

import (
	"net/url"
	"testing"

	"github.com/yuin/goldmark/util"
)

// FuzzAttributeURLRoundTrip holds the two directions to being each other's
// inverse for any string at all, not only for the shapes a vault happens to
// hold today. A URL that comes back from the trip changed is a file the reader
// asks for by a name nothing on disk has.
func FuzzAttributeURLRoundTrip(f *testing.F) {
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
		f.Add(u)
	}
	f.Fuzz(func(t *testing.T, u string) {
		if got := attributeUnescaper.Replace(attributeEscaper.Replace(u)); got != u {
			t.Errorf("round trip of %q came back as %q", u, got)
		}
	})
}

// TestAttributeEscaperAgreesOnTheBytesAURLCanCarry keeps this package to one
// spelling of an attribute value. The asset pass reads a source the markup
// renderer wrote, so a value escaped here by different rules is one that pass
// would read back as a different URL. The apostrophe is the case that separates
// the two candidate escapers: a URL may carry it, and an attribute in double
// quotes does not need it touched.
//
// The claim is bounded to what a percent-escaped URL can carry, which is what
// reaches an attribute here. The one byte the two escapers spell differently is
// the null, which goldmark's replaces with U+FFFD; it cannot arrive, because
// both URL escapers on the way in write it as "%00" — asserted below rather
// than assumed, since the bound is what makes the agreement true.
func TestAttributeEscaperAgreesOnTheBytesAURLCanCarry(t *testing.T) {
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

	for _, escaped := range []string{
		url.PathEscape("\x00"),
		string(util.URLEscape([]byte("\x00"), true)),
	} {
		if escaped != "%00" {
			t.Errorf("a null reaches an attribute as %q, so the byte the two escapers disagree on is reachable after all", escaped)
		}
	}
}
