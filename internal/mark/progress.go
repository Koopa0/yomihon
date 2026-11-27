package mark

import (
	"fmt"
	"slices"
	"time"

	"github.com/koopa0/yomihon/internal/mark/machinemarks"
)

// NoCourse is the course of a bookmark set while reading outside any course.
const NoCourse = ""

// stamp is the time the reader's own acts are kept at: UTC and to the
// millisecond a page's clock can say, so that a time sent round a page and back
// is the time that was kept.
func stamp() time.Time { return time.Now().UTC().Truncate(time.Millisecond) }

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
	if err := machinemarks.CheckAnchor(b.Anchor); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	switch {
	case b.Offset < 0 || b.Offset > maxOffset:
		return fmt.Errorf("%w: the offset is outside one document", ErrInvalid)
	case !machinemarks.ValidIdentity(b.Identity):
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

// Expectation is what a caller who puts a bookmark back says it expects to
// replace: the bookmark set at At, or none where At is nil. A move that finds
// another is refused as ErrChanged and changes nothing, so an undo cannot
// overwrite what another page did in between.
type Expectation struct {
	At *time.Time
}

// MoveBookmark replaces the bookmark kept for course with b and answers the
// one it replaced, so a page can say where the bookmark was and put it back. had
// is false where the course had none. expect is nil for a move that expects
// nothing in particular. A refused bookmark, or one that did not find what was
// expected, changes nothing.
func (s *Store) MoveBookmark(course string, b *Bookmark, expect *Expectation) (previous Bookmark, had bool, err error) {
	err = validateBookmark(course, b)
	if err != nil {
		return Bookmark{}, false, err
	}
	// A move that expects what is not kept changes nothing, so it is refused from
	// an unlocked read, as asking a store that holds nothing takes no lock: its
	// directory would be left behind. The locked read decides whenever it is taken.
	if held, loadErr := s.loadProgress(); loadErr == nil {
		err = expect.check(held.bookmarks, course)
		if err != nil {
			return Bookmark{}, false, err
		}
	}
	err = s.exclusive(func() error {
		held, loadErr := s.loadProgress()
		if loadErr != nil {
			return loadErr
		}
		checkErr := expect.check(held.bookmarks, course)
		if checkErr != nil {
			return checkErr
		}
		var at int
		previous, at = bookmarkIn(held.bookmarks, course)
		had = at >= 0
		switch {
		case had:
			held.bookmarks[at].Bookmark = *b
		case len(held.bookmarks) >= maxBookmarks:
			return fmt.Errorf("%w: %d bookmarks are already kept", ErrInvalid, len(held.bookmarks))
		default:
			held.bookmarks = append(held.bookmarks, courseBookmark{Course: course, Bookmark: *b})
		}
		return s.saveProgress(held)
	})
	if err != nil {
		return Bookmark{}, false, err
	}
	return previous, had, nil
}

// bookmarkIn is the bookmark held for course and where it is, at -1 where there
// is none.
func bookmarkIn(held []courseBookmark, course string) (b Bookmark, at int) {
	at = slices.IndexFunc(held, func(c courseBookmark) bool { return c.Course == course })
	if at >= 0 {
		b = held[at].Bookmark
	}
	return b, at
}

// check is nil where the bookmark held for course is the one e expects, and
// where e is nil, which expects nothing in particular.
func (e *Expectation) check(held []courseBookmark, course string) error {
	if e == nil {
		return nil
	}
	b, at := bookmarkIn(held, course)
	if (e.At == nil && at < 0) || (e.At != nil && at >= 0 && b.At.Equal(*e.At)) {
		return nil
	}
	return fmt.Errorf("%w: the bookmark kept is not the one expected", ErrChanged)
}

// ClearBookmark removes the bookmark kept for course and answers it, so it can
// be set back; had is false where there was none. Nothing is written where there
// was none, so asking a store that has never kept anything leaves no directory
// behind.
func (s *Store) ClearBookmark(course string) (removed Bookmark, had bool, err error) {
	if course != NoCourse {
		err = validatePath(course)
		if err != nil {
			return Bookmark{}, false, err
		}
	}
	// Asking a store that holds nothing takes no lock: its directory would be
	// left behind. The locked read below decides whenever there is something.
	if held, loadErr := s.loadProgress(); loadErr == nil && !slices.ContainsFunc(held.bookmarks, func(b courseBookmark) bool { return b.Course == course }) {
		return Bookmark{}, false, nil
	}
	err = s.exclusive(func() error {
		held, loadErr := s.loadProgress()
		if loadErr != nil {
			return loadErr
		}
		for _, b := range held.bookmarks {
			if b.Course == course {
				removed, had = b.Bookmark, true
			}
		}
		if !had {
			return nil
		}
		held.bookmarks = slices.DeleteFunc(held.bookmarks, func(b courseBookmark) bool { return b.Course == course })
		return s.saveProgress(held)
	})
	if err != nil {
		return Bookmark{}, false, err
	}
	return removed, had, nil
}

// SetFinished says whether the reader has read note to the end, with the time
// they said so. Saying what is already so changes nothing and writes nothing,
// so the first time stays the time.
func (s *Store) SetFinished(note string, finished bool) error {
	if err := validatePath(note); err != nil {
		return err
	}
	if held, err := s.loadProgress(); err == nil && !finished && !slices.ContainsFunc(held.finished, func(f Finished) bool { return f.RelPath == note }) {
		return nil
	}
	return s.exclusive(func() error {
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
			held.finished = append(held.finished, Finished{RelPath: note, At: stamp()})
		default:
			held.finished = slices.Delete(held.finished, at, at+1)
		}
		return s.saveProgress(held)
	})
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
