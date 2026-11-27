package mark_test

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/mark/machinemarks"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// anIdentity is a content identity in the one spelling a reading page stamps.
const anIdentity = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// The two bodies a page sends: the JSON of the mark and bookmark routes, and
// the form the question and bookmark buttons post.
const (
	jsonBody = "application/json"
	formBody = "application/x-www-form-urlencoded"
)

func markID(n int) string { return fmt.Sprintf("%032x", n) }

func aPlace() *mark.Bookmark {
	return &mark.Bookmark{
		RelPath:  "Writing/lessons/japanese/L01.md",
		Anchor:   "the-topic-particle",
		Offset:   420,
		Identity: anIdentity,
		At:       time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC),
	}
}

// aQuestion is a question about a whole section.
func aQuestion() *machinemarks.Mark {
	return &machinemarks.Mark{
		Kind:   machinemarks.Question,
		Path:   "Notes/source.md",
		Anchor: "section-one",
		At:     time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC),
	}
}

// aHighlight is a highlight of a span of text, with what stood around it.
func aHighlight() *machinemarks.Mark {
	return &machinemarks.Mark{
		Kind:     machinemarks.Highlight,
		Path:     "Notes/source.md",
		Anchor:   "section-one",
		Quote:    "the topic particle",
		Before:   "In Japanese, ",
		After:    " marks what the sentence is about",
		Identity: anIdentity,
		At:       time.Date(2026, 9, 22, 11, 0, 0, 0, time.UTC),
	}
}

func newStore(t *testing.T) *mark.Store {
	t.Helper()
	return newStoreFor(t, t.TempDir(), "/vaults/notes")
}

func newStoreFor(t *testing.T, configDir, vaultRoot string) *mark.Store {
	t.Helper()
	store, err := mark.Open(confdir.At(configDir), vaultRoot)
	if err != nil {
		t.Fatalf("mark.Open(%q, %q): %v", configDir, vaultRoot, err)
	}
	return store
}

// moveBookmark sets the bookmark of a course and asks nothing of what it
// replaced, for the tests that are about something else.
func moveBookmark(store *mark.Store, course string, b *mark.Bookmark) error {
	_, _, err := store.MoveBookmark(course, b, nil)
	return err
}

// bookmarkFor is the bookmark kept for course and whether there is one, failing
// the test when the file cannot be read.
func bookmarkFor(t *testing.T, store *mark.Store, course string) (mark.Bookmark, bool) {
	t.Helper()
	held, found, err := store.Bookmark(course)
	if err != nil {
		t.Fatalf("Bookmark(%q): %v", course, err)
	}
	return held, found
}

func marksPath(store *mark.Store) string { return filepath.Join(store.Dir(), "marks.json") }

func progressPath(store *mark.Store) string { return filepath.Join(store.Dir(), "progress.json") }

// writeFile puts bytes where a store looks, as a reader or an earlier version
// would have left them.
func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil { // #nosec G703 -- a test's own path under t.TempDir
		t.Fatal(err)
	}
}

// contents is what a file holds, and nothing where there is none, failing the
// test when it cannot be read.
func contents(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) // #nosec G304 -- a file under a directory this test made
	if errors.Is(err, fs.ErrNotExist) {
		return ""
	}
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// tree is every file and folder under dir, each file with what it holds and
// each link with where it points.
func tree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p) //nolint:errcheck // p is under dir
		if d.Type()&fs.ModeSymlink != 0 {
			target, linkErr := os.Readlink(p)
			out[rel] = "-> " + target
			return linkErr
		}
		if d.IsDir() {
			out[rel+"/"] = ""
			return nil
		}
		b, readErr := os.ReadFile(p) // #nosec G304 G122 -- a test's own path
		out[rel] = string(b)
		return readErr
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// notes are the notes a route can place a mark in: each path with the words its
// page holds. Every note has one place, "section-one", and the note as a whole.
// No course walks any of them.
type notes map[string]string

func (n notes) HasPlace(rel, anchor string) bool {
	_, ok := n[rel]
	return ok && (anchor == "" || anchor == "section-one")
}

func (n notes) Reading(rel string, _ wording.Lang) (mark.Page, bool) {
	text, ok := n[rel]
	return mark.Page{Text: text, Identity: anIdentity}, ok
}

func (notes) Units(string) (mainLine, branches []string, ok bool) { return nil, nil, false }

func (notes) Rows(string) (planned int, rows []string, ok bool) { return 0, nil, false }

// vaultPlaces is a vault that holds the places it names, at any anchor, and
// whose notes have no words: the bookmark and question routes find none. A nil
// map holds every place.
type vaultPlaces map[[2]string]bool

func (p vaultPlaces) HasPlace(rel, anchor string) bool {
	return p == nil || p[[2]string{rel, anchor}]
}

func (vaultPlaces) Reading(string, wording.Lang) (mark.Page, bool) { return mark.Page{}, true }

func (vaultPlaces) Units(string) (mainLine, branches []string, ok bool) { return nil, nil, false }

func (vaultPlaces) Rows(string) (planned int, rows []string, ok bool) { return 0, nil, false }

// aServer is the routes over a real folder serving as the vault: where it is,
// the configuration directory its marks are kept beside, and the marks as the
// reading server holds them.
type aServer struct {
	root, config string
	store        *mark.Store
	vaultMarks   *mark.VaultMarks
	handler      http.Handler
}

// newServer is the routes over a vault whose places are those given, which a
// test may change while it runs. Nil places hold every one.
func newServer(t *testing.T, places mark.Places) aServer {
	t.Helper()
	if places == nil {
		places = vaultPlaces(nil)
	}
	_, root, err := vault.ResolveRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	config := t.TempDir()
	store := newStoreFor(t, config, root)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reader.Close() }) //nolint:errcheck // the test is over
	vaultMarks := mark.OpenVaultMarks(store, reader, slog.New(slog.DiscardHandler))
	t.Cleanup(func() { _ = vaultMarks.Close() }) //nolint:errcheck // the test is over
	mux := http.NewServeMux()
	mark.NewHandler(store, vaultMarks, places, slog.New(slog.DiscardHandler)).Register(mux)
	return aServer{root: root, config: config, store: store, vaultMarks: vaultMarks, handler: mux}
}

// send makes one request of handler, of contentType when it is not empty and
// in the language asked for when there is one.
func send(t *testing.T, handler http.Handler, method, target, contentType, body string, lang ...string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	if len(lang) > 0 {
		request.Header.Set("Accept-Language", lang[0])
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
