package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/render"
)

func TestExcerptBudgetCountsAuthoredBytes(t *testing.T) {
	t.Parallel()
	const tail = "WORDS THE CARD MUST KEEP"
	body := "para\n" + strings.Repeat("%%hidden%%\n", 3_000) + tail + "\n"
	excerpt, found, narrowed := render.ReadExcerptPreview(body, "")
	if !found || narrowed {
		t.Fatalf("not-applied: whole-note cut found=%t narrowed=%t", found, narrowed)
	}
	excerpt, truncated := excerpt.Cap(24 << 10)
	want := "para\n" + strings.Repeat("\n", 3_000) + tail + "\n"
	if source := excerpt.Source(); source != want || truncated {
		t.Errorf("caught: S5 preview-budget original bytes source-length=%d tail=%t truncated=%t", len(source), strings.Contains(source, tail), truncated)
	}
	for _, tc := range []struct {
		body, want string
		budget     int
		truncated  bool
	}{
		{body: "一行\n二行\n三行\n", budget: 8, want: "一行", truncated: true},
		{body: "unbroken line", budget: 4, want: "unbroken line"},
		{body: "unbroken line\nomit", budget: 4, want: "unbroken line", truncated: true},
		{body: "para\n%%hidden%%\n    [[Missing]]\nomit\n", budget: 22, want: "para\n\n    [[Missing]]", truncated: true},
	} {
		excerpt, _, _ := render.ReadExcerptPreview(tc.body, "")
		excerpt, truncated := excerpt.Cap(tc.budget)
		if got := excerpt.Source(); got != tc.want || truncated != tc.truncated {
			t.Errorf("caught: S5 preview-budget boundary source=%q truncated=%t want=%q truncated=%t", got, truncated, tc.want, tc.truncated)
		}
	}
	t.Log("AGREEMENT-INVOKED S5/stage5-preview-budget")
}
