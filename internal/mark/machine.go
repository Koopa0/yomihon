package mark

import (
	"fmt"

	"github.com/koopa0/yomihon/internal/mark/machinemarks"
)

// MachineMarks are the marks marks.json holds, as the marks of a machine file:
// what the first write of a machine that has no file of marks takes in, as the
// legacy function of machinemarks.Options. It only reads: marks.json is not
// changed, renamed or removed, so that a reader who goes back to a version that
// keeps marks there finds them as they left them.
//
// A question's reason becomes its words, since the machine file has one field
// for the reader's words in every state, and the marks of the files an earlier
// version still kept are included, as List has them. A file that cannot be read
// is ErrUnreadable, and the write that asked stops with it.
func (s *Store) MachineMarks() ([]machinemarks.Mark, error) {
	held, err := s.List()
	if err != nil {
		return nil, err
	}
	out := make([]machinemarks.Mark, len(held))
	for i := range held {
		m := &held[i]
		words := m.Body
		if m.Kind == Question {
			words = m.Reason
		}
		out[i] = machinemarks.Mark{
			ID: m.ID, Kind: machinemarks.Kind(m.Kind), Path: m.RelPath, Anchor: m.Anchor,
			Quote: m.Quote, Before: m.Before, After: m.After, Identity: m.Identity,
			Body: words, Resolved: m.ResolvedAt, At: m.At,
		}
		if !isKind(m.Kind) {
			return nil, fmt.Errorf("%w: %q is not a kind of mark", ErrUnreadable, string(m.Kind))
		}
	}
	return out, nil
}

// isKind says k is one of the three states a mark is in.
func isKind(k Kind) bool { return k == Highlight || k == Question || k == Thought }
