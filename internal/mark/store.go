package mark

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/koopa0/yomihon/internal/mark/confdir"
)

// Store keeps the reader's marks for one vault under the configuration
// directory of the person running it.
//
// It holds nothing in memory. The files on disk are the record, so a reader who
// deletes one has deleted it, rather than arguing with a copy this process is
// still holding. A reader who sets nothing leaves no directory behind: it is
// created by the first write.
type Store struct {
	// configDir is kept to name the directory of another vault's marks, which
	// Adopt reads.
	configDir confdir.ConfigDir
	dir       confdir.Dir
	// vaultRoot is written inside each file, because the directory holding it
	// is named after a digest.
	vaultRoot string
	// mu serializes writers, whose updates read a file and replace it whole.
	mu sync.Mutex
}

// Open names this vault's marks. It reads and creates nothing.
//
// configDir is the platform's configuration directory, resolved once by the
// command that composes this — reading the environment is the process's own
// business, and a package handed a directory cannot widen the surface the
// command promises. It is a confdir.ConfigDir and not a string, so the vault
// root cannot be passed in its place. vaultRoot is the resolved absolute root
// being served, which names which vault these marks belong to. The zero
// ConfigDir and an empty root are refused with ErrNotNamed.
func Open(configDir confdir.ConfigDir, vaultRoot string) (*Store, error) {
	dir, err := confdir.VaultDir(configDir, vaultRoot)
	if err != nil {
		return nil, err
	}
	return &Store{configDir: configDir, dir: dir, vaultRoot: vaultRoot}, nil
}

// Dir is the directory this keeps its files in. It is what the interface names
// when it tells a reader where their marks live.
func (s *Store) Dir() string {
	return s.dir.Path()
}

// List returns the marks of the given kinds, or of every kind when none is
// named, in the order they were made.
//
// Each call reads the files again, so deleting one clears what it held. Marks
// that are not stored yet but are in a file an earlier version wrote are
// returned as converted; nothing is written by asking. A file that cannot be
// read is ErrUnreadable and not an empty list.
func (s *Store) List(kinds ...Kind) ([]Mark, error) {
	held, _, err := s.loadMarks()
	if err != nil {
		return nil, err
	}
	if len(kinds) == 0 {
		return held, nil
	}
	return slices.DeleteFunc(held, func(m Mark) bool { return !slices.Contains(kinds, m.Kind) }), nil
}

// Stored reports whether the reader has any marks file for this vault. A reader
// who never marked anything and one who cleared every mark both hold no marks,
// and only the files tell them apart. A file that cannot be read is stored.
func (s *Store) Stored() bool {
	_, stored, err := s.loadMarks()
	return stored || err != nil
}

// Add keeps m and returns it as it is kept, with the id it has been given. It
// reads m and never writes through the pointer. A refusal wraps ErrInvalid and
// writes nothing.
func (s *Store) Add(m *Mark) (Mark, error) {
	if m == nil {
		return Mark{}, fmt.Errorf("%w: no mark", ErrInvalid)
	}
	kept := *m
	if kept.ID == "" {
		id, err := newID()
		if err != nil {
			return Mark{}, err
		}
		kept.ID = id
	}
	if err := validateMark(&kept); err != nil {
		return Mark{}, err
	}
	err := s.change(func(held []Mark) ([]Mark, bool, error) {
		if len(held) >= MaxMarks {
			return nil, false, fmt.Errorf("%w: %d marks are already kept", ErrInvalid, len(held))
		}
		if slices.ContainsFunc(held, func(other Mark) bool { return other.ID == kept.ID }) {
			return nil, false, fmt.Errorf("%w: a mark already has the id %s: %w", ErrInvalid, kept.ID, ErrExists)
		}
		return append(held, kept), true, nil
	})
	if err != nil {
		return Mark{}, err
	}
	return kept, nil
}

// Remove deletes the mark with the given id and returns it, so that what was
// removed can be offered back. With no such mark nothing is written and found
// is false.
func (s *Store) Remove(id string) (removed Mark, found bool, err error) {
	err = s.changeIfThereIsAnyToChange(func(held []Mark) ([]Mark, bool, error) {
		removed, found = Mark{}, false
		i := slices.IndexFunc(held, func(m Mark) bool { return m.ID == id })
		if i < 0 {
			return nil, false, nil
		}
		removed, found = held[i], true
		return slices.Delete(held, i, i+1), true, nil
	})
	return removed, found && err == nil, err
}

// Keep keeps m as a new mark, or says which mark already stands where m would.
// A mark of the same kind in the same note, anchor and words is that mark, and
// is returned with added false and nothing written; a thought is never the same
// act twice, so it is always new. The check and the write are one step under
// the writers' lock, so two submissions of one highlight at once keep one.
//
// admit is asked while the file is locked, and only for a mark that is new. It
// is what the caller knows about the note that the file does not. A refusal
// wraps ErrInvalid and writes nothing. m is read and never written through.
func (s *Store) Keep(m *Mark, admit func(*Mark) bool) (kept Mark, added bool, err error) {
	if m == nil {
		return Mark{}, false, fmt.Errorf("%w: no mark", ErrInvalid)
	}
	candidate := *m
	if candidate.ID, err = newID(); err != nil {
		return Mark{}, false, err
	}
	if invalid := validateMark(&candidate); invalid != nil {
		return Mark{}, false, invalid
	}
	err = s.change(func(held []Mark) ([]Mark, bool, error) {
		if candidate.Kind != Thought {
			if i := slices.IndexFunc(held, func(other Mark) bool { return sameMark(&other, &candidate) }); i >= 0 {
				kept = held[i]
				return nil, false, nil
			}
		}
		if admit == nil || !admit(&candidate) {
			return nil, false, fmt.Errorf("%w: the place is not one this vault renders", ErrInvalid)
		}
		if len(held) >= MaxMarks {
			return nil, false, fmt.Errorf("%w: %d marks are already kept", ErrInvalid, len(held))
		}
		kept, added = candidate, true
		return append(held, candidate), true, nil
	})
	if err != nil {
		return Mark{}, false, err
	}
	return kept, added, nil
}

// Revision is a change to a mark that is kept. A field left nil is left as it
// is. Each says something only a mark of one kind has: a reason and a
// resolution are a question's, a body is a thought's.
type Revision struct {
	Reason   *string
	Body     *string
	Resolved *bool
}

// Revise changes the mark with the given id and returns it as it is now. With
// no such mark, or an id the Store never issues, nothing is read or written and
// found is false. A change the mark's kind does not have, a revision that
// names nothing, or a result these files cannot hold is a refusal that wraps
// ErrInvalid, and nothing is written.
//
// Resolving a question that is resolved already leaves the time it was
// resolved at; reopening it clears the time. now is the moment a resolution is
// made at.
func (s *Store) Revise(id string, r Revision, now time.Time) (revised Mark, found bool, err error) {
	if !isID(id) {
		return Mark{}, false, nil
	}
	err = s.changeIfThereIsAnyToChange(func(held []Mark) ([]Mark, bool, error) {
		found = false
		i := slices.IndexFunc(held, func(m Mark) bool { return m.ID == id })
		if i < 0 {
			return nil, false, nil
		}
		found = true
		next, refused := revision(&held[i], r, now)
		if refused != nil {
			return nil, false, refused
		}
		revised = next
		changed := slices.Clone(held)
		changed[i] = next
		return changed, true, nil
	})
	if err != nil {
		return Mark{}, false, err
	}
	return revised, found, nil
}

// revision applies r to a copy of m and returns it, validated.
func revision(held *Mark, r Revision, now time.Time) (Mark, error) {
	m := *held
	switch {
	case r.Reason == nil && r.Body == nil && r.Resolved == nil:
		return Mark{}, fmt.Errorf("%w: nothing to change", ErrInvalid)
	case (r.Reason != nil || r.Resolved != nil) && m.Kind != Question:
		return Mark{}, fmt.Errorf("%w: only a question has a reason or is resolved", ErrInvalid)
	case r.Body != nil && m.Kind != Thought:
		return Mark{}, fmt.Errorf("%w: only a thought has a body", ErrInvalid)
	}
	if r.Reason != nil {
		m.Reason = *r.Reason
	}
	if r.Body != nil {
		m.Body = *r.Body
	}
	if r.Resolved != nil {
		switch {
		case !*r.Resolved:
			m.ResolvedAt = time.Time{}
		case m.ResolvedAt.IsZero():
			m.ResolvedAt = now
		}
	}
	if err := validateMark(&m); err != nil {
		return Mark{}, err
	}
	return m, nil
}

// Toggle keeps m when no mark of its kind already names the same note, anchor
// and quote, and removes that mark when there is one. The result says whether m
// is now kept. Removing is always allowed; adding needs admit to accept m
// (asked while the file is locked, and only for a mark that is new) and room
// under MaxMarks. A refusal wraps ErrInvalid. A refusal or a failure to read
// leaves the stored files as they were, a damaged one included.
func (s *Store) Toggle(m *Mark, admit func(*Mark) bool) (added bool, err error) {
	if m == nil {
		return false, fmt.Errorf("%w: no mark", ErrInvalid)
	}
	kept := *m
	id, err := newID()
	if err != nil {
		return false, err
	}
	kept.ID = id
	if invalid := validateMark(&kept); invalid != nil {
		return false, invalid
	}
	err = s.change(func(held []Mark) ([]Mark, bool, error) {
		if i := slices.IndexFunc(held, func(other Mark) bool { return sameMark(&other, &kept) }); i >= 0 {
			return slices.Delete(held, i, i+1), true, nil
		}
		if admit == nil || !admit(&kept) {
			return nil, false, fmt.Errorf("%w: the place is not one this vault renders", ErrInvalid)
		}
		if len(held) >= MaxMarks {
			return nil, false, fmt.Errorf("%w: %d places are already kept", ErrInvalid, len(held))
		}
		added = true
		return append(held, kept), true, nil
	})
	return added && err == nil, err
}

// sameMark reports whether two marks are the same reader's act: one kind in one
// place with the same words.
func sameMark(a, b *Mark) bool {
	return a.Kind == b.Kind && a.RelPath == b.RelPath && a.Anchor == b.Anchor && a.Quote == b.Quote
}

// change reads the marks, lets edit change them, and replaces the file when
// edit says it did. It runs under the writers' lock, so two changes never
// overwrite each other's.
func (s *Store) change(edit func(held []Mark) (changed []Mark, did bool, err error)) error {
	return s.exclusive(func() error {
		held, _, err := s.loadMarks()
		if err != nil {
			return err
		}
		changed, did, err := edit(held)
		if err != nil || !did {
			return err
		}
		data, err := encode(marksDocument{Version: documentVersion, Vault: s.vaultRoot, Marks: changed}, maxMarksFileBytes)
		if err != nil {
			return err
		}
		return confdir.Replace(s.dir, marksFile, data)
	})
}

// changeIfThereIsAnyToChange is change for an edit that only takes away or
// alters what is already kept. Asked of a store that holds nothing, such an edit
// answers from an unlocked read and takes no lock, because the lock's directory
// would be the first thing a reader who kept nothing left behind. The locked
// read decides when there is something, so a mark another process is adding
// at this moment is not missed: the removal of what was absent comes first.
func (s *Store) changeIfThereIsAnyToChange(edit func(held []Mark) ([]Mark, bool, error)) error {
	if held, _, err := s.loadMarks(); err == nil {
		if _, did, editErr := edit(slices.Clone(held)); editErr == nil && !did {
			return nil
		}
	}
	return s.change(edit)
}

// exclusive runs change as the only writer of this vault's marks and progress,
// in this process and in any other on the same directory: a change reads a file
// and replaces it whole, so one that read before another replaced would write
// the other's away.
func (s *Store) exclusive(change func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return confdir.Exclusive(s.dir, lockFile, confdir.LockWait, change)
}

// loadMarks reads marks.json. When it is not there the marks are those an
// earlier version kept, converted. stored says whether either file exists.
func (s *Store) loadMarks() (held []Mark, stored bool, err error) {
	data, err := confdir.Read(s.dir, marksFile, maxMarksFileBytes)
	switch {
	case err == nil:
		held, err = decodeMarks(data)
		return held, true, err
	case errors.Is(err, os.ErrNotExist):
		return legacyQuestions(s.dir)
	default:
		return nil, true, fmt.Errorf("%w: %s: %w", ErrUnreadable, marksFile, err)
	}
}

// loadProgress reads progress.json. When it is not there the bookmarks are the
// one an earlier version kept, converted into the bookmark for no course.
func (s *Store) loadProgress() (progress, error) {
	data, err := confdir.Read(s.dir, progressFile, maxProgressFileBytes)
	switch {
	case err == nil:
		return decodeProgress(data)
	case errors.Is(err, os.ErrNotExist):
		legacy, found, legacyErr := legacyBookmark(s.dir)
		if legacyErr != nil || !found {
			return progress{bookmarks: []courseBookmark{}}, legacyErr
		}
		return progress{bookmarks: []courseBookmark{{Course: NoCourse, Bookmark: legacy}}}, nil
	default:
		return progress{}, fmt.Errorf("%w: %s: %w", ErrUnreadable, progressFile, err)
	}
}

// saveProgress replaces progress.json with p, or refuses it unchanged. The
// caller holds the lock and read p from loadProgress under it.
func (s *Store) saveProgress(p progress) error {
	if p.bookmarks == nil {
		p.bookmarks = []courseBookmark{}
	}
	// One order, so that clearing a bookmark and setting it back leaves the
	// file as it was whatever else is kept.
	slices.SortFunc(p.bookmarks, func(a, b courseBookmark) int { return strings.Compare(a.Course, b.Course) })
	data, err := encode(progressDocument{Version: documentVersion, Vault: s.vaultRoot, Bookmarks: p.bookmarks, Finished: p.finished}, maxProgressFileBytes)
	if err != nil {
		return err
	}
	return confdir.Replace(s.dir, progressFile, data)
}

// newID makes the name of a new mark: 128 random bits, which two marks never
// share.
func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("name the mark: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}
