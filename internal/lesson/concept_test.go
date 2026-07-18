package lesson_test

import (
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"

	"github.com/koopa0/yomihon/internal/lesson"
	"github.com/koopa0/yomihon/internal/vault"
)

func conceptNote(domain, name, body string) *vault.Note {
	return vault.Parse("Concepts/"+domain+"/"+name, []byte(body))
}

func TestNewConceptIndexKeysByPath(t *testing.T) {
	t.Parallel()
	idx, err := lesson.NewConceptIndex([]*vault.Note{
		conceptNote("japanese", "は (主題助詞).md", "---\ntitle: は\n---\n\nThe topic particle.\n"),
		conceptNote("japanese", "です.md", "the copula"),
	})
	if err != nil {
		t.Fatalf("NewConceptIndex() error = %v", err)
	}
	// The index is keyed by the vault-relative path — the shape a rendered
	// wikilink href decodes to — so the render post-pass can look it up directly.
	id, ok := idx.IDForPath("Concepts/japanese/は (主題助詞).md")
	if !ok {
		t.Fatalf("IDForPath(は ...) not found; index keyed by something other than the vault path?")
	}
	if id == "" {
		t.Error("concept ID is empty")
	}
	if _, ok := idx.IDForPath("Concepts/japanese/nope.md"); ok {
		t.Error("IDForPath reported a non-existent concept as present")
	}
}

// TestNewConceptIndexCoversAllDomains: the index covers every domain subdir,
// not just japanese — a golang or rust lesson's concept links resolve too.
func TestNewConceptIndexCoversAllDomains(t *testing.T) {
	t.Parallel()
	idx, err := lesson.NewConceptIndex([]*vault.Note{
		conceptNote("japanese", "は.md", "topic particle"),
		conceptNote("golang", "Go Array.md", "a fixed-size sequence"),
		conceptNote("rust", "Ownership.md", "the borrow checker"),
	})
	if err != nil {
		t.Fatalf("NewConceptIndex() error = %v", err)
	}
	for _, rel := range []string{
		"Concepts/japanese/は.md",
		"Concepts/golang/Go Array.md",
		"Concepts/rust/Ownership.md",
	} {
		if _, ok := idx.IDForPath(rel); !ok {
			t.Errorf("IDForPath(%q) not found — a non-japanese concept was not indexed", rel)
		}
	}
}

// TestNewConceptIndexIDsAreDomainUnique: the same basename in two domains
// must yield distinct sheet ids, or the dialog would clone the wrong <template>.
func TestNewConceptIndexIDsAreDomainUnique(t *testing.T) {
	t.Parallel()
	idx, err := lesson.NewConceptIndex([]*vault.Note{
		conceptNote("golang", "Interface.md", "a Go interface"),
		conceptNote("system-design", "Interface.md", "an API boundary"),
	})
	if err != nil {
		t.Fatalf("NewConceptIndex() error = %v", err)
	}
	a, aok := idx.IDForPath("Concepts/golang/Interface.md")
	b, bok := idx.IDForPath("Concepts/system-design/Interface.md")
	if !aok || !bok {
		t.Fatalf("both same-named concepts must be indexed; got golang=%v system-design=%v", aok, bok)
	}
	if a == b {
		t.Errorf("same-basename concepts in different domains share sheet id %q — the dialog would open the wrong note", a)
	}
}

func TestNewConceptIndexIDsAreInjectiveAcrossPunctuation(t *testing.T) {
	t.Parallel()
	idx, err := lesson.NewConceptIndex([]*vault.Note{
		conceptNote("golang", "A-B.md", "hyphenated concept"),
		conceptNote("golang", "A B.md", "spaced concept"),
	})
	if err != nil {
		t.Fatalf("NewConceptIndex() error = %v", err)
	}
	tests := []struct {
		path string
		want string
	}{
		{path: "Concepts/golang/A-B.md", want: "path-Q29uY2VwdHMvZ29sYW5nL0EtQi5tZA"},
		{path: "Concepts/golang/A B.md", want: "path-Q29uY2VwdHMvZ29sYW5nL0EgQi5tZA"},
	}
	for _, tt := range tests {
		if got, ok := idx.IDForPath(tt.path); !ok || got != tt.want {
			t.Errorf("concept ID for %q = %q, want %q", tt.path, got, tt.want)
		}
	}
	first, _ := idx.IDForPath(tests[0].path)
	second, _ := idx.IDForPath(tests[1].path)
	if first == second {
		t.Errorf("punctuation-distinct paths share concept ID %q", first)
	}
}

// TestNewConceptIndexKeyIsNFCEvenWhenCapturedPathIsNFD guards the trigger-to-sheet
// join across the Unicode-form boundary: the wikilink resolver hands the lookup
// an NFC path, so captured NFD spellings must still key by NFC.
func TestNewConceptIndexKeyIsNFCEvenWhenCapturedPathIsNFD(t *testing.T) {
	t.Parallel()
	const composed = "が (主格助詞).md" // が = U+304C; decomposes to か (U+304B) + U+3099
	decomposed := norm.NFD.String(composed)
	if decomposed == composed {
		t.Fatalf("test setup invalid: NFD form of %q did not change", composed)
	}
	idx, err := lesson.NewConceptIndex([]*vault.Note{
		conceptNote("japanese", decomposed, "the subject particle"),
	})
	if err != nil {
		t.Fatalf("NewConceptIndex() error = %v", err)
	}
	// Looked up by the NFC path — the shape a wikilink [[が (主格助詞)]] resolves to.
	if _, ok := idx.IDForPath("Concepts/japanese/" + norm.NFC.String(composed)); !ok {
		t.Error("IDForPath missed an NFD-captured concept by its NFC path")
	}
	doc, ok := idx.Document(func(body string) string { return body }, "Concepts/japanese/"+norm.NFC.String(composed))
	if !ok || doc.HTML != "the subject particle" {
		t.Errorf("Document(NFC path for NFD input) = (%q, %t), want captured bytes", doc.HTML, ok)
	}
}

func TestNewConceptIndexEmptyInput(t *testing.T) {
	t.Parallel()
	idx, err := lesson.NewConceptIndex(nil)
	if err != nil {
		t.Fatalf("NewConceptIndex(nil) error = %v", err)
	}
	if idx.Len() != 0 {
		t.Errorf("index over an absent concept dir = %d entries, want 0", idx.Len())
	}
}

func TestConceptIndexDocumentRendersBody(t *testing.T) {
	t.Parallel()
	idx, err := lesson.NewConceptIndex([]*vault.Note{
		conceptNote("japanese", "は.md", "---\ntitle: は (主題助詞)\n---\n\nMarks the topic.\n"),
	})
	if err != nil {
		t.Fatalf("NewConceptIndex() error = %v", err)
	}
	// renderBody stands in for the note renderer; asserted only that its output
	// reaches the doc HTML, not how it renders (that is render's own test).
	renderBody := func(body string) string { return "<rendered>" + body + "</rendered>" }
	doc, ok := idx.Document(renderBody, "Concepts/japanese/は.md")
	if !ok {
		t.Fatal("Document(は) not ok; expected the concept to load")
	}
	if doc.Title != "は (主題助詞)" {
		t.Errorf("doc.Title = %q, want the frontmatter title %q", doc.Title, "は (主題助詞)")
	}
	if doc.ID == "" {
		t.Error("doc.ID is empty")
	}
	if !strings.Contains(doc.HTML, "Marks the topic") {
		t.Errorf("doc.HTML = %q, want the rendered body", doc.HTML)
	}
}

func TestConceptIndexDocumentRejectsNonConcept(t *testing.T) {
	t.Parallel()
	idx := lesson.ConceptIndex{}
	if _, ok := idx.Document(func(string) string { return "" }, "Writing/x.md"); ok {
		t.Error("Document accepted a path that is not in the concept index")
	}
}

func TestNewConceptIndexOwnsParsedNote(t *testing.T) {
	t.Parallel()
	const relPath = "Concepts/japanese/は.md"
	note := vault.Parse(relPath, []byte("---\ntitle: は (主題助詞)\n---\n\nMarks the topic.\n"))

	idx, err := lesson.NewConceptIndex([]*vault.Note{note})
	if err != nil {
		t.Fatalf("NewConceptIndex() error = %v", err)
	}

	note.RelPath = "Concepts/japanese/replaced.md"
	note.Body = "replacement body"
	note.Frontmatter["title"] = "replacement title"

	doc, ok := idx.Document(func(body string) string { return "<rendered>" + body + "</rendered>" }, relPath)
	if !ok {
		t.Fatal("Document() did not find the captured concept after caller mutation")
	}
	if doc.Title != "は (主題助詞)" || !strings.Contains(doc.HTML, "Marks the topic") {
		t.Errorf("Document() = %+v, want the captured title and body", doc)
	}
	if _, ok := idx.Document(func(body string) string { return body }, note.RelPath); ok {
		t.Error("Document() found a path installed after construction")
	}
}

func TestNewConceptIndexRejectsInvalidInputs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		notes []*vault.Note
		want  string
	}{
		{
			name:  "nil note",
			notes: []*vault.Note{nil},
			want:  "concept note 0 is nil",
		},
		{
			name: "duplicate canonical path",
			notes: []*vault.Note{
				conceptNote("japanese", "は.md", "first"),
				conceptNote("japanese", "は.md", "second"),
			},
			want: `concept path "Concepts/japanese/は.md" appears more than once`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := lesson.NewConceptIndex(tt.notes)
			if err == nil || err.Error() != tt.want {
				t.Errorf("NewConceptIndex() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestConceptIndexZeroValue(t *testing.T) {
	t.Parallel()
	var idx lesson.ConceptIndex
	if idx.Len() != 0 {
		t.Errorf("zero ConceptIndex.Len() = %d, want 0", idx.Len())
	}
	if _, ok := idx.IDForPath("Concepts/japanese/は.md"); ok {
		t.Error("zero ConceptIndex.IDForPath() reported a concept")
	}
	if _, ok := idx.Document(func(body string) string { return body }, "Concepts/japanese/は.md"); ok {
		t.Error("zero ConceptIndex.Document() reported a concept")
	}
}
