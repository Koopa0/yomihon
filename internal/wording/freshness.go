package wording

// FreshnessNewVersion is what the reading page says once the file has moved on
// from the words below it and reloading would show the newer ones.
var FreshnessNewVersion = both("此筆記已有新版本。", "This note has a newer version.")

// FreshnessReload names the control beside that sentence. It is a verb: what
// the sentence reports and what the control does are two different things, and
// a label repeating the report would leave the action unnamed.
var FreshnessReload = both("重新載入", "Reload")
