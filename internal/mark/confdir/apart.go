package confdir

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/koopa0/yomihon/internal/vault"
)

// Overlap is how a folder and the configuration directory are one inside the
// other.
type Overlap int

const (
	// FolderIsIt is a folder that is the configuration directory.
	FolderIsIt Overlap = iota + 1
	// FolderHoldsIt is a folder the configuration directory is inside.
	FolderHoldsIt
	// FolderIsInside is a folder inside the configuration directory.
	FolderIsInside
)

// OverlapError is the answer for a folder that is the configuration directory,
// holds it, or is inside it. Reading such a folder would put what yomihon keeps
// for the reader inside the folder being read, which yomihon never does, so a
// folder like it is not served and nothing is kept for it.
type OverlapError struct {
	// Folder is the folder as it was given; Resolved is where it leads.
	Folder, Resolved string
	// ConfigDir is the configuration directory as the platform names it.
	ConfigDir string
	How       Overlap
}

func (e *OverlapError) Error() string {
	folder := vault.Tilde(e.Folder)
	if given, err := filepath.Abs(e.Folder); err != nil || filepath.Clean(given) != e.Resolved {
		folder += " (" + vault.Tilde(e.Resolved) + ")"
	}
	configDir := vault.Tilde(e.ConfigDir)
	switch e.How {
	case FolderIsIt:
		return folder + " is the configuration directory where yomihon keeps your marks, so reading it would write into it; open the folder your notes are in"
	case FolderHoldsIt:
		return fmt.Sprintf("%s holds %s, the configuration directory where yomihon keeps your marks, so reading it would write into it; open the folder your notes are in, not one that holds it", folder, configDir)
	case FolderIsInside:
		return fmt.Sprintf("%s is inside %s, the configuration directory where yomihon keeps your marks, so what yomihon keeps would be written into the folder being read; move the folder out of the configuration directory, or open another", folder, configDir)
	default:
		panic(fmt.Sprintf("confdir: unknown Overlap: %d", e.How))
	}
}

// Apart answers nil when the folder and the configuration directory are apart:
// neither is the other or inside it. A folder that is the configuration
// directory, holds it or is inside it answers an *OverlapError, and the zero
// ConfigDir, which keeps nothing, is apart from every folder.
//
// The two are compared as the filesystem has them, not as they are spelled.
// Each is resolved through every symbolic link and "..", the configuration
// directory as far as it exists, since the part not made yet holds no link; and
// a directory is matched by its identity, os.SameFile, against each directory
// above the other, so a second spelling of one directory — another case on a
// filesystem that ignores case, a trailing separator, another letter case of
// a drive, or a link to it — is the same directory. Anything that stops the
// comparison is an error: what cannot be told apart is not served.
func (c ConfigDir) Apart(folder string) error {
	if c.IsZero() {
		return nil
	}
	_, root, err := vault.ResolveRoot(folder)
	if err != nil {
		return err
	}
	configDir, err := resolveExisting(c.dir)
	if err == nil {
		var how Overlap
		if how, err = overlap(root, configDir); err == nil && how != 0 {
			return &OverlapError{Folder: folder, Resolved: root, ConfigDir: c.dir, How: how}
		}
	}
	if err != nil {
		return fmt.Errorf("tell %s apart from the configuration directory %s: %w", folder, c.dir, err)
	}
	return nil
}

// overlap is how root, a resolved folder, and configDir, the configuration
// directory resolved as far as it exists, are one inside the other, or 0.
func overlap(root, configDir string) (Overlap, error) {
	rootInfo, err := os.Stat(root)
	if err != nil {
		return 0, err
	}
	if how, holdsErr := holds(rootInfo, configDir); how != 0 || holdsErr != nil {
		return how, holdsErr
	}
	// The folder is inside the configuration directory, which it can be only if
	// the configuration directory is there.
	configInfo, err := os.Stat(configDir)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	under, err := isUnder(root, configInfo)
	if err != nil || !under {
		return 0, err
	}
	return FolderIsInside, nil
}

// holds says whether the folder is the configuration directory, or one of the
// directories it is in. What is not there yet is no directory to be.
func holds(folder fs.FileInfo, configDir string) (Overlap, error) {
	for at := configDir; ; at = filepath.Dir(at) {
		info, err := os.Stat(at)
		switch {
		case err == nil && os.SameFile(info, folder) && at == configDir:
			return FolderIsIt, nil
		case err == nil && os.SameFile(info, folder):
			return FolderHoldsIt, nil
		case err != nil && !errors.Is(err, fs.ErrNotExist):
			return 0, err
		}
		if filepath.Dir(at) == at {
			return 0, nil
		}
	}
}

// isUnder says whether dir is one of the directories above path.
func isUnder(path string, dir fs.FileInfo) (bool, error) {
	for at := filepath.Dir(path); ; at = filepath.Dir(at) {
		info, err := os.Stat(at)
		if err != nil {
			return false, err
		}
		if os.SameFile(info, dir) {
			return true, nil
		}
		if filepath.Dir(at) == at {
			return false, nil
		}
	}
}

// resolveExisting is path followed through every link and "..", as far as it
// exists, with the names below that which are not there yet joined on as they
// are. The names are taken off one at a time as written, without cleaning, since
// cleaning "link/.." gives the directory beside the link and the system gives
// the one above where it leads.
func resolveExisting(path string) (string, error) {
	var missing []string
	for {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			return filepath.Join(append([]string{resolved}, missing...)...), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		dir, name := splitLast(path)
		if name == "" {
			return "", err
		}
		missing = append([]string{name}, missing...)
		path = dir
	}
}

// splitLast is path without its last name, and that name, both as written. At
// the top of a volume the name is "".
func splitLast(path string) (dir, name string) {
	volume := filepath.VolumeName(path)
	rest := strings.TrimRightFunc(path[len(volume):], isSeparator)
	i := strings.LastIndexFunc(rest, isSeparator)
	if i < 0 {
		return path, ""
	}
	return volume + rest[:i+1], rest[i+1:]
}

func isSeparator(r rune) bool { return r == '/' || r == filepath.Separator }
