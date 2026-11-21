package vault

import (
	"os"
	"path/filepath"
	"strings"
)

// Tilde spells a path the way the reader writes it: under their home directory
// it begins with ~. It is the one place the home directory is read, and the one
// spelling of a vault's path for a person; output for an agent keeps the full
// path. A path outside the home directory, a path that is not absolute, and a
// home directory that cannot be found all give the path back as it was.
func Tilde(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(home) {
		return path
	}
	bases := []string{home}
	// A vault's path is resolved, and the home directory may be reached through a
	// link, so each is compared with the other's spelling too.
	if resolved, resolveErr := filepath.EvalSymlinks(home); resolveErr == nil && resolved != home {
		bases = append(bases, resolved)
	}
	for _, base := range bases {
		rel, relErr := filepath.Rel(base, path)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		return filepath.Join("~", rel)
	}
	return path
}
