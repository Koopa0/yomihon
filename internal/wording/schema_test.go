package wording

import (
	"strings"
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

func TestAllowedEnumValuesUsesTheReadersSeparator(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		lang   Lang
		values []string
		want   string
		codes  []string
	}{
		{"Chinese full", ZhHant, []string{"draft", "ready", "archived"}, " 允許的 status 值：draft、ready、archived。", []string{"status", "draft", "ready", "archived"}},
		{"English full", En, []string{"draft", "ready", "archived"}, " Allowed status values: draft, ready, archived.", []string{"status", "draft", "ready", "archived"}},
		{"Chinese empty", ZhHant, []string{}, " 允許的 status 值：無。", []string{"status"}},
		{"English empty", En, []string{}, " Allowed status values: none.", []string{"status"}},
		{"Chinese single", ZhHant, []string{"draft"}, " 允許的 status 值：draft。", []string{"status", "draft"}},
		{"English single", En, []string{"draft"}, " Allowed status values: draft.", []string{"status", "draft"}},
		{"Chinese unavailable", ZhHant, nil, "", nil},
		{"English unavailable", En, nil, "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parts := AllowedEnumValues(tc.lang, "status", tc.values)
			t.Log("hit: allowed-sentence producer reached")
			var sentence strings.Builder
			var codes []string
			for _, part := range parts {
				sentence.WriteString(part.Text)
				if part.Code {
					codes = append(codes, part.Text)
				}
			}
			if got := sentence.String(); got != tc.want {
				t.Errorf("caught: complete allowed sentence = %q, want %q", got, tc.want)
			}
			if diff := cmp.Diff(tc.codes, codes); diff != "" {
				t.Errorf("caught: code pieces (-want +got):\n%s", diff)
			}
			if tc.values == nil && parts != nil {
				t.Error("caught: unavailable vocabulary produced sentence parts")
			}
		})
	}
}
