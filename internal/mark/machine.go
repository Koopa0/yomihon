package mark

import (
	"github.com/koopa0/yomihon/internal/mark/machinemarks"
)

// MachineMarks are the marks marks.json holds, as the marks of a machine file:
// what the first write of a machine that has no file of marks takes in, as the
// legacy function machinemarks.Open takes. It only reads: marks.json is not
// changed, renamed or removed, so that a reader who goes back to a version that
// keeps marks there finds them as they left them.
//
// A question's reason becomes its words, since the machine file has one field
// for the reader's words in every state, and the marks of the files an earlier
// version still kept are included, as the file has them. A file that cannot be
// read is ErrUnreadable, and the write that asked stops with it.
func (s *Store) MachineMarks() ([]machinemarks.Mark, error) {
	held, _, err := s.loadMarks()
	if err != nil {
		return nil, err
	}
	out := make([]machinemarks.Mark, len(held))
	for i := range held {
		out[i] = toMachine(&held[i])
	}
	return out, nil
}
