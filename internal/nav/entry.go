package nav

import "strconv"

// EntryKind distinguishes a linked entry from each warning-row reason. It
// belongs to neither of the two row families and is carried by both: MapEntry
// is a row of a general map, PathEntry a row of a study path, and what
// resolving a row's target found is the same question on either.
type EntryKind uint8

const (
	// EntryResolved is a unique wikilink target that can be navigated to.
	EntryResolved EntryKind = iota
	// EntryUnresolved has no matching vault target.
	EntryUnresolved
	// EntryAmbiguous has several candidates and deliberately names none of them.
	EntryAmbiguous
	// EntryNonInstance resolves uniquely to a readable artifact that is outside
	// the governed instance set.
	EntryNonInstance
)

// String names a resolution outcome for a diagnostic, a log line or a panic.
// The words are the ones a rail row already carries in its markup, so one
// outcome is one word wherever it is read.
//
// A kind outside the four constants is a programming error and stops here
// naming its number, the same as every other closed set in this repository. A
// surface that must keep drawing over such a value asks for the number
// directly rather than for a name; the reading interface does exactly that,
// because a row it has no words for is still news worth showing.
func (k EntryKind) String() string {
	switch k {
	case EntryResolved:
		return "resolved"
	case EntryUnresolved:
		return "unresolved"
	case EntryAmbiguous:
		return "ambiguous"
	case EntryNonInstance:
		return "non-instance"
	default:
		panic("nav: unknown EntryKind: " + strconv.Itoa(int(k)))
	}
}
