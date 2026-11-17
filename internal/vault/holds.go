package vault

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// A folder is only known to have no rules file when the places a rules file
// would govern it from are empty too. AncestorHolds asks above the root, where
// a command started inside a vault's folder is governed by that vault's rules,
// and HoldsBelow asks inside it, where a command started above a vault would
// read that vault's withheld folders as ordinary ones. Both look by name in the
// raw directory listing, at any letter case, and neither trusts what a scan
// chose to list: a scan leaves out symlinks, ignored folders and ones it could
// not open, which are exactly the shapes a rules file hides behind.

// AncestorHolds reports whether any directory above the folder this Reader was
// opened on holds rel, with each part of it spelled at any letter case.
//
// Both the path the Reader was given and the one it resolved are walked, since
// a symlinked working directory has a different set of parents under each. A
// parent that cannot be listed answers with an error, never with "no": silence
// about a place that could hold the rules is not a verdict.
func (r *Reader) AncestorHolds(rel string) (bool, error) {
	if r == nil {
		return false, errors.New("no vault root")
	}
	parts := strings.Split(rel, "/")
	seen := map[string]bool{}
	for _, start := range []string{r.name, r.selectedPath} {
		for dir := filepath.Dir(start); !seen[dir]; dir = filepath.Dir(dir) {
			seen[dir] = true
			if held, err := holdsFolded(os.DirFS(dir), ".", parts); err != nil || held {
				return held, err
			}
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}
	return false, nil
}

// HoldsBelow reports whether the root, or any directory a scan of it enters,
// holds rel with each part spelled at any letter case. A directory the scan
// does not enter at all (dot-named, ignored, or kept out by the boundary) is not
// looked in: none of its notes is read, so none can reach an output. A directory
// that is entered and cannot be listed answers with an error.
//
// It also leaves the reader watching for rel, so this is the first look and not
// the only one: see WatchFor.
func (r *Reader) HoldsBelow(ctx context.Context, rel string) (bool, error) {
	r.WatchFor(rel)
	if _, err := r.scan(ctx, scanAvailable); err != nil {
		return false, err
	}
	return r.Watched()
}

// WatchFor makes every walk of this Reader from now on note whether a folder it
// enters holds rel, in the one listing of that folder the walk reads anyway. A
// look made once and finished before the notes are read leaves a window in which
// a vault moved in is read as ordinary ground; a look made by the walk the notes
// are read from has none, because the folder it asked is the folder it read.
// Watched reports what the walks have found.
func (r *Reader) WatchFor(rel string) {
	r.watch.Store(&rulesWatch{parts: strings.Split(rel, "/")})
}

// Watched reports whether any walk since WatchFor entered a folder that holds the
// watched path, or entered one it could not list. Both answers stay once given.
// A Reader that was never asked to watch answers no.
func (r *Reader) Watched() (held bool, err error) {
	if r == nil {
		return false, errors.New("no vault root")
	}
	watch := r.watch.Load()
	if watch == nil {
		return false, nil
	}
	return watch.result()
}

// rulesWatch is what a Reader's walks have found of the one path they were asked
// to watch for.
type rulesWatch struct {
	parts []string
	mu    sync.Mutex
	found bool
	err   error
}

// lookIn asks one entered directory, by its raw listing, for the path.
func (w *rulesWatch) lookIn(fsys fs.FS, dir string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.found || w.err != nil {
		return
	}
	held, err := holdsFolded(fsys, dir, w.parts)
	w.found, w.err = held, err
}

func (w *rulesWatch) result() (held bool, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.found, w.err
}

// holdsFolded reports whether dir, in fsys, holds parts as a path with any part
// at any letter case. A name is matched however many entries fold to it, since a
// case-sensitive filesystem keeps System and system apart and a synced vault may
// carry either. Whatever stands where a part should be counts: a symlink at any
// level is a rules file somebody shared, and following it out of the folder is
// not this look's to do, and a file where a folder should be is refused as the
// loader refuses it at the root.
func holdsFolded(fsys fs.FS, dir string, parts []string) (bool, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if !strings.EqualFold(entry.Name(), parts[0]) {
			continue
		}
		if len(parts) == 1 || !entry.IsDir() {
			return true, nil
		}
		if held, holdErr := holdsFolded(fsys, joinSlash(dir, entry.Name()), parts[1:]); holdErr != nil || held {
			return held, holdErr
		}
	}
	return false, nil
}

func joinSlash(dir, name string) string {
	if dir == "." {
		return name
	}
	return dir + "/" + name
}
