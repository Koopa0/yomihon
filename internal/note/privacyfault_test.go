package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/wording"
)

// contractWithPrivacySection builds the test contract with a privacy
// declaration appended, so the three states a declaration can be in — usable,
// refused, absent — can each be put in front of the home page.
func contractWithPrivacySection(t *testing.T, privacySection string) *schema.Contract {
	t.Helper()
	contract, _ := contractFileWithPrivacySection(t, privacySection)
	return contract
}

// contractFileWithPrivacySection is contractWithPrivacySection for a test that
// goes on to edit the file the contract was read from, which is why it also
// says where that file is.
func contractFileWithPrivacySection(t *testing.T, privacySection string) (contract *schema.Contract, path string) {
	t.Helper()
	base, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read the schema test contract: %v", err)
	}
	path = filepath.Join(t.TempDir(), "vault-schema.toml")
	err = os.WriteFile(path, []byte(string(base)+"\n"+privacySection), 0o600) // #nosec G703 -- fixed basename under this test's TempDir
	if err != nil {
		t.Fatalf("write the contract: %v", err)
	}
	contract, err = schema.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile = %v", err)
	}
	return contract, path
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
	code, body := get(t, srv.Client(), srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if !strings.Contains(body, "never_egress_dirs") {
		t.Errorf("the page does not carry the reason the contract was refused:\n%s", body)
	}
	if !strings.Contains(body, `data-nothing="privacy"`) {
		t.Errorf("the page has no block for a refused egress declaration:\n%s", body)
	}

	// The control: a usable declaration earns no block, or the block above is
	// furniture rather than news.
	usable := contractWithPrivacySection(t, "[privacy]\nnever_egress_dirs = [\"Private\"]\n")
	fine := newServerWithContract(t, fragmentSplitVault(t), usable)
	if _, body := get(t, fine.Client(), fine.URL+"/"); strings.Contains(body, `data-nothing="privacy"`) {
		t.Errorf("a usable egress declaration was reported as a fault:\n%s", body)
	}
}

// TestHomeSaysToRestartWhenTheContractChangedUnderThePrivacyDeclaration is the
// egress declaration's half of the moment the whole-site test drives. The
// contract's bytes move while yomihon is running, the privacy declaration is
// latched shut on the next reconcile, and the desk's block about the judging
// commands then quoted the operator's English line, in a zh-Hant page, in place
// of telling the reader what to do. The reconcile is a wall-clock tick, so this
// calls what the tick calls rather than waiting for one.
func TestHomeSaysToRestartWhenTheContractChangedUnderThePrivacyDeclaration(t *testing.T) {
	t.Parallel()

	contract, path := contractFileWithPrivacySection(t, "[privacy]\nnever_egress_dirs = [\"Private\"]\n")
	srv := newServerWithContract(t, fragmentSplitVault(t), contract)

	// The control: a usable declaration earns no block, so the sentence below is
	// news about the edit and not furniture.
	if _, body := get(t, srv.Client(), srv.URL+"/"); strings.Contains(body, `data-nothing="privacy"`) {
		t.Fatalf("a usable egress declaration was reported as a fault:\n%s", body)
	}

	declared, err := os.ReadFile(path) // #nosec G304 -- the file the helper just wrote, under this test's TempDir
	if err != nil {
		t.Fatalf("read the contract back: %v", err)
	}
	if err = os.WriteFile(path, append(declared, []byte("# note: a comment\n")...), 0o600); err != nil { // #nosec G703 -- fixed basename under this test's TempDir
		t.Fatalf("edit the contract under the running instance: %v", err)
	}
	contract.PrivacyPolicy().ValidateSource()

	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		page := getInLanguage(t, srv.Client(), srv.URL+"/", lang)
		block := pageSection(page, `data-home-block="privacy"`)
		if block == "" {
			t.Fatalf("the desk in %s carries no block about the commands being closed:\n%s", lang, page)
		}
		if want := wording.JoinGuide(wording.ContractChanged, wording.RestartYomihon, lang); !strings.Contains(block, want) {
			t.Errorf("the privacy block in %s does not say %q:\n%s", lang, want, block)
		}
		for _, raw := range []string{"source changed after startup", "until restart"} {
			if strings.Contains(page, raw) {
				t.Errorf("the desk in %s prints the diagnostic constant (%q)", lang, raw)
			}
		}
	}
}
