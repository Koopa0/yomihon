package mark_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"log/slog"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/mark"
)

// anIdentity is a content identity in the one spelling a reading page stamps.
const anIdentity = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func aPlace() *mark.Continuation {
	return &mark.Continuation{
		RelPath:  "Writing/lessons/japanese/L01.md",
		Anchor:   "the-topic-particle",
		Offset:   420,
		Identity: anIdentity,
		At:       time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC),
	}
}

func newFile(t *testing.T) *mark.File {
	t.Helper()
	return newFileFor(t, t.TempDir(), "/vaults/notes")
}

func newFileFor(t *testing.T, configDir, vaultRoot string) *mark.File {
	t.Helper()
	file, err := mark.New(configDir, vaultRoot)
	if err != nil {
		t.Fatalf("mark.New(%q, %q): %v", configDir, vaultRoot, err)
	}
	return file
}

// TestAFileWithNothingNamingItIsRefused holds the one thing the constructor
// checks. An empty configuration directory builds a relative path, which would
// put the reader's marks beside whatever folder the process started in and
// look like it had worked.
func TestAFileWithNothingNamingItIsRefused(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name      string
		configDir string
		vaultRoot string
	}{
		{"no configuration directory", "", "/vaults/notes"},
		{"no vault root", "/config", ""},
		{"neither", "", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			file, err := mark.New(tt.configDir, tt.vaultRoot)
			if err == nil {
				t.Fatalf("mark.New(%q, %q) built a file at %s", tt.configDir, tt.vaultRoot, file.Path())
			}
			if !errors.Is(err, mark.ErrNotNamed) {
				t.Errorf("mark.New(%q, %q) = %v, want it to be ErrNotNamed", tt.configDir, tt.vaultRoot, err)
			}
		})
	}
}

func TestAKeptPlaceComesBack(t *testing.T) {
	t.Parallel()

	file := newFile(t)
	if _, ok := file.Continuation(); ok {
		t.Fatal("a file nobody has written to reports a kept place")
	}
	want := aPlace()
	if err := file.SetContinuation(want); err != nil {
		t.Fatalf("SetContinuation: %v", err)
	}
	got, ok := file.Continuation()
	if !ok {
		t.Fatal("the place that was just kept is not reported")
	}
	if diff := cmp.Diff(*want, got); diff != "" {
		t.Errorf("Continuation() mismatch (-want +got):\n%s", diff)
	}
}

// TestKeepingAnotherPlaceReplacesTheFirst holds the ruled shape: one place in
// total. A file that accumulated them would be a reading history, which is a
// different thing and was not asked for.
func TestKeepingAnotherPlaceReplacesTheFirst(t *testing.T) {
	t.Parallel()

	file := newFile(t)
	if err := file.SetContinuation(aPlace()); err != nil {
		t.Fatalf("keep the first place: %v", err)
	}
	second := aPlace()
	second.RelPath = "Notes/alpha.md"
	second.Anchor = "elsewhere"
	second.Offset = 12
	if err := file.SetContinuation(second); err != nil {
		t.Fatalf("keep the second place: %v", err)
	}
	got, ok := file.Continuation()
	if !ok {
		t.Fatal("no place is reported after two were kept")
	}
	if diff := cmp.Diff(*second, got); diff != "" {
		t.Errorf("the second place did not replace the first (-want +got):\n%s", diff)
	}

	// And on disk: one continuation, not a list that only reads as one.
	data, err := os.ReadFile(file.Path())
	if err != nil {
		t.Fatalf("read the marks file: %v", err)
	}
	var document map[string]any
	if err = json.Unmarshal(data, &document); err != nil {
		t.Fatalf("decode the marks file: %v", err)
	}
	if _, isObject := document["continuation"].(map[string]any); !isObject {
		t.Errorf("the file holds continuation as %T, want one object", document["continuation"])
	}
	if got := strings.Count(string(data), `"path"`); got != 1 {
		t.Errorf("the file names %d paths, want exactly 1", got)
	}
}

// TestTheFileNamesItsVault holds the half of the directory decision that a
// person acts on. The directory is a digest and says nothing, so what tells
// them which vault they are looking at is inside the file.
func TestTheFileNamesItsVault(t *testing.T) {
	t.Parallel()

	const root = "/vaults/notes"
	file := newFileFor(t, t.TempDir(), root)
	if err := file.SetContinuation(aPlace()); err != nil {
		t.Fatalf("SetContinuation: %v", err)
	}
	data, err := os.ReadFile(file.Path())
	if err != nil {
		t.Fatalf("read the marks file: %v", err)
	}
	var document struct {
		Version int    `json:"version"`
		Vault   string `json:"vault"`
	}
	if err = json.Unmarshal(data, &document); err != nil {
		t.Fatalf("decode the marks file: %v", err)
	}
	if document.Vault != root {
		t.Errorf("the file names vault %q, want %q", document.Vault, root)
	}
	if document.Version != 1 {
		t.Errorf("the file is written at version %d, want 1", document.Version)
	}
}

func TestTwoVaultsGetTwoDirectories(t *testing.T) {
	t.Parallel()

	config := t.TempDir()
	first := newFileFor(t, config, "/vaults/notes")
	second := newFileFor(t, config, "/vaults/other")
	if first.Path() == second.Path() {
		t.Fatalf("two vaults share one marks file at %s", first.Path())
	}
	if err := first.SetContinuation(aPlace()); err != nil {
		t.Fatalf("keep a place in the first vault: %v", err)
	}
	if _, ok := second.Continuation(); ok {
		t.Error("a place kept for one vault is reported for another")
	}
}

// TestTheSameFolderSpelledTwoWaysIsOneVault holds the NFC normalization the
// directory name is derived through. A folder whose name carries a composed
// character can be handed to the command in either spelling — the filesystem
// answers to both — and two directories with two different marks in them would
// be one vault quietly split.
func TestTheSameFolderSpelledTwoWaysIsOneVault(t *testing.T) {
	t.Parallel()

	config := t.TempDir()
	// One character written the two ways Unicode allows: as itself, and as the
	// letter it is built from followed by the mark that modifies it. They are
	// spelled in code points rather than typed, because the two are identical
	// in every editor and a test whose two inputs were secretly one string
	// would compare nothing and pass.
	const (
		composed   = "/vaults/\u30d1"       // U+30D1 alone
		decomposed = "/vaults/\u30cf\u309a" // U+30CF followed by U+309A
	)
	if composed == decomposed {
		t.Fatal("the two spellings are the same bytes, so comparing what they derive proves nothing")
	}
	if mark.Directory(config, composed) != mark.Directory(config, decomposed) {
		t.Errorf("the same folder spelled two ways got two directories:\n%s\n%s",
			mark.Directory(config, composed), mark.Directory(config, decomposed))
	}
}

// TestTheDirectoryNameSpellsNoPath holds the other half of that decision: a
// path is not a safe directory name, so none of it appears in one.
func TestTheDirectoryNameSpellsNoPath(t *testing.T) {
	t.Parallel()

	config := t.TempDir()
	dir := mark.Directory(config, "/vaults/secret project/notes")
	name := filepath.Base(dir)
	for _, fragment := range []string{"secret", "project", "notes", "vaults", " ", "/"} {
		if strings.Contains(name, fragment) {
			t.Errorf("the directory name %q carries %q from the vault path", name, fragment)
		}
	}
	if parent := filepath.Base(filepath.Dir(dir)); parent != "yomihon" {
		t.Errorf("the directory sits under %q, want yomihon", parent)
	}
}

func TestARefusedPlaceIsNotStored(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		spoil func(*mark.Continuation)
	}{
		{"no note", func(c *mark.Continuation) { c.RelPath = "" }},
		{"an absolute path", func(c *mark.Continuation) { c.RelPath = "/etc/passwd" }},
		{"a path that climbs out", func(c *mark.Continuation) { c.RelPath = "../outside.md" }},
		{"a path that is not in NFC", func(c *mark.Continuation) { c.RelPath = "Notes/\u30cf\u309a.md" }},
		// fs.ValidPath lets control characters through, so each class the
		// predicate names is driven: C0, DELETE, C1 and the two Unicode
		// separators that end a line without being a newline.
		{"a path carrying a NUL", func(c *mark.Continuation) { c.RelPath = "Notes/a\x00b.md" }},
		{"a path carrying a line feed", func(c *mark.Continuation) { c.RelPath = "Notes/a\nb.md" }},
		{"a path carrying a NUL and a line feed", func(c *mark.Continuation) { c.RelPath = "Notes/a\x00b\nc.md" }},
		{"a path carrying a carriage return", func(c *mark.Continuation) { c.RelPath = "Notes/a\rb.md" }},
		{"a path carrying a tab", func(c *mark.Continuation) { c.RelPath = "Notes/a\tb.md" }},
		{"a path carrying DELETE", func(c *mark.Continuation) { c.RelPath = "Notes/a\x7fb.md" }},
		{"a path carrying a C1 control", func(c *mark.Continuation) { c.RelPath = "Notes/a\u0085b.md" }},
		{"a path carrying a line separator", func(c *mark.Continuation) { c.RelPath = "Notes/a\u2028b.md" }},
		{"a path carrying a paragraph separator", func(c *mark.Continuation) { c.RelPath = "Notes/a\u2029b.md" }},
		// fs.ValidPath already refused this on main; the row pins it.
		{"a path that is not UTF-8", func(c *mark.Continuation) { c.RelPath = "Notes/a\xffb.md" }},
		{"an anchor that could cut the address", func(c *mark.Continuation) { c.Anchor = "here#elsewhere" }},
		{"an anchor carrying a control character", func(c *mark.Continuation) { c.Anchor = "here\nthere" }},
		// The anchor had only the character check, which reads an invalid byte
		// as U+FFFD and let it through to be stored as something its sender
		// never wrote.
		{"an anchor that is not UTF-8", func(c *mark.Continuation) { c.Anchor = "here\xff" }},
		{"a negative offset", func(c *mark.Continuation) { c.Offset = -1 }},
		{"an offset past any document", func(c *mark.Continuation) { c.Offset = 1 << 30 }},
		{"an identity that is not one", func(c *mark.Continuation) { c.Identity = "not-a-digest" }},
		{"an identity in the other case", func(c *mark.Continuation) { c.Identity = strings.ToUpper(anIdentity) }},
		{"no identity at all", func(c *mark.Continuation) { c.Identity = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			file := newFile(t)
			refused := aPlace()
			tt.spoil(refused)
			if err := file.SetContinuation(refused); !errors.Is(err, mark.ErrInvalid) {
				t.Fatalf("SetContinuation(%s) = %v, want ErrInvalid", tt.name, err)
			}
			if _, ok := file.Continuation(); ok {
				t.Error("a refused place was stored anyway")
			}
			if _, statErr := os.Stat(file.Path()); !os.IsNotExist(statErr) {
				t.Errorf("a refused place created %s", file.Path())
			}
		})
	}
}

// TestARefusalLeavesTheKeptPlaceAlone is the half the test above cannot see.
// That one starts from an empty file, so "nothing was stored" is also what a
// refusal that wiped the file would look like. A refusal must not be a way to
// clear a place the reader meant to keep, and only a file that already holds
// one can show it.
func TestARefusalLeavesTheKeptPlaceAlone(t *testing.T) {
	t.Parallel()

	file := newFile(t)
	want := aPlace()
	if err := file.SetContinuation(want); err != nil {
		t.Fatalf("keep a place: %v", err)
	}
	refused := aPlace()
	refused.Identity = "not-a-digest"
	if err := file.SetContinuation(refused); err == nil {
		t.Fatal("a place with no identity was accepted")
	}
	got, ok := file.Continuation()
	if !ok {
		t.Fatal("the kept place is gone after a refusal")
	}
	if diff := cmp.Diff(*want, got); diff != "" {
		t.Errorf("a refusal changed the kept place (-want +got):\n%s", diff)
	}
}

// TestOrdinaryNoteNamesAreKept is the half the refusals in
// TestARefusedPlaceIsNotStored cannot see: a rule on control characters that
// grew too wide would show here rather than on someone's lesson.
func TestOrdinaryNoteNamesAreKept(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ name, path string }{
		{"a space", "Notes/A published note.md"},
		{"Chinese", "Notes/go/提早返回的管線.md"},
		{"an ideographic space", "Notes/日本\u3000語.md"},
		{"a dotted name", "Notes/v1.2.md"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			place := aPlace()
			place.RelPath = tt.path
			if err := newFile(t).SetContinuation(place); err != nil {
				t.Errorf("SetContinuation(%q) = %v, want it kept", tt.path, err)
			}
		})
	}
}

// TestAnUnreadableFileReportsNoPlace covers the shapes a file can be in that
// this version does not recognise. Each answers the same way a missing file
// does, and the next place the reader keeps replaces it.
func TestAnUnreadableFileReportsNoPlace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{"not JSON at all", "this is not a file yomihon wrote\n"},
		{"a version from another day", `{"version":99,"continuation":{"path":"a.md","offset":0,"identity":"` + anIdentity + `"}}`},
		{"no continuation in it", `{"version":1,"vault":"/vaults/notes"}`},
		{"a place the shape refuses", `{"version":1,"continuation":{"path":"../outside.md","offset":0,"identity":"` + anIdentity + `"}}`},
		{"a path carrying a control character", `{"version":1,"continuation":{"path":"Notes/a\u0000b\nc.md","offset":1,"identity":"` + anIdentity + `"}}`},
		{"an empty file", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			config := t.TempDir()
			file := newFileFor(t, config, "/vaults/notes")
			if err := os.MkdirAll(filepath.Dir(file.Path()), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file.Path(), []byte(tt.body), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, ok := file.Continuation(); ok {
				t.Errorf("%s was read as a kept place", tt.name)
			}
			// And the reader can still keep one.
			if err := file.SetContinuation(aPlace()); err != nil {
				t.Errorf("keeping a place over %s failed: %v", tt.name, err)
			}
			if _, ok := file.Continuation(); !ok {
				t.Errorf("keeping a place over %s did not take", tt.name)
			}
		})
	}
}

// TestNothingIsLeftBesideTheFile holds the replace discipline's tidy half: the
// temporary sibling is consumed by the rename, so a directory holding a mark
// holds exactly one file.
func TestNothingIsLeftBesideTheFile(t *testing.T) {
	t.Parallel()

	file := newFile(t)
	for range 3 {
		if err := file.SetContinuation(aPlace()); err != nil {
			t.Fatalf("SetContinuation: %v", err)
		}
	}
	entries, err := os.ReadDir(filepath.Dir(file.Path()))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if diff := cmp.Diff([]string{"reader.json"}, names); diff != "" {
		t.Errorf("the marks directory holds more than the file (-want +got):\n%s", diff)
	}
}

// TestNoDirectoryUntilAPlaceIsKept holds the footprint: a reader who never
// keeps a place leaves nothing on their machine.
func TestNoDirectoryUntilAPlaceIsKept(t *testing.T) {
	t.Parallel()

	config := t.TempDir()
	file := newFileFor(t, config, "/vaults/notes")
	if _, ok := file.Continuation(); ok {
		t.Fatal("a place is reported before one was kept")
	}
	if _, err := os.Stat(filepath.Join(config, "yomihon")); !os.IsNotExist(err) {
		t.Errorf("naming the file created %s before any place was kept", filepath.Join(config, "yomihon"))
	}
}

// newHandler is the route over a vault that holds every note, so a test that
// is about a field's shape or the body's size is not also about the vault.
func newHandler(t *testing.T) (*mark.File, http.Handler) {
	t.Helper()
	return newHandlerFor(t, nil)
}

// newHandlerFor is the route over a vault that holds the places given. A nil
// map is the vault that holds every one.
func newHandlerFor(t *testing.T, places vaultPlaces) (*mark.File, http.Handler) {
	t.Helper()
	file := newFile(t)
	mux := http.NewServeMux()
	mark.NewHandler(file, places, slog.New(slog.DiscardHandler)).Register(mux)
	return file, mux
}

func post(t *testing.T, handler http.Handler, form url.Values) *http.Response {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, mark.Address, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder.Result()
}

func TestTheRouteKeepsWhatAReadingPageSends(t *testing.T) {
	t.Parallel()

	file, handler := newHandler(t)
	response := post(t, handler, url.Values{
		"path":     {"Notes/alpha.md"},
		"anchor":   {"a-heading"},
		"offset":   {"320"},
		"identity": {anIdentity},
	})
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	if response.StatusCode != http.StatusNoContent {
		t.Errorf("POST %s = %d, want %d", mark.Address, response.StatusCode, http.StatusNoContent)
	}
	got, ok := file.Continuation()
	if !ok {
		t.Fatal("the route answered but kept no place")
	}
	if got.RelPath != "Notes/alpha.md" || got.Anchor != "a-heading" || got.Offset != 320 {
		t.Errorf("the route kept %+v, want the place that was posted", got)
	}
	if got.At.IsZero() {
		t.Error("the kept place records no time")
	}
}

func TestTheRouteRefusesWhatItCannotKeep(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		form url.Values
		want int
	}{
		{"a path that climbs out", url.Values{"path": {"../x.md"}, "offset": {"0"}, "identity": {anIdentity}}, http.StatusUnprocessableEntity},
		// newHandler's vault holds every note, so what refuses these two is the
		// shape of the path and not the vault failing to find a name.
		{"a path carrying a line feed", url.Values{"path": {"Notes/a\nb.md"}, "offset": {"0"}, "identity": {anIdentity}}, http.StatusUnprocessableEntity},
		{"a path carrying a NUL and a line feed", url.Values{"path": {"Notes/a\x00b\nc.md"}, "offset": {"0"}, "identity": {anIdentity}}, http.StatusUnprocessableEntity},
		{"an offset that is not a number", url.Values{"path": {"a.md"}, "offset": {"soon"}, "identity": {anIdentity}}, http.StatusUnprocessableEntity},
		{"no offset at all", url.Values{"path": {"a.md"}, "identity": {anIdentity}}, http.StatusUnprocessableEntity},
		{"no identity", url.Values{"path": {"a.md"}, "offset": {"0"}}, http.StatusUnprocessableEntity},
		{"nothing at all", url.Values{}, http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			file, handler := newHandler(t)
			response := post(t, handler, tt.form)
			defer func() {
				if err := response.Body.Close(); err != nil {
					t.Error(err)
				}
			}()
			if response.StatusCode != tt.want {
				t.Errorf("POST %s (%s) = %d, want %d", mark.Address, tt.name, response.StatusCode, tt.want)
			}
			if _, ok := file.Continuation(); ok {
				t.Error("a refused submission kept a place anyway")
			}
		})
	}
}

// TestTheRouteRefusesABodyBeyondItsCap holds the cap the threat model states
// in public. A place is four short fields; a body past the cap is not one, and
// the reader of that document is owed the number being true.
//
// The padding rides in a field validate never looks at, which is the whole
// point: an oversized anchor is refused at 256 bytes by the shape check long
// before the body is measured, so a test built on one passed with the cap
// deleted from the handler. Both sides are asserted — over the cap and just
// under it — because a cap that refuses everything is not a cap either.
func TestTheRouteRefusesABodyBeyondItsCap(t *testing.T) {
	t.Parallel()

	// A field the handler reads nothing from, so what decides each case is the
	// size of the body and nothing else.
	const padField = "pad"

	tests := []struct {
		name    string
		padding int
		want    int
	}{
		{"past the cap", 8192, http.StatusBadRequest},
		{"inside the cap", 3000, http.StatusNoContent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			file, handler := newHandler(t)
			form := url.Values{
				"path":     {"Notes/alpha.md"},
				"anchor":   {"a-heading"},
				"offset":   {"0"},
				"identity": {anIdentity},
				padField:   {strings.Repeat("a", tt.padding)},
			}
			// The case only means what it says if the body really is on the
			// side of the cap the name claims.
			size := len(form.Encode())
			if tt.want == http.StatusBadRequest && size <= 4096 {
				t.Fatalf("the %q body is %d bytes, which is not past the 4096-byte cap", tt.name, size)
			}
			if tt.want == http.StatusNoContent && size > 4096 {
				t.Fatalf("the %q body is %d bytes, which is already past the 4096-byte cap", tt.name, size)
			}
			response := post(t, handler, form)
			defer func() {
				if err := response.Body.Close(); err != nil {
					t.Error(err)
				}
			}()
			if response.StatusCode != tt.want {
				t.Errorf("POST %s with a %d-byte body = %d, want %d", mark.Address, size, response.StatusCode, tt.want)
			}
			_, kept := file.Continuation()
			if wantKept := tt.want == http.StatusNoContent; kept != wantKept {
				t.Errorf("a %d-byte body kept a place = %t, want %t", size, kept, wantKept)
			}
		})
	}
}

// TestOnlyPostReachesTheRoute holds the method. A place is a change, and a
// change does not answer a GET.
func TestOnlyPostReachesTheRoute(t *testing.T) {
	t.Parallel()

	_, handler := newHandler(t)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, mark.Address, http.NoBody))
	response := recorder.Result()
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	if response.StatusCode == http.StatusNoContent || response.StatusCode == http.StatusOK {
		t.Errorf("GET %s = %d, want a refusal", mark.Address, response.StatusCode)
	}
}

// aPost is what a reading page sends for a place, with the fields this file's
// tests do not vary already in their ordinary spelling.
func aPost(path, anchor string) url.Values {
	return url.Values{"path": {path}, "anchor": {anchor}, "offset": {"1"}, "identity": {anIdentity}}
}

// replyTo posts form and returns the status and the body the route answered.
func replyTo(t *testing.T, handler http.Handler, form url.Values) (status int, body string) {
	t.Helper()
	response := post(t, handler, form)
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	text, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read the reply: %v", err)
	}
	return response.StatusCode, string(text)
}

// TestTheRouteKeepsAPlaceOnlyInANoteTheVaultHolds holds what the route asks of
// the vault, and the one reply it gives to a refusal of either kind. The vault
// holds one note; the others are a note it does not have and a sentence that is
// a perfectly good path and names nothing — the shape of the text a visitor to
// a shared instance would want printed on the desk.
func TestTheRouteKeepsAPlaceOnlyInANoteTheVaultHolds(t *testing.T) {
	t.Parallel()

	places := vaultPlaces{{"Notes/real.md", ""}: true}
	// The sentence the route gives a shape it refuses, which a refusal for the
	// vault not holding the note must be indistinguishable from: the reader
	// cannot act on which field was wrong, and a visitor is told nothing about
	// what the vault holds.
	_, shaped := newHandlerFor(t, places)
	shapeStatus, shapeRefusal := replyTo(t, shaped, aPost("../x.md", ""))
	if shapeStatus != http.StatusUnprocessableEntity || shapeRefusal == "" {
		t.Fatalf("a path that climbs out = %d %q, want a refusal with words", shapeStatus, shapeRefusal)
	}
	tests := []struct {
		name string
		path string
		want int
	}{
		{"a note the vault holds", "Notes/real.md", http.StatusNoContent},
		{"a note the vault does not hold", "Notes/nope.md", http.StatusUnprocessableEntity},
		{"a sentence that names no note", "Visit spam.example for free prizes", http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			file, handler := newHandlerFor(t, places)
			status, body := replyTo(t, handler, aPost(tt.path, ""))
			if status != tt.want {
				t.Errorf("POST %s over %q = %d %q, want %d", mark.Address, tt.path, status, body, tt.want)
			}
			got, kept := file.Continuation()
			if wantKept := tt.want == http.StatusNoContent; kept != wantKept {
				t.Errorf("POST over %q kept a place = %t, want %t", tt.path, kept, wantKept)
			}
			if kept && got.RelPath != tt.path {
				t.Errorf("the route kept %q, want %q", got.RelPath, tt.path)
			}
			if tt.want == http.StatusUnprocessableEntity {
				if body != shapeRefusal {
					t.Errorf("a path the vault does not hold was answered %q, want the one refusal %q", body, shapeRefusal)
				}
				if strings.Contains(body, tt.path) {
					t.Errorf("the refusal %q echoes the submitted path", body)
				}
			}
		})
	}
}

// TestTheRouteAsksTheVaultAboutTheNoteAndNotTheAnchor holds the scope of the
// question. The vault answers for an anchor only where it is a heading, and a
// reading page stamps the nearest id of any kind above the window — a block
// address, a footnote. The vault here holds the note and nothing else, so a
// route that asked it about the anchor too would refuse a place the page
// honestly sent.
func TestTheRouteAsksTheVaultAboutTheNoteAndNotTheAnchor(t *testing.T) {
	t.Parallel()

	file, handler := newHandlerFor(t, vaultPlaces{{"Notes/real.md", ""}: true})
	status, body := replyTo(t, handler, aPost("Notes/real.md", "fn-3"))
	if status != http.StatusNoContent {
		t.Fatalf("POST %s over a real note under an id that is no heading = %d %q, want %d",
			mark.Address, status, body, http.StatusNoContent)
	}
	if got, ok := file.Continuation(); !ok || got.Anchor != "fn-3" {
		t.Errorf("the route kept %+v, %t, want the place under fn-3", got, ok)
	}
}

// TestAPlaceTheVaultDoesNotHoldLeavesTheKeptPlaceAlone is the half the table
// above cannot see: a refusal that wiped the file would look the same as one
// that stored nothing, and only a file that already holds a place shows it.
func TestAPlaceTheVaultDoesNotHoldLeavesTheKeptPlaceAlone(t *testing.T) {
	t.Parallel()

	file, handler := newHandlerFor(t, vaultPlaces{{"Notes/real.md", ""}: true})
	if status, body := replyTo(t, handler, aPost("Notes/real.md", "one")); status != http.StatusNoContent {
		t.Fatalf("keep the first place = %d %q, want %d", status, body, http.StatusNoContent)
	}
	if status, _ := replyTo(t, handler, aPost("Notes/nope.md", "two")); status != http.StatusUnprocessableEntity {
		t.Fatalf("a note the vault does not hold = %d, want %d", status, http.StatusUnprocessableEntity)
	}
	got, ok := file.Continuation()
	if !ok || got.RelPath != "Notes/real.md" || got.Anchor != "one" {
		t.Errorf("after the refusal the file holds %+v, %t, want the first place", got, ok)
	}
}

// TestTheRouteRefusesAVaultItIsNotGiven holds the constructor's own guard,
// which the uncertainty route has too: a route with no vault to ask would
// either accept everything or fail at the first request, and neither is a
// refusal anyone chose.
func TestTheRouteRefusesAVaultItIsNotGiven(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Error("mark.NewHandler accepted a nil Places")
		}
	}()
	mark.NewHandler(newFile(t), nil, slog.New(slog.DiscardHandler))
}

func TestAnchorShapeSharedByReaderMarks(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, anchor string
		accepted     bool
	}{
		{"empty fallback", "", true},
		{"heading", "topic-one", true},
		{"block", "^safe", true},
		{"ampersand", "a&b", true},
		{"non-ASCII control remains accepted", "a" + string(rune(0x85)) + "b", true},
		{"ASCII boundary", strings.Repeat("a", 256), true},
		{"ASCII over boundary", strings.Repeat("a", 257), false},
		{"CJK boundary", strings.Repeat("漢", 85) + "a", true},
		{"CJK over boundary", strings.Repeat("漢", 86), false},
		{"invalid UTF-8", string([]byte{0xff}), false},
	}
	for _, r := range "#?/\\\"'<>" {
		cases = append(cases, struct {
			name, anchor string
			accepted     bool
		}{fmt.Sprintf("delimiter %U", r), "a" + string(r) + "b", false})
	}
	for r := rune(0); r <= 0x20; r++ {
		cases = append(cases, struct {
			name, anchor string
			accepted     bool
		}{fmt.Sprintf("control or space %U", r), "a" + string(r) + "b", false})
	}
	cases = append(cases, struct {
		name, anchor string
		accepted     bool
	}{"DEL", "a" + string(rune(0x7f)) + "b", false})
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			file := newFile(t)
			place := aPlace()
			place.Anchor = tt.anchor
			uncertainty := anUncertainty()
			uncertainty.Anchor = tt.anchor
			_, uncertaintyErr := file.ToggleUncertainty(uncertainty, func(*mark.Uncertainty) bool { return true })
			for name, err := range map[string]error{
				"shape":        mark.ValidateAnchor(tt.anchor),
				"continuation": file.SetContinuation(place),
				"uncertainty":  uncertaintyErr,
			} {
				if tt.accepted && err != nil {
					t.Errorf("%s rejected %q: %v", name, tt.anchor, err)
				}
				if !tt.accepted && !errors.Is(err, mark.ErrInvalid) {
					t.Errorf("%s accepted %q or returned %v; want ErrInvalid", name, tt.anchor, err)
				}
			}
		})
	}
}
