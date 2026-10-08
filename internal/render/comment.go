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

// stripBody hides body's comments and retains original diagnostic coordinates.
func stripBody(body string) (stripped strippedBody, report commentReport) {
	text, report := stripObsidianComments(body)
	return strippedBody{text: text, address: BlockAddressLines(strings.Split(body, "\n"), text)}, report
}

// commentReport distinguishes container silence from a body-wide remainder.
type commentReport struct {
	bodywide   unclosedComment
	containers []unclosedComment
}

type unclosedComment struct {
	line   int
	marker string
}

// stripObsidianComments delegates the single graph-owned strip protocol.
func stripObsidianComments(body string) (stripped string, report commentReport) {
	facts := graph.ReadBody(body)
	comment := facts.UnclosedComment()
	report.bodywide = unclosedComment{line: comment.Line, marker: comment.Marker}
	for container := range facts.ContainerUnclosedComments() {
		report.containers = append(report.containers, unclosedComment{line: container.Line, marker: container.Marker})
	}
	return facts.CommentFree(), report
}

// htmlCommentCode protects a bounded raw-markup presentation fragment using
// canonical authored recognition, without a separately configured parser.
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

// commentDiagnostics publishes source-order container records before the
// body-wide kind, which alone explains an empty page.
func commentDiagnostics(report commentReport) []Diagnostic {
	var diagnostics []Diagnostic
	for _, container := range report.containers {
		diagnostics = append(diagnostics, Diagnostic{
			Kind:    DiagCommentContainerUnclosed,
			Target:  container.marker,
			Message: fmt.Sprintf("an unclosed %s comment opened at line %d of the note body hides the rest of its Markdown container", container.marker, container.line),
		})
	}
	if report.bodywide.line != 0 {
		diagnostics = append(diagnostics, unclosedCommentDiagnostic(report.bodywide))
	}
	return diagnostics
}
