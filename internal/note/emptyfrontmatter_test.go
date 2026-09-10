package note_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestEmptyFrontmatterBlockIsToldApartFromAnAbsentBlock is the lock for the
// two source shapes the judge already keeps apart and the reading face used
// to collapse: an empty fence pair is a frontmatter block with no status, and
// a file with no delimiters is told that the block is absent. Required-field
// findings follow the empty shape only. Neither face offers a write.
func TestEmptyFrontmatterBlockIsToldApartFromAnAbsentBlock(t *testing.T) {
	t.Parallel()

	if wording.NoFrontmatter.In(wording.En) == wording.StatusUnreadable.In(wording.En) ||
		wording.NoFrontmatter.In(wording.ZhHant) == wording.StatusUnreadable.In(wording.ZhHant) {
		t.Fatal("the absent-block sentence and the no-status sentence are the same in at least one language")
	}

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

			noStatus := wording.StatusUnreadable.In(lang)
			noBlock := wording.NoFrontmatter.In(lang)
			required := requiredFieldSaid(lang, "title")

			if !strings.Contains(emptyPage, noStatus) {
				t.Errorf("empty block is not told it has no status; want %q", noStatus)
			}
			if strings.Contains(emptyPage, noBlock) {
				t.Errorf("empty block is told the block is absent; page carries %q", noBlock)
			}
			if !strings.Contains(emptyPage, required) {
				t.Errorf("empty block is missing the required-field finding; want %q", required)
			}
			if strings.Contains(emptyPage, `class="y-statusform"`) {
				t.Error("empty block offered a status write")
			}

			if !strings.Contains(absentPage, noBlock) {
				t.Errorf("absent block is not told the block is absent; want %q", noBlock)
			}
			if strings.Contains(absentPage, noStatus) {
				t.Errorf("absent block is told it has no status; page carries %q", noStatus)
			}
			if strings.Contains(absentPage, required) {
				t.Errorf("absent block carries a required-field finding; page carries %q", required)
			}
			if strings.Contains(absentPage, `class="y-statusform"`) {
				t.Error("absent block offered a status write")
			}
		})
	}
}

func requiredFieldSaid(lang wording.Lang, field string) string {
	var b strings.Builder
	for _, part := range wording.SchemaSentence(lang, "schema.required", field, "", "") {
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
