package note_test

import (
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/vault"
)

const freshRel = "Writing/watched.md"

// writeFreshNote puts body at rel under root, creating the folders it needs.
func writeFreshNote(t *testing.T, root, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(freshRel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", full, err)
	}
}

// identityOf is what a reading page carries for these bytes.
func identityOf(body string) string {
	sum := vault.ContentIdentity([]byte(body))
	return hex.EncodeToString(sum[:])
}

// askFreshness performs the request a page open on rel makes every few
// seconds, and returns the status code beside the answer.
func askFreshness(t *testing.T, srvURL, identity string) (code int, answer string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
		srvURL+"/freshness/"+freshRel+"?identity="+identity, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest error = %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /freshness/%s error = %v", freshRel, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("Body.Close() error = %v", closeErr)
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body error = %v", err)
	}
	return resp.StatusCode, string(body)
}

func TestFreshnessSaysUnchangedWhileTheNoteIsUntouched(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const body = "# Watched\n\nThe words a reader is reading.\n"
	writeFreshNote(t, root, body)
	srv := newServer(t, root)

	code, got := askFreshness(t, srv.URL, identityOf(body))
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "unchanged" {
		t.Errorf("freshness of an untouched note = %q, want %q", got, "unchanged")
	}
}

// TestFreshnessOffersReloadOnlyOncePublishedCatchesUp is the invariant behind
// the two answers: a page is invited to reload only when reloading would
// render what the check just saw on disk. While the published generation still
// holds the old bytes, a reload would repeat them, so the answer says a new
// version was detected and withholds the invitation.
func TestFreshnessOffersReloadOnlyOncePublishedCatchesUp(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const rendered = "# Watched\n\nWhat the page rendered.\n"
	writeFreshNote(t, root, rendered)
	// The generation is built here and never rebuilt in this test, so it holds
	// exactly the bytes on disk at this moment.
	srv := newServer(t, root)

	// The published generation agrees with the disk and both differ from what
	// this page rendered: reloading now shows the reader something new.
	code, got := askFreshness(t, srv.URL, identityOf("# Watched\n\nAn older draft.\n"))
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "stale" {
		t.Fatalf("freshness with the generation caught up = %q, want %q", got, "stale")
	}

	// Now the disk moves ahead of the generation. Reloading would render the
	// same bytes again, so the invitation is withheld.
	writeFreshNote(t, root, "# Watched\n\nJust saved in Obsidian.\n")
	code, got = askFreshness(t, srv.URL, identityOf(rendered))
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "preparing" {
		t.Errorf("freshness with the generation behind the disk = %q, want %q", got, "preparing")
	}
}

func TestFreshnessSaysGoneWhenTheFileLeaves(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const body = "# Watched\n\nHere for now.\n"
	writeFreshNote(t, root, body)
	srv := newServer(t, root)

	if err := os.Remove(filepath.Join(root, filepath.FromSlash(freshRel))); err != nil {
		t.Fatalf("Remove error = %v", err)
	}
	code, got := askFreshness(t, srv.URL, identityOf(body))
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "gone" {
		t.Errorf("freshness of a removed note = %q, want %q", got, "gone")
	}
}

// TestFreshnessNeverCallsAnUnreadableFileGone holds the line the write face
// holds: what could not be confirmed does not become a confirmed fact. A
// passing read failure must not reach a reader as a deletion.
func TestFreshnessNeverCallsAnUnreadableFileGone(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const body = "# Watched\n\nStill on disk.\n"
	writeFreshNote(t, root, body)
	srv := newServer(t, root)
	lockNote(t, filepath.Join(root, filepath.FromSlash(freshRel)))

	code, got := askFreshness(t, srv.URL, identityOf(body))
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got == "gone" {
		t.Fatal("an unreadable note is reported as removed; a passing read failure must not become a deletion")
	}
	if got != "unreadable" {
		t.Errorf("freshness of an unreadable note = %q, want %q", got, "unreadable")
	}
}

// TestFreshnessIgnoresAStatusOnlyRewrite pins what the carried identity covers.
// A status value is the one field yomihon writes, and the page reads it live on
// every request, so a flip is never the stale thing a banner would be
// announcing.
func TestFreshnessIgnoresAStatusOnlyRewrite(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const before = "---\ntype: lesson\nstatus: draft\n---\n\nBody.\n"
	const after = "---\ntype: lesson\nstatus: ready\n---\n\nBody.\n"
	writeFreshNote(t, root, before)
	srv := newServer(t, root)
	writeFreshNote(t, root, after)

	code, got := askFreshness(t, srv.URL, identityOf(before))
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "unchanged" {
		t.Errorf("freshness after a status-only rewrite = %q, want %q", got, "unchanged")
	}
}

func TestFreshnessRefusesAnIdentityItCannotHaveIssued(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFreshNote(t, root, "# Watched\n\nBody.\n")
	srv := newServer(t, root)

	for _, tt := range []struct {
		name     string
		identity string
	}{
		{name: "absent", identity: ""},
		{name: "too short", identity: "abcdef"},
		{name: "right width, not hex", identity: strings.Repeat("z", 64)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code, _ := askFreshness(t, srv.URL, tt.identity)
			if code != http.StatusBadRequest {
				t.Errorf("status for a %s identity = %d, want %d", tt.name, code, http.StatusBadRequest)
			}
		})
	}
}

// TestReadingPageSendsNoBannerOfItsOwn holds the shape of the whole feature: the
// notice is the client's, built only once the server has said the file moved on.
// A server that rendered it and left the client to hide it would leave a reader
// without scripting looking at a permanent claim that the page is out of date.
// The watch attributes are asserted alongside, so an absent banner cannot be
// read as proof when the real cause is a feature that is not wired at all.
func TestReadingPageSendsNoBannerOfItsOwn(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFreshNote(t, root, "# Watched\n\nThe words a reader is reading.\n")
	srv := newServer(t, root)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/notes/"+freshRel, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest error = %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET the reading page error = %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("Body.Close() error = %v", closeErr)
		}
	}()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body error = %v", err)
	}
	page := string(raw)

	if !strings.Contains(page, "data-freshness-path=") {
		t.Fatal("the reading page carries no watch attributes, so an absent banner proves nothing")
	}
	if strings.Contains(page, "y-freshness") {
		t.Error("the server rendered the freshness banner; it must exist only after the client is told the file moved on")
	}
}

// TestFreshnessNeverCallsAnUnfindableNoteGone is the other half of the refusal
// above, and it is a different code path: this one never reaches the file at
// all. A folder that cannot be searched makes the note unfindable rather than
// absent, and the two must not be reported as the same thing — a reader whose
// permissions slipped for a moment has not lost a note.
func TestFreshnessNeverCallsAnUnfindableNoteGone(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const body = "# Watched\n\nStill on disk.\n"
	writeFreshNote(t, root, body)
	srv := newServer(t, root)

	// The folder is closed after the generation was built, so the note is in
	// the snapshot and out of reach at once.
	folder := filepath.Dir(filepath.Join(root, filepath.FromSlash(freshRel)))
	if err := os.Chmod(folder, 0o000); err != nil {
		t.Fatalf("chmod folder: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(folder, 0o750); err != nil { // #nosec G302 -- a directory needs its search bit; this restores the mode the test removed
			t.Errorf("restore folder mode: %v", err)
		}
	})
	if _, err := os.ReadFile(filepath.Join(folder, "watched.md")); err == nil { // #nosec G304 -- probing a path inside this test's own TempDir
		t.Skip("mode 000 does not block a directory here (running as a privileged user)")
	}

	code, got := askFreshness(t, srv.URL, identityOf(body))
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got == "gone" {
		t.Fatal("a note behind a closed folder is reported as removed; not being able to look is not the same as nothing being there")
	}
	if got != "unreadable" {
		t.Errorf("freshness of a note behind a closed folder = %q, want %q", got, "unreadable")
	}
}
