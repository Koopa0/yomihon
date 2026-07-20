package semantic

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/tools/go/packages"
)

const (
	providerOwnerPath            = "internal/search/semantic/provider.go"
	generationCatalogOwnerDir    = "internal/search/semantic"
	generationCatalogPackagePath = "github.com/koopa0/yomihon/internal/search/semantic/catalog"
)

var outboundSymbols = map[string]map[string]struct{}{
	"net/http": {
		"Client": {}, "DefaultClient": {}, "DefaultTransport": {},
		"Get": {}, "Head": {}, "ListenAndServe": {}, "ListenAndServeTLS": {},
		"NewRequest": {}, "NewRequestWithContext": {}, "Post": {}, "PostForm": {},
		"RoundTripper": {}, "Serve": {}, "ServeTLS": {}, "Transport": {},
	},
	"net": {
		"Dial": {}, "Dialer": {}, "DialIP": {}, "DialTCP": {}, "DialTimeout": {},
		"DialUDP": {}, "DialUnix": {}, "FileConn": {}, "FileListener": {},
		"FilePacketConn": {}, "Listen": {}, "ListenIP": {}, "ListenMulticastUDP": {},
		"ListenPacket": {}, "ListenTCP": {}, "ListenUDP": {}, "ListenUnix": {},
		"ListenUnixgram": {}, "LookupAddr": {}, "LookupCNAME": {}, "LookupHost": {},
		"LookupIP": {}, "LookupMX": {}, "LookupNS": {}, "LookupPort": {}, "LookupSRV": {},
		"LookupTXT": {}, "ResolveIPAddr": {}, "ResolveTCPAddr": {}, "ResolveUDPAddr": {},
		"ResolveUnixAddr": {},
	},
	"crypto/tls": {
		"Dial": {}, "Dialer": {}, "Listen": {}, "LoadX509KeyPair": {}, "NewListener": {},
	},
}

var outboundMethodNames = map[string]struct{}{
	"Dial":              {},
	"DialContext":       {},
	"Do":                {},
	"Get":               {},
	"Head":              {},
	"Listen":            {},
	"ListenAndServe":    {},
	"ListenAndServeTLS": {},
	"ListenPacket":      {},
	"LookupAddr":        {},
	"LookupCNAME":       {},
	"LookupHost":        {},
	"LookupIP":          {},
	"LookupIPAddr":      {},
	"LookupMX":          {},
	"LookupNS":          {},
	"LookupNetIP":       {},
	"LookupPort":        {},
	"LookupSRV":         {},
	"LookupTXT":         {},
	"Post":              {},
	"PostForm":          {},
	"RoundTrip":         {},
	"Serve":             {},
	"ServeTLS":          {},
}

var directFileSymbols = map[string]map[string]struct{}{
	"crypto/tls": {"LoadX509KeyPair": {}},
	"database/sql": {
		"Open":   {},
		"OpenDB": {},
	},
	"github.com/koopa0/yomihon/internal/schema": {
		"Load":       {},
		"LoadFile":   {},
		"LoadReader": {},
	},
	"github.com/koopa0/yomihon/internal/search/evalset/recording": {"Load": {}},
	"github.com/koopa0/yomihon/internal/vault":                    {"Open": {}},
	"io/fs": {"ReadFile": {}},
	"net/http": {
		"Dir":                {},
		"FileServer":         {},
		"FileServerFS":       {},
		"ListenAndServeTLS":  {},
		"NewFileTransport":   {},
		"NewFileTransportFS": {},
		"ServeFile":          {},
		"ServeFileFS":        {},
	},
	"path/filepath": {
		"EvalSymlinks": {},
		"Glob":         {},
		"Walk":         {},
		"WalkDir":      {},
	},
}

// vaultReaderAccessMethods is checked against vault.Reader's complete exported
// method surface below. The three exclusions release/describe/compare an
// already-pinned capability; every method that can enumerate, select, refresh,
// or read vault state must remain visible to the owner inventory.
var vaultReaderAccessMethods = map[string]struct{}{
	"Entries":       {},
	"Lookup":        {},
	"OpenFile":      {},
	"ReadFile":      {},
	"ReadPrefix":    {},
	"Refresh":       {},
	"ScanAvailable": {},
	"ScanComplete":  {},
}

var nonAccessVaultReaderMethods = map[string]struct{}{
	"Close":    {},
	"Name":     {},
	"SameRoot": {},
}

// osFunctionSymbols follows the complete exported os function surface. The
// two explicit exclusions inspect already-open identities or return a cache
// location; every other os function must remain visible to the owner lock.
var osFunctionSymbols = func() map[string]struct{} {
	pkg, err := importer.Default().Import("os")
	if err != nil {
		panic(fmt.Sprintf("import os API for boundary test: %v", err))
	}
	functions := make(map[string]struct{})
	for _, name := range pkg.Scope().Names() {
		if _, ok := pkg.Scope().Lookup(name).(*types.Func); ok {
			functions[name] = struct{}{}
		}
	}
	return functions
}()

var nonPathOSFunctions = map[string]struct{}{
	"SameFile":     {},
	"UserCacheDir": {},
}

var nonPathUnixSymbols = map[string]struct{}{
	"EWOULDBLOCK": {},
	"Flock":       {},
	"LOCK_EX":     {},
	"LOCK_NB":     {},
	"LOCK_UN":     {},
}

var ownedSemanticFileAccess = func() map[string]map[string]int {
	accesses := map[string]map[string]int{
		"internal/search/agent/command.go": {
			"loadVaultCapabilities:github.com/koopa0/yomihon/internal/schema.LoadReader": 1,
			"loadVaultCapabilities:github.com/koopa0/yomihon/internal/vault.Open":        1,
		},
		"internal/search/agent/snapshot.go": {
			"readSnapshotNotes:github.com/koopa0/yomihon/internal/vault.Reader.Entries":  1,
			"readSnapshotNotes:github.com/koopa0/yomihon/internal/vault.Reader.ReadFile": 1,
		},
		"internal/search/semantic/build.go": {
			"resolvePlannedLocation:os.Lstat":                   1,
			"resolvePlannedLocation:path/filepath.EvalSymlinks": 1,
			"validateStoreLocation:path/filepath.EvalSymlinks":  1,
		},
		"internal/search/semantic/corpus.go": {
			"readChunks:github.com/koopa0/yomihon/internal/vault.Reader.Entries":      1,
			"readNoteChunks:github.com/koopa0/yomihon/internal/vault.Reader.ReadFile": 1,
		},
		"internal/search/semantic/egress.go": {
			"authorizeChunk:github.com/koopa0/yomihon/internal/vault.Reader.ReadFile": 1,
		},
		"internal/search/semantic/lease.go": {
			"openPrivateLeaseFile:os.File.Stat":     1,
			"openPrivateLeaseFile:os.Root.Lstat":    1,
			"openPrivateLeaseFile:os.Root.OpenFile": 1,
		},
		"internal/search/semantic/store.go": {
			"createDatabase:os.File.Stat":                 1,
			"createDatabase:os.Root.OpenFile":             1,
			"createStoreParent:os.OpenRoot":               1,
			"createStoreParent:os.Root.Stat":              1,
			"inspectDatabase:os.Root.Lstat":               1,
			"lstatStorePath:os.OpenRoot":                  2,
			"lstatStorePath:os.Root.Lstat":                1,
			"lstatStorePath:os.Root.Stat":                 1,
			"openOrCreateCacheDirectory:os.Root.Lstat":    2,
			"openOrCreateCacheDirectory:os.Root.Mkdir":    1,
			"openOrCreateCacheDirectory:os.Root.OpenRoot": 1,
			"openOrCreateCacheDirectory:os.Root.Stat":     1,
			"openStoreDB:database/sql.Open":               1,
			"openStoreDirectory:os.Root.Lstat":            2,
			"openStoreDirectory:os.Root.Mkdir":            1,
			"openStoreDirectory:os.Root.OpenRoot":         1,
			"openStoreDirectory:os.Root.Stat":             1,
			"openStoreParent:os.OpenRoot":                 1,
			"requireCurrent:os.Root.Lstat":                2,
			"requireCurrent:os.Root.Stat":                 2,
			"requirePrivateSidecars:os.Root.Lstat":        1,
			"resetDatabaseFiles:os.File.Stat":             1,
			"resetDatabaseFiles:os.Root.OpenFile":         1,
			"resetDatabaseFiles:os.Root.Remove":           1,
		},
	}
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		accesses["internal/search/semantic/lease_"+runtime.GOOS+".go"] = map[string]int{
			"lockWriterFileNonblocking:os.File.Fd": 1,
			"unlockWriterFile:os.File.Fd":          1,
		}
	}
	return accesses
}()

var ownedSemanticOutboundReferences = map[string]map[string]int{
	"internal/search/semantic/provider.go": {
		"<package>:net/http.Client":                              1,
		"<package>:net/http.RoundTripper":                        1,
		"NewIndexerWithTransport:net/http.RoundTripper":          2,
		"embed:net/http.Client.Do":                               1,
		"embed:net/http.NewRequestWithContext":                   1,
		"newGeminiEmbedder:net/http.RoundTripper":                1,
		"newGeminiWire:net/http.Client":                          1,
		"newGeminiWire:net/http.RoundTripper":                    1,
		"newProductionEmbeddingTransport:net.Dialer.DialContext": 1,
		"newProductionEmbeddingTransport:net.Dialer":             1,
		"newProductionEmbeddingTransport:net/http.Transport":     2,
		"productionEmbeddingTransport:net/http.RoundTripper":     1,
	},
}

type directFileAccess struct {
	function string
	symbol   string
	line     int
	column   int
}

type outboundReference struct {
	function string
	symbol   string
	line     int
	column   int
}

func TestEnumeratedDirectEmbeddingTransportsHaveOneProductionOwner(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	var violations []string
	for _, source := range productionGoSources(t, root) {
		rel, err := filepath.Rel(root, source)
		if err != nil {
			t.Fatal(err)
		}
		rel = filepath.ToSlash(rel)
		if rel == providerOwnerPath {
			continue
		}
		parsed, fset := parseGoFile(t, source)
		for _, reach := range outboundReaches(fset, parsed) {
			violations = append(violations, rel+":"+reach)
		}
	}
	if len(violations) != 0 {
		t.Fatalf("an enumerated direct outbound constructor escaped the provider owner:\n%s", strings.Join(violations, "\n"))
	}
}

func TestSemanticAndAgentNetworkCapabilitiesHaveExactOwners(t *testing.T) {
	root := repositoryRoot(t)
	seenOwned := make(map[string]map[string]int)
	var violations []string
	for _, loaded := range loadBoundaryPackages(t, root, "./internal/search/agent/...", "./internal/search/semantic/...") {
		for index, parsed := range loaded.Syntax {
			rel := relativeSourcePath(t, root, loaded.CompiledGoFiles[index])
			for _, reference := range typedOutboundReferences(loaded.Fset, loaded.TypesInfo, parsed) {
				key := reference.function + ":" + reference.symbol
				if _, owned := ownedSemanticOutboundReferences[rel][key]; owned {
					if seenOwned[rel] == nil {
						seenOwned[rel] = make(map[string]int)
					}
					seenOwned[rel][key]++
					continue
				}
				violations = append(violations, fmt.Sprintf(
					"%s:%d:%d %s",
					rel, reference.line, reference.column, key,
				))
			}
		}
	}
	for rel, references := range ownedSemanticOutboundReferences {
		for reference, want := range references {
			if got := seenOwned[rel][reference]; got != want {
				violations = append(violations, fmt.Sprintf(
					"%s: owned network reference %s count = %d, want %d",
					rel, reference, got, want,
				))
			}
		}
	}
	if len(violations) != 0 {
		slices.Sort(violations)
		t.Fatalf("semantic/agent network capabilities changed outside their exact owners:\n%s", strings.Join(violations, "\n"))
	}
}

func TestSemanticAndAgentPackagesRestrictDirectFileAccess(t *testing.T) {
	root := repositoryRoot(t)
	seenOwned := make(map[string]map[string]int)
	var violations []string
	for _, loaded := range loadBoundaryPackages(t, root, "./internal/search/agent/...", "./internal/search/semantic/...") {
		for index, parsed := range loaded.Syntax {
			rel := relativeSourcePath(t, root, loaded.CompiledGoFiles[index])
			for _, access := range typedDirectFileAccesses(loaded.Fset, loaded.TypesInfo, parsed) {
				key := access.function + ":" + access.symbol
				if _, owned := ownedSemanticFileAccess[rel][key]; owned {
					if seenOwned[rel] == nil {
						seenOwned[rel] = make(map[string]int)
					}
					seenOwned[rel][key]++
					continue
				}
				violations = append(violations, fmt.Sprintf("%s:%d:%d %s", rel, access.line, access.column, key))
			}
		}
	}
	for rel, accesses := range ownedSemanticFileAccess {
		for access, want := range accesses {
			if got := seenOwned[rel][access]; got != want {
				violations = append(violations, fmt.Sprintf(
					"%s: owned direct file access %s count = %d, want %d",
					rel, access, got, want,
				))
			}
		}
	}
	if len(violations) != 0 {
		t.Fatalf("semantic or agent package added direct file access outside the owned local-store inventory:\n%s", strings.Join(violations, "\n"))
	}
}

func TestVaultReaderAccessInventoryCoversExportedMethods(t *testing.T) {
	root := repositoryRoot(t)
	loaded := loadBoundaryPackages(t, root, "./internal/vault")
	if len(loaded) != 1 {
		t.Fatalf("load vault package count = %d, want 1", len(loaded))
	}
	readerObject, ok := loaded[0].Types.Scope().Lookup("Reader").(*types.TypeName)
	if !ok {
		t.Fatal("vault.Reader type is missing")
	}
	got := make(map[string]struct{})
	methods := types.NewMethodSet(types.NewPointer(readerObject.Type()))
	for selection := range methods.Methods() {
		method := selection.Obj()
		if !method.Exported() {
			continue
		}
		if _, excluded := nonAccessVaultReaderMethods[method.Name()]; excluded {
			continue
		}
		got[method.Name()] = struct{}{}
	}
	if diff := cmp.Diff(vaultReaderAccessMethods, got); diff != "" {
		t.Fatalf("vault.Reader access-method classification changed (-want +got):\n%s", diff)
	}
}

func TestRootedReaderBoundaryDetectsDirectFileAccesses(t *testing.T) {
	t.Parallel()

	const source = `package alternate
import (
  . "os"
	filesystem "os"
	filefs "io/fs"
	legacy "io/ioutil"
	system "syscall"
	unixfs "golang.org/x/sys/unix"
)
func read(sourceFS filefs.FS) {
	_, _ = Open("zero")
  _, _ = ReadFile("one")
  _, _ = filesystem.ReadFile("two")
	_, _ = filesystem.Open("three")
	_, _ = filesystem.OpenFile("four", 0, 0)
	_, _ = filesystem.OpenRoot("five")
	_ = filesystem.DirFS("six")
	_, _ = filefs.ReadFile(sourceFS, "seven")
	_, _ = legacy.ReadFile("eight")
	_, _ = system.Open("nine", 0, 0)
	_, _ = system.Openat(0, "ten", 0, 0)
	_, _ = unixfs.Open("eleven", 0, 0)
	_, _ = unixfs.Openat(0, "twelve", 0, 0)
}`
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "alternate.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"os.*": true, "os.Open": true, "os.ReadFile": true, "os.OpenFile": true,
		"os.OpenRoot": true, "os.DirFS": true, "io/fs.ReadFile": true,
		"io/ioutil.ReadFile": true, "syscall.Open": true, "syscall.Openat": true,
		"golang.org/x/sys/unix.Open": true, "golang.org/x/sys/unix.Openat": true,
	}
	got := make(map[string]bool)
	for _, access := range directFileAccesses(fset, parsed) {
		got[access.symbol] = true
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("directFileAccesses() mismatch (-want +got):\n%s", diff)
	}
}

func TestRootedReaderBoundaryDetectsPathMutationAPIs(t *testing.T) {
	t.Parallel()

	const source = `package alternate
import (
  filesystem "os"
  system "syscall"
  unixfs "golang.org/x/sys/unix"
)
func mutate() {
  _ = filesystem.Remove("one")
  _ = system.Rename("two", "three")
  _ = unixfs.Unlinkat(0, "four", 0)
}`
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "alternate.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"os.Remove":                      true,
		"syscall.Rename":                 true,
		"golang.org/x/sys/unix.Unlinkat": true,
	}
	got := make(map[string]bool)
	for _, access := range directFileAccesses(fset, parsed) {
		got[access.symbol] = true
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("directFileAccesses() path-mutation mismatch (-want +got):\n%s", diff)
	}
}

func TestRootedReaderBoundaryDetectsAlternatePathPackages(t *testing.T) {
	t.Parallel()

	const source = `package alternate
import (
  tlswire "crypto/tls"
  contract "github.com/koopa0/yomihon/internal/schema"
  recording "github.com/koopa0/yomihon/internal/search/evalset/recording"
  sqlite "modernc.org/sqlite"
  pathfs "path/filepath"
  web "net/http"
)
var walk = pathfs.WalkDir
var directory = web.Dir("outside")
var tlsServer = web.ListenAndServeTLS("127.0.0.1:0", "cert", "key", nil)
var loadKeyPair = tlswire.LoadX509KeyPair
var loadContract = contract.LoadFile
var loadRecording = recording.Load
var openSQLite = sqlite.Driver.Open
type opener interface { Open(string) (any, error) }
func read(files opener) { _, _ = files.Open("outside") }
type prefixReader interface { ReadPrefix(string, int64) ([]byte, error) }
func readPrefix(files prefixReader) { _, _ = files.ReadPrefix("outside", 1) }
type tlsServer interface { ServeTLS(any, string, string) error }
func serveTLS(server tlsServer) { _ = server.ServeTLS(nil, "cert", "key") }
`
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "alternate.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"crypto/tls.LoadX509KeyPair":                                       true,
		"github.com/koopa0/yomihon/internal/schema.LoadFile":               true,
		"github.com/koopa0/yomihon/internal/search/evalset/recording.Load": true,
		"net/http.Dir":               true,
		"net/http.ListenAndServeTLS": true,
		"path/filepath.WalkDir":      true,
	}
	got := make(map[string]bool)
	for _, access := range directFileAccesses(fset, parsed) {
		got[access.symbol] = true
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("directFileAccesses() alternate-package mismatch (-want +got):\n%s", diff)
	}
}

func TestRootedReaderBoundaryDetectsFunctionValueBypass(t *testing.T) {
	t.Parallel()

	const source = `package alternate
import filesystem "os"
var open = filesystem.OpenFile
`
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "alternate.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	accesses := directFileAccesses(fset, parsed)
	if len(accesses) != 1 || accesses[0].symbol != "os.OpenFile" {
		t.Fatalf("directFileAccesses() = %+v, want one os.OpenFile function-value reference", accesses)
	}
}

func TestRootedReaderBoundaryRejectsTrackedDotImportWithoutCall(t *testing.T) {
	t.Parallel()

	const source = `package alternate
import . "os"
var open = OpenFile
`
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "alternate.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	accesses := directFileAccesses(fset, parsed)
	if len(accesses) == 0 {
		t.Fatal("directFileAccesses() accepted a tracked dot import used as a function value")
	}
}

func TestEmbeddingTransportBoundaryDetectsAlternateOwners(t *testing.T) {
	t.Parallel()

	const source = `package alternate
import (
  tlswire "crypto/tls"
  wire "net"
  web "net/http"
)
func send() {
  _, _ = web.Post("https://example.invalid", "text/plain", nil)
	_, _ = wire.Dial("tcp", "example.invalid:443")
	_, _ = wire.DialTCP("tcp", nil, nil)
	_, _ = wire.Listen("tcp", "127.0.0.1:0")
	_, _ = wire.LookupHost("query.example.invalid")
  _, _ = tlswire.Dial("tcp", "example.invalid:443", nil)
}
type client interface { Do(*web.Request) (*web.Response, error) }
func sendWith(client client, request *web.Request) { _, _ = client.Do(request) }
`
	fset, parsed, info := typeCheckSynthetic(t, source)
	got := typedOutboundReferences(fset, info, parsed)
	if len(got) != 7 {
		t.Fatalf("typedOutboundReferences() = %v, want all seven enumerated alternate transports", got)
	}
}

func TestTypedBoundaryDistinguishesCapabilityReceivers(t *testing.T) {
	t.Parallel()

	const source = `package alternate
import (
  "net/http"
  "os"
  "sync"
)
func use(root *os.Root, file *os.File, client *http.Client, once *sync.Once, header http.Header, request *http.Request) {
  _, _ = root.Stat(".")
  _, _ = root.Lstat("cache.db")
  _ = root.Mkdir("generation", 0700)
  _ = root.Remove("cache.db")
  _, _ = file.Stat()
  _, _ = client.Do(request)
  once.Do(func() {})
  _ = header.Get("Retry-After")
}`
	fset, parsed, info := typeCheckSynthetic(t, source)
	fileAccesses := make(map[string]bool)
	for _, access := range typedDirectFileAccesses(fset, info, parsed) {
		fileAccesses[access.symbol] = true
	}
	wantFileAccesses := map[string]bool{
		"os.Root.Stat":   true,
		"os.Root.Lstat":  true,
		"os.Root.Mkdir":  true,
		"os.Root.Remove": true,
		"os.File.Stat":   true,
	}
	if diff := cmp.Diff(wantFileAccesses, fileAccesses); diff != "" {
		t.Fatalf("typed direct-file receiver classification mismatch (-want +got):\n%s", diff)
	}

	network := make(map[string]bool)
	for _, reference := range typedOutboundReferences(fset, info, parsed) {
		network[reference.symbol] = true
	}
	if diff := cmp.Diff(map[string]bool{
		"net/http.Client":    true,
		"net/http.Client.Do": true,
	}, network); diff != "" {
		t.Fatalf("typed network receiver classification mismatch (-want +got):\n%s", diff)
	}
}

func TestEmbeddingTransportBoundaryRejectsDotImportBypass(t *testing.T) {
	t.Parallel()

	const source = `package alternate
import . "net/http"
func send() { _, _ = Post("https://example.invalid", "text/plain", nil) }
`
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "alternate.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := outboundReaches(fset, parsed); len(got) != 1 {
		t.Fatalf("outboundReaches() = %v, want dot-import violation", got)
	}
}

func TestGeneratedCatalogStaysBehindSemanticStore(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	var violations []string
	for _, source := range productionGoSources(t, root) {
		rel, err := filepath.Rel(root, source)
		if err != nil {
			t.Fatal(err)
		}
		rel = filepath.ToSlash(rel)
		parsed, _ := parseGoFile(t, source)
		qualifier, imported := importQualifier(parsed, generationCatalogPackagePath)
		if !imported {
			continue
		}
		if filepath.ToSlash(filepath.Dir(rel)) != generationCatalogOwnerDir {
			violations = append(violations, rel+": imports the semantic owner's generated storage package")
			continue
		}
		if qualifier == "." {
			violations = append(violations, rel+": dot-imports the generated storage package")
		}
	}
	for _, name := range exportedGeneratedReferences(t, root) {
		violations = append(violations, name+": exposes a generated storage type")
	}
	if len(violations) != 0 {
		t.Fatalf("the generated storage package escaped its semantic store owner:\n%s", strings.Join(violations, "\n"))
	}
}

func TestCatalogBoundaryDetectsAliasedExportedReferences(t *testing.T) {
	t.Parallel()

	generated := types.NewPackage(generationCatalogPackagePath, "catalog")
	queriesName := types.NewTypeName(token.NoPos, generated, "Queries", nil)
	queries := types.NewNamed(queriesName, types.NewStruct(nil, nil), nil)

	owner := types.NewPackage("github.com/koopa0/yomihon/internal/search/semantic", "semantic")
	aliasName := types.NewTypeName(token.NoPos, owner, "queries", nil)
	alias := types.NewAlias(aliasName, queries)
	leak := types.NewPointer(alias)

	if !referencesGeneratedPackage(leak, generationCatalogPackagePath, make(map[types.Type]bool)) {
		t.Fatal("an unexported alias hid a generated type from the public-API detector")
	}
	inferredExport := types.NewVar(token.NoPos, owner, "Queries", leak)
	if !inferredExport.Exported() || !referencesGeneratedPackage(inferredExport.Type(), generationCatalogPackagePath, make(map[types.Type]bool)) {
		t.Fatal("an inferred exported variable hid a generated type from the public-API detector")
	}
	safeField := types.NewField(token.NoPos, owner, "queries", leak, false)
	safeStruct := types.NewStruct([]*types.Var{safeField}, []string{""})
	if referencesGeneratedPackage(safeStruct, generationCatalogPackagePath, make(map[types.Type]bool)) {
		t.Fatal("an unexported implementation field was mistaken for public API")
	}
	safeName := types.NewTypeName(token.NoPos, owner, "Safe", nil)
	types.NewNamed(safeName, safeStruct, nil)
	owner.Scope().Insert(aliasName)
	owner.Scope().Insert(inferredExport)
	owner.Scope().Insert(safeName)
	if got := exportedGeneratedObjects(owner.Scope(), generationCatalogPackagePath); len(got) != 1 || got[0] != inferredExport.String() {
		t.Fatalf("exportedGeneratedObjects() = %v, want only %q", got, inferredExport)
	}
}

func TestOrdinarySearchAndServeStayOutsideSemanticPackages(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	targets := []struct {
		path       string
		recursive  bool
		allowAgent bool
	}{
		{path: "cmd/yomihon/main.go", allowAgent: true},
		{path: "internal/search"},
		{path: "internal/snapshot", recursive: true},
		{path: "internal/ui", recursive: true},
	}
	for _, target := range targets {
		path := filepath.Join(root, filepath.FromSlash(target.path))
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		var files []string
		switch {
		case info.IsDir() && target.recursive:
			files = productionGoSources(t, path)
		case info.IsDir():
			files = packageGoSources(t, path)
		default:
			files = []string{path}
		}
		for _, source := range files {
			parsed, _ := parseGoFile(t, source)
			for _, spec := range parsed.Imports {
				importPath, unquoteErr := strconv.Unquote(spec.Path.Value)
				if unquoteErr != nil {
					t.Fatal(unquoteErr)
				}
				importsSemantic := strings.HasSuffix(importPath, "/internal/search/semantic")
				importsAgent := strings.HasSuffix(importPath, "/internal/search/agent")
				if importsSemantic || importsAgent && !target.allowAgent {
					t.Errorf("%s imports %s; ordinary search/serve must remain lexical-only", source, importPath)
				}
			}
		}
	}
}

func TestProductionSearchRuntimeDoesNotImportEvaluationPackages(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	command := exec.CommandContext(t.Context(), "go", "list", "-deps", "-f", "{{.ImportPath}}", "./cmd/yomihon")
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("list production command dependencies: %v\n%s", err, output)
	}
	const evaluationPrefix = "github.com/koopa0/yomihon/internal/search/evalset"
	for dependency := range strings.Lines(string(output)) {
		dependency = strings.TrimSpace(dependency)
		if dependency == evaluationPrefix || strings.HasPrefix(dependency, evaluationPrefix+"/") {
			t.Fatalf("production command imports evaluation-only package %q", dependency)
		}
	}
}

func packageGoSources(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
			files = append(files, filepath.Join(root, entry.Name()))
		}
	}
	return files
}

func outboundReaches(fset *token.FileSet, file *ast.File) []string {
	references := outboundReferences(fset, file)
	reaches := make([]string, 0, len(references))
	for _, reference := range references {
		if strings.HasPrefix(reference.symbol, "method.") {
			continue
		}
		reaches = append(reaches, fmt.Sprintf(
			"%d:%d %s",
			reference.line, reference.column, reference.symbol,
		))
	}
	return reaches
}

func loadBoundaryPackages(t *testing.T, root string, patterns ...string) []*packages.Package {
	t.Helper()
	loaded, err := packages.Load(&packages.Config{
		Context: t.Context(),
		Dir:     root,
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedTypesSizes,
	}, patterns...)
	if err != nil {
		t.Fatalf("load boundary packages: %v", err)
	}
	if packages.PrintErrors(loaded) != 0 {
		t.Fatal("load boundary packages: package errors")
	}
	for _, pkg := range loaded {
		if len(pkg.CompiledGoFiles) != len(pkg.Syntax) {
			t.Fatalf(
				"load boundary package %s: %d compiled files but %d syntax trees",
				pkg.PkgPath, len(pkg.CompiledGoFiles), len(pkg.Syntax),
			)
		}
	}
	return loaded
}

func typeCheckSynthetic(t *testing.T, source string) (*token.FileSet, *ast.File, *types.Info) {
	t.Helper()
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "alternate.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Defs:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Uses:       make(map[*ast.Ident]types.Object),
	}
	config := types.Config{Importer: importer.Default()}
	if _, err := config.Check("alternate", fset, []*ast.File{parsed}, info); err != nil {
		t.Fatalf("type-check synthetic boundary fixture: %v", err)
	}
	return fset, parsed, info
}

var outboundReceiverTypes = map[string]struct{}{
	"crypto/tls.Conn":       {},
	"crypto/tls.Dialer":     {},
	"net.Conn":              {},
	"net.Dialer":            {},
	"net.ListenConfig":      {},
	"net.Listener":          {},
	"net.PacketConn":        {},
	"net.Resolver":          {},
	"net/http.Client":       {},
	"net/http.RoundTripper": {},
	"net/http.Server":       {},
	"net/http.Transport":    {},
}

var directFileReceiverExclusions = map[string]map[string]struct{}{
	"github.com/koopa0/yomihon/internal/vault.Reader": nonAccessVaultReaderMethods,
	"io/fs.FS":            {},
	"io/fs.GlobFS":        {},
	"io/fs.ReadDirFS":     {},
	"io/fs.ReadFileFS":    {},
	"io/fs.StatFS":        {},
	"io/fs.SubFS":         {},
	"net/http.FileSystem": {},
	"os.File":             {"Close": {}, "Name": {}},
	"os.Root":             {"Close": {}, "Name": {}},
}

func typedOutboundReferences(
	fset *token.FileSet,
	info *types.Info,
	file *ast.File,
) []outboundReference {
	var references []outboundReference
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || spec.Name == nil || spec.Name.Name != "." {
			continue
		}
		if _, tracked := outboundSymbols[path]; !tracked {
			continue
		}
		position := fset.Position(spec.Pos())
		references = append(references, outboundReference{
			function: "<package>",
			symbol:   "dot-import " + path,
			line:     position.Line,
			column:   position.Column,
		})
	}
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			references = append(references, typedOutboundReferencesInNode(
				fset, info, declaration, declaration.Name.Name,
			)...)
		case *ast.GenDecl:
			references = append(references, typedOutboundReferencesInNode(
				fset, info, declaration, "<package>",
			)...)
		}
	}
	return references
}

func typedOutboundReferencesInNode(
	fset *token.FileSet,
	info *types.Info,
	node ast.Node,
	function string,
) []outboundReference {
	var references []outboundReference
	ast.Inspect(node, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		symbol := typedOutboundSymbol(info, selector)
		if symbol == "" {
			return true
		}
		position := fset.Position(selector.Pos())
		references = append(references, outboundReference{
			function: function,
			symbol:   symbol,
			line:     position.Line,
			column:   position.Column,
		})
		return true
	})
	return references
}

func typedOutboundSymbol(info *types.Info, selector *ast.SelectorExpr) string {
	if selection := info.Selections[selector]; selection != nil {
		method, ok := selection.Obj().(*types.Func)
		if !ok {
			return ""
		}
		receiver := namedTypeIdentity(selection.Recv())
		if _, tracked := outboundReceiverTypes[receiver]; tracked {
			return receiver + "." + method.Name()
		}
		if _, tracked := outboundMethodNames[method.Name()]; tracked &&
			typeReferencesAnyPackage(method.Type(), "crypto/tls", "net", "net/http") {
			return "interface." + method.Name()
		}
		return ""
	}
	object := info.Uses[selector.Sel]
	if object == nil || object.Pkg() == nil {
		return ""
	}
	path := object.Pkg().Path()
	if _, tracked := outboundSymbols[path][object.Name()]; !tracked {
		return ""
	}
	return path + "." + object.Name()
}

func typedDirectFileAccesses(
	fset *token.FileSet,
	info *types.Info,
	file *ast.File,
) []directFileAccess {
	var accesses []directFileAccess
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || spec.Name == nil || spec.Name.Name != "." || !trackedDirectFilePackage(path) {
			continue
		}
		position := fset.Position(spec.Pos())
		accesses = append(accesses, directFileAccess{
			function: "<package>",
			symbol:   path + ".*",
			line:     position.Line,
			column:   position.Column,
		})
	}
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			accesses = append(accesses, typedDirectFileAccessesInNode(
				fset, info, declaration, declaration.Name.Name,
			)...)
		case *ast.GenDecl:
			accesses = append(accesses, typedDirectFileAccessesInNode(
				fset, info, declaration, "<package>",
			)...)
		}
	}
	return accesses
}

func typedDirectFileAccessesInNode(
	fset *token.FileSet,
	info *types.Info,
	node ast.Node,
	function string,
) []directFileAccess {
	var accesses []directFileAccess
	ast.Inspect(node, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		symbol := typedDirectFileSymbol(info, selector)
		if symbol == "" {
			return true
		}
		position := fset.Position(selector.Pos())
		accesses = append(accesses, directFileAccess{
			function: function,
			symbol:   symbol,
			line:     position.Line,
			column:   position.Column,
		})
		return true
	})
	return accesses
}

func typedDirectFileSymbol(info *types.Info, selector *ast.SelectorExpr) string {
	if selection := info.Selections[selector]; selection != nil {
		return typedDirectFileMethodSymbol(selection)
	}
	object := info.Uses[selector.Sel]
	if object == nil || object.Pkg() == nil || !trackedDirectFileSymbol(object.Pkg().Path(), object.Name()) {
		return ""
	}
	return object.Pkg().Path() + "." + object.Name()
}

func typedDirectFileMethodSymbol(selection *types.Selection) string {
	method, ok := selection.Obj().(*types.Func)
	if !ok {
		return ""
	}
	receiver := namedTypeIdentity(selection.Recv())
	if exclusions, tracked := directFileReceiverExclusions[receiver]; tracked {
		if _, excluded := exclusions[method.Name()]; excluded || !method.Exported() {
			return ""
		}
		return receiver + "." + method.Name()
	}
	if !typeReferencesAnyPackage(
		method.Type(),
		"github.com/koopa0/yomihon/internal/vault", "io/fs", "os",
	) {
		return ""
	}
	switch method.Name() {
	case "Entries", "Glob", "Lstat", "Lookup", "Mkdir", "MkdirAll", "Open", "OpenFile",
		"OpenRoot", "ReadDir", "ReadFile", "ReadPrefix", "Refresh", "Remove", "RemoveAll",
		"Rename", "ScanAvailable", "ScanComplete", "Stat", "Sub":
		return "interface." + method.Name()
	default:
		return ""
	}
}

func namedTypeIdentity(value types.Type) string {
	value = types.Unalias(value)
	if pointer, ok := value.(*types.Pointer); ok {
		value = types.Unalias(pointer.Elem())
	}
	named, ok := value.(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return ""
	}
	return named.Obj().Pkg().Path() + "." + named.Obj().Name()
}

func typeReferencesAnyPackage(value types.Type, paths ...string) bool {
	for _, path := range paths {
		if referencesGeneratedPackage(value, path, make(map[types.Type]bool)) {
			return true
		}
	}
	return false
}

func outboundReferences(fset *token.FileSet, file *ast.File) []outboundReference {
	qualifiers := make(map[string]string, len(file.Imports))
	var references []outboundReference
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := filepath.Base(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == "." {
			if _, outbound := outboundSymbols[path]; outbound {
				position := fset.Position(spec.Pos())
				references = append(references, outboundReference{
					function: "<package>",
					symbol:   "dot-import " + path,
					line:     position.Line,
					column:   position.Column,
				})
			}
			continue
		}
		qualifiers[name] = path
	}
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			references = append(references, outboundReferencesInNode(
				fset, declaration, declaration.Name.Name, qualifiers,
			)...)
		case *ast.GenDecl:
			references = append(references, outboundReferencesInNode(
				fset, declaration, "<package>", qualifiers,
			)...)
		}
	}
	return references
}

func outboundReferencesInNode(
	fset *token.FileSet,
	node ast.Node,
	function string,
	qualifiers map[string]string,
) []outboundReference {
	var references []outboundReference
	ast.Inspect(node, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		qualifier, qualified := selector.X.(*ast.Ident)
		importPath := ""
		if qualified {
			importPath = qualifiers[qualifier.Name]
		}
		if _, tracked := outboundSymbols[importPath][selector.Sel.Name]; !tracked {
			return true
		}
		position := fset.Position(selector.Pos())
		references = append(references, outboundReference{
			function: function,
			symbol:   importPath + "." + selector.Sel.Name,
			line:     position.Line,
			column:   position.Column,
		})
		return true
	})
	return references
}

func directFileAccesses(fset *token.FileSet, file *ast.File) []directFileAccess {
	qualifiers := make(map[string]string)
	var accesses []directFileAccess
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		if !trackedDirectFilePackage(importPath) {
			continue
		}
		name := filepath.Base(importPath)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == "." {
			position := fset.Position(spec.Pos())
			accesses = append(accesses, directFileAccess{
				function: "<package>",
				symbol:   importPath + ".*",
				line:     position.Line,
				column:   position.Column,
			})
			continue
		}
		qualifiers[name] = importPath
	}
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			accesses = append(accesses, fileAccessesInNode(fset, declaration.Body, declaration.Name.Name, qualifiers)...)
		case *ast.GenDecl:
			accesses = append(accesses, fileAccessesInNode(fset, declaration, "<package>", qualifiers)...)
		}
	}
	return accesses
}

func fileAccessesInNode(
	fset *token.FileSet,
	node ast.Node,
	function string,
	qualifiers map[string]string,
) []directFileAccess {
	var accesses []directFileAccess
	ast.Inspect(node, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		qualifier, qualified := selector.X.(*ast.Ident)
		importPath := ""
		if qualified {
			importPath = qualifiers[qualifier.Name]
		}
		if !trackedDirectFileSymbol(importPath, selector.Sel.Name) {
			return true
		}
		position := fset.Position(selector.Pos())
		accesses = append(accesses, directFileAccess{
			function: function,
			symbol:   importPath + "." + selector.Sel.Name,
			line:     position.Line,
			column:   position.Column,
		})
		return true
	})
	return accesses
}

func trackedDirectFilePackage(importPath string) bool {
	switch importPath {
	case "os", "io/ioutil", "syscall", "golang.org/x/sys/unix":
		return true
	default:
		_, tracked := directFileSymbols[importPath]
		return tracked
	}
}

func trackedDirectFileSymbol(importPath, name string) bool {
	switch importPath {
	case "os":
		if _, safe := nonPathOSFunctions[name]; safe {
			return false
		}
		_, function := osFunctionSymbols[name]
		return function
	case "golang.org/x/sys/unix":
		_, safe := nonPathUnixSymbols[name]
		return !safe
	case "io/ioutil", "syscall":
		return true
	default:
		_, tracked := directFileSymbols[importPath][name]
		return tracked
	}
}

func importQualifier(file *ast.File, target string) (string, bool) {
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || path != target {
			continue
		}
		if spec.Name != nil {
			return spec.Name.Name, true
		}
		return filepath.Base(path), true
	}
	return "", false
}

func exportedGeneratedReferences(t *testing.T, root string) []string {
	t.Helper()
	fset := token.NewFileSet()
	sources := activeSemanticSources(t, root)
	files := make([]*ast.File, 0, len(sources))
	for _, source := range sources {
		file, err := parser.ParseFile(fset, source, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}

	exports := compilerExports(t, root)
	lookup := func(path string) (io.ReadCloser, error) {
		export, ok := exports[path]
		if !ok || export == "" {
			return nil, fmt.Errorf("no compiler export data for %s", path)
		}
		return os.Open(export) // #nosec G304 -- export is a compiler-produced path returned by go list
	}
	config := types.Config{Importer: importer.ForCompiler(fset, runtime.Compiler, lookup)}
	checked, err := config.Check("github.com/koopa0/yomihon/internal/search/semantic", fset, files, nil)
	if err != nil {
		t.Fatalf("type-check semantic owner: %v", err)
	}

	return exportedGeneratedObjects(checked.Scope(), generationCatalogPackagePath)
}

func exportedGeneratedObjects(scope *types.Scope, target string) []string {
	var references []string
	for _, name := range scope.Names() {
		object := scope.Lookup(name)
		if !object.Exported() {
			continue
		}
		if referencesGeneratedPackage(object.Type(), target, make(map[types.Type]bool)) {
			references = append(references, object.String())
		}
	}
	return references
}

func activeSemanticSources(t *testing.T, root string) []string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "go", "list", "-json", "./"+generationCatalogOwnerDir)
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			t.Fatalf("list active semantic files: %v\n%s", err, exit.Stderr)
		}
		t.Fatalf("list active semantic files: %v", err)
	}
	var listed struct {
		Dir      string
		GoFiles  []string
		CgoFiles []string
	}
	if err := json.Unmarshal(output, &listed); err != nil {
		t.Fatalf("decode active semantic files: %v", err)
	}
	sources := make([]string, 0, len(listed.GoFiles)+len(listed.CgoFiles))
	for _, name := range append(listed.GoFiles, listed.CgoFiles...) {
		sources = append(sources, filepath.Join(listed.Dir, name))
	}
	return sources
}

func compilerExports(t *testing.T, root string) map[string]string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "go", "list", "-deps", "-export", "-json", "./"+generationCatalogOwnerDir)
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			t.Fatalf("list compiler exports: %v\n%s", err, exit.Stderr)
		}
		t.Fatalf("list compiler exports: %v", err)
	}

	type listedPackage struct {
		ImportPath string
		Export     string
	}
	exports := make(map[string]string)
	decoder := json.NewDecoder(bytes.NewReader(output))
	for {
		var listed listedPackage
		if err := decoder.Decode(&listed); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("decode compiler exports: %v", err)
		}
		if listed.Export != "" {
			exports[listed.ImportPath] = listed.Export
		}
	}
	return exports
}

func referencesGeneratedPackage(current types.Type, target string, seen map[types.Type]bool) bool {
	if current == nil || seen[current] {
		return false
	}
	seen[current] = true

	switch current := current.(type) {
	case *types.Alias:
		if objectFromPackage(current.Obj(), target) {
			return true
		}
		return referencesGeneratedPackage(types.Unalias(current), target, seen)
	case *types.Named:
		if objectFromPackage(current.Obj(), target) {
			return true
		}
		if typeListReferencesGenerated(current.TypeArgs(), target, seen) ||
			typeParamListReferencesGenerated(current.TypeParams(), target, seen) {
			return true
		}
		for method := range current.Methods() {
			if method.Exported() && referencesGeneratedPackage(method.Type(), target, seen) {
				return true
			}
		}
		return referencesGeneratedPackage(current.Underlying(), target, seen)
	case *types.Pointer:
		return referencesGeneratedPackage(current.Elem(), target, seen)
	case *types.Array:
		return referencesGeneratedPackage(current.Elem(), target, seen)
	case *types.Slice:
		return referencesGeneratedPackage(current.Elem(), target, seen)
	case *types.Map:
		return referencesGeneratedPackage(current.Key(), target, seen) ||
			referencesGeneratedPackage(current.Elem(), target, seen)
	case *types.Chan:
		return referencesGeneratedPackage(current.Elem(), target, seen)
	case *types.Signature:
		return tupleReferencesGenerated(current.Params(), target, seen) ||
			tupleReferencesGenerated(current.Results(), target, seen) ||
			typeParamListReferencesGenerated(current.TypeParams(), target, seen)
	case *types.Struct:
		for field := range current.Fields() {
			if (field.Exported() || field.Embedded()) && referencesGeneratedPackage(field.Type(), target, seen) {
				return true
			}
		}
	case *types.Interface:
		current.Complete()
		for method := range current.ExplicitMethods() {
			if referencesGeneratedPackage(method.Type(), target, seen) {
				return true
			}
		}
		for embedded := range current.EmbeddedTypes() {
			if referencesGeneratedPackage(embedded, target, seen) {
				return true
			}
		}
	case *types.Tuple:
		return tupleReferencesGenerated(current, target, seen)
	case *types.TypeParam:
		return referencesGeneratedPackage(current.Constraint(), target, seen)
	case *types.Union:
		for term := range current.Terms() {
			if referencesGeneratedPackage(term.Type(), target, seen) {
				return true
			}
		}
	}
	return false
}

func objectFromPackage(object types.Object, target string) bool {
	return object != nil && object.Pkg() != nil && object.Pkg().Path() == target
}

func tupleReferencesGenerated(tuple *types.Tuple, target string, seen map[types.Type]bool) bool {
	if tuple == nil {
		return false
	}
	for variable := range tuple.Variables() {
		if referencesGeneratedPackage(variable.Type(), target, seen) {
			return true
		}
	}
	return false
}

func typeListReferencesGenerated(list *types.TypeList, target string, seen map[types.Type]bool) bool {
	if list == nil {
		return false
	}
	for current := range list.Types() {
		if referencesGeneratedPackage(current, target, seen) {
			return true
		}
	}
	return false
}

func typeParamListReferencesGenerated(list *types.TypeParamList, target string, seen map[types.Type]bool) bool {
	if list == nil {
		return false
	}
	for parameter := range list.TypeParams() {
		if referencesGeneratedPackage(parameter, target, seen) {
			return true
		}
	}
	return false
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(root, "go.mod")); statErr == nil {
			return root
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("repository root: go.mod not found")
		}
		root = parent
	}
}

func productionGoSources(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func parseGoFile(t *testing.T, path string) (*ast.File, *token.FileSet) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	return file, fset
}
