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
	// Vault is the folder itself, as the rail's foot states it.
	Vault Vault
}

// Vault is the folder being read, said in the three facts a reader at the foot
// of the rail is asking for: which folder this is, how much of it there is, and
// whether anything stands against it. All three belong to one generation of one
// folder, so they travel together rather than being gathered a second time
// beside whichever surface happens to need them.
//
// The zero value is the honest answer for a folder whose name could not be
// taken and which holds nothing: the rail says so in words rather than drawing
// three blanks.
type Vault struct {
	// Name is the folder's own name — the base name of the directory the
	// server was pointed at, since a vault contract declares none. Empty where
	// the path yields no name to show.
	Name string
	// Notes is how many markdown notes this generation holds, at every depth
	// and outside any declared knowledge layer: the question is how large the
	// folder is, not how much of it the shelf lists.
	Notes int
	// Findings is how many things the health page has to report about this
	// folder, counted the same way that page counts its own rows.
	Findings int
}

// WithoutInstanceProjections returns a shell whose navigation and topbar carry
// no instance-derived state. Direct file and folder navigation remain in the
// model; the supplied claim records why instance projections closed and, when
// it carries one, the sentence to show.
func (s Shell) WithoutInstanceProjections(claim schema.Claim) Shell {
	s.Nav = s.Nav.WithoutInstanceProjections(Close(claim))
	return s
}
