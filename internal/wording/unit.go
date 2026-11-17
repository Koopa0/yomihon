package wording

// UnitWords is how the interface counts and steps through the units of a
// course. A unit is counted "N 單元" wherever it is counted, so a surface never
// mixes two nouns on one page.
//
// Chinese does not inflect a noun for number and English does, so a count is a
// pair. The branched count is one format: Chinese names the unit inside it and
// English says only where the rows are.
type UnitWords struct {
	CountOne, CountMany Phrase
	// BranchedFmt takes the number of entries the path's side branches list.
	BranchedFmt Phrase
	// DraftsFmt takes how many of the counted entries declare draft: true. It
	// says what is still to be finished and never how far a reader has come,
	// and a page draws nothing at all when none is left.
	DraftsFmt Phrase
	// Previous and Next are the step words at the foot of the article, and
	// RailPrevious and RailNext the same steps in the sidebar, which carry their
	// own separator because the name follows immediately.
	Previous, Next         Phrase
	RailPrevious, RailNext Phrase
}

// Units is the one set of unit words.
var Units = UnitWords{
	CountOne:     both("%d 單元", "%d unit"),
	CountMany:    both("%d 單元", "%d units"),
	BranchedFmt:  both("支線 %d 單元", "%d in a branch"),
	DraftsFmt:    both("其中 %d 單元是草稿", "%d in draft"),
	Previous:     both("上一單元", "Previous unit"),
	Next:         both("下一單元", "Next unit"),
	RailPrevious: both("上一單元：", "Previous unit: "),
	RailNext:     both("下一單元：", "Next unit: "),
}
