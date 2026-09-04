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

// HeadingName is what a heading is called once the role it declares comes off:
// the words before the marker when the line declares a role, and the line
// exactly as it was written when it does not. A branch is a heading from level
// 2 to 6, so a marker on a level-1 heading declares nothing and stays part of
// the name — the author is told about that one, and a report quoting words the
// page does not show cannot be acted on.
//
// A declaration that is the whole heading stays as well: with it off the
// heading has no words at all, so a reader would meet a blank row in the
// contents and could address the section only by the name every nameless
// heading shares. The course lists such a branch unnamed, having no name to
// list, while the page shows the line the author typed.
//
// The line may be a heading's markdown source or the inner markup its rendered
// form carries, because a role is read only as the last thing on its line: a
// marker the author quoted in code keeps a closing backtick or a closing tag
// after it in either form, so it stays the text about the grammar that it is.
// Reading a name is all this does; whether the heading stands where a course
// could open a branch is the caller's question.
func HeadingName(line string, level int) string {
	if level < 2 {
		return line
	}
	name, _, _ := readMarker(line, markerSpans(line))
	if strings.TrimSpace(name) == "" {
		return line
	}
	return name
}

// Marker is the declaration an author writes to give a branch a role, spelled
// from the same three values a declaration is read with, so a page teaching
// the grammar cannot drift from the parser that accepts it. Only a role a line
// can declare has a marker: the two a branch is left with when it declared
// nothing return the empty string, since no line can say them.
func Marker(role Role) string {
	var value string
	switch role {
	case RolePrimary:
		value = valuePrimary
	case RoleLocal:
		value = valueLocal
	case RoleNone:
		value = valueNone
	default:
		return ""
	}
	return markerOpen + "=" + value + markerClose
}

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
