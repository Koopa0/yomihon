package render

import (
	"fmt"
	"strings"
)

// stripObsidianComments removes Obsidian %% comment regions while preserving the
// delimiters and contents of fenced code blocks. An unclosed comment runs to the
// end of the body, as Obsidian hides it too. unclosedLine is the 1-based line the
// still-open marker was written on, or 0 when every marker found its pair, so a
// page that went quiet can name where the silence starts.
func stripObsidianComments(body string) (stripped string, unclosedLine int) {
	lines := strings.Split(body, "\n")
	inComment := false
	inFence := false
	var fenceByte byte
	pending := 0

	for i, line := range lines {
		if inFence {
			if fenceCloses(line, fenceByte) {
				inFence = false
			}
			continue
		}
		if marker, _, ok := fenceOpen(line); ok {
			inFence = true
			fenceByte = marker
			continue
		}

		var openedHere bool
		lines[i], openedHere = stripObsidianCommentLine(line, &inComment)
		switch {
		case !inComment:
			pending = 0
		case openedHere:
			pending = i + 1
		}
		if inComment {
			continue
		}
		if marker, _, ok := fenceOpen(lines[i]); ok {
			inFence = true
			fenceByte = marker
		}
	}
	return strings.Join(lines, "\n"), pending
}

// stripObsidianCommentLine removes the hidden parts of one line and reports
// whether a comment left open when it returns was opened here; one carried in from
// an earlier line keeps that line's number. Inside a code span the percent signs
// are displayed text, and spans read as the CommonMark spec reads them: a run of N
// backticks closes on the next run of exactly N, and an unpaired run is ordinary
// text. A multi-line span is not recognized, and an open comment hides backticks.
func stripObsidianCommentLine(line string, inComment *bool) (visible string, openedHere bool) {
	var b strings.Builder
	for line != "" {
		if *inComment {
			_, after, found := strings.Cut(line, "%%")
			if !found {
				return b.String(), openedHere
			}
			*inComment = false
			openedHere = false
			line = after
			continue
		}
		tick := strings.IndexByte(line, '`')
		mark := strings.Index(line, "%%")
		switch {
		case mark >= 0 && (tick < 0 || mark < tick):
			b.WriteString(line[:mark])
			*inComment = true
			openedHere = true
			line = line[mark+2:]
		case tick >= 0:
			end, _ := codeSpanAt(line, tick)
			b.WriteString(line[:end])
			line = line[end:]
		default:
			b.WriteString(line)
			line = ""
		}
	}
	return b.String(), openedHere
}

// unclosedCommentDiagnostic describes a marker that never met its pair, carrying
// the line number beside the marker itself, which is what a reader needs to find
// where their words stopped appearing. Each body is scanned once, where it is
// first read, so no second pass can reopen what the first ruled literal.
func unclosedCommentDiagnostic(line int) Diagnostic {
	return Diagnostic{
		Kind:    DiagCommentUnclosed,
		Target:  "%%",
		Message: fmt.Sprintf("an unclosed %%%% comment opened at line %d of the note body hides everything after it", line),
	}
}

// appendUnclosedComment adds that report when a marker was left open, and adds
// nothing for a body whose markers all matched — which is nearly every body.
func appendUnclosedComment(diagnostics []Diagnostic, line int) []Diagnostic {
	if line == 0 {
		return diagnostics
	}
	return append(diagnostics, unclosedCommentDiagnostic(line))
}
