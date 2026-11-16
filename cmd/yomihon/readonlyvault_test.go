package main

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/ui/pages"
)

// vaultState is everything a write into the vault could change, entry by
// entry: what kind of entry it is, its permission bits, its size, when it was
// last modified, and for a file its bytes. A directory is recorded too, so a
// sibling temporary file created and removed again still moves its parent's
// modification time.
func vaultState(t *testing.T, root string) map[string]string {
	t.Helper()
	pinned, err := os.OpenRoot(root)
	if err != nil {
		t.Fatalf("open the vault under %s: %v", root, err)
	}
	defer func() {
		if closeErr := pinned.Close(); closeErr != nil {
			t.Errorf("close the vault under %s: %v", root, closeErr)
		}
	}()
	state := map[string]string{}
	walkErr := fs.WalkDir(pinned.FS(), ".", func(rel string, d fs.DirEntry, entryErr error) error {
		if entryErr != nil {
			return entryErr
		}
		info, err := pinned.Lstat(rel)
		if err != nil {
			return err
		}
		entry := fmt.Sprintf("%v %d %s", info.Mode(), info.Size(), info.ModTime().UTC().Format("2006-01-02T15:04:05.000000000"))
		if d.Type().IsRegular() {
			data, err := pinned.ReadFile(rel)
			if err != nil {
				return err
			}
			entry += fmt.Sprintf(" %x", sha256.Sum256(data))
		}
		state[rel] = entry
		return nil
	})
	if walkErr != nil {
		t.Fatalf("record the vault under %s: %v", root, walkErr)
	}
	return state
}

// TestNoRouteWritesIntoTheVault drives every route the production composition
// registers, the three that change state included, over a copy of the example
// vault, and fails if a single entry under the vault differs afterwards. yomihon
// is read-only toward the vault: what a reader does is kept in files of its own
// under the configuration directory, and this is where that promise is held at
// the level of the running server rather than of one package.
//
// The configuration directory is checked too, the other way round: the marks
// the requests below kept have to have landed there, so a run where every
// write silently failed cannot pass as a run where nothing wrote into the vault.
func TestNoRouteWritesIntoTheVault(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "examples", "vault"))); err != nil {
		t.Fatalf("copy the example vault: %v", err)
	}
	configDir := t.TempDir()
	before := vaultState(t, root)

	site, err := newReadingSite(t.Context(), root, configDir, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newReadingSite: %v", err)
	}

	address := func(prefix, rel string) string {
		segments := strings.Split(rel, "/")
		for i, segment := range segments {
			segments[i] = url.PathEscape(segment)
		}
		return prefix + strings.Join(segments, "/")
	}
	const (
		lesson  = "Lessons/L02 Add a contract.md"
		draft   = "Notes/go/提早返回的管線.md"
		thought = "Notes/go/可以取消的管線的想法 2026-10-31 0900.md"
		course  = "Notes/Books/Go 並行入門.md"
	)
	serve := func(method, target, body string) int {
		req := siteRequest(t, method, target, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		recorder := httptest.NewRecorder()
		site.ServeHTTP(recorder, req)
		return recorder.Code
	}
	identity := strings.Repeat("a", 64)
	for _, target := range []string{
		"/",
		"/health",
		"/maps",
		"/journal",
		"/folders",
		"/folders/Notes",
		"/courses",
		"/reports",
		"/open-thoughts",
		"/preferences",
		"/search?q=channel",
		"/search?q=draft%3Atrue",
		"/search/results?q=channel",
		mark.UncertaintyAddress,
		address("/notes/", lesson),
		address("/notes/", draft),
		address("/notes/", thought),
		address("/thought/", draft),
		address("/courses/", course),
		address("/listen/", course),
		address("/compare/", draft) + "?" + url.Values{pages.CompareWithParam: {thought}}.Encode(),
		address("/raw/", lesson),
		address("/preview/", lesson),
		address("/freshness/", lesson) + "?identity=" + identity,
		address("/embed/", lesson),
		"/static/css/components.css",
		"/nowhere",
	} {
		if code := serve(http.MethodGet, target, ""); code >= http.StatusInternalServerError {
			t.Errorf("GET %s = %d; a route that failed may have stopped before the read this test is about", target, code)
		}
	}
	for _, write := range []struct {
		target string
		form   url.Values
	}{
		{mark.Address, url.Values{"path": {lesson}, "offset": {"0"}, "identity": {identity}}},
		{mark.UncertaintyAddress, url.Values{"path": {lesson}, "anchor": {""}}},
		{mark.UncertaintyAddress, url.Values{"path": {draft}, "anchor": {""}}},
		{"/preferences", url.Values{"theme": {"dark"}}},
	} {
		if code := serve(http.MethodPost, write.target, write.form.Encode()); code >= http.StatusBadRequest && code != http.StatusSeeOther {
			t.Errorf("POST %s = %d, want it accepted so the write it makes is one this test observes", write.target, code)
		}
	}

	if err := site.close(); err != nil {
		t.Fatalf("readingSite.close() error = %v", err)
	}
	after := vaultState(t, root)
	for rel, was := range before {
		if now, ok := after[rel]; !ok {
			t.Errorf("%s was removed from the vault", rel)
		} else if now != was {
			t.Errorf("%s changed in the vault:\n before %s\n after  %s", rel, was, now)
		}
	}
	for rel := range after {
		if _, ok := before[rel]; !ok {
			t.Errorf("%s was created in the vault", rel)
		}
	}

	var kept []string
	if err := filepath.WalkDir(configDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr == nil && d.Type().IsRegular() {
			kept = append(kept, d.Name())
		}
		return walkErr
	}); err != nil {
		t.Fatalf("read the configuration directory: %v", err)
	}
	for _, want := range []string{"reader.json", "uncertainty.json"} {
		if !strings.Contains(strings.Join(kept, " "), want) {
			t.Errorf("the configuration directory holds %v and no %s, so the writes above never happened and this test proves nothing", kept, want)
		}
	}
}
