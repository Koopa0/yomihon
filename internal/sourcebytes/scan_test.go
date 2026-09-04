package sourcebytes

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type finding struct {
	path string
	kind string
}

var textExtensions = []string{
	".css", ".go", ".golden", ".html", ".js", ".json", ".jsonl", ".md", ".mjs", ".sql",
	".py", ".sh", ".svg", ".templ", ".toml", ".txt", ".xml", ".yaml", ".yml",
}

var dangerousBytes = []struct {
	kind  string
	bytes []byte
}{
	{kind: "NUL", bytes: []byte{0}},
	{kind: "U+2028", bytes: []byte{0xe2, 0x80, 0xa8}},
	{kind: "U+2029", bytes: []byte{0xe2, 0x80, 0xa9}},
}

func scan(root string, allowed map[finding]struct{}) ([]finding, error) {
	tree, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("open repository root: %w", err)
	}
	findings, scanErr := scanTree(tree, allowed)
	closeErr := tree.Close()
	if scanErr != nil {
		if closeErr != nil {
			return nil, errors.Join(scanErr, fmt.Errorf("close repository root: %w", closeErr))
		}
		return nil, scanErr
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close repository root: %w", closeErr)
	}
	return findings, nil
}

func scanTree(tree *os.Root, allowed map[finding]struct{}) ([]finding, error) {
	var findings []finding
	err := fs.WalkDir(tree.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		found, stepErr := inspect(tree, allowed, name, entry, walkErr)
		findings = append(findings, found...)
		return stepErr
	})
	if err != nil {
		return nil, fmt.Errorf("scan repository source bytes: %w", err)
	}
	slices.SortFunc(findings, func(a, b finding) int {
		if order := strings.Compare(a.path, b.path); order != 0 {
			return order
		}
		return strings.Compare(a.kind, b.kind)
	})
	return findings, nil
}

// inspect reports what one step of the walk found, and how the walk should
// continue. It keeps no list of its own: the walk that started it owns the
// findings, so a step that reaches a directory, a file that is not text, or
// text that is clean all say the same thing by returning nothing.
func inspect(tree *os.Root, allowed map[finding]struct{}, name string, entry fs.DirEntry, walkErr error) ([]finding, error) {
	if walkErr != nil {
		return nil, walkErr
	}
	if entry.IsDir() {
		if name != "." && ignoredDirectory(entry.Name()) {
			return nil, fs.SkipDir
		}
		return nil, nil
	}
	if !isTextSource(entry.Name()) {
		return nil, nil
	}
	data, err := tree.ReadFile(name)
	if err != nil {
		return nil, err
	}
	var found []finding
	for _, pattern := range dangerousBytes {
		carried := finding{path: name, kind: pattern.kind}
		if _, ok := allowed[carried]; !ok && bytes.Contains(data, pattern.bytes) {
			found = append(found, carried)
		}
	}
	return found, nil
}

func ignoredDirectory(name string) bool {
	switch name {
	case ".agents", ".claude", ".codex", ".git", "bin", "node_modules", "tmp":
		return true
	default:
		return false
	}
}

func isTextSource(name string) bool {
	if name == "Makefile" || name == "LICENSE" || name == "go.mod" || name == "go.sum" || strings.HasPrefix(name, ".git") {
		return true
	}
	return slices.Contains(textExtensions, filepath.Ext(name))
}
