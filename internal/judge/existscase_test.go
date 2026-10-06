package judge

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestExistsSimpleCasePeerRemainsANearMatch(t *testing.T) {
	t.Parallel()
	authority := loadTestAuthority(t, "testdata/vault-exists-fold")
	got := existsLookup([]note{{path: "Notes/ϐ.md"}}, "β", authority)
	want := existsReport{Query: "β", Matches: []existsMatch{}, NearMatches: []existsMatch{{Path: "Notes/ϐ.md", Field: "filename", Value: "ϐ"}}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("existsLookup(β) (-want +got):\n%s", diff)
	}
	if got.found() {
		t.Fatal("existsLookup(β) promoted a near match to an exact answer")
	}
	wire, err := marshalWire(got)
	if err != nil {
		t.Fatal(err)
	}
	const expected = "{\"query\":\"β\",\"matches\":[],\"near_matches\":[{\"path\":\"Notes/ϐ.md\",\"field\":\"filename\",\"value\":\"ϐ\"}]}\n"
	if diff := cmp.Diff(expected, string(wire)); diff != "" {
		t.Errorf("exists JSON (-want +got):\n%s", diff)
	}
}
