package render

import (
	"strings"
	"testing"
)

// TestAHeadingInsideASelfClosingRawTextBlockIsNotASection holds the half of the
// HTML-block table this package's other reader depends on. A self-closing
// <pre/> or <script/> opens a raw-text block that no blank line ends, so the
// lines after it are shown to a reader as the characters they typed. A section
// list naming a heading from inside one offers a place on the page that is not
// there: following it lands the reader somewhere else, and nothing tells them
// why.
//
// This scan and the callout scan read the same table for that reason. When they
// each kept their own, this one recognised no block at all for these two
// openers — neither element is in its list of the tags a blank line ends — and
// counted the heading.
func TestAHeadingInsideASelfClosingRawTextBlockIsNotASection(t *testing.T) {
	t.Parallel()

	for _, opener := range []string{"<pre/>", "<script/>"} {
		t.Run(opener, func(t *testing.T) {
			t.Parallel()

			body := opener + "\n\n# Heading after\n\nProse after that.\n"
			if slice, found := Excerpt(body, "heading-after"); found {
				t.Errorf("Excerpt() cut %q at a heading inside the block %s opens, which the page never closes", slice, opener)
			}
		})
	}
}

// A fence written four spaces in is an indented code example, not an opening.
// Read as one it never meets a closer, so the heading walk that finds a
// section treats the whole remainder of the note as code and can no longer
// find any heading below it — the page stops being able to open at the place
// a link names, while it goes on showing the heading.
func TestExcerptFindsAHeadingBelowAnIndentedFenceExample(t *testing.T) {
	t.Parallel()

	const below = "## Later Heading\n\ntext under it\n"
	for _, tt := range []struct {
		name string
		body string
	}{
		{name: "an indented backtick example", body: "# Title\n\n    ```\n\n" + below},
		{name: "an indented tilde example", body: "# Title\n\n    ~~~\n\n" + below},
		{name: "ordinary indented text, the control", body: "# Title\n\n    marker\n\n" + below},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			slice, found := Excerpt(tt.body, "later-heading")
			if !found {
				t.Fatalf("Excerpt() found no heading below the indented example")
			}
			if !strings.Contains(slice, "text under it") {
				t.Errorf("Excerpt() = %q, want the text under the heading", slice)
			}
		})
	}
}

// A note showing what a fence looks like writes the marker four spaces in,
// which CommonMark keeps inside the block. A walk that ends the fence there
// reads the code text as prose and the real closing marker as an opening, so
// the preview a reader opens before following a link offers the sections the
// page does not have and refuses the one it does.
func TestExcerptReadsTheSectionsTheFenceReallyLeaves(t *testing.T) {
	t.Parallel()

	const body = "```\n    ```\n## Code Example\n```\n\n## Real Heading\nreal section body\n"

	slice, found := Excerpt(body, "real-heading")
	if !found {
		t.Fatalf("Excerpt() cannot find the one heading the page shows")
	}
	if !strings.Contains(slice, "real section body") {
		t.Errorf("Excerpt() = %q, want the text under the heading", slice)
	}
	if strings.Contains(slice, "## Code Example") {
		t.Errorf("Excerpt() = %q, want no code text from inside the fence", slice)
	}
	if _, found := Excerpt(body, "code-example"); found {
		t.Error("Excerpt() offers a section made of code text; the page has no such heading")
	}
}

// A course prints its map note's own opening under the title. The table below
// is that cut: the words above the first heading; for a note that opens on its
// own title, the prose under it; nothing where the note opens on any other
// heading; and the whole of a note that carries no heading at all.
func TestOpeningIsWhatTheNoteSaysItIsBeforeItListsAnything(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "prose above the first heading",
			body: "What this course is.\n\n## Part One\n\n- [[L01]]\n",
			want: "What this course is.\n",
		},
		{
			name: "several paragraphs, all of them",
			body: "What this course is.\n\nWho it is for.\n\n## Part One\n",
			want: "What this course is.\n\nWho it is for.\n",
		},
		{
			// The note's own preface: its title stands above it and is printed
			// by whoever shows the opening, so the words under it are what is
			// left to say what the note is.
			name: "a note that opens on its title opens with the prose under it",
			body: "# Title\n\nUnder the title.\n\nAnd who it is for.\n\n## Part One\n",
			want: "Under the title.\n\nAnd who it is for.\n",
		},
		{
			name: "the preface of a title runs to the end of a note with no other heading",
			body: "# Title\n\nOnly this.\n",
			want: "Only this.\n",
		},
		{
			name: "any heading ends the preface, not only one of the same level",
			body: "# Title\n\nUnder the title.\n\n### Deeper\n\nNot the preface.\n",
			want: "Under the title.\n",
		},
		{
			name: "a second title ends the preface too",
			body: "# One\n\nFirst.\n\n# Two\n\nSecond.\n",
			want: "First.\n",
		},
		{
			name: "a title followed at once by another heading has no preface",
			body: "# Title\n\n## Part One\n\n- [[L01]]\n",
			want: "",
		},
		{
			name: "a title with nothing under it has no preface",
			body: "# Title\n",
			want: "",
		},
		{
			// The underline is part of the heading and never of what follows.
			name: "an underlined title keeps its underline out of the preface",
			body: "Title\n=====\n\nUnder the title.\n\n## Part One\n",
			want: "Under the title.\n",
		},
		{
			name: "a title that wraps over two lines keeps all of it out of the preface",
			body: "A long\ntitle\n=====\n\nUnder the title.\n\n## Part One\n",
			want: "Under the title.\n",
		},
		{
			name: "a comment under the title comes off",
			body: "# Title\n\nShown %%hidden%% and shown.\n\n## Part One\n",
			want: "Shown  and shown.\n",
		},
		{
			// Only a title earns the second reading: a note that opens on a
			// part has gone straight to what it lists.
			name: "a note that opens on a level-2 heading has no opening",
			body: "## Part One\n\nUnder the part.\n\n- [[L01]]\n",
			want: "",
		},
		{
			name: "words above the title are still the opening",
			body: "Above.\n\n# Title\n\nUnder.\n\n## Part One\n",
			want: "Above.\n",
		},
		{
			name: "blank lines before that heading are not an opening",
			body: "\n\n## Part One\n\n- [[L01]]\n",
			want: "",
		},
		{
			name: "a note with no heading at all is all opening",
			body: "Nothing here declares a course.\n",
			want: "Nothing here declares a course.\n",
		},
		{
			name: "an empty note opens with nothing",
			body: "\n \n",
			want: "",
		},
		{
			name: "a comment in the opening comes off",
			body: "Shown %%hidden%% and shown.\n\n## Part One\n",
			want: "Shown  and shown.\n",
		},
		{
			// A heading written inside a fence is code the author is showing,
			// not the place the note's own structure begins.
			name: "a heading inside a fence does not end the opening",
			body: "Before.\n\n```\n## Not a heading\n```\n\nAfter.\n\n## Part One\n",
			want: "Before.\n\n```\n## Not a heading\n```\n\nAfter.\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Opening(tt.body); got != tt.want {
				t.Errorf("Opening() = %q, want %q", got, tt.want)
			}
		})
	}
}

// The hover card's lede and a course cover are one cut wherever the note has an
// opening of its own. Two readings of that would drift, and a reader who saw
// the card and then opened the course would be shown two different openings of
// one note.
func TestTheHoverCardsLedeIsTheSameOpening(t *testing.T) {
	t.Parallel()

	const body = "What this course is.\n\n## Part One\n\n- [[L01]]\n"
	opening := Opening(body)
	if opening == "" {
		t.Fatalf("Opening() found nothing in %q, so the comparison below proves nothing", body)
	}
	lede, found, narrowed := ExcerptPreview(body, "")
	if !found {
		t.Fatalf("ExcerptPreview() found no lede")
	}
	if !narrowed {
		t.Errorf("ExcerptPreview() reports nothing left behind, but the note goes on after its opening")
	}
	if lede != opening {
		t.Errorf("the card cuts %q and the cover cuts %q; they are meant to be one cut", lede, opening)
	}
}
