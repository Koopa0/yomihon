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
