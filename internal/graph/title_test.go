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

// TestTitleSkipsWhatTheReaderNeverSees holds the title to the text a reader is
// shown: a heading-shaped line inside an Obsidian comment or a math block is
// not the note's title, and a comment or math marker the page would not treat
// as one (in code, or with no partner) hides nothing.
func TestTitleSkipsWhatTheReaderNeverSees(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{"a heading inside a comment is hidden", "%%\n# secret draft heading\n%%\n\n# Real Title\n", "Real Title"},
		{"a comment closed on its own line", "%% # hidden %%\n# Real Title\n", "Real Title"},
		{"a comment opened mid-line runs over the lines it covers", "Intro %% start\n# hidden\nend %%\n# Real Title\n", "Real Title"},
		{"words after the closer on its line stay visible", "%%\nhidden\n%% # Real Title\n", "Real Title"},
		{"a comment inside the heading line is not part of the title", "# Real %% a note %% Title\n", "Real Title"},
		{"a comment that never closes hides what follows it", "# Real Title\n%% never closed\n# Hidden\n", "Real Title"},
		{"a comment that never closes before any heading leaves the file name", "%% never closed\n# Hidden\n", "a"},
		{"an underlined heading inside a comment is hidden", "%%\nHidden\n======\n%%\n# Real Title\n", "Real Title"},
		{"an underlined heading right after a comment is read", "%% a note %%\nUnderlined\n=========\n", "Underlined"},
		{"the first heading outside a comment wins over later ones", "%%\n# A\n%%\n# B\n# C\n", "B"},
		{"an authored HTML comment opened mid-line hides its lines", "text <!--\n# hidden\n-->\n# Real Title\n", "Real Title"},
		{"a comment marker inside a code fence is code", "```\n%%\n```\n\n# Real Title\n", "Real Title"},
		{"a comment marker in a code span on a prose line is code", "Use `%%` to comment.\n\n# Real Title\n", "Real Title"},
		{"a comment marker in a code span in the title is part of it", "# Use `%%` marks\n", "Use `%%` marks"},
		{"a heading inside a math block is hidden", "$$\n# not heading\n$$\n\n# Real Title\n", "Real Title"},
		{"a one-line math block hides nothing after it", "Cost $$x$$ here.\n\n# Real Title\n", "Real Title"},
		{"a math marker with no partner is text", "$$ only once\n\n# Real Title\n", "Real Title"},
		{"a math marker after the title with no partner is text", "# Real Title\n$$ x\n", "Real Title"},
		{"a math marker in a code fence does not pair with one outside", "```\n$$\n```\n\n# Real Title\n\nprice $$ 5\n", "Real Title"},
		{"an escaped dollar pair is text", "Costs \\$$5\n# Real Title\nmore $$ x\n", "Real Title"},
		{"a comment inside a math block adds nothing to hide", "$$ a %% b %% c\n# hidden\n$$\n# Real Title\n", "Real Title"},
		{"a math marker in a comment does not open a block", "%% $$ %%\n# Real Title\n\n$$\nx\n$$\n", "Real Title"},
		{"a heading in a code fence is still code", "~~~\n# not a title\n~~~\n# Real Title\n", "Real Title"},
		{"a heading in an authored HTML block is still not one", "<div>\n# not a title\n</div>\n\n# Real Title\n", "Real Title"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := TitleOf("Notes/a.md", tt.body); got != tt.want {
				t.Errorf("TitleOf(%q) = %q, want %q", tt.body, got, tt.want)
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
	if got, ok := FirstH1("%%\n# hidden\n%%\n"); ok || got != "" {
		t.Errorf("FirstH1(a heading only inside a comment) = %q, %v, want no heading", got, ok)
	}
}
