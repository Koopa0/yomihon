package wording

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestReferenceSequenceSentence(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		lang Lang
		want []SchemaPart
	}{
		{lang: ZhHant, want: []SchemaPart{{Text: "based_on", Code: true}, {Text: " 裡有一項被 YAML 讀成巢狀清單，沒有讀成引用。請替連結加上引號，例如 "}, {Text: "based_on: [\"[[Note]]\"]", Code: true}, {Text: " 或 "}, {Text: "based_on: [\"Note\"]", Code: true}, {Text: "。"}}},
		{lang: En, want: []SchemaPart{{Text: "An item in "}, {Text: "based_on", Code: true}, {Text: " was read as a nested YAML list, not a reference. Quote the link, for example "}, {Text: "based_on: [\"[[Note]]\"]", Code: true}, {Text: " or "}, {Text: "based_on: [\"Note\"]", Code: true}, {Text: "."}}},
	} {
		t.Run(string(tt.lang), func(t *testing.T) {
			t.Parallel()
			got := SchemaSentence(tt.lang, "schema.reference_nested_sequence", "based_on", "", "")
			t.Log("hit: reference sequence wording reached")
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("caught: reference repair sentence (-want +got):\n%s", diff)
			}
		})
	}
}

func TestReferenceSequenceSentenceKeepsAuthoredFieldAsText(t *testing.T) {
	t.Parallel()
	const field = "replacement<&>"
	for _, lang := range []Lang{ZhHant, En} {
		t.Run(string(lang), func(t *testing.T) {
			t.Parallel()
			parts := SchemaSentence(lang, "schema.reference_nested_sequence", field, "", "")
			var got []string
			for _, part := range parts {
				if part.Code {
					got = append(got, part.Text)
				}
			}
			want := []string{field, "replacement<&>: [\"[[Note]]\"]", "replacement<&>: [\"Note\"]"}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("caught: authored reference field pieces (-want +got):\n%s", diff)
			}
		})
	}
}
