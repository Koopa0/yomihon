package note

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"

	"github.com/koopa0/yomihon/internal/mark"
)

// annotateMarkAnchors identifies positions the marks file can hold after all
// note-local IDs have their final spelling. Tokenization excludes markup that
// is only text in a script or comment. Copying raw tokens preserves authored
// IDs and every other byte, including attribute quoting and entity spellings.
func annotateMarkAnchors(body string) string {
	var annotated strings.Builder
	annotated.Grow(len(body))
	tokens := html.NewTokenizer(strings.NewReader(body))
	for {
		kind := tokens.Next()
		raw := tokens.Raw()
		if kind == html.StartTagToken || kind == html.SelfClosingTagToken {
			raw = bytes.Clone(raw)
			token := tokens.Token()
			if markAnchorEligible(token.Attr) {
				end := bytes.IndexAny(raw, " \t\n\r\f/>")
				annotated.Write(raw[:end])
				annotated.WriteString(" data-mark-anchor")
				annotated.Write(raw[end:])
				continue
			}
		}
		annotated.Write(raw)
		if kind == html.ErrorToken {
			return annotated.String()
		}
	}
}

// markAnchorEligible follows the browser's first-ID rule for duplicate
// attributes. The decoded value is validated; its authored spelling is kept.
func markAnchorEligible(attrs []html.Attribute) bool {
	var id string
	var hasID bool
	for _, attr := range attrs {
		switch attr.Key {
		case "data-mark-anchor":
			return false
		case "id":
			if !hasID {
				id, hasID = attr.Val, true
			}
		}
	}
	return id != "" && mark.ValidateAnchor(id) == nil
}
