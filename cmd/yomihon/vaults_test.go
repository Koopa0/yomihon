package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/mark/vaults"
	"github.com/koopa0/yomihon/internal/schema"
)

func runVaultsCommand(t *testing.T, configDir string, isTerminal bool, args ...string) (exit int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	exit = runVaults(t.Context(), args, config{configDir: configDir}, &out, &errOut, isTerminal)
	return exit, out.String(), errOut.String()
}

// folderNamed makes a folder with a name of its own, so two vaults can share
// one.
func folderNamed(t *testing.T, parent, name string) string {
	t.Helper()
	dir := filepath.Join(parent, name)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func TestVaultsListsEachVaultsNamePathAndAddress(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	store := listOf(t, configDir)
	notes := folderNamed(t, t.TempDir(), "notes")
	first, _, err := store.Add(notes, someFreePort(t))
	if err != nil {
		t.Fatal(err)
	}

	exit, stdout, stderr := runVaultsCommand(t, configDir, false)
	if exit != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", exit, stderr)
	}
	var got []vaultsLine
	if err = json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout)
	}
	want := vaultsLine{Name: "notes", Path: notes, Address: addressOf(first.Port)}
	if len(got) != 1 || got[0] != want {
		t.Errorf("got %+v, want [%+v]", got, want)
	}

	exit, human, _ := runVaultsCommand(t, configDir, true)
	if exit != 0 {
		t.Fatalf("exit %d", exit)
	}
	for _, field := range []string{want.Name, want.Path, want.Address} {
		if !strings.Contains(human, field) {
			t.Errorf("the table does not hold %q:\n%s", field, human)
		}
	}
	exit, forced, _ := runVaultsCommand(t, configDir, true, "--format", "json")
	if exit != 0 || !strings.HasPrefix(forced, "[") {
		t.Errorf("--format json on a terminal = exit %d, %q; want JSON", exit, forced)
	}
}

func TestVaultsOfAnEmptyListIsAnEmptyAnswerNotAnError(t *testing.T) {
	t.Parallel()
	exit, stdout, stderr := runVaultsCommand(t, t.TempDir(), false)
	if exit != 0 || strings.TrimSpace(stdout) != "[]" || stderr != "" {
		t.Errorf("exit %d, stdout %q, stderr %q; want 0, [], empty", exit, stdout, stderr)
	}
	exit, stdout, _ = runVaultsCommand(t, t.TempDir(), true)
	if exit != 0 || stdout != "" {
		t.Errorf("a terminal's empty list = exit %d, %q; want 0 and nothing", exit, stdout)
	}
}

func TestVaultsRemoveTakesOneOutByNameOrByPathAndKeepsItsMarks(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	store := listOf(t, configDir)
	start := someFreePort(t)
	parent := t.TempDir()
	a, b, c := folderNamed(t, parent, "alpha"), folderNamed(t, parent, "beta"), folderNamed(t, parent, "gamma")
	var added []vaults.Entry
	for _, root := range []string{a, b, c} {
		entry, _, err := store.Add(root, start)
		if err != nil {
			t.Fatal(err)
		}
		added = append(added, entry)
	}
	// The vault's marks are the reader's, kept apart from the list.
	marks := filepath.Join(configDir, "yomihon", added[0].ID)
	if err := os.MkdirAll(marks, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(marks, "marks.json"), []byte("{}"), 0o600); err != nil { // #nosec G703 -- a file under t.TempDir
		t.Fatal(err)
	}

	if exit, stdout, stderr := runVaultsCommand(t, configDir, true, "remove", "alpha"); exit != 0 || !strings.Contains(stdout, a) || stderr != "" {
		t.Fatalf("remove by name = exit %d, stdout %q, stderr %q", exit, stdout, stderr)
	}
	if exit, _, stderr := runVaultsCommand(t, configDir, true, "remove", b); exit != 0 {
		t.Fatalf("remove by path = exit %d, stderr %q", exit, stderr)
	}
	// A path spelled through a link finds the same vault.
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(c, link); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
	if exit, _, stderr := runVaultsCommand(t, configDir, true, "remove", link); exit != 0 {
		t.Fatalf("remove by a path through a link = exit %d, stderr %q", exit, stderr)
	}
	list, err := store.List()
	if err != nil || len(list) != 0 {
		t.Errorf("after removing three the list holds %+v, %v", list, err)
	}
	if _, err = os.Stat(filepath.Join(marks, "marks.json")); err != nil {
		t.Errorf("the vault's marks were touched: %v", err)
	}
	if _, err = os.Stat(a); err != nil {
		t.Errorf("the vault's own folder was touched: %v", err)
	}
}

func TestVaultsRemoveRefusesWhatItCannotChooseBetween(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	store := listOf(t, configDir)
	start := someFreePort(t)
	one := folderNamed(t, t.TempDir(), "notes")
	two := folderNamed(t, t.TempDir(), "notes")
	for _, root := range []string{one, two} {
		if _, _, err := store.Add(root, start); err != nil {
			t.Fatal(err)
		}
	}

	exit, _, stderr := runVaultsCommand(t, configDir, true, "remove", "notes")
	if exit != 2 || !strings.Contains(stderr, one) || !strings.Contains(stderr, two) {
		t.Errorf("a name two vaults share = exit %d, stderr %q; want 2, naming both paths", exit, stderr)
	}
	exit, _, stderr = runVaultsCommand(t, configDir, true, "remove", "nothing")
	if exit != 2 || stderr == "" {
		t.Errorf("a vault that is not there = exit %d, stderr %q; want 2 and a reason", exit, stderr)
	}
	if list, err := store.List(); err != nil || len(list) != 2 {
		t.Errorf("a refused removal changed the list: %+v, %v", list, err)
	}
}

func TestVaultsRefusesWhatItDoesNotTake(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	for _, args := range [][]string{
		{"remove"},
		{"remove", "a", "b"},
		{"remove", "--format", "json", "a"},
		{"unexpected"},
		{"--root", "x"},
		{"--format", "xml"},
		{"--all"},
	} {
		if exit, stdout, stderr := runVaultsCommand(t, configDir, true, args...); exit != 2 || stdout != "" || stderr == "" {
			t.Errorf("yomihon vaults %v = exit %d, stdout %q, stderr %q; want 2, nothing, a usage", args, exit, stdout, stderr)
		}
	}
	var out, errOut bytes.Buffer
	if exit := runVaults(t.Context(), nil, config{noConfigDir: "no home"}, &out, &errOut, false); exit != 2 || !strings.Contains(errOut.String(), "no home") {
		t.Errorf("with no configuration directory = exit %d, stderr %q; want 2 saying why", exit, errOut.String())
	}
}

// The address the command prints is the address the running yomihon serves the
// vault on, so what a reader copies from it opens that vault.
func TestTheAddressVaultsPrintsOpensThatVault(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	start := someFreePort(t)
	r := startRunning(t, configDir, newVaultFolder(t), start)
	r.address(t)

	exit, stdout, _ := runVaultsCommand(t, configDir, false)
	var got []vaultsLine
	if exit != 0 || json.Unmarshal([]byte(stdout), &got) != nil || len(got) != 1 {
		t.Fatalf("exit %d, stdout %q", exit, stdout)
	}
	eventually(t, "the vault to be read", func() bool {
		status, body, _ := httpGet(t, got[0].Address, "/")
		return status == http.StatusOK && !strings.Contains(body, "data-scanning")
	})
}

// What the list keeps as a vault's name is read back by `yomihon vaults` for a
// program, so it is never made from a README the rules file withholds or skips:
// the folder's name stands in. readingSite.listName is the function that decides.
func TestTheNameVaultsPrintsNeverComesFromAReadmeTheRulesWithholdOrSkip(t *testing.T) {
	t.Parallel()
	const secret = "Secret Plans"
	skipping := skipBasenameLockContract
	withholding := strings.Replace(strings.Replace(skipBasenameLockContract, `skipped_files = ["README.md"]`, `skipped_files = []`, 1), `never_egress_dirs = []`, `never_egress_dirs = ["README.md"]`, 1)
	silent := strings.Replace(strings.Replace(skipBasenameLockContract, `skipped_files = ["README.md"]`, `skipped_files = []`, 1), "\n[privacy]\nnever_egress_dirs = []\n", "", 1)
	for _, test := range []struct {
		name  string
		rules string
		named bool
	}{
		{"no rules file", "", true},
		{"a rules file that withholds something else", strings.Replace(withholding, `["README.md"]`, `["Private"]`, 1), true},
		{"a rules file that skips the README", skipping, false},
		{"a rules file that withholds the README", withholding, false},
		{"a rules file that declares no privacy", silent, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			configDir := t.TempDir()
			root := t.TempDir()
			files := map[string]string{"README.md": "# " + secret + "\n\nBody.\n", "Notes/Kept.md": "---\ntype: note\n---\n# Kept\n"}
			if test.rules != "" {
				files[schema.ContractRelPath] = test.rules
			}
			for rel, body := range files {
				full := filepath.Join(root, filepath.FromSlash(rel))
				if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(body), 0o600); err != nil { // #nosec G703 -- a file under t.TempDir
					t.Fatal(err)
				}
			}

			r := startRunning(t, configDir, root, someFreePort(t))
			r.address(t)
			entry := idOf(t, listOf(t, configDir), root)
			var m *mount
			eventually(t, "the vault to be read", func() bool {
				m = r.mountOf(entry.ID)
				return m != nil && m.site.Load() != nil
			})
			r.host.recordRead(t.Context(), m)

			exit, stdout, stderr := runVaultsCommand(t, configDir, false)
			var got []vaultsLine
			if exit != 0 || json.Unmarshal([]byte(stdout), &got) != nil || len(got) != 1 {
				t.Fatalf("exit %d, stdout %q, stderr %q", exit, stdout, stderr)
			}
			want := filepath.Base(entry.Path)
			if test.named {
				want = secret
			}
			if got[0].Name != want {
				t.Errorf("yomihon vaults names the vault %q, want %q", got[0].Name, want)
			}
			if !test.named && strings.Contains(stdout, secret) {
				t.Errorf("the output holds a title the rules file keeps out: %s", stdout)
			}
		})
	}
}

// An entry that cannot be used is left out of the answer, and said on stderr; the
// vaults beside it are listed on the ports they have.
func TestVaultsListsTheRestAndSaysWhichEntryItLeftOut(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	store := listOf(t, configDir)
	notes := folderNamed(t, t.TempDir(), "notes")
	entry, _, err := store.Add(notes, someFreePort(t))
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(configDir, "yomihon", "vaults.json")
	raw, err := os.ReadFile(file) // #nosec G304 -- a file under t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	spliced := strings.Replace(string(raw), `"vaults": [`, `"vaults": [{"id":"x","path":"relative","name":"x","last_read":"2026-11-18T09:00:00Z","port":9000},`, 1)
	if err = os.WriteFile(file, []byte(spliced), 0o600); err != nil { // #nosec G703 -- a file under t.TempDir
		t.Fatal(err)
	}

	exit, stdout, stderr := runVaultsCommand(t, configDir, false)
	if exit != 0 || strings.Count(strings.TrimSpace(stderr), "\n") != 0 || !strings.Contains(stderr, "1 entry that could not be used") {
		t.Fatalf("exit %d, stderr %q; want exit 0 and one line about the entry left out", exit, stderr)
	}
	var got []vaultsLine
	if err = json.Unmarshal([]byte(stdout), &got); err != nil || len(got) != 1 || got[0].Address != addressOf(entry.Port) {
		t.Errorf("stdout %q (%v), want the one good vault on its address", stdout, err)
	}
}
