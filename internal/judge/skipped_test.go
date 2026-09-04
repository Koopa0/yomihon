package judge

import (
	"bytes"
	"testing"

	"github.com/koopa0/yomihon/internal/vaultfs"
)

// TestSkippedFindingBytesAreFrozenForEveryKind pins what a consumer parses for
// each way a path can be passed over. The symlink line is also pinned by a
// golden, because a fixture can carry a symbolic link; the other kind cannot be
// committed to a repository — a socket or a device file is not a file anything
// checks out — so its bytes are held here or nowhere.
//
// The kind's phrase is spelled by the scan and reaches the evidence field and
// the fingerprint input. A reword there moves an external consumer's
// fingerprint for findings that are otherwise unchanged, which is what this
// stops from happening quietly.
func TestSkippedFindingBytesAreFrozenForEveryKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind vaultfs.SkipKind
		want string
	}{
		{
			kind: vaultfs.SkipSymlink,
			want: `{"rule_id":"scan.skipped","severity":"warn","path":"Writing/Linked lesson.md","message":"this path is a symbolic link, so nothing was read from it: it holds no note, answers no link, and appears in no listing","evidence":"the scan observed the path and left it out, because a note is read only out of a regular file: symbolic link","suggested_action":"move the file itself into the vault, and cite it from a note with a wikilink rather than linking to it on disk","source_rule":"yomihon","fingerprint":"v1:e0d7ac73da0fe5cc"}` + "\n",
		},
		{
			kind: vaultfs.SkipNotRegular,
			want: `{"rule_id":"scan.skipped","severity":"warn","path":"Writing/Linked lesson.md","message":"this path is not a regular file, so nothing was read from it: it holds no note, answers no link, and appears in no listing","evidence":"the scan observed the path and left it out, because a note is read only out of a regular file: not a regular file","suggested_action":"replace it with a regular file, or remove it if nothing needs it","source_rule":"yomihon","fingerprint":"v1:915c7ddacad7393a"}` + "\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.kind.String(), func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := WriteJSONL(&buf, []Finding{skippedFinding("Writing/Linked lesson.md", tt.kind)}); err != nil {
				t.Fatalf("WriteJSONL: %v", err)
			}
			if buf.String() != tt.want {
				t.Errorf("finding bytes moved:\n got %s\nwant %s", buf.String(), tt.want)
			}
		})
	}
}

// TestEverySkipKindHasItsOwnFinding is the other half: the table above is a
// list, and a kind added to the scan without a line there would be pinned by
// nothing. The set comes from the scan rather than from a bound written here,
// which is the part that makes it a check — a walk that stopped when a kind had
// no phrase would stop at exactly the member somebody forgot, and report
// nothing.
func TestEverySkipKindHasItsOwnFinding(t *testing.T) {
	t.Parallel()

	kinds := vaultfs.SkipKinds()
	if len(kinds) < 2 {
		t.Fatalf("the scan declares %d kinds; with fewer than two this walk cannot tell them apart", len(kinds))
	}
	seen := map[string]bool{}
	for _, kind := range kinds {
		if kind.String() == "unknown" {
			t.Errorf("kind %d has no phrase of its own, so every finding about it reads as unknown", kind)
		}
		f := skippedFinding("A.md", kind)
		if !bytes.Contains([]byte(f.Evidence), []byte(kind.String())) {
			t.Errorf("the finding for %v does not name the kind in its evidence", kind)
		}
		if seen[f.Message] {
			t.Errorf("%v reuses another kind's message, so the two are indistinguishable to a reader", kind)
		}
		seen[f.Message] = true
	}
	if len(seen) != len(kinds) {
		t.Errorf("%d kinds produced %d distinct messages", len(kinds), len(seen))
	}
}
