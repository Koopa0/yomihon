package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The rail's type drawers open to meet the page the reader is on — the journal
// drawer on a journal page, the report drawer on a report, the study-path
// drawer on a note some path has placed — and render closed everywhere else.
// The rail's shape never changes; only which drawer stands open does. Both
// directions are locked per drawer, because a drawer that is always open is
// exactly the state this replaced: the study-path drawer used to greet every
// page, including the reports, fully expanded.
func TestSidebarDrawersOpenForThePageAtHand(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	write("Writing/lessons/golang/Slices.md", "---\ntitle: Slices\ntype: lesson\ndomain: golang\nstatus: draft\n---\n\nbody\n")
	write("Writing/lessons/golang/Maps lesson.md", "---\ntitle: Maps lesson\ntype: lesson\ndomain: golang\nstatus: draft\n---\n\nbody\n")
	write("Maps/Go path.md", "---\ntitle: Go path\ntype: study-path\ndomain: golang\n---\n\n## data | Data | 資料\n\n- [[Slices]]\n- [[Planned lesson]]\n- [[Maps lesson]]\n")
	write("Diary/2026-07-31.md", "today\n")
	write("Concepts/plain.md", "---\ntitle: Plain\ntype: concept\ndomain: golang\nstatus: draft\n---\n\nprose\n")
	write("System/reports/weekly.md", "---\ntitle: Weekly\ntype: system\ndomain: meta\n---\n\nreport\n")
	write("Concepts/linked.md", "---\ntitle: Linked\ntype: concept\ndomain: golang\nstatus: draft\n---\n\nprose\n")
	write("Maps/atlas.md", "---\ntitle: Atlas\ntype: topic-map\ndomain: golang\n---\n\n## Themes\n\n- [[linked]]\n")

	srv := newServerWithContract(t, root, loadHomeContract(t))

	const (
		pathsOpen     = `<details open data-sidebar-group="paths"`
		pathsClosed   = `<details data-sidebar-group="paths"`
		journalOpen   = `<details open data-sidebar-group="journal"`
		journalShut   = `<details data-sidebar-group="journal"`
		reportsOpen   = `<details open data-sidebar-group="reports"`
		reportsClosed = `<details data-sidebar-group="reports"`
		mapsOpen      = `<details open data-sidebar-group="maps"`
		mapsClosed    = `<details data-sidebar-group="maps"`
	)
	tests := []struct {
		name string
		url  string
		want []string
		ban  []string
	}{
		{
			name: "a placed lesson opens the study-path drawer alone",
			url:  "/notes/Writing/lessons/golang/Slices.md",
			// The planned lesson between the two written ones is a warning
			// row, not a stop: the step forward from the first lesson lands
			// on the third, and the first lesson has no step back.
			want: []string{
				pathsOpen, journalShut, reportsClosed, mapsClosed,
				`下一課：Maps lesson →`,
			},
		},
		{
			name: "the closing lesson steps back across the planned row",
			url:  "/notes/Writing/lessons/golang/Maps%20lesson.md",
			want: []string{pathsOpen, `← 上一課：Slices`},
		},
		{
			name: "a journal entry opens the journal drawer alone",
			url:  "/notes/Diary/2026-07-31.md",
			want: []string{journalOpen, pathsClosed, reportsClosed, mapsClosed},
		},
		{
			name: "a report opens the report drawer alone",
			url:  "/notes/System/reports/weekly.md",
			want: []string{reportsOpen, pathsClosed, journalShut, mapsClosed},
		},
		{
			name: "an unplaced note opens none of them",
			url:  "/notes/Concepts/plain.md",
			want: []string{pathsClosed, journalShut, reportsClosed, mapsClosed},
			ban:  []string{"上一課", "下一課"},
		},
		{
			name: "a note some map places opens the map drawer alone",
			url:  "/notes/Concepts/linked.md",
			want: []string{mapsOpen, pathsClosed, journalShut, reportsClosed},
		},
		{
			name: "the map itself opens its own drawer",
			url:  "/notes/Maps/atlas.md",
			want: []string{mapsOpen, `<details open data-map-tree="Maps/atlas.md"`, pathsClosed},
		},
		{
			name: "the study path itself opens its own drawer",
			url:  "/notes/Maps/Go path.md",
			want: []string{pathsOpen, `<details open data-map-tree="Maps/Go path.md"`, mapsClosed},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code, body := get(t, srv.URL+tt.url)
			if code != http.StatusOK {
				t.Fatalf("GET %s status = %d, want 200", tt.url, code)
			}
			for _, want := range tt.want {
				if !strings.Contains(body, want) {
					t.Errorf("GET %s sidebar missing %q", tt.url, want)
				}
			}
			for _, ban := range tt.ban {
				if strings.Contains(body, ban) {
					t.Errorf("GET %s sidebar carries %q, want absent", tt.url, ban)
				}
			}
		})
	}
}
