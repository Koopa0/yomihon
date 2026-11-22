package note_test

import (
	"html"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/schema"
)

// sectionLine reads, for every section the page draws, the line of the file
// its wrapper says the heading opens on, or "" where it says none.
var sectionLine = regexp.MustCompile(`<div class="y-sect"(?: data-source-line="([0-9]+)")?><h[1-6] data-mark-anchor id="([^"]+)"`)

// TestEveryNoteOffersItsPathAndLines holds the way a note is handed to the
// reader's own editor in a folder Obsidian never opened: the page names the
// note's full path, and each section the line of the file its heading opens
// on, counted with the frontmatter, past a comment the page hides, and twice
// over for two headings with the same words. A heading in a callout is a
// section with no line, since the file cannot vouch for one. Every line is
// counted by hand from the file below.
func TestEveryNoteOffersItsPathAndLines(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "courses", "go")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	const source = "---\n" + // 1
		"type: note\n" + // 2
		"---\n" + // 3
		"# A\n" + // 4
		"\n" + // 5
		"## Two\n" + // 6
		"\n" + // 7
		"Words.\n" + // 8
		"\n" + // 9
		"%%\n" + // 10
		"## Hidden\n" + // 11
		"%%\n" + // 12
		"\n" + // 13
		"## Two\n" + // 14
		"\n" + // 15
		"> [!note]\n" + // 16
		"> ## In a callout\n" + // 17
		"\n" + // 18
		"## After\n" + // 19
		"\n" + // 20
		"End.\n" // 21
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	log := slog.New(slog.DiscardHandler)
	store, reader := newSnapshotStore(t, root, log, nil, schema.Ungoverned())
	handler := note.New(&note.Sources{
		Source: reader, Snapshot: store.Current,
		Authority:       func() schema.Authority { return schema.CaptureAuthority(nil, schema.Ungoverned()) },
		Bookmark:        noMark,
		QuestionAddress: mark.Paths.Question,
		Log:             log,
	})
	routes := http.NewServeMux()
	handler.Register(routes)
	recorded := serve(t, routes, http.MethodGet, "/notes/courses/go/a.md", "")
	if recorded.Code != http.StatusOK {
		t.Fatalf("GET the note = %d, want 200", recorded.Code)
	}
	page := recorded.Body.String()

	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(resolved, "courses", "go", "a.md")
	if want := `data-path="` + html.EscapeString(full) + `"`; !strings.Contains(page, want) {
		t.Errorf("the page does not offer the note's full path %q", full)
	}
	if strings.Contains(page, "obsidian://") {
		t.Error("a folder Obsidian never opened was offered a link into Obsidian")
	}

	got := map[string]string{}
	for _, m := range sectionLine.FindAllStringSubmatch(page, -1) {
		got[m[2]] = m[1]
	}
	want := map[string]string{"two": "6", "two-2": "14", "in-a-callout": "", "after": "19"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("the lines the sections name (-want +got):\n%s", diff)
	}
}
