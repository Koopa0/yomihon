package graph_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
)

// TestCommentZonesHoldsObsidianPairing is the lock over the shared comment-zone
// reading. Sequence and the judge used to drop an unpaired trailing mark and
// keep reading; the page hid everything after it. One table here is the
// pairing both faces now consume: a closed pair, an unclosed run to the end,
// and a percent sign that is only code.
func TestCommentZonesHoldsObsidianPairing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		code []graph.Span
		want []graph.Span
	}{
		{
			name: "paired",
			body: "keep %%hide%% keep",
			want: []graph.Span{{Start: 5, Stop: 13}},
		},
		{
			name: "unpaired trailing",
			body: "keep %%hide the rest",
			want: []graph.Span{{Start: 5, Stop: len("keep %%hide the rest")}},
		},
		{
			name: "%% inside a fence",
			body: fenceBody,
			code: []graph.Span{fenceContent},
		},
		{
			name: "%% inside inline code",
			body: inlineBody,
			code: []graph.Span{inlineContent},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := graph.CommentZones(tt.body, tt.code)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("CommentZones() mismatch (-want +got):\n%s", diff)
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

func mustSpan(body, inner string) graph.Span {
	start := strings.Index(body, inner)
	if start < 0 {
		panic("graph: inner not found in body")
	}
	return graph.Span{Start: start, Stop: start + len(inner)}
}

func TestCommentZonesPairsHTMLAndPercentInSourceOrder(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, body string
		want       []graph.Span
	}{
		{name: "HTML", body: "A<!-- x -->B", want: []graph.Span{{Start: 1, Stop: 11}}},
		{name: "percent inside HTML", body: "A<!-- %% -->B", want: []graph.Span{{Start: 1, Stop: 12}}},
		{name: "HTML inside percent", body: "A%% <!-- %%B", want: []graph.Span{{Start: 1, Stop: 11}}},
		{name: "both kinds", body: "A<!--x-->B%%y%%C", want: []graph.Span{{Start: 1, Stop: 9}, {Start: 10, Stop: 15}}},
		{name: "short forms", body: "A<!-->B<!--->C", want: []graph.Span{{Start: 1, Stop: 6}, {Start: 7, Stop: 13}}},
		{name: "escaped", body: `A\<!-- x -->B`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tt.want, graph.CommentZones(tt.body, nil)); diff != "" {
				t.Errorf("CommentZones HTML pairing (-want +got):\n%s", diff)
			}
		})
	}
}

func FuzzHTMLCommentSpan(f *testing.F) {
	for _, s := range []string{"<!-- x -->", "<!-->", "<!--->", "<!-- x > [[Ghost]] -->", "<!-- unclosed", "\\<!-- literal -->", "> <!-- private\n> secret\n\nAfter"} {
		f.Add(s, 0)
	}
	f.Fuzz(func(t *testing.T, body string, open int) {
		span, closed := graph.HTMLCommentSpan(body, open)
		if span.Zero() {
			return
		}
		if span.Start != open || span.Start < 0 || span.Stop > len(body) || span.Stop <= span.Start {
			t.Fatalf("HTMLCommentSpan invalid source range: %+v", span)
		}
		if closed && !strings.HasSuffix(body[span.Start:span.Stop], "-->") && body[span.Start:span.Stop] != "<!-->" && body[span.Start:span.Stop] != "<!--->" {
			t.Fatalf("HTMLCommentSpan closed without its closer: %q", body[span.Start:span.Stop])
		}
	})
}
