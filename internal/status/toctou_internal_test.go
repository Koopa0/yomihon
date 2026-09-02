package status

// This file is a deliberate, narrow exception to the repo's usual black-box
// testing convention. It injects filesystem changes into one synchronous Flip
// call so the final identity and content checks can be exercised without
// scheduling sleeps or an inherently flaky external race.

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/vaultfs"
)

func internalVault(t *testing.T) (string, *Writer) {
	t.Helper()
	root, _, writer := internalVaultWithMutableContract(t)
	return root, writer
}

func internalVaultWithMutableContract(t *testing.T) (root, contractPath string, writer *Writer) {
	t.Helper()
	root = t.TempDir()
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
	reader, err := vaultfs.Open(root)
	if err != nil {
		t.Fatalf("vaultfs.Open(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	writer, err = Open(reader, contract, contract.Governance(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("Open(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if closeErr := writer.Close(); closeErr != nil {
			t.Errorf("Writer.Close() error = %v", closeErr)
		}
	})
	return root, contractPath, writer
}

func readNoteFixture(t *testing.T, root string) []byte {
	t.Helper()
	data, err := fs.ReadFile(os.DirFS(root), "note.md")
	if err != nil {
		t.Fatalf("read note fixture: %v", err)
	}
	return data
}

// internalLessonIdentity is the content identity a page rendered from
// internalLesson's bytes would embed with its transition forms.
func internalLessonIdentity() [sha256.Size]byte {
	return vault.ContentIdentity([]byte(internalLesson()))
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

	root, contractPath, writer := internalVaultWithMutableContract(t)
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

	err = writer.Flip(t.Context(), "Writing/missing.md", "draft", schema.SealStatus, [sha256.Size]byte{})
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

	root, contractPath, writer := internalVaultWithMutableContract(t)
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

	firstAtAuthority := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstErr := make(chan error, 1)
	secondAtLock := make(chan struct{})
	secondErr := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Go(func() {
		firstErr <- writer.flip(t.Context(),
			firstRel,
			"draft",
			schema.SealStatus,
			internalLessonIdentity(),
			flipHooks{beforeAuthority: func() {
				close(firstAtAuthority)
				<-releaseFirst
			}},
		)
	})
	<-firstAtAuthority
	wg.Go(func() {
		secondErr <- writer.flip(t.Context(),
			secondRel,
			"draft",
			schema.SealStatus,
			internalLessonIdentity(),
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

func TestWriterProjectionsCloseWhenContractChanges(t *testing.T) {
	t.Parallel()

	_, contractPath, writer := internalVaultWithMutableContract(t)
	data, err := os.ReadFile(contractPath) // #nosec G304 -- helper returns a fixed basename under this test's TempDir
	if err != nil {
		t.Fatalf("read mutable contract: %v", err)
	}
	if err = os.WriteFile(contractPath, append(data, '\n'), 0o600); err != nil { // #nosec G703 -- helper returns a fixed basename under this test's TempDir
		t.Fatalf("change contract source: %v", err)
	}

	if !writer.View().Closed() {
		t.Error("Closed() = false after contract source change, want true")
	}
	const want = "vault artifact policy source changed after startup; instance projections disabled until restart"
	if got := writer.View().Diagnostic(); got != want {
		t.Errorf("WriteDiagnostic() = %q, want %q", got, want)
	}
	if got := writer.View().Transitions("Writing/lessons/japanese/L05.md", "lesson", "draft"); got != nil {
		t.Errorf("Transitions() after contract source change = %v, want nil", got)
	}
}

func TestWriterViewIsImmutableAndNextCaptureObservesClosure(t *testing.T) {
	t.Parallel()

	_, contractPath, writer := internalVaultWithMutableContract(t)
	view := writer.View()
	if diagnostic := view.Diagnostic(); diagnostic != "" {
		t.Fatalf("initial View() diagnostic = %q, want open", diagnostic)
	}
	wantOrder := view.Order()
	if len(wantOrder) == 0 {
		t.Fatal("initial View() has no lifecycle order")
	}
	wantTransitions := view.Transitions("Writing/lessons/japanese/L05.md", "lesson", "draft")
	if len(wantTransitions) == 0 {
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

	next := writer.View()
	if diagnostic := next.Diagnostic(); diagnostic == "" {
		t.Fatal("next View() after source drift remained open")
	}
	if got := next.Order(); got != nil {
		t.Errorf("next View().Order() after source drift = %v, want nil", got)
	}
	if got := view.Transitions("Writing/lessons/japanese/L05.md", "lesson", "draft"); !slices.Equal(got, wantTransitions) {
		t.Errorf("captured View().Transitions() after source drift = %v, want immutable %v", got, wantTransitions)
	}
}

func TestWriterLatchesContractChangeUntilRestart(t *testing.T) {
	t.Parallel()

	root, contractPath, writer := internalVaultWithMutableContract(t)
	original, err := os.ReadFile(contractPath) // #nosec G304 -- helper returns a fixed basename under this test's TempDir
	if err != nil {
		t.Fatalf("read mutable contract: %v", err)
	}
	if err = os.WriteFile(contractPath, append(original, '\n'), 0o600); err != nil { // #nosec G703 -- helper returns a fixed basename under this test's TempDir
		t.Fatalf("change contract source: %v", err)
	}
	if !writer.View().Closed() {
		t.Fatal("Closed() = false after contract source change, want latched closure")
	}
	if err = os.WriteFile(contractPath, original, 0o600); err != nil { // #nosec G703 -- helper returns a fixed basename under this test's TempDir
		t.Fatalf("restore original contract bytes: %v", err)
	}

	if !writer.View().Closed() {
		t.Error("Closed() = false after restoring contract bytes, want closure until restart")
	}
	const want = "vault artifact policy source changed after startup; instance projections disabled until restart"
	if got := writer.View().Diagnostic(); got != want {
		t.Errorf("WriteDiagnostic() after restore = %q, want %q", got, want)
	}
	if got := writer.View().Transitions("Writing/lessons/japanese/L05.md", "lesson", "draft"); got != nil {
		t.Errorf("Transitions() after restore = %v, want nil until restart", got)
	}

	reloaded, err := schema.LoadFile(contractPath)
	if err != nil {
		t.Fatalf("LoadFile(restored contract) = %v", err)
	}
	if restarted := internalOpenWriter(t, root, reloaded); restarted.View().Closed() {
		t.Errorf("new Writer after restored contract remained closed: %s", restarted.View().Diagnostic())
	}
}

func internalOpenWriter(t *testing.T, root string, contract *schema.Contract) *Writer {
	t.Helper()
	reader, err := vaultfs.Open(root)
	if err != nil {
		t.Fatalf("vaultfs.Open(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	writer, err := Open(reader, contract, contract.Governance(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("Open(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if closeErr := writer.Close(); closeErr != nil {
			t.Errorf("Writer.Close() error = %v", closeErr)
		}
	})
	return writer
}

func TestFlipRejectsContractChangeBeforeInstall(t *testing.T) {
	t.Parallel()

	root, contractPath, writer := internalVaultWithMutableContract(t)
	const rel = "Writing/lessons/japanese/L05.md"
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir note parent: %v", err)
	}
	original := internalLesson()
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}

	err := writer.flip(t.Context(), rel, "draft", schema.SealStatus, internalLessonIdentity(), flipHooks{beforeAuthority: func() {
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
	assertNoStatusTemps(t, filepath.Dir(path))
}

func TestWriterCloseWaitsForFlipAndLaterOperationsFail(t *testing.T) {
	t.Parallel()

	root, writer := internalVault(t)
	const rel = "Writing/lessons/japanese/L05.md"
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir note parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(internalLesson()), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}

	flipLocked := make(chan struct{})
	releaseFlip := make(chan struct{})
	flipResult := make(chan error, 1)
	closeStarted := make(chan struct{})
	closeLocked := make(chan struct{})
	closeResult := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Go(func() {
		flipResult <- writer.flip(t.Context(), rel, "draft", schema.SealStatus, internalLessonIdentity(), flipHooks{
			afterLock: func() {
				close(flipLocked)
				<-releaseFlip
			},
		})
	})
	<-flipLocked
	wg.Go(func() {
		closeResult <- writer.close(closeHooks{
			beforeLock: func() { close(closeStarted) },
			afterLock:  func() { close(closeLocked) },
		})
	})
	<-closeStarted
	select {
	case <-closeLocked:
		t.Fatal("Close acquired Writer.mu while Flip still held it")
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
		t.Fatal("Close never acquired Writer.mu after Flip returned")
	}
	if err := writer.Flip(t.Context(), rel, schema.SealStatus, "archived", [sha256.Size]byte{}); !errors.Is(err, ErrClosed) {
		t.Errorf("Flip() after Close = %v, want %v", err, ErrClosed)
	}
	if !writer.View().Closed() {
		t.Error("View().Closed() after Close = false, want true")
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
		nil,
		installHooks{
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
	err = replaceRegularFile(openedRoot, rel, rel, &source, []byte("replacement"), nil, installHooks{
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
	err = replaceRegularFile(openedRoot, rel, rel, &source, []byte("replacement"), nil, installHooks{
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

func TestReplaceRegularFileDirectorySyncFailureMarksInstalled(t *testing.T) {
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
	err = replaceRegularFile(openedRoot, rel, rel, &source, []byte("replacement"), nil, installHooks{
		syncParent: func(*os.Root) error { return wantErr },
	}, func() error { return nil })
	if !errors.Is(err, ErrInstallUncertain) || !errors.Is(err, wantErr) {
		t.Fatalf("replaceRegularFile() = %v, want errors wrapping %v and %v", err, ErrInstallUncertain, wantErr)
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
	root, writer := internalVault(t)
	const rel = "Writing/lessons/japanese/L05.md"
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir note parent: %v", err)
	}
	original := internalLesson()
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}

	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat note: %v", err)
	}
	replacement := strings.Replace(original, "\nbody\n", "\nBODY\n", 1)
	if len(replacement) != len(original) {
		t.Fatalf("replacement length = %d, want same as original %d", len(replacement), len(original))
	}

	err = writer.flip(t.Context(), rel, "draft", schema.SealStatus, internalLessonIdentity(), flipHooks{beforeAuthority: func() {
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
	assertNoStatusTemps(t, filepath.Dir(path))
}

func TestFlipDetectsPathIdentityReplacement(t *testing.T) {
	root, writer := internalVault(t)
	const rel = "Writing/lessons/japanese/L05.md"
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir note parent: %v", err)
	}
	original := internalLesson()
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat note: %v", err)
	}
	replacement := strings.Replace(original, "\nbody\n", "\nreplacement\n", 1)

	err = writer.flip(t.Context(), rel, "draft", schema.SealStatus, internalLessonIdentity(), flipHooks{beforeAuthority: func() {
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
	assertNoStatusTemps(t, filepath.Dir(path))
}

// TestTouchingTheContractDoesNotCloseTheWriteFace separates two questions the
// identity check had collapsed into one.
//
// The pinned entry records which file the contract was at startup, and that
// record includes the modification time — so a git checkout, a pull, a restored
// backup or an editor that saves by rename moves it without changing a byte.
// The vault this serves commonly lives under version control and sync tools,
// so this is an ordinary event. Treating it as a contract change closed the
// write face until restart and told the operator their contract had changed,
// which it had not.
func TestTouchingTheContractDoesNotCloseTheWriteFace(t *testing.T) {
	t.Parallel()

	_, contractPath, writer := internalVaultWithMutableContract(t)
	before, err := os.Stat(contractPath)
	if err != nil {
		t.Fatalf("stat contract: %v", err)
	}
	if writer.View().Closed() {
		t.Fatal("the write face was already closed before the contract was touched")
	}
	original, err := os.ReadFile(contractPath) // #nosec G304 -- helper returns a fixed basename under this test's TempDir
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}

	// Move only the modification time. Writing the same bytes back is how an
	// editor and git both do it, and it is what the identity check saw as a
	// different file.
	moved := before.ModTime().Add(2 * time.Second)
	if err = os.Chtimes(contractPath, moved, moved); err != nil {
		t.Fatalf("move the contract's modification time: %v", err)
	}
	after, err := os.Stat(contractPath)
	if err != nil {
		t.Fatalf("stat contract after: %v", err)
	}
	if after.ModTime().Equal(before.ModTime()) {
		t.Fatal("the modification time did not move, so this proves nothing")
	}
	current, err := os.ReadFile(contractPath) // #nosec G304 -- helper returns a fixed basename under this test's TempDir
	if err != nil {
		t.Fatalf("re-read contract: %v", err)
	}
	if !bytes.Equal(original, current) {
		t.Fatal("the contract's bytes changed, so this is the other test")
	}

	view := writer.View()
	if view.Closed() {
		t.Errorf("the write face closed after a touch: %s", view.Diagnostic())
	}
	if got := view.Transitions("Writing/lessons/japanese/L05.md", "lesson", "draft"); got == nil {
		t.Error("no transition is offered after a touch; the note's state machine did not change")
	}
}
