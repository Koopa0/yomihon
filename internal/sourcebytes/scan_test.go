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
		return inspect(tree, allowed, &findings, name, entry, walkErr)
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

func inspect(tree *os.Root, allowed map[finding]struct{}, findings *[]finding, name string, entry fs.DirEntry, walkErr error) error {
	if walkErr != nil {
		return walkErr
	}
	if entry.IsDir() {
		if name != "." && ignoredDirectory(entry.Name()) {
			return fs.SkipDir
		}
		return nil
	}
	if !isTextSource(entry.Name()) {
		return nil
	}
	data, err := tree.ReadFile(name)
	if err != nil {
		return err
	}
	for _, pattern := range dangerousBytes {
		found := finding{path: name, kind: pattern.kind}
		if _, ok := allowed[found]; !ok && bytes.Contains(data, pattern.bytes) {
			*findings = append(*findings, found)
		}
	}
	return nil
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
