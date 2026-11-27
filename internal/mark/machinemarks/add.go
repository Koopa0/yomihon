package machinemarks

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
)

// Outcome is what an Add or a Toggle found and did.
type Outcome struct {
	Result

	// Mark is the mark as it stands: the one written, the one that was there
	// already, or the one that was taken away.
	Mark Mark
	// Written says a line was written. An Add that found the mark there already
	// writes none.
	Written bool
	// Remaining is how many other marks that are not deleted the same place
	// still names after a Toggle took one away. Two machines that each marked a
	// place before either had read the other's file leave two.
	Remaining int
}

// Add writes mark as a new mark, unless a mark that is not deleted is the same
// act as it (SameAct), in which case that one is the answer and nothing is
// written. The look and the write are one step under the vault's lock, so two
// Adds of one highlight at once keep one.
//
// mark's ID has to be one no mark has: an Add is a new mark and never a change
// of one the reader had read, which has to say what it had read. admit is asked
// of a mark that is new, under the lock, and says whether the place is one the
// caller can stand behind.
func (m *Marks) Add(ctx context.Context, mark *Mark, admit func(*Mark) bool) (Outcome, error) {
	if err := validate(mark, nil); err != nil {
		return Outcome{}, err
	}
	var out Outcome
	res, err := m.change(ctx, func(v *view) (*proposal, error) {
		if there := sameMark(v, func(head *Mark) bool { return SameAct(head, mark) }); len(there) > 0 {
			out.Mark = there[0].heads[0].Mark
			return &proposal{noWrite: true, base: entrySeen(v, out.Mark.ID)}, nil
		}
		if err := admitted(v, mark, admit); err != nil {
			return nil, err
		}
		out.Mark, out.Written = *mark, true
		return &proposal{mark: *mark, grows: true}, nil
	})
	out.Result = res
	return out, err
}

// admitted says whether mark may be made new: its id is one no mark has, and the
// caller, which knows what the file does not, admits it. admit is asked under the
// vault's lock, and only for a mark that is new; none refuses every mark.
func admitted(v *view, mark *Mark, admit func(*Mark) bool) error {
	if _, taken := v.byMark[mark.ID]; taken {
		return fmt.Errorf("%w: a mark has that id already", ErrInvalidMark)
	}
	if admit == nil || !admit(mark) {
		return fmt.Errorf("%w: the place is not one the caller admits", ErrInvalidMark)
	}
	return nil
}

// Toggle takes away the mark that is the same as mark, as same says, if a mark
// that is not deleted is, and otherwise writes mark as a new mark. It is the one
// control of a reader who marks a place and unmarks it, with no page of lines to
// read first, so it reads under the lock what the reader would have read and
// writes on top of exactly that.
//
// A mark that two machines changed differently, which has more than one value,
// is not taken away: the reader has not been shown what one of the values says,
// and taking the mark away would cover them all. That is ErrStale, with what
// is read now. Marks that are the same place and were each made by a machine
// that had not read the other leave Remaining to take away by asking again.
// Taking away is always allowed; admit is asked of a mark that is new only.
func (m *Marks) Toggle(ctx context.Context, mark *Mark, same, admit func(*Mark) bool) (Outcome, error) {
	if err := validate(mark, nil); err != nil {
		return Outcome{}, err
	}
	var out Outcome
	res, err := m.change(ctx, func(v *view) (*proposal, error) {
		there := sameMark(v, same)
		if len(there) == 0 {
			if err := admitted(v, mark, admit); err != nil {
				return nil, err
			}
			out.Mark, out.Written = *mark, true
			return &proposal{mark: *mark, grows: true}, nil
		}
		r := there[0]
		if len(r.heads) > 1 {
			return nil, &StaleError{Seen: entrySeen(v, r.heads[0].Mark.ID)}
		}
		gone := r.heads[0].Mark
		gone.Deleted = true
		out.Mark, out.Written, out.Remaining = gone, true, len(there)-1
		return &proposal{mark: gone, base: entrySeen(v, gone.ID)}, nil
	})
	out.Result = res
	return out, err
}

// sameMark is the marks that are not deleted and whose value that counts is the
// same as what same says, the oldest first.
func sameMark(v *view, same func(*Mark) bool) []resolved {
	var there []resolved
	for _, r := range v.byMark {
		head := &r.heads[0].Mark
		if head.Deleted || !same(head) {
			continue
		}
		there = append(there, r)
	}
	slices.SortFunc(there, func(a, b resolved) int {
		ma, mb := &a.heads[0].Mark, &b.heads[0].Mark
		return cmp.Or(ma.At.Compare(mb.At), strings.Compare(ma.ID, mb.ID))
	})
	return there
}

// TakeIn takes in the marks an adoption brings from the vault's earlier path,
// those of them convert says, and writes this machine's file as the ledger and
// the file say it is. A machine that has not taken in this vault's own earlier
// file takes that in first, as any first write does.
func (m *Marks) TakeIn(ctx context.Context, adopted []Mark) (Result, error) {
	return m.run(ctx, adopted, func(*view) (*proposal, error) {
		return &proposal{restore: true}, nil
	})
}

// SameAct says a, a mark that is there, is the same act of the reader's as b:
// one kind in one place, which for a passage is the words and the words around
// them, with the same words of the reader's own, and not a question that was
// resolved, which asking again opens anew. The same words in another place are
// another mark, and so are words that differ: nothing the reader wrote is
// answered with an earlier mark that says something else. A thought is never
// the same act as another: it says something, and two say two things.
func SameAct(a, b *Mark) bool {
	if a.Kind == Thought || a.Kind != b.Kind || a.Path != b.Path || a.Anchor != b.Anchor ||
		a.Quote != b.Quote || a.Body != b.Body || !a.Resolved.IsZero() {
		return false
	}
	return a.Quote == "" || (a.Before == b.Before && a.After == b.After)
}
