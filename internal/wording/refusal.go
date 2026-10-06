package wording

// The router and the browser's origin refusal need their own explanation:
// neither means a page is missing or that a note failed to save.
var (
	RequestRefusedKicker      = both("請求遭拒", "Request refused")
	RequestMethodRefusedTitle = both("這個地址不接受這種請求", "This address does not accept this request")
	RequestMethodRefusedLede  = both("請用下方連結重新開啟頁面。", "Open the page again using a link below.")
	RequestOriginRefusedTitle = both("這個請求未通過來源檢查", "This request did not pass the origin check")
	RequestOriginRefusedLede  = both("請從閱讀頁面重新操作。", "Try again from the reading page.")
)
