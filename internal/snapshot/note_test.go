package snapshot

import (
	"crypto/sha256"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

func TestCaptureNoteDetachesPublishedReadingData(t *testing.T) {
	t.Parallel()

	data := []byte("---\ntitle: Original\ntype: concept\nstatus: draft\nslug: original\n---\nbody\n")
	parsed := vault.Parse("Concepts/Original.md", data)
	got := newReading(parsed, data, schema.ArticleLanguage{})

	parsed.RelPath = "Concepts/Changed.md"
	parsed.Body = "changed"
	parsed.Frontmatter["title"] = "Changed"
	parsed.Frontmatter["type"] = "lesson"
	parsed.Frontmatter["status"] = "ready"
	parsed.Frontmatter["slug"] = "changed"
	data[0] = 'x'

	if got.RelPath != "Concepts/Original.md" || got.Title != "Original" || got.Body != "body\n" {
		t.Fatalf("captured note identity/body changed with its inputs: %+v", got)
	}
	if got.Type != "concept" || got.Status != "draft" || got.Slug != "original" {
		t.Fatalf("captured note metadata changed with its inputs: %+v", got)
	}
	if !got.HasFrontmatter || got.Language != "" || got.LanguageDiagnostic != "" {
		t.Fatalf("captured note authority = frontmatter %t language %q diagnostic %q", got.HasFrontmatter, got.Language, got.LanguageDiagnostic)
	}
	// The status value is outside the identity — the page binds the status
	// separately — so the expectation splices it out by hand. The rest of that
	// line stays in: the write does not touch it, so a ruling is bound by it.
	wantIdentity := sha256.Sum256([]byte("---\ntitle: Original\ntype: concept\nstatus: \nslug: original\n---\nbody\n"))
	if got.ContentIdentity != wantIdentity {
		t.Errorf("ContentIdentity = %x, want %x", got.ContentIdentity, wantIdentity)
	}
}

func TestCaptureNoteRetainsFrontmatterDiagnosticWithoutAuthority(t *testing.T) {
	t.Parallel()

	data := []byte("---\ntitle: [broken\n---\nbody\n")
	got := newReading(vault.Parse("Broken.md", data), data, schema.ArticleLanguage{})
	if !got.HasFrontmatter {
		t.Fatal("malformed frontmatter dropped the block that produced the diagnostic")
	}
	if got.FMDiagnostic == "" {
		t.Fatal("malformed frontmatter diagnostic was dropped")
	}
}

// TestCaptureNoteKeepsEmptyBlockApartFromAbsent holds the projection to the
// split's own answer: an empty fence pair is a present block, and a file with
// no delimiters is not. Deriving the flag from a nil field map collapses them.
func TestCaptureNoteKeepsEmptyBlockApartFromAbsent(t *testing.T) {
	t.Parallel()

	empty := []byte("---\n---\nbody\n")
	if got := newReading(vault.Parse("Empty.md", empty), empty, schema.ArticleLanguage{}); !got.HasFrontmatter {
		t.Fatal("empty frontmatter block was projected as absent")
	}

	absent := []byte("body\n")
	if got := newReading(vault.Parse("Absent.md", absent), absent, schema.ArticleLanguage{}); got.HasFrontmatter {
		t.Fatal("a file with no delimiters was projected as a frontmatter block")
	}
}

// TestCaptureNoteKeepsAFenceNothingClosedApartFromAbsent holds the second
// projection of a file the split read no block from. A note whose closing fence
// lost a character and a note that never wrote frontmatter both have no block,
// and a page that cannot tell them apart calls the first one legal. Neither is a
// frontmatter block, and only the first is a fence nothing closed.
func TestCaptureNoteKeepsAFenceNothingClosedApartFromAbsent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		content      string
		wantBlock    bool
		wantUnclosed bool
	}{
		{name: "the closing fence lost a dash", content: "---\ntitle: Unclosed\ntype: note\nstatus: draft\n--\n\n# Body\n", wantUnclosed: true},
		{name: "a block that closes", content: "---\ntitle: Closed\n---\nbody\n", wantBlock: true},
		{name: "an empty fence pair", content: "---\n---\nbody\n", wantBlock: true},
		{name: "a thematic break and prose", content: "---\n\nbody\n"},
		{name: "no delimiters", content: "body\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := []byte(tt.content)
			got := newReading(vault.Parse("Note.md", data), data, schema.ArticleLanguage{})
			if got.HasFrontmatter != tt.wantBlock || got.FrontmatterUnclosed != tt.wantUnclosed {
				t.Errorf("newReading(%q) = block %v, unclosed %v; want block %v, unclosed %v",
					tt.content, got.HasFrontmatter, got.FrontmatterUnclosed, tt.wantBlock, tt.wantUnclosed)
			}
		})
	}
}
