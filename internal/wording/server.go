package wording

// What the server itself says when it refuses a request before any reading
// face has seen it.

// The two refusals below are written by the outermost middleware, which knows
// nothing about the page that was asked for and answers with a line of text
// rather than a page. They are still what a reader reads, so they are written
// here in both languages like every other sentence, and resolved against the
// language that reader's own request carries.
var (
	ServerStopping = both(
		"yomihon 正在停止服務，已接受的工作完成後就結束。",
		"yomihon is stopping; it is finishing the work it already accepted.",
	)
	ServerIsForThisMachine = both(
		"yomihon 只服務這台機器。",
		"yomihon serves this machine only.",
	)
)
