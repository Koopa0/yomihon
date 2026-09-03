package nav

import "github.com/koopa0/yomihon/internal/schema"

// Shell is the snapshot-derived state shared by the topbar and sidebar, handed
// to a handler as one value so navigation and the governed flag cannot come
// from different snapshot reads. Governed says whether anything claimed
// authority over this vault; it gates every surface that would otherwise name a
// status the vault never declared.
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
