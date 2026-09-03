package nav

import "github.com/koopa0/yomihon/internal/schema"

// Shell is the snapshot-derived state shared by the topbar and sidebar.
// A handler receives it as one value so navigation and the governed flag
// cannot come from different atomic snapshot reads.
//
// It is built one package away, by internal/shell, which reaches both the
// generation and the write face's authority. Neither of those may be reached
// from here — this package is the model a projection produces, not the wiring
// that gathers one — so the type and the projector share a word and nothing
// else. That package's own doc says why it could live nowhere better.
//
// Governed says whether anything claimed authority over this vault. It gates
// every surface that would otherwise present a lifecycle vocabulary — a status
// chip, the write face — because naming a status the vault never declared
// invents a vocabulary rather than reporting one.
type Shell struct {
	Nav      *Model
	Governed bool
}

// WithoutInstanceProjections returns a shell whose navigation and topbar carry
// no instance-derived state. Direct file and folder navigation remain in the
// model; the supplied claim records why instance projections closed and, when
// it carries one, the sentence to show.
func (s Shell) WithoutInstanceProjections(claim schema.Claim) Shell {
	s.Nav = s.Nav.WithoutInstanceProjections(Close(claim))
	return s
}
