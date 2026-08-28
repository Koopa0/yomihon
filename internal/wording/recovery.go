package wording

// RecoveryStartOver is what a write refused for a malformed request offers as
// the way on. It names the two pages a reader can get back to rather than
// asking them to fix the request, because the request was the page's to build
// and there is nothing on this one to correct.
var RecoveryStartOver = both(
	"返回筆記或首頁，從目前頁面重新開始這次操作。",
	"Go back to the note or to the home page, and start this again from the page you were on.",
)
