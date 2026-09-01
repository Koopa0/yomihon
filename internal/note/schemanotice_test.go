package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNotePageNamesTheFieldAtFault is the claim this surface was built for:
// the page tells a reader what the command already knew, and names the field
// the fault is actually in. A note whose type is not one the schema declares
// used to be answered with a sentence about its status, which was legal —
// pointing a reader at the one field that was fine.
func TestNotePageNamesTheFieldAtFault(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		rel      string
		body     string
		wantSaid []string
		wantNot  []string
	}{
		{
			name:     "a type outside the schema's list is reported against type",
			rel:      "Concepts/golang/Memo.md",
			body:     "---\ntitle: Memo\ntype: memorandum\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n",
			wantSaid: []string{"<code>type</code>", "memorandum"},
		},
		{
			name:     "a slug the schema's shape rejects is reported against slug",
			rel:      "Writing/lessons/golang/L01.md",
			body:     "---\ntitle: L01\ntype: lesson\ndomain: golang\nstatus: draft\nslug: Not Kebab\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n",
			wantSaid: []string{"<code>slug</code>", "Not Kebab"},
		},
		{
			name:     "a frontmatter key the schema does not know is named",
			rel:      "Concepts/golang/Extra.md",
			body:     "---\ntitle: Extra\ntype: concept\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\nnot_a_field: 1\n---\n\nbody\n",
			wantSaid: []string{"<code>not_a_field</code>"},
		},
		{
			name:     "a missing required field is named",
			rel:      "Concepts/golang/Bare.md",
			body:     "---\ntitle: Bare\ntype: concept\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n",
			wantSaid: []string{"<code>domain</code>"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			full := filepath.Join(root, filepath.FromSlash(tc.rel))
			if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.WriteFile(full, []byte(tc.body), 0o600); err != nil {
				t.Fatalf("write: %v", err)
			}
			srv := newServerWithContract(t, root, loadHomeContract(t))

			code, page := get(t, srv.URL+"/notes/"+tc.rel)
			if code != http.StatusOK {
				t.Fatalf("note page status = %d, want %d", code, http.StatusOK)
			}
			for _, want := range tc.wantSaid {
				if !strings.Contains(page, want) {
					t.Errorf("the page never says %q; a reader is left with the silence this replaced", want)
				}
			}
			for _, unwanted := range tc.wantNot {
				if strings.Contains(page, unwanted) {
					t.Errorf("the page says %q, which points at a field that is not the one at fault", unwanted)
				}
			}
		})
	}
}

// TestNotePageShowsTheFolderTheDomainRuleCompared holds the one value the page
// has to work out for itself. The rule compares the first folder under the
// configured root, so a note nested deeper must be told about that folder and
// not about the one it happens to sit in.
func TestNotePageShowsTheFolderTheDomainRuleCompared(t *testing.T) {
	t.Parallel()

	const rel = "Concepts/japanese/nested/Deep.md"
	body := "---\ntitle: Deep\ntype: concept\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n\nbody\n"

	root := t.TempDir()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))

	code, page := get(t, srv.URL+"/notes/"+rel)
	if code != http.StatusOK {
		t.Fatalf("note page status = %d, want %d", code, http.StatusOK)
	}
	if !strings.Contains(page, "<code>japanese</code>") {
		t.Error("the page does not name japanese, the folder the rule actually compared")
	}
	if strings.Contains(page, "<code>nested</code>") {
		t.Error("the page names nested, which is the folder the note sits in and not the one the rule compared")
	}
}

// TestNotePageEscapesTheNotesOwnTextInANotice covers what a notice is made of:
// the note's own words, quoted back to a reader. A note is a file anyone can
// write, so a value that looks like markup has to arrive as the characters the
// author typed rather than as anything the page acts on.
func TestNotePageEscapesTheNotesOwnTextInANotice(t *testing.T) {
	t.Parallel()

	const rel = "Writing/lessons/golang/L02.md"
	const hostile = `<b onclick="x">y</b>`
	body := "---\ntitle: L02\ntype: lesson\ndomain: golang\nstatus: draft\nslug: '" + hostile + "'\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n"

	root := t.TempDir()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))

	code, page := get(t, srv.URL+"/notes/"+rel)
	if code != http.StatusOK {
		t.Fatalf("note page status = %d, want %d", code, http.StatusOK)
	}
	if !strings.Contains(page, "&lt;b onclick=") {
		t.Errorf("the notice does not carry the author's characters escaped; page did not contain the escaped form")
	}
	if strings.Contains(page, hostile) {
		t.Errorf("the page carries %q verbatim, so a note's own text reached the reader as markup", hostile)
	}
}

// TestNotePageReadsTogetherForAStatusThatIsNotText holds the two sentences a
// non-text status draws, because they are said side by side and a reader has
// to be able to act on the pair. One explains why no status could be read; the
// other names what was written. Before the first was completed it said the
// value was missing or not single, which is false of 123 — a number is both
// present and single — leaving the two sentences disagreeing about whether
// there was a value at all.
func TestNotePageReadsTogetherForAStatusThatIsNotText(t *testing.T) {
	t.Parallel()

	const rel = "Concepts/golang/Numeric.md"
	body := "---\ntitle: Numeric\ntype: concept\ndomain: golang\nstatus: 123\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n\nbody\n"

	root := t.TempDir()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))

	code, page := get(t, srv.URL+"/notes/"+rel)
	if code != http.StatusOK {
		t.Fatalf("note page status = %d, want %d", code, http.StatusOK)
	}
	if !strings.Contains(page, "不是文字") {
		t.Error("the page does not say the value is not text, so its account of why nothing could be read omits this note's actual cause")
	}
	if !strings.Contains(page, "<code>123</code>") {
		t.Error("the page does not name what was written")
	}
}
