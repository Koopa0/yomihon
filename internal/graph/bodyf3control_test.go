package graph_test

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
)

func TestBodyFactsF3NilControl(t *testing.T) {
	t.Log("AGREEMENT-INVOKED F3/f3-nil-zero")
	var facts graph.BodyFacts
	if got := facts.Source(); got != "" {
		t.Errorf("caught: F3 nil-source Source() = %q, want empty", got)
	}
}

func TestBodyFactsF3CopyControl(t *testing.T) {
	t.Log("AGREEMENT-INVOKED F3/f3-copy-handle")
	original := graph.ReadBody("## Kept\n\n- row\n")
	copied := original
	first := original.Source()
	got := []string{first, copied.Source(), original.Source(), copied.Source(), original.Source()}
	want := []string{
		"## Kept\n\n- row\n",
		"## Kept\n\n- row\n",
		"## Kept\n\n- row\n",
		"## Kept\n\n- row\n",
		"## Kept\n\n- row\n",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("caught: F3 copied-source repeated Source() (-want +got):\n%s", diff)
	}
}

func TestBodyFactsF3EarlyStopControl(t *testing.T) {
	t.Log("AGREEMENT-INVOKED F3/f3-early-stop")
	facts := graph.ReadBody("A%%one%%B%%two%%C\n")
	comments := facts.Comments()
	var stopped []graph.Span
	for span := range comments {
		stopped = append(stopped, span)
		break
	}
	got := [][]graph.Span{stopped, slices.Collect(comments), slices.Collect(comments)}
	want := [][]graph.Span{
		{{Start: 1, Stop: 8}},
		{{Start: 1, Stop: 8}, {Start: 9, Stop: 16}},
		{{Start: 1, Stop: 8}, {Start: 9, Stop: 16}},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("caught: F3 iterator-reuse same Comments iterator (-want +got):\n%s", diff)
	}
}
