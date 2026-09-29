package note_test

import (
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
)

func TestThoughtRouteOffersOrdinaryDocumentsAndLessonsTheSameDoor(t *testing.T) {
	t.Parallel()
	for _, noteType := range []string{"", "lesson"} {
		t.Run(noteType, func(t *testing.T) {
			t.Parallel()
			mux, _, root := thoughtRouteFixture(t, true, noteType)
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/thought/Source.md?section=chapter-one", http.NoBody))
			if recorder.Code != http.StatusOK {
				t.Fatalf("thought route status = %d, body %s", recorder.Code, recorder.Body.String())
			}
			markdown := thoughtTextarea(t, recorder.Body.String())
			const want = "---\ntype: \"writing\"\nstatus: \"draft\"\ndomain: \"japanese\"\nbased_on: \"[[Source#chapter-one]]\"\n---\n"
			if markdown != want {
				t.Errorf("offered Markdown = %q, want %q", markdown, want)
			}
			if _, err := os.Stat(filepath.Join(root, "Source-thought.md")); !os.IsNotExist(err) {
				t.Errorf("opening the door created a thought file: %v", err)
			}
			if strings.Contains(recorder.Body.String(), `<img src=x onerror=alert(1)>`) {
				t.Error("source title reached the page as executable HTML")
			}
		})
	}
}

func TestThoughtRouteRejectsUnavailableAuthorityAndInvalidSections(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		answer  bool
		revoke  bool
		request string
	}{
		{name: "absent role", request: "/thought/Source.md"},
		{name: "unknown section", answer: true, request: "/thought/Source.md?section=not-a-section"},
		{name: "revoked role", answer: true, revoke: true, request: "/thought/Source.md"},
		{name: "missing file", answer: true, request: "/thought/Missing.md"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mux, contractPath, _ := thoughtRouteFixture(t, tc.answer, "")
			if tc.revoke {
				data, err := os.ReadFile(contractPath)
				if err != nil {
					t.Fatal(err)
				}
				updated := strings.Replace(string(data), "answer_type = \"writing\"\n", "", 1)
				if updated == string(data) {
					t.Fatal("revocation did not remove the declared role")
				}
				if err := os.WriteFile(contractPath, []byte(updated), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tc.request, http.NoBody))
			if recorder.Code != http.StatusNotFound {
				t.Errorf("thought route status = %d, want 404", recorder.Code)
			}
			if strings.Contains(recorder.Body.String(), "data-thought-markdown") {
				t.Error("unavailable thought door still offered Markdown")
			}
		})
	}
}

func thoughtRouteFixture(t *testing.T, declareAnswer bool, sourceType string) (http.Handler, string, string) {
	t.Helper()
	root := t.TempDir()
	content := "---\ntitle: '<img src=x onerror=alert(1)>'\ndomain: japanese\n"
	if sourceType != "" {
		content += "type: " + sourceType + "\n"
	}
	content += "---\n# Chapter one\nPROMPT AND EXPLANATION MUST NOT BE COPIED\n"
	if err := os.WriteFile(filepath.Join(root, "Source.md"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if declareAnswer {
		text = strings.Replace(text, "[navigation]\n", "[navigation]\nanswer_type = \"writing\"\n", 1)
	}
	contractPath := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err := os.WriteFile(contractPath, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	contract, err := schema.LoadFile(contractPath)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.DiscardHandler)
	store, source := newSnapshotStore(t, root, log, contract, contract.Governance())
	writer := openStatusWriter(t, source, contract, contract.Governance())
	handler := note.New(&note.Sources{
		Source: source, Contract: contract, Snapshot: store.Current, Status: writer.Authority,
		ObservedStatus: writer.ObservedStatus, ConsumeReceipt: writer.ConsumeReceipt,
		Continuation: noMark, Log: log,
	})
	mux := http.NewServeMux()
	handler.Register(mux)
	return mux, contractPath, root
}

func thoughtTextarea(t *testing.T, body string) string {
	t.Helper()
	_, after, ok := strings.Cut(body, "<textarea")
	if !ok {
		t.Fatal("thought page has no copyable Markdown textarea")
	}
	_, after, ok = strings.Cut(after, ">")
	if !ok {
		t.Fatal("thought textarea opening tag is incomplete")
	}
	content, _, ok := strings.Cut(after, "</textarea>")
	if !ok {
		t.Fatal("thought textarea closing tag is missing")
	}
	return html.UnescapeString(content)
}
