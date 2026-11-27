package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/mark/machinemarks"
	"github.com/koopa0/yomihon/internal/mark/marktest"
	"github.com/koopa0/yomihon/internal/vault"
)

// keptMarks is where a test leaves the marks of a vault: in the vault's own
// machine files, with the ledger under the configuration directory, the way the
// reading server leaves them.
type keptMarks struct {
	t      *testing.T
	root   string
	config string
}

// markID is the id of the n-th mark a test names by hand.
func markID(n int) string { return fmt.Sprintf("%032x", n) }

// keep leaves each mark in the vault. A mark with no id is given one.
func (k keptMarks) keep(marks ...machinemarks.Mark) {
	k.t.Helper()
	marktest.Keep(k.t, k.root, confdir.At(k.config), marks...)
}

// forget deletes the mark with this id, the way the page's delete does.
func (k keptMarks) forget(id string) {
	k.t.Helper()
	reader, err := vault.Open(k.root)
	if err != nil {
		k.t.Fatal(err)
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			k.t.Error(closeErr)
		}
	}()
	store, err := mark.Open(confdir.At(k.config), reader.Name())
	if err != nil {
		k.t.Fatal(err)
	}
	kept := mark.OpenKept(store, reader, slog.New(slog.DiscardHandler))
	defer func() {
		if closeErr := kept.Close(); closeErr != nil {
			k.t.Error(closeErr)
		}
	}()
	hand, err := kept.Marks()
	if err != nil {
		k.t.Fatal(err)
	}
	view, err := hand.View()
	if err != nil {
		k.t.Fatal(err)
	}
	entry, ok := view.Entry(id, mark.AllMarks)
	if !ok {
		k.t.Fatalf("no mark %s to forget", id)
	}
	if _, err = hand.Delete(k.t.Context(), id, entry.Seen, nil); err != nil {
		k.t.Fatalf("forget mark %s: %v", id, err)
	}
}

// earlierDir is the directory under the configuration directory where the
// earlier file of marks (marks.json) of this vault is, or would be.
func (k keptMarks) earlierDir() string {
	k.t.Helper()
	return earlierMarksDir(k.t, k.config, k.root)
}

// earlierMarksDir is the directory the earlier file of marks of the vault at
// root is kept in under configDir.
func earlierMarksDir(t *testing.T, configDir, root string) string {
	t.Helper()
	return nameMarksDir(t, configDir, vaultNameOf(t, root))
}

// vaultNameOf is the name the marks of the vault at root go by: its path as the
// vault reader resolves it.
func vaultNameOf(t *testing.T, root string) string {
	t.Helper()
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	name := reader.Name()
	if err = reader.Close(); err != nil {
		t.Fatal(err)
	}
	return name
}

// nameMarksDir is the same for a vault that is named by its path alone, one
// that need not exist.
func nameMarksDir(t *testing.T, configDir, vaultRoot string) string {
	t.Helper()
	store, err := mark.Open(confdir.At(configDir), vaultRoot)
	if err != nil {
		t.Fatal(err)
	}
	return store.Dir()
}

// writeEarlierMarks writes the earlier file of marks (marks.json) of the vault
// at vaultRoot, by hand: the shape a reader who used the version before the
// machine files left behind, which the product only reads.
func writeEarlierMarks(t *testing.T, configDir, vaultRoot string, marks ...machinemarks.Mark) {
	t.Helper()
	dir := nameMarksDir(t, configDir, vaultRoot)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	// The earlier file kept a question's words as its reason.
	type earlier struct {
		machinemarks.Mark

		Reason string `json:"reason,omitempty"`
	}
	held := make([]earlier, len(marks))
	for i := range marks {
		held[i] = earlier{Mark: marks[i]}
		if marks[i].Kind == machinemarks.Question {
			held[i].Reason, held[i].Body = marks[i].Body, ""
		}
	}
	data, err := json.Marshal(struct {
		Version int       `json:"version"`
		Vault   string    `json:"vault"`
		Marks   []earlier `json:"marks"`
	}{Version: 1, Vault: vaultRoot, Marks: held})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "marks.json"), data, 0o600); err != nil { // #nosec G703 -- a file under t.TempDir
		t.Fatal(err)
	}
}

// heldQuestions is the questions the reader has open in the vault at root, read
// the way the marks command reads them.
func heldQuestions(t *testing.T, root, configDir string) []machinemarks.Mark {
	t.Helper()
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	store, err := mark.Open(confdir.At(configDir), reader.Name())
	if err != nil {
		t.Fatal(err)
	}
	kept := mark.OpenKept(store, reader, slog.New(slog.DiscardHandler))
	defer func() {
		if closeErr := kept.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	held, err := kept.Questions()
	if err != nil {
		t.Fatal(err)
	}
	return held
}
