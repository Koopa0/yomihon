package vault

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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
func (r *Reader) HoldsBelow(ctx context.Context, rel string) (bool, error) {
	look := &lookFor{parts: strings.Split(rel, "/")}
	if _, err := r.scanLooking(ctx, scanAvailable, look); err != nil {
		return false, err
	}
	return look.found, look.err
}

// lookFor is the search a walk makes in every directory it enters.
type lookFor struct {
	parts []string
	found bool
	err   error
}

// lookIn asks one entered directory, by its raw listing, for the path.
func (w *sourceWalk) lookIn(dir string) {
	if w.look == nil || w.look.found || w.look.err != nil {
		return
	}
	held, err := holdsFolded(w.root.FS(), dir, w.look.parts)
	if err != nil {
		w.look.err = err
		return
	}
	w.look.found = held
}

// holdsFolded reports whether dir, in fsys, holds parts as a path with any part
// at any letter case. A name is matched however many entries fold to it, since a
// case-sensitive filesystem keeps System and system apart and a synced vault may
// carry either. Whatever stands where a part should be counts, and so does a
// symlink at any level: a link is a rules file somebody shared, and following it
// out of the folder is not this look's to do.
func holdsFolded(fsys fs.FS, dir string, parts []string) (bool, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if !strings.EqualFold(entry.Name(), parts[0]) {
			continue
		}
		if len(parts) == 1 || entry.Type()&fs.ModeSymlink != 0 {
			return true, nil
		}
		if !entry.IsDir() {
			continue
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
