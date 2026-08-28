package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// writeDegradedFixture builds a folder where one note is captured by the scan
// but cannot be read: note-ok.md cites the locked note, so the health page has
// a citation to classify, and note-locked.md carries no read permission at
// all. The chmod is undone by the test's cleanup so the temp dir can be
// removed.
func writeDegradedFixture(t *testing.T) (root string) {
	t.Helper()
	root = t.TempDir()
	okPath := filepath.Join(root, "note-ok.md")
	if err := os.WriteFile(okPath, []byte("---\ntitle: OK\ntype: concept\n---\nsee [[note-locked]]\n"), 0o600); err != nil {
		t.Fatalf("write note-ok.md: %v", err)
	}
	lockedPath := filepath.Join(root, "note-locked.md")
	if err := os.WriteFile(lockedPath, []byte("---\ntitle: Locked\ntype: concept\n---\nlocked body\n"), 0o600); err != nil {
		t.Fatalf("write note-locked.md: %v", err)
	}
	lockNote(t, lockedPath)
	return root
}

// lockNote takes the read permission off a file for the rest of the test and
// gives it back afterwards, so the temporary directory can be removed. A
// privileged user reads the file anyway, and a folder that is not degraded
// proves nothing about the pages that report degradation.
func lockNote(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod %s: %v", filepath.Base(path), err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(path, 0o600); err != nil {
			t.Errorf("restore %s mode: %v", filepath.Base(path), err)
		}
	})
	if _, err := os.ReadFile(path); err == nil { // #nosec G304 -- probing a path inside this test's own TempDir
		t.Skip("mode 000 does not block reads here (running as a privileged user)")
	}
}

// TestUnreadableNoteIsVisibleOnEveryReadingSurface pins the three surfaces
// that must say what stderr alone said before: with one note unreadable at
// startup, the published generation is served for availability, and the home
// page states the degradation, the health page names the blocked file and
// stops classifying citations to it as pointing at nothing, and the note's
// own page says the file exists but could not be read rather than answering
// with the plain not-found page.
func TestUnreadableNoteIsVisibleOnEveryReadingSurface(t *testing.T) {
	t.Parallel()
	root := writeDegradedFixture(t)
	srv := newServer(t, root)

	code, home := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", code, http.StatusOK)
	}
	// The rail lists every file, so the name alone proves nothing; the notice
	// and the name it carries are looked for from the notice's marker on.
	if i := strings.Index(home, "讀不進來"); i < 0 {
		t.Error("home page carries no degraded notice for an unreadable source")
	} else if !strings.Contains(home[i:], "note-locked.md") {
		t.Error("home degraded notice does not name the blocked file")
	}

	code, health := get(t, srv.URL+"/health")
	if code != http.StatusOK {
		t.Fatalf("GET /health status = %d, want %d", code, http.StatusOK)
	}
	if i := strings.Index(health, "讀不進來的檔案"); i < 0 {
		t.Error("health page has no blocked-sources section")
	} else if !strings.Contains(health[i:], "note-locked.md") {
		t.Error("health blocked-sources section does not name the blocked file")
	}
	// The citation in note-ok.md lands on a file that exists; listing it with
	// the citations whose targets do not exist sends the reader to write a
	// note that is already on disk.
	if strings.Contains(health, "連到「note-locked」") {
		t.Error("health page classifies a citation to an existing but unreadable file as unwritten")
	}

	code, page := get(t, srv.URL+"/notes/note-locked.md")
	if code != http.StatusNotFound {
		t.Fatalf("GET /notes/note-locked.md status = %d, want %d", code, http.StatusNotFound)
	}
	if !strings.Contains(page, "檔案存在") {
		t.Error("the unreadable note's page does not say the file exists but could not be read")
	}
	if strings.Contains(page, wording.NothingHere.In(wording.ZhHant)) {
		t.Error("the unreadable note's page is indistinguishable from the plain not-found page")
	}
}

// TestFolderNamedLikeANoteGetsThePlainNotFoundPage separates the two repairs
// the missing-note page offers. A folder whose name ends in .md is observed by
// the scan the same way a note is, and the page saying the file exists but
// could not be read this time — clear the permission, refresh in a few
// seconds — describes a repair a folder can never satisfy. Only a regular file
// the generation captured earns that page.
func TestFolderNamedLikeANoteGetsThePlainNotFoundPage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Folder.md"), 0o750); err != nil {
		t.Fatalf("mkdir Folder.md: %v", err)
	}
	inner := filepath.Join(root, "Folder.md", "inner.md")
	if err := os.WriteFile(inner, []byte("---\ntitle: Inner\ntype: concept\n---\ninner body\n"), 0o600); err != nil {
		t.Fatalf("write inner note: %v", err)
	}
	srv := newServer(t, root)

	code, page := get(t, srv.URL+"/notes/Folder.md")
	if code != http.StatusNotFound {
		t.Fatalf("GET /notes/Folder.md status = %d, want %d", code, http.StatusNotFound)
	}
	if strings.Contains(page, "這個檔案目前讀不進來") {
		t.Error("a folder named like a note is sent to repair a file's permissions")
	}
	if !strings.Contains(page, wording.NothingHere.In(wording.ZhHant)) {
		t.Error("a folder named like a note does not get the plain not-found page")
	}
}

// TestDegradedSurfacesNameEveryFileTheFolderCouldNotRead is the plural case of
// the same two surfaces. One blocked file is the case where naming the first
// one and naming all of them look identical; with two, a report that stops at
// the first sends the reader to clear one permission and leaves the folder
// degraded with no account of the rest. Home says how many there are and lists
// them; the health page gives each its own line.
func TestDegradedSurfacesNameEveryFileTheFolderCouldNotRead(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const (
		first  = "note-locked-one.md"
		second = "note-locked-two.md"
	)
	if err := os.WriteFile(filepath.Join(root, "note-ok.md"),
		[]byte("---\ntitle: OK\ntype: concept\n---\nreadable body\n"), 0o600); err != nil {
		t.Fatalf("write note-ok.md: %v", err)
	}
	for _, name := range []string{first, second} {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte("---\ntitle: Locked\ntype: concept\n---\nlocked body\n"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		lockNote(t, path)
	}
	srv := newServer(t, root)

	code, home := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", code, http.StatusOK)
	}
	if !strings.Contains(home, "有 2 個檔案讀不進來") {
		t.Error("home page does not say how many files the folder could not read")
	}
	// The rail lists every file in the folder, so a name found anywhere on the
	// page proves nothing. The notice's own detail is what is read here.
	detail := degradedDetail(t, home)
	for _, name := range []string{first, second} {
		if !strings.Contains(detail, name) {
			t.Errorf("home degraded notice detail %q does not name %s", detail, name)
		}
	}

	code, health := get(t, srv.URL+"/health")
	if code != http.StatusOK {
		t.Fatalf("GET /health status = %d, want %d", code, http.StatusOK)
	}
	blocked := healthBlockedSection(t, health)
	for _, name := range []string{first, second} {
		if !strings.Contains(blocked, `<span class="y-healthlist__name">`+name+`</span>`) {
			t.Errorf("health blocked-sources section does not list %s as a file of its own", name)
		}
	}
}

// TestStatusFaceReadsTheFileNotTheGeneration pins where the write face gets
// the status it puts under the controls. The rest of the page comes from a
// generation that lags the folder by a couple of seconds, and lags it without
// bound for a file that generation could not re-read and carried. A body that
// is a few seconds old is what reading costs; a status that is a few seconds
// old is a control acting on a value the reader has already changed.
func TestStatusFaceReadsTheFileNotTheGeneration(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const rel = "Concepts/Carried.md"
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(path,
		[]byte("---\ntitle: Carried\ntype: concept\nstatus: seedling\n---\ncarried body\n"), 0o600); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))

	// The generation now holds the status the file carried when it was built.
	// The file moves on without it.
	if err := os.WriteFile(path,
		[]byte("---\ntitle: Carried\ntype: concept\nstatus: growing\n---\ncarried body\n"), 0o600); err != nil {
		t.Fatalf("rewrite %s: %v", rel, err)
	}

	code, page := get(t, srv.URL+"/notes/"+rel)
	if code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want %d", rel, code, http.StatusOK)
	}
	if !strings.Contains(page, "ui-status--growing") {
		t.Error("the status face does not show the status the file itself carries")
	}
	if strings.Contains(page, "ui-status--seedling") {
		t.Error("the status face shows the generation's copy, which a transition would be built from")
	}
}

// degradedDetail returns the technical detail the home page's degraded notice
// carries, which is the part of the page that names the files. Reading it out
// of the notice keeps the assertion off the file rail, where every name in the
// folder appears whether or not anything went wrong.
func degradedDetail(t *testing.T, page string) string {
	t.Helper()
	_, afterNotice, ok := strings.Cut(page, "個檔案讀不進來")
	if !ok {
		t.Fatal("home page carries no degraded notice")
	}
	// The opening tag is matched by its name, not by the exact attributes it
	// happens to carry: what this reads is the notice's machine detail, and a
	// styling hook added to that element is not a change in what it holds.
	_, afterOpen, ok := strings.Cut(afterNotice, "<code")
	if !ok {
		t.Fatal("the home page's degraded notice carries no detail naming what could not be read")
	}
	_, detail, ok := strings.Cut(afterOpen, ">")
	if !ok {
		t.Fatal("the home page's degraded detail has an unclosed opening tag")
	}
	detail, _, ok = strings.Cut(detail, "</code>")
	if !ok {
		t.Fatal("the home page's degraded detail is not closed")
	}
	return detail
}

// healthBlockedSection returns the blocked-sources section of the health page,
// bounded by the section that follows it, so a name found in a later section
// cannot stand in for one this section left out.
func healthBlockedSection(t *testing.T, page string) string {
	t.Helper()
	_, section, ok := strings.Cut(page, "讀不進來的檔案")
	if !ok {
		t.Fatal("health page has no blocked-sources section")
	}
	if before, _, found := strings.Cut(section, "連到不存在的目標"); found {
		section = before
	}
	return section
}
