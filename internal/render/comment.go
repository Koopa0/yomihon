package render

import (
	"fmt"
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
)

// strippedBody keeps the one strip's text beside authored line geometry.
type strippedBody struct {
	text     string
	address  []string
	roleGaps []string
}

// stripBody hides body's comments and retains original diagnostic coordinates.
func stripBody(body string) (stripped strippedBody, report commentReport) {
	facts := graph.ReadBody(body)
	text := facts.PresentationSource()
	return strippedBody{text: text, address: BlockAddressLines(strings.Split(body, "\n"), text), roleGaps: slices.Collect(facts.PresentationRoleGaps())}, bodyCommentReport(facts)
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
	return facts.CommentFree(), bodyCommentReport(facts)
}

func bodyCommentReport(facts graph.BodyFacts) (report commentReport) {
	comment := facts.UnclosedComment()
	report.bodywide = unclosedComment{line: comment.Line, marker: comment.Marker}
	for container := range facts.ContainerUnclosedComments() {
		report.containers = append(report.containers, unclosedComment{line: container.Line, marker: container.Marker})
	}
	return report
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

func (s strippedBody) sourceSlice(value string) string {
	for _, gap := range s.roleGaps {
		value = strings.ReplaceAll(value, gap, "")
	}
	return value
}

// Cuts are contiguous source lines. Carry their address geometry rather than
// asking the comment-free spelling to decide its former block roles again.
func (s strippedBody) cut(value string, line int) strippedBody {
	end := min(len(s.address), line+strings.Count(value, "\n")+1)
	s.address = slices.Clone(s.address[line:end])
	s.text = value
	return s
}
