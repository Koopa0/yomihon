package lesson_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/kurodo/internal/lesson"
)

// writeConcept writes a concept note under <root>/Concepts/japanese/<name>.
func writeConcept(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, "Concepts", "japanese")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir concepts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write concept %s: %v", name, err)
	}
}

func TestBuildConceptIndexKeysByPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeConcept(t, root, "は (主題助詞).md", "---\ntitle: は\n---\n\nThe topic particle.\n")
	writeConcept(t, root, "です.md", "the copula")

	idx, err := lesson.BuildConceptIndex(root)
	if err != nil {
		t.Fatalf("BuildConceptIndex(%q) = %v", root, err)
	}
	// The index is keyed by the vault-relative path — the shape a rendered
	// wikilink href decodes to — so the render post-pass can look it up directly.
	slug, ok := idx.SlugForPath("Concepts/japanese/は (主題助詞).md")
	if !ok {
		t.Fatalf("SlugForPath(は ...) not found; index keyed by something other than the vault path?")
	}
	if slug == "" {
		t.Errorf("concept slug is empty; want a CJK-safe id")
	}
	if _, ok := idx.SlugForPath("Concepts/japanese/nope.md"); ok {
		t.Errorf("SlugForPath reported a non-existent concept as present")
	}
}

func TestBuildConceptIndexMissingDirIsEmpty(t *testing.T) {
	t.Parallel()
	// A vault with no concept directory is legal (fail-open): empty index, no error.
	idx, err := lesson.BuildConceptIndex(t.TempDir())
	if err != nil {
		t.Fatalf("BuildConceptIndex(no concepts dir) = %v, want nil error", err)
	}
	if len(idx) != 0 {
		t.Errorf("index over an absent concept dir = %d entries, want 0", len(idx))
	}
}

func TestLoadConceptRendersBody(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeConcept(t, root, "は.md", "---\ntitle: は (主題助詞)\n---\n\nMarks the topic.\n")
	idx, err := lesson.BuildConceptIndex(root)
	if err != nil {
		t.Fatalf("BuildConceptIndex: %v", err)
	}
	// renderBody stands in for the note renderer; asserted only that its output
	// reaches the doc HTML, not how it renders (that is render's own test).
	renderBody := func(body string) string { return "<rendered>" + body + "</rendered>" }
	doc, ok := lesson.LoadConcept(renderBody, idx, root, "Concepts/japanese/は.md")
	if !ok {
		t.Fatal("LoadConcept(は) not ok; expected the concept to load")
	}
	if doc.Title != "は (主題助詞)" {
		t.Errorf("doc.Title = %q, want the frontmatter title %q", doc.Title, "は (主題助詞)")
	}
	if doc.Slug == "" {
		t.Errorf("doc.Slug is empty; want the index slug")
	}
	if !strings.Contains(doc.HTML, "Marks the topic") {
		t.Errorf("doc.HTML = %q, want the rendered body", doc.HTML)
	}
}

func TestLoadConceptRejectsNonConcept(t *testing.T) {
	t.Parallel()
	idx := lesson.ConceptIndex{}
	if _, ok := lesson.LoadConcept(func(string) string { return "" }, idx, t.TempDir(), "Writing/x.md"); ok {
		t.Error("LoadConcept accepted a path that is not in the concept index")
	}
}
