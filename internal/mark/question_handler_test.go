package mark_test

import (
	"encoding/json"
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
)

// vaultPlaces answers for one vault. A nil map accepts every place.
type vaultPlaces map[[2]string]bool

func (p vaultPlaces) HasPlace(rel, anchor string) bool {
	return p == nil || p[[2]string{rel, anchor}]
}

func newQuestionHandler(t *testing.T) (*mark.File, http.Handler) {
	t.Helper()
	return newQuestionHandlerFor(t, nil)
}

func newQuestionHandlerFor(t *testing.T, places vaultPlaces) (*mark.File, http.Handler) {
	t.Helper()
	file := newFile(t)
	mux := http.NewServeMux()
	mark.NewQuestionHandler(file, places, slog.New(slog.DiscardHandler)).Register(mux)
	return file, mux
}

func questionRequest(t *testing.T, handler http.Handler, method, body, lang string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, mark.QuestionAddress, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{Name: "yomihon_lang", Value: lang, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestQuestionRouteListsAndTogglesWithServerTime(t *testing.T) {
	t.Parallel()
	file, handler := newQuestionHandler(t)
	empty := questionRequest(t, handler, http.MethodGet, "", "en")
	if empty.Code != http.StatusOK || empty.Body.String() != "[]\n" {
		t.Fatalf("empty GET = %d %s", empty.Code, empty.Body.String())
	}
	form := url.Values{
		"path": {"Notes/source.md"}, "anchor": {"one"},
		"time": {"2000-01-01T00:00:00Z"},
	}
	before := time.Now()
	added := questionRequest(t, handler, http.MethodPost, form.Encode(), "en")
	after := time.Now()
	if added.Code != http.StatusOK || added.Body.String() != "{\"marked\":true}\n" {
		t.Fatalf("add = %d %s", added.Code, added.Body.String())
	}
	listed := questionRequest(t, handler, http.MethodGet, "", "en")
	if listed.Code != http.StatusOK {
		t.Fatalf("GET = %d %s", listed.Code, listed.Body.String())
	}
	var held []mark.Question
	if err := json.Unmarshal(listed.Body.Bytes(), &held); err != nil || len(held) != 1 {
		t.Fatalf("GET marks = %v, %v", held, err)
	}
	if held[0].RelPath != "Notes/source.md" || held[0].Anchor != "one" {
		t.Fatalf("GET returned wrong location: %+v", held[0])
	}
	if held[0].At.Before(before) || held[0].At.After(after) {
		t.Fatalf("time %s was not assigned during the server request", held[0].At)
	}
	for _, response := range []*httptest.ResponseRecorder{empty, added, listed} {
		result := response.Result()
		if result.Header.Get("Content-Type") != "application/json; charset=utf-8" || result.Header.Get("Cache-Control") != "no-store" {
			t.Errorf("JSON response headers = %v", result.Header)
		}
		if err := result.Body.Close(); err != nil {
			t.Error(err)
		}
	}
	removed := questionRequest(t, handler, http.MethodPost, form.Encode(), "en")
	if removed.Code != http.StatusOK || removed.Body.String() != "{\"marked\":false}\n" {
		t.Fatalf("remove = %d %s", removed.Code, removed.Body.String())
	}
	if marks, err := file.Questions(); err != nil || len(marks) != 0 {
		t.Fatalf("removed mark still stored: %v, %v", marks, err)
	}
}

func TestQuestionRouteRefusesBadFormsAndCapsTheBody(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		body string
		want int
	}{
		{"missing path", "anchor=one", http.StatusUnprocessableEntity},
		{"traversal", "path=../private.md&anchor=one", http.StatusUnprocessableEntity},
		{"invalid anchor", "path=source.md&anchor=one%23two", http.StatusUnprocessableEntity},
		{"invalid utf-8 anchor", "path=source.md&anchor=x%FF", http.StatusUnprocessableEntity},
		{"malformed form", "path=%zz", http.StatusBadRequest},
		{"past body cap", "path=source.md&anchor=one&pad=" + strings.Repeat("x", 4096), http.StatusBadRequest},
		{"inside body cap", "path=source.md&anchor=one&pad=" + strings.Repeat("x", 3000), http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			file, handler := newQuestionHandler(t)
			response := questionRequest(t, handler, http.MethodPost, tt.body, "en")
			if response.Code != tt.want {
				t.Fatalf("POST = %d %s; want %d", response.Code, response.Body.String(), tt.want)
			}
			marks, err := file.Questions()
			if err != nil {
				t.Fatal(err)
			}
			if (len(marks) == 1) != (tt.want == http.StatusOK) {
				t.Fatalf("POST status %d left marks %v", response.Code, marks)
			}
			if tt.want != http.StatusOK && strings.Contains(response.Body.String(), "private.md") {
				t.Fatal("refusal echoes the submitted private path")
			}
		})
	}
}

func TestQuestionRoutePreservesCorruptFileAndReportsFailureInBothLanguages(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		lang string
		want string
	}{
		{"en", "Marks cannot be read right now. The existing file was left unchanged.\n"},
		{"zh-Hant", "目前無法讀取標記，原有檔案未更動。\n"},
	} {
		t.Run(tt.lang, func(t *testing.T) {
			t.Parallel()
			file, handler := newQuestionHandler(t)
			if err := os.MkdirAll(filepath.Dir(file.QuestionPath()), 0o700); err != nil {
				t.Fatal(err)
			}
			const damaged = `[{"path":"private/secret.md"`
			if err := os.WriteFile(file.QuestionPath(), []byte(damaged), 0o600); err != nil {
				t.Fatal(err)
			}
			for _, method := range []string{http.MethodGet, http.MethodPost} {
				response := questionRequest(t, handler, method, "path=source.md&anchor=one", tt.lang)
				if response.Code != http.StatusInternalServerError || response.Body.String() != tt.want {
					t.Errorf("%s damaged file = %d %q; want localized 500", method, response.Code, response.Body.String())
				}
			}
			data, err := os.ReadFile(file.QuestionPath())
			if err != nil || string(data) != damaged {
				t.Fatalf("route replaced corrupt file with %s: %v", data, err)
			}
		})
	}
}

func TestQuestionRouteRejectsOtherWriteMethods(t *testing.T) {
	t.Parallel()
	for _, method := range []string{http.MethodPut, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			file, handler := newQuestionHandler(t)
			response := questionRequest(t, handler, method, "path=source.md&anchor=one", "en")
			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("%s = %d, want 405", method, response.Code)
			}
			if _, err := os.Stat(file.QuestionPath()); !os.IsNotExist(err) {
				t.Fatalf("%s created storage: %v", method, err)
			}
		})
	}
}

func TestQuestionRouteRefusesInvalidUTF8RepeatedlyAndStaysReadable(t *testing.T) {
	t.Parallel()
	_, handler := newQuestionHandler(t)
	for range 2 {
		if response := questionRequest(t, handler, http.MethodPost, "path=source.md&anchor=x%FF", "en"); response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("POST invalid UTF-8 = %d, want 422", response.Code)
		}
	}
	if response := questionRequest(t, handler, http.MethodGet, "", "en"); response.Code != http.StatusOK {
		t.Fatalf("GET after the refusals = %d %s, want 200", response.Code, response.Body.String())
	}
}

func TestQuestionRouteAcceptsOnlyPlacesTheVaultRenders(t *testing.T) {
	t.Parallel()
	places := vaultPlaces{{"Notes/real.md", "one"}: true, {"Notes/real.md", ""}: true}
	file, handler := newQuestionHandlerFor(t, places)
	for _, tt := range []struct {
		body string
		want int
	}{
		{"path=Notes/real.md&anchor=one", http.StatusOK},
		{"path=Notes/real.md", http.StatusOK},
		{"path=Notes/real.md&anchor=missing", http.StatusUnprocessableEntity},
		{"path=Visit+example.com+for+free+prizes.md&anchor=one", http.StatusUnprocessableEntity},
	} {
		if response := questionRequest(t, handler, http.MethodPost, tt.body, "en"); response.Code != tt.want {
			t.Errorf("POST %q = %d, want %d", tt.body, response.Code, tt.want)
		}
	}
	held, err := file.Questions()
	if err != nil || len(held) != 2 {
		t.Fatalf("stored marks = %v, %v; want only the two real places", held, err)
	}
	// A place that has since disappeared can still be cleared.
	delete(places, [2]string{"Notes/real.md", "one"})
	if response := questionRequest(t, handler, http.MethodPost, "path=Notes/real.md&anchor=one", "en"); response.Code != http.StatusOK || response.Body.String() != "{\"marked\":false}\n" {
		t.Fatalf("clearing a vanished place = %d %s", response.Code, response.Body.String())
	}
}
