package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// notePath escapes a vault path for a request. A note whose own name carries
// a hash is exactly the subject here, and an unescaped one would cut the
// request short and fetch a different note.
func notePath(rel string) string {
	return strings.ReplaceAll(rel, "#", "%23")
}

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
		"Concepts/golang/Citing.md": "---\ntitle: Citing\ntype: concept\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n\n見 [[The Long Title]]。\n",
		"Concepts/golang/Target.md": "---\ntitle: The Long Title\ntype: concept\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n\nbody\n",
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

	shared := "---\ntitle: Shared Name\ntype: concept\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n\nbody\n"
	root := writeNotes(t, map[string]string{
		"Concepts/golang/Citing.md": "---\ntitle: Citing\ntype: concept\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n\n見 [[Shared Name]]。\n",
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

// TestNotePageReportsATitleCutAtAHash covers the frontmatter shape that loses
// its own words: an unquoted value ends at a space followed by "#", because
// that is where YAML starts a comment.
func TestNotePageReportsATitleCutAtAHash(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		rel      string
		title    string
		wantSaid bool
	}{
		{
			name:     "an unquoted title loses everything after the hash",
			rel:      "Concepts/golang/場景 #7 未命名.md",
			title:    "場景 #7 未命名",
			wantSaid: true,
		},
		{
			// A hash with no space before it starts no comment, so this title
			// arrives whole. Reporting it would accuse a file with nothing
			// wrong with it.
			name:     "a hash with no space before it is not a comment",
			rel:      "Concepts/golang/trailing#nospace.md",
			title:    "trailing#nospace",
			wantSaid: false,
		},
		{
			// The same rule where it actually bites: the title is a prefix of
			// the filename and the remainder begins with a hash, but with no
			// space before it — so YAML never cut anything and this title was
			// written exactly as it stands. A check that looked only for a
			// hash would accuse it.
			name:     "a filename whose extra part begins with a bare hash",
			rel:      "Concepts/golang/Plan#2.md",
			title:    "\"Plan\"",
			wantSaid: false,
		},
		{
			name:     "an ordinary title unlike its filename is not this",
			rel:      "Concepts/golang/Filename.md",
			title:    "A Quite Different Title",
			wantSaid: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := writeNotes(t, map[string]string{
				tc.rel: "---\ntitle: " + tc.title + "\ntype: concept\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n\nbody\n",
			})
			srv := newServerWithContract(t, root, loadHomeContract(t))

			_, page := get(t, srv.Client(), srv.URL+"/notes/"+notePath(tc.rel))
			said := strings.Contains(page, "截斷")
			if said != tc.wantSaid {
				t.Errorf("the page says the title was cut at a hash = %v, want %v", said, tc.wantSaid)
			}
		})
	}
}

// TestTheCutTitleSentenceIsTrueForADeliberatelyShortTitle is the edge the
// wording is built around. A title written short on purpose, in quotes,
// produces exactly the coincidence an unquoted truncation produces, and
// nothing in the parsed frontmatter separates them. The sentence therefore
// has to be one that stays true for the author who meant it — it reports the
// coincidence and offers the remedy, and accuses nobody of anything.
func TestTheCutTitleSentenceIsTrueForADeliberatelyShortTitle(t *testing.T) {
	t.Parallel()

	root := writeNotes(t, map[string]string{
		"Concepts/golang/Plan #2 final.md": "---\ntitle: \"Plan\"\ntype: concept\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n\nbody\n",
	})
	srv := newServerWithContract(t, root, loadHomeContract(t))

	_, page := get(t, srv.Client(), srv.URL+"/notes/"+notePath("Concepts/golang/Plan #2 final.md"))
	if !strings.Contains(page, "恰好是") {
		t.Error("the sentence does not read as an observed coincidence, so it accuses an author who wrote what they meant")
	}
	for _, accusation := range []string{"錯誤", "遺失", "應該"} {
		if strings.Contains(page, accusation) {
			t.Errorf("the sentence carries %q, which is false about a title written short on purpose", accusation)
		}
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
		"Concepts/golang/Citing.md": "---\ntitle: Citing\ntype: concept\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n\n見 [[The Long Title]]。\n",
		"Concepts/golang/Target.md": "---\ntitle: The Long Title\ntype: concept\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n\nbody\n",
	})
	srv := newServerWithContract(t, root, loadHomeContract(t))

	_, page := get(t, srv.Client(), srv.URL+"/health")
	if !strings.Contains(page, "Citing") {
		t.Fatal("the citing note is not on the page at all, so this proves nothing about which list it is in")
	}
	islands := healthSectionBody(t, page, "沒有人連過來的筆記")
	if strings.Contains(islands, "Target") {
		t.Error("a note someone reached by its title is listed among the notes nothing reaches")
	}
}
