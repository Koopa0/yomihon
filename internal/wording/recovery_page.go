package wording

// The recovery page's own frame: the words around whichever refusal brought
// the reader here.

// What the page's heading says, which turns on the one thing that matters:
// whether the note on disk was changed before the refusal.
var (
	RecoveryChangedTitle   = both("狀態已寫入，需要手動收尾", "The status was written and needs finishing by hand")
	RecoveryUnchangedTitle = both("狀態尚未變更", "The status has not changed")
	RecoveryChangedState   = both("這次操作已變更筆記檔案；請勿重送這次操作。", "This changed the note's file. Do not resend it.")
	RecoveryUnchangedState = both("這次操作沒有變更筆記檔案。", "Nothing was written to the note's file.")
)

// The page's sections and the ways off it.
var (
	LifecycleWrite  = both("生命週期寫入", "Lifecycle write")
	TechnicalDetail = both("技術細節", "Technical detail")
	RecoveryActions = both("復原操作", "Ways on")
	BackToNote      = both("返回筆記", "Back to the note")
)
