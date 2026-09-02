package judge

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
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
	exit := RunCommand(t.Context(), "check", []string{vault}, &stdout, &stderr, false)

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
		exit := RunCommand(t.Context(), "check", []string{"--root", vault, "Writing"}, &stdout, &stderr, false)
		if exit != 0 {
			t.Errorf("exit = %d, want 0; stderr:\n%s", exit, stderr.String())
		}
	})

	t.Run("a file inside the vault still filters", func(t *testing.T) {
		t.Parallel()
		var stdout, stderr bytes.Buffer
		exit := RunCommand(t.Context(), "check", []string{"--root", vault, "Writing/Lesson.md"}, &stdout, &stderr, false)
		if exit != 0 {
			t.Errorf("exit = %d, want 0; stderr:\n%s", exit, stderr.String())
		}
	})

	t.Run("a filter that names nothing keeps its own refusal", func(t *testing.T) {
		t.Parallel()
		var stdout, stderr bytes.Buffer
		exit := RunCommand(t.Context(), "check", []string{"--root", vault, "Nowhere"}, &stdout, &stderr, false)
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
		exit := RunCommand(t.Context(), "check", []string{"--root", vault, other}, &stdout, &stderr, false)
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
	exit := RunCommand(t.Context(), "check", []string{"."}, &stdout, &stderr, false)
	if exit != 0 {
		t.Errorf("exit = %d, want 0; stderr:\n%s", exit, stderr.String())
	}
}

// TestCheckRefusesAScopeTheContractWithholds holds the one answer a targeted
// check must never give about a directory it may report nothing from. Findings
// are dropped for egress before the scope filter runs, so a path inside a
// never-egress directory reached exactly the outcome filterByPaths already
// refuses for a mistyped path: an empty result that reads as a verdict over
// ground the command did not report on.
//
// The --deny half is the load-bearing one. The vault's own cron wrappers run
// targeted checks with --deny error, so exit 0 there is a PASS certificate
// issued for a scope whose genuine error was withheld.
func TestCheckRefusesAScopeTheContractWithholds(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeWithheldScopeContract(t, root)
	const broken = "---\ntitle: T\ntype: lesson\ndomain: meta\nstatus: not-a-declared-status\ncreated: 2026-01-01\nupdated: 2026-01-01\n---\nbody\n"
	writeJudgeNote(t, root, "Writing/Public.md", broken)
	writeJudgeNote(t, root, "Private/Secret.md", broken)

	t.Run("the public control proves the finding is real and gated", func(t *testing.T) {
		t.Parallel()
		var stdout, stderr bytes.Buffer
		exit := RunCommand(t.Context(), "check", []string{"--root", root, "--deny", "error", "Writing/Public.md"}, &stdout, &stderr, false)
		if exit != 1 {
			t.Fatalf("exit = %d, want 1; the fixture does not produce the error this test is about\nstdout:\n%s\nstderr:\n%s", exit, stdout.String(), stderr.String())
		}
	})

	t.Run("a withheld scope is refused rather than answered", func(t *testing.T) {
		t.Parallel()
		var stdout, stderr bytes.Buffer
		exit := RunCommand(t.Context(), "check", []string{"--root", root, "Private/Secret.md"}, &stdout, &stderr, false)
		if exit != 2 {
			t.Errorf("exit = %d, want 2; an empty answer certifies a scope the command withheld\nstdout:\n%s", exit, stdout.String())
		}
		if stdout.Len() != 0 {
			t.Errorf("stdout = %q, want empty", stdout.String())
		}
		if !strings.Contains(stderr.String(), "withholds") {
			t.Errorf("the refusal never says the scope is withheld:\n%s", stderr.String())
		}
	})

	t.Run("the deny gate cannot pass on a withheld scope", func(t *testing.T) {
		t.Parallel()
		var stdout, stderr bytes.Buffer
		exit := RunCommand(t.Context(), "check", []string{"--root", root, "--deny", "error", "Private/Secret.md"}, &stdout, &stderr, false)
		if exit == 0 {
			t.Errorf("exit = 0: the gate passed a scope holding an error it withheld\nstdout:\n%s\nstderr:\n%s", stdout.String(), stderr.String())
		}
	})

	t.Run("the directory itself is refused the same way", func(t *testing.T) {
		t.Parallel()
		var stdout, stderr bytes.Buffer
		exit := RunCommand(t.Context(), "check", []string{"--root", root, "Private"}, &stdout, &stderr, false)
		if exit != 2 {
			t.Errorf("exit = %d, want 2; naming the directory answers as cleanly as naming the file\nstdout:\n%s", exit, stdout.String())
		}
	})

	t.Run("a public scope is unaffected", func(t *testing.T) {
		t.Parallel()
		var stdout, stderr bytes.Buffer
		exit := RunCommand(t.Context(), "check", []string{"--root", root, "Writing"}, &stdout, &stderr, false)
		if exit != 0 {
			t.Errorf("exit = %d, want 0; stderr:\n%s", exit, stderr.String())
		}
	})
}

// writeWithheldScopeContract writes a contract whose knowledge directories
// include both a public and a never-egress directory, so the same fault
// authored in each produces a finding in one and a withheld finding in the
// other. writeTestContract cannot serve here: it empties knowledge_dirs, and a
// schema rule that never fires would leave the gate subtest passing on a scope
// that held nothing to withhold.
func writeWithheldScopeContract(t *testing.T, root string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read schema fixture: %v", err)
	}
	const knowledgeDirs = `knowledge_dirs = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`
	text := strings.Replace(string(data), knowledgeDirs, `knowledge_dirs = ["Writing", "Private"]`, 1)
	if text == string(data) {
		t.Fatalf("schema fixture does not declare %s", knowledgeDirs)
	}
	text += "\n[privacy]\nnever_egress_dirs = [\"Private\"]\n"
	writeJudgeNote(t, root, schema.ContractRelPath, text)
}
