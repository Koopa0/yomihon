package mark

import (
	"fmt"
	"time"

	"github.com/koopa0/yomihon/internal/mark/confdir"
)

// NoCourse is the course of a bookmark set while reading outside any course.
const NoCourse = ""

// maxBookmarks bounds how many bookmarks are kept: one for each course the
// reader has bookmarked in, and one for none.
const maxBookmarks = 1000

// Bookmark is the place a reader deliberately left off at. It is a position and
// nothing more: no judgement about the note, no history, and no second one in
// the same course.
type Bookmark struct {
	// RelPath is the note, as a vault-relative slash path in NFC.
	RelPath string `json:"path"`
	// Anchor is the id of the nearest anchor above where the reader stopped —
	// any rendered id the authored body carries: a heading or block address
	// on most notes, and a footnote reference or a container elsewhere. The
	// nearest one is the most precise thing the page can name at that point,
	// whatever kind of element carries it. It is empty where the position had
	// none above it, which a short note ordinarily has; Offset is then
	// measured from the top of the document, and the mark keeps the weaker
	// promise a pixel can keep.
	Anchor string `json:"anchor,omitempty"`
	// Offset is how far below Anchor the reader was when they set the mark.
	// Landing scrolls to Anchor and adds it.
	Offset int `json:"offset"`
	// Identity is the content identity of the bytes the reader was looking at,
	// as lowercase hex: the SHA-256 of every byte of the file. A bookmark whose
	// identity no longer matches still points at the note — it is the reader's
	// best pointer — and the row that offers it says the note has changed.
	Identity string `json:"identity"`
	// At is when the bookmark was set.
	At time.Time `json:"at"`
}

// validateBookmark reports whether b is a bookmark these files may hold for
// course, naming the part it refuses.
func validateBookmark(course string, b *Bookmark) error {
	if course != NoCourse {
		if err := validatePath(course); err != nil {
			return err
		}
	}
	if err := validatePath(b.RelPath); err != nil {
		return err
	}
	if err := ValidateAnchor(b.Anchor); err != nil {
		return err
	}
	switch {
	case b.Offset < 0 || b.Offset > maxOffset:
		return fmt.Errorf("%w: the offset is outside one document", ErrInvalid)
	case !isContentIdentity(b.Identity):
		return fmt.Errorf("%w: the identity is not a content identity", ErrInvalid)
	}
	return nil
}

// Bookmark reports the bookmark kept for course, and whether there is one. A
// file that cannot be read is an error and not an absent bookmark: the reader
// is told, and the file is left for them to recover.
func (s *Store) Bookmark(course string) (Bookmark, bool, error) {
	held, err := s.loadProgress()
	if err != nil {
		return Bookmark{}, false, err
	}
	for i := range held {
		if held[i].Course == course {
			return held[i].Bookmark, true, nil
		}
	}
	return Bookmark{}, false, nil
}

// SetBookmark replaces the bookmark kept for course with b, or refuses it
// unchanged. It reads b and never writes through the pointer. Other courses'
// bookmarks stay as they are.
func (s *Store) SetBookmark(course string, b *Bookmark) error {
	if err := validateBookmark(course, b); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	held, err := s.loadProgress()
	if err != nil {
		return err
	}
	replaced := false
	for i := range held {
		if held[i].Course == course {
			held[i].Bookmark = *b
			replaced = true
		}
	}
	if !replaced {
		if len(held) >= maxBookmarks {
			return fmt.Errorf("%w: %d bookmarks are already kept", ErrInvalid, len(held))
		}
		held = append(held, courseBookmark{Course: course, Bookmark: *b})
	}
	data, err := encode(progressDocument{Version: documentVersion, Vault: s.vaultRoot, Bookmarks: held}, maxProgressFileBytes)
	if err != nil {
		return err
	}
	return confdir.Replace(s.dir, progressFile, data)
}
