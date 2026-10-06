package status

import (
	"bytes"
	"encoding/hex"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/nav"
)

func readOnlyRecovery(t *testing.T, language string) string {
	t.Helper()
	requirePermissionBits(t)
	root, writer := internalVault(t)
	path := seedInstallNote(t, root)
	setNoteMode(t, path, 0o444)
	before := readOnlyInventory(t, filepath.Dir(path))
	identity := internalLessonIdentity()
	form := url.Values{
		"path":             {installRel},
		"from":             {"draft"},
		"to":               {"ready"},
		"content_identity": {hex.EncodeToString(identity[:])},
	}
	var logs bytes.Buffer
	mux := http.NewServeMux()
	NewHandler(writer, func() nav.Shell { return nav.Shell{} }, slog.New(slog.NewTextHandler(&logs, nil))).Register(mux)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/status", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{Name: "yomihon_lang", Value: language, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	response := recorder.Result()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read recorded recovery response: %v", err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatalf("close recorded recovery response: %v", err)
	}
	got := struct {
		Code        int
		ContentType string
		Cache       string
		Nosniff     string
		Location    string
	}{response.StatusCode, response.Header.Get("Content-Type"), response.Header.Get("Cache-Control"), response.Header.Get("X-Content-Type-Options"), response.Header.Get("Location")}
	want := struct {
		Code        int
		ContentType string
		Cache       string
		Nosniff     string
		Location    string
	}{403, "text/html; charset=utf-8", "no-store", "nosniff", ""}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("caught: read-only committed recovery response (-want +got):\n%s", diff)
	}
	if logs.Len() != 0 {
		t.Errorf("read-only client refusal was logged as an internal failure: %q", logs.String())
	}
	assertReadOnlyInventory(t, filepath.Dir(path), before)
	assertNoReadOnlyReceipt(t, writer)
	return string(data)
}

func TestReadOnlyHandlerRecovery(t *testing.T) {
	t.Parallel()
	tests := []struct {
		language string
		summary  string
		next     string
		state    string
	}{
		{
			language: "zh-Hant",
			summary:  "這篇筆記是唯讀的。",
			next:     "請在 yomihon 外變更這篇筆記的檔案權限，允許擁有者寫入，再重新載入筆記。",
			state:    "這次操作沒有變更筆記檔案。",
		},
		{
			language: "en",
			summary:  "This note is read-only.",
			next:     "Change this note's file permissions outside yomihon to allow its owner to write, then reload the note.",
			state:    "Nothing was written to the note's file.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			t.Parallel()
			body := readOnlyRecovery(t, tt.language)
			for _, text := range []string{`<html lang="` + tt.language + `"`, `data-status-changed="false"`, html.EscapeString(tt.summary), html.EscapeString(tt.next), html.EscapeString(tt.state), `href="/notes/Writing/lessons/japanese/L05.md"`} {
				if !strings.Contains(body, text) {
					t.Errorf("caught: read-only %s recovery lacks %q", tt.language, text)
				}
			}
			if strings.Contains(body, `action="/status"`) {
				t.Error("caught: read-only recovery offers a status retry")
			}
			if strings.Contains(body, `class="y-recovery__detail"`) || strings.Contains(body, "private permission detail") {
				t.Error("read-only recovery exposed technical permission detail")
			}
		})
	}
}
