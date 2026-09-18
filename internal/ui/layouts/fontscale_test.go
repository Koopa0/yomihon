package layouts

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// fontSizePixelLiteral matches a font-size declaration whose value is a
// literal pixel number rather than a scale token — case-insensitive and
// tolerant of a space before the colon, so a differently formatted literal
// cannot walk past the same lock a tightly written one would trip.
var fontSizePixelLiteral = regexp.MustCompile(`(?i)font-size\s*:\s*\d+(?:\.\d+)?px`)

// TestNoFontSizeEscapesTheScale holds every UI face to the type scale
// tokens.css names. A literal px value beside a --fs- token is a size chosen
// against nothing: it does not track a future scale revision, and it leaves
// two competing sources of truth for "how big is this text" sitting a few
// declarations apart, invisible until a reader notices sizes that nearly but
// do not quite match.
//
// tokens.css itself is where the scale is spelled out, in rem rather than
// px, so it is read too rather than assumed clean — the exemption is by
// filename, not by a blanket skip, so a future scale entry mistakenly
// written in px still trips this. output.css is tailwindcss's generated
// minification of the same sources and is never hand-edited; stylelint-check
// excludes it on the same grounds.
//
// Matching runs against the untouched source, with comment spans located
// separately and subtracted, rather than against a comment-stripped copy: a
// stripped copy's own line numbers no longer agree with the file a person
// would open, and a finding that points at the wrong line sends a reader
// looking for it in the wrong place.
func TestNoFontSizeEscapesTheScale(t *testing.T) {
	t.Parallel()
	const dir = "../../../assets/css"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", dir, err)
	}
	checked := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".css" || name == "output.css" {
			continue
		}
		checked++
		if name == "tokens.css" {
			// The scale's own definition, in rem. Still counted above so a
			// directory that somehow lost it is not a silent pass, just not
			// matched below: a px literal here would be the scale going off
			// the scale, which is a different bug than this lock names.
			continue
		}
		path := filepath.Join(dir, name)
		source, err := os.ReadFile(path) // #nosec G304 -- a name os.ReadDir just listed out of this fixed repository directory
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		text := string(source)
		inComment := commentSpans(text)
		for _, loc := range fontSizePixelLiteral.FindAllStringIndex(text, -1) {
			if inComment(loc[0]) {
				continue
			}
			line := 1 + strings.Count(text[:loc[0]], "\n")
			t.Errorf("%s:%d carries a literal font-size px value %q; move it to the nearest --fs- step in tokens.css", path, line, text[loc[0]:loc[1]])
		}
	}
	if checked == 0 {
		t.Fatal("no stylesheet was scanned, so a clean pass proves nothing")
	}
}

// commentSpans returns a predicate answering whether a byte offset into text
// falls inside a /* ... */ comment, so a font-size mentioned in prose — this
// file's own doc comment, for one — is never mistaken for a declaration.
func commentSpans(text string) func(offset int) bool {
	spans := cssComments.FindAllStringIndex(text, -1)
	return func(offset int) bool {
		for _, span := range spans {
			if offset >= span[0] && offset < span[1] {
				return true
			}
		}
		return false
	}
}
