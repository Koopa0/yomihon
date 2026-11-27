package mark

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/mark/machinemarks"
)

// Store is what the reader keeps for one vault under the configuration
// directory of the person running it, beside the vault's ledger: the bookmarks
// and the finished notes, and the earlier files of marks, which it only reads.
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

// exclusive runs change as the only writer of this vault's marks and progress,
// in this process and in any other on the same directory: a change reads a file
// and replaces it whole, so one that read before another replaced would write
// the other's away. The store's methods take no context, so the wait for the
// lock is bounded by confdir.LockWait alone.
func (s *Store) exclusive(change func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return confdir.Exclusive(context.Background(), s.dir, lockFile, confdir.LockWait, change)
}

// loadMarks reads marks.json, and the vault it records. When it is not there
// the marks are those an earlier version kept, converted, which record none.
// stored says whether either file exists.
func (s *Store) loadMarks() (held []machinemarks.Mark, recorded string, stored bool, err error) {
	data, err := confdir.Read(s.dir, marksFile, maxMarksFileBytes)
	switch {
	case err == nil:
		held, recorded, err = decodeMarks(data)
		return held, recorded, true, err
	case errors.Is(err, os.ErrNotExist):
		held, stored, err = legacyQuestions(s.dir)
		return held, "", stored, err
	default:
		return nil, "", true, fmt.Errorf("%w: %s: %w", ErrUnreadable, marksFile, err)
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
func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never returns an error
	return hex.EncodeToString(b[:])
}
