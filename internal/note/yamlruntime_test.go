package note_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestNoteExplainsUnsupportedYAML keeps the parser's implementation failure
// out of the reading page and names the authored forms a reader can change.
func TestNoteExplainsUnsupportedYAML(t *testing.T) {
	t.Parallel()
	const body = "---\n<<: {a: 1}\n? [1, 2]\n: 3\n---\nReadable body.\n"
	for _, governed := range []bool{false, true} {
		for _, chrome := range []struct {
			lang wording.Lang
			want string
		}{
			{wording.ZhHant, "frontmatter 使用了 yomihon 讀不進來的 YAML 寫法：合併鍵（<<），或把串列、對應表當作鍵。請直接編輯 frontmatter，改用一般的文字鍵。"},
			{wording.En, "The frontmatter uses a YAML form yomihon cannot read: a merge key (<<), or a list or mapping used as a key. Edit the frontmatter directly to use ordinary text keys."},
		} {
			name := string(chrome.lang) + "/ungoverned"
			if governed {
				name = string(chrome.lang) + "/governed"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				root := writeNotes(t, map[string]string{"Writing/Bad.md": body})
				contract := loadContract(t)
				governance := schema.Ungoverned()
				if governed {
					governance = contract.Governance()
				}
				server := newServerWithGovernance(t, root, contract, governance)
				page := frontmatterNoticePage(t, server, chrome.lang, "Writing/Bad.md")
				if !strings.Contains(page, strings.ReplaceAll(chrome.want, "<<", "&lt;&lt;")) {
					t.Errorf("note explanation omitted %q", chrome.want)
				}
				for _, unwanted := range []string{"runtime error", "interface {}"} {
					if strings.Contains(page, unwanted) {
						t.Errorf("note explanation leaks %q", unwanted)
					}
				}
			})
		}
	}
}
