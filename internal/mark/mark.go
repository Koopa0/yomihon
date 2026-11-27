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
	"errors"
	"fmt"

	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/vault"
)

// ErrInvalid means what was submitted is not a shape these files can hold. The
// wrapped sentence names the part that was refused. Nothing is written: a
// bookmark is one value, and half of one would send the reader somewhere nobody
// chose.
var ErrInvalid = errors.New("the mark is not a shape this file can hold")

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

// The bounds a bookmark is held to beside a mark's: a note path, and how far
// below its anchor a place is.
const (
	maxPathBytes = 1024
	maxOffset    = 1 << 24
)

// validatePath reports whether p is a note path these files may hold.
func validatePath(p string) error {
	if err := vault.CheckStoredPath(p, maxPathBytes); err != nil {
		return fmt.Errorf("%w: the note path: %w", ErrInvalid, err)
	}
	return nil
}
