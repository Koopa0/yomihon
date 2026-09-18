package wording

// The journal, read one month at a time: what the page calls itself, what it
// says while a month holds nothing, and how a calendar month is named.

// What the journal page says around its calendar.
var (
	JournalIndexLede = both(
		"依月份閱讀日誌。",
		"Read the journal a month at a time.",
	)
	// JournalIndexEmpty is for a vault with no journal at all, which is a
	// different thing from a month nobody wrote in.
	JournalIndexEmpty = both(
		"這個書庫裡沒有日誌。",
		"There are no journal entries in this vault.",
	)
	JournalMonthEmpty = both(
		"這個月沒有寫下任何日誌。",
		"Nothing was written in the journal this month.",
	)
)

// How many entries a month holds. Chinese counts writings with its own measure
// word and English inflects the noun, so the count is a pair like every other.
var (
	JournalCountOne  = both("%d 篇", "%d entry")
	JournalCountMany = both("%d 篇", "%d entries")
)

// The way out of a month, in both directions. Each link is named for the month
// it leads to rather than for being the previous or the next one: the journal
// is written in the months its author sat down, not in every month, so a step
// reaches the nearest one that holds something and says which that is.
//
// The arrow is written the same way in both languages because both are read
// left to right; the pair repeats itself rather than pretending the difference
// lies somewhere it does not.
var (
	JournalEarlierFmt = both("← %s", "← %s")
	JournalLaterFmt   = both("%s →", "%s →")
	JournalMonthsNav  = both("月份", "Months")
)

// JournalUndatedTitle heads the entries that wrote no day. They belong to no
// month, so they would fall off every page of a calendar; they are given a
// group of their own instead, at the foot of whichever month is open, and the
// heading says why they are there.
var JournalUndatedTitle = both("沒有寫日期的日誌", "Journal entries with no date")

// MonthFmt writes a calendar month with its year. Chinese counts the month and
// closes with the measure word, English calls the month by name, so the two
// forms take the same pair of values in opposite orders.
var MonthFmt = both("%[1]d 年 %[2]s", "%[2]s %[1]d")

// MonthNames are the twelve months, January first, as each language writes one
// inside MonthFmt. Chinese numbers them and English names them, which is why
// the Chinese half looks like a repetition of the number already in the format:
// the format places it, and this supplies the measure word that follows.
var MonthNames = [12]Phrase{
	both("1 月", "January"),
	both("2 月", "February"),
	both("3 月", "March"),
	both("4 月", "April"),
	both("5 月", "May"),
	both("6 月", "June"),
	both("7 月", "July"),
	both("8 月", "August"),
	both("9 月", "September"),
	both("10 月", "October"),
	both("11 月", "November"),
	both("12 月", "December"),
}

// WeekdayNames head the seven columns of a month, Monday first. Chinese keeps
// the 週 in front of the number so a column is read as a weekday rather than as
// the digit it would otherwise be mistaken for, aloud above all.
var WeekdayNames = [7]Phrase{
	both("週一", "Mon"),
	both("週二", "Tue"),
	both("週三", "Wed"),
	both("週四", "Thu"),
	both("週五", "Fri"),
	both("週六", "Sat"),
	both("週日", "Sun"),
}
