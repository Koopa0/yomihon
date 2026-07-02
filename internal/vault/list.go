package vault

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// List walks root and returns the vault-relative, slash-form, Unicode
// NFC-normalized paths of every regular file, skipping any path whose
// name — or any ancestor directory's name — begins with a dot (.git,
// .obsidian, .claude, ...). It is the shared starting point for anything
// that needs to see every file in the vault (internal/graph's index
// build is the first caller); it does not distinguish markdown from
// other files, that is the caller's job.
//
// NFC normalization happens here, at walk time, mirroring kura's
// vault.rs::relative_key (docs/vault-model.md, docs/design.md): macOS
// stores filenames on disk as NFD regardless of how they were typed, so
// without this every downstream consumer — graph.Index's stored path
// values, rendered <a href> targets, diagnostic candidate lists — would
// silently carry raw NFD bytes instead of Obsidian's canonical NFC form.
//
// A single unreadable entry (permissions, a mid-walk race) does not abort
// the whole walk (wall 4): it is skipped and the walk continues. Only a
// failure to walk root itself is a real error.
func List(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		rel, keep, action := listEntry(root, path, d, walkErr)
		if keep {
			paths = append(paths, rel)
		}
		return action
	})
	if err != nil {
		return nil, fmt.Errorf("list vault %s: %w", root, err)
	}
	return paths, nil
}

// listEntry decides what one WalkDir callback invocation means: the
// action to hand back to WalkDir (nil to continue, fs.SkipDir to skip a
// dot-prefixed directory's whole subtree, or a real error only when the
// root itself can't be walked at all), and — only when keep is true —
// the vault-relative slash-form path to record.
func listEntry(root, path string, d fs.DirEntry, walkErr error) (rel string, keep bool, action error) {
	if walkErr != nil {
		if path == root {
			return "", false, walkErr
		}
		if d != nil && d.IsDir() {
			return "", false, fs.SkipDir
		}
		return "", false, nil // best-effort: skip one broken entry, keep walking
	}
	if path == root {
		return "", false, nil
	}
	if strings.HasPrefix(d.Name(), ".") {
		if d.IsDir() {
			return "", false, fs.SkipDir
		}
		return "", false, nil
	}
	if d.IsDir() || !d.Type().IsRegular() {
		return "", false, nil
	}
	r, err := filepath.Rel(root, path)
	if err != nil {
		return "", false, nil // best-effort: skip a path we can't relativize
	}
	return norm.NFC.String(filepath.ToSlash(r)), true, nil
}
