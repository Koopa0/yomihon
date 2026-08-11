package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// noContractNote is the sentence the browser chrome owes a reader whose folder
// carries no vault contract, and holdRHelpEntry is the certification key that
// folder can never press. The two travel together: the key leaves the help
// panel and the sentence takes its place, so a reader who opens the panel
// looking for it is told why it is not there instead of finding nothing.
const (
	noContractNote  = "這個資料夾沒有 vault contract：閱讀與搜尋完整可用，狀態認證這一面則不存在。"
	holdRHelpEntry  = "（按住）"
	holdRPrefTip    = "按住 R 都只會照瀏覽器原本的方式輸入"
	kbdHelpNoteMark = `class="y-kbdhelp__note"`
)

// writePlainFolder builds an ordinary folder of ordinary notes with no
// System/schemas/vault-schema.toml anywhere under it. Nothing here ever claimed
// authority, so the write face has nothing to write against.
func writePlainFolder(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range map[string]string{
		"README.md":     "# Notes\n\nA plain folder.\n",
		"Meeting.md":    "# Monday meeting\n\nAgreed to ship on Friday.\n",
		"Recipe.md":     "# Tomato soup\n\nSimmer for twenty minutes.\n",
		"Kyoto trip.md": "# Kyoto\n\nThree days in April.\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, schema.ContractRelPath)); !os.IsNotExist(err) {
		t.Fatalf("os.Stat(%q) = %v, want the contract to be absent", schema.ContractRelPath, err)
	}
	return root
}

// TestChromeSpeaksOnceForAFolderWithNoVaultContract is the ungoverned half. A
// plain folder of ordinary notes reads and searches whole, and the one thing it
// cannot do is adjudicate — so the chrome says that once, where the reader went
// looking for the key that would have done it, and offers the key nowhere.
func TestChromeSpeaksOnceForAFolderWithNoVaultContract(t *testing.T) {
	t.Parallel()

	srv := newServerWithGovernance(t, writePlainFolder(t), nil, schema.Ungoverned())
	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / = %d, want %d", code, http.StatusOK)
	}
	for _, want := range []string{noContractNote, kbdHelpNoteMark} {
		if !strings.Contains(body, want) {
			t.Errorf("GET / body does not carry %q, want it present", want)
		}
	}
	if got := strings.Count(body, noContractNote); got != 1 {
		t.Errorf("strings.Count(body, noContractNote) = %d, want 1", got)
	}
	for _, unwanted := range []string{holdRHelpEntry, holdRPrefTip} {
		if strings.Contains(body, unwanted) {
			t.Errorf("GET / body advertises %q, want it absent for a folder with no contract", unwanted)
		}
	}
}

// TestChromeStaysSilentForAGovernedFolder is the paired negative. A folder that
// declared a contract owns the certification key, and telling its reader that
// the adjudication face does not exist would be false — so the key stays and
// the sentence never appears.
func TestChromeStaysSilentForAGovernedFolder(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Vault\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))
	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / = %d, want %d", code, http.StatusOK)
	}
	for _, want := range []string{holdRHelpEntry, holdRPrefTip} {
		if !strings.Contains(body, want) {
			t.Errorf("GET / body does not carry %q, want the certification key on a governed folder", want)
		}
	}
	for _, unwanted := range []string{noContractNote, kbdHelpNoteMark} {
		if strings.Contains(body, unwanted) {
			t.Errorf("GET / body carries %q, want it absent from a governed folder", unwanted)
		}
	}
}

// TestChromeStaysSilentForAContractItCouldNotRead separates the two absences
// this whole distinction rests on. A folder whose contract exists and could not
// be loaded claimed authority and failed to deliver it — it is in trouble, and
// the vault-level sentence already says so elsewhere. Telling that reader the
// adjudication face does not exist would report the wrong absence.
func TestChromeStaysSilentForAContractItCouldNotRead(t *testing.T) {
	t.Parallel()

	srv := newServerWithGovernance(t, writePlainFolder(t), nil, schema.Unreadable(nil))
	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / = %d, want %d", code, http.StatusOK)
	}
	for _, unwanted := range []string{noContractNote, kbdHelpNoteMark} {
		if strings.Contains(body, unwanted) {
			t.Errorf("GET / body carries %q, want it absent from a folder whose contract could not be read", unwanted)
		}
	}
	if !strings.Contains(body, holdRHelpEntry) {
		t.Errorf("GET / body does not carry %q, want the certification key kept where a contract was claimed", holdRHelpEntry)
	}
}
