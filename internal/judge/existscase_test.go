package judge

import "testing"

// TestExistsSimpleCasePeerRemainsANearMatch holds a name that only folds to
// the query by a simple case peer to a near match: it is not an exact answer.
func TestExistsSimpleCasePeerRemainsANearMatch(t *testing.T) {
	t.Parallel()
	if got := existsLookup([]note{{path: "Notes/ϐ.md"}}, "β", testScanAuthority(t)); got.found() {
		t.Errorf("existsLookup(β) = %+v, want no exact answer", got)
	}
}
