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
		{"text before the heading does not keep it", "Go 並行", "Intro.\n\n# Go 並行\n", true},
		{"an underlined heading is dropped", "Go 並行", "Go 並行\n=======\n\nText.\n", true},
		{"an indented heading is dropped", "Go 並行", "   # Go 並行\n\nText.\n", true},
		{"bold and code marks are not part of the words", "Go 並行 and code", "# **Go 並行** and `code`\n\nText.\n", true},
		{"a heading inside a fence is not the title", "Go 並行", "```\n# Go 並行\n```\n", false},
		{"only the first level-one heading is the title", "Go 並行", "# Slices\n\n# Go 並行\n", false},
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

func TestTheTitleHeadingLeavesTheRestOfTheBodyAsItWas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		title string
		body  string
		want  string
	}{
		{"a marked heading becomes a blank line", "T", "# T\n\ntext\n", "\n\ntext\n"},
		{"text before it is not joined to the text after", "T", "before\n# T\nafter\n", "before\n\nafter\n"},
		{"an underlined heading takes its underline with it", "T", "T\n===\ntext\n", "\ntext\n"},
		{"a two-line underlined heading takes both lines", "a b", "a\nb\n===\ntext\n", "\ntext\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, anchor, _, dropped := removeBodyFirstH1(tt.title, tt.body)
			if got != tt.want || dropped == 0 || anchor == "" {
				t.Errorf("removeBodyFirstH1(%q, %q) = %q, anchor %q, dropped %d; want %q", tt.title, tt.body, got, anchor, dropped, tt.want)
			}
		})
	}
}
