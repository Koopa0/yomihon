package pages

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/nav"
)

func TestAncestorDirs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		relPath string
		want    []string
	}{
		{"Concepts/golang/Foo.md", []string{"Concepts", "Concepts/golang"}},
		{"Inbox/note.md", []string{"Inbox"}},
		{"a/b/c/d.md", []string{"a", "a/b", "a/b/c"}},
		{"README.md", nil},
	}
	for _, tt := range tests {
		t.Run(tt.relPath, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tt.want, ancestorDirs(tt.relPath)); diff != "" {
				t.Errorf("ancestorDirs(%q) mismatch (-want +got):\n%s", tt.relPath, diff)
			}
		})
	}
}

func TestHereLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		dir  string
		want string
	}{
		{"Concepts/golang", "golang"},
		{"Inbox", "Inbox"},
		{"a/b/c", "c"},
		{"", "Vault root"},
	}
	for _, tt := range tests {
		t.Run(tt.dir, func(t *testing.T) {
			t.Parallel()
			if got := hereLabel(tt.dir); got != tt.want {
				t.Errorf("hereLabel(%q) = %q, want %q", tt.dir, got, tt.want)
			}
		})
	}
}

// buildModel writes a small vault to disk and builds the real nav model from it,
// so the wayfinding test resolves lesson wikilinks and folder structure through
// the production build rather than a hand-assembled model.
func buildModel(t *testing.T) *nav.Model {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		// A study-path that lists L01 twice — once deep under Decode > Bytes, once
		// directly under Review — so the reverse index yields two placements.
		"Maps/Go path.md": "---\ntype: study-path\n---\n" +
			"## decode | Decode | 解碼\n\n" +
			"### bytes | Bytes | 位元\n\n" +
			"- [[L01]]\n- [[L02]]\n\n" +
			"## review | Review | 複習\n\n" +
			"- [[L01]]\n",
		"Writing/lessons/go/L01.md": "---\ntype: lesson\nstatus: draft\n---\nbody\n",
		"Writing/lessons/go/L02.md": "---\ntype: lesson\nstatus: draft\n---\nbody\n",
		// A concept note not listed by any study-path.
		"Concepts/go/C01.md": "---\ntype: concept\n---\nbody\n",
		"Concepts/go/C02.md": "---\ntype: concept\n---\nbody\n",
		// A Sources note with no frontmatter at all (a legal shape).
		"Sources/articles/Other.md": "just prose, no frontmatter\n",
		"Sources/articles/Raw.md":   "raw clipping, no frontmatter\n",
	}
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	idx, err := graph.Build(root)
	if err != nil {
		t.Fatalf("graph.Build: %v", err)
	}
	model, err := nav.Build(root, idx)
	if err != nil {
		t.Fatalf("nav.Build: %v", err)
	}
	return model
}

// TestNewSidebarWayfinding checks the resolved navigation for a note that lives
// in a folder and is listed by a study-path: its siblings surface, the study-path
// and every section on the path to it open, its folder ancestors expand, and it
// is the one entry marked current. A lesson listed by two sections opens both.
func TestNewSidebarWayfinding(t *testing.T) {
	t.Parallel()
	model := buildModel(t)

	const (
		goPath  = "Maps/Go path.md"
		current = "Writing/lessons/go/L01.md"
		sibling = "Writing/lessons/go/L02.md"
	)
	sb := NewSidebar(model, current, nil, 0, false)

	if !sb.syllabusOpen(goPath) {
		t.Errorf("syllabusOpen(%q) = false, want true", goPath)
	}
	for _, chain := range [][]string{{"Decode"}, {"Decode", "Bytes"}, {"Review"}} {
		if !sb.sectionOpen(goPath, chain) {
			t.Errorf("sectionOpen(%q, %v) = false, want true", goPath, chain)
		}
	}
	for _, dir := range []string{"Writing", "Writing/lessons", "Writing/lessons/go"} {
		if !sb.folderOpen(dir) {
			t.Errorf("folderOpen(%q) = false, want true", dir)
		}
	}
	if sb.folderOpen("Concepts") {
		t.Error("folderOpen(\"Concepts\") = true, want false (not an ancestor of the current note)")
	}
	if !sb.folderTreeOpen() {
		t.Error("folderTreeOpen() = false, want true (the current note lives in a folder)")
	}
	if !sb.current(current) {
		t.Errorf("current(%q) = false, want true", current)
	}
	if sb.current(sibling) {
		t.Errorf("current(%q) = true, want false", sibling)
	}
	if sb.HereDir != "Writing/lessons/go" {
		t.Errorf("HereDir = %q, want %q", sb.HereDir, "Writing/lessons/go")
	}
	wantHere := []nav.NoteRef{
		{Name: "L01", RelPath: current},
		{Name: "L02", RelPath: sibling},
	}
	if diff := cmp.Diff(wantHere, sb.Here); diff != "" {
		t.Errorf("Here mismatch (-want +got):\n%s", diff)
	}

	// A note listed only under Decode > Bytes must not open the Review section:
	// the open set is per note, not per study-path.
	sb2 := NewSidebar(model, sibling, nil, 0, false)
	if sb2.sectionOpen(goPath, []string{"Review"}) {
		t.Errorf("sectionOpen(%q, [Review]) = true for a note absent from Review, want false", goPath)
	}
	if !sb2.sectionOpen(goPath, []string{"Decode", "Bytes"}) {
		t.Errorf("sectionOpen(%q, [Decode Bytes]) = false, want true", goPath)
	}
}

// TestNewSidebarNonLessonFixtures checks the other two representative shapes: a
// concept note and a frontmatter-less Sources note. Neither is listed by a
// study-path, so no syllabus branch opens — but siblings still surface and the
// folder ancestors still expand, the same wayfinding a lesson gets.
func TestNewSidebarNonLessonFixtures(t *testing.T) {
	t.Parallel()
	model := buildModel(t)

	tests := []struct {
		name     string
		current  string
		wantDir  string
		wantHere []nav.NoteRef
		wantDirs []string
	}{
		{
			name:    "concept note",
			current: "Concepts/go/C01.md",
			wantDir: "Concepts/go",
			wantHere: []nav.NoteRef{
				{Name: "C01", RelPath: "Concepts/go/C01.md"},
				{Name: "C02", RelPath: "Concepts/go/C02.md"},
			},
			wantDirs: []string{"Concepts", "Concepts/go"},
		},
		{
			name:    "no-frontmatter Sources note",
			current: "Sources/articles/Raw.md",
			wantDir: "Sources/articles",
			wantHere: []nav.NoteRef{
				{Name: "Other", RelPath: "Sources/articles/Other.md"},
				{Name: "Raw", RelPath: "Sources/articles/Raw.md"},
			},
			wantDirs: []string{"Sources", "Sources/articles"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sb := NewSidebar(model, tt.current, nil, 0, false)
			if sb.syllabusOpen("Maps/Go path.md") {
				t.Error("syllabusOpen = true for a note no study-path lists, want false")
			}
			if sb.HereDir != tt.wantDir {
				t.Errorf("HereDir = %q, want %q", sb.HereDir, tt.wantDir)
			}
			if diff := cmp.Diff(tt.wantHere, sb.Here); diff != "" {
				t.Errorf("Here mismatch (-want +got):\n%s", diff)
			}
			for _, dir := range tt.wantDirs {
				if !sb.folderOpen(dir) {
					t.Errorf("folderOpen(%q) = false, want true", dir)
				}
			}
			if !sb.current(tt.current) {
				t.Errorf("current(%q) = false, want true", tt.current)
			}
		})
	}
}

// TestSidebarMarksDisclosureStateForTheScript locks the HTML contract the
// disclosure state owner reads: every sidebar disclosure carries a stable
// data-key, the current note's wayfinding chain also carries data-chain
// (forced open, never persisted), discretionary branches carry no chain
// marker, and the pre-paint restore script and the filter's no-match notice
// ride with the sidebar. Rendered attributes come from a map, so the marker
// pair appears in alphabetical order (data-chain before data-key).
func TestSidebarMarksDisclosureStateForTheScript(t *testing.T) {
	t.Parallel()
	model := buildModel(t)
	sb := NewSidebar(model, "Writing/lessons/go/L01.md", nil, 0, false)

	var buf bytes.Buffer
	if err := sidebar(sb).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()

	for _, want := range []string{
		`data-chain data-key="paths"`,
		`data-chain data-key="syl:Maps/Go path.md"`,
		`data-chain data-key="dir:Writing/lessons/go"`,
		`data-chain data-key="folders"`,
		`data-key="dir:Concepts"`,
		`data-filter-empty`,
		`kurodo.nav`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered sidebar is missing %q", want)
		}
	}
	if strings.Contains(html, `data-chain data-key="dir:Concepts"`) {
		t.Error("dir:Concepts carries data-chain, but it is not an ancestor of the current note")
	}
}

// TestNewSidebarNoCurrentNote is the report-page shape: with no current note,
// every branch stays closed and unmarked, and the "here" block is empty.
func TestNewSidebarNoCurrentNote(t *testing.T) {
	t.Parallel()
	model := buildModel(t)
	sb := NewSidebar(model, "", nil, 0, false)

	if sb.syllabusOpen("Maps/Go path.md") {
		t.Error("syllabusOpen = true with no current note, want false")
	}
	if sb.folderTreeOpen() {
		t.Error("folderTreeOpen = true with no current note, want false")
	}
	if len(sb.Here) != 0 {
		t.Errorf("Here = %v with no current note, want empty", sb.Here)
	}
}
