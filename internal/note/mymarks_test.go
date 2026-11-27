package note_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const marksNote = "---\ntype: lesson\n---\n# Marked\n\nIntro words here.\n\n## First\n\nThe topic particle marks the topic. A second sentence follows.\n\n- a list item with words\n\n## Second\n\nPlain closing.\n"

// keepMark sends one mark to the routes and answers what came back.
func keepMark(t *testing.T, mux http.Handler, body string) (id, note string) {
	t.Helper()
	got := serveJSON(t, mux, http.MethodPost, "/mark", body)
	if got.Code != http.StatusCreated {
		t.Fatalf("POST /mark %s = %d %s", body, got.Code, got.Body.String())
	}
	var answer struct {
		ID   string `json:"id"`
		Note string `json:"note"`
	}
	if err := json.Unmarshal(got.Body.Bytes(), &answer); err != nil {
		t.Fatal(err)
	}
	return answer.ID, answer.Note
}

func marksPage(t *testing.T) (page func() string, mux http.Handler, root, config string) {
	t.Helper()
	root, contract := sectionVault(t)
	if err := os.WriteFile(filepath.Join(root, "Marked.md"), []byte(marksNote), 0o600); err != nil {
		t.Fatal(err)
	}
	config = t.TempDir()
	mux, _ = readerSession(t, root, contract, config, nil)
	return func() string {
		t.Helper()
		got := serve(t, mux, http.MethodGet, "/notes/Marked.md", "")
		if got.Code != http.StatusOK {
			t.Fatalf("GET the note = %d", got.Code)
		}
		return got.Body.String()
	}, mux, root, config
}

// The page a reader arrives at holds the marks as the answer to making each was
// told to draw them: the passage wrapped in the text, the note after the
// paragraph and not in it, a highlight with none; and the note is the very
// markup the route answered with, so the two cannot be drawn apart.
func TestAPageDrawsTheMarksTheRoutesKeptWithoutAScript(t *testing.T) {
	t.Parallel()
	page, mux, _, _ := marksPage(t)
	hid, hnote := keepMark(t, mux, `{"kind":"highlight","path":"Marked.md","anchor":"first","quote":"The topic particle"}`)
	tid, tnote := keepMark(t, mux, `{"kind":"thought","path":"Marked.md","anchor":"first","quote":"marks the topic","body":"why <b>this</b>?"}`)
	qid, qnote := keepMark(t, mux, `{"kind":"question","path":"Marked.md","anchor":"first","quote":"a list item"}`)
	if hnote != "" || tnote == "" || qnote == "" {
		t.Fatalf("notes of a highlight, a thought and a question = %q, %q, %q; want only the last two", hnote, tnote, qnote)
	}
	body := page()
	for _, want := range []string{
		`<mark class="y-mark" data-mark="` + hid + `" data-kind="highlight" data-state="highlight" tabindex="0" aria-label="劃線：The topic particle">The topic particle</mark>`,
		`data-mark="` + tid + `" data-kind="thought"`,
		`aria-describedby="mn-` + tid + `"`,
		// The paragraph closes before its note, and the list item holds its own at the end.
		`follows.</p>` + tnote,
		qnote + `</li>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the page does not hold %q", want)
		}
	}
	if strings.Contains(body, "<b>this</b>") || !strings.Contains(tnote, "why &lt;b&gt;this&lt;/b&gt;?") {
		t.Errorf("the reader's words were not escaped: %q", tnote)
	}
	if strings.Contains(body, `data-mnote="`+hid+`"`) {
		t.Error("a highlight on a passage was given a note")
	}
	if !strings.Contains(qnote, "「a list item」") && !strings.Contains(qnote, `data-text="「a list item」"`) {
		t.Errorf("a question with no words does not stand for its passage: %q", qnote)
	}
	_ = qid
}

// A mark of a whole section stands after its heading, drawn from attributes, and
// a thought on it is no passage in the text.
func TestAWholeSectionThoughtStandsAfterItsHeading(t *testing.T) {
	t.Parallel()
	page, mux, _, _ := marksPage(t)
	_, note := keepMark(t, mux, `{"kind":"thought","path":"Marked.md","anchor":"first","body":"about all of First"}`)
	body := page()
	first := strings.Index(body, `id="first"`)
	at := strings.Index(body, note)
	if at < 0 || at < first || at > strings.Index(body, `id="second"`) {
		t.Fatalf("the section's note is at %d, want it after the heading at %d and before the next", at, first)
	}
	if !strings.Contains(note, `data-section`) || !strings.Contains(note, `data-text="about all of First"`) || strings.Contains(note, ">about all of First<") {
		t.Errorf("the note of a section = %q, want its words drawn from an attribute", note)
	}
	if strings.Contains(body, `<mark class="y-mark"`) {
		t.Error("a mark of a whole section was drawn on a passage")
	}
}

// A passage the author has since rewritten is not dropped: its note is after its
// section's heading, with the words of the passage it was about and the line
// that says it changed.
func TestAMarkWhosePassageChangedIsKeptAfterItsHeading(t *testing.T) {
	t.Parallel()
	_, mux, root, config := marksPage(t)
	_, note := keepMark(t, mux, `{"kind":"thought","path":"Marked.md","anchor":"first","quote":"A second sentence follows.","before":"marks the topic. ","body":"mine"}`)
	changed := strings.Replace(marksNote, "A second sentence follows.", "Something else entirely now.", 1)
	changed = strings.Replace(changed, "The topic particle marks the topic. ", "Unrelated opening. ", 1)
	if err := os.WriteFile(filepath.Join(root, "Marked.md"), []byte(changed), 0o600); err != nil {
		t.Fatal(err)
	}
	// The reader comes back another day, to a note the author has rewritten.
	_, contract := sectionVault(t)
	again, _ := readerSession(t, root, contract, config, nil)
	body := serve(t, again, http.MethodGet, "/notes/Marked.md", "").Body.String()
	if strings.Contains(body, `<mark class="y-mark"`) {
		t.Fatal("a passage that is gone was drawn on other words")
	}
	if strings.Contains(note, "data-changed") {
		t.Fatalf("the answer for a passage in place says it changed: %q", note)
	}
	if !strings.Contains(body, `data-changed`) || !strings.Contains(body, `data-text="原文已變動"`) {
		t.Errorf("the page does not say the passage changed")
	}
}

// serveJSON sends a JSON body to the routes.
func serveJSON(t *testing.T, mux http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	return recorder
}
