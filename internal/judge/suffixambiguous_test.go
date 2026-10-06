package judge

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

// A link the page renders ambiguous because several paths end with it names
// no file and is no collision of names, so check reports it exactly as the
// broken link it was before any suffix was looked up. A private file among
// the candidates is neither named nor counted.
func TestRunCheckSuffixAmbiguousLinkStaysBroken(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, schema.ContractRelPath, contractFixture(t, []string{"Diary"}))
	write(t, root, "Concepts/Source.md", "---\ntitle: Source\n---\n[[Shared/Twin]]\n[[Mixed/Twin]]\n")
	write(t, root, "A/Shared/Twin.md", "body\n")
	write(t, root, "B/Shared/Twin.md", "body\n")
	write(t, root, "Notes/Mixed/Twin.md", "body\n")
	write(t, root, "Diary/Mixed/Twin.md", "body\n")
	got, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON})
	if err != nil {
		t.Fatal(err)
	}
	var broken []string
	for line := range strings.SplitSeq(string(got), "\n") {
		if strings.HasPrefix(line, `{"rule_id":"link.broken",`) {
			broken = append(broken, line)
		}
	}
	// The exact bytes check writes for a link that resolves to nothing: the
	// suffix matches change none of them.
	want := []string{
		`{"rule_id":"link.broken","severity":"warn","path":"Concepts/Source.md","line":4,"message":"[[Shared/Twin]] resolves to no note","evidence":"no filename or alias matches the target","suggested_action":"create the target note, or change the link to an existing filename/alias","source_rule":"yomihon","target":"Shared/Twin","fingerprint":"v1:44f9e7a61a237251"}`,
		`{"rule_id":"link.broken","severity":"warn","path":"Concepts/Source.md","line":5,"message":"[[Mixed/Twin]] resolves to no note","evidence":"no filename or alias matches the target","suggested_action":"create the target note, or change the link to an existing filename/alias","source_rule":"yomihon","target":"Mixed/Twin","fingerprint":"v1:8611c60463ae4a83"}`,
	}
	if diff := cmp.Diff(want, broken); diff != "" {
		t.Errorf("caught: suffix-ambiguous link lines (-want +got):\n%s", diff)
	}
}
