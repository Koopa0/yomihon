package pages

// leads reports whether block opens the desk. A reader who kept a place is
// opened on that place; one who kept none is opened on the courses, which are
// what a desk is for, and only where the vault has some to open: a block with
// nothing to link to would lead with an apology.
func (v *HomeView) leads(block *DeskBlock) bool {
	return !v.Continue.Show && block.Mode == courseMode && len(shelfRows(&block.Shelf, deskBlockItems)) > 0
}

// deskWay is one of the ways in that stand below the lead, and whether it
// takes the desk's whole width.
type deskWay struct {
	Block DeskBlock
	Whole bool
}

// ways lays out the blocks that do not lead. They stand two to a row, as
// shelves side by side, except the folders, which are the way into everything
// and run across the desk as one strip of their own wherever they come in the
// order. A block that would be left alone in a row takes the row rather than
// leave half the desk empty beside it.
func (v *HomeView) ways() []deskWay {
	var ways []deskWay
	half := 0
	for i := range v.Blocks {
		block := v.Blocks[i]
		if v.leads(&block) {
			continue
		}
		whole := block.Mode == folderMode
		if !whole {
			half++
		}
		ways = append(ways, deskWay{Block: block, Whole: whole})
	}
	if half%2 == 1 {
		for i := len(ways) - 1; i >= 0; i-- {
			if !ways[i].Whole {
				ways[i].Whole = true
				break
			}
		}
	}
	return ways
}
