package judge

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/vault"
)

// TestANotRegularFileFindingKeepsItsBytes pins what a consumer parses for a
// path that is neither a regular file nor a symbolic link. A socket or a device
// file cannot be committed to a repository, so unlike the symbolic link, which
// a golden holds, its bytes are held here.
func TestANotRegularFileFindingKeepsItsBytes(t *testing.T) {
	t.Parallel()

	const want = `{"rule_id":"scan.skipped","severity":"warn","path":"Writing/Linked lesson.md","message":"this path is not a regular file, so nothing was read from it: it holds no note, answers no link, and appears in no listing","evidence":"the scan observed the path and left it out, because a note is read only out of a regular file: not a regular file","suggested_action":"replace it with a regular file, or remove it if nothing needs it","source_rule":"yomihon","fingerprint":"v1:915c7ddacad7393a"}` + "\n"
	var buf bytes.Buffer
	if err := writeJSONL(&buf, []Finding{skippedFinding("Writing/Linked lesson.md", vault.SkipNotRegular)}); err != nil {
		t.Fatalf("WriteJSONL: %v", err)
	}
	if buf.String() != want {
		t.Errorf("finding bytes moved:\n got %s\nwant %s", buf.String(), want)
	}
}

// TestASkippedFindingNamesItsKind holds that the evidence of a skipped path
// says which kind of path it was, for every kind the scan declares.
func TestASkippedFindingNamesItsKind(t *testing.T) {
	t.Parallel()

	for _, kind := range vault.SkipKinds() {
		if f := skippedFinding("A.md", kind); !strings.Contains(f.Evidence, kind.String()) {
			t.Errorf("skippedFinding(%v).Evidence = %q, want it to name the kind", kind, f.Evidence)
		}
	}
}
