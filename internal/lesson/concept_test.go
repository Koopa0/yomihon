package lesson_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"

	"github.com/koopa0/kurodo/internal/lesson"
)

// writeConceptIn writes a concept note under <root>/Concepts/<domain>/<name>.
func writeConceptIn(t *testing.T, root, domain, name, body string) {
	t.Helper()
	dir := filepath.Join(root, "Concepts", domain)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir concepts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write concept %s: %v", name, err)
	}
}

// writeConcept writes a concept note under <root>/Concepts/japanese/<name>.
func writeConcept(t *testing.T, root, name, body string) {
	t.Helper()
	writeConceptIn(t, root, "japanese", name, body)
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

// TestBuildConceptIndexScansAllDomains: the index covers every domain subdir,
// not just japanese — a golang or rust lesson's concept links resolve too.
func TestBuildConceptIndexScansAllDomains(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeConceptIn(t, root, "japanese", "は.md", "topic particle")
	writeConceptIn(t, root, "golang", "Go Array.md", "a fixed-size sequence")
	writeConceptIn(t, root, "rust", "Ownership.md", "the borrow checker")

	idx, err := lesson.BuildConceptIndex(root)
	if err != nil {
		t.Fatalf("BuildConceptIndex: %v", err)
	}
	for _, rel := range []string{
		"Concepts/japanese/は.md",
		"Concepts/golang/Go Array.md",
		"Concepts/rust/Ownership.md",
	} {
		if _, ok := idx.SlugForPath(rel); !ok {
			t.Errorf("SlugForPath(%q) not found — a non-japanese concept was not indexed", rel)
		}
	}
}

// TestBuildConceptIndexSlugsAreDomainUnique: the same basename in two domains
// must yield distinct sheet ids, or the dialog would clone the wrong <template>.
func TestBuildConceptIndexSlugsAreDomainUnique(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeConceptIn(t, root, "golang", "Interface.md", "a Go interface")
	writeConceptIn(t, root, "system-design", "Interface.md", "an API boundary")

	idx, err := lesson.BuildConceptIndex(root)
	if err != nil {
		t.Fatalf("BuildConceptIndex: %v", err)
	}
	a, aok := idx.SlugForPath("Concepts/golang/Interface.md")
	b, bok := idx.SlugForPath("Concepts/system-design/Interface.md")
	if !aok || !bok {
		t.Fatalf("both same-named concepts must be indexed; got golang=%v system-design=%v", aok, bok)
	}
	if a == b {
		t.Errorf("same-basename concepts in different domains share sheet id %q — the dialog would open the wrong note", a)
	}
}

// TestBuildConceptIndexKeyIsNFCEvenWhenDiskFilenameIsNFD guards the trigger↔sheet
// join across the Unicode-form boundary: the wikilink resolver hands the lookup an
// NFC path (graph normalizes at walk time), so the index must key by NFC too even
// when macOS stores the concept filename NFD — otherwise the sheet silently fails
// to open. Mirrors graph's TestBuildResolvedPathIsNFCEvenWhenDiskFilenameIsNFD.
func TestBuildConceptIndexKeyIsNFCEvenWhenDiskFilenameIsNFD(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	const composed = "が (主格助詞).md" // が = U+304C; decomposes to か (U+304B) + U+3099
	decomposed := norm.NFD.String(composed)
	if decomposed == composed {
		t.Fatalf("test setup invalid: NFD form of %q did not change", composed)
	}
	dir := filepath.Join(root, "Concepts", "japanese")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, decomposed), []byte("the subject particle"), 0o600); err != nil {
		t.Fatalf("write NFD concept: %v", err)
	}

	idx, err := lesson.BuildConceptIndex(root)
	if err != nil {
		t.Fatalf("BuildConceptIndex: %v", err)
	}
	// Looked up by the NFC path — the shape a wikilink [[が (主格助詞)]] resolves to.
	if _, ok := idx.SlugForPath("Concepts/japanese/" + norm.NFC.String(composed)); !ok {
		t.Errorf("SlugForPath missed an NFD-on-disk concept by its NFC path; index keyed by raw disk bytes?")
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
