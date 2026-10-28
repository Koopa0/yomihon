package wording

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestFrontmatterRuntimeExplanationBoundaries distinguishes the captured
// runtime prefix from the same text quoted inside ordinary parser evidence.
func TestFrontmatterRuntimeExplanationBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, diagnostic string
		unsupported      bool
	}{
		{"runtime list key", "frontmatter is not valid YAML: yaml: runtime error: hash of unhashable type []interface {}", true},
		{"runtime mapping key", "frontmatter is not valid YAML: yaml: runtime error: hash of unhashable type map[string]interface {}", true},
		{"runtime prefix only", "frontmatter is not valid YAML: yaml: runtime error:", true},
		{"invalid list key", "frontmatter is not valid YAML: yaml: invalid map key: []interface {}{1}", true},
		{"invalid mapping key", "frontmatter is not valid YAML: yaml: invalid map key: map[string]interface {}{\"a\":1}", true},
		{"invalid map key prefix only", "frontmatter is not valid YAML: yaml: invalid map key:", true},
		{"ordinary syntax", "frontmatter is not valid YAML: yaml: line 2: mapping values are not allowed in this context", false},
		{"quoted runtime text", "frontmatter is not valid YAML: yaml: unmarshal errors:\n  line 4: mapping key \"frontmatter is not valid YAML: yaml: runtime error: <script>&\" already defined at line 3", false},
		{"quoted invalid map key text", "frontmatter is not valid YAML: yaml: unmarshal errors:\n  line 4: mapping key \"frontmatter is not valid YAML: yaml: invalid map key:\" already defined at line 3", false},
		{"unwrapped text", "yaml: runtime error: hash of unhashable type []interface {}", false},
		{"unwrapped invalid map key", "yaml: invalid map key: []interface {}{1}", false},
	} {
		for _, chrome := range []struct {
			lang                            Lang
			rail, schemaNotice, unsupported string
		}{
			{ZhHant, "frontmatter 不是有效的 YAML。", "frontmatter 不是有效的 YAML。解析器指出：", "frontmatter 使用了 yomihon 讀不進來的 YAML 寫法：合併鍵（<<），或把串列、對應表當作鍵。請直接編輯 frontmatter，改用一般的文字鍵。"},
			{En, "The frontmatter is not valid YAML.", "The frontmatter is not valid YAML. The parser reported: ", "The frontmatter uses a YAML form yomihon cannot read: a merge key (<<), or a list or mapping used as a key. Edit the frontmatter directly to use ordinary text keys."},
		} {
			t.Run(tc.name+"/"+string(chrome.lang), func(t *testing.T) {
				t.Parallel()
				wantRail := [2]string{chrome.rail, tc.diagnostic}
				wantSchema := []SchemaPart{{Text: chrome.schemaNotice}, {Text: tc.diagnostic, Code: true}}
				if tc.unsupported {
					wantRail = [2]string{chrome.unsupported, ""}
					wantSchema = []SchemaPart{{Text: chrome.unsupported}}
				}
				summary, detail := FrontmatterExplanation(chrome.lang, tc.diagnostic)
				if diff := cmp.Diff(wantRail, [2]string{summary, detail}); diff != "" {
					t.Errorf("FrontmatterExplanation(%q) mismatch (-want +got):\n%s", tc.diagnostic, diff)
				}
				gotSchema := SchemaSentence(chrome.lang, "schema.frontmatter", "", tc.diagnostic, "")
				if diff := cmp.Diff(wantSchema, gotSchema); diff != "" {
					t.Errorf("SchemaSentence(%q) mismatch (-want +got):\n%s", tc.diagnostic, diff)
				}
			})
		}
	}
}
