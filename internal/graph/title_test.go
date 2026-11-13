package graph

import "testing"

func TestTitleOf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rel  string
		body string
		want string
	}{
		{"the first level-one heading is the title", "Notes/a.md", "# What it says\n\nBody.\n", "What it says"},
		{"a heading after other prose still names the note", "Notes/a.md", "Intro.\n\n# The title\n", "The title"},
		{"lesser headings do not stand in for it", "Notes/a.md", "## Summary\n\ntext\n\n# The title\n", "The title"},
		{"the first of several level-one headings wins", "Notes/a.md", "# First\n\n# Second\n", "First"},
		{"a heading inside a code fence is code", "Notes/a.md", "```sh\n# not a title\n```\n\n# The real one\n", "The real one"},
		{"a heading inside an authored HTML block is not one", "Notes/a.md", "<!--\n# hidden\n-->\n\n# Shown\n", "Shown"},
		{"an underlined heading is a level-one heading", "Notes/a.md", "Underlined title\n================\n\nBody.\n", "Underlined title"},
		{"a heading's words travel as typed, closing marks aside", "Notes/a.md", "#   Spaced   **words**   ##\n", "Spaced **words**"},
		{"a heading with no words is passed over", "Notes/a.md", "# \n\n# Second\n", "Second"},
		{"no heading falls back to the file name", "Notes/vault-check.md", "Just prose.\n", "vault-check"},
		{"a note with no body is called by its file", "Notes/deep/empty.md", "", "empty"},
		{"a path with no folder is its own file name", "root.md", "text\n", "root"},
		{"only the final extension comes off a dotted name", "Writing/note.ja.md", "text\n", "note.ja"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := TitleOf(tt.rel, tt.body); got != tt.want {
				t.Errorf("TitleOf(%q, %q) = %q, want %q", tt.rel, tt.body, got, tt.want)
			}
		})
	}
}

func TestFirstH1ReportsWhetherTheBodyHasOne(t *testing.T) {
	t.Parallel()

	if got, ok := FirstH1("## only a lesser heading\n"); ok || got != "" {
		t.Errorf("FirstH1(lesser heading) = %q, %v, want no heading", got, ok)
	}
	if got, ok := FirstH1("# Found\n"); !ok || got != "Found" {
		t.Errorf("FirstH1(level one) = %q, %v, want Found", got, ok)
	}
}
