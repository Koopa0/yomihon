package commentzone

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestZonesHoldsObsidianPairing is the lock over the shared comment-zone
// reading. Sequence and the judge used to drop an unpaired trailing mark and
// keep reading; the page hid everything after it. One table here is the
// pairing both faces now consume: a closed pair, an unclosed run to the end,
// and a percent sign that is only code.
func TestZonesHoldsObsidianPairing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		code []Span
		want []Span
	}{
		{
			name: "paired",
			body: "keep %%hide%% keep",
			want: []Span{{Start: 5, Stop: 13}},
		},
		{
			name: "unpaired trailing",
			body: "keep %%hide the rest",
			want: []Span{{Start: 5, Stop: len("keep %%hide the rest")}},
		},
		{
			name: "%% inside a fence",
			body: fenceBody,
			code: []Span{fenceContent},
		},
		{
			name: "%% inside inline code",
			body: inlineBody,
			code: []Span{inlineContent},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Zones(tt.body, tt.code)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Zones() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

const (
	fenceBody  = "keep\n```\n%% not a comment\n```\nkeep"
	inlineBody = "keep `%%` keep"
)

var (
	fenceContent  = mustSpan(fenceBody, "%% not a comment\n")
	inlineContent = mustSpan(inlineBody, "%%")
)

func mustSpan(body, inner string) Span {
	start := strings.Index(body, inner)
	if start < 0 {
		panic("commentzone: inner not found in body")
	}
	return Span{Start: start, Stop: start + len(inner)}
}
