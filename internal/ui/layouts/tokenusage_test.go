package layouts

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// Definitions in the design palette must have a reference in shipped source.
// Tests, recorded pages and comments cannot keep an otherwise unused token.
func TestDesignTokensHaveProductionReferences(t *testing.T) {
	t.Parallel()
	stripComments := func(source string) string {
		source = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(source, "")
		return regexp.MustCompile(`(?m)^\s*//[^\n]*`).ReplaceAllString(source, "")
	}
	data, err := os.ReadFile("../../../assets/css/tokens.css")
	if err != nil {
		t.Fatal(err)
	}
	definitions := make(map[string]bool)
	for _, match := range regexp.MustCompile(`(?m)^\s*(--[a-z0-9-]+)\s*:`).FindAllStringSubmatch(stripComments(string(data)), -1) {
		definitions[match[1]] = true
	}
	if len(definitions) < 40 {
		t.Fatalf("parsed only %d design tokens; the usage check would prove too little", len(definitions))
	}
	references := make(map[string]bool)
	variable := regexp.MustCompile(`var\(\s*(--[a-z0-9-]+)`)
	literal := regexp.MustCompile(`['"](--[a-z0-9-]+)['"]`)
	for _, root := range []string{"../../../assets", "../../../internal"} {
		sourceRoot, err := os.OpenRoot(root)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if closeErr := sourceRoot.Close(); closeErr != nil {
				t.Errorf("close source root %s: %v", root, closeErr)
			}
		})
		err = fs.WalkDir(sourceRoot.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if entry.Name() == "testdata" {
					return fs.SkipDir
				}
				return nil
			}
			if !slices.Contains([]string{".css", ".js", ".mjs", ".go", ".templ"}, filepath.Ext(path)) || strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "_templ.go") {
				return nil
			}
			data, readErr := sourceRoot.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			source := stripComments(string(data))
			for _, pattern := range []*regexp.Regexp{variable, literal} {
				for _, match := range pattern.FindAllStringSubmatch(source, -1) {
					references[match[1]] = true
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	var unused []string
	for token := range definitions {
		if !references[token] {
			unused = append(unused, token)
		}
	}
	slices.Sort(unused)
	if len(unused) > 0 {
		t.Errorf("caught: design tokens without production references: %s", strings.Join(unused, ", "))
	}
}
