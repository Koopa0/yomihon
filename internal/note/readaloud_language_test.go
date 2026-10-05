package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

func TestAuthoredReadAloudLanguagesStayWithinLessonInstances(t *testing.T) {
	t.Parallel()
	const paragraphs = "<!-- read-aloud: zh-Hant -->\n中文段落。\n\n<!-- read-aloud: en -->\nEnglish paragraph.\n\n<!-- read-aloud: en_US -->\nInvalid marker prose.\n"
	for _, tt := range []struct {
		name, path, kind string
		governed, speaks bool
	}{
		{"lesson", "Writing/Lesson.md", "lesson", true, true},
		{"concept", "Writing/Concept.md", "concept", true, false},
		{"template", "System/templates/Lesson.md", "lesson", true, false},
		{"ungoverned", "Writing/Lesson.md", "lesson", false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			full := filepath.Join(root, filepath.FromSlash(tt.path))
			if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
				t.Fatal(err)
			}
			body := "---\ntitle: Marked prose\ntype: " + tt.kind + "\ndomain: golang\nstatus: ready\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\n" + paragraphs
			if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			var contract *schema.Contract
			if tt.governed {
				contract = loadHomeContract(t)
			}
			srv := newServerWithContract(t, root, contract)
			code, page := get(t, srv.Client(), srv.URL+"/notes/"+tt.path)
			if code != http.StatusOK {
				t.Fatalf("GET status = %d", code)
			}
			t.Log("invoked: read-aloud lesson instance boundary")
			want := 0
			if tt.speaks {
				want = 2
			}
			if got := strings.Count(page, `data-tts="`); got != want {
				t.Errorf("caught: lesson scope control count = %d want %d", got, want)
			}
			for _, text := range []string{"中文段落。", "English paragraph.", "Invalid marker prose."} {
				if !strings.Contains(page, text) {
					t.Errorf("caught: authored prose lost: %q", text)
				}
			}
			if tt.speaks {
				for _, tag := range []string{"zh-Hant", "en"} {
					if !strings.Contains(page, `<div class="y-reading" lang="`+tag+`">`) {
						t.Errorf("caught: lesson own language lost: %q", tag)
					}
				}
			}
			if strings.Contains(page, "read-aloud: en_US") {
				t.Error("caught: malformed instruction visible")
			}
		})
	}
}
