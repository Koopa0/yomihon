package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeOneNote lays down a single note and serves the folder holding it.
func writeOneNote(t *testing.T, rel, body string) string {
	t.Helper()
	root := t.TempDir()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return root
}

// TestHealthIsNotCleanWhileTheSchemaHasSomethingToSay is the claim the page
// makes about itself. It prints one sentence saying the folder has nothing to
// answer for, and that sentence is decided by a fixed list of findings — so a
// finding added to the page without being added to that list produces a page
// that lists faults and says all is well in the same breath.
func TestHealthIsNotCleanWhileTheSchemaHasSomethingToSay(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		rel         string
		body        string
		wantSection string
	}{
		{
			name:        "frontmatter that cannot be read",
			rel:         "Concepts/golang/Broken.md",
			body:        "---\ntitle: Broken\ntype: concept\ndomain: golang\nstatus: draft\nstatus: ready\n---\n\nbody\n",
			wantSection: "frontmatter 讀不出來的筆記",
		},
		{
			name:        "frontmatter that reads and the schema rejects",
			rel:         "Concepts/golang/Bad.md",
			body:        "---\ntitle: Bad\ntype: memorandum\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n",
			wantSection: "schema 有話說的筆記",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := newServerWithContract(t, writeOneNote(t, tc.rel, tc.body), loadHomeContract(t))

			code, body := get(t, srv.Client(), srv.URL+"/health")
			if code != http.StatusOK {
				t.Fatalf("health status = %d, want 200", code)
			}
			if !strings.Contains(body, tc.wantSection) {
				t.Errorf("the health page carries no %q section", tc.wantSection)
			}
			if strings.Contains(body, "沒有需要回答的問題") {
				t.Error("the page lists a finding and says the folder has nothing to answer for, in the same breath")
			}
		})
	}
}

// TestHealthStillSaysAllClearForAFolderWithNothingToAnswer is the other side:
// adding findings to the page must not make a clean folder read as a faulty
// one.
func TestHealthStillSaysAllClearForAFolderWithNothingToAnswer(t *testing.T) {
	t.Parallel()

	const body = "---\ntitle: Fine\ntype: concept\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[Fine]]\"\n---\n\nbody\n"
	srv := newServerWithContract(t, writeOneNote(t, "Concepts/golang/Fine.md", body), loadHomeContract(t))

	code, page := get(t, srv.Client(), srv.URL+"/health")
	if code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", code)
	}
	for _, unwanted := range []string{"frontmatter 讀不出來的筆記", "schema 有話說的筆記"} {
		if strings.Contains(page, unwanted) {
			t.Errorf("the page carries the %q section for a folder with nothing to answer for", unwanted)
		}
	}
}
