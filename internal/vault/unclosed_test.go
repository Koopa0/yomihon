package vault_test

import (
	"testing"

	"github.com/koopa0/yomihon/internal/vault"
)

// TestAnOpeningFenceNothingClosesIsRecognised holds the question asked beside
// the split: of bytes that came back with no frontmatter block, which ones
// plainly meant to open one. The opening is a "---" line and the line after it
// is shaped like a field, and the split found no closing line anywhere.
//
// The cases that answer false are as much of the contract as the ones that
// answer true. A note may legitimately open with a thematic break, and calling
// that a lost fence would put a notice on a page that has nothing wrong with it.
func TestAnOpeningFenceNothingClosesIsRecognised(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{name: "the closing fence lost a dash", content: "---\ntitle: Unclosed\ntype: note\nstatus: draft\n--\n\n# Body\n", want: true},
		{name: "the closing fence is missing altogether", content: "---\ntitle: Unclosed\ntype: note\n\n# Body\n", want: true},
		{name: "nothing follows the first field", content: "---\ntitle: Unclosed\n", want: true},
		{name: "the first field is the end of the file", content: "---\ntitle: Unclosed", want: true},
		{name: "a field with no value yet", content: "---\ntitle:\n", want: true},
		{name: "a field name with an underscore", content: "---\nsource_kind: course\n", want: true},
		{name: "a field name with a hyphen", content: "---\nsource-kind: course\n", want: true},
		{name: "a field name in Chinese", content: "---\n標題: 未結尾\n", want: true},
		{name: "a tab after the colon", content: "---\ntitle:\tUnclosed\n", want: true},
		{name: "carriage return and line feed endings", content: "---\r\ntitle: Unclosed\r\ntype: note\r\n--\r\n\r\n# Body\r\n", want: true},
		{name: "a byte-order mark before the fence", content: "\xef\xbb\xbf---\ntitle: Unclosed\n--\n", want: true},
		{name: "a closing fence cut down to one dash", content: "---\ntitle: Unclosed\n-\nbody\n", want: true},

		{name: "a block that closes", content: "---\ntitle: Closed\n---\nbody\n", want: false},
		{name: "a block that closes with dots", content: "---\ntitle: Closed\n...\nbody\n", want: false},
		{name: "a block whose closing fence ends the file", content: "---\ntitle: Closed\n---", want: false},
		{name: "a block that closes after carriage return and line feed", content: "---\r\ntitle: Closed\r\n---\r\nbody\r\n", want: false},
		{name: "an empty fence pair", content: "---\n---\nbody\n", want: false},
		{name: "a fence pair around only a comment", content: "---\n# nothing here\n---\nbody\n", want: false},
		{name: "a block whose first line is not a field", content: "---\n# nothing here\ntitle: x\n", want: false},

		{name: "a thematic break and a blank line", content: "---\n\nA note that opens with a rule.\n", want: false},
		{name: "a thematic break and a heading", content: "---\n# Heading\n\nbody\n", want: false},
		{name: "a thematic break and a sentence", content: "---\nJust a sentence after the rule.\n", want: false},
		{name: "a thematic break and a sentence with a colon in it", content: "---\nA sentence with a colon: and more.\n", want: false},
		{name: "a thematic break and an address", content: "---\nhttps://example.com/path\n", want: false},
		{name: "a thematic break and a list item", content: "---\n- title: x\n", want: false},
		{name: "a thematic break and an indented line", content: "---\n  title: x\n", want: false},
		{name: "a colon straight into text", content: "---\ntitle:x\n", want: false},
		{name: "a line that is only a colon", content: "---\n: x\n", want: false},
		{name: "a name that starts with a digit", content: "---\n2026: x\n", want: false},
		{name: "a name with a space in it", content: "---\nnote to self: x\n", want: false},
		{name: "a name that starts with a hyphen", content: "---\n-title: x\n", want: false},
		{name: "a quoted name", content: "---\n\"title\": x\n", want: false},

		{name: "an opening fence with trailing space", content: "--- \ntitle: x\n", want: false},
		{name: "four dashes", content: "----\ntitle: x\n", want: false},
		{name: "a fence of carriage returns only", content: "---\rtitle: x\r", want: false},
		{name: "the fence on the second line", content: "\n---\ntitle: x\n", want: false},
		{name: "the fence alone", content: "---\n", want: false},
		{name: "the fence alone without a newline", content: "---", want: false},
		{name: "no fence at all", content: "title: x\n", want: false},
		{name: "an empty file", content: "", want: false},
		{name: "invalid text after a fence", content: "---\n\xff: x\n", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := vault.OpensUnclosedFrontmatter([]byte(tt.content)); got != tt.want {
				t.Errorf("OpensUnclosedFrontmatter(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}

// TestTheUnclosedFenceIsTheSplitsOwnVerdictAndNothingMore pins the two things
// that make detecting beside the split safe. It never calls a block unclosed
// that the split closed, so the notice a page draws cannot contradict the
// frontmatter the same page was read from. And asking the question changes
// nothing the split or the status write path answer: the same bytes still have
// no block and no status line to rewrite.
func TestTheUnclosedFenceIsTheSplitsOwnVerdictAndNothingMore(t *testing.T) {
	t.Parallel()

	for _, content := range []string{
		"---\ntitle: Unclosed\ntype: note\nstatus: draft\n--\n\n# Body\n",
		"---\r\ntitle: Unclosed\r\nstatus: draft\r\n",
		"\xef\xbb\xbf---\ntitle: Unclosed\nstatus: draft\n",
		"---\ntitle: Closed\nstatus: draft\n---\nbody\n",
		"---\ntitle: Closed\nstatus: draft\n...\nbody\n",
		"---\n\nA note that opens with a rule.\n",
	} {
		data := []byte(content)
		_, closed := vault.SplitFrontmatter(data)
		unclosed := vault.OpensUnclosedFrontmatter(data)
		if closed && unclosed {
			t.Errorf("OpensUnclosedFrontmatter(%q) = true for a block SplitFrontmatter closed", content)
		}

		_, _, hasStatusLine := vault.StatusLineSpan(data)
		if unclosed && hasStatusLine {
			t.Errorf("StatusLineSpan(%q) found a status line in a block that never closes; the write path must keep reading it as body text", content)
		}
		if unclosed && vault.Parse("note.md", data).HasFrontmatter {
			t.Errorf("Parse(%q) read a frontmatter block from a fence that never closes", content)
		}
	}
}
