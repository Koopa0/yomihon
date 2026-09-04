package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A tool for walking between notes gets walked off the end of. Every reader who
// tried this one did it — by truncating a URL to see the folder, by mistyping a
// name, by following a link to a note that was never written — and each landed
// on a bare line of text with nowhere to go, one of them in English from the
// router's own fallback. The page routes now answer with a page.
//
// The split is the part worth locking: /raw/ hands the browser bytes, so an
// HTML apology there would be a different bug wearing this fix's clothes.
func TestMissingPathsAnswerAsPagesAndMissingBytesDoNot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "微積分"), 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "微積分", "極限.md"), []byte("# 極限\n\n本文。\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))

	tests := []struct {
		name     string
		path     string
		wantPage bool
	}{
		{name: "a note that was never written", path: "/notes/沒寫過.md", wantPage: true},
		{name: "a folder, which is what truncating a URL reaches", path: "/notes/微積分", wantPage: true},
		{name: "a path outside every route", path: "/wat", wantPage: true},
		{name: "raw bytes that are not there stay bytes", path: "/raw/沒寫過.md", wantPage: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code, body := get(t, srv.Client(), srv.URL+tt.path)
			if code != http.StatusNotFound {
				t.Fatalf("GET %s status = %d, want %d", tt.path, code, http.StatusNotFound)
			}
			if strings.Contains(body, "404 page not found") {
				t.Errorf("GET %s answered in the router's own English", tt.path)
			}
			// A page is one the reader can leave: the way home and the way to
			// search are what every one of them went looking for and did not find.
			gotPage := strings.Contains(body, `href="/search"`) && strings.Contains(body, `href="/"`)
			if gotPage != tt.wantPage {
				t.Errorf("GET %s returned a page with a way out = %v, want %v\nbody = %q",
					tt.path, gotPage, tt.wantPage, body)
			}
		})
	}
}
