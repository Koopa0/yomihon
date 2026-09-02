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
	got := newReading(parsed, data, schema.ArticleLanguage{}, true)

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
	got := newReading(vault.Parse("Broken.md", data), data, schema.ArticleLanguage{}, true)
	if got.HasFrontmatter {
		t.Fatal("malformed frontmatter was marked authoritative")
	}
	if got.FMDiagnostic == "" {
		t.Fatal("malformed frontmatter diagnostic was dropped")
	}
}
