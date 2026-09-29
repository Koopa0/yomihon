package sequence

import "testing"

// TestARowSaysWhetherItWroteAnAlias holds what a consumer naming the lesson
// needs from the parse: whether the author wrote display text after the
// separator, carried from the link's bytes. The candidate's Text stays the
// display either way, because diagnostics quote it.
func TestARowSaysWhetherItWroteAnAlias(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		row         string
		wantText    string
		wantAliased bool
	}{
		{name: "no alias", row: "- [[Lessons/a]]\n", wantText: "Lessons/a", wantAliased: false},
		{name: "an alias", row: "- [[Lessons/a|Words]]\n", wantText: "Words", wantAliased: true},
		{name: "an alias that repeats the target", row: "- [[a|a]]\n", wantText: "a", wantAliased: true},
		{name: "a heading is not an alias", row: "- [[a#Part]]\n", wantText: "a#Part", wantAliased: false},
		{name: "a separator with nothing after it", row: "- [[a|]]\n", wantText: "", wantAliased: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := Parse("## P {sequence=primary}\n\n"+tt.row, 1)
			entries := doc.Groups[0].entries()
			if len(entries) != 1 {
				t.Fatalf("row %q produced %d entries, want 1", tt.row, len(entries))
			}
			if got := entries[0].Text; got != tt.wantText {
				t.Errorf("row %q Text = %q, want %q", tt.row, got, tt.wantText)
			}
			if got := entries[0].Aliased; got != tt.wantAliased {
				t.Errorf("row %q Aliased = %v, want %v", tt.row, got, tt.wantAliased)
			}
		})
	}
}
