package schema

// Settlement records which lifecycle statuses a contract declares settled: the
// resting state of a note whose author has finished with it. A surface that
// prints a status marks the notes that are not at one, so the ordinary state is
// not repeated on every row.
//
// The zero value declares nothing, which is also what a contract that never
// wrote the key gets: no status is settled, so every one is marked. Declared
// tells that apart from a contract that did write it, because a count of the
// notes not yet settled over a vault that declared nothing would count every
// note in it.
type Settlement struct {
	declared bool
	settled  map[string]struct{}
}

// Declared reports whether any lifecycle row wrote the settled key, as true or
// as false. A contract that wrote false everywhere has said something a
// contract that wrote nothing has not: its author considered the question and
// settled no status.
func (s Settlement) Declared() bool {
	return s.declared
}

// Settled reports whether status is one the contract settles. A lifecycle row
// may repeat a status to give it another applies_to list, and settledness is a
// fact about the word rather than about one type's use of it, so a status is
// settled when any row naming it says so. The argument is folded to the
// spelling the contract's own words carry, so a note's value can be passed as
// the file spelled it.
func (s Settlement) Settled(status string) bool {
	_, ok := s.settled[NormalizeWord(status)]
	return ok
}

// deriveSettlement reads the settled key off the lifecycle rows. stages are
// those same rows after decoding, in the same order, with their statuses
// already folded; the key itself lives only on the raw row, where an absent
// key and a false one can still be told apart.
func deriveSettlement(rows []rawLifecycleStage, stages []Stage) Settlement {
	var out Settlement
	for i := range rows {
		if rows[i].Settled == nil {
			continue
		}
		out.declared = true
		if !*rows[i].Settled {
			continue
		}
		if out.settled == nil {
			out.settled = make(map[string]struct{})
		}
		out.settled[stages[i].Status] = struct{}{}
	}
	return out
}

// Settlement returns the contract's declaration of which statuses are settled.
// A vault no contract governs declares none.
func (c *Contract) Settlement() Settlement {
	if c == nil {
		return Settlement{}
	}
	return c.settlement
}

// Settled reports whether the contract declares status settled.
func (c *Contract) Settled(status string) bool {
	return c.Settlement().Settled(status)
}

// DeclaresSettled reports whether any lifecycle row wrote the settled key. A
// face that marks only the notes not yet settled falls back to marking every
// status when this is false, which is how it behaved before the key existed.
func (c *Contract) DeclaresSettled() bool {
	return c.Settlement().Declared()
}
