package recording

import "testing"

func TestCommittedRecordingIsTheSelectedLatencyWorkload(t *testing.T) {
	t.Parallel()

	vectors, err := Committed()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.Dimension() != 1536 {
		t.Errorf("Committed().Dimension() = %d, want 1536", vectors.Dimension())
	}
	if got := len(vectors.Queries()); got != 40 {
		t.Errorf("Committed() query count = %d, want 40", got)
	}
	if got := len(vectors.Chunks()); got != 32 {
		t.Errorf("Committed() chunk count = %d, want 32", got)
	}
}
