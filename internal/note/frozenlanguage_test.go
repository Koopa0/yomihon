package note_test

import (
	"context"
	"errors"
	"html"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/schema"
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
			page := getInLanguage(t, srv.URL+"/health", lang)
			sentenceFollowsTheReader(t, page, lang, wording.VaultRoot, "the name of the folder at the top")
		})
	}
}

// governedLessonVault holds one lesson the contract governs, so the reading
// page for it reaches the status panel rather than stopping short of it.
func governedLessonVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := "---\ntitle: L01\ntype: lesson\ndomain: japanese\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "L01.md"), []byte(body), 0o600); err != nil {
		t.Fatalf("write the lesson: %v", err)
	}
	return root
}

// newServerWithObservedStatus is newServerWithContract with the note's own
// status line supplied by the caller, so a test can hold the read that yomihon
// cannot repeat at the moment a reader asks.
func newServerWithObservedStatus(
	t *testing.T,
	root string,
	contract *schema.Contract,
	observed func(context.Context, string) (string, error),
	releaseWriter bool,
) *httptest.Server {
	t.Helper()
	governance := schema.Ungoverned()
	if contract != nil {
		governance = contract.Governance()
	}
	mux := http.NewServeMux()
	log := slog.New(slog.DiscardHandler)
	store, source := newSnapshotStore(t, root, log, contract, governance)
	writer := openStatusWriter(t, source, contract, governance)
	if releaseWriter {
		if err := writer.Close(); err != nil {
			t.Fatalf("release the write face: %v", err)
		}
	}
	note.New(&note.Sources{
		Source:         source,
		Status:         writer.Authority,
		Snapshot:       store.Current,
		ObservedStatus: observed,
		ConsumeReceipt: writer.ConsumeReceipt,
		Log:            log,
	}).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestAReleasedWriteFaceSpeaksTheReadersLanguage covers the sentence yomihon
// says about itself rather than about the folder: the process held a write
// face and lost it. It was written at package scope, in whichever language was
// guessed there, and reached a reader through an authority sample that carries
// no request — so an English reader was told in Traditional Chinese.
func TestAReleasedWriteFaceSpeaksTheReadersLanguage(t *testing.T) {
	t.Parallel()

	srv := newServerWithObservedStatus(t, governedLessonVault(t), loadContract(t),
		func(context.Context, string) (string, error) { return "draft", nil }, true)
	for _, lang := range bothLanguages {
		t.Run(string(lang), func(t *testing.T) {
			t.Parallel()
			page := getInLanguage(t, srv.URL+"/notes/Writing/lessons/japanese/L01.md", lang)
			sentenceFollowsTheReader(t, page, lang, wording.ContractUnavailable, "the explanation of a released write face")
			// The rail states the same closure, and it used to state it only
			// because the refusal carried a sentence. A refusal whose whole
			// answer is its summary still has to reach the reader.
			if want := inPage(wording.ArtifactsUnavailable, lang); !strings.Contains(page, want) {
				t.Errorf("the rail says nothing about the closed instance projections: want %q", want)
			}
		})
	}
}

// TestANoteWhoseStatusCouldNotBeReadSpeaksTheReadersLanguage covers the other
// sentence written before any reader existed: the note's own status line was
// unreadable at the moment the page was assembled.
func TestANoteWhoseStatusCouldNotBeReadSpeaksTheReadersLanguage(t *testing.T) {
	t.Parallel()

	srv := newServerWithObservedStatus(t, governedLessonVault(t), loadContract(t),
		func(context.Context, string) (string, error) { return "", errors.New("refused") }, false)
	for _, lang := range bothLanguages {
		t.Run(string(lang), func(t *testing.T) {
			t.Parallel()
			page := getInLanguage(t, srv.URL+"/notes/Writing/lessons/japanese/L01.md", lang)
			sentenceFollowsTheReader(t, page, lang, wording.NoteStatusUnreadable, "the note's unreadable status line")
		})
	}
}
