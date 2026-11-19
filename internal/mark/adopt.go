package mark

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/koopa0/yomihon/internal/mark/confdir"
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

// Adopt brings the marks, the bookmarks and the finished notes the reader kept
// for the vault at old, the path it had before it moved, into this vault's own.
// A vault's marks directory is named from its path, so a vault that moved is a
// vault with no marks until they are brought across.
//
// admit is asked, for each note a thing names, whether this vault holds that
// note and lets it be read. What it refuses stays where it was. That is the
// whole of what is left behind: a mark on a note this vault has not got has
// nowhere to be drawn, and a mark on ground this vault withholds must not
// reach an output through a list that was never asked about it. A nil admit
// refuses everything.
//
// The old directory is only read, never changed, so the same call can be made
// again and brings across only what was not. Where this vault already has a
// mark in the same place, or a bookmark for the same course, or a finished
// note, its own stays. old has to be absolute: the files record the path they
// were made for, and it is that record Adopt checks old against, so a path
// as the record spells it is the path to give.
//
// Nothing is written unless what results can be held: a bound passed refuses
// the whole call with ErrInvalid. The two files are replaced one after the
// other, so a failure between them leaves marks adopted and progress not, which
// the same call finishes.
func (s *Store) Adopt(old string, admit func(relPath string) bool) error {
	theirs, err := s.earlier(old)
	if err != nil {
		return err
	}
	if admit == nil {
		admit = func(string) bool { return false }
	}

	return s.exclusive(func() error {
		marks, _, err := s.loadMarks()
		if err != nil {
			return err
		}
		progress, err := s.loadProgress()
		if err != nil {
			return err
		}
		marks, marksChanged := adoptMarks(marks, theirs.marks, admit)
		progress, progressChanged := adoptProgress(progress, theirs.progress, admit)
		if len(marks) > MaxMarks || len(progress.bookmarks) > maxBookmarks || len(progress.finished) > maxFinished {
			return fmt.Errorf("%w: what is kept for that path does not fit beside what is kept here", ErrInvalid)
		}
		return s.install(&adopted{marks: marks, marksChanged: marksChanged, progress: progress, progressChanged: progressChanged})
	})
}

// earlier is what was kept for the vault at old, or the reason there is none to
// adopt: a path that is not absolute, this vault's own, one nothing is kept for,
// or one whose directory is another vault's.
func (s *Store) earlier(old string) (kept, error) {
	if !filepath.IsAbs(old) {
		return kept{}, fmt.Errorf("%w: the path to adopt from is not absolute", ErrInvalid)
	}
	old = filepath.Clean(old)
	if confdir.VaultID(old) == confdir.VaultID(s.vaultRoot) {
		return kept{}, ErrOwnPath
	}
	oldDir, err := confdir.Dir(s.configDir, old)
	if err != nil {
		return kept{}, err
	}
	theirs, found, err := (&Store{dir: oldDir, vaultRoot: old}).read()
	if err != nil {
		return kept{}, err
	}
	if !found {
		return kept{}, ErrNothingKept
	}
	return theirs, nil
}

// adopted is what an adoption results in, and which of the two files it changes.
type adopted struct {
	marks           []Mark
	marksChanged    bool
	progress        progress
	progressChanged bool
}

// install writes the files an adoption changed. Both are encoded before either
// is replaced, so what the files cannot hold is refused with nothing written.
// The caller holds the lock.
func (s *Store) install(a *adopted) error {
	var marksData, progressData []byte
	var err error
	if a.marksChanged {
		if marksData, err = encode(marksDocument{Version: documentVersion, Vault: s.vaultRoot, Marks: a.marks}, maxMarksFileBytes); err != nil {
			return err
		}
	}
	if a.progressChanged {
		doc := progressDocument{Version: documentVersion, Vault: s.vaultRoot, Bookmarks: a.progress.bookmarks, Finished: a.progress.finished}
		if progressData, err = encode(doc, maxProgressFileBytes); err != nil {
			return err
		}
	}
	if a.marksChanged {
		if replaceErr := confdir.Replace(s.dir, marksFile, marksData); replaceErr != nil {
			return replaceErr
		}
	}
	if a.progressChanged {
		return confdir.Replace(s.dir, progressFile, progressData)
	}
	return nil
}

// kept is everything the reader has for one vault.
type kept struct {
	marks    []Mark
	progress progress
}

// read is what this store's directory holds, converted where it is in a file an
// earlier version wrote, and whether it holds anything. It checks that the
// files record this store's vault, which is what makes the directory the one
// its name says it is. Files an earlier version wrote record nothing, so a
// directory that has only those is taken by its name.
func (s *Store) read() (held kept, found bool, err error) {
	if held.marks, _, err = s.loadMarks(); err != nil {
		return kept{}, false, err
	}
	if held.progress, err = s.loadProgress(); err != nil {
		return kept{}, false, err
	}
	for _, name := range [...]string{marksFile, progressFile} {
		recorded, ok, err := recordedVault(s.dir, name)
		if err != nil {
			return kept{}, false, err
		}
		if ok && vault.NormalizeNFC(filepath.Clean(recorded)) != vault.NormalizeNFC(s.vaultRoot) {
			return kept{}, false, ErrOtherVault
		}
	}
	found = len(held.marks) > 0 || len(held.progress.bookmarks) > 0 || len(held.progress.finished) > 0
	return held, found, nil
}

// recordedVault is the vault path a file in dir says it was made for. ok is
// false when the file is not there. The file has been read whole and strictly
// by the caller already, so only the one field is looked at.
func recordedVault(dir, name string) (path string, ok bool, err error) {
	data, err := confdir.Read(dir, name, maxMarksFileBytes)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("%w: %s: %w", ErrUnreadable, name, err)
	}
	var head struct {
		Vault string `json:"vault"`
	}
	if err = json.Unmarshal(data, &head); err != nil {
		return "", false, fmt.Errorf("%w: %s: %w", ErrUnreadable, name, err)
	}
	return head.Vault, true, nil
}

// adoptMarks is held with the marks of theirs that admit allows and that held
// does not already have, by id or as the same reader's act in the same place. A
// thought is not the same act twice: it says something, and two say two things.
func adoptMarks(held, theirs []Mark, admit func(string) bool) (merged []Mark, changed bool) {
	merged = slices.Clone(held)
	for i := range theirs {
		m := &theirs[i]
		if !admit(m.RelPath) {
			continue
		}
		if slices.ContainsFunc(merged, func(other Mark) bool {
			return other.ID == m.ID || (m.Kind != Thought && sameMark(&other, m))
		}) {
			continue
		}
		merged, changed = append(merged, *m), true
	}
	return merged, changed
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
