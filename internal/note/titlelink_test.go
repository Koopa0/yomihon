package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeNotes(t *testing.T, notes map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range notes {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return root
}

// TestNotePageDoesNotDenyANoteItCanSee holds the reading page to what the
// folder actually contains. A citation written against a note's title finds
// nothing — a title is deliberately not a name a link resolves by, which is
// what the vault's own reader does with one too — but the note is right there,
// and telling the reader it does not exist is false about a file they can open.
func TestNotePageDoesNotDenyANoteItCanSee(t *testing.T) {
	t.Parallel()

	root := writeNotes(t, map[string]string{
		"Concepts/golang/Citing.md": "---\ntype: concept\ncreated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n# Citing\n\n見 [[The Long Title]]。\n",
		"Concepts/golang/Target.md": "---\ntype: concept\ncreated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n# The Long Title\n\nbody\n",
	})
	srv := newServerWithContract(t, root, loadHomeContract(t))

	code, page := get(t, srv.Client(), srv.URL+"/notes/Concepts/golang/Citing.md")
	if code != http.StatusOK {
		t.Fatalf("note page status = %d, want %d", code, http.StatusOK)
	}
	if strings.Contains(page, "還沒有") {
		t.Error("the page says the note has not been written, about a note the folder holds")
	}
	if !strings.Contains(page, "title") {
		t.Error("the page does not say the name was a title, so a reader is not told what to fix")
	}
}

// TestNotePageNamesEveryHolderOfASharedTitle holds the collision stance the
// reading pages take: list them all. Naming one would be a guess, and the
// reader is the one who knows which they meant.
func TestNotePageNamesEveryHolderOfASharedTitle(t *testing.T) {
	t.Parallel()

	shared := "---\ntype: concept\ncreated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n# Shared Name\n\nbody\n"
	root := writeNotes(t, map[string]string{
		"Concepts/golang/Citing.md": "---\ntype: concept\ncreated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n# Citing\n\n見 [[Shared Name]]。\n",
		"Concepts/golang/A.md":      shared,
		"Concepts/golang/B.md":      shared,
	})
	srv := newServerWithContract(t, root, loadHomeContract(t))

	_, page := get(t, srv.Client(), srv.URL+"/notes/Concepts/golang/Citing.md")
	if strings.Contains(page, "還沒有") {
		t.Error("the page denies a note two files answer to")
	}
	if !strings.Contains(page, "篇筆記共同的 title") {
		t.Error("the page does not say several notes share the title, so it is either guessing or silent")
	}
}

// TestHealthDoesNotCallATitleReferencedNoteUncited holds the other half of the
// same truth. A note reached only by its title is cited by nobody as far as
// the resolver is concerned, because a title is not a name a link follows —
// but someone wrote its name down, and listing it among the notes nothing
// reaches would be true of the graph and false of the folder.
func TestHealthDoesNotCallATitleReferencedNoteUncited(t *testing.T) {
	t.Parallel()

	root := writeNotes(t, map[string]string{
		"Concepts/golang/Citing.md": "---\ntype: concept\ncreated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n# Citing\n\n見 [[The Long Title]]。\n",
		"Concepts/golang/Target.md": "---\ntype: concept\ncreated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n# The Long Title\n\nbody\n",
	})
	srv := newServerWithContract(t, root, loadHomeContract(t))

	_, page := get(t, srv.Client(), srv.URL+"/health")
	if !strings.Contains(page, "Citing") {
		t.Fatal("the citing note is not on the page at all, so this proves nothing about which list it is in")
	}
	islands := healthSectionBody(t, page, "沒有正文 [[…]] 連結連過來的筆記")
	if strings.Contains(islands, "Target") {
		t.Error("a note someone reached by its title is listed among the notes nothing reaches")
	}
}
