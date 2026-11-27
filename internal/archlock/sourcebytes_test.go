package archlock

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// textExtensions and textNames are what the walk below reads as text, with a
// git file such as .gitignore. A file named otherwise, a fuzz seed or a
// checksum list, is not read.
var (
	textExtensions = []string{
		".css", ".go", ".golden", ".html", ".js", ".json", ".jsonl", ".md", ".mjs", ".sql",
		".py", ".sh", ".svg", ".templ", ".toml", ".txt", ".xml", ".yaml", ".yml",
	}
	textNames = []string{"Makefile", "LICENSE", "go.mod", "go.sum"}
)

// separatorFixtures carry U+2028 and U+2029 on purpose: the judge's tests
// read them to show the output escapes both.
var separatorFixtures = []string{
	"internal/judge/testdata/golden/escapes.jsonl",
	"internal/judge/testdata/vault-escapes/note.md",
}

// TestTextCarriesNoInvisibleBreak keeps a NUL, a U+2028 and a U+2029 out of the
// repository's text: none shows in review, and the last two end a line for a
// JavaScript parser while ending nothing for a person.
func TestTextCarriesNoInvisibleBreak(t *testing.T) {
	t.Parallel()

	root, err := os.OpenRoot(repoRoot)
	if err != nil {
		t.Fatalf("open the repository: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("close the repository: %v", closeErr)
		}
	})
	err = fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			switch name {
			case ".agents", ".claude", ".codex", ".git", "bin", "node_modules", "tmp":
				return fs.SkipDir
			}
			return nil
		}
		if !slices.Contains(textExtensions, filepath.Ext(name)) && !slices.Contains(textNames, name) && !strings.HasPrefix(name, ".git") {
			return nil
		}
		data, err := root.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.IndexByte(data, 0) >= 0 {
			t.Errorf("%s carries a NUL", path)
		}
		if !slices.Contains(separatorFixtures, path) && (bytes.ContainsRune(data, 0x2028) || bytes.ContainsRune(data, 0x2029)) {
			t.Errorf("%s carries a raw U+2028 or U+2029; write it as an escape", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the repository: %v", err)
	}
}
