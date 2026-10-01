package origin

import (
	"path"
	"strings"
	"unicode"
)

// LocalNext validates the address a form asks to be sent back to after the
// server answers it. The field is client-controlled bytes, so only a same-site
// absolute path survives: anything else — an empty value, a full URL, a
// protocol-relative or backslashed address a browser would read as one — falls
// back to Home rather than carrying the reader somewhere the form never stood.
//
// What is checked is the address the response will carry, not the bytes the
// form sent. http.Redirect cleans the path it is given, and cleaning can turn a
// harmless-looking path into one a browser reads as another host: "/./\host"
// passes a check of its first two bytes and leaves as "/\host", which a browser
// reads as "//host". So the answer is cleaned here, the way http.Redirect would
// clean it, and the cleaned address is the one judged and returned. Cleaning a
// clean path again changes nothing, so the redirect sends exactly what passed.
func LocalNext(next string) string {
	// A rune that can end a line or a control sequence is refused before any
	// shape check. A C0 control survives into the header the redirect writes,
	// and a parser on the receiving side may drop it before it reads the shape
	// — so "/\t/host" would leave here as a same-site path and arrive as a
	// protocol-relative address. Delete is refused with them. C1 — where U+0085
	// lives — and the two Unicode separators that end a line without being a
	// newline are refused as well, though the redirect percent-escapes every
	// non-ASCII byte and so never writes one raw: the language route refused
	// them before the two checks were one, both routes do now, and an answer
	// that holds none can be written anywhere else without a second thought.
	// Invalid UTF-8 decodes to the replacement character, which is in none of
	// these sets and needs no case of its own. No address a page's own form
	// carries contains any of these, so the fallback refuses no honest request.
	if strings.ContainsFunc(next, EndsALine) {
		return "/"
	}
	if next == "" || next[0] != '/' {
		return "/"
	}
	cleaned := cleanLikeRedirect(next)
	if !sameSitePath(next) || !sameSitePath(cleaned) {
		return "/"
	}
	return cleaned
}

// EndsALine reports whether r can end a line or a control sequence: a C0 or C1
// control, DELETE, or one of the two Unicode separators that end a line without
// being a newline. It is the one definition of that set; LocalNext refuses an
// address that holds such a rune, and the mark route refuses a note path that
// does.
func EndsALine(r rune) bool {
	return unicode.IsControl(r) || r == 0x2028 || r == 0x2029
}

// cleanLikeRedirect cleans an absolute path the way http.Redirect does before
// it writes the Location header: the part before the first "?" is cleaned, a
// trailing slash is kept, and the query rides along untouched.
func cleanLikeRedirect(next string) string {
	p, query, hasQuery := strings.Cut(next, "?")
	cleaned := path.Clean(p)
	if strings.HasSuffix(p, "/") && !strings.HasSuffix(cleaned, "/") {
		cleaned += "/"
	}
	if hasQuery {
		cleaned += "?" + query
	}
	return cleaned
}

// sameSitePath reports whether an absolute path stays on this site when a
// browser reads it. A browser treats a backslash as a slash, so a path opening
// with either pair names another host.
func sameSitePath(p string) bool {
	return !strings.HasPrefix(p, "//") && !strings.HasPrefix(p, `/\`)
}
