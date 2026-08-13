package judge

import (
	"bytes"
	"strings"
	"testing"
)

// writeCheckableVault builds a vault a check can actually run on: the contract
// that gives the command its vocabulary, and one note for a scope filter to
// name. It returns the vault root.
func writeCheckableVault(t *testing.T, root string) string {
	t.Helper()
	writeSupersessionContract(t, root)
	writeJudgeNote(t, root, "Writing/Lesson.md",
		"---\ntitle: Lesson\ntype: lesson\nstatus: ready\n---\nbody\n")
	return root
}

// TestCheckSaysWhichWordToMoveWhenTheVaultIsTypedAsScope holds the refusal a
// reader gets for the mistake this command invites. The vault and a scope
// filter are both directories typed after the command, and putting the vault in
// the scope position produced a message about vault-relative paths that never
// named the word to move.
func TestCheckSaysWhichWordToMoveWhenTheVaultIsTypedAsScope(t *testing.T) {
	vault := writeCheckableVault(t, t.TempDir())
	elsewhere := t.TempDir()
	t.Chdir(elsewhere)

	var stdout, stderr bytes.Buffer
	exit := RunCommand("check", []string{vault}, &stdout, &stderr, false)

	if exit != 2 {
		t.Errorf("exit = %d, want 2", exit)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--root") {
		t.Errorf("the refusal never names the flag the vault belongs to:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), vault) {
		t.Errorf("the refusal never names the directory it is about:\n%s", stderr.String())
	}
}

// TestCheckKeepsScopeFilteringWhenTheVaultIsNamed holds that the refusal above
// did not narrow what a scope filter may say. Once --root names the vault,
// everything after it is scope and is read inside that vault.
func TestCheckKeepsScopeFilteringWhenTheVaultIsNamed(t *testing.T) {
	t.Parallel()
	vault := writeCheckableVault(t, t.TempDir())

	t.Run("a path inside the vault still filters", func(t *testing.T) {
		t.Parallel()
		var stdout, stderr bytes.Buffer
		exit := RunCommand("check", []string{"--root", vault, "Writing"}, &stdout, &stderr, false)
		if exit != 0 {
			t.Errorf("exit = %d, want 0; stderr:\n%s", exit, stderr.String())
		}
	})

	t.Run("a file inside the vault still filters", func(t *testing.T) {
		t.Parallel()
		var stdout, stderr bytes.Buffer
		exit := RunCommand("check", []string{"--root", vault, "Writing/Lesson.md"}, &stdout, &stderr, false)
		if exit != 0 {
			t.Errorf("exit = %d, want 0; stderr:\n%s", exit, stderr.String())
		}
	})

	t.Run("a filter that names nothing keeps its own refusal", func(t *testing.T) {
		t.Parallel()
		var stdout, stderr bytes.Buffer
		exit := RunCommand("check", []string{"--root", vault, "Nowhere"}, &stdout, &stderr, false)
		if exit != 2 {
			t.Errorf("exit = %d, want 2; stderr:\n%s", exit, stderr.String())
		}
		if !strings.Contains(stderr.String(), "names nothing in this vault") {
			t.Errorf("a relative filter that misses was answered by the wrong refusal:\n%s", stderr.String())
		}
	})

	t.Run("an absolute filter is refused even once the vault is named", func(t *testing.T) {
		t.Parallel()
		other := writeCheckableVault(t, t.TempDir())
		var stdout, stderr bytes.Buffer
		exit := RunCommand("check", []string{"--root", vault, other}, &stdout, &stderr, false)
		if exit != 2 {
			t.Errorf("exit = %d, want 2; stderr:\n%s", exit, stderr.String())
		}
		if !strings.Contains(stderr.String(), "absolute path") {
			t.Errorf("an absolute filter was not named as the problem:\n%s", stderr.String())
		}
	})
}

// TestCheckStillAcceptsTheVaultRootAsItsOwnScope holds the one positional that
// names a vault and is not the mistake: a reader standing in the vault who
// writes "." means the whole of it, which is what no filter already means.
func TestCheckStillAcceptsTheVaultRootAsItsOwnScope(t *testing.T) {
	vault := writeCheckableVault(t, t.TempDir())
	t.Chdir(vault)

	var stdout, stderr bytes.Buffer
	exit := RunCommand("check", []string{"."}, &stdout, &stderr, false)
	if exit != 0 {
		t.Errorf("exit = %d, want 0; stderr:\n%s", exit, stderr.String())
	}
}
