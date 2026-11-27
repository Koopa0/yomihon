package graph

import (
	"testing"

	"github.com/koopa0/yomihon/internal/vault"
)

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
		{"a heading's words are the ones a reader sees, closing marks aside", "Notes/a.md", "#   Spaced   **words**   ##\n", "Spaced words"},
		{"bold and code marks are not part of the title", "Notes/a.md", "# The **bold** `code` title\n", "The bold code title"},
		{"a link and a wikilink name their words", "Notes/a.md", "# [Docs](https://example.com) and [[Other|alias]]\n", "Docs and alias"},
		{"an escaped mark is the character it escapes", "Notes/a.md", "# Cost \\*not bold\\*\n", "Cost *not bold*"},
		{"an underlined heading over two lines is one title", "Notes/a.md", "First line\nsecond line\n===\n", "First line second line"},
		{"a CRLF file's underlined heading is the title", "Notes/a.md", "Underlined\r\n=====\r\n\r\nBody.\r\n", "Underlined"},
		{"an inline footnote is not part of the title", "Notes/a.md", "# Title^[inline footnote]\n", "Title"},
		{"a footnote reference is not part of the title", "Notes/a.md", "# Title[^1]\n\n[^1]: The note.\n", "Title"},
		{"a reference to no footnote is the text it was typed", "Notes/a.md", "# Title[^1]\n", "Title[^1]"},
		{"only the last closing run is marks", "Notes/a.md", "# a b #  #\n", "a b #"},
		{"a run that is the whole of the words is text", "Notes/a.md", "# #\n", "#"},
		{"an indented marked heading is read", "Notes/a.md", "   # Indented\n", "Indented"},
		{"a heading with no words is passed over", "Notes/a.md", "# \n\n# Second\n", "Second"},
		{"no heading falls back to the file name", "Notes/vault-check.md", "Just prose.\n", "vault-check"},
		{"a note with no body is called by its file", "Notes/deep/empty.md", "", "empty"},
		{"a path with no folder is its own file name", "root.md", "text\n", "root"},
		{"only the final extension comes off a dotted name", "Writing/note.ja.md", "text\n", "note.ja"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := TitleOf(&vault.Note{RelPath: tt.rel, Body: tt.body}); got != tt.want {
				t.Errorf("TitleOf(%q, %q) = %q, want %q", tt.rel, tt.body, got, tt.want)
			}
		})
	}
}

func TestTitleOfReadsTheDeclaredTitleBeforeTheFileName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		note vault.Note
		want string
	}{
		{"a heading outranks a declared title", vault.Note{RelPath: "a.md", Frontmatter: map[string]any{"title": "Declared"}, Body: "# Heading\n"}, "Heading"},
		{"a declared title outranks the file name", vault.Note{RelPath: "dir/a.md", Frontmatter: map[string]any{"title": "  Declared   title "}, Body: "Prose.\n"}, "Declared title"},
		{"a blank declared title falls through", vault.Note{RelPath: "dir/a.md", Frontmatter: map[string]any{"title": "  "}}, "a"},
		{"a declared title that is not text falls through", vault.Note{RelPath: "dir/a.md", Frontmatter: map[string]any{"title": 42}}, "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := TitleOf(&tt.note); got != tt.want {
				t.Errorf("TitleOf = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFirstTitleHeadingSaysWhichLinesItTakes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want TitleHeading
	}{
		{"a marked heading is one line", "intro\n\n# Title\n", TitleHeading{Words: "Title", Line: 2, Lines: 1}},
		{"an underlined heading takes its underline", "Title\n=====\n", TitleHeading{Words: "Title", Line: 0, Lines: 2}},
		{"a heading over two lines takes both and the underline", "a\nb\n===\n", TitleHeading{Words: "a b", Line: 0, Lines: 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := FirstTitleHeading(tt.body)
			if !ok || got != tt.want {
				t.Errorf("FirstTitleHeading(%q) = %+v, %v, want %+v", tt.body, got, ok, tt.want)
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
		{"a comment marker in a code span in the title is part of it", "# Use `%%` marks\n", "Use %% marks"},
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
			if got := TitleOf(&vault.Note{RelPath: "Notes/a.md", Body: tt.body}); got != tt.want {
				t.Errorf("TitleOf(%q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}

func TestTitleKeepsWhatACodeSpanHoldsLiterally(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, body, want string }{
		{"an entity in a code span is text", "# Use `&amp;` literally\n", "Use &amp; literally"},
		{"a backslash in a code span stays", "# Path `a\\*b` here\n", "Path a\\*b here"},
		{"an entity outside code resolves", "# Fish &amp; chips\n", "Fish & chips"},
		{"a block address is not part of the title", "# Title ^t-1\n", "Title"},
		{"an escape outside code resolves", "# Cost \\*each\\*\n", "Cost *each*"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := TitleOf(&vault.Note{RelPath: "a.md", Body: tt.body}); got != tt.want {
				t.Errorf("TitleOf(%q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}
