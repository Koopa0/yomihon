package render

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"
	mdast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// The reader copies complete programs from both lessons and author templates.
// Discover their fences from the entire example vault so a new note cannot
// escape the same formatting and offline type checks.
func TestExampleGoPrograms(t *testing.T) {
	root, openErr := os.OpenRoot(exampleVaultRoot)
	if openErr != nil {
		t.Fatalf("open example vault: %v", openErr)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("close example vault: %v", err)
		}
	})
	fset := token.NewFileSet()
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	programs := 0
	err := fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		body, err := root.ReadFile(path)
		if err != nil {
			return err
		}
		for i, src := range exampleGoPrograms(t, body) {
			programs++
			t.Run(filepath.ToSlash(path)+"/program-"+strconv.Itoa(i+1), func(t *testing.T) {
				formatted, err := format.Source(src)
				if err != nil {
					t.Fatalf("format.Source(program) failed: %v", err)
				}
				if diff := cmp.Diff(string(src), string(formatted)); diff != "" {
					t.Errorf("program is not gofmt-clean (-authored +formatted):\n%s", diff)
				}
				file, err := parser.ParseFile(fset, path, src, parser.AllErrors)
				if err != nil {
					t.Fatalf("parser.ParseFile(program) failed: %v", err)
				}
				if _, err := conf.Check("example", fset, []*ast.File{file}, nil); err != nil {
					t.Errorf("types.Check(program) failed: %v", err)
				}
			})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk example vault: %v", err)
	}
	if programs == 0 {
		t.Fatal("example vault contains no complete Go programs")
	}
	t.Logf("checked %d source-derived Go programs", programs)
}

func exampleGoPrograms(t *testing.T, body []byte) [][]byte {
	t.Helper()
	var programs [][]byte
	doc := goldmark.New().Parser().Parse(text.NewReader(body))
	if err := mdast.Walk(doc, func(node mdast.Node, entering bool) (mdast.WalkStatus, error) {
		fence, ok := node.(*mdast.FencedCodeBlock)
		if !entering || !ok || string(fence.Language(body)) != "go" {
			return mdast.WalkContinue, nil
		}
		src := fence.Lines().Value(body)
		if bytes.Contains(src, []byte("package main")) {
			programs = append(programs, src)
		}
		return mdast.WalkContinue, nil
	}); err != nil {
		t.Fatalf("walk Markdown fences: %v", err)
	}
	return programs
}

func TestExampleGoProgramDiscovery(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
		want []string
	}{
		{
			name: "all programs and no snippets or other languages",
			body: "```go\npackage main\n\nfunc main() {}\n```\n\n```go\nfmt.Println(42)\n```\n\n```text\npackage main\n```\n\n```go\npackage main\n\nfunc main() { println(42) }\n```\n",
			want: []string{"package main\n\nfunc main() {}\n", "package main\n\nfunc main() { println(42) }\n"},
		},
		{
			name: "nested longer and tilde fences",
			body: "> ````go\n> package main\n> \n> func main() {}\n> ````\n\n~~~go\npackage main\n\nfunc main() { println(42) }\n~~~\n",
			want: []string{"package main\n\nfunc main() {}\n", "package main\n\nfunc main() { println(42) }\n"},
		},
		{
			name: "no programs",
			body: "The words package main are prose.\n\n```go\nfunc example() {}\n```\n",
			want: nil,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, src := range exampleGoPrograms(t, []byte(tt.body)) {
				got = append(got, string(src))
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("exampleGoPrograms(Markdown) mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
