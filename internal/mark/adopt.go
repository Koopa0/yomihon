package mark

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"

	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/mark/ledger"
	"github.com/koopa0/yomihon/internal/mark/machinemarks"
	"github.com/koopa0/yomihon/internal/vault"
)

// ErrNothingKept means nothing is kept for the path Adopt was given: no
// directory is named by it, or what is there holds no mark, bookmark or
// finished note.
var ErrNothingKept = errors.New("nothing is kept for that path")

// ErrOwnPath means the path Adopt was given is this vault's own.
var ErrOwnPath = errors.New("that path is this vault's own")

// ErrOtherVault means the files kept under the path say they are another
// vault's. The directory is named from the path and not by it, so a directory
// copied or renamed by hand is the one way the two can disagree.
var ErrOtherVault = errors.New("what is kept for that path records another folder")

// Adopt brings the bookmarks and the finished notes the reader kept for the
// vault at old, the path it had before it moved, into this vault's own, with
// the ledger of what this machine wrote there, and answers the marks kept
// there, for this vault's marks to take in (Kept.TakeIn): they belong in the
// vault's machine files, which a Store does not write, and the earlier files of
// marks are only ever read. A vault's directory under the configuration
// directory is named from its path, so a vault that moved is a vault with none
// of these until they are brought across.
//
// admit is asked, for each note a thing names, whether this vault holds that
// note and lets it be read. What it refuses stays where it was. That is the
// whole of what is left behind: a mark on a note this vault has not got has
// nowhere to be drawn, and a mark on ground this vault withholds must not
// reach an output through a list that was never asked about it. A nil admit
// refuses every mark, bookmark and finished note.
//
// The ledger is not a list and admit is not asked of it: it is this machine's
// record of what it wrote into the vault's own files of marks, which moved
// with the vault, and it is carried whole, byte for byte, so that this machine
// goes on under the machine id it wrote them under (ledger.Carry). It is
// carried only into a vault that has no ledger of its own; one that has keeps
// it, and the earlier ledger stays where it was.
//
// The old directory is only read, never changed, so the same call can be made
// again and brings across only what was not. Where this vault already has a
// bookmark for the same course, or a finished note, its own stays. old has to
// be absolute: the files record the path they were made for, and it is that
// record Adopt checks old against, so a path as the record spells it is the
// path to give. Bookmarks and finished notes that would pass their bound
// refuse the whole call with ErrInvalid, and nothing is written.
func (s *Store) Adopt(old string, admit func(relPath string) bool) ([]machinemarks.Mark, error) {
	theirs, err := s.earlier(old)
	if err != nil {
		return nil, err
	}
	if admit == nil {
		admit = func(string) bool { return false }
	}
	err = s.exclusive(func() error {
		held, loadErr := s.loadProgress()
		if loadErr != nil {
			return loadErr
		}
		merged, changed := adoptProgress(held, theirs.progress, admit)
		if len(merged.bookmarks) > maxBookmarks || len(merged.finished) > maxFinished {
			return fmt.Errorf("%w: what is kept for that path does not fit beside what is kept here", ErrInvalid)
		}
		if changed {
			if saveErr := s.saveProgress(merged); saveErr != nil {
				return saveErr
			}
		}
		// The store's methods take no context, so the ledger's lock is waited
		// for as long as confdir.LockWait, as the store's own is.
		_, carryErr := s.ledger().Carry(context.Background(), theirs.dir)
		return carryErr
	})
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(theirs.marks, func(m machinemarks.Mark) bool { return !admit(m.Path) }), nil
}

// ledger is this vault's ledger. Nothing Adopt asks of it is said in a log: a
// carry compacts nothing, and a temporary file a stopped compaction left that
// it could not remove is said by the next process that records a line.
func (s *Store) ledger() *ledger.Ledger {
	return ledger.Open(s.dir, slog.New(slog.DiscardHandler))
}

// earlier is what was kept for the vault at old, or the reason there is none to
// adopt: a path that is not absolute, this vault's own, one nothing is kept for,
// or one whose directory is another vault's.
func (s *Store) earlier(old string) (earlierFiles, error) {
	if !filepath.IsAbs(old) {
		return earlierFiles{}, fmt.Errorf("%w: the path to adopt from is not absolute", ErrInvalid)
	}
	old = filepath.Clean(old)
	if confdir.VaultID(old) == confdir.VaultID(s.vaultRoot) {
		return earlierFiles{}, ErrOwnPath
	}
	oldDir, err := confdir.VaultDir(s.configDir, old)
	if err != nil {
		return earlierFiles{}, err
	}
	earlierStore := &Store{dir: oldDir, vaultRoot: old}
	theirs, found, err := earlierStore.read()
	if err != nil {
		return earlierFiles{}, err
	}
	// A ledger is kept too: a vault whose marks are all in its machine files
	// has nothing else for the earlier path. Reading it writes nothing.
	lines, changed, err := earlierStore.ledger().Events()
	if err != nil {
		return earlierFiles{}, fmt.Errorf("%w: the ledger: %w", ErrUnreadable, err)
	}
	if !found && len(lines) == 0 && changed == 0 {
		return earlierFiles{}, ErrNothingKept
	}
	theirs.dir = oldDir
	return theirs, nil
}

// earlierFiles is everything the reader has for one vault, and the directory it
// is in.
type earlierFiles struct {
	marks    []machinemarks.Mark
	progress progress
	dir      confdir.Dir
}

// read is what this store's directory holds, converted where it is in a file an
// earlier version wrote, and whether it holds anything. It checks that the
// files record this store's vault, which is what makes the directory the one
// its name says it is. Files an earlier version wrote record nothing, so a
// directory that has only those is taken by its name.
func (s *Store) read() (held earlierFiles, found bool, err error) {
	var recorded string
	if held.marks, recorded, _, err = s.loadMarks(); err != nil {
		return earlierFiles{}, false, err
	}
	if held.progress, err = s.loadProgress(); err != nil {
		return earlierFiles{}, false, err
	}
	for _, path := range [...]string{recorded, held.progress.vault} {
		if path != "" && vault.NormalizeNFC(filepath.Clean(path)) != vault.NormalizeNFC(s.vaultRoot) {
			return earlierFiles{}, false, ErrOtherVault
		}
	}
	found = len(held.marks) > 0 || len(held.progress.bookmarks) > 0 || len(held.progress.finished) > 0
	return held, found, nil
}

// adoptProgress is held with the bookmarks of theirs for a course held has none
// for, and the finished notes held does not have, each only when admit allows
// the note it names, and the course for a bookmark that has one.
func adoptProgress(held, theirs progress, admit func(string) bool) (merged progress, changed bool) {
	merged = progress{bookmarks: slices.Clone(held.bookmarks), finished: slices.Clone(held.finished)}
	for i := range theirs.bookmarks {
		b := &theirs.bookmarks[i]
		if (b.Course != NoCourse && !admit(b.Course)) || !admit(b.Bookmark.RelPath) {
			continue
		}
		if slices.ContainsFunc(merged.bookmarks, func(other courseBookmark) bool { return other.Course == b.Course }) {
			continue
		}
		merged.bookmarks, changed = append(merged.bookmarks, *b), true
	}
	for i := range theirs.finished {
		f := &theirs.finished[i]
		if !admit(f.RelPath) || slices.ContainsFunc(merged.finished, func(other Finished) bool { return other.RelPath == f.RelPath }) {
			continue
		}
		merged.finished, changed = append(merged.finished, *f), true
	}
	return merged, changed
}
