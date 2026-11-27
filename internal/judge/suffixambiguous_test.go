package judge

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

// A link the page renders ambiguous because several paths end with it names
// no file and is no collision of names, so check reports it exactly as the
// broken link it was before any suffix was looked up — in prose, in a source
// list and in a course row alike. A private file among the candidates is
// neither named nor counted.
func TestRunCheckSuffixAmbiguousLinkStaysBroken(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, schema.ContractRelPath, contractFixture(t, []string{"Diary"}))
	write(t, root, "Concepts/Source.md", "---\nbased_on: ['[[Shared/Twin]]', '[[Mixed/Twin]]']\n---\n[[Shared/Twin]]\n[[Mixed/Twin]]\n")
	write(t, root, "Maps/Course.md", "---\ntype: study-path\n---\n\n## Main {sequence=primary}\n\n- [[Shared/Twin]]\n- [[Mixed/Twin]]\n")
	write(t, root, "A/Shared/Twin.md", "body\n")
	write(t, root, "B/Shared/Twin.md", "body\n")
	write(t, root, "Notes/Mixed/Twin.md", "body\n")
	write(t, root, "Diary/Mixed/Twin.md", "body\n")
	got, _, err := checkBytes(t.Context(), &CheckOptions{Root: root, All: true, Format: FormatJSON})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got, []byte("Diary")) {
		t.Errorf("RunCheck() names a private candidate:\n%s", got)
	}
	var keys []string
	for _, f := range decodeFindings(t, got) {
		switch f.RuleID {
		case "provenance.unresolved", "link.broken", "course.disk_mismatch":
			key := f.Path + " " + string(f.RuleID)
			if f.Field != nil {
				key += " " + *f.Field
			}
			keys = append(keys, key+" "+*f.Target)
		default:
			// The other rules are not what this test is about.
		}
	}
	want := []string{
		"Concepts/Source.md provenance.unresolved based_on [[Shared/Twin]]",
		"Concepts/Source.md provenance.unresolved based_on [[Mixed/Twin]]",
		"Concepts/Source.md link.broken Shared/Twin",
		"Concepts/Source.md link.broken Mixed/Twin",
		"Maps/Course.md course.disk_mismatch Shared/Twin",
		"Maps/Course.md course.disk_mismatch Mixed/Twin",
	}
	if diff := cmp.Diff(want, keys); diff != "" {
		t.Errorf("suffix-ambiguous reference findings (-want +got):\n%s", diff)
	}
}
