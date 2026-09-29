// Package mark keeps the one place a reader meant to come back to.
//
// A note's status is the author's word about the material and never the
// reader's own progress, so a reader who stops in the middle of one needs a
// mark of their own. There is one, in total: setting another replaces it. It
// records where to land rather than anything about what was understood — the
// id of the nearest anchor above where the reader stopped, how far below it
// they were, and the identity of the bytes that were on screen — because a
// mark meant to survive days has to survive a change of text size or typeface,
// and a pixel alone does not.
//
// It lives in a file of yomihon's own, under the configuration directory the
// platform gives the person running it, one directory per vault. The vault
// therefore keeps exactly one written field and nothing else, no folder needs
// an ignore entry, two machines never write one file between them, and the
// scanner never meets this. What it costs is that a mark does not travel: one
// left on this machine is invisible on the next, which the interface says in
// words rather than leaving the reader to find out.
//
// Only the reading server builds one. The adjudication commands never read it:
// what they answer is about the vault, and this is about the reader.
package mark

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/koopa0/yomihon/internal/vault"
)

// ErrInvalid means the submitted mark is not a shape this file can hold. The
// wrapped sentence names the part that was refused. Nothing is written: a mark
// is one value, and half of one would send the reader somewhere nobody chose.
var ErrInvalid = errors.New("the mark is not a shape this file can hold")

// ErrNotNamed means the marks file was asked for without both of the things
// that name it: where this platform keeps a program's own files, and which
// vault the marks belong to.
var ErrNotNamed = errors.New("the marks file needs a configuration directory and a vault root")

// documentVersion is the shape the file is written in and the only one read
// back. A file carrying any other number is treated as holding no mark and is
// replaced whole by the next one, which is what this number is for: it refuses
// a shape rather than converting it.
const documentVersion = 1

// fileName is the marks file inside this vault's own directory.
const fileName = "reader.json"

// directoryNameLength is how much of the vault digest names the directory.
// Half of the digest distinguishes any two folders a person has, and a
// directory name is read by people who have to find it to delete it.
const directoryNameLength = 32

// The bounds a stored mark is held to. They are the size of the things
// themselves — an anchor is an id the renderer wrote, an offset is a distance
// down one document — so a value beyond them was not produced by a reading
// page and is refused rather than stored and handed back later.
const (
	maxRelPathBytes = 1024
	maxAnchorBytes  = 256
	maxOffset       = 1 << 24
	identityLength  = 2 * sha256.Size
)

// Continuation is the place a reader deliberately left off at. It is a
// position and nothing more: no judgement about the note, no history, and no
// second one.
type Continuation struct {
	// RelPath is the note, as a vault-relative slash path in NFC — the same
	// spelling every other face here uses, so a mark and a page name one note.
	RelPath string
	// Anchor is the id of the nearest anchor above where the reader stopped —
	// any rendered id the authored body carries: a heading or block address
	// on most notes, and a footnote reference or a container elsewhere. The
	// nearest one is the most precise thing the page can name at that point,
	// whatever kind of element carries it. It is empty where the position had
	// none above it, which a short note ordinarily has; Offset is then
	// measured from the top of the document, and the mark keeps the weaker
	// promise a pixel can keep.
	Anchor string
	// Offset is how far below Anchor the reader was when they set the mark.
	// Landing scrolls to Anchor and adds it.
	Offset int
	// Identity is the content identity of the bytes the reader was looking at,
	// as lowercase hex. The note's own status is spliced out of that identity,
	// so the author moving a note through its lifecycle does not make every
	// mark look stale. A mark whose identity no longer matches still points at
	// the note — it is the reader's best pointer — and the row that offers it
	// says the note has changed.
	Identity string
	// At is when the mark was set.
	At time.Time
}

// Directory is where this vault's marks file sits inside configDir.
//
// The name is derived from the vault root rather than spelled from it: a path
// is not a safe directory name, and two vaults must land in two directories.
// The root is normalized to NFC first, because a folder reached by two
// spellings of the same name is one vault and would otherwise be given two
// directories with two different marks in them. The name says nothing about
// which vault it belongs to, which is what the vault field inside the file is
// for — a person who has to find the right directory opens one file.
func Directory(configDir, vaultRoot string) string {
	sum := sha256.Sum256([]byte(vault.NormalizeNFC(vaultRoot)))
	return filepath.Join(configDir, "yomihon", hex.EncodeToString(sum[:])[:directoryNameLength])
}

// validate reports whether c is a mark this file may hold, naming the part it
// refuses. The checks are the file's own: what reaches here came in over a
// route anyone on this machine can post to, and a value stored unchecked is
// handed back to a later reading page as though a reader had chosen it.
func validate(c *Continuation) error {
	switch {
	case c.RelPath == "" || len(c.RelPath) > maxRelPathBytes:
		return fmt.Errorf("%w: the note path is empty or too long", ErrInvalid)
	case !fs.ValidPath(c.RelPath) || c.RelPath == ".":
		return fmt.Errorf("%w: %q is not a local vault-relative path", ErrInvalid, c.RelPath)
	case c.RelPath != vault.NormalizeNFC(c.RelPath):
		return fmt.Errorf("%w: %q is not written in NFC", ErrInvalid, c.RelPath)
	case len(c.Anchor) > maxAnchorBytes:
		return fmt.Errorf("%w: the anchor is too long", ErrInvalid)
	case strings.ContainsFunc(c.Anchor, isNotAnchorRune):
		return fmt.Errorf("%w: the anchor carries a character an id cannot hold", ErrInvalid)
	case c.Offset < 0 || c.Offset > maxOffset:
		return fmt.Errorf("%w: the offset is outside one document", ErrInvalid)
	case !isContentIdentity(c.Identity):
		return fmt.Errorf("%w: the identity is not a content identity", ErrInvalid)
	}
	return nil
}

// isNotAnchorRune reports whether r cannot appear in an anchor this file
// stores. An id the renderer wrote holds no space, no control character and no
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
	if len(s) != identityLength {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
