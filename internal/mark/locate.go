package mark

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Status is where a mark stands in a note as the note is now.
type Status int

// The standings a mark can have. A mark that is Changed is still the reader's:
// it is kept and shown as such, never dropped, because it is the note that no
// longer says where the words it was about are, not the mark that is at fault.
const (
	// Found means the words the reader chose are in the note, at one place, or
	// a mark of a whole section has its section.
	Found Status = iota + 1
	// SectionOnly means the mark names a section or the note as a whole, not a
	// span of text, and the section is there.
	SectionOnly
	// Changed means the words are not in the note and what stood around them
	// does not place them, or they are at several places that nothing tells
	// apart, or the section a mark of a section names is gone.
	Changed
)

// Placement is the answer Locate gives.
type Placement struct {
	Status Status
	// Start and End are byte offsets into the text Locate was given, of the
	// words the mark is now on. They are set only when Status is Found and the
	// mark is of a span of text.
	Start, End int
	// Exact is true when the words themselves were found. It is false when
	// only what stood on either side of them was, so the span between is what
	// the author has put there since.
	Exact bool
	// SectionKept is whether the anchor the mark was made under is still among
	// the note's. A Changed mark whose section is kept is drawn at that
	// section's heading, and one whose section is gone is gathered at the end
	// of the note.
	SectionKept bool
}

// contextSteps are how much of the context on each side is asked for, nearest
// the words first, longest first. An author who edits a sentence that stood a
// paragraph away leaves the sentences beside the words as they were.
var contextSteps = [...]int{maxContextBytes, 96, 32}

// Locate says where a mark is in text, the note's words as a reader sees them,
// and sections, the anchor ids the note has now. It takes plain text from the
// caller, so what it knows of a note is only what the caller hands it.
//
// A mark of a whole section has no words, so it stands where its section does.
// A mark of a span is found by its words first. Where they occur more than once
// the one with the most of the same words around it wins. Where several have as
// much, nothing tells them apart and the answer is Changed, with no place
// chosen among them. Where the words do not occur, the span between the words
// that stood before and after them is found instead, when exactly one such span
// is near enough to be an edit of them: a wrong place is worse than none, so an
// unclear answer is Changed. Runs of white space are one space throughout,
// because a selection and the page it came from break lines differently.
//
// A nil mark is Changed.
func Locate(text string, sections []string, m *Mark) Placement {
	if m == nil {
		return Placement{Status: Changed}
	}
	kept := m.Anchor == "" || slices.Contains(sections, m.Anchor)
	quote := squeezed(m.Quote)
	if quote == "" {
		if kept {
			return Placement{Status: SectionOnly, SectionKept: true}
		}
		return Placement{Status: Changed}
	}
	v := newView(text)
	if start, end, found, alone := v.exact(quote, squeezed(m.Before), squeezed(m.After)); found {
		if !alone {
			return Placement{Status: Changed, SectionKept: kept}
		}
		return v.placed(start, end, true, kept)
	}
	if start, end, ok := v.between(quote, squeezed(m.Before), squeezed(m.After)); ok {
		return v.placed(start, end, false, kept)
	}
	return Placement{Status: Changed, SectionKept: kept}
}

// view is a note's text with every run of white space made one space, and where
// each byte of that came from in the text the caller gave.
type view struct {
	text string
	// from[i] is the offset in the caller's text of byte i of text, and the last
	// entry is the length of the caller's text.
	from []int
}

func newView(text string) *view {
	var b strings.Builder
	b.Grow(len(text))
	from := make([]int, 0, len(text)+1)
	space := false
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if unicode.IsSpace(r) {
			if !space {
				b.WriteByte(' ')
				from = append(from, i)
				space = true
			}
			i += size
			continue
		}
		space = false
		b.WriteString(text[i : i+size])
		for k := range size {
			from = append(from, i+k)
		}
		i += size
	}
	return &view{text: b.String(), from: append(from, len(text))}
}

// squeezed is s with every run of white space made one space and none at either
// end, which is how a view is searched.
func squeezed(s string) string {
	return strings.TrimSpace(newView(s).text)
}

func (v *view) placed(start, end int, exact, kept bool) Placement {
	return Placement{Status: Found, Start: v.from[start], End: v.from[end], Exact: exact, SectionKept: kept}
}

// exact is the place in the view where quote occurs with the most of before
// and after around it. found is false where quote does not occur, and alone is
// false where another place has as much around it, so that start and end are
// one of several the same.
func (v *view) exact(quote, before, after string) (start, end int, found, alone bool) {
	best := -1
	for from := 0; ; {
		at := strings.Index(v.text[from:], quote)
		if at < 0 {
			return start, end, found, alone
		}
		s := from + at
		e := s + len(quote)
		score := commonSuffix(strings.TrimRight(v.text[:s], " "), before) + commonPrefix(strings.TrimLeft(v.text[e:], " "), after)
		switch {
		case score > best:
			best, start, end, found, alone = score, s, e, true, true
		case score == best:
			alone = false
		}
		from = s + 1
	}
}

// between is the span the words stood in when what stood on both sides of them
// is still in the view with a little between, and false when that is not so
// for exactly one place. It tries the whole of the context, then less of it.
func (v *view) between(quote, before, after string) (start, end int, ok bool) {
	if before == "" || after == "" {
		return 0, 0, false
	}
	reach := 3*len(quote) + 128
	for _, step := range contextSteps {
		b, a := tail(before, step), head(after, step)
		if start, end, ok = v.span(b, a, reach); ok {
			for v.text[start] == ' ' {
				start++
			}
			for v.text[end-1] == ' ' {
				end--
			}
			return start, end, true
		}
	}
	return 0, 0, false
}

// span is the one place a stands after b with at least a byte of words and at
// most reach bytes between them, and false when there is none or more than one:
// two copies of what stood around the words are two guesses.
func (v *view) span(b, a string, reach int) (start, end int, ok bool) {
	if b == "" || a == "" {
		return 0, 0, false
	}
	for from := 0; ; {
		at := strings.Index(v.text[from:], b)
		if at < 0 {
			return start, end, ok
		}
		s := from + at + len(b)
		from += at + 1
		gap := strings.Index(v.text[s:min(len(v.text), s+reach+len(a)+1)], a)
		if gap < 1 || gap > reach || strings.TrimSpace(v.text[s:s+gap]) == "" {
			continue
		}
		if ok {
			return 0, 0, false
		}
		start, end, ok = s, s+gap, true
	}
}

// commonSuffix is how many bytes a and b end in the same way.
func commonSuffix(a, b string) int {
	n := 0
	for n < len(a) && n < len(b) && a[len(a)-1-n] == b[len(b)-1-n] {
		n++
	}
	return n
}

// commonPrefix is how many bytes a and b begin in the same way.
func commonPrefix(a, b string) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

// tail is the last n bytes of s, begun at a character.
func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[len(s)-n:]
	for s != "" && !utf8.RuneStart(s[0]) {
		s = s[1:]
	}
	return strings.TrimLeft(s, " ")
}

// head is the first n bytes of s, ended at a character.
func head(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for s != "" && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return strings.TrimRight(s, " ")
}
