package search

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

// A zero here reads as "I never wrote that down" — a clinician searched 20mg,
// the note said 20-40mg, and the page's only advice was to try different
// words. The empty page now offers loosened searches, and only ones the index
// has already answered with something: advice that leads to a second zero
// teaches the reader to distrust the page that gave it.
func TestStepBacksOfferOnlySearchesThatFindSomething(t *testing.T) {
	t.Parallel()
	idx := NewIndex([]Document{
		{RelPath: "臨床/利尿劑調整.md", Title: "利尿劑", PlainText: "Furosemide 起始 20-40mg"},
		{RelPath: "臨床/心衰竭.md", Title: "HFrEF", PlainText: "四大支柱與劑量"},
	}, schema.ArtifactPolicy{})

	tests := []struct {
		name string
		raw  string
		want []StepBack
	}{
		{
			name: "a digit-letter run splits into terms the note carries",
			raw:  "20mg",
			want: []StepBack{{Query: "20 mg", Count: 1}},
		},
		{
			name: "terms that never share a note are offered one at a time",
			raw:  "Furosemide 劑量",
			want: []StepBack{
				{Query: "Furosemide", Count: 1},
				{Query: "劑量", Count: 1},
			},
		},
		{
			name: "no offer is made from a search that also finds nothing",
			raw:  "lasix",
			want: nil,
		},
		{
			// The candidate machinery generates "lasix" as a lone term here,
			// and it finds nothing — the offer must be filtered out, not
			// carried with a zero. This is the case the whole feature exists
			// to never repeat.
			name: "a generated candidate that finds nothing is dropped",
			raw:  "Furosemide lasix",
			want: []StepBack{{Query: "Furosemide", Count: 1}},
		},
		{
			name: "a single-letter fragment is not worth offering",
			raw:  "L4",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := idx.StepBacks(tt.raw)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("StepBacks(%q) mismatch (-want +got):\n%s", tt.raw, diff)
			}
		})
	}
}
