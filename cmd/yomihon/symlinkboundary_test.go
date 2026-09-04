package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"log/slog"
)

// TestASymlinkOutOfTheVaultIsNeitherIndexedNorServed writes down the boundary
// the whole product stands on, in the one place that can see all of it at once.
// A symbolic link is a note-shaped name whose bytes live wherever its target
// does, so following one would put a file outside the folder on a reading page,
// in the search index, and behind an address anything on this machine can ask
// for — the vault would stop being the thing yomihon reads.
//
// The scan already refuses to follow one and now says it skipped it, which is a
// different promise from this one. This is the promise that the refusal holds
// end to end: through the routes, not only through the reader underneath them.
// It passes today, and it is written down because nothing else fails if some
// later change starts following links — the pages would simply begin showing
// somebody's home directory.
func TestASymlinkOutOfTheVaultIsNeitherIndexedNorServed(t *testing.T) {
	t.Parallel()

	const needle = "PRIVATE-BYTES-FROM-OUTSIDE-THE-VAULT"

	root := t.TempDir()
	writeDeskFixture(t, root)
	outside := filepath.Join(t.TempDir(), "private.md")
	note := "---\ntitle: Private\ntype: concept\nstatus: draft\n---\n\n" + needle + "\n"
	if err := os.WriteFile(outside, []byte(note), 0o600); err != nil {
		t.Fatalf("write the outside file: %v", err)
	}
	link := filepath.Join(root, "Concepts", "leaked.md")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("this filesystem will not hold a symbolic link: %v", err)
	}

	site, err := newReadingSite(t.Context(), root, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newReadingSite: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Errorf("readingSite.close() error = %v", closeErr)
		}
	})

	ask := func(target string) (int, string) {
		t.Helper()
		recorder := httptest.NewRecorder()
		site.ServeHTTP(recorder, siteRequest(t, http.MethodGet, target, nil))
		response := recorder.Result()
		defer func() {
			if closeErr := response.Body.Close(); closeErr != nil {
				t.Errorf("close response body: %v", closeErr)
			}
		}()
		return response.StatusCode, recorder.Body.String()
	}

	// Controls first, one per route asked about below: each answers 200 for the
	// fixture's own note, so a 404 there is the boundary answering rather than
	// a route that serves nothing to anybody.
	for _, target := range []string{"/notes/Concepts/alpha.md", "/raw/Concepts/alpha.md", "/preview/Concepts/alpha.md"} {
		if code, _ := ask(target); code != http.StatusOK {
			t.Fatalf("GET %s answered %d for a note this vault holds; a refusal below would prove nothing", target, code)
		}
	}

	for _, target := range []string{"/notes/Concepts/leaked.md", "/raw/Concepts/leaked.md", "/preview/Concepts/leaked.md"} {
		code, body := ask(target)
		if code == http.StatusOK {
			t.Errorf("GET %s = 200: a link out of the vault is being served", target)
		}
		if strings.Contains(body, needle) {
			t.Errorf("GET %s carries the bytes from outside the vault", target)
		}
	}

	// Not served is half of it. These are the faces that would carry the name
	// or the words if the scan had taken the link as a note. The search page is
	// asked about the name rather than about the words, because a search page
	// prints the query back and would report its own echo as a leak.
	for _, target := range []string{"/folders/Concepts", "/"} {
		code, body := ask(target)
		if code != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200 so its content can be read", target, code)
		}
		if strings.Contains(body, needle) {
			t.Errorf("GET %s carries the bytes from outside the vault", target)
		}
		if strings.Contains(body, "leaked.md") {
			t.Errorf("GET %s lists the link as a note of this vault", target)
		}
	}

	// The health page names the path, and must: it is where the scan reports
	// what it passed over. What it may not carry is the file's words.
	code, health := ask("/health")
	if code != http.StatusOK {
		t.Fatalf("GET /health = %d, want 200", code)
	}
	if strings.Contains(health, needle) {
		t.Error("the health page carries the bytes from outside the vault")
	}

	code, results := ask("/search?q=" + needle)
	if code != http.StatusOK {
		t.Fatalf("GET the search results = %d, want 200", code)
	}
	if strings.Contains(results, "/notes/Concepts/leaked.md") {
		t.Error("the search index holds a link out of the vault")
	}
}
