package schema

import (
	"os"
	"path/filepath"
	"testing"
)

// A type may begin at more than one status. The vault this repository is
// written for declares two starting rows for a lesson — one for a note written
// here and one for a note brought in from elsewhere — so a predicate that
// answered for a single initial status would withdraw the exemption from
// whichever row it did not pick, and the rule that asks would then report every
// note in that state. The rows are read one status at a time for that reason.
func TestAStatusIsAStartingPointRowByRow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault-schema.toml")
	contract := `schema_version = "1"
[enums]
type = ["probe"]
[enums.status]
note = ["imported", "draft", "ready"]
[fields]
known = ["title", "type", "status"]
[privacy]
never_egress_dirs = []
[[lifecycle]]
status = "imported"
applies_to = ["probe"]
initial = true
from = []
owner = ["author"]
[[lifecycle]]
status = "draft"
applies_to = ["probe"]
initial = true
from = []
owner = ["author"]
[[lifecycle]]
status = "ready"
applies_to = ["probe"]
initial = false
from = ["draft", "imported"]
owner = ["author"]
`
	if err := os.WriteFile(path, []byte(contract), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := LoadFile(path)
	if err != nil {
		t.Fatalf("the contract did not load, so nothing below is about StartsAt: %v", err)
	}
	for _, tc := range []struct {
		status string
		want   bool
	}{
		{"imported", true},
		{"draft", true},
		{"ready", false},
		{"nonesuch", false},
	} {
		if got := c.StartsAt("probe", tc.status); got != tc.want {
			t.Errorf("StartsAt(probe, %q) = %v, want %v", tc.status, got, tc.want)
		}
	}
}
