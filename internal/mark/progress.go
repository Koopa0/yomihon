package mark

import (
	"fmt"
	"slices"
	"time"
)

// NoCourse is the course of a bookmark set while reading outside any course.
const NoCourse = ""

// The bounds on what progress.json holds. A reader bookmarks in a few courses
// and finishes a few thousand notes in a long reading.
const (
	// maxBookmarks is one for each course the reader has bookmarked in, and one
	// for none.
	maxBookmarks = 1000
	maxFinished  = 10000
)

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

// Finished is a note the reader has read to the end, by their own word. It is
// the reader's and says nothing about the note, whose status is the author's.
type Finished struct {
	// RelPath is the note, as a vault-relative slash path in NFC.
	RelPath string `json:"path"`
	// At is when the reader finished it.
	At time.Time `json:"at"`
}

func validateFinished(f *Finished) error {
	if err := validatePath(f.RelPath); err != nil {
		return err
	}
	if f.At.IsZero() {
		return fmt.Errorf("%w: finished at no time", ErrInvalid)
	}
	return nil
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
	for i := range held.bookmarks {
		if held.bookmarks[i].Course == course {
			return held.bookmarks[i].Bookmark, true, nil
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
	for i := range held.bookmarks {
		if held.bookmarks[i].Course == course {
			held.bookmarks[i].Bookmark = *b
			replaced = true
		}
	}
	if !replaced {
		if len(held.bookmarks) >= maxBookmarks {
			return fmt.Errorf("%w: %d bookmarks are already kept", ErrInvalid, len(held.bookmarks))
		}
		held.bookmarks = append(held.bookmarks, courseBookmark{Course: course, Bookmark: *b})
	}
	return s.saveProgress(held)
}

// ClearBookmark removes the bookmark kept for course, and reports whether there
// was one. Nothing is written where there was none, so asking a store that has
// never kept anything leaves no directory behind.
func (s *Store) ClearBookmark(course string) (bool, error) {
	if course != NoCourse {
		if err := validatePath(course); err != nil {
			return false, err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	held, err := s.loadProgress()
	if err != nil {
		return false, err
	}
	kept := slices.DeleteFunc(held.bookmarks, func(b courseBookmark) bool { return b.Course == course })
	if len(kept) == len(held.bookmarks) {
		return false, nil
	}
	held.bookmarks = kept
	return true, s.saveProgress(held)
}

// SetFinished says whether the reader has read note to the end, with the time
// they said so. Saying what is already so changes nothing and writes nothing,
// so the first time stays the time.
func (s *Store) SetFinished(note string, finished bool) error {
	if err := validatePath(note); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	held, err := s.loadProgress()
	if err != nil {
		return err
	}
	at := slices.IndexFunc(held.finished, func(f Finished) bool { return f.RelPath == note })
	switch {
	case finished && at >= 0, !finished && at < 0:
		return nil
	case finished:
		if len(held.finished) >= maxFinished {
			return fmt.Errorf("%w: %d notes are already finished", ErrInvalid, len(held.finished))
		}
		held.finished = append(held.finished, Finished{RelPath: note, At: time.Now().UTC()})
	default:
		held.finished = slices.Delete(held.finished, at, at+1)
	}
	return s.saveProgress(held)
}

// FinishedAt reports when the reader finished note, and whether they have.
func (s *Store) FinishedAt(note string) (time.Time, bool, error) {
	held, err := s.loadProgress()
	if err != nil {
		return time.Time{}, false, err
	}
	for i := range held.finished {
		if held.finished[i].RelPath == note {
			return held.finished[i].At, true, nil
		}
	}
	return time.Time{}, false, nil
}

// FinishedNotes lists every note the reader has finished, the first they
// finished first.
func (s *Store) FinishedNotes() ([]Finished, error) {
	held, err := s.loadProgress()
	if err != nil {
		return nil, err
	}
	return slices.Clone(held.finished), nil
}
