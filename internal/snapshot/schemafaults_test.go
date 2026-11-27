package snapshot

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestSchemaFaultsArriveInVaultPathOrder holds the order the whole-folder view
// hands the schema's complaints over in: the folder's own path order. The page
// that lists them shows its rows in that order, so a list that arrived
// shuffled would reorder the table on every rebuild.
func TestSchemaFaultsArriveInVaultPathOrder(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// Enough notes that an order which is not the folder's own cannot pass for
	// it. A handful would be walked in one bucket by any map standing in for
	// this list, and one rotation of a handful is the right answer by accident
	// often enough that the check would report a regression as a coin toss.
	names := []string{
		"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel",
		"india", "juliett", "kilo", "lima", "mike", "november", "oscar", "papa",
		"quebec", "romeo", "sierra", "tango", "uniform", "victor", "whiskey", "xray",
		"yankee", "zulu",
	}
	// Written back to front, so nothing downstream can be reading the order
	// they were laid down in and passing for the order the folder is in.
	for _, name := range slices.Backward(names) {
		writeNote(t, root, "Writing/"+name+".md",
			"---\ntype: lesson\nnot_a_field: 1\n---\n\nbody\n")
	}
	contract := testContract(t, root)
	store, _ := newTestStore(t, root, contract)
	health := store.Current().Health()

	got := make([]string, 0, len(health.SchemaFaults))
	for _, row := range health.SchemaFaults {
		got = append(got, row.Note.RelPath)
	}
	want := make([]string, 0, len(names))
	for _, name := range names {
		want = append(want, "Writing/"+name+".md")
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("schema faults are not in vault path order (-want +got):\n%s", diff)
	}
	for _, row := range health.SchemaFaults {
		if row.Count < 1 {
			t.Errorf("%s drew a row counting %d findings, and a row exists because something was found", row.Note.RelPath, row.Count)
		}
		if row.Note.Name == "" {
			t.Errorf("%s drew a row naming no note", row.Note.RelPath)
		}
	}
}
