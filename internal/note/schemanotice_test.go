package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/wording"
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
			// The status is legal for every type the schema declares, and the
			// schema declares no statuses at all for a type it does not know —
			// so nothing here can rule on it. Saying it is outside the list
			// points the reader at the one field that is fine, and it used to
			// be said twice, beside the sentence that names the real fault.
			wantNot: []string{wording.StatusOutsideList.In(wording.ZhHant)},
		},
		{
			name:     "a declared type with a status outside its list still says so",
			rel:      "Concepts/golang/Off.md",
			body:     "---\ntitle: Off\ntype: concept\ndomain: golang\nstatus: nonesuch\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n\nbody\n",
			wantSaid: []string{wording.StatusOutsideList.In(wording.ZhHant), "nonesuch"},
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

			code, page := get(t, srv.Client(), srv.URL+"/notes/"+tc.rel)
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

	code, page := get(t, srv.Client(), srv.URL+"/notes/"+rel)
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

	code, page := get(t, srv.Client(), srv.URL+"/notes/"+rel)
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

	code, page := get(t, srv.Client(), srv.URL+"/notes/"+rel)
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

// TestThePageNamesTheFolderTheJudgeActuallyCompared binds the two sides that
// decide "which folder" for one note. The rule lives in the judging package —
// the domain must equal the first folder under a configured root — and the page
// works the same segment out again so it can name that folder in a sentence.
// Two implementations of one rule, and nothing joined them: changing the page's
// arithmetic turned this package red, and changing the rule's turned nothing
// red at all, which is the direction that ships a page confidently naming the
// wrong folder.
//
// So the folder is not written down here. It is read out of the finding the
// real judge produced for this very note, and the page is asked for that. A
// change to either side that moves the segment now moves them apart, and the
// test says so.
func TestThePageNamesTheFolderTheJudgeActuallyCompared(t *testing.T) {
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
	contractBytes, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read the contract fixture: %v", err)
	}
	contractPath := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	if mkdirErr := os.MkdirAll(filepath.Dir(contractPath), 0o750); mkdirErr != nil {
		t.Fatalf("mkdir contract: %v", mkdirErr)
	}
	// The judging commands fail closed without an egress declaration, and the
	// shared fixture declares none, so this one is written with an empty list:
	// the question here is which folder a rule compared, not what may leave.
	contractBytes = append(contractBytes, []byte("\n[privacy]\nnever_egress_dirs = []\n")...)
	if writeErr := os.WriteFile(contractPath, contractBytes, 0o600); writeErr != nil { // #nosec G703 -- the path is this test's own t.TempDir() joined with a fixed name
		t.Fatalf("write contract: %v", writeErr)
	}

	// A second note, alike but for its domain matching the folder the rule is
	// supposed to compare. It binds the comparison as well as the naming: the
	// finding's message would still say "japanese" if the condition beside it
	// started comparing some other segment, and this note is what notices —
	// under the rule as written it draws nothing, and under a rule comparing
	// the folder the note merely sits in it draws a finding.
	const agreeing = "Concepts/japanese/nested/Agrees.md"
	agreeingBody := "---\ntitle: Agrees\ntype: concept\ndomain: japanese\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n\nbody\n"
	if writeErr := os.WriteFile(filepath.Join(filepath.Dir(full), "Agrees.md"), []byte(agreeingBody), 0o600); writeErr != nil { // #nosec G703 -- a path under this test's own t.TempDir()
		t.Fatalf("write the agreeing note: %v", writeErr)
	}

	findings, err := judge.Check(t.Context(), root)
	if err != nil {
		t.Fatalf("judge.Check: %v", err)
	}
	for i := range findings {
		if findings[i].RuleID == "schema.domain_folder" && findings[i].Path == agreeing {
			t.Errorf("a note whose domain matches the folder the rule compares drew %q", findings[i].Message)
		}
	}
	var compared string
	seen := 0
	for i := range findings {
		if findings[i].RuleID != "schema.domain_folder" || findings[i].Path != rel {
			continue
		}
		seen++
		_, folder, found := strings.Cut(findings[i].Message, "does not match its folder ")
		if !found {
			t.Fatalf("the finding's message no longer names a folder, so this test cannot read one out of it: %q", findings[i].Message)
		}
		compared = folder
	}
	if seen != 1 {
		t.Fatalf("the judge reported %d folder findings for this note; the fixture is meant to draw exactly one", seen)
	}

	srv := newServerWithContract(t, root, loadHomeContract(t))
	code, page := get(t, srv.Client(), srv.URL+"/notes/"+rel)
	if code != http.StatusOK {
		t.Fatalf("note page status = %d, want %d", code, http.StatusOK)
	}
	if !strings.Contains(page, "<code>"+compared+"</code>") {
		t.Errorf("the judge compared the folder %q and the page does not name it", compared)
	}
}
