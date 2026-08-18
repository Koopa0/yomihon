package status_test

import (
	"crypto/sha256"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/vault"
)

func removeOnCleanup(t *testing.T, path string) {
	t.Helper()
	t.Cleanup(func() {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Errorf("remove test artifact %s: %v", path, err)
		}
	})
}

// testRel is the vault-relative path every fixture note in this package
// uses. Fixing it keeps the git assertions (commit message, dirty-file
// targeting) simple; the path itself is never what's under test.
const testRel = "Writing/lessons/japanese/L05.md"

// The testdata contract is a loader fixture, not a second schema: runtime
// code only ever reads the real vault contract. schema.LoadFile is
// reused as-is — there is no second schema-loading path in this package.
func loadContract(t *testing.T) *schema.Contract {
	t.Helper()
	s, err := schema.LoadFile(filepath.Join("testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("LoadFile(testdata/contract.toml) = %v", err)
	}
	return s
}

func loadContractWithEmptyDefaultStatusGroup(t *testing.T) *schema.Contract {
	t.Helper()
	const contractText = `schema_version = "1"

[enums]
type = ["doc"]

[enums.status]
note = []
doc = ["draft"]

[fields.status_group]
doc = ["doc"]

[artifacts]
non_instance_dirs = []

[[lifecycle]]
status = "draft"
applies_to = ["doc"]
from = []
owner = ["koopa"]
`
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	if writeErr := os.WriteFile(path, []byte(contractText), 0o600); writeErr != nil { // #nosec G703 -- path is a fixed basename under t.TempDir
		t.Fatalf("write test contract: %v", writeErr)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile(%q) = %v", path, err)
	}
	return contract
}

func loadContractWithArtifactSection(t *testing.T, section string) *schema.Contract {
	t.Helper()
	return loadFixtureWithArtifactSection(t, filepath.Join("testdata", "contract.toml"), section)
}

func loadFixtureWithArtifactSection(t *testing.T, fixturePath, section string) *schema.Contract {
	t.Helper()
	data, err := os.ReadFile(fixturePath) // #nosec G304 -- callers pass fixed in-package fixture paths
	if err != nil {
		t.Fatalf("read test contract: %v", err)
	}
	const valid = "[artifacts]\nnon_instance_dirs = [\"System/templates\"]\n"
	modified := strings.Replace(string(data), valid, section, 1)
	if modified == string(data) {
		t.Fatal("artifact section replacement did not apply")
	}
	contractPath := filepath.Join(t.TempDir(), "vault-schema.toml")
	err = os.WriteFile(contractPath, []byte(modified), 0o600) // #nosec G703 -- contractPath is a fixed basename under this test's TempDir
	if err != nil {
		t.Fatalf("write test contract: %v", err)
	}
	contract, err := schema.LoadFile(contractPath)
	if err != nil {
		t.Fatalf("LoadFile(%q) = %v", contractPath, err)
	}
	return contract
}

func newLifecycle(t *testing.T, root string, contract *schema.Contract) *status.Lifecycle {
	t.Helper()
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	lifecycle, err := status.Open(reader, contract, contract.Governance())
	if err != nil {
		t.Fatalf("status.Open(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if closeErr := lifecycle.Close(); closeErr != nil {
			t.Errorf("Lifecycle.Close() error = %v", closeErr)
		}
	})
	return lifecycle
}

// newVault creates a temp git repo standing in for the vault, with a fake
// local git identity scoped to that repo only. The flip's commit author
// must come from this config — never anything yomihon hardcodes —
// so tests exercise the real git behavior instead of mocking it.
// commit.gpgsign is forced off so the test does not depend on whatever the
// host's global git config or GPG agent happens to be set up to do.
func newVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	initVault(t, root)
	return root
}

func initVault(t *testing.T, root string) {
	t.Helper()
	runGit(t, root, "init", "--initial-branch=main")
	runGit(t, root, "config", "user.name", "Test Vault")
	runGit(t, root, "config", "user.email", "test-vault@example.invalid")
	runGit(t, root, "config", "commit.gpgsign", "false")
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	fullArgs := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(t.Context(), "git", fullArgs...) // #nosec G204 -- fixed test-controlled git invocation; args never shell-interpreted
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func writeNote(t *testing.T, root, content string) {
	t.Helper()
	writeVaultFile(t, root, testRel, content)
}

func writeVaultFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func readNote(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(testRel))) // #nosec G304 -- testRel is a fixed in-package constant, not external input
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(b)
}

func commitAll(t *testing.T, root string) {
	t.Helper()
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "seed lesson")
}

func commitCount(t *testing.T, root string) int {
	t.Helper()
	out := strings.TrimSpace(runGit(t, root, "log", "--oneline"))
	if out == "" {
		return 0
	}
	return len(strings.Split(out, "\n"))
}

// lessonContent is a minimal, legal lesson note with a single status line.
func lessonContent(noteStatus string) string {
	return "---\n" +
		"title: L05\n" +
		"type: lesson\n" +
		"domain: japanese\n" +
		"status: " + noteStatus + "\n" +
		"created: 2026-06-01\n" +
		"updated: 2026-06-01\n" +
		"---\n" +
		"\nbody\n"
}

func TestFlipHappyPath(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	lifecycle := newLifecycle(t, root, loadContract(t))

	original := "---\n" +
		"title: L05 助詞の使い方\n" +
		"type: lesson\n" +
		"domain: japanese\n" +
		"status: draft\n" +
		"based_on: \"[[大家的日本語 第5課]]\"\n" +
		"# hand-written note, must survive verbatim\n" +
		"created: 2026-06-01\n" +
		"updated: 2026-06-15\n" +
		"---\n" +
		"\n" +
		"<ruby>今日<rt>きょう</rt></ruby>は<ruby>晴<rt>は</rt></ruby>れ。\n"
	writeNote(t, root, original)
	commitAll(t, root)
	before := commitCount(t, root)

	if err := lifecycle.Flip(t.Context(), testRel, "draft", schema.SealStatus); err != nil {
		t.Fatalf("Flip() = %v, want nil", err)
	}

	// The single highest-stakes assertion in this feature: everything but
	// the status line's content must be byte-identical.
	want := strings.Replace(original, "status: draft", "status: "+schema.SealStatus, 1)
	got := readNote(t, root)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("file mismatch after flip (-want +got):\n%s", diff)
	}

	if after := commitCount(t, root); after != before+1 {
		t.Fatalf("commit count = %d, want %d (exactly one new commit)", after, before+1)
	}

	wantSubject := "status(" + testRel + "): draft → " + schema.SealStatus + " (via yomihon)"
	if got := strings.TrimSpace(runGit(t, root, "log", "-1", "--format=%s")); got != wantSubject {
		t.Errorf("commit subject = %q, want %q", got, wantSubject)
	}

	wantName := strings.TrimSpace(runGit(t, root, "config", "user.name"))
	wantEmail := strings.TrimSpace(runGit(t, root, "config", "user.email"))
	gotName := strings.TrimSpace(runGit(t, root, "log", "-1", "--format=%an"))
	gotEmail := strings.TrimSpace(runGit(t, root, "log", "-1", "--format=%ae"))
	if gotName != wantName || gotEmail != wantEmail {
		t.Errorf("commit author = %q <%s>, want the vault's own git config %q <%s>", gotName, gotEmail, wantName, wantEmail)
	}

	if porcelain := strings.TrimSpace(runGit(t, root, "status", "--porcelain")); porcelain != "" {
		t.Errorf("git status --porcelain not empty after flip: %q", porcelain)
	}
}

func TestOpenRejectsAReplacementOfTheReadersRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	moved := root + "-selected"
	if err = os.Rename(root, moved); err != nil {
		t.Fatalf("rename selected vault: %v", err)
	}
	t.Cleanup(func() {
		if removeErr := os.RemoveAll(moved); removeErr != nil {
			t.Errorf("remove moved vault: %v", removeErr)
		}
	})
	if err = os.Mkdir(root, 0o750); err != nil {
		t.Fatalf("create replacement vault: %v", err)
	}

	lifecycle, err := status.Open(reader, nil, schema.Ungoverned())
	if lifecycle != nil {
		t.Cleanup(func() {
			if closeErr := lifecycle.Close(); closeErr != nil {
				t.Errorf("Lifecycle.Close() error = %v", closeErr)
			}
		})
	}
	if err == nil || !strings.Contains(err.Error(), "vault root changed") {
		t.Fatalf("status.Open(replaced root) = (%v, %v), want nil and root-changed error", lifecycle, err)
	}
}

func TestFlipKeepsFileAndGitCommitInTheSelectedRootAfterPathReplacement(t *testing.T) {
	t.Parallel()

	root := newVault(t)
	original := lessonContent("draft")
	writeNote(t, root, original)
	commitAll(t, root)
	lifecycle := newLifecycle(t, root, loadContract(t))

	moved := root + "-selected"
	if err := os.Rename(root, moved); err != nil {
		t.Fatalf("rename selected vault: %v", err)
	}
	t.Cleanup(func() {
		if removeErr := os.RemoveAll(moved); removeErr != nil {
			t.Errorf("remove moved vault: %v", removeErr)
		}
	})
	if err := os.Mkdir(root, 0o750); err != nil {
		t.Fatalf("create replacement vault: %v", err)
	}
	initVault(t, root)
	writeNote(t, root, original)
	commitAll(t, root)

	if err := lifecycle.Flip(t.Context(), testRel, "draft", schema.SealStatus); err != nil {
		t.Fatalf("Flip() after top-level replacement = %v, want nil", err)
	}
	wantSelected := strings.Replace(original, "status: draft", "status: "+schema.SealStatus, 1)
	selected, err := os.ReadFile(filepath.Join(moved, filepath.FromSlash(testRel))) // #nosec G304 -- moved is a test-owned temporary directory
	if err != nil {
		t.Fatalf("read selected note: %v", err)
	}
	if string(selected) != wantSelected {
		t.Errorf("selected note = %q, want %q", selected, wantSelected)
	}
	replacement, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(testRel))) // #nosec G304 -- root is a test-owned temporary directory
	if err != nil {
		t.Fatalf("read replacement note: %v", err)
	}
	if string(replacement) != original {
		t.Errorf("replacement note = %q, want untouched %q", replacement, original)
	}
	if got := commitCount(t, moved); got != 2 {
		t.Errorf("selected commit count = %d, want 2", got)
	}
	if got := commitCount(t, root); got != 1 {
		t.Errorf("replacement commit count = %d, want 1", got)
	}
}

func TestLastCommitHashReadsSelectedHistoryAfterPathReplacement(t *testing.T) {
	t.Parallel()

	root := newVault(t)
	content := lessonContent("draft")
	writeNote(t, root, content)
	commitAll(t, root)
	want := strings.TrimSpace(runGit(t, root, "rev-parse", "--short", "HEAD"))
	lifecycle := newLifecycle(t, root, loadContract(t))

	moved := root + "-selected"
	if err := os.Rename(root, moved); err != nil {
		t.Fatalf("rename selected vault: %v", err)
	}
	t.Cleanup(func() {
		if removeErr := os.RemoveAll(moved); removeErr != nil {
			t.Errorf("remove moved vault: %v", removeErr)
		}
	})
	if err := os.Mkdir(root, 0o750); err != nil {
		t.Fatalf("create replacement vault: %v", err)
	}
	initVault(t, root)
	writeNote(t, root, content)
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "replacement history")
	replacementHash := strings.TrimSpace(runGit(t, root, "rev-parse", "--short", "HEAD"))
	if replacementHash == want {
		t.Fatal("replacement history accidentally has the selected commit hash")
	}

	got, err := lifecycle.LastCommitHash(t.Context(), testRel, sha256.Sum256([]byte(content)))
	if err != nil {
		t.Fatalf("LastCommitHash() after top-level replacement = %v", err)
	}
	if got != want {
		t.Errorf("LastCommitHash() = %q, want selected history %q, not replacement %q", got, want, replacementHash)
	}
}

func TestFlipStripsGitRepositoryRedirectionEnvironment(t *testing.T) {
	root := newVault(t)
	original := lessonContent("draft")
	writeNote(t, root, original)
	commitAll(t, root)
	lifecycle := newLifecycle(t, root, loadContract(t))
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "redirected.git"))
	t.Setenv("GIT_WORK_TREE", t.TempDir())

	if err := lifecycle.Flip(t.Context(), testRel, "draft", schema.SealStatus); err != nil {
		t.Fatalf("Flip() with repository-redirection environment = %v, want nil", err)
	}
	want := strings.Replace(original, "status: draft", "status: "+schema.SealStatus, 1)
	if got := readNote(t, root); got != want {
		t.Errorf("note after Flip() = %q, want %q", got, want)
	}
}

func TestFlipDoesNotExposeApplicationEnvironmentToGitHooks(t *testing.T) {
	root := newVault(t)
	original := lessonContent("draft")
	writeNote(t, root, original)
	commitAll(t, root)
	lifecycle := newLifecycle(t, root, loadContract(t))

	hook := filepath.Join(root, ".git", "hooks", "pre-commit")
	const script = `#!/bin/sh
if env | grep -q '^YOMIHON_'; then
	exit 91
fi
`
	if err := os.WriteFile(hook, []byte(script), 0o700); err != nil { // #nosec G306 -- git executes this test-owned pre-commit fixture, so it must have an execute bit
		t.Fatalf("write pre-commit hook: %v", err)
	}
	t.Setenv("YOMIHON_EMBED_KEY", "must-not-reach-git")
	t.Setenv("YOMIHON_ROOT", "/must/not/reach/git")
	t.Setenv("YOMIHON_PORT", "65535")
	t.Setenv("YOMIHON_FUTURE_SECRET", "must-also-stay-private")

	if err := lifecycle.Flip(t.Context(), testRel, "draft", schema.SealStatus); err != nil {
		t.Fatalf("Flip() with application secrets in the parent environment = %v, want nil", err)
	}
}

func TestGitChildFailureBeforePublicationLeavesSourceUntouched(t *testing.T) {
	root := newVault(t)
	original := lessonContent("draft")
	writeNote(t, root, original)
	commitAll(t, root)
	lifecycle := newLifecycle(t, root, loadContract(t))
	t.Setenv("PATH", t.TempDir())

	err := lifecycle.Flip(t.Context(), testRel, "draft", schema.SealStatus)
	if err == nil || errors.Is(err, status.ErrCommitFailed) {
		t.Fatalf("Flip() without git executable = %v, want pre-publication failure not wrapping ErrCommitFailed", err)
	}
	if got := readNote(t, root); got != original {
		t.Errorf("note after pre-publication child failure = %q, want untouched %q", got, original)
	}
}

func TestFlipCommitsOnlyTarget(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	lifecycle := newLifecycle(t, root, loadContract(t))

	original := lessonContent("draft")
	writeNote(t, root, original)
	otherRel := "Writing/notes/Unrelated.md"
	otherPath := filepath.Join(root, filepath.FromSlash(otherRel))
	if err := os.MkdirAll(filepath.Dir(otherPath), 0o750); err != nil {
		t.Fatalf("mkdir unrelated parent: %v", err)
	}
	if err := os.WriteFile(otherPath, []byte("original\n"), 0o600); err != nil {
		t.Fatalf("write unrelated seed: %v", err)
	}
	commitAll(t, root)

	wantStaged := "staged but unrelated\n"
	if err := os.WriteFile(otherPath, []byte(wantStaged), 0o600); err != nil {
		t.Fatalf("write unrelated change: %v", err)
	}
	runGit(t, root, "add", "--", otherRel)
	stagedBefore := runGit(t, root, "show", ":"+otherRel)

	if err := lifecycle.Flip(t.Context(), testRel, "draft", schema.SealStatus); err != nil {
		t.Fatalf("Flip() = %v, want nil", err)
	}

	changed := strings.Fields(runGit(t, root, "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD"))
	if diff := cmp.Diff([]string{testRel}, changed); diff != "" {
		t.Errorf("HEAD changed paths mismatch (-want +got):\n%s", diff)
	}
	staged := strings.Fields(runGit(t, root, "diff", "--cached", "--name-only"))
	if diff := cmp.Diff([]string{otherRel}, staged); diff != "" {
		t.Errorf("staged paths mismatch (-want +got):\n%s", diff)
	}
	stagedAfter := runGit(t, root, "show", ":"+otherRel)
	if diff := cmp.Diff(stagedBefore, stagedAfter); diff != "" {
		t.Errorf("unrelated staged bytes changed (-want +got):\n%s", diff)
	}
	if got := stagedAfter; got != wantStaged {
		t.Errorf("unrelated staged bytes = %q, want %q", got, wantStaged)
	}

	wantNote := strings.Replace(original, "status: draft", "status: "+schema.SealStatus, 1)
	if got := readNote(t, root); got != wantNote {
		t.Errorf("note after flip = %q, want %q", got, wantNote)
	}
	wantSubject := "status(" + testRel + "): draft → " + schema.SealStatus + " (via yomihon)"
	if got := strings.TrimSpace(runGit(t, root, "log", "-1", "--format=%s")); got != wantSubject {
		t.Errorf("commit subject = %q, want %q", got, wantSubject)
	}
}

func TestFlipTreatsGitPathspecMetacharactersLiterally(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	lifecycle := newLifecycle(t, root, loadContract(t))

	targetRel := "Writing/lessons/japanese/L*.md"
	stagedRel := "Writing/lessons/japanese/L05.md"
	unstagedRel := "Writing/lessons/japanese/L06.md"
	original := lessonContent("draft")
	writeVaultFile(t, root, targetRel, original)
	writeVaultFile(t, root, stagedRel, "staged seed\n")
	writeVaultFile(t, root, unstagedRel, "unstaged seed\n")
	commitAll(t, root)

	wantStaged := "staged sibling change\n"
	writeVaultFile(t, root, stagedRel, wantStaged)
	runGit(t, root, "add", "--", stagedRel)
	stagedBefore := runGit(t, root, "show", ":"+stagedRel)
	wantUnstaged := "unstaged sibling change\n"
	writeVaultFile(t, root, unstagedRel, wantUnstaged)

	if err := lifecycle.Flip(t.Context(), targetRel, "draft", schema.SealStatus); err != nil {
		t.Fatalf("Flip(%q) = %v, want nil", targetRel, err)
	}

	changed := strings.Fields(runGit(t, root, "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD"))
	if diff := cmp.Diff([]string{targetRel}, changed); diff != "" {
		t.Errorf("HEAD changed paths mismatch (-want +got):\n%s", diff)
	}
	staged := strings.Fields(runGit(t, root, "diff", "--cached", "--name-only"))
	if diff := cmp.Diff([]string{stagedRel}, staged); diff != "" {
		t.Errorf("staged paths mismatch (-want +got):\n%s", diff)
	}
	stagedAfter := runGit(t, root, "show", ":"+stagedRel)
	if diff := cmp.Diff(stagedBefore, stagedAfter); diff != "" {
		t.Errorf("sibling staged bytes changed (-want +got):\n%s", diff)
	}
	if got := stagedAfter; got != wantStaged {
		t.Errorf("sibling staged bytes = %q, want %q", got, wantStaged)
	}
	unstaged := strings.Fields(runGit(t, root, "diff", "--name-only"))
	if diff := cmp.Diff([]string{unstagedRel}, unstaged); diff != "" {
		t.Errorf("unstaged paths mismatch (-want +got):\n%s", diff)
	}
	unstagedBytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(unstagedRel))) // #nosec G304 -- path is test-owned under t.TempDir
	if err != nil {
		t.Fatalf("read unstaged sibling: %v", err)
	}
	if got := string(unstagedBytes); got != wantUnstaged {
		t.Errorf("unstaged sibling bytes = %q, want %q", got, wantUnstaged)
	}

	wantNote := strings.Replace(original, "status: draft", "status: "+schema.SealStatus, 1)
	targetBytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(targetRel))) // #nosec G304 -- path is test-owned under t.TempDir
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if got := string(targetBytes); got != wantNote {
		t.Errorf("target after flip = %q, want %q", got, wantNote)
	}
	targetCommit := strings.TrimSpace(runGit(t, root, "rev-parse", "--short", "HEAD"))
	wantSubject := "status(" + targetRel + "): draft → " + schema.SealStatus + " (via yomihon)"
	if got := strings.TrimSpace(runGit(t, root, "log", "-1", "--format=%s")); got != wantSubject {
		t.Errorf("commit subject = %q, want %q", got, wantSubject)
	}

	runGit(t, root, "commit", "--only", "-m", "update sibling", "--", stagedRel)
	gotCommit, err := lifecycle.LastCommitHash(t.Context(), targetRel, sha256.Sum256(targetBytes))
	if err != nil {
		t.Fatalf("LastCommitHash(%q) = %v", targetRel, err)
	}
	if gotCommit != targetCommit {
		t.Errorf("LastCommitHash(%q) = %q, want %q", targetRel, gotCommit, targetCommit)
	}
}

func TestLastCommitHashOmitsUncommittedState(t *testing.T) {
	t.Parallel()

	root := newVault(t)
	lifecycle := newLifecycle(t, root, loadContract(t))
	writeNote(t, root, lessonContent("draft"))
	commitAll(t, root)

	writeNote(t, root, lessonContent(schema.SealStatus))
	got, err := lifecycle.LastCommitHash(
		t.Context(),
		testRel,
		sha256.Sum256([]byte(lessonContent(schema.SealStatus))),
	)
	if err != nil {
		t.Fatalf("LastCommitHash(%q) = %v", testRel, err)
	}
	if got != "" {
		t.Errorf("LastCommitHash(%q) = %q for uncommitted bytes, want empty", testRel, got)
	}
}

func TestLastCommitHashBindsToTheReadBytes(t *testing.T) {
	t.Parallel()

	root := newVault(t)
	lifecycle := newLifecycle(t, root, loadContract(t))
	readBytes := []byte(lessonContent(schema.SealStatus))
	writeNote(t, root, string(readBytes))
	commitAll(t, root)

	writeNote(t, root, lessonContent(schema.SealStatus)+"\nnewer body\n")
	commitAll(t, root)

	got, err := lifecycle.LastCommitHash(t.Context(), testRel, sha256.Sum256(readBytes))
	if err != nil {
		t.Fatalf("LastCommitHash(%q) = %v", testRel, err)
	}
	if got != "" {
		t.Errorf("LastCommitHash(%q) = %q for bytes from an older reading snapshot, want empty", testRel, got)
	}
}

func TestFlipRefusesSymlinkTarget(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	lifecycle := newLifecycle(t, root, loadContract(t))

	outside := filepath.Join(t.TempDir(), "outside.md")
	original := lessonContent("draft")
	if err := os.WriteFile(outside, []byte(original), 0o600); err != nil {
		t.Fatalf("write outside target: %v", err)
	}
	notePath := filepath.Join(root, filepath.FromSlash(testRel))
	if err := os.MkdirAll(filepath.Dir(notePath), 0o750); err != nil {
		t.Fatalf("mkdir note parent: %v", err)
	}
	if err := os.Symlink(outside, notePath); err != nil {
		t.Fatalf("symlink note: %v", err)
	}
	commitAll(t, root)
	beforeCommits := commitCount(t, root)

	err := lifecycle.Flip(t.Context(), testRel, "draft", schema.SealStatus)
	if err == nil {
		t.Fatal("Flip(symlink) = nil, want refusal")
	}
	got, readErr := os.ReadFile(outside) // #nosec G304 -- outside is a test-owned path under t.TempDir
	if readErr != nil {
		t.Fatalf("read outside target: %v", readErr)
	}
	if diff := cmp.Diff(original, string(got)); diff != "" {
		t.Errorf("outside target changed (-want +got):\n%s", diff)
	}
	info, statErr := os.Lstat(notePath)
	if statErr != nil {
		t.Fatalf("lstat note: %v", statErr)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("note mode = %v, want symlink preserved", info.Mode())
	}
	if after := commitCount(t, root); after != beforeCommits {
		t.Errorf("commit count = %d, want unchanged %d", after, beforeCommits)
	}
}

func TestFlipRefusesSymlinkDirectory(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	lifecycle := newLifecycle(t, root, loadContract(t))

	outsideWriting := filepath.Join(t.TempDir(), "Writing")
	notePath := filepath.Join(outsideWriting, "lessons", "japanese", "L05.md")
	if err := os.MkdirAll(filepath.Dir(notePath), 0o750); err != nil {
		t.Fatalf("mkdir outside note parent: %v", err)
	}
	original := lessonContent("draft")
	if err := os.WriteFile(notePath, []byte(original), 0o600); err != nil {
		t.Fatalf("write outside target: %v", err)
	}
	if err := os.Symlink(outsideWriting, filepath.Join(root, "Writing")); err != nil {
		t.Fatalf("symlink Writing: %v", err)
	}
	commitAll(t, root)
	beforeCommits := commitCount(t, root)

	err := lifecycle.Flip(t.Context(), testRel, "draft", schema.SealStatus)
	if err == nil {
		t.Fatal("Flip(path through symlink directory) = nil, want refusal")
	}
	got, readErr := os.ReadFile(notePath) // #nosec G304 -- notePath is a test-owned path under t.TempDir
	if readErr != nil {
		t.Fatalf("read outside target: %v", readErr)
	}
	if diff := cmp.Diff(original, string(got)); diff != "" {
		t.Errorf("outside target changed (-want +got):\n%s", diff)
	}
	if after := commitCount(t, root); after != beforeCommits {
		t.Errorf("commit count = %d, want unchanged %d", after, beforeCommits)
	}
}

func TestFlipRefusesNonRegularTarget(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	lifecycle := newLifecycle(t, root, loadContract(t))
	notePath := filepath.Join(root, filepath.FromSlash(testRel))
	if err := os.MkdirAll(notePath, 0o750); err != nil {
		t.Fatalf("mkdir target directory: %v", err)
	}
	markerPath := filepath.Join(notePath, "marker")
	if err := os.WriteFile(markerPath, []byte("unchanged"), 0o600); err != nil {
		t.Fatalf("write directory marker: %v", err)
	}
	commitAll(t, root)
	beforeCommits := commitCount(t, root)

	err := lifecycle.Flip(t.Context(), testRel, "draft", schema.SealStatus)
	if err == nil {
		t.Fatal("Flip(directory) = nil, want refusal")
	}
	info, statErr := os.Lstat(notePath)
	if statErr != nil {
		t.Fatalf("lstat target: %v", statErr)
	}
	if !info.IsDir() {
		t.Errorf("target mode = %v, want directory preserved", info.Mode())
	}
	marker, readErr := os.ReadFile(markerPath) // #nosec G304 -- markerPath is a test-owned path under t.TempDir
	if readErr != nil {
		t.Fatalf("read directory marker: %v", readErr)
	}
	if got, want := string(marker), "unchanged"; got != want {
		t.Errorf("marker bytes = %q, want %q", got, want)
	}
	if after := commitCount(t, root); after != beforeCommits {
		t.Errorf("commit count = %d, want unchanged %d", after, beforeCommits)
	}
}

func TestFlipClassifiesNonInstanceBeforeFilesystemAndGit(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	contract := loadContract(t)
	lifecycle := newLifecycle(t, root, contract)

	nonInstanceRel := "System/templates/Loud lesson.md"
	path := filepath.Join(root, filepath.FromSlash(nonInstanceRel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir template directory: %v", err)
	}
	original := lessonContent("draft")
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write template note: %v", err)
	}
	commitAll(t, root)
	beforeCommits := commitCount(t, root)

	err := lifecycle.Flip(t.Context(), nonInstanceRel, "draft", schema.SealStatus)
	if !errors.Is(err, status.ErrNonInstance) {
		t.Fatalf("Flip(non-instance) = %v, want %v", err, status.ErrNonInstance)
	}
	got, readErr := os.ReadFile(path) // #nosec G304 -- path is a fixed test-relative file under newVault's TempDir
	if readErr != nil {
		t.Fatalf("read template note: %v", readErr)
	}
	if diff := cmp.Diff(original, string(got)); diff != "" {
		t.Errorf("non-instance bytes changed (-want +got):\n%s", diff)
	}
	if after := commitCount(t, root); after != beforeCommits {
		t.Errorf("commit count = %d, want unchanged %d", after, beforeCommits)
	}

	err = lifecycle.Flip(t.Context(), "System/templates/Missing.md", "draft", schema.SealStatus)
	if !errors.Is(err, status.ErrNonInstance) {
		t.Errorf("Flip(nonexistent non-instance) = %v, want %v before stat", err, status.ErrNonInstance)
	}
	err = lifecycle.Flip(t.Context(), "System/temporary/../templates/Normalized.md", "draft", schema.SealStatus)
	if !errors.Is(err, status.ErrNonInstance) {
		t.Errorf("Flip(normalized non-instance) = %v, want %v before stat", err, status.ErrNonInstance)
	}

	err = lifecycle.Flip(t.Context(), "System/templates-old/Missing.md", "draft", schema.SealStatus)
	if errors.Is(err, status.ErrNonInstance) {
		t.Errorf("Flip(component-boundary sibling) = %v, must reach filesystem instead of non-instance gate", err)
	}
}

// TestFlipRefusesNonMarkdownTarget locks the write face to the same note
// definition the reading scan uses: a note is a file whose path ends in
// ".md", and everything else is a resource. A resource carrying note-shaped
// frontmatter and a legal transition must not receive a committed status
// flip — that would mint a note-lifecycle receipt for a file the reading
// face itself refuses to offer a write form for.
func TestFlipRefusesNonMarkdownTarget(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	lifecycle := newLifecycle(t, root, loadContract(t))

	const txtRel = "Writing/lessons/japanese/L05.txt"
	original := lessonContent("draft")
	writeVaultFile(t, root, txtRel, original)
	commitAll(t, root)
	before := commitCount(t, root)

	err := lifecycle.Flip(t.Context(), txtRel, "draft", schema.SealStatus)
	if !errors.Is(err, status.ErrNonInstance) {
		t.Fatalf("Flip(%q) = %v, want %v", txtRel, err, status.ErrNonInstance)
	}
	got, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(txtRel))) // #nosec G304 -- txtRel is a fixed in-test constant under newVault's TempDir
	if readErr != nil {
		t.Fatalf("read resource: %v", readErr)
	}
	if diff := cmp.Diff(original, string(got)); diff != "" {
		t.Errorf("resource bytes changed (-want +got):\n%s", diff)
	}
	if after := commitCount(t, root); after != before {
		t.Errorf("commit count = %d, want unchanged %d (no commit created)", after, before)
	}
}

func TestFlipValidatesPathBeforeClosure(t *testing.T) {
	t.Parallel()
	lifecycle := newLifecycle(t, t.TempDir(), nil)
	for _, rel := range []string{"", ".", "..", "../outside.md", "/absolute.md", `System\templates\T.md`} {
		err := lifecycle.Flip(t.Context(), rel, "draft", schema.SealStatus)
		if !errors.Is(err, status.ErrInvalidPath) {
			t.Errorf("Flip(%q on closed service) = %v, want %v", rel, err, status.ErrInvalidPath)
		}
		if errors.Is(err, status.ErrClosed) {
			t.Errorf("Flip(%q on closed service) = %v, path validation must precede closure", rel, err)
		}
	}
}

func TestArtifactPolicyClosureIsDistinct(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		contract *schema.Contract
		want     string
	}{
		{name: "missing", contract: loadFixtureWithArtifactSection(t, filepath.Join("..", "schema", "testdata", "contract.toml"), ""), want: "contract declares no artifact policy; instance projections disabled until it does"},
		{name: "invalid", contract: loadFixtureWithArtifactSection(t, filepath.Join("..", "schema", "testdata", "contract.toml"), "[artifacts]\nnon_instance_dirs = [\".\"]\n"), want: `invalid artifact policy: non_instance_dirs contains "."`},
		{name: "incomplete", contract: loadFixtureWithArtifactSection(t, filepath.Join("..", "schema", "testdata", "contract.toml"), "[artifacts]\n"), want: `invalid artifact policy: missing required key "non_instance_dirs"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lifecycle := newLifecycle(t, t.TempDir(), tt.contract)
			if !lifecycle.View().Closed() {
				t.Fatal("Closed() = false, want artifact-policy closure")
			}
			if got, want := lifecycle.View().Diagnostic(), tt.want; got != want {
				t.Errorf("WriteDiagnostic() = %q, want %q", got, want)
			}
			if got := lifecycle.View().Order(); got != nil {
				t.Errorf("Order() = %v while artifact policy closes instance projections, want nil", got)
			}
			if lifecycle.View().Advanceable("lesson", "draft") {
				t.Error("Advanceable() = true while artifact policy closes writes")
			}
			err := lifecycle.Flip(t.Context(), testRel, "draft", schema.SealStatus)
			if !errors.Is(err, status.ErrArtifactPolicyUnavailable) {
				t.Errorf("Flip() = %v, want %v", err, status.ErrArtifactPolicyUnavailable)
			}
			if got, want := err.Error(), tt.want; got != want {
				t.Errorf("Flip() error = %q, want exact diagnostic %q", got, want)
			}
		})
	}
}

func TestFlipRefusals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		onDiskStatus string // status committed into the note before flipping
		from, to     string
		dirtyEdit    bool
		wantErr      error
	}{
		{
			name:         "stale form: from does not match actual current status",
			onDiskStatus: "draft",
			from:         "imported",
			to:           schema.SealStatus,
			wantErr:      status.ErrStale,
		},
		{
			name:         "illegal transition: skips a required intermediate status",
			onDiskStatus: "imported",
			from:         "imported",
			to:           schema.SealStatus,
			wantErr:      schema.ErrIllegalTransition,
		},
		{
			name:         "dirty file: unrelated uncommitted edit already present",
			onDiskStatus: "draft",
			from:         "draft",
			to:           schema.SealStatus,
			dirtyEdit:    true,
			wantErr:      status.ErrDirty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := newVault(t)
			lifecycle := newLifecycle(t, root, loadContract(t))

			committed := lessonContent(tt.onDiskStatus)
			writeNote(t, root, committed)
			commitAll(t, root)
			before := commitCount(t, root)

			onDisk := committed
			if tt.dirtyEdit {
				onDisk = committed + "<!-- an unrelated, uncommitted edit -->\n"
				writeNote(t, root, onDisk)
			}

			err := lifecycle.Flip(t.Context(), testRel, tt.from, tt.to)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Flip() = %v, want %v", err, tt.wantErr)
			}
			if got := readNote(t, root); got != onDisk {
				t.Errorf("file was touched:\ngot:  %q\nwant: %q", got, onDisk)
			}
			if after := commitCount(t, root); after != before {
				t.Errorf("commit count = %d, want unchanged %d (no commit created)", after, before)
			}
		})
	}
}

// writeHook installs an executable repo-local git hook in the fixture vault.
// Hooks are how a vault owner's own tooling participates in commits; these
// tests use them to make the created commit diverge from the intended change.
func writeHook(t *testing.T, root, name, script string) {
	t.Helper()
	hook := filepath.Join(root, ".git", "hooks", name)
	if err := os.WriteFile(hook, []byte(script), 0o700); err != nil { // #nosec G306 -- git executes this test-owned hook fixture, so it must keep an execute bit
		t.Fatalf("write %s hook: %v", name, err)
	}
}

// TestFlipReportsReceiptDivergenceWhenHookEditsTheNote covers a pre-commit
// hook that edits the note and restages it: the hook writes into the partial
// commit's temporary index, so the created commit carries bytes the flip
// never published while its subject still claims a clean status transition.
func TestFlipReportsReceiptDivergenceWhenHookEditsTheNote(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	lifecycle := newLifecycle(t, root, loadContract(t))
	writeNote(t, root, lessonContent("draft"))
	commitAll(t, root)
	writeHook(t, root, "pre-commit",
		"#!/bin/sh\nprintf 'INJECTED-BY-HOOK\\n' >> '"+testRel+"'\ngit add -- '"+testRel+"'\n")

	err := lifecycle.Flip(t.Context(), testRel, "draft", schema.SealStatus)
	if !errors.Is(err, status.ErrReceiptDiverged) {
		t.Fatalf("Flip() with a note-editing pre-commit hook = %v, want %v", err, status.ErrReceiptDiverged)
	}
	// No rollback: the diverged commit stays, and its blob shows what the
	// hook actually committed.
	if blob := runGit(t, root, "cat-file", "blob", "HEAD:"+testRel); !strings.Contains(blob, "INJECTED-BY-HOOK") {
		t.Errorf("committed blob = %q, want it to carry the hook's injected line", blob)
	}
}

// TestFlipReportsReceiptDivergenceWhenHookStagesAnotherFile covers a
// pre-commit hook that creates and stages an unrelated file: the --only
// pathspec confines what the flip stages, but a hook staging into the
// partial commit's temporary index widens the commit anyway.
func TestFlipReportsReceiptDivergenceWhenHookStagesAnotherFile(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	lifecycle := newLifecycle(t, root, loadContract(t))
	writeNote(t, root, lessonContent("draft"))
	commitAll(t, root)
	writeHook(t, root, "pre-commit",
		"#!/bin/sh\nprintf 'planted by hook\\n' > Writing/hook-planted.md\ngit add -- Writing/hook-planted.md\n")

	err := lifecycle.Flip(t.Context(), testRel, "draft", schema.SealStatus)
	if !errors.Is(err, status.ErrReceiptDiverged) {
		t.Fatalf("Flip() with an unrelated-file-staging pre-commit hook = %v, want %v", err, status.ErrReceiptDiverged)
	}
	changed := strings.Fields(runGit(t, root, "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD"))
	if len(changed) != 2 {
		t.Errorf("HEAD changed paths = %v, want the note plus the hook's file (the divergence being reported)", changed)
	}
}

// TestFlipReportsReceiptDivergenceWhenHookRewritesTheMessage covers a
// commit-msg hook replacing the subject line wholesale: the note's bytes are
// committed as intended, but the transition is recorded nowhere.
func TestFlipReportsReceiptDivergenceWhenHookRewritesTheMessage(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	lifecycle := newLifecycle(t, root, loadContract(t))
	writeNote(t, root, lessonContent("draft"))
	commitAll(t, root)
	writeHook(t, root, "commit-msg",
		"#!/bin/sh\nprintf 'chore: routine maintenance\\n' > \"$1\"\n")

	err := lifecycle.Flip(t.Context(), testRel, "draft", schema.SealStatus)
	if !errors.Is(err, status.ErrReceiptDiverged) {
		t.Fatalf("Flip() with a message-rewriting commit-msg hook = %v, want %v", err, status.ErrReceiptDiverged)
	}
	if got := strings.TrimSpace(runGit(t, root, "log", "-1", "--format=%s")); got != "chore: routine maintenance" {
		t.Errorf("commit subject = %q, want the hook's replacement (the divergence being reported)", got)
	}
}

// TestFlipRefusesDetachedHead locks the write face's refusal to commit while
// the vault's HEAD names no branch. git itself accepts such a commit, but it
// lands on no ref: the moment the operator checks a branch out again, the
// note reverts and the receipt is reachable only through the reflog — a
// success report whose evidence quietly evaporates.
func TestFlipRefusesDetachedHead(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	lifecycle := newLifecycle(t, root, loadContract(t))

	original := lessonContent("draft")
	writeNote(t, root, original)
	commitAll(t, root)
	runGit(t, root, "checkout", "--detach")
	before := commitCount(t, root)

	err := lifecycle.Flip(t.Context(), testRel, "draft", schema.SealStatus)
	if !errors.Is(err, status.ErrDetachedHead) {
		t.Fatalf("Flip() on a detached HEAD = %v, want %v", err, status.ErrDetachedHead)
	}
	if got := readNote(t, root); got != original {
		t.Errorf("note after detached-HEAD refusal = %q, want untouched %q", got, original)
	}
	if after := commitCount(t, root); after != before {
		t.Errorf("commit count = %d, want unchanged %d (no commit on a detached HEAD)", after, before)
	}
	if porcelain := strings.TrimSpace(runGit(t, root, "status", "--porcelain")); porcelain != "" {
		t.Errorf("git status --porcelain not empty after refusal: %q", porcelain)
	}
}

func TestFlipMalformedStatusLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		wantErr error
	}{
		{
			name: "zero status lines",
			content: "---\n" +
				"title: L05\n" +
				"type: lesson\n" +
				"created: 2026-06-01\n" +
				"---\n" +
				"\nbody\n",
			wantErr: status.ErrStatusLine,
		},
		{
			name: "two status lines",
			content: "---\n" +
				"title: L05\n" +
				"type: lesson\n" +
				"status: draft\n" +
				"status: archived\n" +
				"---\n" +
				"\nbody\n",
			wantErr: schema.ErrUnknownStatus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := newVault(t)
			lifecycle := newLifecycle(t, root, loadContract(t))

			writeNote(t, root, tt.content)
			commitAll(t, root)
			before := commitCount(t, root)

			// A missing status retains the declared note type, so the wildcard
			// archived transition reaches the surgical rewrite and reports the
			// malformed line count. Duplicate YAML keys invalidate the parsed
			// frontmatter, including its type. Lifecycle validation therefore
			// fails closed before the surgical rewrite.
			err := lifecycle.Flip(t.Context(), testRel, "", "archived")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Flip() = %v, want %v", err, tt.wantErr)
			}
			if got := readNote(t, root); got != tt.content {
				t.Errorf("file was touched:\ngot:  %q\nwant: %q", got, tt.content)
			}
			if after := commitCount(t, root); after != before {
				t.Errorf("commit count = %d, want unchanged %d (no commit created)", after, before)
			}
		})
	}
}

// TestFlipRefusesStatusAnchorThatRewritingWouldSever covers the one
// frontmatter shape where the byte-level rewrite used to corrupt a readable
// note: a status value carrying a YAML anchor that another field aliases.
// Replacing the whole status line deletes the anchor definition, the alias
// dangles, and the note stops parsing — committed as a clean transition.
// The flip must refuse before writing anything.
func TestFlipRefusesStatusAnchorThatRewritingWouldSever(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	lifecycle := newLifecycle(t, root, loadContract(t))

	content := "---\n" +
		"title: L05\n" +
		"type: lesson\n" +
		"status: &s draft\n" +
		"domain: *s\n" +
		"created: 2026-06-01\n" +
		"updated: 2026-06-01\n" +
		"---\n" +
		"\nbody\n"
	writeNote(t, root, content)
	commitAll(t, root)
	before := commitCount(t, root)

	observed, err := lifecycle.ObservedStatus(t.Context(), testRel)
	if err != nil || observed.Status != "draft" {
		t.Fatalf("ObservedStatus() = (%+v, %v), want the reader to see draft", observed, err)
	}

	err = lifecycle.Flip(t.Context(), testRel, "draft", schema.SealStatus)
	if !errors.Is(err, status.ErrStatusSyntaxUnsupported) {
		t.Fatalf("Flip(status carrying an aliased anchor) = %v, want %v", err, status.ErrStatusSyntaxUnsupported)
	}
	if got := readNote(t, root); got != content {
		t.Errorf("note after refusal = %q, want untouched %q", got, content)
	}
	if after := commitCount(t, root); after != before {
		t.Errorf("commit count = %d, want unchanged %d (no commit created)", after, before)
	}
}

// TestFlipRefusesUnsupportedStatusSyntax covers frontmatter the reader parses
// fine but the surgical rewriter cannot locate: the status key written in
// explicit-key, quoted-key, space-before-colon, or flow-mapping form. These
// always failed closed, but were reported as a schema violation — a fault
// report for a note that is not at fault, pointing the human at the wrong
// thing. The refusal must name the unsupported syntax instead.
func TestFlipRefusesUnsupportedStatusSyntax(t *testing.T) {
	t.Parallel()

	block := func(statusLines string) string {
		return "---\n" +
			"title: L05\n" +
			"type: lesson\n" +
			"domain: japanese\n" +
			statusLines +
			"created: 2026-06-01\n" +
			"updated: 2026-06-01\n" +
			"---\n" +
			"\nbody\n"
	}
	tests := []struct {
		name    string
		content string
	}{
		{name: "explicit key", content: block("? status\n: draft\n")},
		{name: "double-quoted key", content: block("\"status\": draft\n")},
		{name: "single-quoted key", content: block("'status': draft\n")},
		{name: "space before colon", content: block("status : draft\n")},
		{
			name: "flow mapping",
			content: "---\n" +
				"{title: L05, type: lesson, domain: japanese, status: draft, created: 2026-06-01, updated: 2026-06-01}\n" +
				"---\n" +
				"\nbody\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := newVault(t)
			lifecycle := newLifecycle(t, root, loadContract(t))
			writeNote(t, root, tt.content)
			commitAll(t, root)
			before := commitCount(t, root)

			observed, err := lifecycle.ObservedStatus(t.Context(), testRel)
			if err != nil || observed.Status != "draft" {
				t.Fatalf("ObservedStatus() = (%+v, %v), want the reader to see draft", observed, err)
			}

			err = lifecycle.Flip(t.Context(), testRel, "draft", schema.SealStatus)
			if !errors.Is(err, status.ErrStatusSyntaxUnsupported) {
				t.Fatalf("Flip() = %v, want %v", err, status.ErrStatusSyntaxUnsupported)
			}
			if got := readNote(t, root); got != tt.content {
				t.Errorf("note after refusal = %q, want untouched %q", got, tt.content)
			}
			if after := commitCount(t, root); after != before {
				t.Errorf("commit count = %d, want unchanged %d (no commit created)", after, before)
			}
		})
	}
}

func TestFlipFailClosed(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	lifecycle := newLifecycle(t, root, nil)

	if !lifecycle.View().Closed() {
		t.Fatal("Closed() = false, want true for a nil contract")
	}
	if got := lifecycle.View().Transitions(testRel, "lesson", "draft"); got != nil {
		t.Errorf("Transitions() = %v, want nil", got)
	}

	tests := []struct {
		name          string
		rel, from, to string
	}{
		{"well-formed args", "Writing/lessons/japanese/L05.md", "draft", schema.SealStatus},
		{"garbage args", "does/not/exist.md", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := lifecycle.Flip(t.Context(), tt.rel, tt.from, tt.to); !errors.Is(err, status.ErrClosed) {
				t.Errorf("Flip(%q, %q, %q) = %v, want %v", tt.rel, tt.from, tt.to, err, status.ErrClosed)
			}
		})
	}
}

func TestClosed(t *testing.T) {
	t.Parallel()
	if !newLifecycle(t, t.TempDir(), nil).View().Closed() {
		t.Error("Closed() = false, want true for nil contract")
	}
	if newLifecycle(t, t.TempDir(), loadContract(t)).View().Closed() {
		t.Error("Closed() = true, want false for a loaded contract")
	}
}

func TestOrderDistinguishesUnavailableCoreFromEmptyGroup(t *testing.T) {
	t.Parallel()
	if got := newLifecycle(t, t.TempDir(), nil).View().Order(); got != nil {
		t.Errorf("Order() with unavailable core = %v, want nil", got)
	}
	got := newLifecycle(t, t.TempDir(), loadContractWithEmptyDefaultStatusGroup(t)).View().Order()
	if got == nil {
		t.Error("Order() with valid core and explicit empty default group = nil, want available empty slice")
	}
	if len(got) != 0 {
		t.Errorf("Order() with explicit empty default group = %v, want empty", got)
	}
}

func TestTransitions(t *testing.T) {
	t.Parallel()
	lifecycle := newLifecycle(t, t.TempDir(), loadContract(t))

	tests := []struct {
		name     string
		relPath  string
		noteType string
		current  string
		want     []string
	}{
		// Cross-checked by hand against testdata/contract.toml's lifecycle
		// table for actor "koopa":
		//   imported: from=[] owner=[claude,koopa]
		//   draft:    from=[imported] owner=[claude,koopa]
		//   ready:    from=[draft] owner=[koopa]
		//   archived: from=[*] applies_to=[*] owner=[claude,koopa]
		{name: "lesson from draft", relPath: testRel, noteType: "lesson", current: "draft", want: []string{schema.SealStatus, "archived"}},
		{name: "lesson from imported", relPath: testRel, noteType: "lesson", current: "imported", want: []string{"draft", "archived"}},
		{name: "non-instance path", relPath: "System/templates/Lesson.md", noteType: "lesson", current: "draft", want: nil},
		{name: "normalized non-instance path", relPath: "System/temporary/../templates/Lesson.md", noteType: "lesson", current: "draft", want: nil},
		{name: "component-boundary sibling remains governed", relPath: "System/templates-old/Lesson.md", noteType: "lesson", current: "draft", want: []string{schema.SealStatus, "archived"}},
		{name: "empty note type", relPath: testRel, noteType: "", current: "draft", want: nil},
		{name: "empty current status", relPath: testRel, noteType: "lesson", current: "", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := lifecycle.View().Transitions(tt.relPath, tt.noteType, tt.current)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Transitions(%q, %q, %q) mismatch (-want +got):\n%s", tt.relPath, tt.noteType, tt.current, diff)
			}
		})
	}
}

// TestAdvanceable checks that the named-onward-transition predicate is closed
// under a nil contract and, when open, asks the contract with the operator as
// the actor — so an onward step someone else owns does not count. The contract
// is synthetic (states a, b, c) to keep the test about the actor wiring, not any
// real vault's status words.
func TestAdvanceable(t *testing.T) {
	t.Parallel()

	// a→b is owned by the operator; b→c is owned by someone else, so only a is
	// advanceable by the operator.
	const contractText = `schema_version = "1"

[enums]
type = ["doc"]

[enums.status]
note = ["a", "b", "c"]

[artifacts]
non_instance_dirs = []

[[lifecycle]]
status = "b"
applies_to = ["doc"]
from = ["a"]
owner = ["koopa"]

[[lifecycle]]
status = "c"
applies_to = ["doc"]
from = ["b"]
owner = ["bot"]
`
	contractPath := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err := os.WriteFile(contractPath, []byte(contractText), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) = %v", contractPath, err)
	}
	contract, err := schema.LoadFile(contractPath)
	if err != nil {
		t.Fatalf("LoadFile(%q) = %v", contractPath, err)
	}

	if newLifecycle(t, t.TempDir(), nil).View().Advanceable("doc", "a") {
		t.Error("Advanceable on a closed write face = true, want false")
	}

	lifecycle := newLifecycle(t, t.TempDir(), contract)
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"operator owns the onward step", "a", true},
		{"onward step owned by someone else", "b", false},
		{"no onward step defined", "c", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := lifecycle.View().Advanceable("doc", tt.status); got != tt.want {
				t.Errorf("Advanceable(%q, %q) = %v, want %v", "doc", tt.status, got, tt.want)
			}
		})
	}
}

func TestFlipByteIdentical(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{
			name: "trailing spaces on other lines survive untouched",
			content: "---\n" +
				"title: L05  \n" +
				"type: lesson  \n" +
				"status: draft\n" +
				"created: 2026-06-01   \n" +
				"---\n" +
				"\nbody\n",
		},
		{
			name: "CRLF line ending on the status line specifically",
			content: "---\n" +
				"title: L05\n" +
				"type: lesson\n" +
				"status: draft\r\n" +
				"created: 2026-06-01\n" +
				"---\n" +
				"\nbody\n",
		},
		{
			name: "a column-zero status: line inside the body is ignored",
			content: "---\n" +
				"title: L05\n" +
				"type: lesson\n" +
				"status: draft\n" +
				"---\n" +
				"\nstatus: this looks like frontmatter but is body text\n\nMore body.\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := newVault(t)
			lifecycle := newLifecycle(t, root, loadContract(t))

			writeNote(t, root, tt.content)
			commitAll(t, root)

			if err := lifecycle.Flip(t.Context(), testRel, "draft", schema.SealStatus); err != nil {
				t.Fatalf("Flip() = %v, want nil", err)
			}

			want := strings.Replace(tt.content, "status: draft", "status: ready", 1)
			got := readNote(t, root)
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("file mismatch after flip (-want +got):\n%s", diff)
			}
		})
	}
}

// TestFlipGitAddFailureWrapsErrCommitFailed guards against the file being
// rewritten on disk while a failing `git add` (staging, not just the final
// `git commit`) is reported as a plain error instead of ErrCommitFailed —
// which would route callers to a generic 500 instead of the
// "file already changed, here is the git error" presentation. A held
// `.git/index.lock` is a realistic, deterministic stand-in for the
// concurrent-git-process contention this guards against (another Flip, an
// Obsidian git-sync plugin, a cron script).
func TestFlipGitAddFailureWrapsErrCommitFailed(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	lifecycle := newLifecycle(t, root, loadContract(t))

	writeNote(t, root, lessonContent("draft"))
	commitAll(t, root)

	lockPath := filepath.Join(root, ".git", "index.lock")
	if err := os.WriteFile(lockPath, nil, 0o600); err != nil {
		t.Fatalf("create index.lock: %v", err)
	}
	removeOnCleanup(t, lockPath)

	err := lifecycle.Flip(t.Context(), testRel, "draft", schema.SealStatus)
	if !errors.Is(err, status.ErrCommitFailed) {
		t.Fatalf("Flip() = %v, want an error wrapping %v", err, status.ErrCommitFailed)
	}

	// replaceRegularFile already ran before commit() ever touched git: the file on
	// disk must show the rewritten status even though no commit exists —
	// exactly the dangerous, silently-diverged state ErrCommitFailed exists
	// to make unmistakable.
	if got := readNote(t, root); !strings.Contains(got, "status: ready") {
		t.Errorf("file after failed git add = %q, want it already rewritten to ready despite the failed commit", got)
	}
}

// TestFlipConcurrentNeverLiesInTheCommitMessage reproduces the double-tab /
// double-click race (the UI offers every legal transition as its own
// pressable key, so two can be in flight at once): two goroutines flip
// the SAME note to two different target statuses concurrently. Without the
// Lifecycle serializing Flip, the loser's
// atomic replacement can be silently overwritten by the winner and the loser's
// commit() then stages and commits whatever the winner left on disk under
// the loser's own from→to commit message — a false audit-trail entry that
// still returns err == nil to the loser. With Flip holding the Lifecycle's
// lock for its whole duration, exactly one of the two must succeed, and
// the loser must get a real, actionable error (ErrStale — the winner
// already moved the file out from under it) rather than a false success.
func TestFlipConcurrentNeverLiesInTheCommitMessage(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	lifecycle := newLifecycle(t, root, loadContract(t))

	writeNote(t, root, lessonContent("draft"))
	commitAll(t, root)
	before := commitCount(t, root)

	var errReady, errArchived error
	var wg sync.WaitGroup
	wg.Go(func() { errReady = lifecycle.Flip(t.Context(), testRel, "draft", schema.SealStatus) })
	wg.Go(func() { errArchived = lifecycle.Flip(t.Context(), testRel, "draft", "archived") })
	wg.Wait()

	succeeded := 0
	if errReady == nil {
		succeeded++
	}
	if errArchived == nil {
		succeeded++
	}
	if succeeded != 1 {
		t.Fatalf("exactly one concurrent Flip should succeed, got %d (errReady=%v, errArchived=%v)", succeeded, errReady, errArchived)
	}

	loser, wantStatus := errArchived, schema.SealStatus
	if errReady != nil {
		loser, wantStatus = errReady, "archived"
	}
	if !errors.Is(loser, status.ErrStale) {
		t.Errorf("losing Flip() = %v, want it to report %v (the winner already moved the file), not a false success", loser, status.ErrStale)
	}

	// The file and the commit message must agree on which transition
	// actually happened — never the false-audit-trail state the race
	// produces without serialization.
	final := readNote(t, root)
	if want := "status: " + wantStatus; !strings.Contains(final, want) {
		t.Errorf("file after concurrent flips = %q, want it to contain %q", final, want)
	}
	wantSubject := "status(" + testRel + "): draft → " + wantStatus + " (via yomihon)"
	if got := strings.TrimSpace(runGit(t, root, "log", "-1", "--format=%s")); got != wantSubject {
		t.Errorf("commit subject = %q, want %q (must match what was actually committed)", got, wantSubject)
	}
	if after := commitCount(t, root); after != before+1 {
		t.Errorf("commit count = %d, want %d (exactly one new commit; the loser must not also commit)", after, before+1)
	}
}

func TestWriteBlockReasonReportsUncommittedChanges(t *testing.T) {
	t.Parallel()

	root := newVault(t)
	lifecycle := newLifecycle(t, root, loadContract(t))
	writeNote(t, root, lessonContent("draft"))
	commitAll(t, root)

	clean, err := lifecycle.WriteBlockReason(t.Context(), testRel)
	if err != nil {
		t.Fatalf("WriteBlockReason(%q) on a committed note = %v", testRel, err)
	}
	if clean.Blocked() {
		t.Fatalf("WriteBlockReason(%q) on a committed note = %+v, want no refusal", testRel, clean)
	}

	writeNote(t, root, lessonContent("draft")+"\nan edit nobody committed\n")

	dirty, err := lifecycle.WriteBlockReason(t.Context(), testRel)
	if err != nil {
		t.Fatalf("WriteBlockReason(%q) after an uncommitted edit = %v", testRel, err)
	}
	if dirty != status.DirtyBlock {
		t.Errorf("WriteBlockReason(%q) after an uncommitted edit = %+v, want %+v",
			testRel, dirty, status.DirtyBlock)
	}
}

func TestWriteBlockReasonAgreesWithFlip(t *testing.T) {
	t.Parallel()

	root := newVault(t)
	lifecycle := newLifecycle(t, root, loadContract(t))
	writeNote(t, root, lessonContent("draft"))
	commitAll(t, root)
	writeNote(t, root, lessonContent("draft")+"\nan edit nobody committed\n")

	reason, err := lifecycle.WriteBlockReason(t.Context(), testRel)
	if err != nil {
		t.Fatalf("WriteBlockReason(%q) = %v", testRel, err)
	}
	flipErr := lifecycle.Flip(t.Context(), testRel, "draft", schema.SealStatus)
	if !errors.Is(flipErr, status.ErrDirty) {
		t.Fatalf("Flip(%q) = %v, want ErrDirty so the two faces describe one rule", testRel, flipErr)
	}
	if !reason.Blocked() {
		t.Error("WriteBlockReason gave no reason for a note Flip refuses; the page would offer a control that cannot work")
	}
}
