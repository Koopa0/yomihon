package machinemarks

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/mark/sealedline"
	"github.com/koopa0/yomihon/internal/vault"
)

// Format is the one line format this version reads and writes. A line that
// says a larger one is kept where it is and is not read.
const Format = 1

// The kinds a mark is in. They are one mark in three states: the reader turns a
// highlight into a thought or a question, and back, and the mark keeps its id.
const (
	Highlight Kind = "highlight"
	Thought   Kind = "thought"
	Question  Kind = "question"
)

// Kind is the state a mark is in.
type Kind string

// Mark is the whole value of one mark, as one line of a machine file holds it.
// A change of state, a new word, a resolved question or a deletion is a new
// whole value under the same ID.
type Mark struct {
	// ID names the mark for as long as it exists: 32 lowercase hexadecimal
	// digits.
	ID   string `json:"id"`
	Kind Kind   `json:"kind"`
	// Path is the note, as the product spells it: a vault-relative slash path.
	Path   string `json:"path"`
	Anchor string `json:"anchor,omitempty"`
	// Quote is the words the reader chose, and Before and After a little of
	// what stood on either side. A mark of a whole section has none of the
	// three, and so is never a highlight: a highlight is a passage.
	Quote    string `json:"quote,omitempty"`
	Before   string `json:"before,omitempty"`
	After    string `json:"after,omitempty"`
	Identity string `json:"identity,omitempty"`
	// Body is the reader's own words in any state: what a thought says, why a
	// question is one.
	Body string `json:"body,omitempty"`
	// Resolved is when the reader said a question was answered. Only the reader
	// sets it: writing in a question does not, and it is kept when the mark
	// becomes another state, so it is still there if it becomes a question
	// again.
	Resolved time.Time `json:"resolved,omitzero"`
	// Deleted is a tombstone: the mark stays in the file, so that an older copy
	// of the file coming back through git does not bring it back to life.
	Deleted bool      `json:"deleted,omitempty"`
	At      time.Time `json:"at"`
}

// Edit names one editing session of one page, and the send of it that a line
// holds. A later send of the same session may follow an earlier one that is
// still on its way, as the last words a page sends as it is left are; this is
// how the two are told from two sessions writing over each other.
type Edit struct {
	ID  string `json:"id"`
	Seq uint64 `json:"seq"`
}

// Seen is which revision of each writer's line of one mark a page had read: the
// keys are writers, "machine/process", and the values the highest rev of that
// writer's line the page showed. It is what a change says it was made on top
// of.
type Seen map[string]uint64

// Line is one line of a machine file.
type Line struct {
	Fmt int `json:"fmt"`
	// M is the machine that wrote the line, and S the process of that machine.
	M string `json:"m"`
	// Rev counts the lines a writer has written; see counter.
	Rev uint64 `json:"rev"`
	// T is the time in milliseconds, for a person to read and to order
	// conflicts for showing. Nothing is decided by it.
	T    int64 `json:"t"`
	Mark Mark  `json:"mark"`
	// Seen is what the writer's page had read of this mark's other lines.
	Seen Seen   `json:"seen,omitempty"`
	Edit *Edit  `json:"edit,omitempty"`
	S    string `json:"s,omitempty"`
}

// Writer is who wrote a line: its machine and process, "machine/process". Two
// lines of one writer are in the order of their revs; lines of two writers,
// even of one machine, are in an order only when the later says it had read the
// earlier.
func (l *Line) Writer() string { return l.M + "/" + l.S }

// encode is l as one line without its newline, sealed (internal/mark/sealedline).
func (l *Line) encode() ([]byte, error) {
	return sealedline.Seal(l)
}

// verdict is what reading one line found.
type verdict int

const (
	// readable is a line this version reads.
	readable verdict = iota
	// marker is one of git's conflict markers, which stand between the two
	// sides of a conflict in the file itself.
	marker
	// damaged is a line that is not one this version wrote and left alone:
	// not JSON, not whole, a checksum that does not fit, or a value no writer
	// makes.
	damaged
	// newer is a whole line of a later format. It is kept and not read.
	newer
)

// decode reads one line, without its newline.
func decode(raw []byte) (Line, verdict) {
	if conflictMarker(raw) {
		return Line{}, marker
	}
	// A line of a later format is judged first, since the way a later format is
	// checked is that format's own: it is kept as it is and not read.
	var head struct {
		Fmt int `json:"fmt"`
	}
	if json.Unmarshal(raw, &head) != nil {
		return Line{}, damaged
	}
	if head.Fmt > Format {
		return Line{}, newer
	}
	if sealedline.Check(raw) != sealedline.Whole {
		return Line{}, damaged
	}
	var l Line
	if json.Unmarshal(raw, &l) != nil || l.Fmt != Format || !l.wellFormed() {
		return Line{}, damaged
	}
	return l, readable
}

// wellFormed says a line is one every writer here makes: ids and codes in
// their shapes, a rev, and a mark of a kind there is.
func (l *Line) wellFormed() bool {
	if !isMachine(l.M) || (l.S != "" && !isMachine(l.S)) || l.Rev == 0 {
		return false
	}
	return l.Mark.shape() == nil
}

// The bounds a mark is held to, the same as the file of marks that came before
// this one holds a mark to: the size of the things themselves, so that a value
// beyond them was not produced by a reading page. A line of another machine is
// untrusted, and is held to them too.
const (
	maxPathBytes    = 1024
	maxAnchorBytes  = 256
	maxQuoteBytes   = 8 << 10
	maxContextBytes = 512
	maxBodyBytes    = 32 << 10
	identityLength  = 2 * sha256.Size
)

// shape says whether a mark is one every writer here makes, and says what is
// wrong when it is not.
func (m *Mark) shape() error {
	switch {
	case !isID(m.ID):
		return errors.New("the id is not 32 lowercase hex digits")
	case !m.Kind.valid():
		return fmt.Errorf("%q is not a kind of mark", string(m.Kind))
	case m.At.IsZero():
		return errors.New("the mark has no time")
	case m.Kind == Highlight && m.Quote == "" && !m.Deleted:
		return errors.New("a highlight is a passage and has its words")
	case m.Quote == "" && (m.Before != "" || m.After != ""):
		return errors.New("words around nothing")
	case m.Identity != "" && !isHex(m.Identity, identityLength):
		return errors.New("the identity is not a content identity")
	}
	if err := vault.CheckStoredPath(m.Path, maxPathBytes); err != nil {
		return fmt.Errorf("the note path: %w", err)
	}
	if err := checkAnchor(m.Anchor); err != nil {
		return err
	}
	return m.checkWords()
}

// checkWords holds the reader's words to their sizes and to being text.
func (m *Mark) checkWords() error {
	for _, f := range [...]struct {
		name  string
		value string
		limit int
	}{
		{"the quote", m.Quote, maxQuoteBytes},
		{"the words before it", m.Before, maxContextBytes},
		{"the words after it", m.After, maxContextBytes},
		{"the body", m.Body, maxBodyBytes},
	} {
		switch {
		case !utf8.ValidString(f.value) || strings.ContainsRune(f.value, 0):
			return fmt.Errorf("%s is not text", f.name)
		case len(f.value) > f.limit:
			return fmt.Errorf("%s is too long", f.name)
		}
	}
	return nil
}

// checkAnchor says a is an anchor a mark may hold: an id a renderer wrote,
// with no space, control character or character that ends the fragment of an
// address.
func checkAnchor(a string) error {
	switch {
	case !utf8.ValidString(a):
		return errors.New("the anchor is not valid UTF-8")
	case len(a) > maxAnchorBytes:
		return errors.New("the anchor is too long")
	case strings.ContainsFunc(a, func(r rune) bool {
		return strings.ContainsRune(`#?/\"'<>`, r) || r <= ' ' || r == 0x7f
	}):
		return errors.New("the anchor carries a character an id cannot hold")
	}
	return nil
}

// conflictMarker says a line is one of git's conflict markers: a run of seven
// or more '<', '>' or '|' alone or followed by a space, or a line of seven or
// more '=' alone. A line of marks is a JSON object, which begins with '{', so
// no line of marks is one.
func conflictMarker(line []byte) bool {
	if len(line) < 7 {
		return false
	}
	c := line[0]
	if c != '<' && c != '>' && c != '|' && c != '=' {
		return false
	}
	run := len(line) - len(bytes.TrimLeft(line, string(c)))
	if run < 7 {
		return false
	}
	if run == len(line) {
		return true
	}
	return c != '=' && line[run] == ' '
}

func (k Kind) valid() bool { return k == Highlight || k == Thought || k == Question }

func isMachine(s string) bool { return isHex(s, 8) }
func isID(s string) bool      { return isHex(s, 32) }

func isHex(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for i := range len(s) {
		if c := s[i]; (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
