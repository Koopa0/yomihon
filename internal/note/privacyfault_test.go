package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// contractWithPrivacySection builds the test contract with a privacy
// declaration appended, so the three states a declaration can be in — usable,
// refused, absent — can each be put in front of the home page.
func contractWithPrivacySection(t *testing.T, privacySection string) *schema.Contract {
	t.Helper()
	base, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read the schema test contract: %v", err)
	}
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	err = os.WriteFile(path, []byte(string(base)+"\n"+privacySection), 0o600) // #nosec G703 -- fixed basename under this test's TempDir
	if err != nil {
		t.Fatalf("write the contract: %v", err)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile = %v", err)
	}
	return contract
}

// TestHomeSaysWhyTheAdjudicationCommandsAreClosed keeps the other half of a
// promise the program makes twice. The commands that judge a vault refuse to
// print why a contract could not be used — their output is written for a
// program to read, and naming the fault would quote the vault back out under
// exactly the policy that is missing — and they tell the operator to read it
// on this page instead. The page said nothing at all: a refused declaration, a
// usable one, and a contract with no declaration produced the same bytes.
//
// The sentence is about the commands, not about this page. Nothing here
// consults egress authority, so borrowing the wording that closes a block
// would claim a loss the reader cannot find.
func TestHomeSaysWhyTheAdjudicationCommandsAreClosed(t *testing.T) {
	t.Parallel()

	refused := contractWithPrivacySection(t, "[privacy]\nnever_egress_dirs = [\"/\"]\n")
	srv := newServerWithContract(t, fragmentSplitVault(t), refused)
	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if !strings.Contains(body, "never_egress_dirs") {
		t.Errorf("the page does not carry the reason the contract was refused:\n%s", body)
	}
	if !strings.Contains(body, "data-home-privacy") {
		t.Errorf("the page has no block for a refused egress declaration:\n%s", body)
	}

	// The control: a usable declaration earns no block, or the block above is
	// furniture rather than news.
	usable := contractWithPrivacySection(t, "[privacy]\nnever_egress_dirs = [\"Private\"]\n")
	fine := newServerWithContract(t, fragmentSplitVault(t), usable)
	if _, body := get(t, fine.URL+"/"); strings.Contains(body, "data-home-privacy") {
		t.Errorf("a usable egress declaration was reported as a fault:\n%s", body)
	}
}
