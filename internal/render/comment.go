package render

import (
	"fmt"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
)

// strippedBody keeps the one strip's text beside authored line geometry.
type strippedBody struct {
	text    string
	address []string
}

func stripBody(body string) (stripped strippedBody, unclosed unclosedComment) {
	text, unclosed := stripObsidianComments(body)
	return strippedBody{text: text, address: BlockAddressLines(strings.Split(body, "\n"), text)}, unclosed
}

type unclosedComment struct {
	line   int
	marker string
}

// stripObsidianComments delegates the single graph-owned strip protocol.
func stripObsidianComments(body string) (stripped string, unclosed unclosedComment) {
	facts := graph.ReadBody(body)
	comment := facts.UnclosedComment()
	return facts.CommentFree(), unclosedComment{line: comment.Line, marker: comment.Marker}
}

// htmlCommentCode is protection for a bounded raw-markup presentation fragment.
// It uses canonical authored recognition, not a separately configured parser.
func htmlCommentCode(body string) []graph.Span {
	if !strings.Contains(body, "<!--") {
		return nil
	}
	var zones []graph.Span
	for code := range graph.CodeProtection(body) {
		zones = append(zones, code.Span)
	}
	return zones
}

func unclosedCommentDiagnostic(unclosed unclosedComment) Diagnostic {
	return Diagnostic{
		Kind:    DiagCommentUnclosed,
		Target:  unclosed.marker,
		Message: fmt.Sprintf("an unclosed %s comment opened at line %d of the note body hides everything after it", unclosed.marker, unclosed.line),
	}
}

func appendUnclosedComment(diagnostics []Diagnostic, unclosed unclosedComment) []Diagnostic {
	if unclosed.line == 0 {
		return diagnostics
	}
	return append(diagnostics, unclosedCommentDiagnostic(unclosed))
}
