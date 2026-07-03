package search

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestParse reproduces every row of docs/search-plan.md §5a's parser case
// table with its hand-computed expected Tokens/Filters — the acceptance basis
// for the R1-R4 parse rules. NFD fixtures use explicit code points so the
// input's Unicode form is unambiguous: "が" is NFD が (か + combining
// dakuten), whose NFC form is "が" (が).
func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  Query
	}{
		{name: "1 empty", input: "", want: Query{}},
		{name: "2 whitespace only", input: "   \t ", want: Query{}},
		{name: "3 bare token", input: "kafka", want: Query{Tokens: []string{"kafka"}}},
		{name: "4 fold uppercase", input: "Kafka", want: Query{Tokens: []string{"kafka"}}},
		{name: "5 two cjk tokens", input: "深度 工作", want: Query{Tokens: []string{"深度", "工作"}}},
		{name: "6 nfd token folds to nfc", input: "が", want: Query{Tokens: []string{"が"}}},
		{name: "7 type filter", input: "type:lesson", want: Query{Filters: []Filter{{Key: "type", Value: "lesson"}}}},
		{
			name:  "8 token order preserved around filter",
			input: "深度 type:lesson 工作",
			want:  Query{Tokens: []string{"深度", "工作"}, Filters: []Filter{{Key: "type", Value: "lesson"}}},
		},
		{
			name:  "9 repeated topic key is AND",
			input: "topic:a topic:b",
			want:  Query{Filters: []Filter{{Key: "topic", Value: "a"}, {Key: "topic", Value: "b"}}},
		},
		{
			name:  "10 repeated type key",
			input: "type:a type:b",
			want:  Query{Filters: []Filter{{Key: "type", Value: "a"}, {Key: "type", Value: "b"}}},
		},
		{name: "11 folder drops trailing slash", input: "folder:Writing/", want: Query{Filters: []Filter{{Key: "folder", Value: "Writing"}}}},
		{name: "12 slug splits on first colon", input: "slug:a:b", want: Query{Filters: []Filter{{Key: "slug", Value: "a:b"}}}},
		{name: "13 unknown key is bare token", input: "foo:bar", want: Query{Tokens: []string{"foo:bar"}}},
		{name: "14 classify before fold", input: "Type:lesson", want: Query{Tokens: []string{"type:lesson"}}},
		{name: "15 empty filter value", input: "status:", want: Query{Filters: []Filter{{Key: "status", Value: ""}}}},
		{name: "16 literal percent", input: "%", want: Query{Tokens: []string{"%"}}},
		{name: "17 percent inside token", input: "100%", want: Query{Tokens: []string{"100%"}}},
		{name: "18 domain nfc value", input: "domain:日本語", want: Query{Filters: []Filter{{Key: "domain", Value: "日本語"}}}},
		{name: "19 slug value not case folded", input: "slug:ABC", want: Query{Filters: []Filter{{Key: "slug", Value: "ABC"}}}},
		{name: "20 folder empty value", input: "folder:", want: Query{Filters: []Filter{{Key: "folder", Value: ""}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Parse(tt.input)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Parse(%q) mismatch (-want +got):\n%s", tt.input, diff)
			}
		})
	}
}

// TestParseFilterValueNFD proves R3 on a filter value with a genuine NFD form
// (row 18's 日本語 has no canonical decomposition, so it cannot). "domain:" +
// NFD が ("が") must store the NFC form "が", un-case-folded.
func TestParseFilterValueNFD(t *testing.T) {
	t.Parallel()

	got := Parse("domain:が")
	want := Query{Filters: []Filter{{Key: "domain", Value: "が"}}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Parse NFD filter value mismatch (-want +got):\n%s", diff)
	}
}
