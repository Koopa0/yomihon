package semantic

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const yomihonModulePath = "github.com/koopa0/yomihon"

var sensitiveSearchImports = map[string]struct{}{
	"bytes": {}, "cmp": {}, "context": {}, "crypto/sha256": {},
	"database/sql": {}, "embed": {}, "encoding/base64": {},
	"encoding/binary": {}, "encoding/hex": {}, "encoding/json": {},
	"errors": {}, "fmt": {}, "hash": {}, "io": {}, "io/fs": {},
	"maps": {}, "math": {}, "net": {}, "net/http": {}, "net/url": {},
	"os": {}, "path": {}, "path/filepath": {}, "slices": {},
	"strconv": {}, "strings": {}, "sync": {}, "time": {}, "unicode": {},
	"unicode/utf8": {},

	"github.com/koopa0/yomihon/internal/render":                   {},
	"github.com/koopa0/yomihon/internal/schema":                   {},
	"github.com/koopa0/yomihon/internal/search":                   {},
	"github.com/koopa0/yomihon/internal/search/agent":             {},
	"github.com/koopa0/yomihon/internal/search/evalset/recording": {},
	"github.com/koopa0/yomihon/internal/search/semantic":          {},
	"github.com/koopa0/yomihon/internal/search/semantic/catalog":  {},
	"github.com/koopa0/yomihon/internal/search/semantic/observer": {},
	"github.com/koopa0/yomihon/internal/vault":                    {},
	"golang.org/x/sys/unix":                                       {},
	"modernc.org/sqlite":                                          {},
	"modernc.org/sqlite/lib":                                      {},
}

var privilegedSearchImports = map[string]map[string]struct{}{
	"github.com/koopa0/yomihon/internal/search/evalset/recording": {
		"internal/search/evalset/agent.go":    {},
		"internal/search/evalset/semantic.go": {},
		"internal/search/evalset/suite.go":    {},
	},
	"net": {
		"internal/search/semantic/provider.go": {},
	},
	"net/http": {
		"internal/search/semantic/provider.go": {},
	},
	"golang.org/x/sys/unix": {
		"internal/search/semantic/lease_darwin.go": {},
		"internal/search/semantic/lease_linux.go":  {},
	},
	"modernc.org/sqlite": {
		"internal/search/semantic/store.go": {},
	},
	"modernc.org/sqlite/lib": {
		"internal/search/semantic/store.go": {},
	},
	"database/sql": {
		"internal/search/semantic/catalog/db.go":        {},
		"internal/search/semantic/catalog/models.go":    {},
		"internal/search/semantic/catalog/query.sql.go": {},
		"internal/search/semantic/manifest.go":          {},
		"internal/search/semantic/schema.go":            {},
		"internal/search/semantic/staging.go":           {},
		"internal/search/semantic/store.go":             {},
		"internal/search/semantic/writer.go":            {},
	},
}

var approvedSearchPackages = map[string]struct{}{
	"github.com/koopa0/yomihon/internal/graph":                    {},
	"github.com/koopa0/yomihon/internal/lesson":                   {},
	"github.com/koopa0/yomihon/internal/nav":                      {},
	"github.com/koopa0/yomihon/internal/origin":                   {},
	"github.com/koopa0/yomihon/internal/render":                   {},
	"github.com/koopa0/yomihon/internal/schema":                   {},
	"github.com/koopa0/yomihon/internal/search":                   {},
	"github.com/koopa0/yomihon/internal/search/agent":             {},
	"github.com/koopa0/yomihon/internal/search/evalset":           {},
	"github.com/koopa0/yomihon/internal/search/evalset/recording": {},
	"github.com/koopa0/yomihon/internal/search/semantic":          {},
	"github.com/koopa0/yomihon/internal/search/semantic/catalog":  {},
	"github.com/koopa0/yomihon/internal/search/semantic/observer": {},
	// sequence reads a study path's declared structure out of a Markdown body
	// already in memory. Reviewed for this closure: its whole import set is
	// goldmark, the vault's own wikilink splitter, and strings — it opens no
	// file and makes no network call.
	"github.com/koopa0/yomihon/internal/sequence":   {},
	"github.com/koopa0/yomihon/internal/ui/layouts": {},
	"github.com/koopa0/yomihon/internal/ui/pages":   {},
	"github.com/koopa0/yomihon/internal/vault":      {},
}

var approvedSearchModules = map[string]struct{}{
	"github.com/BurntSushi/toml":       {},
	"github.com/a-h/templ":             {},
	"github.com/alecthomas/chroma/v3":  {},
	"github.com/dlclark/regexp2/v2":    {},
	"github.com/dustin/go-humanize":    {},
	"github.com/remyoudompheng/bigfft": {},
	"github.com/yuin/goldmark":         {},
	"golang.org/x/sys":                 {},
	"golang.org/x/text":                {},
	"gopkg.in/yaml.v3":                 {},
	"modernc.org/libc":                 {},
	"modernc.org/mathutil":             {},
	"modernc.org/memory":               {},
	"modernc.org/sqlite":               {},
}

var approvedSearchModulesByOS = map[string]map[string]struct{}{
	"darwin": {
		"github.com/google/uuid":         {},
		"github.com/mattn/go-isatty":     {},
		"github.com/ncruces/go-strftime": {},
	},
	"linux": {
		"github.com/google/uuid": {},
	},
	"windows": {
		"github.com/mattn/go-isatty":     {},
		"github.com/ncruces/go-strftime": {},
	},
}

var providerChokePointIdentifiers = map[string]struct{}{
	"actionProvider":                  {},
	"chunkSender":                     {},
	"embedChunk":                      {},
	"embedQuery":                      {},
	"geminiEmbedder":                  {},
	"geminiWire":                      {},
	"newGeminiEmbedder":               {},
	"newGeminiWire":                   {},
	"newIndexerWithProvider":          {},
	"newProductionEmbeddingTransport": {},
	"openActionProvider":              {},
	"openProvider":                    {},
	"providerFactory":                 {},
	"productionEmbeddingTransport":    {},
	"querySender":                     {},
}

var ownedProductionProviderChokePoints = map[string]int{
	"internal/search/semantic/build.go:<package>:actionProvider":                      2,
	"internal/search/semantic/build.go:<package>:chunkSender":                         2,
	"internal/search/semantic/build.go:<package>:embedChunk":                          1,
	"internal/search/semantic/build.go:<package>:embedQuery":                          1,
	"internal/search/semantic/build.go:<package>:openProvider":                        1,
	"internal/search/semantic/build.go:<package>:providerFactory":                     2,
	"internal/search/semantic/build.go:<package>:querySender":                         2,
	"internal/search/semantic/build.go:NewIndexer:productionEmbeddingTransport":       1,
	"internal/search/semantic/build.go:embedFull:embedChunk":                          1,
	"internal/search/semantic/build.go:embedFull:openActionProvider":                  1,
	"internal/search/semantic/build.go:embedFullBuildChunk:chunkSender":               1,
	"internal/search/semantic/build.go:embedInteractive:actionProvider":               1,
	"internal/search/semantic/build.go:embedInteractive:embedChunk":                   1,
	"internal/search/semantic/build.go:newIndexer:actionProvider":                     2,
	"internal/search/semantic/build.go:newIndexer:embedChunk":                         1,
	"internal/search/semantic/build.go:newIndexer:embedQuery":                         1,
	"internal/search/semantic/build.go:newIndexer:newGeminiEmbedder":                  1,
	"internal/search/semantic/build.go:newIndexer:newIndexerWithProvider":             1,
	"internal/search/semantic/build.go:newIndexer:openProvider":                       2,
	"internal/search/semantic/build.go:newIndexerWithProvider:newIndexerWithProvider": 1,
	"internal/search/semantic/build.go:newIndexerWithProvider:openProvider":           4,
	"internal/search/semantic/build.go:newIndexerWithProvider:providerFactory":        1,
	"internal/search/semantic/build.go:openActionProvider:actionProvider":             1,
	"internal/search/semantic/build.go:openActionProvider:embedChunk":                 1,
	"internal/search/semantic/build.go:openActionProvider:embedQuery":                 1,
	"internal/search/semantic/build.go:openActionProvider:openActionProvider":         1,
	"internal/search/semantic/build.go:openActionProvider:openProvider":               1,
	"internal/search/semantic/build.go:validateIndexer:openProvider":                  2,
	"internal/search/semantic/build.go:validateIndexer:providerFactory":               1,
	"internal/search/semantic/reconcile.go:<package>:querySender":                     1,
	"internal/search/semantic/reconcile.go:newReadySearch:actionProvider":             1,
	"internal/search/semantic/reconcile.go:newReadySearch:embedQuery":                 2,
	"internal/search/semantic/reconcile.go:reconcileCorpus:openActionProvider":        2,
	"internal/search/semantic/reconcile.go:reconcileGeneration:actionProvider":        1,
	"internal/search/semantic/reconcile.go:recoverAfterWriterHeld:actionProvider":     1,
}

var forbiddenSensitiveSourceExtensions = map[string]struct{}{
	".c":       {},
	".cc":      {},
	".cpp":     {},
	".cxx":     {},
	".f":       {},
	".f90":     {},
	".for":     {},
	".h":       {},
	".hh":      {},
	".hpp":     {},
	".m":       {},
	".mm":      {},
	".s":       {},
	".swig":    {},
	".swigcxx": {},
	".syso":    {},
}

func TestSensitiveSearchImportsStayOwned(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	directories := []string{
		"internal/search/agent",
		"internal/search/evalset",
		"internal/search/semantic",
	}
	var violations []string
	seenImports := make(map[string]struct{}, len(sensitiveSearchImports))
	seenPrivileged := make(map[string]map[string]struct{}, len(privilegedSearchImports))
	for _, directory := range directories {
		for _, source := range productionGoSources(t, filepath.Join(root, directory)) {
			rel := relativeSourcePath(t, root, source)
			parsed, fset := parseGoFile(t, source)
			for _, spec := range parsed.Imports {
				importPath, err := strconv.Unquote(spec.Path.Value)
				if err != nil {
					t.Fatal(err)
				}
				position := fset.Position(spec.Pos())
				if spec.Name != nil && spec.Name.Name == "." {
					violations = append(violations, fmt.Sprintf("%s:%d dot-imports %s", rel, position.Line, importPath))
					continue
				}
				if _, allowed := sensitiveSearchImports[importPath]; !allowed {
					violations = append(violations, fmt.Sprintf("%s:%d imports unreviewed capability %s", rel, position.Line, importPath))
					continue
				}
				seenImports[importPath] = struct{}{}
				if owners, privileged := privilegedSearchImports[importPath]; privileged {
					if _, owned := owners[rel]; !owned {
						violations = append(violations, fmt.Sprintf("%s:%d imports privileged capability %s", rel, position.Line, importPath))
						continue
					}
					if seenPrivileged[importPath] == nil {
						seenPrivileged[importPath] = make(map[string]struct{})
					}
					seenPrivileged[importPath][rel] = struct{}{}
				}
			}
		}
	}
	for importPath := range sensitiveSearchImports {
		if _, seen := seenImports[importPath]; !seen {
			violations = append(violations, "stale approved import "+importPath)
		}
	}
	for importPath, owners := range privilegedSearchImports {
		for owner := range owners {
			if _, seen := seenPrivileged[importPath][owner]; !seen {
				violations = append(violations, fmt.Sprintf("stale privileged owner %s for %s", owner, importPath))
			}
		}
	}
	if len(violations) != 0 {
		slices.Sort(violations)
		t.Fatalf("the sensitive search import boundary changed without an explicit owner review:\n%s", strings.Join(violations, "\n"))
	}
}

func TestSensitiveSearchPackagesRejectAlternateCompiledSources(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	var violations []string
	for _, directory := range []string{"internal/search/agent", "internal/search/semantic"} {
		base := filepath.Join(root, filepath.FromSlash(directory))
		err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			extension := strings.ToLower(filepath.Ext(entry.Name()))
			if _, forbidden := forbiddenSensitiveSourceExtensions[extension]; !forbidden {
				return nil
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			violations = append(violations, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(violations) != 0 {
		slices.Sort(violations)
		t.Fatalf("semantic/agent gained a non-Go compiled source that bypasses AST capability locks:\n%s", strings.Join(violations, "\n"))
	}
}

func TestSensitiveSearchDependencyClosureStaysOwned(t *testing.T) {
	t.Parallel()

	command := exec.CommandContext(
		t.Context(),
		"go", "list", "-deps", "-json",
		"./internal/search/agent", "./internal/search/evalset",
	)
	command.Dir = repositoryRoot(t)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("list sensitive search dependency closure: %v\n%s", err, output)
	}

	type listedPackage struct {
		ImportPath string
		Standard   bool
		Module     *struct {
			Path string
		}
	}
	var violations []string
	seenPackages := make(map[string]struct{}, len(approvedSearchPackages))
	platformModules := approvedSearchModulesByOS[runtime.GOOS]
	seenModules := make(map[string]struct{}, len(approvedSearchModules)+len(platformModules))
	decoder := json.NewDecoder(bytes.NewReader(output))
	for {
		var listed listedPackage
		if err := decoder.Decode(&listed); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("decode sensitive search dependency closure: %v", err)
		}
		if listed.Standard || listed.Module == nil {
			continue
		}
		if listed.Module.Path == yomihonModulePath {
			if _, approved := approvedSearchPackages[listed.ImportPath]; !approved {
				violations = append(violations, "unreviewed repository package "+listed.ImportPath)
			} else {
				seenPackages[listed.ImportPath] = struct{}{}
			}
			continue
		}
		_, approvedCommon := approvedSearchModules[listed.Module.Path]
		_, approvedPlatform := platformModules[listed.Module.Path]
		if !approvedCommon && !approvedPlatform {
			violations = append(violations, "unreviewed external module "+listed.Module.Path)
		} else {
			seenModules[listed.Module.Path] = struct{}{}
		}
	}
	for importPath := range approvedSearchPackages {
		if _, seen := seenPackages[importPath]; !seen {
			violations = append(violations, "stale approved repository package "+importPath)
		}
	}
	for modulePath := range approvedSearchModules {
		if _, seen := seenModules[modulePath]; !seen {
			violations = append(violations, "stale approved external module "+modulePath)
		}
	}
	for modulePath := range platformModules {
		if _, seen := seenModules[modulePath]; !seen {
			violations = append(violations, "stale approved platform module "+modulePath)
		}
	}
	if len(violations) != 0 {
		slices.Sort(violations)
		t.Fatalf("the sensitive search dependency closure gained a new capability:\n%s", strings.Join(violations, "\n"))
	}
}

func TestEmbeddingTransportSymbolsHaveNamedOwners(t *testing.T) {
	t.Parallel()

	want := map[string]int{
		"NewIndexerWithTransport internal/search/agent/run_test.go":                       1,
		"NewIndexerWithTransport internal/search/semantic/provider.go":                    1,
		"NewIndexerWithTransport internal/search/semantic/provider_test.go":               1,
		"newProductionEmbeddingTransport internal/search/semantic/egress_test.go":         1,
		"newProductionEmbeddingTransport internal/search/semantic/provider.go":            2,
		"newProductionEmbeddingTransport internal/search/semantic/provider_live_test.go":  3,
		"newProductionEmbeddingTransport internal/search/semantic/recording_live_test.go": 1,
	}
	got := make(map[string]int)
	root := repositoryRoot(t)
	for _, source := range allGoSources(t, root) {
		rel := relativeSourcePath(t, root, source)
		parsed, _ := parseGoFile(t, source)
		ast.Inspect(parsed, func(node ast.Node) bool {
			identifier, ok := node.(*ast.Ident)
			if !ok {
				return true
			}
			name := identifier.Name
			if name == "newProductionEmbeddingTransport" || name == "NewIndexerWithTransport" {
				got[name+" "+rel]++
			}
			return true
		})
	}
	if diff := mapCountDiff(want, got); diff != "" {
		t.Fatalf("embedding transport ownership changed:\n%s", diff)
	}
}

func TestProviderChokePointsHaveExactProductionReferences(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	got := make(map[string]int)
	for _, directory := range []string{"internal/search/agent", "internal/search/semantic"} {
		for _, source := range productionGoSources(t, filepath.Join(root, filepath.FromSlash(directory))) {
			rel := relativeSourcePath(t, root, source)
			if rel == providerOwnerPath {
				continue
			}
			parsed, _ := parseGoFile(t, source)
			for _, declaration := range parsed.Decls {
				function := "<package>"
				if declared, ok := declaration.(*ast.FuncDecl); ok {
					function = declared.Name.Name
				}
				ast.Inspect(declaration, func(node ast.Node) bool {
					identifier, ok := node.(*ast.Ident)
					if !ok {
						return true
					}
					if _, tracked := providerChokePointIdentifiers[identifier.Name]; tracked {
						got[rel+":"+function+":"+identifier.Name]++
					}
					return true
				})
			}
		}
	}
	if diff := mapCountDiff(ownedProductionProviderChokePoints, got); diff != "" {
		t.Fatalf("provider choke-point ownership changed:\n%s", diff)
	}
}

func mapCountDiff(want, got map[string]int) string {
	keys := make([]string, 0, len(want)+len(got))
	seen := make(map[string]struct{}, len(want)+len(got))
	for key := range want {
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	for key := range got {
		if _, ok := seen[key]; !ok {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	var differences []string
	for _, key := range keys {
		if want[key] != got[key] {
			differences = append(differences, fmt.Sprintf("%s: got %d, want %d", key, got[key], want[key]))
		}
	}
	return strings.Join(differences, "\n")
}

func relativeSourcePath(t *testing.T, root, source string) string {
	t.Helper()
	rel, err := filepath.Rel(root, source)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(rel)
}

func allGoSources(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "vendor" || entry.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestSensitiveSearchImportBoundaryRejectsAlternateEgress(t *testing.T) {
	t.Parallel()

	const source = `package agent
import (
  "os/exec"
  "unsafe"
)
func send(query string) {
  _, _ = exec.Command("curl", "--data-binary", query).Output()
  _ = unsafe.Sizeof(query)
}`
	parsed, err := parser.ParseFile(token.NewFileSet(), "alternate.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range parsed.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		if _, allowed := sensitiveSearchImports[path]; allowed {
			t.Fatalf("alternate egress import %q is unexpectedly allowed", path)
		}
	}
}
