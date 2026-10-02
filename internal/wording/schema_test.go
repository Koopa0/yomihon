package wording

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFrontmatterSchemaSentenceNamesItsActualCause(t *testing.T) {
	t.Parallel()
	const diagnostic = "frontmatter is not valid YAML: yaml: line 2: mapping values are not allowed in this context"
	for _, tc := range []struct {
		name       string
		lang       Lang
		diagnostic string
		want       []SchemaPart
	}{
		{"invalid YAML in Chinese", ZhHant, diagnostic, []SchemaPart{{Text: "frontmatter 不是有效的 YAML。解析器指出："}, {Text: diagnostic, Code: true}}},
		{"invalid YAML in English", En, diagnostic, []SchemaPart{{Text: "The frontmatter is not valid YAML. The parser reported: "}, {Text: diagnostic, Code: true}}},
		{"required block missing in Chinese", ZhHant, "", []SchemaPart{{Text: "這份筆記需要 frontmatter，但沒有 frontmatter 區塊。"}}},
		{"required block missing in English", En, "", []SchemaPart{{Text: "This note requires frontmatter, but no frontmatter block is present."}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := SchemaSentence(tc.lang, "schema.frontmatter", "", tc.diagnostic, "")
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("frontmatter explanation (-want +got):\n%s", diff)
			}
		})
	}
}
