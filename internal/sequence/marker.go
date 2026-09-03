package sequence

import "strings"

// The marker declares a branch's part in the course order and nothing else:
// one key, three values, closed on purpose.
const (
	markerOpen  = "{sequence"
	markerClose = "}"

	valuePrimary = "primary"
	valueLocal   = "local"
	valueNone    = "none"
)

// lineSpan is a half-open byte range into one source line, counted from that
// line's own first byte. It is deliberately not [Span], which is measured from
// the body's first byte: the two frames differ by wherever the line begins.
type lineSpan struct{ Start, Stop int }

// declKind is what a line's marker turned out to be. Every kind but declValid
// declares nothing, so the branch stays undeclared and the author is told why.
type declKind uint8

const (
	declNone declKind = iota
	declValid
	declInvalid
	declDuplicate
	declMisplaced
)

// readMarker reads a role declaration off one source line, considering only
// the marker spans the caller passes in — one inside code or an Obsidian
// comment is quoted, not written. A recognized marker is removed from the
// name; anything unrecognized is left exactly as the author typed it.
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
		// The marker sits at end of line, so the name is everything before it.
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

// markerSpans locates every "{sequence...}" on a line. An opener with no
// closing brace is text the author wrote, so it ends the scan.
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

// isTaskRow reports whether a row's own text opens with a GFM task checkbox: a
// bracketed space or x followed by whitespace or nothing. A checkbox row tracks
// whether something was done and is never a lesson in the course order.
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
