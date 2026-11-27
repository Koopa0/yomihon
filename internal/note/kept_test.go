package note_test

import (
	"fmt"
	"log/slog"
	"testing"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/vault"
)

// openKept names the marks of the vault source holds, with the ledger under
// configDir, and opens the place they are kept in: the vault's own machine
// files. It is closed when the test ends, before the vault is.
func openKept(tb testing.TB, source *vault.Reader, configDir string) (*mark.Store, *mark.VaultMarks) {
	tb.Helper()
	store, err := mark.Open(confdir.At(configDir), source.Name())
	if err != nil {
		tb.Fatal(err)
	}
	kept := mark.OpenVaultMarks(store, source, slog.New(slog.DiscardHandler))
	tb.Cleanup(func() {
		if err := kept.Close(); err != nil {
			tb.Errorf("close the marks: %v", err)
		}
	})
	return store, kept
}

// markID is the id of the n-th mark a test names by hand. Marks made at one
// moment are listed by id, so ids that rise with n keep the order they were
// written in.
func markID(n int) string { return fmt.Sprintf("%032x", n) }
