package note

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/snapshot"
)

func TestThoughtMarkdownContainsOnlyTheTypeTheDraftFlagAndTheSource(t *testing.T) {
	t.Parallel()
	for _, sourceType := range []string{"", "lesson"} {
		t.Run(sourceType, func(t *testing.T) {
			t.Parallel()
			source := &snapshot.Reading{
				RelPath: "Sources/Book.md", Type: sourceType, Domain: "japanese",
				Title: "Do not copy this title", Body: "Do not copy this prompt or answer",
			}
			got := thoughtMarkdown("writing", source, "Chapter one")
			const want = "---\ntype: \"writing\"\ndraft: true\nbased_on: \"[[Sources/Book#Chapter one]]\"\n---\n"
			if got != want {
				t.Errorf("thoughtMarkdown() = %q, want %q", got, want)
			}
		})
	}
}

// TestThoughtMarkdownStartsUnfinished pins the stub's one claim about the
// thought's progress: it starts as a draft, which is what puts it on the shelf
// of what the reader left open. The flag is the product's own, so it is
// written whatever the contract lists, and no status line is written.
func TestThoughtMarkdownStartsUnfinished(t *testing.T) {
	t.Parallel()
	got := thoughtMarkdown("writing", &snapshot.Reading{RelPath: "Source.md"}, "")
	if !strings.Contains(got, "\ndraft: true\n") {
		t.Errorf("thoughtMarkdown() = %q, want draft: true written", got)
	}
	if strings.Contains(got, "status:") {
		t.Errorf("thoughtMarkdown() = %q, want no status line", got)
	}
}

// TestThoughtMarkdownWritesNoTitleOrDomain holds the stub to the fields a note
// writes itself: the title is the name the file is saved under and the domain
// is the folder it lands in, so neither is a line of the frontmatter.
func TestThoughtMarkdownWritesNoTitleOrDomain(t *testing.T) {
	t.Parallel()
	got := thoughtMarkdown("writing", &snapshot.Reading{RelPath: "Source.md", Domain: "japanese", Title: "A title"}, "")
	for _, line := range []string{"title:", "domain:"} {
		if strings.Contains(got, line) {
			t.Errorf("thoughtMarkdown() = %q, want no %s line", got, line)
		}
	}
}

func TestThoughtMarkdownWritesHeadingTextNotItsID(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ heading, want string }{
		{"Chapter one", `[[Source#Chapter one]]`},
		{"日本語の 章", `[[Source#日本語の 章]]`},
		{"A | B # C [x] ^d", `[[Source#A B C x d]]`},
		{"", `[[Source]]`},
	} {
		got := thoughtMarkdown("writing", &snapshot.Reading{RelPath: "Source.md"}, tc.heading)
		if !strings.Contains(got, `based_on: "`+tc.want+`"`) {
			t.Errorf("heading %q: thoughtMarkdown() = %q, want based_on %s", tc.heading, got, tc.want)
		}
	}
}

func TestThoughtMarkdownQuotesSourceWithoutInjectingFrontmatter(t *testing.T) {
	t.Parallel()
	source := &snapshot.Reading{RelPath: "Sources/quote\" and & plus+.md"}
	got := thoughtMarkdown("writing", source, "")
	const want = "---\ntype: \"writing\"\ndraft: true\nbased_on: \"[[Sources/quote\\\" and & plus+]]\"\n---\n"
	if got != want {
		t.Errorf("thoughtMarkdown() = %q, want %q", got, want)
	}
}
