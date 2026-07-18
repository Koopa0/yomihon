package observer

import (
	"encoding/hex"
	"errors"
	"testing"
)

func TestCommittedReturnsFortyOwnedQueryVectors(t *testing.T) {
	t.Parallel()

	compatibility := committedCompatibility(t)
	workload, err := Committed(compatibility)
	if err != nil {
		t.Fatal(err)
	}
	if workload.Dimension() != 1536 {
		t.Fatalf("Committed().Dimension() = %d, want 1536", workload.Dimension())
	}
	queries := workload.QueryVectors()
	if len(queries) != 40 {
		t.Fatalf("len(Committed().QueryVectors()) = %d, want 40", len(queries))
	}
	wantFirst := queries[0][0]
	wantSecond := queries[1][0]
	queries[0][0]++
	queries[1] = nil

	again := workload.QueryVectors()
	if again[0][0] != wantFirst {
		t.Fatal("Workload.QueryVectors() shared an inner vector with its caller")
	}
	if len(again[1]) == 0 || again[1][0] != wantSecond {
		t.Fatal("Workload.QueryVectors() shared its outer slice with its caller")
	}

	// A separately parsed workload must not share storage either. This is a
	// distinct boundary from repeat reads of one Workload above.
	queries[0][0]++
	second, err := Committed(compatibility)
	if err != nil {
		t.Fatal(err)
	}
	if second.QueryVectors()[0][0] == queries[0][0] {
		t.Fatal("Committed() shared mutable query-vector storage between calls")
	}
}

func TestCommittedRejectsAnotherCompatibilityIdentity(t *testing.T) {
	t.Parallel()

	compatibility := committedCompatibility(t)
	compatibility[0] ^= 0xff
	if _, err := Committed(compatibility); !errors.Is(err, ErrIdentityMismatch) {
		t.Fatalf("Committed() error = %v, want ErrIdentityMismatch", err)
	}
}

func committedCompatibility(t *testing.T) [identitySize]byte {
	t.Helper()
	decoded, err := hex.DecodeString("0c6be8d840df073e343de8adf4e200a14f7ea364db6198ada907ea0079c78ede")
	if err != nil {
		t.Fatal(err)
	}
	var compatibility [identitySize]byte
	copy(compatibility[:], decoded)
	return compatibility
}
