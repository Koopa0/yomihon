package wording

// The page that holds two notes at once: the offer that leads to it from
// either of them, what the page is called, and the words around the two
// columns.

// CompareOffer is the quiet link a note carries when one other note belongs
// beside it. It says what following it does rather than naming a relation,
// because the same words are shown whether the pair was declared by one of the
// two notes or read off a shared filename and two declared languages — a reader
// is being offered a way to read, not told how it was found.
var (
	CompareOffer    = both("對照閱讀", "Read side by side")
	CompareOfferNav = both("和這篇並排的筆記", "The note that belongs beside this one")
)

// CompareTitleFmt names the page after the two notes on it, in the order the
// address names them.
var CompareTitleFmt = both("%s 與 %s 對照閱讀", "%s and %s side by side")

// The words around a column. CompareColumns labels the strip of links that
// moves between the two where they are stacked, and CompareOpenAlone is each
// column's way back to the note's own page, where it can be adjudicated.
var (
	CompareColumns   = both("這一頁的兩篇", "The two notes on this page")
	CompareOpenAlone = both("單獨開啟", "Open on its own")
)

// CompareAlign names the control that keeps the two columns at the same place
// in their headings while one of them is scrolled. It needs a script, so the
// page draws it only where one is running.
var CompareAlign = both("對齊捲動", "Scroll together")
