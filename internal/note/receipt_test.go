package note

import (
	"path/filepath"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestAReceiptIsVouchedInOneDirectionOnly is the direction lock. The two
// statuses a receipt is about come from one vocabulary and read alike, so the
// check that legalises the move has to be asked about the move that happened
// and not its return journey — the transposition that used to compile is what
// this table walks.
func TestAReceiptIsVouchedInOneDirectionOnly(t *testing.T) {
	t.Parallel()

	contract, err := schema.LoadFile(filepath.Join("testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("schema.LoadFile: %v", err)
	}
	view := homeStatusView(t, contract, contract.Governance())

	tests := []struct {
		name  string
		move  transition
		spent bool
		want  string
	}{
		{
			// The contract declares draft -> ready and does not declare the
			// way back, so only one of these two rows can be vouched for.
			name:  "the move the contract legalises",
			move:  transition{from: "draft", to: "ready"},
			spent: true, want: "draft",
		},
		{
			name:  "the same pair the other way round",
			move:  transition{from: "ready", to: "draft"},
			spent: false, want: "",
		},
		{
			name:  "a status the contract never declared",
			move:  transition{from: "invented", to: "ready"},
			spent: false, want: "",
		},
		{
			name:  "a move that went nowhere",
			move:  transition{from: "ready", to: "ready"},
			spent: false, want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			asked := ""
			consume := func(rel, from string) bool {
				asked = rel + " " + from
				return true
			}
			got := vouchedOrigin(view, consume, "Writing/lesson-01.md", "lesson", tt.move)
			if got != tt.want {
				t.Errorf("vouchedOrigin(%+v) = %q, want %q", tt.move, got, tt.want)
			}
			// A claim the page could never repeat must not spend the one
			// attestation the write face minted for a real flip.
			if spent := asked != ""; spent != tt.spent {
				t.Errorf("the receipt was spent = %v (asked %q), want %v", spent, asked, tt.spent)
			}
			if tt.spent && asked != "Writing/lesson-01.md draft" {
				t.Errorf("the write face was asked about %q, want the note and the status it left", asked)
			}
		})
	}
}
