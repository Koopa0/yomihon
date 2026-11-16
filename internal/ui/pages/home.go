package pages

// leads reports whether block opens the desk. A reader who kept a place is
// opened on that place; one who kept none is opened on the courses, which are
// what a desk is for, and only where the vault has some to open: a block with
// nothing to link to would lead with an apology.
func (v *HomeView) leads(block *DeskBlock) bool {
	return !v.Continue.Show && block.Mode == courseMode && len(shelfRows(&block.Shelf, deskBlockItems)) > 0
}
