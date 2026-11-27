package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
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
	"time"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/mark/confdir"
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

// theExampleVault is a copy of the example vault, with the configuration
// directory beside it and the reading site over both.
func theExampleVault(t *testing.T) (site *readingSite, root, configDir string) {
	t.Helper()
	root = t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "examples", "vault"))); err != nil {
		t.Fatalf("copy the example vault: %v", err)
	}
	configDir = t.TempDir()
	site, err := newReadingSite(t.Context(), root, confdir.At(configDir), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newReadingSite: %v", err)
	}
	return site, root, configDir
}

const (
	exLesson  = "Lessons/L02 Add a contract.md"
	exDraft   = "Notes/go/提早返回的管線.md"
	exThought = "Notes/go/可以取消的管線的想法 2026-10-31 0900.md"
	exCourse  = "Notes/Books/Go 並行入門.md"
)

func address(prefix, rel string) string {
	segments := strings.Split(rel, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return prefix + strings.Join(segments, "/")
}

// siteServer answers one request the way the listener would and says the status.
func siteServer(t *testing.T, site *readingSite) func(method, target, contentType, body string) (code int, answer string) {
	t.Helper()
	return func(method, target, contentType, body string) (int, string) {
		req := siteRequest(t, method, target, strings.NewReader(body))
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		recorder := httptest.NewRecorder()
		site.ServeHTTP(recorder, req)
		return recorder.Code, recorder.Body.String()
	}
}

const formType = "application/x-www-form-urlencoded"

// everyReadingRoute is a GET of each page and file the site serves.
func everyReadingRoute() []string {
	identity := strings.Repeat("a", 64)
	return []string{
		"/",
		"/health",
		"/indexes",
		"/journal",
		"/folders",
		"/folders/Notes",
		"/courses",
		"/reports",
		"/marks",
		"/preferences",
		"/search?q=channel",
		"/search?q=draft%3Atrue",
		"/search/results?q=channel",
		mark.BookmarkPath,
		mark.QuestionPath,
		address("/notes/", exLesson),
		address("/notes/", exDraft),
		address("/notes/", exThought),
		address("/thought/", exDraft),
		address("/courses/", exCourse),
		address("/listen/", exCourse),
		address("/compare/", exDraft) + "?" + url.Values{pages.CompareWithParam: {exThought}}.Encode(),
		address("/raw/", exLesson),
		address("/preview/", exLesson),
		address("/freshness/", exLesson) + "?identity=" + identity,
		address("/embed/", exLesson),
		"/static/css/components.css",
		"/nowhere",
	}
}

// TestReadsWriteNothingInTheVault drives a GET of every route the production
// composition registers, over a copy of the example vault that has never been
// marked, and fails if a single entry under the vault differs afterwards: no
// file, no folder, no modification time. A page that asks about the marks
// (their list, where they are kept, whether they can be written) reads them and
// makes nothing, not even the reader folder.
func TestReadsWriteNothingInTheVault(t *testing.T) {
	t.Parallel()
	site, root, _ := theExampleVault(t)
	before := vaultState(t, root)
	serve := siteServer(t, site)
	for _, target := range everyReadingRoute() {
		if code, _ := serve(http.MethodGet, target, "", ""); code >= http.StatusInternalServerError {
			t.Errorf("GET %s = %d; a route that failed may have stopped before the read this test is about", target, code)
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
			t.Errorf("%s was created in the vault by a read", rel)
		}
	}
}

// The one place a route writes in the vault.
const marksFolder = "reader-notes/.yomihon/marks"

// TestNoRouteWritesIntoTheVaultOutsideTheMarksFolder drives every route the
// production composition registers, the ones that change state included, and
// the new ones that keep, change, delete and put back marks, over a copy of the
// example vault, and fails if a single entry outside reader-notes/.yomihon/marks/
// differs afterwards, or if a file was made in the vault anywhere else. The
// vault's own notes, and everything an agent keeps in it, are the reader's and
// are never written. What is written is the one machine file of this computer.
//
// The configuration directory is checked the other way round: the writes the
// requests below made have to have landed, in the vault's marks folder and in
// the configuration directory, so a run where every write silently failed
// cannot pass as a run where nothing wrote into the vault.
func TestNoRouteWritesIntoTheVaultOutsideTheMarksFolder(t *testing.T) {
	t.Parallel()
	site, root, configDir := theExampleVault(t)
	before := vaultState(t, root)
	serve := siteServer(t, site)
	for _, target := range everyReadingRoute() {
		if code, _ := serve(http.MethodGet, target, "", ""); code >= http.StatusInternalServerError {
			t.Errorf("GET %s = %d; a route that failed may have stopped before the read this test is about", target, code)
		}
	}
	identity := strings.Repeat("a", 64)
	for _, write := range []struct {
		target string
		form   url.Values
	}{
		{mark.BookmarkPath, url.Values{"path": {exLesson}, "offset": {"0"}, "identity": {identity}}},
		{mark.QuestionPath, url.Values{"path": {exLesson}, "anchor": {""}}},
		{mark.QuestionPath, url.Values{"path": {exDraft}, "anchor": {""}}},
		{"/preferences", url.Values{"theme": {"dark"}}},
	} {
		if code, _ := serve(http.MethodPost, write.target, formType, write.form.Encode()); code >= http.StatusBadRequest && code != http.StatusSeeOther {
			t.Errorf("POST %s = %d, want it accepted so the write it makes is one this test observes", write.target, code)
		}
	}

	// The marks routes: keep each kind, change one, delete one, put it back.
	const jsonType = "application/json"
	code, body := serve(http.MethodPost, "/mark", jsonType, `{"kind":"thought","path":"`+exLesson+`","body":"a thought of mine"}`)
	if code != http.StatusCreated {
		t.Fatalf("POST /mark of a thought = %d %s, want 201", code, body)
	}
	var kept struct {
		ID   string         `json:"id"`
		Seen map[string]int `json:"seen"`
	}
	decode := func(answer string) {
		t.Helper()
		if err := json.Unmarshal([]byte(answer), &kept); err != nil {
			t.Fatalf("the answer %q: %v", answer, err)
		}
	}
	decode(body)
	seen := func() string {
		sent, err := json.Marshal(kept.Seen)
		if err != nil {
			t.Fatal(err)
		}
		return `{"seen":` + string(sent)
	}
	if code, body = serve(http.MethodPost, "/mark", jsonType, `{"kind":"question","path":"`+exLesson+`"}`); code != http.StatusOK {
		// The question of the whole note is the one the form above marked.
		t.Errorf("POST /mark of a question = %d %s, want 200: it was marked already", code, body)
	}
	if code, body = serve(http.MethodPatch, "/mark/"+kept.ID, jsonType, seen()+`,"body":"changed"}`); code != http.StatusOK {
		t.Fatalf("PATCH /mark/%s = %d %s, want 200", kept.ID, code, body)
	}
	decode(body)
	if code, body = serve(http.MethodDelete, "/mark/"+kept.ID, jsonType, seen()+`}`); code != http.StatusOK {
		t.Fatalf("DELETE /mark/%s = %d %s, want 200", kept.ID, code, body)
	}
	decode(body)
	if code, body = serve(http.MethodPut, "/mark/"+kept.ID, jsonType, seen()+`}`); code != http.StatusOK {
		t.Errorf("PUT /mark/%s = %d %s, want 200", kept.ID, code, body)
	}
	if err := site.close(); err != nil {
		t.Fatalf("readingSite.close() error = %v", err)
	}
	after := vaultState(t, root)
	inMarks := func(rel string) bool {
		return rel == "." || rel == "reader-notes" || rel == "reader-notes/.yomihon" || rel == marksFolder || strings.HasPrefix(rel, marksFolder+"/")
	}
	for rel, was := range before {
		switch now, ok := after[rel]; {
		case inMarks(rel):
		case !ok:
			t.Errorf("%s was removed from the vault", rel)
		case now != was:
			t.Errorf("%s changed in the vault:\n before %s\n after  %s", rel, was, now)
		}
	}
	var machineFiles []string
	for rel := range after {
		if _, ok := before[rel]; ok {
			continue
		}
		if !inMarks(rel) {
			t.Errorf("%s was created in the vault, outside %s", rel, marksFolder)
		}
		if strings.HasPrefix(rel, marksFolder+"/") {
			machineFiles = append(machineFiles, rel)
		}
	}
	if len(machineFiles) != 1 || !strings.HasSuffix(machineFiles[0], ".jsonl") {
		t.Errorf("the marks folder holds %v after the requests, want this computer's one .jsonl file", machineFiles)
	}

	var kept2 []string
	if err := filepath.WalkDir(configDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr == nil && d.Type().IsRegular() {
			kept2 = append(kept2, d.Name())
		}
		return walkErr
	}); err != nil {
		t.Fatalf("read the configuration directory: %v", err)
	}
	for _, want := range []string{"ledger.jsonl", "progress.json"} {
		if !strings.Contains(strings.Join(kept2, " "), want) {
			t.Errorf("the configuration directory holds %v and no %s, so the writes above never happened and this test proves nothing", kept2, want)
		}
	}
	if strings.Contains(strings.Join(kept2, " "), "marks.json") {
		t.Errorf("the configuration directory holds a marks.json: marks are kept in the vault now")
	}
}

// TestAPageThatReadsTheMarksPutsBackWhatGitTook drives the production assembly:
// a thought is kept, git takes this computer's file out of the vault before it
// is committed, and the next page that reads the marks puts it back as it was,
// with no control for it and not a word of it in the page. Each of the two
// pages that read the marks does it, a note's and the page of marks.
func TestAPageThatReadsTheMarksPutsBackWhatGitTook(t *testing.T) {
	t.Parallel()
	for _, page := range []string{address("/notes/", exLesson), "/marks"} {
		t.Run(page, func(t *testing.T) {
			t.Parallel()
			site, root, _ := theExampleVault(t)
			serve := siteServer(t, site)
			code, body := serve(http.MethodPost, "/mark", "application/json", `{"kind":"thought","path":"`+exLesson+`","body":"a thought of mine"}`)
			if code != http.StatusCreated {
				t.Fatalf("POST /mark = %d %s, want 201", code, body)
			}
			files, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(marksFolder), "*.jsonl"))
			if err != nil || len(files) != 1 {
				t.Fatalf("the marks folder holds %v, %v; want one file", files, err)
			}
			was, err := os.ReadFile(files[0]) // #nosec G304 -- a test's own path
			if err != nil {
				t.Fatal(err)
			}
			if err = os.Remove(files[0]); err != nil {
				t.Fatal(err)
			}
			code, body = serve(http.MethodGet, page, "", "")
			if code != http.StatusOK {
				t.Fatalf("GET %s = %d, want 200", page, code)
			}
			for _, word := range []string{"git", "reader-notes", "放回", "Put back", "不在書庫"} {
				if strings.Contains(body, word) {
					t.Errorf("GET %s tells the reader %q; the reader is told nothing of where marks are kept", page, word)
				}
			}
			got, err := os.ReadFile(files[0]) // #nosec G304 -- a test's own path
			if err != nil {
				t.Fatalf("after GET %s the file is not back: %v", page, err)
			}
			if !bytes.Equal(got, was) {
				t.Errorf("after GET %s the file is not as it was:\n was %q\n got %q", page, was, got)
			}
			if err = site.close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// copyMarks stands for git: the machine files of one vault's marks folder are
// copied into another's.
func copyMarks(t *testing.T, from, to string) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(from, filepath.FromSlash(marksFolder), "*.jsonl"))
	if err != nil || len(files) == 0 {
		t.Fatalf("the marks folder of %s holds %v, %v", from, files, err)
	}
	if err = os.MkdirAll(filepath.Join(to, filepath.FromSlash(marksFolder)), 0o750); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		data, readErr := os.ReadFile(f) // #nosec G304 -- a test's own path
		if readErr != nil {
			t.Fatal(readErr)
		}
		if err = os.WriteFile(filepath.Join(to, filepath.FromSlash(marksFolder), filepath.Base(f)), data, 0o600); err != nil { // #nosec G703 -- a test copies between its own temporary folders
			t.Fatal(err)
		}
	}
}

// TestAMarkTwoComputersChangedDifferentlyShowsTheNewestValueAndNothingElse drives
// the production assembly: two computers change one thought, git brings each
// one's file to the other, and the pages and the routes that show the thought
// show the newest live value, with no sign that there is another. Both values are
// still in the files of the vault.
func TestAMarkTwoComputersChangedDifferentlyShowsTheNewestValueAndNothingElse(t *testing.T) {
	t.Parallel()
	siteA, rootA, _ := theExampleVault(t)
	siteB, rootB, _ := theExampleVault(t)
	a, b := siteServer(t, siteA), siteServer(t, siteB)

	code, body := a(http.MethodPost, "/mark", "application/json", `{"kind":"thought","path":"`+exLesson+`","body":"first words"}`)
	if code != http.StatusCreated {
		t.Fatalf("POST /mark = %d %s, want 201", code, body)
	}
	var kept struct {
		ID   string          `json:"id"`
		Seen json.RawMessage `json:"seen"`
	}
	if err := json.Unmarshal([]byte(body), &kept); err != nil {
		t.Fatal(err)
	}
	copyMarks(t, rootA, rootB)

	// B reads the mark and changes it; A, which has not read B's change, changes
	// it later, so that A's is the newest and B's is a value that does not count.
	code, body = b(http.MethodPatch, "/mark/"+kept.ID, "application/json", `{"seen":`+string(kept.Seen)+`,"body":"words of the other computer"}`)
	if code != http.StatusOK {
		t.Fatalf("PATCH on B = %d %s, want 200", code, body)
	}
	time.Sleep(5 * time.Millisecond)
	code, body = a(http.MethodPatch, "/mark/"+kept.ID, "application/json", `{"seen":`+string(kept.Seen)+`,"body":"words of this computer, the newest"}`)
	if code != http.StatusOK {
		t.Fatalf("PATCH on A = %d %s, want 200", code, body)
	}
	copyMarks(t, rootB, rootA)

	const newest, other = "words of this computer, the newest", "words of the other computer"
	for _, page := range []string{"/mark/" + kept.ID, "/marks", address("/notes/", exLesson)} {
		code, body = a(http.MethodGet, page, "", "")
		if code != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200", page, code)
		}
		if strings.Contains(body, other) {
			t.Errorf("GET %s shows the value that does not count", page)
		}
		if strings.Contains(strings.ToLower(body), "conflict") || strings.Contains(body, "衝突") {
			t.Errorf("GET %s tells the reader of a conflict", page)
		}
		if page == "/mark/"+kept.ID && !strings.Contains(body, newest) {
			t.Errorf("GET %s = %s, want the newest live value", page, body)
		}
	}
	files, err := filepath.Glob(filepath.Join(rootA, filepath.FromSlash(marksFolder), "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var all strings.Builder
	for _, f := range files {
		data, readErr := os.ReadFile(f) // #nosec G304 -- a test's own path
		if readErr != nil {
			t.Fatal(readErr)
		}
		all.Write(data)
	}
	for _, words := range []string{newest, other} {
		if !strings.Contains(all.String(), words) {
			t.Errorf("the machine files of the vault no longer hold %q", words)
		}
	}
	for _, s := range []*readingSite{siteA, siteB} {
		if err = s.close(); err != nil {
			t.Fatal(err)
		}
	}
}
