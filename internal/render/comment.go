package render

import (
	"fmt"
	"strings"
)

// stripObsidianComments removes Obsidian %% comment regions while preserving
// the delimiters and contents of fenced code blocks. An unclosed comment runs
// to the end of the body, matching Obsidian's hidden-source behavior.
//
// unclosedLine is the 1-based line the still-open marker was written on, or 0
// when every marker found its pair. Hiding the rest of a note is what Obsidian
// does too, so the text is not brought back; what the line number buys is the
// difference between a page that went quiet for no stated reason and one that
// can name the marker the silence starts at.
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

// stripObsidianCommentLine removes the hidden parts of one line, and reports
// whether a comment left open when it returns was opened on this line. A
// comment carried in from an earlier line keeps that earlier line's number, so
// what the caller ends up holding is where the runaway marker was written.
//
// Inside a code span the two percent signs are text the author is displaying
// rather than a marker, so a span that opens and closes on this line passes
// through whole and shifts nothing. Spans are read the way the CommonMark spec
// reads them: a run of N backticks is closed by the next run of exactly N, so
// a one-backtick span holds together, and a two-backtick span reads a lone
// backtick inside itself as content rather than as its own end. A run that
// never meets its match is ordinary text and the scan carries on through it —
// refusing to read past a stray backtick would hide the rest of a line on the
// strength of a typo.
//
// Two limits are deliberate, and pinned by their own tests. A span the author
// spread across several lines is not recognized, because this scan reads one
// line at a time. And while a comment is open its content is hidden rather
// than read, so a backtick inside it protects nothing: the marker that closes
// the comment is looked for first, and spans start counting again only after
// it is found.
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

// unclosedCommentDiagnostic describes a marker that never met its pair. The
// line number is carried in the technical detail, beside the marker itself,
// because that is the one fact a reader needs in order to find the place their
// words stopped appearing.
//
// Each body is scanned once, where it is first read, and reports there: a note
// on the page that renders it, a transcluded excerpt on the page that cites
// it. Scanning the same text twice would let one pass reopen what the other
// already ruled to be literal text, and the second reading would hide words
// the first had kept.
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
