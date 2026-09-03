//go:build windows || yomihon_nodurable

package main

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestUnsupportedPlatformKeepsReaderOpenAndStatusWritesClosed(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeRecoverySiteFixture(t, root)
	site, err := newReadingSite(t.Context(), root, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newReadingSite: %v", err)
	}
	t.Cleanup(func() {
		if err := site.close(); err != nil {
			t.Errorf("readingSite.close() error = %v", err)
		}
	})
	notePath := filepath.Join(root, "Maps", "study.md")
	before, err := os.ReadFile(notePath) // #nosec G304 -- fixed fixture path under t.TempDir
	if err != nil {
		t.Fatalf("ReadFile(%q) before requests = %v", notePath, err)
	}

	read := httptest.NewRecorder()
	site.ServeHTTP(read, siteRequest(t, http.MethodGet, "/notes/Maps/study.md", nil))
	readResult := read.Result()
	readBody := responseBody(t, readResult)
	if readResult.StatusCode != http.StatusOK {
		t.Fatalf("GET /notes/Maps/study.md = %d, want %d", readResult.StatusCode, http.StatusOK)
	}
	for _, want := range []string{"Study Path", wording.DurabilityUnsupported.In(wording.ZhHant)} {
		if !strings.Contains(readBody, want) {
			t.Errorf("reading page is missing %q; body = %q", want, readBody)
		}
	}
	if strings.Contains(readBody, `action="/status"`) {
		t.Errorf("reading page offers a status POST on an unsupported platform; body = %q", readBody)
	}

	search := httptest.NewRecorder()
	site.ServeHTTP(search, siteRequest(t, http.MethodGet, "/search?q=Study", nil))
	searchResult := search.Result()
	searchBody := responseBody(t, searchResult)
	if searchResult.StatusCode != http.StatusOK {
		t.Errorf("GET /search?q=Study = %d, want %d", searchResult.StatusCode, http.StatusOK)
	}
	if !strings.Contains(searchBody, "Study Path") {
		t.Errorf("lexical search lost the fixture note; body = %q", searchBody)
	}

	form := url.Values{
		"path":             {"Maps/study.md"},
		"from":             {"draft"},
		"to":               {schema.SealStatus},
		"content_identity": {strings.Repeat("0", 64)},
	}
	write := httptest.NewRecorder()
	req := siteRequest(t, http.MethodPost, "/status", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	site.ServeHTTP(write, req)
	writeResult := write.Result()
	writeBody := responseBody(t, writeResult)
	if writeResult.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("POST /status = %d, want %d", writeResult.StatusCode, http.StatusServiceUnavailable)
	}
	if location := writeResult.Header.Get("Location"); location != "" {
		t.Errorf("POST /status Location = %q, want empty", location)
	}
	for _, want := range []string{wording.DurabilityUnsupported.In(wording.ZhHant), "這次操作沒有變更筆記檔案"} {
		if !strings.Contains(writeBody, want) {
			t.Errorf("status refusal is missing %q; body = %q", want, writeBody)
		}
	}

	after, err := os.ReadFile(notePath) // #nosec G304 -- fixed fixture path under t.TempDir
	if err != nil {
		t.Fatalf("ReadFile(%q) after requests = %v", notePath, err)
	}
	if !bytes.Equal(after, before) {
		t.Errorf("status refusal changed the note\n before: %q\n  after: %q", before, after)
	}
}

func responseBody(t *testing.T, response *http.Response) string {
	t.Helper()
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Errorf("close response body: %v", err)
		}
	}()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return string(body)
}
