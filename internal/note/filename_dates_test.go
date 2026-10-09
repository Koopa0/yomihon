package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJournalKeepsImpossibleFilenameDatesReachable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, stem := range []string{"2026-13-01", "2026-02-30", "2024-02-29"} {
		path := filepath.Join(root, "Diary", stem+".md")
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("readable journal prose\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))
	for _, month := range []string{"2026-02", "2026-10", "2024-02"} {
		code, body := get(t, srv.Client(), srv.URL+"/journal?month="+month)
		if code != http.StatusOK {
			t.Fatalf("journal %s: status = %d, want 200", month, code)
		}
		_, undated, found := strings.Cut(body, "data-journal-undated>")
		undated, _, _ = strings.Cut(undated, "</section>")
		for _, stem := range []string{"2026-13-01", "2026-02-30"} {
			if !found || !strings.Contains(undated, `href="/notes/Diary/`+stem+`.md"`) {
				t.Errorf("journal %s: impossible filename date %s is not reachable in the undated section", month, stem)
			}
		}
		if month == "2024-02" && !strings.Contains(body, `href="/notes/Diary/2024-02-29.md" data-journal-entry`) {
			t.Error("valid leap day lost its calendar link")
		}
	}
	code, body := get(t, srv.Client(), srv.URL+"/notes/Diary/2026-13-01.md")
	if code != http.StatusOK || !strings.Contains(body, "readable journal prose") {
		t.Error("the impossible filename date made direct note reading unavailable")
	}
}
