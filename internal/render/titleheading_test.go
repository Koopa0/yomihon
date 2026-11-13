package render

import "testing"

func TestTheHeadingATitleIsReadFromIsDroppedFromTheBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		title string
		body  string
		drops bool
	}{
		{"the heading and the title are the same words", "Go 並行", "# Go 並行\n\nText.\n", true},
		{"closing marks are not part of the words", "Go 並行", "# Go 並行 ##\n\nText.\n", true},
		{"a run of spaces reads as one", "Go 並行", "# Go   並行\n\nText.\n", true},
		{"a tab reads as one space", "Go 並行", "# Go\t並行\n\nText.\n", true},
		{"a different title keeps the heading", "Slices", "# Go 並行\n\nText.\n", false},
		{"only the first non-blank line qualifies", "Go 並行", "Intro.\n\n# Go 並行\n", false},
		{"a lesser heading is no title", "Go 並行", "## Go 並行\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := DropsTitleHeading(tt.title, tt.body); got != tt.drops {
				t.Errorf("DropsTitleHeading(%q, %q) = %t, want %t", tt.title, tt.body, got, tt.drops)
			}
		})
	}
}
