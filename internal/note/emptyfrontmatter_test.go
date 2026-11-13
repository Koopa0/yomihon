package note_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestEmptyFrontmatterBlockIsToldApartFromAnAbsentBlock is the lock for the
// two source shapes the judge keeps apart: an empty fence pair is a
// frontmatter block that declares nothing, so its required fields are missing,
// while a file with no delimiters declared no block at all and is asked for
// none of them.
func TestEmptyFrontmatterBlockIsToldApartFromAnAbsentBlock(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := filepath.Join(root, "Writing")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	empty := []byte("---\n---\n# Empty frontmatter\n\nReadable body.\n")
	absent := []byte("# Absent frontmatter\n\nReadable body.\n")
	if err := os.WriteFile(filepath.Join(dir, "Empty.md"), empty, 0o600); err != nil {
		t.Fatalf("write empty: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Absent.md"), absent, 0o600); err != nil {
		t.Fatalf("write absent: %v", err)
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))

	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		t.Run(string(lang), func(t *testing.T) {
			t.Parallel()
			emptyPage := getInLanguage(t, srv.Client(), srv.URL+"/notes/Writing/Empty.md", lang)
			absentPage := getInLanguage(t, srv.Client(), srv.URL+"/notes/Writing/Absent.md", lang)

			required := requiredFieldSaid(lang, "type")
			if !strings.Contains(emptyPage, required) {
				t.Errorf("empty block is missing the required-field finding; want %q", required)
			}
			if strings.Contains(absentPage, required) {
				t.Errorf("absent block carries a required-field finding; page carries %q", required)
			}
		})
	}
}

func requiredFieldSaid(lang wording.Lang, field string) string {
	var b strings.Builder
	for _, part := range wording.SchemaSentence(lang, "schema.required", field, "") {
		if part.Code {
			b.WriteString("<code>")
			b.WriteString(part.Text)
			b.WriteString("</code>")
		} else {
			b.WriteString(part.Text)
		}
	}
	return b.String()
}
