package wording

import "testing"

// The words of an export are the ones the reader approved, in both languages:
// a page that copies a group and the command that exports every group say them
// alike, and a script that reads an export looks for them.
func TestTheWordsOfAnExport(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name         string
		phrase       Phrase
		zhHant, want string
	}{
		{"lede", ExportLede, "連結相對書庫的根目錄。", "Links are relative to the vault's root folder."},
		{"open questions", ExportOpenQuestions, "未解決的疑問", "Open questions"},
		{"thoughts", ExportThoughts, "想法", "Thoughts"},
		{"highlights", ExportHighlights, "劃線", "Highlights"},
		{"resolved questions", ExportResolvedQuestions, "已解決的疑問", "Resolved questions"},
		{"the comma before a section", CiteSectionSep, "，", ", "},
	} {
		if got := tt.phrase.In(ZhHant); got != tt.zhHant {
			t.Errorf("%s in Traditional Chinese = %q, want %q", tt.name, got, tt.zhHant)
		}
		if got := tt.phrase.In(En); got != tt.want {
			t.Errorf("%s in English = %q, want %q", tt.name, got, tt.want)
		}
	}
}
