// Package mark keeps what a reader leaves in the notes: highlights, questions,
// thoughts and bookmarks.
//
// A mark is the reader's, never the author's: a note's status is the author's
// word about the material, and a mark is the reader's word about their own
// reading. Highlights, thoughts and questions live in the vault, in one file for
// each computer under reader-notes/.yomihon/marks/, written by package
// machinemarks and read together, so that git carries them between computers
// and two machines never write one file. The vault keeps no field of them, the
// scanner never meets them (it leaves out reader-notes/), and the one place in
// the vault yomihon writes is that folder. The ledger, which is the local
// backup, and everything else of the reader's stay in files of yomihon's own,
// under the configuration directory the platform gives the person running it,
// one directory per vault.
//
// Each mark has the note, the id of the nearest anchor above it, and, for a span
// of text, the words chosen and a little of what surrounds them, so a mark can
// find its place again after the note is edited. Bookmarks are in progress.json,
// one for each course and one for reading outside any course, replaced whole
// through confdir; a file that cannot be read is kept as it is and reported,
// never replaced by the next change.
//
// Files an earlier version wrote, marks.json, reader.json and uncertainty.json,
// are read, and the first write of a machine that has no file of marks takes
// marks.json in. They are never changed or removed. See migrate.go.
//
// The reading server writes the marks, through the routes of this package. The
// marks command, which answers about the reader, reads, and writes only when it
// is asked to adopt what was kept for the vault's earlier path (see adopt.go).
// The adjudication commands — check, coverage, exists — never read any of it:
// what they answer is about the vault, and this is about the reader.
package mark

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/vault"
)

// ErrInvalid means the submitted mark is not a shape these files can hold. The
// wrapped sentence names the part that was refused. Nothing is written: a mark
// is one value, and half of one would send the reader somewhere nobody chose.
var ErrInvalid = errors.New("the mark is not a shape this file can hold")

// ErrExists means a mark with the id it was given is already kept. It is
// reported together with ErrInvalid, and tells putting a mark back that the
// mark is already back from a refusal of its shape.
var ErrExists = errors.New("a mark already has that id")

// ErrChanged is what a move is refused with when what it expected to replace is
// not what is kept, because another page changed it first.
var ErrChanged = errors.New("what was kept has changed")

// ErrUnreadable means a stored file cannot be read safely: it is damaged, in a
// shape this version does not know, or longer than anything written here. The
// file is left as it is so the reader can recover it, and what depends on it
// is refused rather than answered from nothing.
var ErrUnreadable = errors.New("the marks cannot be read")

// ErrNotNamed means a Store was asked for without both of the things that name
// it: where this platform keeps a program's own files, and which vault the
// marks belong to.
var ErrNotNamed = confdir.ErrNotNamed

// Kind is what a mark is.
type Kind string

// The kinds a reader can leave on a span of text or on a whole section.
const (
	// Highlight is a span of text the reader picked out. It writes nothing.
	Highlight Kind = "highlight"
	// Question is a place the reader does not understand yet, with a reason
	// they may give, open until they resolve it.
	Question Kind = "question"
	// Thought is something the reader wrote beside a span or a section.
	Thought Kind = "thought"
)

// The bounds a stored mark is held to. They are the size of the things
// themselves — an anchor is an id the renderer wrote, a quote is a paragraph at
// most — so a value beyond them was not produced by a reading page and is
// refused rather than stored and handed back later.
const (
	maxRelPathBytes = 1024
	maxAnchorBytes  = 256
	maxQuoteBytes   = 8 << 10
	maxContextBytes = 512
	maxReasonBytes  = 4 << 10
	maxBodyBytes    = 32 << 10
	maxOffset       = 1 << 24
	identityLength  = 2 * sha256.Size
)

// MaxMarks bounds how many marks are kept at once. A reader marks a few hundred
// places in a long reading; the bound keeps an unbounded stream of requests from
// growing a file that every later request reads whole.
const MaxMarks = 5000

// Mark is one thing a reader left in a note.
type Mark struct {
	// ID names this mark for as long as it exists. It is assigned by Add.
	ID   string `json:"id"`
	Kind Kind   `json:"kind"`
	// RelPath is the note, as a vault-relative slash path in NFC — the same
	// spelling every other face here uses, so a mark and a page name one note.
	RelPath string `json:"path"`
	// Anchor is the id of the nearest anchor above the mark — any rendered id
	// the authored body carries: a heading or block address on most notes, a
	// footnote or a container elsewhere. It is empty where nothing above it has
	// one, and the mark then names the note as a whole.
	Anchor string `json:"anchor,omitempty"`
	// Quote is the words the reader chose, and Before and After a little of
	// what stood on either side of them. A mark that finds its place again
	// does so by these. A mark of a whole section has none of the three.
	Quote  string `json:"quote,omitempty"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
	// Identity is the content identity of the note's bytes when the mark was
	// made, as lowercase hex: the SHA-256 of every byte of the file.
	Identity string `json:"identity,omitempty"`
	// Reason is why a question is one, in the reader's words. It may be empty.
	Reason string `json:"reason,omitempty"`
	// Body is what a thought says. It is never empty.
	Body string `json:"body,omitempty"`
	// ResolvedAt is when a question stopped being one; zero while it is open.
	ResolvedAt time.Time `json:"resolved,omitzero"`
	// At is when the mark was made.
	At time.Time `json:"at"`
}

// validateMark reports whether m is a mark these files may hold, naming the
// part it refuses. The checks are the file's own: what reaches here came in
// over a route anyone on this machine can post to, and a value stored
// unchecked is handed back to a later reading page as though a reader had
// chosen it.
func validateMark(m *Mark) error {
	if m == nil {
		return fmt.Errorf("%w: no mark", ErrInvalid)
	}
	if !isID(m.ID) {
		return fmt.Errorf("%w: the id is not 32 lowercase hex digits", ErrInvalid)
	}
	if err := validateKind(m); err != nil {
		return err
	}
	if err := validatePath(m.RelPath); err != nil {
		return err
	}
	if err := ValidateAnchor(m.Anchor); err != nil {
		return err
	}
	if err := validateWords(m); err != nil {
		return err
	}
	switch {
	case m.Identity != "" && !isContentIdentity(m.Identity):
		return fmt.Errorf("%w: the identity is not a content identity", ErrInvalid)
	case m.At.IsZero():
		return fmt.Errorf("%w: the mark has no time", ErrInvalid)
	}
	return nil
}

// validateKind holds what a kind may carry: a highlight is a span and says
// nothing, a question may give a reason and be resolved, a thought says
// something.
func validateKind(m *Mark) error {
	switch m.Kind {
	case Highlight:
		if m.Quote == "" {
			return fmt.Errorf("%w: a highlight is a span of text", ErrInvalid)
		}
		if m.Reason != "" || m.Body != "" || !m.ResolvedAt.IsZero() {
			return fmt.Errorf("%w: a highlight carries no reason, body or resolution", ErrInvalid)
		}
	case Question:
		if m.Body != "" {
			return fmt.Errorf("%w: a question carries no body", ErrInvalid)
		}
	case Thought:
		if m.Body == "" {
			return fmt.Errorf("%w: a thought says something", ErrInvalid)
		}
		if m.Reason != "" || !m.ResolvedAt.IsZero() {
			return fmt.Errorf("%w: a thought carries no reason or resolution", ErrInvalid)
		}
	default:
		return fmt.Errorf("%w: %q is not a kind of mark", ErrInvalid, string(m.Kind))
	}
	return nil
}

// validateWords holds the text a mark carries to its size and to being text.
func validateWords(m *Mark) error {
	for _, field := range []struct {
		name  string
		value string
		limit int
	}{
		{"the quote", m.Quote, maxQuoteBytes},
		{"the words before it", m.Before, maxContextBytes},
		{"the words after it", m.After, maxContextBytes},
		{"the reason", m.Reason, maxReasonBytes},
		{"the body", m.Body, maxBodyBytes},
	} {
		switch {
		case !utf8.ValidString(field.value) || strings.ContainsRune(field.value, 0):
			return fmt.Errorf("%w: %s is not text", ErrInvalid, field.name)
		case len(field.value) > field.limit:
			return fmt.Errorf("%w: %s is too long", ErrInvalid, field.name)
		}
	}
	if m.Quote == "" && (m.Before != "" || m.After != "") {
		return fmt.Errorf("%w: words around nothing", ErrInvalid)
	}
	return nil
}

// validatePath reports whether p is a note path these files may hold.
func validatePath(p string) error {
	switch {
	case !utf8.ValidString(p):
		return fmt.Errorf("%w: the note path is not valid UTF-8", ErrInvalid)
	case p == "" || len(p) > maxRelPathBytes:
		return fmt.Errorf("%w: the note path is empty or too long", ErrInvalid)
	case !fs.ValidPath(p) || p == ".":
		return fmt.Errorf("%w: %q is not a local vault-relative path", ErrInvalid, p)
	case strings.ContainsFunc(p, origin.EndsALine):
		// fs.ValidPath lets a NUL or a line break through, and a path that
		// holds a control character is not one line of text to put on the
		// desk. Almost no note carries one in its name. A file whose own name
		// does (a tab or a line break, which Linux and macOS allow) is still
		// offered the control on its page, and pressing it is refused here
		// with the rest; the cost is accepted because such names are very
		// rare.
		return fmt.Errorf("%w: %q carries a control character", ErrInvalid, p)
	case p != vault.NormalizeNFC(p):
		return fmt.Errorf("%w: %q is not written in NFC", ErrInvalid, p)
	}
	return nil
}

// ValidateAnchor reports whether an anchor can be stored in a mark. Empty is
// valid when no accepted ID precedes the position. Values must be valid UTF-8,
// fit within 256 bytes, and contain no fragment delimiters, spaces or ASCII
// control characters. A refusal wraps ErrInvalid.
func ValidateAnchor(anchor string) error {
	switch {
	case !utf8.ValidString(anchor):
		return fmt.Errorf("%w: the anchor is not valid UTF-8", ErrInvalid)
	case len(anchor) > maxAnchorBytes:
		return fmt.Errorf("%w: the anchor is too long", ErrInvalid)
	case strings.ContainsFunc(anchor, isNotAnchorRune):
		return fmt.Errorf("%w: the anchor carries a character an id cannot hold", ErrInvalid)
	}
	return nil
}

// isNotAnchorRune reports whether r cannot appear in an anchor these files
// store. An id the renderer wrote holds no space, no control character and no
// character that would end the fragment of an address it is later put into.
func isNotAnchorRune(r rune) bool {
	switch r {
	case '#', '?', '/', '\\', '"', '\'', '<', '>':
		return true
	}
	return r <= ' ' || r == 0x7f
}

// isContentIdentity reports whether s is a content identity written as
// lowercase hex, which is the one spelling the reading page stamps.
func isContentIdentity(s string) bool {
	return isHex(s, identityLength)
}

// isID reports whether s is a mark id: 32 lowercase hex digits.
func isID(s string) bool {
	return isHex(s, 32)
}

func isHex(s string, length int) bool {
	if len(s) != length {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
