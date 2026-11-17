package vault

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// AncestorHolds reports whether any directory above the folder this Reader was
// opened on holds rel, with each part of it spelled at any letter case. A
// folder is only known to hold no rules file when the places a rules file
// would govern it from are empty too: the one the command was started inside is
// the vault's own, and a Reader opened below it sees nothing of it.
//
// Both the path the Reader was given and the one it resolved are walked, since
// a symlinked working directory has a different set of parents under each. A
// directory that cannot be listed is asked for rel at its exact spelling
// instead, and one that cannot be asked at all answers with an error, never with
// "no": silence about a place that could hold the rules is not a verdict. A
// name that cannot be followed is the same: it cannot be shown to hold nothing.
func (r *Reader) AncestorHolds(rel string) (bool, error) {
	if r == nil {
		return false, errors.New("no vault root")
	}
	parts := strings.Split(rel, "/")
	seen := map[string]bool{}
	for _, start := range []string{r.name, r.selectedPath} {
		for dir := filepath.Dir(start); !seen[dir]; dir = filepath.Dir(dir) {
			seen[dir] = true
			if held, err := holdsFolded(dir, parts); err != nil || held {
				return held, err
			}
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}
	return false, nil
}

// holdsFolded reports whether dir holds parts as a path, any part at any letter
// case. A name is matched however many entries fold to it, since a case-sensitive
// filesystem keeps System and system apart and a synced vault may carry either.
func holdsFolded(dir string, parts []string) (bool, error) {
	entries, err := os.ReadDir(dir) // #nosec G304 G703 -- dir is an ancestor of the folder the operator named
	if err != nil {
		return holdsExactly(dir, parts)
	}
	for _, entry := range entries {
		if !strings.EqualFold(entry.Name(), parts[0]) {
			continue
		}
		if len(parts) == 1 {
			return true, nil
		}
		below := filepath.Join(dir, entry.Name())
		switch {
		case entry.IsDir():
		case entry.Type()&fs.ModeSymlink != 0:
			info, statErr := os.Stat(below) // #nosec G703 -- below is a name just read from an ancestor
			if statErr != nil {
				return false, fmt.Errorf("follow %q above the vault root: %w", entry.Name(), statErr)
			}
			if !info.IsDir() {
				continue
			}
		default:
			continue
		}
		if held, holdErr := holdsFolded(below, parts[1:]); holdErr != nil || held {
			return held, holdErr
		}
	}
	return false, nil
}

// holdsExactly is holdsFolded for a directory that cannot be listed: it can
// still be asked about one name, at the spelling the loader itself reads.
func holdsExactly(dir string, parts []string) (bool, error) {
	_, err := os.Lstat(filepath.Join(append([]string{dir}, parts...)...)) // #nosec G703 -- dir is an ancestor of the folder the operator named
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, syscall.ENOTDIR):
		return false, nil
	default:
		return false, err
	}
}
