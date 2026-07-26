package status

// This file is a deliberate, narrow exception to the repo's usual black-box
// testing convention. It injects filesystem changes into one synchronous Flip
// call so the final identity and content checks can be exercised without
// scheduling sleeps or an inherently flaky external race.

import (
	"crypto/sha256"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

func internalRunGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	fullArgs := append([]string{"-C", root}, args...)
	cmd := exec.CommandContext(t.Context(), "git", fullArgs...) // #nosec G204 -- fixed test-controlled invocation, never passed through a shell
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func internalVault(t *testing.T) (string, *Lifecycle) {
	t.Helper()
	root, _, lifecycle := internalVaultWithMutableContract(t)
	return root, lifecycle
}

func internalVaultWithMutableContract(t *testing.T) (root, contractPath string, lifecycle *Lifecycle) {
	t.Helper()
	root = t.TempDir()
	internalRunGit(t, root, "init", "--initial-branch=main")
	internalRunGit(t, root, "config", "user.name", "Test Vault")
	internalRunGit(t, root, "config", "user.email", "test-vault@example.invalid")
	internalRunGit(t, root, "config", "commit.gpgsign", "false")
	contractBytes, err := os.ReadFile(filepath.Join("testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read test contract: %v", err)
	}
	contractPath = filepath.Join(t.TempDir(), "vault-schema.toml")
	if err = os.WriteFile(contractPath, contractBytes, 0o600); err != nil { // #nosec G703 -- fixed basename under this test's TempDir
		t.Fatalf("write mutable test contract: %v", err)
	}
	contract, err := schema.LoadFile(contractPath)
	if err != nil {
		t.Fatalf("LoadFile(%q) = %v", contractPath, err)
	}
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	lifecycle, err = Open(reader, contract, contract.Governance())
	if err != nil {
		t.Fatalf("Open(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if closeErr := lifecycle.Close(); closeErr != nil {
			t.Errorf("Lifecycle.Close() error = %v", closeErr)
		}
	})
	return root, contractPath, lifecycle
}

func readNoteFixture(t *testing.T, root string) []byte {
	t.Helper()
	data, err := fs.ReadFile(os.DirFS(root), "note.md")
	if err != nil {
		t.Fatalf("read note fixture: %v", err)
	}
	return data
}

func internalLesson() string {
	return "---\n" +
		"title: L05\n" +
		"type: lesson\n" +
		"domain: japanese\n" +
		"status: draft\n" +
		"created: 2026-06-01\n" +
		"updated: 2026-06-01\n" +
		"---\n" +
		"\nbody\n"
}

func internalCommitCount(t *testing.T, root string) int {
	t.Helper()
	out := strings.TrimSpace(internalRunGit(t, root, "log", "--oneline"))
	if out == "" {
		return 0
	}
	return len(strings.Split(out, "\n"))
}

func assertNoStatusTemps(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read temp directory: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".yomihon-status-") {
			t.Errorf("temporary status file remains after refusal: %q", entry.Name())
		}
	}
}

func internalRoot(t *testing.T, path string) *os.Root {
	t.Helper()
	root, err := os.OpenRoot(path)
	if err != nil {
		t.Fatalf("os.OpenRoot(%q) error = %v", path, err)
	}
	t.Cleanup(func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("Root.Close() error = %v", closeErr)
		}
	})
	return root
}

func TestFlipRejectsChangedContractBeforeFilesystem(t *testing.T) {
	t.Parallel()

	root, contractPath, lifecycle := internalVaultWithMutableContract(t)
	data, err := os.ReadFile(contractPath) // #nosec G304 -- helper returns a fixed basename under this test's TempDir
	if err != nil {
		t.Fatalf("read mutable contract: %v", err)
	}
	const current = `non_instance_dirs = ["System/templates"]`
	const changed = `non_instance_dirs = ["System/templates", "Writing"]`
	updated := strings.Replace(string(data), current, changed, 1)
	if updated == string(data) {
		t.Fatal("artifact-policy reclassification did not apply")
	}
	if err = os.WriteFile(contractPath, []byte(updated), 0o600); err != nil { // #nosec G703 -- helper returns a fixed basename under this test's TempDir
		t.Fatalf("write reclassified contract: %v", err)
	}

	err = lifecycle.Flip(t.Context(), "Writing/missing.md", "draft", schema.SealStatus)
	if !errors.Is(err, ErrArtifactPolicyUnavailable) {
		t.Fatalf("Flip() after contract change = %v, want %v before target access", err, ErrArtifactPolicyUnavailable)
	}
	if got := err.Error(); got != "vault artifact policy source changed after startup; instance projections disabled until restart" {
		t.Errorf("Flip() diagnostic = %q, want stale artifact-policy diagnostic", got)
	}
	if _, statErr := os.Stat(filepath.Join(root, "Writing")); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("Writing directory stat after refusal = %v, want not created", statErr)
	}
}

func TestQueuedFlipRechecksAuthorityBeforeTargetAccess(t *testing.T) {
	t.Parallel()

	root, contractPath, lifecycle := internalVaultWithMutableContract(t)
	const (
		firstRel  = "Writing/lessons/japanese/First.md"
		secondRel = "Writing/lessons/japanese/Second.md"
	)
	for _, rel := range []string{firstRel, secondRel} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("mkdir note parent: %v", err)
		}
		if err := os.WriteFile(path, []byte(internalLesson()), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	internalRunGit(t, root, "add", "-A")
	internalRunGit(t, root, "commit", "-m", "seed lessons")

	firstAtPublish := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstErr := make(chan error, 1)
	secondAtLock := make(chan struct{})
	secondErr := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Go(func() {
		firstErr <- lifecycle.flip(
			t.Context(),
			firstRel,
			"draft",
			schema.SealStatus,
			flipHooks{beforePublish: func() {
				close(firstAtPublish)
				<-releaseFirst
			}},
		)
	})
	<-firstAtPublish
	wg.Go(func() {
		secondErr <- lifecycle.flip(
			t.Context(),
			secondRel,
			"draft",
			schema.SealStatus,
			flipHooks{beforeLock: func() { close(secondAtLock) }},
		)
	})
	<-secondAtLock

	contractBytes, err := os.ReadFile(contractPath) // #nosec G304 -- helper returns a fixed basename under this test's TempDir
	if err != nil {
		t.Fatalf("read mutable contract: %v", err)
	}
	if err = os.WriteFile(contractPath, append(contractBytes, '\n'), 0o600); err != nil { // #nosec G703 -- helper returns a fixed basename under this test's TempDir
		t.Fatalf("change contract source: %v", err)
	}
	if err = os.Remove(filepath.Join(root, filepath.FromSlash(secondRel))); err != nil {
		t.Fatalf("remove queued target: %v", err)
	}
	close(releaseFirst)
	wg.Wait()

	if err = <-firstErr; !errors.Is(err, ErrArtifactPolicyUnavailable) {
		t.Errorf("first Flip() error = %v, want %v", err, ErrArtifactPolicyUnavailable)
	}
	if err = <-secondErr; !errors.Is(err, ErrArtifactPolicyUnavailable) {
		t.Errorf("queued Flip() error = %v, want %v before target access", err, ErrArtifactPolicyUnavailable)
	}
}

func TestLastCommitHashHoldsLifecycleLock(t *testing.T) {
	t.Parallel()

	root, lifecycle := internalVault(t)
	const rel = "Writing/lessons/japanese/L05.md"
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir note parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(internalLesson()), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	internalRunGit(t, root, "add", "-A")
	internalRunGit(t, root, "commit", "-m", "seed lesson")

	_, err := lifecycle.lastCommitHash(t.Context(), rel, sha256.Sum256([]byte(internalLesson())), provenanceHooks{
		afterLock: func() {
			if lifecycle.mu.TryLock() {
				lifecycle.mu.Unlock()
				t.Error("LastCommitHash reached git without holding Lifecycle.mu")
			}
		},
	})
	if err != nil {
		t.Fatalf("LastCommitHash() error = %v", err)
	}
}

func TestLastCommitHashRechecksBytesAfterGit(t *testing.T) {
	t.Parallel()

	root, lifecycle := internalVault(t)
	const rel = "Writing/lessons/japanese/L05.md"
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir note parent: %v", err)
	}
	original := []byte(internalLesson())
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	internalRunGit(t, root, "add", "-A")
	internalRunGit(t, root, "commit", "-m", "seed lesson")

	got, err := lifecycle.lastCommitHash(t.Context(), rel, sha256.Sum256(original), provenanceHooks{
		afterGit: func() {
			if writeErr := os.WriteFile(path, append(original, []byte("\nnewer bytes\n")...), 0o600); writeErr != nil {
				t.Fatalf("change note after git query: %v", writeErr)
			}
		},
	})
	if err != nil {
		t.Fatalf("LastCommitHash() error = %v", err)
	}
	if got != "" {
		t.Errorf("LastCommitHash() = %q after bytes changed behind the git query, want empty", got)
	}
}

func TestLifecycleProjectionsCloseWhenContractChanges(t *testing.T) {
	t.Parallel()

	_, contractPath, lifecycle := internalVaultWithMutableContract(t)
	data, err := os.ReadFile(contractPath) // #nosec G304 -- helper returns a fixed basename under this test's TempDir
	if err != nil {
		t.Fatalf("read mutable contract: %v", err)
	}
	if err = os.WriteFile(contractPath, append(data, '\n'), 0o600); err != nil { // #nosec G703 -- helper returns a fixed basename under this test's TempDir
		t.Fatalf("change contract source: %v", err)
	}

	if !lifecycle.View().Closed() {
		t.Error("Closed() = false after contract source change, want true")
	}
	const want = "vault artifact policy source changed after startup; instance projections disabled until restart"
	if got := lifecycle.View().Diagnostic(); got != want {
		t.Errorf("WriteDiagnostic() = %q, want %q", got, want)
	}
	if got := lifecycle.View().Transitions("Writing/lessons/japanese/L05.md", "lesson", "draft"); got != nil {
		t.Errorf("Transitions() after contract source change = %v, want nil", got)
	}
	if lifecycle.View().Advanceable("lesson", "draft") {
		t.Error("Advanceable() after contract source change = true, want false")
	}
}

func TestLifecycleViewIsImmutableAndNextCaptureObservesClosure(t *testing.T) {
	t.Parallel()

	_, contractPath, lifecycle := internalVaultWithMutableContract(t)
	view := lifecycle.View()
	if diagnostic := view.Diagnostic(); diagnostic != "" {
		t.Fatalf("initial View() diagnostic = %q, want open", diagnostic)
	}
	wantOrder := view.Order()
	if len(wantOrder) == 0 {
		t.Fatal("initial View() has no lifecycle order")
	}
	wantTransitions := view.Transitions("Writing/lessons/japanese/L05.md", "lesson", "draft")
	if len(wantTransitions) == 0 || !view.Advanceable("lesson", "draft") {
		t.Fatal("initial View() has no governed draft transition")
	}

	data, err := os.ReadFile(contractPath) // #nosec G304 -- helper returns a fixed basename under this test's TempDir
	if err != nil {
		t.Fatalf("read mutable contract: %v", err)
	}
	if err = os.WriteFile(contractPath, append(data, '\n'), 0o600); err != nil { // #nosec G703 -- helper returns a fixed basename under this test's TempDir
		t.Fatalf("change contract source: %v", err)
	}

	if got := view.Order(); !slices.Equal(got, wantOrder) {
		t.Errorf("captured View().Order() after source drift = %v, want immutable %v", got, wantOrder)
	}
	if diagnostic := view.Diagnostic(); diagnostic != "" {
		t.Errorf("captured View().Diagnostic() after source drift = %q, want immutable open view", diagnostic)
	}

	next := lifecycle.View()
	if diagnostic := next.Diagnostic(); diagnostic == "" {
		t.Fatal("next View() after source drift remained open")
	}
	if got := next.Order(); got != nil {
		t.Errorf("next View().Order() after source drift = %v, want nil", got)
	}
	if got := view.Transitions("Writing/lessons/japanese/L05.md", "lesson", "draft"); !slices.Equal(got, wantTransitions) {
		t.Errorf("captured View().Transitions() after source drift = %v, want immutable %v", got, wantTransitions)
	}
	if !view.Advanceable("lesson", "draft") {
		t.Error("captured View().Advanceable() after source drift = false, want immutable true")
	}
}

func TestLifecycleLatchesContractChangeUntilRestart(t *testing.T) {
	t.Parallel()

	root, contractPath, lifecycle := internalVaultWithMutableContract(t)
	original, err := os.ReadFile(contractPath) // #nosec G304 -- helper returns a fixed basename under this test's TempDir
	if err != nil {
		t.Fatalf("read mutable contract: %v", err)
	}
	if err = os.WriteFile(contractPath, append(original, '\n'), 0o600); err != nil { // #nosec G703 -- helper returns a fixed basename under this test's TempDir
		t.Fatalf("change contract source: %v", err)
	}
	if !lifecycle.View().Closed() {
		t.Fatal("Closed() = false after contract source change, want latched closure")
	}
	if err = os.WriteFile(contractPath, original, 0o600); err != nil { // #nosec G703 -- helper returns a fixed basename under this test's TempDir
		t.Fatalf("restore original contract bytes: %v", err)
	}

	if !lifecycle.View().Closed() {
		t.Error("Closed() = false after restoring contract bytes, want closure until restart")
	}
	const want = "vault artifact policy source changed after startup; instance projections disabled until restart"
	if got := lifecycle.View().Diagnostic(); got != want {
		t.Errorf("WriteDiagnostic() after restore = %q, want %q", got, want)
	}
	if got := lifecycle.View().Transitions("Writing/lessons/japanese/L05.md", "lesson", "draft"); got != nil {
		t.Errorf("Transitions() after restore = %v, want nil until restart", got)
	}

	reloaded, err := schema.LoadFile(contractPath)
	if err != nil {
		t.Fatalf("LoadFile(restored contract) = %v", err)
	}
	if restarted := internalOpenLifecycle(t, root, reloaded); restarted.View().Closed() {
		t.Errorf("new Lifecycle after restored contract remained closed: %s", restarted.View().Diagnostic())
	}
}

func internalOpenLifecycle(t *testing.T, root string, contract *schema.Contract) *Lifecycle {
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
	lifecycle, err := Open(reader, contract, contract.Governance())
	if err != nil {
		t.Fatalf("Open(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if closeErr := lifecycle.Close(); closeErr != nil {
			t.Errorf("Lifecycle.Close() error = %v", closeErr)
		}
	})
	return lifecycle
}

func TestFlipRejectsContractChangeBeforePublish(t *testing.T) {
	t.Parallel()

	root, contractPath, lifecycle := internalVaultWithMutableContract(t)
	const rel = "Writing/lessons/japanese/L05.md"
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir note parent: %v", err)
	}
	original := internalLesson()
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	internalRunGit(t, root, "add", "-A")
	internalRunGit(t, root, "commit", "-m", "seed lesson")
	beforeCommits := internalCommitCount(t, root)

	err := lifecycle.flip(t.Context(), rel, "draft", schema.SealStatus, flipHooks{beforePublish: func() {
		data, readErr := os.ReadFile(contractPath) // #nosec G304 -- helper returns a fixed basename under this test's TempDir
		if readErr != nil {
			t.Fatalf("read mutable contract: %v", readErr)
		}
		const current = `non_instance_dirs = ["System/templates"]`
		const changed = `non_instance_dirs = ["System/templates", "Writing"]`
		updated := strings.Replace(string(data), current, changed, 1)
		if updated == string(data) {
			t.Fatal("artifact-policy reclassification did not apply")
		}
		if writeErr := os.WriteFile(contractPath, []byte(updated), 0o600); writeErr != nil { // #nosec G703 -- helper returns a fixed basename under this test's TempDir
			t.Fatalf("write reclassified contract: %v", writeErr)
		}
	}})
	if !errors.Is(err, ErrArtifactPolicyUnavailable) {
		t.Fatalf("Flip() after final authority change = %v, want %v", err, ErrArtifactPolicyUnavailable)
	}
	const wantDiagnostic = "vault artifact policy source changed after startup; instance projections disabled until restart"
	if got := err.Error(); got != wantDiagnostic {
		t.Errorf("Flip() diagnostic = %q, want %q", got, wantDiagnostic)
	}
	got, err := os.ReadFile(path) // #nosec G304 -- path is a fixed test file under t.TempDir
	if err != nil {
		t.Fatalf("read note after refusal: %v", err)
	}
	if string(got) != original {
		t.Errorf("note changed after authority refusal:\ngot:  %q\nwant: %q", got, original)
	}
	if after := internalCommitCount(t, root); after != beforeCommits {
		t.Errorf("commit count = %d, want unchanged %d", after, beforeCommits)
	}
	assertNoStatusTemps(t, filepath.Dir(path))
}

func TestFlipCommitsSelectedRootWhenPathIsReplacedAfterPublication(t *testing.T) {
	t.Parallel()

	root, lifecycle := internalVault(t)
	const rel = "Writing/lessons/japanese/L05.md"
	original := internalLesson()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir note parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	internalRunGit(t, root, "add", "-A")
	internalRunGit(t, root, "commit", "-m", "seed selected vault")

	moved := root + "-selected"
	err := lifecycle.flip(t.Context(), rel, "draft", schema.SealStatus, flipHooks{
		afterPublish: func() {
			if renameErr := os.Rename(root, moved); renameErr != nil {
				t.Fatalf("rename selected vault after publication: %v", renameErr)
			}
			t.Cleanup(func() {
				if removeErr := os.RemoveAll(moved); removeErr != nil {
					t.Errorf("remove moved vault: %v", removeErr)
				}
			})
			if mkdirErr := os.Mkdir(root, 0o750); mkdirErr != nil {
				t.Fatalf("create replacement vault: %v", mkdirErr)
			}
			internalRunGit(t, root, "init", "--initial-branch=main")
			internalRunGit(t, root, "config", "user.name", "Replacement Vault")
			internalRunGit(t, root, "config", "user.email", "replacement@example.invalid")
			internalRunGit(t, root, "config", "commit.gpgsign", "false")
			replacementPath := filepath.Join(root, filepath.FromSlash(rel))
			if mkdirErr := os.MkdirAll(filepath.Dir(replacementPath), 0o750); mkdirErr != nil {
				t.Fatalf("mkdir replacement note parent: %v", mkdirErr)
			}
			if writeErr := os.WriteFile(replacementPath, []byte(original), 0o600); writeErr != nil {
				t.Fatalf("write replacement note: %v", writeErr)
			}
			internalRunGit(t, root, "add", "-A")
			internalRunGit(t, root, "commit", "-m", "seed replacement vault")
		},
	})
	if err != nil {
		t.Fatalf("Flip() after publication-time root replacement = %v, want nil", err)
	}
	wantSelected := strings.Replace(original, "status: draft", "status: "+schema.SealStatus, 1)
	selected, err := os.ReadFile(filepath.Join(moved, filepath.FromSlash(rel))) // #nosec G304 -- moved is a test-owned temporary directory
	if err != nil {
		t.Fatalf("read selected note: %v", err)
	}
	if string(selected) != wantSelected {
		t.Errorf("selected note = %q, want %q", selected, wantSelected)
	}
	replacement, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) // #nosec G304 -- root is a test-owned temporary directory
	if err != nil {
		t.Fatalf("read replacement note: %v", err)
	}
	if string(replacement) != original {
		t.Errorf("replacement note = %q, want untouched %q", replacement, original)
	}
	if got := internalCommitCount(t, moved); got != 2 {
		t.Errorf("selected commit count = %d, want 2", got)
	}
	if got := internalCommitCount(t, root); got != 1 {
		t.Errorf("replacement commit count = %d, want 1", got)
	}
}

func TestGitChildExecFailureAfterPublicationPreservesPublishedBytes(t *testing.T) {
	root, lifecycle := internalVault(t)
	const rel = "Writing/lessons/japanese/L05.md"
	original := internalLesson()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir note parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	internalRunGit(t, root, "add", "-A")
	internalRunGit(t, root, "commit", "-m", "seed lesson")
	originalPath := os.Getenv("PATH")

	err := lifecycle.flip(t.Context(), rel, "draft", schema.SealStatus, flipHooks{
		afterPublish: func() { t.Setenv("PATH", t.TempDir()) },
	})
	t.Setenv("PATH", originalPath)
	if !errors.Is(err, ErrCommitFailed) {
		t.Fatalf("Flip() with post-publication child exec failure = %v, want %v", err, ErrCommitFailed)
	}
	want := strings.Replace(original, "status: draft", "status: "+schema.SealStatus, 1)
	got, readErr := os.ReadFile(path) // #nosec G304 -- path is a test-owned file under t.TempDir
	if readErr != nil {
		t.Fatalf("read published note: %v", readErr)
	}
	if string(got) != want {
		t.Errorf("published bytes after child exec failure = %q, want %q", got, want)
	}
	if got := internalCommitCount(t, root); got != 1 {
		t.Errorf("commit count after child exec failure = %d, want unchanged 1", got)
	}
}

func TestLifecycleCloseWaitsForFlipAndLaterOperationsFail(t *testing.T) {
	t.Parallel()

	root, lifecycle := internalVault(t)
	const rel = "Writing/lessons/japanese/L05.md"
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir note parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(internalLesson()), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	internalRunGit(t, root, "add", "-A")
	internalRunGit(t, root, "commit", "-m", "seed lesson")

	flipLocked := make(chan struct{})
	releaseFlip := make(chan struct{})
	flipResult := make(chan error, 1)
	closeStarted := make(chan struct{})
	closeLocked := make(chan struct{})
	closeResult := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Go(func() {
		flipResult <- lifecycle.flip(t.Context(), rel, "draft", schema.SealStatus, flipHooks{
			afterLock: func() {
				close(flipLocked)
				<-releaseFlip
			},
		})
	})
	<-flipLocked
	wg.Go(func() {
		closeResult <- lifecycle.close(closeHooks{
			beforeLock: func() { close(closeStarted) },
			afterLock:  func() { close(closeLocked) },
		})
	})
	<-closeStarted
	select {
	case <-closeLocked:
		t.Fatal("Close acquired Lifecycle.mu while Flip still held it")
	default:
	}
	close(releaseFlip)
	wg.Wait()
	if err := <-flipResult; err != nil {
		t.Fatalf("in-progress Flip() = %v, want nil before Close", err)
	}
	if err := <-closeResult; err != nil {
		t.Fatalf("Close() after Flip = %v, want nil", err)
	}
	select {
	case <-closeLocked:
	default:
		t.Fatal("Close never acquired Lifecycle.mu after Flip returned")
	}
	if err := lifecycle.Flip(t.Context(), rel, schema.SealStatus, "archived"); !errors.Is(err, ErrClosed) {
		t.Errorf("Flip() after Close = %v, want %v", err, ErrClosed)
	}
	if _, err := lifecycle.LastCommitHash(t.Context(), rel, sha256.Sum256(nil)); !errors.Is(err, ErrClosed) {
		t.Errorf("LastCommitHash() after Close = %v, want %v", err, ErrClosed)
	}
	if !lifecycle.View().Closed() {
		t.Error("View().Closed() after Close = false, want true")
	}
}

func TestGitChildEnvironmentRemovesRepositoryControlAndApplicationValues(t *testing.T) {
	t.Parallel()

	environ := []string{
		"PATH=/usr/bin:/bin",
		"GIT_DIR=/tmp/other.git",
		"GIT_WORK_TREE=/tmp/other-tree",
		"GIT_CONFIG_COUNT=1",
		"GIT_OBJECT_DIRECTORY=/tmp/objects",
		"GITHUB_TOKEN=preserved",
		"HOME=/tmp/home",
		"YOMIHON_EMBED_KEY=secret",
		"YOMIHON_ROOT=/tmp/vault",
		"YOMIHON_FUTURE_SECRET=secret",
	}
	want := []string{
		"PATH=/usr/bin:/bin",
		"GITHUB_TOKEN=preserved",
		"HOME=/tmp/home",
	}
	if diff := cmp.Diff(want, gitChildEnvironment(environ)); diff != "" {
		t.Errorf("gitChildEnvironment() mismatch (-want +got):\n%s", diff)
	}
}

func TestReplaceRegularFileChecksTargetAfterAuthority(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	openedRoot := internalRoot(t, root)
	const rel = "note.md"
	path := filepath.Join(root, rel)
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatalf("write original: %v", err)
	}
	source, err := readRegularFile(openedRoot, rel, rel)
	if err != nil {
		t.Fatalf("readRegularFile() error = %v", err)
	}

	err = replaceRegularFile(
		openedRoot,
		rel,
		rel,
		&source,
		[]byte("replacement"),
		publishHooks{
			afterAuthority: func() {
				if writeErr := os.WriteFile(path, []byte("external edit"), 0o600); writeErr != nil {
					t.Fatalf("write concurrent edit: %v", writeErr)
				}
			},
		},
		func() error { return nil },
	)
	if !errors.Is(err, ErrConcurrentWrite) {
		t.Fatalf("replaceRegularFile() error = %v, want %v", err, ErrConcurrentWrite)
	}
	got, err := os.ReadFile(path) // #nosec G304 -- path is a fixed file under t.TempDir
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(got) != "external edit" {
		t.Errorf("target = %q, want concurrent edit preserved", got)
	}
	assertNoStatusTemps(t, root)
}

func TestReplaceRegularFileSyncOrder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	openedRoot := internalRoot(t, root)
	const rel = "note.md"
	path := filepath.Join(root, rel)
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatalf("write original: %v", err)
	}
	source, err := readRegularFile(openedRoot, rel, rel)
	if err != nil {
		t.Fatalf("readRegularFile() = %v", err)
	}
	var order []string
	err = replaceRegularFile(openedRoot, rel, rel, &source, []byte("replacement"), publishHooks{
		syncTemp: func(file *os.File) error {
			order = append(order, "file")
			if got := readNoteFixture(t, root); string(got) != "original" {
				t.Fatalf("target before temp sync = %q, want original", got)
			}
			return file.Sync()
		},
		syncParent: func(parent *os.Root) error {
			order = append(order, "directory")
			if got := readNoteFixture(t, root); string(got) != "replacement" {
				t.Fatalf("target before directory sync = %q, want replacement", got)
			}
			return syncDirectory(parent)
		},
	}, func() error { return nil })
	if err != nil {
		t.Fatalf("replaceRegularFile() = %v", err)
	}
	if diff := cmp.Diff([]string{"file", "directory"}, order); diff != "" {
		t.Errorf("sync order mismatch (-want +got):\n%s", diff)
	}
}

func TestReplaceRegularFileTempSyncFailureLeavesSource(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	openedRoot := internalRoot(t, root)
	const rel = "note.md"
	path := filepath.Join(root, rel)
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatalf("write original: %v", err)
	}
	source, err := readRegularFile(openedRoot, rel, rel)
	if err != nil {
		t.Fatalf("readRegularFile() = %v", err)
	}
	wantErr := errors.New("sync failed")
	err = replaceRegularFile(openedRoot, rel, rel, &source, []byte("replacement"), publishHooks{
		syncTemp: func(*os.File) error { return wantErr },
	}, func() error { return nil })
	if !errors.Is(err, wantErr) {
		t.Fatalf("replaceRegularFile() = %v, want %v", err, wantErr)
	}
	if got := readNoteFixture(t, root); string(got) != "original" {
		t.Errorf("target after failed temp sync = %q, want original", got)
	}
	assertNoStatusTemps(t, root)
}

func TestReplaceRegularFileDirectorySyncFailureMarksPublished(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	openedRoot := internalRoot(t, root)
	const rel = "note.md"
	path := filepath.Join(root, rel)
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatalf("write original: %v", err)
	}
	source, err := readRegularFile(openedRoot, rel, rel)
	if err != nil {
		t.Fatalf("readRegularFile() = %v", err)
	}
	wantErr := errors.New("directory sync failed")
	err = replaceRegularFile(openedRoot, rel, rel, &source, []byte("replacement"), publishHooks{
		syncParent: func(*os.Root) error { return wantErr },
	}, func() error { return nil })
	if !errors.Is(err, ErrPublishUncertain) || !errors.Is(err, wantErr) {
		t.Fatalf("replaceRegularFile() = %v, want errors wrapping %v and %v", err, ErrPublishUncertain, wantErr)
	}
	if got := readNoteFixture(t, root); string(got) != "replacement" {
		t.Errorf("target after failed directory sync = %q, want replacement", got)
	}
}

func TestSourceUnmodifiedDetectsConcurrentWrite(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	openedRoot := internalRoot(t, root)
	path := filepath.Join(root, "note.md")
	if err := os.WriteFile(path, []byte("v1"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	source, err := readRegularFile(openedRoot, "note.md", "note.md")
	if err != nil {
		t.Fatalf("readRegularFile() = %v", err)
	}

	// Simulate an external process (Obsidian, another cron script) touching
	// the file in the narrow window between Flip's initial read and its
	// pre-write recheck. os.Chtimes with a deliberately displaced mtime
	// avoids depending on filesystem mtime granularity or wall-clock timing.
	touched := source.file.ModTime().Add(time.Hour)
	if err = os.Chtimes(path, touched, touched); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	parent, err := openSameParent(openedRoot, "note.md", "note.md", &source)
	if err != nil {
		t.Fatalf("openSameParent() = %v", err)
	}
	err = sourceUnmodified(parent, "note.md", &source)
	closeRoot(parent)
	if !errors.Is(err, ErrConcurrentWrite) {
		t.Fatalf("sourceUnmodified() = %v, want an error wrapping %v", err, ErrConcurrentWrite)
	}
	// The two refusals are deliberately distinct sentinels:
	// this must NOT also satisfy the unrelated "stale form" sentinel, or
	// handler.go's switch collapses back to one indistinguishable message.
	if errors.Is(err, ErrStale) {
		t.Errorf("sourceUnmodified() = %v, must not also satisfy ErrStale", err)
	}
}

func TestSourceUnmodifiedNoChange(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	openedRoot := internalRoot(t, root)
	path := filepath.Join(root, "note.md")
	if err := os.WriteFile(path, []byte("v1"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	source, err := readRegularFile(openedRoot, "note.md", "note.md")
	if err != nil {
		t.Fatalf("readRegularFile() = %v", err)
	}
	parent, err := openSameParent(openedRoot, "note.md", "note.md", &source)
	if err != nil {
		t.Fatalf("openSameParent() = %v", err)
	}
	if err := sourceUnmodified(parent, "note.md", &source); err != nil {
		t.Errorf("sourceUnmodified() = %v, want nil when nothing touched the file", err)
	}
	closeRoot(parent)
}

func TestSourceUnmodifiedDetectsModeChange(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	openedRoot := internalRoot(t, root)
	path := filepath.Join(root, "note.md")
	if err := os.WriteFile(path, []byte("v1"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	source, err := readRegularFile(openedRoot, "note.md", "note.md")
	if err != nil {
		t.Fatalf("readRegularFile() = %v", err)
	}
	if err = os.Chmod(path, 0o640); err != nil { // #nosec G302 -- the test deliberately makes the fixture more permissive to prove mode drift is rejected
		t.Fatalf("chmod: %v", err)
	}

	parent, err := openSameParent(openedRoot, "note.md", "note.md", &source)
	if err != nil {
		t.Fatalf("openSameParent() = %v", err)
	}
	err = sourceUnmodified(parent, "note.md", &source)
	closeRoot(parent)
	if !errors.Is(err, ErrConcurrentWrite) {
		t.Fatalf("sourceUnmodified() after mode change = %v, want %v", err, ErrConcurrentWrite)
	}
}

func TestOpenSameParentDetectsDirectoryReplacement(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	openedRoot := internalRoot(t, root)
	dir := filepath.Join(root, "notes")
	if err := os.Mkdir(dir, 0o750); err != nil {
		t.Fatalf("mkdir notes: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "note.md"), []byte("v1"), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	source, err := readRegularFile(openedRoot, filepath.Join("notes", "note.md"), "notes/note.md")
	if err != nil {
		t.Fatalf("readRegularFile() = %v", err)
	}
	if err = os.Rename(dir, filepath.Join(root, "old-notes")); err != nil {
		t.Fatalf("rename original parent: %v", err)
	}
	if err = os.Mkdir(dir, 0o750); err != nil {
		t.Fatalf("mkdir replacement parent: %v", err)
	}
	if err = os.WriteFile(filepath.Join(dir, "note.md"), []byte("v1"), 0o600); err != nil {
		t.Fatalf("write replacement note: %v", err)
	}

	parent, err := openSameParent(openedRoot, filepath.Join("notes", "note.md"), "notes/note.md", &source)
	if parent != nil {
		closeRoot(parent)
	}
	if !errors.Is(err, ErrConcurrentWrite) {
		t.Fatalf("openSameParent() after directory replacement = %v, want %v", err, ErrConcurrentWrite)
	}
}

func TestReadRegularFileRefusesNonRegularBeforeOpen(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	openedRoot := internalRoot(t, root)
	if err := os.Mkdir(filepath.Join(root, "note.md"), 0o750); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	_, err := readRegularFile(openedRoot, "note.md", "note.md")
	if !errors.Is(err, errNotRegular) {
		t.Fatalf("readRegularFile(directory) = %v, want %v", err, errNotRegular)
	}
}

func TestFlipDetectsSameMtimeContentChange(t *testing.T) {
	root, lifecycle := internalVault(t)
	const rel = "Writing/lessons/japanese/L05.md"
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir note parent: %v", err)
	}
	original := internalLesson()
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	internalRunGit(t, root, "add", "-A")
	internalRunGit(t, root, "commit", "-m", "seed lesson")
	beforeCommits := internalCommitCount(t, root)

	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat note: %v", err)
	}
	replacement := strings.Replace(original, "\nbody\n", "\nBODY\n", 1)
	if len(replacement) != len(original) {
		t.Fatalf("replacement length = %d, want same as original %d", len(replacement), len(original))
	}

	err = lifecycle.flip(t.Context(), rel, "draft", schema.SealStatus, flipHooks{beforePublish: func() {
		if writeErr := os.WriteFile(path, []byte(replacement), before.Mode().Perm()); writeErr != nil {
			t.Fatalf("replace note bytes: %v", writeErr)
		}
		if timeErr := os.Chtimes(path, before.ModTime(), before.ModTime()); timeErr != nil {
			t.Fatalf("restore note mtime: %v", timeErr)
		}
	}})
	if !errors.Is(err, ErrConcurrentWrite) {
		t.Fatalf("Flip(same-mtime content change) = %v, want %v", err, ErrConcurrentWrite)
	}
	got, readErr := os.ReadFile(path) // #nosec G304 -- path is a test-owned path under t.TempDir
	if readErr != nil {
		t.Fatalf("read replacement: %v", readErr)
	}
	if string(got) != replacement {
		t.Errorf("replacement bytes = %q, want preserved %q", got, replacement)
	}
	if after := internalCommitCount(t, root); after != beforeCommits {
		t.Errorf("commit count = %d, want unchanged %d", after, beforeCommits)
	}
	assertNoStatusTemps(t, filepath.Dir(path))
}

func TestFlipDetectsPathIdentityReplacement(t *testing.T) {
	root, lifecycle := internalVault(t)
	const rel = "Writing/lessons/japanese/L05.md"
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir note parent: %v", err)
	}
	original := internalLesson()
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	internalRunGit(t, root, "add", "-A")
	internalRunGit(t, root, "commit", "-m", "seed lesson")
	beforeCommits := internalCommitCount(t, root)
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat note: %v", err)
	}
	replacement := strings.Replace(original, "\nbody\n", "\nreplacement\n", 1)

	err = lifecycle.flip(t.Context(), rel, "draft", schema.SealStatus, flipHooks{beforePublish: func() {
		tmp := filepath.Join(filepath.Dir(path), ".external-replacement.tmp")
		if writeErr := os.WriteFile(tmp, []byte(replacement), before.Mode().Perm()); writeErr != nil {
			t.Fatalf("write replacement: %v", writeErr)
		}
		if timeErr := os.Chtimes(tmp, before.ModTime(), before.ModTime()); timeErr != nil {
			t.Fatalf("set replacement mtime: %v", timeErr)
		}
		if renameErr := os.Rename(tmp, path); renameErr != nil {
			t.Fatalf("replace path identity: %v", renameErr)
		}
		after, statErr := os.Stat(path)
		if statErr != nil {
			t.Fatalf("stat replacement: %v", statErr)
		}
		if os.SameFile(before, after) {
			t.Fatal("replacement retained original file identity")
		}
	}})
	if !errors.Is(err, ErrConcurrentWrite) {
		t.Fatalf("Flip(path identity replacement) = %v, want %v", err, ErrConcurrentWrite)
	}
	got, readErr := os.ReadFile(path) // #nosec G304 -- path is a test-owned path under t.TempDir
	if readErr != nil {
		t.Fatalf("read replacement: %v", readErr)
	}
	if string(got) != replacement {
		t.Errorf("replacement bytes = %q, want preserved %q", got, replacement)
	}
	if after := internalCommitCount(t, root); after != beforeCommits {
		t.Errorf("commit count = %d, want unchanged %d", after, beforeCommits)
	}
	assertNoStatusTemps(t, filepath.Dir(path))
}
