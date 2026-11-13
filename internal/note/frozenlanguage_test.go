package note_test

import (
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// inPage is a phrase as it reaches the reader: the template escapes what it
// writes, so a sentence carrying an apostrophe is never on the page in the
// spelling the dictionary holds.
func inPage(p wording.Phrase, lang wording.Lang) string {
	return html.EscapeString(p.In(lang))
}

// theOtherLanguage is the one the reader did not choose.
func theOtherLanguage(lang wording.Lang) wording.Lang {
	if lang == wording.En {
		return wording.ZhHant
	}
	return wording.En
}

// bothLanguages is what each test below asks its page in. A sentence written
// before any reader arrived is right by accident in one of the two, so a test
// that asked in only one would pass on the fault it exists to catch.
var bothLanguages = []wording.Lang{wording.ZhHant, wording.En}

// sentenceFollowsTheReader holds one sentence to the language its reader asked
// in, in both directions: a page missing it has lost the sentence, and a page
// carrying the other language's spelling chose for them.
func sentenceFollowsTheReader(t *testing.T, page string, lang wording.Lang, p wording.Phrase, what string) {
	t.Helper()
	if want := inPage(p, lang); !strings.Contains(page, want) {
		t.Errorf("%s is missing from a page asked for in %q: want %q", what, lang, want)
	}
	if stale := inPage(p, theOtherLanguage(lang)); strings.Contains(page, stale) {
		t.Errorf("%s is written in the language the reader did not choose: %q is in the page", what, stale)
	}
}

// rootIslandVault holds one note nobody cites, at the top of the folder, so the
// health page has to name the folder it lives in — and that folder is the one
// with no name of its own.
func rootIslandVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	const rel = "orphan.md"
	body := "---\ntitle: Orphan\ntype: concept\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
	return root
}

// TestTheHealthPageNamesTheRootFolderInTheReadersLanguage covers a label built
// where no reader exists: the scan groups uncited notes by folder long before
// anyone asks for the page, and the folder at the top has no name to group
// under. Resolving the substitute during the scan wrote it in whichever
// language the scan guessed, and an English reader met it under an English
// heading.
func TestTheHealthPageNamesTheRootFolderInTheReadersLanguage(t *testing.T) {
	t.Parallel()

	srv := newServerWithContract(t, rootIslandVault(t), loadHomeContract(t))
	for _, lang := range bothLanguages {
		t.Run(string(lang), func(t *testing.T) {
			t.Parallel()
			page := getInLanguage(t, srv.Client(), srv.URL+"/health", lang)
			sentenceFollowsTheReader(t, page, lang, wording.VaultRoot, "the name of the folder at the top")
		})
	}
}
