package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/wording"
)

// pageIn asks the site for one page as a reader who has chosen lang and returns
// the status it answered with beside the page, because the course page's status
// is half of what this file is about and readingPageIn refuses anything but 200.
func pageIn(t *testing.T, site http.Handler, target string, lang wording.Lang) (code int, page string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := siteRequest(t, http.MethodGet, target, nil)
	// #nosec G124 -- the language cookie the server itself sets carries none of
	// those attributes, and a request that added them would be asking the
	// handler about a reader who does not exist.
	request.AddCookie(&http.Cookie{Name: wording.CookieName, Value: string(lang)})
	site.ServeHTTP(recorder, request)
	response := recorder.Result()
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Errorf("close %s response: %v", target, err)
		}
	}()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read %s response: %v", target, err)
	}
	return response.StatusCode, string(body)
}

// TestAContractEditedUnderARunningInstanceSaysToRestart is the moment the
// latch exists for, seen from the reader's chair. An author reads a diagnostic
// that invites an edit to the contract, makes it, and reloads the page they
// were on. Until yomihon is started again the contract's bytes are no longer
// the ones it read, so nothing derived from it may be shown — and what the
// reader is owed is the reason and the step, in their own language. They were
// handed a 404 that blamed the address they typed, and a line of English
// diagnostic on the desk and the course index.
//
// One comment line is the whole edit: it changes no declaration, which is why
// the page must not describe a fault in the contract. The latch is unchanged
// and so is the diagnostic the commands and the log keep; only what the three
// reading surfaces say is under test.
func TestAContractEditedUnderARunningInstanceSaysToRestart(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeRecoverySiteFixture(t, root)
	site, err := newReadingSite(t.Context(), root, t.TempDir(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newReadingSite: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Errorf("readingSite.close() error = %v", closeErr)
		}
	})

	surfaces := []struct{ name, target string }{
		{"the course page", "/syllabus/Maps/study.md"},
		{"the course read aloud", "/listen/Maps/study.md"},
		{"the paths index", "/paths"},
		{"the maps index", "/maps"},
		{"the desk", "/"},
	}
	restartIn := func(lang wording.Lang) string {
		return wording.JoinGuide(wording.ContractChanged, wording.ContractChangedNext, lang)
	}

	// The control. Before the edit every surface answers, and none says to
	// restart: otherwise the sentence below would be furniture on these pages
	// rather than news about the edit.
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		for _, surface := range surfaces {
			code, page := pageIn(t, site, surface.target, lang)
			if code != http.StatusOK {
				t.Fatalf("%s in %s before the edit = %d, want 200", surface.name, lang, code)
			}
			if strings.Contains(words(page), restartIn(lang)) {
				t.Errorf("%s in %s says to restart before anything was edited", surface.name, lang)
			}
		}
	}

	contractPath := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	original, err := os.ReadFile(contractPath) // #nosec G304 -- the fixture this test just wrote, under t.TempDir
	if err != nil {
		t.Fatalf("read the contract: %v", err)
	}
	if err = os.WriteFile(contractPath, append(original, []byte("\n# note: a comment\n")...), 0o600); err != nil { // #nosec G703 -- fixed contract path under t.TempDir
		t.Fatalf("edit the contract under the running instance: %v", err)
	}

	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		for _, surface := range surfaces {
			t.Run(string(lang)+"/"+surface.name, func(t *testing.T) {
				t.Parallel()

				code, page := pageIn(t, site, surface.target, lang)
				if want := restartIn(lang); !strings.Contains(words(page), want) {
					t.Errorf("%s in %s does not say %q; status %d", surface.name, lang, want, code)
				}
				// The English diagnostic belongs to the log and to check. It is
				// not the interface's voice in either language.
				for _, raw := range []string{"source changed after startup", "until restart"} {
					if strings.Contains(page, raw) {
						t.Errorf("%s in %s prints the diagnostic constant (%q)", surface.name, lang, raw)
					}
				}
				if strings.HasPrefix(surface.target, "/syllabus/") || strings.HasPrefix(surface.target, "/listen/") {
					// The course exists; yomihon has stopped using what names
					// it. Neither a 404 nor its sentence about a mistyped
					// address is true of that.
					if code == http.StatusNotFound {
						t.Errorf("%s in %s answers 404 for a course that was there a moment ago", surface.name, lang)
					}
					if strings.Contains(page, `data-nothing="notfound"`) || strings.Contains(page, wording.NotFoundLede.In(lang)) {
						t.Errorf("%s in %s blames the address the reader typed", surface.name, lang)
					}
					if code != http.StatusServiceUnavailable {
						t.Errorf("%s in %s answered %d, want %d: the instance is refusing until restart", surface.name, lang, code, http.StatusServiceUnavailable)
					}
				}
			})
		}
	}
}
