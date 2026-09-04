package render

import "strings"

// A href this package builds is a URL: percent-escaped, and free to carry the
// bytes a URL is allowed to carry, "&" among them. Standing that URL inside a
// quoted attribute is a second encoding, one that belongs to the attribute
// rather than to the URL, so it is applied once where a URL is written into an
// attribute and undone once where a value is read back out of one. The two
// replacers are each other's inverse over the four bytes an attribute value
// cannot carry literally.
//
// "&" is the byte that makes the two layers visible, because it is the one that
// is safe in a URL and unsafe in an attribute. A picture named "a&copy.png"
// written into an attribute unencoded is read by the browser as a character
// reference and fetched as "a©.png"; encoded a second time it reaches the raw
// route with "&amp;" inside the file name. Neither spelling names a file the
// vault has.
//
// That this states the same rule the markup renderer applies through goldmark's
// escaper is deliberate rather than an oversight. The asset pass reads a source
// that escaper wrote, so the two have to agree, and a test pins the agreement
// over every byte a percent-escaped URL can carry. Neither of the standard
// library's two functions is this rule: its text escaper also encodes the
// apostrophe, which a URL may carry and a double-quoted attribute does not need
// touched, and its unescaper decodes references this escaper never writes, so
// it is an inverse of something wider than what is written here.
var (
	attributeEscaper   = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	attributeUnescaper = strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`)
)
