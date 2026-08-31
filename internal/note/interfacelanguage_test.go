package note_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestOnePageSpeaksOneLanguage holds together the three places a rendered page
// carries the reader's language: the chrome around it, the view of the note
// itself, and the rail beside both. Each is set separately where the response
// is assembled, and a page that set two of the three would render an English
// frame around Chinese words with nothing failing anywhere — the kind of fault
// that ships because every part of it works.
func TestOnePageSpeaksOneLanguage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const templateRel = "System/templates/Loud lesson.md"
	writeLoudLessonFixture(t, root, templateRel)

	srv := newServerWithContract(t, root, loadContract(t))
	page := getInLanguage(t, srv.URL+"/notes/System/templates/Loud%20lesson.md", wording.En)

	if !strings.Contains(page, `<html lang="en"`) {
		t.Fatalf("the page does not declare English; the cookie never reached the chrome")
	}
	for _, tt := range []struct {
		carrier string
		phrase  wording.Phrase
	}{
		{carrier: "the chrome", phrase: wording.SearchNotes},
		{carrier: "the note's own view", phrase: wording.RawFile},
		{carrier: "the navigation rail", phrase: wording.FilterNavigation},
	} {
		if want := tt.phrase.In(wording.En); !strings.Contains(page, want) {
			t.Errorf("%s did not receive the language: want %q in the page", tt.carrier, want)
		}
		if stale := tt.phrase.In(wording.ZhHant); strings.Contains(page, stale) {
			t.Errorf("%s still carries the default language: %q is in the page", tt.carrier, stale)
		}
	}
}

// getInLanguage fetches a page as a reader who has chosen one, which is the
// only way the wiring under test is exercised at all.
func getInLanguage(t *testing.T, urlStr string, lang wording.Lang) string {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, urlStr, http.NoBody)
	if err != nil {
		t.Fatalf("new request %s: %v", urlStr, err)
	}
	// Set as a header rather than built as a cookie value: only the name and
	// the value cross the wire on a request, and the attributes a cookie type
	// carries belong to the server that sets one.
	req.Header.Set("Cookie", wording.CookieName+"="+string(lang))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", urlStr, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close response body: %v", closeErr)
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s: %v", urlStr, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", urlStr, resp.StatusCode)
	}
	return string(body)
}
