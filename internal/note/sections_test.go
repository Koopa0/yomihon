package note_test

import (
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
	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/schema"
)

// sectionVault writes a source note with two sections, the first holding a
// question that starts closed, and the thoughts a reader wrote about it: one
// about the whole note and one about the first section, each with a draft's
// opening line and then the reader's own words.
func sectionVault(t *testing.T) (root string, contract *schema.Contract) {
	t.Helper()
	root = t.TempDir()
	files := map[string]string{
		"Source.md": "---\ntype: lesson\n---\n# Source\n\nIntro.\n\n## First\n\nThe question.\n\n> [!question]- What happens?\n> It stops.\n\n## Second\n\nPlain.\n",
		"Thought on whole.md": "---\ntype: writing\ncreated: 2026-11-02\nbased_on: \"[[Source]]\"\n---\n" +
			"I have not grasped the split between the two signals. A second sentence that is not shown.\n",
		"Thought on first.md": "---\ntype: writing\ncreated: 2026-10-31\nbased_on: \"[[Source#First]]\"\n---\n" +
			"\n看答案前，我的想法：\n\nI thought cancel() already stopped it. Then I read on.\n",
	}
	for rel, content := range files {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err = os.WriteFile(path, []byte(writingHoldsTheThoughtRole(t, string(data))), 0o600); err != nil { // #nosec G703 -- a temporary file this test just named
		t.Fatal(err)
	}
	contract, err = schema.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return root, contract
}

// readerSession is the server a reader has open over the vault and the marks
// file beside it. A second session over the same two is the reader coming back
// another day: nothing is carried in memory from the first.
func readerSession(t *testing.T, root string, contract *schema.Contract, configDir string, kept func() (mark.Continuation, bool)) (mux http.Handler, marks *mark.File) {
	t.Helper()
	log := slog.New(slog.DiscardHandler)
	store, source := newSnapshotStore(t, root, log, contract, contract.Governance())
	var err error
	marks, err = mark.New(configDir, source.Name())
	if err != nil {
		t.Fatal(err)
	}
	if kept == nil {
		kept = noMark
	}
	handler := note.New(&note.Sources{
		Source: source, Contract: contract, Snapshot: store.Current,
		Authority:    func() schema.Authority { return schema.CaptureAuthority(contract, contract.Governance()) },
		Continuation: kept, Uncertainties: marks.Uncertainties,
		UncertaintyAddress: mark.UncertaintyAddress, Log: log,
	})
	routes := http.NewServeMux()
	handler.Register(routes)
	mark.NewUncertaintyHandler(marks, handler, log).Register(routes)
	return routes, marks
}

func serve(t *testing.T, mux http.Handler, method, target, form string) *httptest.ResponseRecorder {
	t.Helper()
	body := http.NoBody
	var reader *strings.Reader
	if form != "" {
		reader = strings.NewReader(form)
	}
	var request *http.Request
	if reader != nil {
		request = httptest.NewRequestWithContext(t.Context(), method, target, reader)
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		request = httptest.NewRequestWithContext(t.Context(), method, target, body)
	}
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	return recorder
}

// TestAReaderWhoLeftAnUncertaintyFindsTheSectionAndTheirThoughtTheNextDay holds
// the third acceptance scenario against the real routes: a reader marks a
// section "Not sure yet" and writes a thought under it; the next day, in a
// session that remembers nothing, the section's own page shows the mark and its
// day, the thought's first sentence and its day, and the draft they would have
// opened it with opens on the line that says it was written before the answer.
// The reader is asked for nothing in any of it.
func TestAReaderWhoLeftAnUncertaintyFindsTheSectionAndTheirThoughtTheNextDay(t *testing.T) {
	t.Parallel()
	root, contract := sectionVault(t)
	configDir := t.TempDir()

	first, _ := readerSession(t, root, contract, configDir, nil)
	recorded := serve(t, first, http.MethodPost, "/uncertainties", url.Values{"path": {"Source.md"}, "anchor": {"first"}}.Encode())
	if recorded.Code != http.StatusOK || !strings.Contains(recorded.Body.String(), `"marked":true`) {
		t.Fatalf("marking the section: status %d, body %s", recorded.Code, recorded.Body.String())
	}

	nextDay, marks := readerSession(t, root, contract, configDir, nil)
	held, err := marks.Uncertainties()
	if err != nil || len(held) != 1 {
		t.Fatalf("the marks the second session reads = %v, %v; want the one the first kept", held, err)
	}
	day := held[0].At.Format(time.DateOnly)

	page := serve(t, nextDay, http.MethodGet, "/notes/Source.md", "").Body.String()
	section := page[strings.Index(page, `<h3 data-mark-anchor id="first"`):strings.Index(page, `<h3 data-mark-anchor id="second"`)]
	for _, want := range []string{
		// The mark and its day: pressed, named as ever, the day shown.
		`data-section-anchor="first" aria-pressed="true" aria-label="還不確定" data-text="還不確定" disabled></button>`,
		`<time class="y-margin__date" data-section-date datetime="` + day + `"></time>`,
		// The thought written about this section: its first sentence, not the
		// line the draft opened with, and the day it was begun.
		`<span class="y-margin__tag" lang="zh-Hant" data-text="想法"></span> <time datetime="2026-10-31"></time> <a class="y-margin__opening" href="/notes/Thought%20on%20first.md" aria-label="I thought cancel() already stopped it." data-text="I thought cancel() already stopped it."></a>`,
		// The prompt above the question, and the link that would write one.
		`看答案前，可以先試著回答 <a href="/thought/Source.md?section=first">寫成想法</a></p><details class="callout callout-note" data-callout="question">`,
	} {
		if !strings.Contains(section, want) {
			t.Errorf("the first section does not carry %q\n%s", want, section)
		}
	}
	if strings.Contains(section, "A second sentence") {
		t.Errorf("more than the first sentence of a thought reached the margin")
	}
	// Another section was not marked, and holds none of it.
	_, rest, _ := strings.Cut(page, `<h3 data-mark-anchor id="second"`)
	if !strings.Contains(rest, `data-section-anchor="second" aria-pressed="false" aria-label="還不確定" data-text="還不確定" disabled></button>`) || strings.Contains(rest, `datetime="`+day+`"`) {
		t.Errorf("the unmarked section carries the mark of the first")
	}
	// The thought about the whole note stands beside the title.
	title := page[strings.Index(page, `<h1 id="source"`):strings.Index(page, `<h3 data-mark-anchor id="first"`)]
	if !strings.Contains(title, `<time datetime="2026-11-02"></time> <a class="y-margin__opening" href="/notes/Thought%20on%20whole.md" aria-label="I have not grasped the split between the two signals." data-text="I have not grasped the split between the two signals."></a>`) {
		t.Errorf("the thought about the whole note is not beside the title:\n%s", title)
	}
}

// TestTheThoughtDraftOpensOnTheLineBeforeTheAnswerOnlyForASectionHoldingAQuestion
// holds the draft the link from the prompt opens: where the source section holds
// a question that starts closed, the draft carries the line saying what is
// written under it; a section without one, and the note as a whole, do not.
func TestTheThoughtDraftOpensOnTheLineBeforeTheAnswerOnlyForASectionHoldingAQuestion(t *testing.T) {
	t.Parallel()
	root, contract := sectionVault(t)
	mux, _ := readerSession(t, root, contract, t.TempDir(), nil)

	for _, tc := range []struct {
		name, target, wantLine string
	}{
		{"a section with a closed question", "/thought/Source.md?section=first", "看答案前，我的想法："},
		{"a section without one", "/thought/Source.md?section=second", ""},
		{"the note as a whole", "/thought/Source.md", ""},
		{"the title's own anchor", "/thought/Source.md?section=source", ""},
	} {
		page := serve(t, mux, http.MethodGet, tc.target, "").Body.String()
		markdown := thoughtTemplate(t, page)
		if got := strings.Contains(markdown, "看答案前，我的想法："); got != (tc.wantLine != "") {
			t.Errorf("%s: the draft carries the before-the-answer line = %v, want %v:\n%s", tc.name, got, tc.wantLine != "", markdown)
		}
		if tc.wantLine != "" && !strings.HasSuffix(markdown, "---\n\n看答案前，我的想法：\n") {
			t.Errorf("%s: the line does not follow the frontmatter on a line of its own: %q", tc.name, markdown)
		}
	}
}

// TestTheKeptPlaceIsDrawnInTheSectionThatHoldsIt holds the place the reader left
// off at, which names any anchor and not only a heading: the margin of the section
// holding that anchor draws it, and no other margin does.
func TestTheKeptPlaceIsDrawnInTheSectionThatHoldsIt(t *testing.T) {
	t.Parallel()
	root, contract := sectionVault(t)
	kept := func() (mark.Continuation, bool) {
		return mark.Continuation{RelPath: "Source.md", Anchor: "first"}, true
	}
	mux, _ := readerSession(t, root, contract, t.TempDir(), kept)

	page := serve(t, mux, http.MethodGet, "/notes/Source.md", "").Body.String()
	if got := strings.Count(page, `data-section-place data-text="你留下待續位置的地方"></p>`); got != 1 {
		t.Fatalf("the place is drawn %d times, want once", got)
	}
	first := page[strings.Index(page, `<h3 data-mark-anchor id="first"`):strings.Index(page, `<h3 data-mark-anchor id="second"`)]
	if !strings.Contains(first, `data-section-place data-text="你留下待續位置的地方"></p>`) {
		t.Errorf("the place is not in the section that holds it:\n%s", first)
	}
}

// TestASectionsMarkIsAnAnchorThePageRenders holds that the control offers no
// mark the server would refuse: every anchor a section's button names is one the
// route admits.
func TestASectionsMarkIsAnAnchorThePageRenders(t *testing.T) {
	t.Parallel()
	root, contract := sectionVault(t)
	mux, _ := readerSession(t, root, contract, t.TempDir(), nil)

	page := serve(t, mux, http.MethodGet, "/notes/Source.md", "").Body.String()
	anchors := 0
	for _, part := range strings.Split(page, `data-section-anchor="`)[1:] {
		anchor, _, _ := strings.Cut(part, `"`)
		anchors++
		got := serve(t, mux, http.MethodPost, "/uncertainties", url.Values{"path": {"Source.md"}, "anchor": {anchor}}.Encode())
		if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"marked":true`) {
			t.Errorf("the button for %q names an anchor the route refuses: status %d, %s", anchor, got.Code, got.Body.String())
		}
	}
	if anchors != 2 {
		t.Errorf("the page offers %d section buttons, want one for each of the two sections", anchors)
	}
}
