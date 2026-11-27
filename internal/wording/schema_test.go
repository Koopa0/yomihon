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
		{"required block missing in Chinese", ZhHant, "", []SchemaPart{{Text: "這篇筆記需要 frontmatter，但沒有 frontmatter 區塊。"}}},
		{"required block missing in English", En, "", []SchemaPart{{Text: "This note requires frontmatter, but no frontmatter block is present."}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := SchemaSentence(tc.lang, "schema.frontmatter", "", tc.diagnostic)
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

// TestAllowedEnumFoldSaysWhatTheSentenceSays pins the fold's heading and list
// to the sentence the health page prints: the same words with nothing before
// the heading and no colon after it, and the values in the reader's separator.
func TestAllowedEnumFoldSaysWhatTheSentenceSays(t *testing.T) {
	t.Parallel()
	text := func(parts []SchemaPart) string {
		var b strings.Builder
		for _, p := range parts {
			b.WriteString(p.Text)
		}
		return b.String()
	}
	for _, tc := range []struct {
		name          string
		lang          Lang
		values        []string
		heading, list string
	}{
		{"Chinese", ZhHant, []string{"draft", "ready"}, "允許的 status 值", "draft、ready"},
		{"English", En, []string{"draft", "ready"}, "Allowed status values", "draft, ready"},
		{"Chinese empty", ZhHant, []string{}, "允許的 status 值", "無。"},
		{"English unavailable", En, nil, "Allowed status values", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := text(AllowedEnumHeading(tc.lang, "status")); got != tc.heading {
				t.Errorf("AllowedEnumHeading(%s) = %q, want %q", tc.lang.Tag(), got, tc.heading)
			}
			if got := text(AllowedEnumList(tc.lang, tc.values)); got != tc.list {
				t.Errorf("AllowedEnumList(%s, %q) = %q, want %q", tc.lang.Tag(), tc.values, got, tc.list)
			}
		})
	}
}

// TestAnUnknownSchemaRuleStillSaysSomething holds the fallback: a rule nobody
// wrote words for is still answered, by a sentence naming the rule a reader
// would have to ask about.
func TestAnUnknownSchemaRuleStillSaysSomething(t *testing.T) {
	t.Parallel()

	const invented = "schema.not_a_rule_anyone_wrote"
	for _, lang := range []Lang{ZhHant, En} {
		var said strings.Builder
		for _, part := range SchemaSentence(lang, invented, "", "") {
			said.WriteString(part.Text)
		}
		if !strings.Contains(said.String(), invented) {
			t.Errorf("SchemaSentence(%s, %q) = %q, want it to name the rule", lang, invented, said.String())
		}
	}
}
