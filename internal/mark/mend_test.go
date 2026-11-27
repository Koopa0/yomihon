package mark_test

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/mark/machinemarks"
	"github.com/koopa0/yomihon/internal/mark/marktest"
	"github.com/koopa0/yomihon/internal/vault"
)

// aMendedVault is a vault with one mark kept the way the server keeps it, the
// marks of it, and what its log says.
func aMendedVault(t *testing.T) (root string, kept *mark.VaultMarks, logged *bytes.Buffer) {
	t.Helper()
	_, root, err := vault.ResolveRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	config := t.TempDir()
	marktest.Leave(t, root, confdir.At(config), machinemarks.Mark{Kind: machinemarks.Thought, Path: "Notes/source.md", Body: "mine", At: time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)})
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reader.Close() }) //nolint:errcheck // the test is over
	store, err := mark.Open(confdir.At(config), reader.Name())
	if err != nil {
		t.Fatal(err)
	}
	logged = &bytes.Buffer{}
	kept = mark.OpenVaultMarks(store, reader, slog.New(slog.NewTextHandler(logged, nil)))
	t.Cleanup(func() { _ = kept.Close() }) //nolint:errcheck // the test is over
	return root, kept, logged
}

func machineFileOf(t *testing.T, root string) string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(root, "reader-notes", ".yomihon", "marks", "*.jsonl"))
	if err != nil || len(files) != 1 {
		t.Fatalf("the vault holds %v, %v; want this computer's one machine file", files, err)
	}
	return files[0]
}

// What git took out of this computer's file before it was committed is put
// back by the next page that reads the marks, with nothing said to the reader:
// the file is as it was, and the log says it at Info.
func TestMendPutsBackWhatGitTookWithoutBeingAsked(t *testing.T) {
	t.Parallel()
	root, kept, logged := aMendedVault(t)
	file := machineFileOf(t, root)
	was, err := os.ReadFile(file) // #nosec G304 -- a test's own path
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(file); err != nil {
		t.Fatal(err)
	}
	kept.Mend()
	got, err := os.ReadFile(file) // #nosec G304 -- a test's own path
	if err != nil {
		t.Fatalf("after a page that reads the marks the file is not back: %v", err)
	}
	if diff := cmp.Diff(string(was), string(got)); diff != "" {
		t.Errorf("the file put back is not the file taken (-was +got):\n%s", diff)
	}
	if n := strings.Count(logged.String(), "level=INFO"); n != 1 {
		t.Errorf("the log says %q, want one Info line that marks were put back", logged.String())
	}
}

// A page that finds nothing missing writes nothing, not even a modification
// time, and says nothing.
func TestMendWritesNothingWhereNothingIsMissing(t *testing.T) {
	t.Parallel()
	root, kept, logged := aMendedVault(t)
	before := tree(t, root)
	kept.Mend()
	kept.Mend()
	if diff := cmp.Diff(before, tree(t, root)); diff != "" {
		t.Errorf("a page that found nothing missing changed the vault (-before +after):\n%s", diff)
	}
	if logged.Len() != 0 {
		t.Errorf("the log says %q, want nothing", logged.String())
	}
}

// Where the reader folder cannot be written to, nothing is put back and
// nothing is made: the log says why, and the marks stay in the ledger for
// when it can.
func TestMendLeavesAFolderItCannotWriteToAsItFound(t *testing.T) {
	t.Parallel()
	root, kept, logged := aMendedVault(t)
	if err := os.RemoveAll(filepath.Join(root, "reader-notes")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "reader-notes"), []byte("not a folder"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := tree(t, root)
	kept.Mend()
	if diff := cmp.Diff(before, tree(t, root)); diff != "" {
		t.Errorf("a mend over a folder it cannot write to changed the vault (-before +after):\n%s", diff)
	}
	if line := logged.String(); !strings.Contains(line, "level=WARN") {
		t.Errorf("the log says %q, want a Warn that marks could not be put back", line)
	}
}

// A state of the files that the reader is not told of is said to the log, once
// for as long as it stays so, and changes nothing.
func TestMendSaysAFileAnotherProgramChangedToTheLogOnce(t *testing.T) {
	t.Parallel()
	root, kept, logged := aMendedVault(t)
	foreign := filepath.Join(root, "reader-notes", ".yomihon", "marks", "deadbeef.jsonl")
	if err := os.WriteFile(foreign, []byte("{\"this\":\"is not a line\"}\nnot json at all\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := tree(t, root)
	at := time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)
	kept.SetClock(func() time.Time { return at })
	for range 3 {
		kept.Mend()
		at = at.Add(time.Minute)
	}
	if diff := cmp.Diff(before, tree(t, root)); diff != "" {
		t.Errorf("a mend over a file it cannot use changed the vault (-before +after):\n%s", diff)
	}
	if n := strings.Count(logged.String(), "level=WARN"); n != 1 {
		t.Errorf("the log says %q, want the state said once, not %d times", logged.String(), n)
	}
}

// A page is drawn many times a minute and a mend reads every machine file, so a
// process mends at most once in a few seconds: the marks git takes out between two
// mends are put back by the first page after that.
func TestMendReadsAtMostOnceInAFewSeconds(t *testing.T) {
	t.Parallel()
	root, kept, _ := aMendedVault(t)
	file := machineFileOf(t, root)
	at := time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)
	kept.SetClock(func() time.Time { return at })

	kept.Mend() // a first mend, which finds nothing missing
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	at = at.Add(time.Second)
	kept.Mend()
	if _, err := os.Stat(file); err == nil {
		t.Fatal("a second mend a second after the first read the files again")
	}
	at = at.Add(10 * time.Second)
	kept.Mend()
	if _, err := os.Stat(file); err != nil {
		t.Errorf("a mend ten seconds on did not put the file back: %v", err)
	}
}

// The warning that marks are missing and cannot be put back is said once for as
// long as it stays so, as every other state of the files is.
func TestMendSaysThatItCannotPutBackOnce(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("a folder's mode does not stop the owner on this system")
	}
	t.Parallel()
	root, kept, logged := aMendedVault(t)
	if err := os.Remove(machineFileOf(t, root)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, "reader-notes", ".yomihon", "marks"), 0o500); err != nil { // #nosec G302 -- a test makes its own folder read only
		t.Fatal(err)
	}
	// #nosec G302 -- a test restores its own temporary folder so that it can be removed
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, "reader-notes", ".yomihon", "marks"), 0o700) }) //nolint:errcheck // so the folder can be removed
	at := time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)
	kept.SetClock(func() time.Time { return at })
	for range 3 {
		kept.Mend()
		at = at.Add(time.Minute)
	}
	if n := strings.Count(logged.String(), "level=WARN"); n != 1 {
		t.Errorf("the log says %q, want it said once, not %d times", logged.String(), n)
	}
}
