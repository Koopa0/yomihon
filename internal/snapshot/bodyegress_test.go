package snapshot

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/egress"
	"github.com/koopa0/yomihon/internal/judge"
)

// TestBodyMasksWhatTheGenerationSaysReachesWithheldGround holds the gate's
// masking against the two readers it is built from in production: the judge's
// extractor, and a generation that resolves each name the way the page does. The
// fakes in the egress package stand for both, so this is where they are checked
// against the real ones, on names a fake would not think of: one a withheld
// note and a readable one share, and a path that climbs.
func TestBodyMasksWhatTheGenerationSaysReachesWithheldGround(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	contract := testContractDeclaringPrivacy(t, root, "Private")
	writeNote(t, root, "Private/Secret Plan.md", "---\ntype: note\n---\nthe plan\n")
	writeNote(t, root, "Private/Shared.md", "---\ntype: note\n---\nwithheld\n")
	writeNote(t, root, "Notes/Shared.md", "---\ntype: note\n---\nreadable\n")
	writeNote(t, root, "Notes/Plain.md", "---\ntype: note\n---\nplain\n")
	const note = "Notes/Sub/Citing.md"
	writeNote(t, root, note, "---\ntype: note\n---\nbody\n")
	store, reader := newTestStore(t, root, contract)
	gate, err := egress.Open(t.Context(), reader, judge.LinkTargets)
	if err != nil {
		t.Fatalf("egress.Open() error = %v", err)
	}

	body := strings.Join([]string{
		"[[Secret Plan]] [[Secret Plan|the plan]] ![[Secret Plan#Goals]] [[secret plan]]",
		"[[Shared]] [[Notes/Shared]]",
		"[a](../../Private/Secret%20Plan.md) [b](<../../Private/Secret Plan.md>) ![c](/Private/fig.png)",
		"`[[Secret Plan]]` %% [[Secret Plan]] %%",
		"[[Plain]] [[Plain|shown]] [d](../Plain.md)",
		"`[[Plain]] [[Secret Plan]]`",
	}, "\n")
	want := strings.Join([]string{
		egress.Masked + " " + egress.Masked + " " + egress.Masked + " " + egress.Masked,
		egress.Masked + " [[Notes/Shared]]",
		egress.Masked + " " + egress.Masked + " " + egress.Masked,
		"`" + egress.Masked + "` %% " + egress.Masked + " %%",
		"[[Plain]] [[Plain|shown]] [d](../Plain.md)",
		"`[[Plain]] " + egress.Masked + "`",
	}, "\n")
	got, places := gate.Body(note, body, store.Current())
	if got != want {
		t.Errorf("Body() mismatch:\n got: %q\nwant: %q", got, want)
	}
	if places != 11 {
		t.Errorf("Body() masked %d places, want 11", places)
	}
}
