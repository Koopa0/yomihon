package nav

import "strconv"

// EntryKind distinguishes a linked entry from each warning-row reason. Both row
// families carry it: what resolving a target found is one question on either.
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

// String names a resolution outcome for a diagnostic, a log line or a panic,
// using the words a rail row already carries in its markup. A kind outside the
// four constants is a programming error and panics; a surface that has to keep
// drawing over such a value asks for the number instead.
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
