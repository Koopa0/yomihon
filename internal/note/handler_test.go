package note_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/kurodo/internal/note"
	"github.com/koopa0/kurodo/internal/render"
)

func newServer(t *testing.T, root string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	h := note.NewHandler(root, render.New(), slog.New(slog.DiscardHandler))
	h.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, url string) (status int, body string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("new request %s: %v", url, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, string(b)
}

func TestShow(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lesson := "---\ntitle: L00 テスト課\ntype: lesson\nstatus: draft\n---\n\n<ruby>今日<rt>きょう</rt></ruby>は<ruby>晴<rt>は</rt></ruby>れ。\n"
	dir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "L00 テスト課.md"), []byte(lesson), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServer(t, root)

	status, body := get(t, srv.URL+"/notes/Writing/lessons/japanese/L00 テスト課.md")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	for _, want := range []string{
		"L00 テスト課",
		"status: draft",
		"<ruby>今日<rt>きょう</rt></ruby>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page missing %q", want)
		}
	}
}

func TestShowNotFound(t *testing.T) {
	t.Parallel()
	srv := newServer(t, t.TempDir())

	status, _ := get(t, srv.URL+"/notes/nope.md")
	if status != http.StatusNotFound {
		t.Errorf("status = %d, want 404", status)
	}
}
