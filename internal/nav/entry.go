package nav

import (
	"strconv"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/schema"
)

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

// Token is the name an outcome carries wherever something other than a reader
// reads it: a diagnostic, a log line, the data-resolution attribute a rail row
// is stamped with. The second result reports whether this package classifies
// the value at all, so a surface that cannot stop halfway through a rail has
// something to draw for a kind nobody here has named.
func (k EntryKind) Token() (string, bool) {
	switch k {
	case EntryResolved:
		return "resolved", true
	case EntryUnresolved:
		return "unresolved", true
	case EntryAmbiguous:
		return "ambiguous", true
	case EntryNonInstance:
		return "non-instance", true
	default:
		return "", false
	}
}

// String names a resolution outcome for a diagnostic, a log line or a panic. A
// kind outside the four constants is a programming error and panics; a surface
// that has to keep drawing over such a value asks Token instead.
func (k EntryKind) String() string {
	token, known := k.Token()
	if !known {
		panic("nav: unknown EntryKind: " + strconv.Itoa(int(k)))
	}
	return token
}

// entryKindOf classifies one resolved target the way both reader-facing rows
// classify it. The map's rows and a study path's rows ask the same question of
// the same resolver, and they used to answer it in two places: the same four
// outcomes written twice, with opposite policies for a value neither of them
// recognized. One of them stopped; the other filed it as "no such note", which
// is a sentence about the vault rather than about the code.
//
// A kind outside the resolver's closed set is a programming error, so this
// stops. Reporting it as an unresolved link would blame an author for a note
// they wrote and yomihon failed to classify.
func entryKindOf(res graph.Resolution, policy schema.ArtifactPolicy) EntryKind {
	switch res.Kind {
	case graph.KindUnique:
		if policy.IsNonInstance(res.RelPath) {
			return EntryNonInstance
		}
		return EntryResolved
	case graph.KindAmbiguous:
		return EntryAmbiguous
	case graph.KindUnresolved:
		return EntryUnresolved
	default:
		panic("nav: unknown graph.Kind: " + res.Kind.String())
	}
}
