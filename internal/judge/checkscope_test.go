package judge

import (
	"context"
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

// TestCheckKeepsScopeFilteringWhenTheVaultIsNamed holds that the refusal above
// did not narrow what a scope filter may say. Once --root names the vault,
// everything after it is scope and is read inside that vault.
func TestCheckKeepsScopeFilteringWhenTheVaultIsNamed(t *testing.T) {
	t.Parallel()
	vault := writeCheckableVault(t, t.TempDir())

	t.Run("a path inside the vault still filters", func(t *testing.T) {
		t.Parallel()
		if exit, err := checkScope(t.Context(), vault, "Writing"); err != nil || exit != 0 {
			t.Errorf("exit = %d, err = %v; want a clean run", exit, err)
		}
	})

	t.Run("a file inside the vault still filters", func(t *testing.T) {
		t.Parallel()
		if exit, err := checkScope(t.Context(), vault, "Writing/Lesson.md"); err != nil || exit != 0 {
			t.Errorf("exit = %d, err = %v; want a clean run", exit, err)
		}
	})

	t.Run("a filter that names nothing keeps its own refusal", func(t *testing.T) {
		t.Parallel()
		_, err := checkScope(t.Context(), vault, "Nowhere")
		if err == nil {
			t.Fatal("a filter naming nothing was answered rather than refused")
		}
		if !strings.Contains(err.Error(), "names nothing in this vault") {
			t.Errorf("a relative filter that misses was answered by the wrong refusal:\n%v", err)
		}
	})

	t.Run("an absolute filter is refused even once the vault is named", func(t *testing.T) {
		t.Parallel()
		other := writeCheckableVault(t, t.TempDir())
		_, err := checkScope(t.Context(), vault, other)
		if err == nil {
			t.Fatal("an absolute filter was answered rather than refused")
		}
		if !strings.Contains(err.Error(), "absolute path") {
			t.Errorf("an absolute filter was not named as the problem:\n%v", err)
		}
	})
}

// checkScope runs a check over root narrowed to scopes, returning the exit code
// the binary would report and the refusal, if there was one. A refusal is a
// tool error, which the binary turns into exit 2; the exit code here therefore
// only distinguishes a clean run from a gated one.
func checkScope(ctx context.Context, root string, scopes ...string) (exit int, err error) {
	_, exit, err = RunCheck(ctx, &CheckOptions{Root: root, Paths: scopes, Format: FormatJSON})
	return exit, err
}

// checkScopeDenying is checkScope with the error gate the vault's own scheduled
// runs use, which is where an empty answer over withheld ground would be read
// as a pass.
func checkScopeDenying(ctx context.Context, root string, scopes ...string) (exit int, err error) {
	_, exit, err = RunCheck(ctx, &CheckOptions{
		Root: root, Paths: scopes, Deny: []string{"error"}, Format: FormatJSON,
	})
	return exit, err
}

// TestCheckStillAcceptsTheVaultRootAsItsOwnScope holds the one positional that
// names a vault and is not the mistake: a reader standing in the vault who
// writes "." means the whole of it, which is what no filter already means.
func TestCheckStillAcceptsTheVaultRootAsItsOwnScope(t *testing.T) {
	vault := writeCheckableVault(t, t.TempDir())

	if exit, err := checkScope(t.Context(), vault, "."); err != nil || exit != 0 {
		t.Errorf("exit = %d, err = %v; want a clean run", exit, err)
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
		exit, err := checkScopeDenying(t.Context(), root, "Writing/Public.md")
		if err != nil || exit != 1 {
			t.Fatalf("exit = %d, err = %v; want a gated run, or the fixture does not produce the error this test is about", exit, err)
		}
	})

	t.Run("a withheld scope is refused rather than answered", func(t *testing.T) {
		t.Parallel()
		_, err := checkScope(t.Context(), root, "Private/Secret.md")
		if err == nil {
			t.Fatal("an empty answer certifies a scope the command withheld")
		}
		if !strings.Contains(err.Error(), "withholds") {
			t.Errorf("the refusal never says the scope is withheld:\n%v", err)
		}
	})

	t.Run("the deny gate cannot pass on a withheld scope", func(t *testing.T) {
		t.Parallel()
		if exit, err := checkScopeDenying(t.Context(), root, "Private/Secret.md"); err == nil && exit == 0 {
			t.Error("the gate passed a scope holding an error it withheld")
		}
	})

	t.Run("the directory itself is refused the same way", func(t *testing.T) {
		t.Parallel()
		if _, err := checkScope(t.Context(), root, "Private"); err == nil {
			t.Error("naming the directory answered where naming the file is refused")
		}
	})

	t.Run("a public scope is unaffected", func(t *testing.T) {
		t.Parallel()
		if exit, err := checkScope(t.Context(), root, "Writing"); err != nil || exit != 0 {
			t.Errorf("exit = %d, err = %v; want a clean run", exit, err)
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

// TestTheScopeShapeRefusalReachesTheLibraryEntry holds the refusal at the
// boundary that owns it. It used to sit in the argument parser, which meant it
// answered a reader typing at a shell and nothing else: a caller reaching the
// engine with the same absolute path got no refusal at all and a scope that
// silently matched nothing, which is the one answer an adjudication face must
// never give about ground it did not cover.
func TestTheScopeShapeRefusalReachesTheLibraryEntry(t *testing.T) {
	t.Parallel()

	root := writeCheckableVault(t, t.TempDir())

	if _, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Paths: []string{"Writing"}}); err != nil {
		t.Fatalf("RunCheck() on a vault-relative scope error = %v; the fixture has to work or the refusal proves nothing", err)
	}

	_, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Paths: []string{root}})
	if err == nil {
		t.Fatal("RunCheck() accepted an absolute path as a scope filter, so the scope matched nothing and the run read as clean")
	}
	for _, part := range []string{"absolute path", "from the vault's own root"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("RunCheck() error = %q, want it to name %q", err, part)
		}
	}
}
