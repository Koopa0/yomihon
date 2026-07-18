package semantic

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

var forbiddenObservabilityImports = map[string]struct{}{
	"expvar":             {},
	"log":                {},
	"log/slog":           {},
	"net/http/httptrace": {},
	"net/http/httputil":  {},
	"runtime/pprof":      {},
	"runtime/trace":      {},
}

var forbiddenObservabilityImportSegments = map[string]struct{}{
	"log":        {},
	"logging":    {},
	"metrics":    {},
	"otel":       {},
	"prometheus": {},
	"statsd":     {},
	"telemetry":  {},
	"zerolog":    {},
	"zap":        {},
}

var ownedDirectWrites = map[string]map[string]int{
	"internal/search/agent/wire.go": {
		"fmt.Fprintf":    5,
		"io.WriteString": 1,
		"method.Encode":  1,
		"method.Write":   1,
	},
	"internal/search/semantic/identity.go": {"method.Write": 2},
	"internal/search/semantic/lease.go":    {"method.OpenFile": 1},
	"internal/search/semantic/manifest.go": {"method.Write": 11},
	"internal/search/semantic/schema.go":   {"method.Write": 2},
	"internal/search/semantic/store.go": {
		"method.Encode":   1,
		"method.Mkdir":    2,
		"method.OpenFile": 2,
		"method.Remove":   1,
	},
}

func TestAgentSearchHasNoUnownedObservabilitySink(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	var violations []string
	for _, dir := range []string{"internal/search/agent", "internal/search/semantic"} {
		for _, source := range productionGoSources(t, filepath.Join(root, filepath.FromSlash(dir))) {
			parsed, fset := parseGoFile(t, source)
			rel, err := filepath.Rel(root, source)
			if err != nil {
				t.Fatal(err)
			}
			for _, sink := range observabilitySinks(fset, parsed) {
				violations = append(violations, filepath.ToSlash(rel)+":"+sink)
			}
		}
	}
	if len(violations) != 0 {
		t.Fatalf("agent search gained an unowned observability sink:\n%s", strings.Join(violations, "\n"))
	}
}

func TestAgentSearchDirectWritesMatchOwnedInventory(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	seen := make(map[string]bool, len(ownedDirectWrites))
	var violations []string
	for _, dir := range []string{"internal/search/agent", "internal/search/semantic"} {
		for _, source := range productionGoSources(t, filepath.Join(root, filepath.FromSlash(dir))) {
			parsed, fset := parseGoFile(t, source)
			rel, err := filepath.Rel(root, source)
			if err != nil {
				t.Fatal(err)
			}
			rel = filepath.ToSlash(rel)
			got := countSinkClasses(directWriteSinks(fset, parsed))
			want := ownedDirectWrites[rel]
			if want != nil {
				seen[rel] = true
			}
			if !maps.Equal(got, want) {
				violations = append(violations, fmt.Sprintf("%s: direct writes = %v, want owned inventory %v", rel, got, want))
			}
		}
	}
	for rel := range ownedDirectWrites {
		if !seen[rel] {
			violations = append(violations, rel+": owned direct-write source is missing")
		}
	}
	if len(violations) != 0 {
		slices.Sort(violations)
		t.Fatalf("agent search direct-write ownership changed:\n%s", strings.Join(violations, "\n"))
	}
}

func TestObservabilitySinkDetectorRejectsEverySupportedBypass(t *testing.T) {
	t.Parallel()

	const source = `package alternate
import (
  "fmt"
  "log/slog"
  "os"
  "runtime/trace"
  telemetry "example.invalid/telemetry/client"
)
func leak() {
  fmt.Println("query")
  _, _ = fmt.Fprintln(os.Stderr, "key")
  _, _ = fmt.Fprintln(os.Stdout, "document")
  println("document")
  _, _, _ = slog.Default, trace.Start, telemetry.Send
}`
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "alternate.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := sinkClasses(observabilitySinks(fset, parsed))
	want := []string{
		"builtin println",
		"fmt.Println",
		"import example.invalid/telemetry/client",
		"import log/slog",
		"import runtime/trace",
		"os.Stderr",
		"os.Stdout",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("observabilitySinks() = %v, want exact set %v", got, want)
	}

	const dotImport = `package alternate
import . "fmt"
func leak() { Println("query") }`
	parsed, err = parser.ParseFile(fset, "dot.go", dotImport, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := observabilitySinks(fset, parsed); len(got) != 1 {
		t.Fatalf("observabilitySinks(dot import) = %v, want one import violation", got)
	}
}

func TestDirectWriteDetectorRejectsEverySupportedBypass(t *testing.T) {
	t.Parallel()

	const source = `package alternate
import (
  "fmt"
  "io"
  "os"
)
func leak(sink io.Writer, enc interface{ Encode(any) error }) {
  _ = os.Chmod("cache", 0600)
  _ = os.Link("cache", "link")
  _ = os.Mkdir("cache", 0700)
  _ = os.MkdirAll("cache/tree", 0700)
  _, _ = os.MkdirTemp("", "cache")
  _ = os.Remove("cache")
  _ = os.RemoveAll("cache")
  _ = os.Rename("cache", "moved")
  _ = os.Symlink("cache", "link")
  _ = os.Truncate("cache", 0)
  _ = os.WriteFile("cache", []byte("query"), 0600)
  _, _ = os.OpenFile("cache", os.O_CREATE|os.O_WRONLY, 0600)
  _, _ = fmt.Fprintf(sink, "query")
  _, _ = io.WriteString(sink, "query")
  _, _ = sink.Write([]byte("query"))
  _ = enc.Encode("query")
}`
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "alternate.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := sinkClasses(directWriteSinks(fset, parsed))
	want := []string{
		"fmt.Fprintf",
		"io.WriteString",
		"method.Encode",
		"method.Write",
		"os.Chmod",
		"os.Link",
		"os.Mkdir",
		"os.MkdirAll",
		"os.MkdirTemp",
		"os.OpenFile",
		"os.Remove",
		"os.RemoveAll",
		"os.Rename",
		"os.Symlink",
		"os.Truncate",
		"os.WriteFile",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("directWriteSinks() = %v, want exact set %v", got, want)
	}

	const dotImport = `package alternate
import . "os"
func leak() { _ = WriteFile("cache", []byte("query"), 0600) }`
	parsed, err = parser.ParseFile(fset, "dot.go", dotImport, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := sinkClasses(directWriteSinks(fset, parsed)); !slices.Equal(got, []string{"dot-import os"}) {
		t.Fatalf("directWriteSinks(dot import) = %v, want dot-import violation", got)
	}
}

func TestDirectWriteDetectorInventoriesRootedFilesystemMutations(t *testing.T) {
	t.Parallel()

	const source = `package alternate
type root interface {
  Mkdir(string, uint32) error
  OpenFile(string, int, uint32) (any, error)
  Remove(string) error
  WriteFile(string, []byte, uint32) error
}
func mutate(files root) {
  _ = files.Mkdir("cache", 0700)
  _, _ = files.OpenFile("cache/data", 0, 0600)
  _ = files.Remove("cache/data")
  _ = files.WriteFile("cache/data", nil, 0600)
}`
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "alternate.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"method.Mkdir", "method.OpenFile", "method.Remove", "method.WriteFile"}
	if got := sinkClasses(directWriteSinks(fset, parsed)); !slices.Equal(got, want) {
		t.Fatalf("directWriteSinks(rooted mutations) = %v, want %v", got, want)
	}
}

func TestForbiddenObservabilityImportRecognizesWholePathSegments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "terminal logger segment", path: "example.invalid/log", want: true},
		{name: "OpenTelemetry module", path: "go.opentelemetry.io/otel", want: true},
		{name: "Prometheus module", path: "github.com/prometheus/client_golang/prometheus", want: true},
		{name: "ordinary semantic package", path: "github.com/koopa0/yomihon/internal/search/semantic", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := forbiddenObservabilityImport(tt.path); got != tt.want {
				t.Errorf("forbiddenObservabilityImport(%q) = %t, want %t", tt.path, got, tt.want)
			}
		})
	}
}

func observabilitySinks(fset *token.FileSet, file *ast.File) []string {
	imports := make(map[string]string, len(file.Imports))
	var sinks []string
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := filepath.Base(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		imports[name] = path
		if name == "." && (path == "fmt" || path == "os") {
			position := fset.Position(spec.Pos())
			sinks = append(sinks, fmt.Sprintf("%d:%d dot-import %s", position.Line, position.Column, path))
		}
		if forbiddenObservabilityImport(path) {
			position := fset.Position(spec.Pos())
			sinks = append(sinks, fmt.Sprintf("%d:%d import %s", position.Line, position.Column, path))
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.CallExpr:
			if ident, ok := node.Fun.(*ast.Ident); ok && (ident.Name == "print" || ident.Name == "println") {
				position := fset.Position(node.Pos())
				sinks = append(sinks, fmt.Sprintf("%d:%d builtin %s", position.Line, position.Column, ident.Name))
			}
			selector, ok := node.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			qualifier, ok := selector.X.(*ast.Ident)
			if ok && imports[qualifier.Name] == "fmt" && strings.HasPrefix(selector.Sel.Name, "Print") {
				position := fset.Position(node.Pos())
				sinks = append(sinks, fmt.Sprintf("%d:%d fmt.%s", position.Line, position.Column, selector.Sel.Name))
			}
		case *ast.SelectorExpr:
			qualifier, ok := node.X.(*ast.Ident)
			if ok && imports[qualifier.Name] == "os" && (node.Sel.Name == "Stderr" || node.Sel.Name == "Stdout") {
				position := fset.Position(node.Pos())
				sinks = append(sinks, fmt.Sprintf("%d:%d os.%s", position.Line, position.Column, node.Sel.Name))
			}
		}
		return true
	})
	return sinks
}

func directWriteSinks(fset *token.FileSet, file *ast.File) []string {
	imports := make(map[string]string, len(file.Imports))
	var sinks []string
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := filepath.Base(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == "." && (path == "fmt" || path == "io" || path == "os") {
			position := fset.Position(spec.Pos())
			sinks = append(sinks, fmt.Sprintf("%d:%d dot-import %s", position.Line, position.Column, path))
			continue
		}
		imports[name] = path
	}
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		position := fset.Position(selector.Pos())
		qualifier, qualified := selector.X.(*ast.Ident)
		if qualified {
			switch imports[qualifier.Name] {
			case "fmt":
				if strings.HasPrefix(selector.Sel.Name, "Fprint") {
					sinks = append(sinks, fmt.Sprintf("%d:%d fmt.%s", position.Line, position.Column, selector.Sel.Name))
				}
				return true
			case "io":
				if selector.Sel.Name == "WriteString" || selector.Sel.Name == "Copy" || selector.Sel.Name == "CopyBuffer" {
					sinks = append(sinks, fmt.Sprintf("%d:%d io.%s", position.Line, position.Column, selector.Sel.Name))
				}
				return true
			case "os":
				if directFileWriteCapability(selector.Sel.Name) {
					sinks = append(sinks, fmt.Sprintf("%d:%d os.%s", position.Line, position.Column, selector.Sel.Name))
				}
				return true
			}
		}
		if directFileWriteCapability(selector.Sel.Name) {
			sinks = append(sinks, fmt.Sprintf("%d:%d method.%s", position.Line, position.Column, selector.Sel.Name))
			return true
		}
		switch selector.Sel.Name {
		case "Encode", "ReadFrom", "Write", "WriteAt", "WriteString", "WriteTo":
			sinks = append(sinks, fmt.Sprintf("%d:%d method.%s", position.Line, position.Column, selector.Sel.Name))
		}
		return true
	})
	return sinks
}

func directFileWriteCapability(name string) bool {
	switch name {
	case "Chmod", "Chown", "Chtimes", "Create", "CreateTemp", "Lchown", "Link", "Mkdir", "MkdirAll",
		"MkdirTemp", "NewFile", "OpenFile", "Remove", "RemoveAll", "Rename", "Symlink", "Truncate", "WriteFile":
		return true
	default:
		return false
	}
}

func sinkClasses(sinks []string) []string {
	classes := make([]string, 0, len(sinks))
	for _, sink := range sinks {
		_, class, ok := strings.Cut(sink, " ")
		if ok {
			classes = append(classes, class)
		}
	}
	slices.Sort(classes)
	return classes
}

func countSinkClasses(sinks []string) map[string]int {
	counts := make(map[string]int)
	for _, class := range sinkClasses(sinks) {
		counts[class]++
	}
	return counts
}

func forbiddenObservabilityImport(path string) bool {
	if _, forbidden := forbiddenObservabilityImports[path]; forbidden {
		return true
	}
	for segment := range strings.SplitSeq(path, "/") {
		if _, forbidden := forbiddenObservabilityImportSegments[segment]; forbidden {
			return true
		}
	}
	return false
}
