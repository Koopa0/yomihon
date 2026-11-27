package mark_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/mark/machinemarks"
)

// aSpan is a mark of some words, with a little of what stood on either side.
func aSpan(quote, before, after string) *machinemarks.Mark {
	return &machinemarks.Mark{Kind: machinemarks.Highlight, Path: "Notes/source.md", Anchor: "first", Quote: quote, Before: before, After: after}
}

// words is what the placement is on, for a placement that found some.
func words(text string, got mark.Placement) string {
	return text[got.Start:got.End]
}

func TestAMarkOfASectionStandsWhereItsSectionDoes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		anchor string
		want   mark.Placement
	}{
		{"a section that is there", "first", mark.Placement{Status: mark.SectionOnly, SectionKept: true}},
		{"the note as a whole", "", mark.Placement{Status: mark.SectionOnly, SectionKept: true}},
		{"a section that is gone", "removed", mark.Placement{Status: mark.Changed}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			question := &machinemarks.Mark{Kind: machinemarks.Question, Path: "Notes/source.md", Anchor: tt.anchor}
			got := mark.Locate("some text", []string{"first", "second"}, question)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Locate (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAMarkOfWordsIsFoundWhereTheWordsAre(t *testing.T) {
	t.Parallel()
	text := "A channel hands a value from one goroutine to another.\n\nA send blocks until a receiver is ready."
	got := mark.Locate(text, []string{"first"}, aSpan("A send blocks", "", ""))
	if got.Status != mark.Found || !got.Exact || !got.SectionKept {
		t.Fatalf("Locate = %+v, want found, exact, with its section", got)
	}
	if want := "A send blocks"; words(text, got) != want {
		t.Errorf("the words are %q, want %q", words(text, got), want)
	}
}

// A selection and the page it came from break lines in different places, and a
// reader's words are found all the same, at the bytes of the text that was given.
func TestWhiteSpaceDoesNotHideTheWords(t *testing.T) {
	t.Parallel()
	text := "Before   it\nstarts,\n\n  a channel\thands  a value\nover."
	got := mark.Locate(text, nil, &machinemarks.Mark{Kind: machinemarks.Highlight, Path: "a.md", Quote: "a channel hands a value"})
	if got.Status != mark.Found || !got.Exact {
		t.Fatalf("Locate = %+v", got)
	}
	if want := "a channel\thands  a value"; words(text, got) != want {
		t.Errorf("the words are %q, want %q", words(text, got), want)
	}
}

func TestWordsInAnyScriptAreFoundAtTheirOwnBytes(t *testing.T) {
	t.Parallel()
	text := "通道把一個值從一個協程交給另一個。\n\n傳送會等到有接收者才繼續。"
	got := mark.Locate(text, nil, &machinemarks.Mark{Kind: machinemarks.Highlight, Path: "a.md", Quote: "傳送會等到有接收者"})
	if got.Status != mark.Found || !got.Exact || words(text, got) != "傳送會等到有接收者" {
		t.Errorf("Locate = %+v, words %q", got, text[got.Start:got.End])
	}
}

// Of several places the same words are, the one with the most of the same
// words around it is the mark's.
func TestWordsThatOccurMoreThanOnceAreToldApartByWhatSurroundsThem(t *testing.T) {
	t.Parallel()
	text := "The cache is cold. See the note. Later: the cache is warm. See the table."
	first := strings.Index(text, "See the note")
	second := strings.Index(text, "See the table")
	tests := []struct {
		name          string
		before, after string
		want          int
	}{
		{"before the first", "The cache is cold.", " the note.", first},
		{"before the second", "Later: the cache is warm.", " the table.", second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := mark.Locate(text, nil, aSpan("See", tt.before, tt.after))
			if got.Status != mark.Found || got.Start != tt.want {
				t.Errorf("Locate = %+v, want found at %d", got, tt.want)
			}
		})
	}
}

// A wrong place is worse than none: words that occur at several places, with
// nothing around them that tells the places apart, are not placed at one of them.
func TestAQuoteAtSeveralPlacesNothingTellsApartIsNotPlaced(t *testing.T) {
	t.Parallel()
	// The shape of a comparison table: one cell value in the same column of five rows.
	cell := func(row string) string { return "| " + row + " | 仍可能 |\n" }
	table := "比較\n" + cell("甲") + cell("乙") + cell("丙") + cell("丁") + cell("戊")
	const prose = "The cache is cold. See the note. Later: the cache is warm. See the table."
	// Three places say "A:" or "B:" before a word; two of them say the same.
	const labelled = "A: See. B: See. B: See."
	// A word the author doubled: each of its two places has as much around it as
	// the other, and what stood around it is still there, one stretch between.
	const doubled = "A send on a channel blocks blocks until a receiver is ready."
	nth := func(text, sub string, n int) int {
		at := -1
		for range n {
			at += strings.Index(text[at+1:], sub) + 1
		}
		return at
	}
	tests := []struct {
		name          string
		text          string
		quote         string
		before, after string
		sections      []string
		want          mark.Placement
	}{
		{"no context, as a quote read back from a file has", table, "仍可能", "", "", []string{"first"}, mark.Placement{Status: mark.Changed, SectionKept: true}},
		{"context every place shares", table, "仍可能", " | ", " |", []string{"first"}, mark.Placement{Status: mark.Changed, SectionKept: true}},
		{"prose, with nothing around the words", prose, "See", "", "", []string{"first"}, mark.Placement{Status: mark.Changed, SectionKept: true}},
		{"the section is gone as well", table, "仍可能", "", "", nil, mark.Placement{Status: mark.Changed}},
		{"two places share the most of what stood around the words", labelled, "See", "B:", "", []string{"first"}, mark.Placement{Status: mark.Changed, SectionKept: true}},
		{"the words twice in a row, with what stood around them still there", doubled, "blocks", "A send on a channel", "until a receiver is", []string{"first"}, mark.Placement{Status: mark.Changed, SectionKept: true}},
		{
			"the last of five, told apart by its row", table, "仍可能", "戊 |", "|", []string{"first"},
			mark.Placement{Status: mark.Found, Start: nth(table, "仍可能", 5), End: nth(table, "仍可能", 5) + len("仍可能"), Exact: true, SectionKept: true},
		},
		{
			"the second of five, told apart by its row", table, "仍可能", "乙 |", "|", []string{"first"},
			mark.Placement{Status: mark.Found, Start: nth(table, "仍可能", 2), End: nth(table, "仍可能", 2) + len("仍可能"), Exact: true, SectionKept: true},
		},
		{
			"one place has the most of what stood around the words, the others are alike", labelled, "See", "A:", "", []string{"first"},
			mark.Placement{Status: mark.Found, Start: nth(labelled, "See", 1), End: nth(labelled, "See", 1) + len("See"), Exact: true, SectionKept: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := mark.Locate(tt.text, tt.sections, aSpan(tt.quote, tt.before, tt.after))
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Locate (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEditedWordsAreFoundBetweenWhatStoodAroundThem(t *testing.T) {
	t.Parallel()
	const (
		before = "A send on a channel"
		after  = "until a receiver is ready."
	)
	tests := []struct {
		name  string
		text  string
		words string
	}{
		{"shorter", before + " waits " + after, "waits"},
		{"longer", before + " pauses the sender and waits " + after, "pauses the sender and waits"},
		{"the same length", before + " stalls " + after, "stalls"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := mark.Locate(tt.text, []string{"first"}, aSpan("blocks", before, after))
			if got.Status != mark.Found || got.Exact {
				t.Fatalf("Locate = %+v", got)
			}
			if words(tt.text, got) != tt.words {
				t.Errorf("the words are %q, want %q", words(tt.text, got), tt.words)
			}
		})
	}
}

func TestEditedWordsAreFoundWhenOnlyTheNearestOfTheContextRemains(t *testing.T) {
	t.Parallel()
	before := "An old opening that has been rewritten since. " + strings.Repeat("The rest of the paragraph stays put. ", 3) + "Then a send on a channel"
	after := "until a receiver is ready, and a long closing that was also rewritten since."
	text := "A new opening altogether. " + strings.Repeat("The rest of the paragraph stays put. ", 3) + "Then a send on a channel waits until a receiver is ready, and a different closing."
	got := mark.Locate(text, nil, aSpan("blocks", before, after))
	if got.Status != mark.Found || got.Exact || words(text, got) != "waits" {
		t.Errorf("Locate = %+v, words %q", got, text[got.Start:got.End])
	}
}

// A mark that cannot be placed is kept and flagged. It is Changed when the
// words are gone and nothing safe says where they went, and what is not known
// is never guessed.
func TestAMarkThatCannotBePlacedIsChanged(t *testing.T) {
	t.Parallel()
	const (
		before = "A send on a channel"
		after  = "until a receiver is ready."
	)
	tests := []struct {
		name          string
		text          string
		before, after string
	}{
		{"the words were deleted", before + " " + after, before, after},
		{"the words and one side were rewritten", "A send on a queue waits " + after, before, after},
		{"the words and the other side were rewritten", before + " waits until somebody listens.", before, after},
		{"there was no context to go by", "A send on a channel waits until a receiver is ready.", "", ""},
		{"only one side of the context was kept", before + " waits until a receiver is ready.", before, ""},
		{"what is between them is too long to be an edit", before + " " + strings.Repeat("an essay ", 60) + after, before, after},
		{"two places have what stood around the words", before + " waits " + after + "\n" + before + " yields now " + after, before, after},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := mark.Locate(tt.text, []string{"first"}, aSpan("blocks", tt.before, tt.after))
			if got.Status != mark.Changed || !got.SectionKept {
				t.Errorf("Locate = %+v, want changed with its section kept", got)
			}
		})
	}
}

func TestAChangedMarkSaysWhetherItsSectionIsStillThere(t *testing.T) {
	t.Parallel()
	for _, kept := range []bool{true, false} {
		sections := []string{"second"}
		if kept {
			sections = []string{"first"}
		}
		got := mark.Locate("nothing here now", sections, aSpan("blocks", "", ""))
		if got.Status != mark.Changed || got.SectionKept != kept {
			t.Errorf("section kept = %v: Locate = %+v", kept, got)
		}
	}
}

func TestANilMarkIsChanged(t *testing.T) {
	t.Parallel()
	if got := mark.Locate("text", nil, nil); got.Status != mark.Changed {
		t.Errorf("Locate(nil) = %+v, want changed", got)
	}
}

// Text that is not valid UTF-8 is a note somebody saved badly, and is no reason
// to lose a mark or its place.
func TestInvalidUTF8InTheTextIsKeptAsItIs(t *testing.T) {
	t.Parallel()
	text := "before \xff\xfe the words \xc3 after"
	got := mark.Locate(text, nil, &machinemarks.Mark{Kind: machinemarks.Highlight, Path: "a.md", Quote: "the words"})
	if got.Status != mark.Found || words(text, got) != "the words" {
		t.Errorf("Locate = %+v", got)
	}
}

// Every stretch of a note, marked with the words that really stood around it, is
// found where it was made, whatever else in the note looks like it.
func TestEveryStretchOfANoteIsFoundWhereItWasMade(t *testing.T) {
	t.Parallel()
	text := "Go starts a goroutine with the go keyword.\n\n" +
		"A channel is typed; a send blocks until a receiver is ready, and a receive blocks until a value is sent.\n\n" +
		"通道有型別；傳送會等到有接收者，接收會等到有值送來。\n\n" +
		"Closing a channel tells receivers that no more values will come; a send on a closed channel panics."
	runes := []rune(text)
	const quoteLength, context = 12, 24
	for i := 0; i+quoteLength <= len(runes); i += 3 {
		quote := string(runes[i : i+quoteLength])
		if strings.TrimSpace(quote) == "" {
			continue
		}
		start := len(string(runes[:i])) + len(quote) - len(strings.TrimLeft(quote, " \n"))
		before := string(runes[max(0, i-context):i])
		after := string(runes[i+quoteLength : min(len(runes), i+quoteLength+context)])
		got := mark.Locate(text, []string{"first"}, aSpan(quote, before, after))
		if got.Status != mark.Found || !got.Exact || got.Start != start || words(text, got) != strings.TrimSpace(quote) {
			t.Fatalf("%q at %d: Locate = %+v, words %q", quote, start, got, text[got.Start:got.End])
		}
	}
}
