package nav

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
