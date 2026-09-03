package note_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

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
	page := getInLanguage(t, srv.URL+"/health", wording.En)

	if want := wording.VaultRoot.In(wording.En); !strings.Contains(page, want) {
		t.Errorf("the health page does not name the root folder in English: want %q", want)
	}
	if stale := wording.VaultRoot.In(wording.ZhHant); strings.Contains(page, stale) {
		t.Errorf("the health page still names the root folder in the default language: %q is in the page", stale)
	}
}
