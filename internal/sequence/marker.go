package sequence

import "strings"

// The marker declares a branch's part in the course order and nothing else.
// There is one key and three values, closed on purpose: a general directive
// syntax would grow into a second contract living in note bodies, and the vault
// already has one contract.
const (
	markerOpen  = "{sequence"
	markerClose = "}"

	valuePrimary = "primary"
	valueLocal   = "local"
	valueNone    = "none"
)

// lineSpan is a half-open byte range into one source line, counted from that
// line's own first byte. It is deliberately not [Span]: a Span is measured from
// the start of the body, and the two frames differ by wherever the line begins.
// Keeping them apart is what stops a range measured on a line from being asked
// a question only the body can answer — which zone it falls in, which row owns
// it — where it would silently name the wrong bytes.
type lineSpan struct{ Start, Stop int }

// declKind is what a line's marker turned out to be. Every kind but declValid
// declares nothing: an unreadable declaration is not a declaration, so the
// branch stays undeclared and the author is told which way it failed.
type declKind uint8

const (
	declNone declKind = iota
	declValid
	declInvalid
	declDuplicate
	declMisplaced
)

// readMarker reads a role declaration off one source line, considering only
// the marker spans the caller passes in. The caller filters out spans that sit
// inside code or an Obsidian comment: a marker there is quoted or switched
// off, not written, so it neither declares a role nor draws a report.
//
// A recognized marker is removed from the name, because a declaration is not
// part of what the branch is called; the file's own bytes are never touched.
// Anything unrecognized leaves the text exactly as written, so a reader sees
// what the author typed and can find it.
func readMarker(line string, spans []lineSpan) (name string, role Role, decl declKind) {
	switch len(spans) {
	case 0:
		return line, RoleStructural, declNone
	case 1:
	default:
		return line, RoleStructural, declDuplicate
	}
	span := spans[0]
	if strings.TrimSpace(line[span.Stop:]) != "" {
		// A marker mid-line is read by nobody: the contract places it at end of
		// line so a branch's name is everything before it.
		return line, RoleStructural, declMisplaced
	}
	role, ok := markerValue(line[span.Start:span.Stop])
	if !ok {
		return line, RoleStructural, declInvalid
	}
	return strings.TrimRight(line[:span.Start], " \t"), role, declValid
}

// markerValue reads the value out of one whole marker.
func markerValue(marker string) (Role, bool) {
	inner := strings.TrimSuffix(strings.TrimPrefix(marker, markerOpen), markerClose)
	value, found := strings.CutPrefix(inner, "=")
	if !found {
		return RoleStructural, false
	}
	switch strings.TrimSpace(value) {
	case valuePrimary:
		return RolePrimary, true
	case valueLocal:
		return RoleLocal, true
	case valueNone:
		return RoleNone, true
	default:
		return RoleStructural, false
	}
}

// markerSpans locates every "{sequence...}" on a line. A marker with no closing
// brace is not a marker at all — it is text the author wrote — so an unclosed
// opener ends the scan rather than swallowing the rest of the line.
func markerSpans(line string) []lineSpan {
	var spans []lineSpan
	for off := 0; ; {
		rel := strings.Index(line[off:], markerOpen)
		if rel < 0 {
			return spans
		}
		start := off + rel
		relEnd := strings.Index(line[start:], markerClose)
		if relEnd < 0 {
			return spans
		}
		stop := start + relEnd + len(markerClose)
		spans = append(spans, lineSpan{start, stop})
		off = stop
	}
}

// isTaskRow reports whether a row's own text opens with a task checkbox. A
// checkbox row is never a lesson in the course order: it tracks whether
// something was done, which is a different question from what the course
// teaches, and a reader walking lessons should not step into one.
//
// The marker is GFM's: a bracketed space or x, followed by whitespace or
// nothing. "[x][[A]]" is not one — no whitespace follows the bracket — and is
// left to the ordinary row grammar rather than guessed at.
func isTaskRow(own string) bool {
	t := strings.TrimLeft(own, " \t")
	if len(t) < 3 || t[0] != '[' || t[2] != ']' {
		return false
	}
	switch t[1] {
	case ' ', 'x', 'X':
	default:
		return false
	}
	rest := t[3:]
	if rest == "" {
		return true
	}
	switch rest[0] {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}
