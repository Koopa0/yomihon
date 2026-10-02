package wording

// UnitWords is how the interface counts and steps through one kind of entry in
// a study path. A path of lessons is read in lessons and a path of anything
// else in items, so the whole set is chosen together, once per path, and a
// surface never mixes the two nouns on one page.
//
// Chinese does not inflect a noun for number and English does, so a count is a
// pair. The branched count is one format: Chinese names the unit inside it and
// English says only where the rows are.
type UnitWords struct {
	CountOne, CountMany Phrase
	// BranchedFmt takes the number of entries the path's side branches list.
	BranchedFmt Phrase
	// UnsettledFmt takes how many of the counted entries are not at a status
	// the contract settles. It says what is still to be finished and never how
	// far a reader has come, and a page draws nothing at all when none is left.
	UnsettledFmt Phrase
	// Previous and Next are the step words at the foot of the article, and
	// RailPrevious and RailNext the same steps in the sidebar, which carry their
	// own separator because the name follows immediately.
	Previous, Next         Phrase
	RailPrevious, RailNext Phrase
}

// LessonWords are the words for a path whose entries are lessons.
var LessonWords = UnitWords{
	CountOne:     both("%d 課", "%d lesson"),
	CountMany:    both("%d 課", "%d lessons"),
	BranchedFmt:  both("支線 %d 課", "%d in a branch"),
	UnsettledFmt: both("其中 %d 課尚未定案", "%d not yet settled"),
	Previous:     both("上一課", "Previous lesson"),
	Next:         both("下一課", "Next lesson"),
	RailPrevious: both("上一課：", "Previous lesson: "),
	RailNext:     both("下一課：", "Next lesson: "),
}

// ItemWords are the words for a path whose entries are anything but lessons.
var ItemWords = UnitWords{
	CountOne:     both("%d 篇", "%d item"),
	CountMany:    both("%d 篇", "%d items"),
	BranchedFmt:  both("支線 %d 篇", "%d in a branch"),
	UnsettledFmt: both("其中 %d 篇尚未定案", "%d not yet settled"),
	Previous:     both("上一篇", "Previous"),
	Next:         both("下一篇", "Next"),
	RailPrevious: both("上一篇：", "Previous: "),
	RailNext:     both("下一篇：", "Next: "),
}
