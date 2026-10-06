package judge

import (
	"strings"
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
	write(t, root, "Concepts/Source.md", "---\ntitle: Source\nbased_on: ['[[Shared/Twin]]', '[[Mixed/Twin]]']\n---\n[[Shared/Twin]]\n[[Mixed/Twin]]\n")
	write(t, root, "Maps/Course.md", "---\ntitle: Course\ntype: study-path\nstatus: ready\n---\n\n## Main {sequence=primary}\n\n- [[Shared/Twin]]\n- [[Mixed/Twin]]\n")
	write(t, root, "A/Shared/Twin.md", "body\n")
	write(t, root, "B/Shared/Twin.md", "body\n")
	write(t, root, "Notes/Mixed/Twin.md", "body\n")
	write(t, root, "Diary/Mixed/Twin.md", "body\n")
	got, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, All: true, Format: FormatJSON})
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for line := range strings.SplitSeq(string(got), "\n") {
		for _, rule := range []string{"provenance.unresolved", "link.broken", "map.disk_mismatch"} {
			if strings.HasPrefix(line, `{"rule_id":"`+rule+`",`) {
				lines = append(lines, line)
			}
		}
	}
	// The exact bytes check writes for a reference that resolves to nothing,
	// taken from check on this vault before any suffix was looked up: the
	// suffix matches change none of them.
	want := []string{
		`{"rule_id":"provenance.unresolved","severity":"warn","path":"Concepts/Source.md","field":"based_on","message":"based_on -> [[Shared/Twin]] resolves to nothing","evidence":"no note, alias, or lesson slug matches the reference","suggested_action":"fix the reference, or create the target note","source_rule":"yomihon","target":"[[Shared/Twin]]","fingerprint":"v1:ac66487d94b275a6"}`,
		`{"rule_id":"provenance.unresolved","severity":"warn","path":"Concepts/Source.md","field":"based_on","message":"based_on -> [[Mixed/Twin]] resolves to nothing","evidence":"no note, alias, or lesson slug matches the reference","suggested_action":"fix the reference, or create the target note","source_rule":"yomihon","target":"[[Mixed/Twin]]","fingerprint":"v1:7707daafad0f8dc2"}`,
		`{"rule_id":"link.broken","severity":"warn","path":"Concepts/Source.md","line":5,"message":"[[Shared/Twin]] resolves to no note","evidence":"no filename or alias matches the target","suggested_action":"create the target note, or change the link to an existing filename/alias","source_rule":"yomihon","target":"Shared/Twin","fingerprint":"v1:44f9e7a61a237251"}`,
		`{"rule_id":"link.broken","severity":"warn","path":"Concepts/Source.md","line":6,"message":"[[Mixed/Twin]] resolves to no note","evidence":"no filename or alias matches the target","suggested_action":"create the target note, or change the link to an existing filename/alias","source_rule":"yomihon","target":"Mixed/Twin","fingerprint":"v1:8611c60463ae4a83"}`,
		`{"rule_id":"map.disk_mismatch","severity":"warn","path":"Maps/Course.md","line":9,"message":"syllabus links [[Shared/Twin]] but it resolves to nothing","evidence":"a study-path entry that resolves to no note on disk","suggested_action":"create the note, fix the entry, or mark it a planned gap","source_rule":"yomihon","target":"Shared/Twin","fingerprint":"v1:4af8c5e8ab76b9da"}`,
		`{"rule_id":"map.disk_mismatch","severity":"warn","path":"Maps/Course.md","line":10,"message":"syllabus links [[Mixed/Twin]] but it resolves to nothing","evidence":"a study-path entry that resolves to no note on disk","suggested_action":"create the note, fix the entry, or mark it a planned gap","source_rule":"yomihon","target":"Mixed/Twin","fingerprint":"v1:e08460f4e2c7bdf2"}`,
	}
	if diff := cmp.Diff(want, lines); diff != "" {
		t.Errorf("caught: suffix-ambiguous reference lines (-want +got):\n%s", diff)
	}
}
