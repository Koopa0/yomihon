package note_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
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

// TestChromeCarriesNoGovernanceVoice locks the browser chrome to silence about
// the write face on every kind of folder. The keyboard help documents reading
// keys only — no shortcut writes a status — so the panel names no
// certification key on a governed folder, and a folder with no vault contract
// gets no substitute sentence about a face it lacks: the status faces on the
// pages that have them carry the whole story.
func TestChromeCarriesNoGovernanceVoice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		serve func(t *testing.T) *httptest.Server
	}{
		{
			name: "governed folder",
			serve: func(t *testing.T) *httptest.Server {
				t.Helper()
				root := t.TempDir()
				if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Vault\n"), 0o600); err != nil {
					t.Fatalf("write README: %v", err)
				}
				return newServerWithContract(t, root, loadHomeContract(t))
			},
		},
		{
			name: "folder with no vault contract",
			serve: func(t *testing.T) *httptest.Server {
				t.Helper()
				return newServerWithGovernance(t, writePlainFolder(t), nil, schema.Ungoverned())
			},
		},
		{
			name: "contract that could not be read",
			serve: func(t *testing.T) *httptest.Server {
				t.Helper()
				return newServerWithGovernance(t, writePlainFolder(t), nil, schema.Unreadable(nil))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := tt.serve(t)
			code, body := get(t, srv.Client(), srv.URL+"/")
			if code != http.StatusOK {
				t.Fatalf("GET / = %d, want %d", code, http.StatusOK)
			}
			for _, unwanted := range []string{
				"（按住）",
				"按住 R",
				"認證",
				"這個資料夾沒有 vault contract",
				`class="y-kbdhelp__note"`,
			} {
				if strings.Contains(body, unwanted) {
					t.Errorf("GET / body carries %q, want the chrome silent about the write face", unwanted)
				}
			}
		})
	}
}
