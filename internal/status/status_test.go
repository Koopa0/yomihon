package status_test

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/vault"
)

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

func TestFlipRefusesSymlinkTarget(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
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

	err := lifecycle.Flip(testRel, "draft", schema.SealStatus)
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
}

func TestFlipRefusesSymlinkDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
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

	err := lifecycle.Flip(testRel, "draft", schema.SealStatus)
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
}

func TestFlipRefusesNonRegularTarget(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lifecycle := newLifecycle(t, root, loadContract(t))
	notePath := filepath.Join(root, filepath.FromSlash(testRel))
	if err := os.MkdirAll(notePath, 0o750); err != nil {
		t.Fatalf("mkdir target directory: %v", err)
	}
	markerPath := filepath.Join(notePath, "marker")
	if err := os.WriteFile(markerPath, []byte("unchanged"), 0o600); err != nil {
		t.Fatalf("write directory marker: %v", err)
	}

	err := lifecycle.Flip(testRel, "draft", schema.SealStatus)
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
}

func TestFlipClassifiesNonInstanceBeforeFilesystemAndGit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
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

	err := lifecycle.Flip(nonInstanceRel, "draft", schema.SealStatus)
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

	err = lifecycle.Flip("System/templates/Missing.md", "draft", schema.SealStatus)
	if !errors.Is(err, status.ErrNonInstance) {
		t.Errorf("Flip(nonexistent non-instance) = %v, want %v before stat", err, status.ErrNonInstance)
	}
	err = lifecycle.Flip("System/temporary/../templates/Normalized.md", "draft", schema.SealStatus)
	if !errors.Is(err, status.ErrNonInstance) {
		t.Errorf("Flip(normalized non-instance) = %v, want %v before stat", err, status.ErrNonInstance)
	}

	err = lifecycle.Flip("System/templates-old/Missing.md", "draft", schema.SealStatus)
	if errors.Is(err, status.ErrNonInstance) {
		t.Errorf("Flip(component-boundary sibling) = %v, must reach filesystem instead of non-instance gate", err)
	}
}

// TestFlipRefusesCaseVariantNonInstancePath locks the non-instance refusal to
// the directory's identity rather than its spelling. On a case-insensitive
// filesystem a lowercase spelling of a declared artifact directory opens the
// same protected file, so a flip addressed to "system/templates" reaches the
// bytes of "System/templates" — the refusal must fire before any filesystem
// access, leave the canonical file byte-identical, and record no commit.
func TestFlipRefusesCaseVariantNonInstancePath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lifecycle := newLifecycle(t, root, loadContract(t))

	const canonicalRel = "System/templates/Card.md"
	original := lessonContent("draft")
	writeVaultFile(t, root, canonicalRel, original)

	for _, rel := range []string{"system/templates/Card.md", "SYSTEM/Templates/Card.md"} {
		err := lifecycle.Flip(rel, "draft", schema.SealStatus)
		if !errors.Is(err, status.ErrNonInstance) {
			t.Errorf("Flip(%q) = %v, want %v", rel, err, status.ErrNonInstance)
		}
	}
	got, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(canonicalRel))) // #nosec G304 -- canonicalRel is a fixed in-test constant under newVault's TempDir
	if readErr != nil {
		t.Fatalf("read template note: %v", readErr)
	}
	if diff := cmp.Diff(original, string(got)); diff != "" {
		t.Errorf("canonical template bytes changed (-want +got):\n%s", diff)
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
	root := t.TempDir()
	lifecycle := newLifecycle(t, root, loadContract(t))

	const txtRel = "Writing/lessons/japanese/L05.txt"
	original := lessonContent("draft")
	writeVaultFile(t, root, txtRel, original)

	err := lifecycle.Flip(txtRel, "draft", schema.SealStatus)
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
}

// TestFlipRefusesADifferentlySpelledOnDiskName locks the write face to the
// name the directory actually holds rather than the one the request typed. On
// a case-insensitive filesystem a request for "L06.md" opens the entry spelled
// "L06.MD", which the scan reads as a resource; the flip has to refuse before
// it rewrites those bytes, not after git declines a path its index does not
// have under that spelling.
func TestFlipRefusesADifferentlySpelledOnDiskName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lifecycle := newLifecycle(t, root, loadContract(t))

	const onDiskRel = "Writing/lessons/japanese/L06.MD"
	const requestedRel = "Writing/lessons/japanese/L06.md"
	original := lessonContent("draft")
	writeVaultFile(t, root, onDiskRel, original)

	onDisk := filepath.Join(root, filepath.FromSlash(onDiskRel))
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(requestedRel))); err != nil {
		t.Skipf("this filesystem keeps the two spellings apart, so the bypass cannot arise here: %v", err)
	}

	err := lifecycle.Flip(requestedRel, "draft", schema.SealStatus)
	if !errors.Is(err, status.ErrNonInstance) {
		t.Fatalf("Flip(%q) against on-disk %q = %v, want %v", requestedRel, onDiskRel, err, status.ErrNonInstance)
	}
	got, readErr := os.ReadFile(onDisk) // #nosec G304 -- a fixed in-test path under newVault's TempDir
	if readErr != nil {
		t.Fatalf("read the on-disk file: %v", readErr)
	}
	if diff := cmp.Diff(original, string(got)); diff != "" {
		t.Errorf("bytes rewritten before the refusal (-want +got):\n%s", diff)
	}
}

// TestFlipReportsAMissingNoteAsMissing guards the boundary the on-disk name
// check introduces: a name that resolves to nothing is a note that is not
// there, and the operator has to be told that rather than that their note is
// somehow ungovernable.
func TestFlipReportsAMissingNoteAsMissing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lifecycle := newLifecycle(t, root, loadContract(t))

	writeNote(t, root, lessonContent("draft"))

	err := lifecycle.Flip("Writing/lessons/japanese/Absent.md", "draft", schema.SealStatus)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Flip(missing note) = %v, want an error wrapping %v", err, fs.ErrNotExist)
	}
	if errors.Is(err, status.ErrNonInstance) {
		t.Errorf("Flip(missing note) = %v, must not report a missing note as a resource", err)
	}
}

// TestFlipRefusesHiddenTarget locks the write face to the whole of the
// reading scan's note definition, not half of it. The scan never descends
// into a dot-prefixed name and never serves a dot-prefixed file, so such a
// file is a resource however note-shaped its frontmatter is. Committing a
// status transition on one would mint a note-lifecycle receipt for a file no
// reading page can ever show.
func TestFlipRefusesHiddenTarget(t *testing.T) {
	t.Parallel()

	for _, rel := range []string{
		"Writing/lessons/japanese/.hidden-lesson.md",
		"Writing/.drafts/japanese/L05.md",
		".obsidian/plugins/note.md",
	} {
		t.Run(rel, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			lifecycle := newLifecycle(t, root, loadContract(t))

			original := lessonContent("draft")
			writeVaultFile(t, root, rel, original)

			err := lifecycle.Flip(rel, "draft", schema.SealStatus)
			if !errors.Is(err, status.ErrNonInstance) {
				t.Fatalf("Flip(%q) = %v, want %v", rel, err, status.ErrNonInstance)
			}
			got, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) // #nosec G304 -- a fixed in-test path under newVault's TempDir
			if readErr != nil {
				t.Fatalf("read hidden file: %v", readErr)
			}
			if diff := cmp.Diff(original, string(got)); diff != "" {
				t.Errorf("hidden file bytes changed (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFlipValidatesPathBeforeClosure(t *testing.T) {
	t.Parallel()
	lifecycle := newLifecycle(t, t.TempDir(), nil)
	for _, rel := range []string{"", ".", "..", "../outside.md", "/absolute.md", `System\templates\T.md`} {
		err := lifecycle.Flip(rel, "draft", schema.SealStatus)
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
			err := lifecycle.Flip(testRel, "draft", schema.SealStatus)
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
		onDiskStatus string // status written into the note before flipping
		from, to     string
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			lifecycle := newLifecycle(t, root, loadContract(t))

			onDisk := lessonContent(tt.onDiskStatus)
			writeNote(t, root, onDisk)

			err := lifecycle.Flip(testRel, tt.from, tt.to)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Flip() = %v, want %v", err, tt.wantErr)
			}
			if got := readNote(t, root); got != onDisk {
				t.Errorf("file was touched:\ngot:  %q\nwant: %q", got, onDisk)
			}
		})
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
			root := t.TempDir()
			lifecycle := newLifecycle(t, root, loadContract(t))

			writeNote(t, root, tt.content)

			// A missing status retains the declared note type, so the wildcard
			// archived transition reaches the surgical rewrite and reports the
			// malformed line count. Duplicate YAML keys invalidate the parsed
			// frontmatter, including its type. Lifecycle validation therefore
			// fails closed before the surgical rewrite.
			err := lifecycle.Flip(testRel, "", "archived")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Flip() = %v, want %v", err, tt.wantErr)
			}
			if got := readNote(t, root); got != tt.content {
				t.Errorf("file was touched:\ngot:  %q\nwant: %q", got, tt.content)
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
	root := t.TempDir()
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

	observed, err := lifecycle.ObservedStatus(testRel)
	if err != nil || observed != "draft" {
		t.Fatalf("ObservedStatus() = (%q, %v), want the reader to see draft", observed, err)
	}

	err = lifecycle.Flip(testRel, "draft", schema.SealStatus)
	if !errors.Is(err, status.ErrStatusSyntaxUnsupported) {
		t.Fatalf("Flip(status carrying an aliased anchor) = %v, want %v", err, status.ErrStatusSyntaxUnsupported)
	}
	if got := readNote(t, root); got != content {
		t.Errorf("note after refusal = %q, want untouched %q", got, content)
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
			root := t.TempDir()
			lifecycle := newLifecycle(t, root, loadContract(t))
			writeNote(t, root, tt.content)

			observed, err := lifecycle.ObservedStatus(testRel)
			if err != nil || observed != "draft" {
				t.Fatalf("ObservedStatus() = (%q, %v), want the reader to see draft", observed, err)
			}

			err = lifecycle.Flip(testRel, "draft", schema.SealStatus)
			if !errors.Is(err, status.ErrStatusSyntaxUnsupported) {
				t.Fatalf("Flip() = %v, want %v", err, status.ErrStatusSyntaxUnsupported)
			}
			if got := readNote(t, root); got != tt.content {
				t.Errorf("note after refusal = %q, want untouched %q", got, tt.content)
			}
		})
	}
}

func TestFlipFailClosed(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
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
			if err := lifecycle.Flip(tt.rel, tt.from, tt.to); !errors.Is(err, status.ErrClosed) {
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

// TestAdvanceable pins the bridge to the human-owner derivation: owner lists
// are declarative data with no enforcement, and the contract cannot yet say
// which owners are people, so no status is reported as waiting on one — on a
// closed view or an open one. Wiring this back to raw from-lists would count
// every note as waiting and must show up here.
func TestAdvanceable(t *testing.T) {
	t.Parallel()

	if newLifecycle(t, t.TempDir(), nil).View().Advanceable("lesson", "draft") {
		t.Error("Advanceable on a closed write face = true, want false")
	}
	if newLifecycle(t, t.TempDir(), loadContract(t)).View().Advanceable("lesson", "draft") {
		t.Error("Advanceable with no human-owner declaration = true, want false")
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
			root := t.TempDir()
			lifecycle := newLifecycle(t, root, loadContract(t))

			writeNote(t, root, tt.content)

			if err := lifecycle.Flip(testRel, "draft", schema.SealStatus); err != nil {
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

// TestFlipQuarantinesAbandonedTempFiles covers the one residue a crash inside
// the publication window leaves behind: a dot-prefixed temp file, invisible to
// the reading scan and reclaimed by nothing. Its bytes cannot be identified
// from the outside — a flip that died after the atomic exchange parks the
// version another program wrote under exactly this name — so a later flip in
// the same directory moves it out of the temp shape instead of deleting it,
// with its bytes intact. Everything else is left alone: a fresh temp may
// belong to a concurrently running process, a name that only resembles the
// shape was never yomihon's, and neither was any other file kind.
func TestFlipQuarantinesAbandonedTempFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lifecycle := newLifecycle(t, root, loadContract(t))

	original := lessonContent("draft")
	writeNote(t, root, original)

	// The middles are the 26 base32 characters crypto/rand's Text produces;
	// a name outside that shape is a decoy, not a temp.
	// Every middle below draws on its own letters: the vault filesystem may
	// be case-insensitive, and two names differing only in case would be one
	// entry there, which would prove nothing.
	const (
		agedMiddle      = "AAAAABBBBBCCCCCDDDDDEEEEEF"
		freshMiddle     = "GGGGGHHHHHIIIIIJJJJJKKKKKL"
		dirMiddle       = "MMMMMNNNNNOOOOOPPPPPQQQQQR"
		lowercaseCase   = "ssssstttttuuuuuvvvvvwwwwwx"
		outsideAlphabet = "YYYYYZZZZZ0000011111888899"
	)
	dir := filepath.Dir(filepath.Join(root, filepath.FromSlash(testRel)))
	agedTemp := filepath.Join(dir, ".yomihon-status-"+agedMiddle+".tmp")
	agedOrphan := filepath.Join(dir, ".yomihon-orphaned-"+agedMiddle+".keep")
	freshTemp := filepath.Join(dir, ".yomihon-status-"+freshMiddle+".tmp")
	shortDecoy := filepath.Join(dir, ".yomihon-status-PLANTEDSHORT.tmp")
	caseDecoy := filepath.Join(dir, ".yomihon-status-"+lowercaseCase+".tmp")
	alphabetDecoy := filepath.Join(dir, ".yomihon-status-"+outsideAlphabet+".tmp")
	suffixDecoy := filepath.Join(dir, ".yomihon-status-"+agedMiddle+".keep")
	prefixDecoy := filepath.Join(dir, "yomihon-status-"+agedMiddle+".tmp")
	dirShape := filepath.Join(dir, ".yomihon-status-"+dirMiddle+".tmp")

	const strandedBytes = "bytes only a person can identify"
	decoys := []string{freshTemp, shortDecoy, caseDecoy, alphabetDecoy, suffixDecoy, prefixDecoy}
	for _, path := range append([]string{agedTemp}, decoys...) {
		if err := os.WriteFile(path, []byte(strandedBytes), 0o600); err != nil {
			t.Fatalf("plant %s: %v", path, err)
		}
	}
	if err := os.Mkdir(dirShape, 0o750); err != nil {
		t.Fatalf("plant directory: %v", err)
	}
	aged := time.Now().Add(-2 * time.Hour)
	for _, path := range []string{agedTemp, shortDecoy, caseDecoy, alphabetDecoy, suffixDecoy, prefixDecoy, dirShape} {
		if err := os.Chtimes(path, aged, aged); err != nil {
			t.Fatalf("age %s: %v", path, err)
		}
	}

	if err := lifecycle.Flip(testRel, "draft", schema.SealStatus); err != nil {
		t.Fatalf("Flip() = %v, want nil", err)
	}

	if _, err := os.Lstat(agedTemp); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("aged conforming temp still under its temp name (lstat error = %v), want it moved aside", err)
	}
	kept, err := os.ReadFile(agedOrphan) // #nosec G304 -- a fixed name under this test's TempDir
	if err != nil {
		t.Errorf("read quarantined temp: %v, want the bytes preserved rather than deleted", err)
	} else if string(kept) != strandedBytes {
		t.Errorf("quarantined temp = %q, want %q", kept, strandedBytes)
	}
	for _, path := range append(decoys, dirShape) {
		if _, statErr := os.Lstat(path); statErr != nil {
			t.Errorf("lstat %s = %v, want the entry left alone", path, statErr)
		}
	}
	want := strings.Replace(original, "status: draft", "status: "+schema.SealStatus, 1)
	if got := readNote(t, root); got != want {
		t.Errorf("note after flip = %q, want %q", got, want)
	}
}

// TestFlipHappyPath is the byte-identity lock: a legal flip rewrites exactly
// the status line and leaves every other byte of the file alone.
func TestFlipHappyPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
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

	if err := lifecycle.Flip(testRel, "draft", schema.SealStatus); err != nil {
		t.Fatalf("Flip() = %v, want nil", err)
	}

	// The single highest-stakes assertion in this feature: everything but
	// the status line's content must be byte-identical.
	want := strings.Replace(original, "status: draft", "status: "+schema.SealStatus, 1)
	got := readNote(t, root)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("file mismatch after flip (-want +got):\n%s", diff)
	}
}

// TestFlipWritesTheSelectedRootAfterPathReplacement locks the write to the
// pinned root capability rather than the pathname: after the vault directory
// is renamed away and another directory takes its path, the flip still lands
// in the originally selected folder and the newcomer stays untouched.
func TestFlipWritesTheSelectedRootAfterPathReplacement(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	original := lessonContent("draft")
	writeNote(t, root, original)
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
	writeNote(t, root, original)

	if err := lifecycle.Flip(testRel, "draft", schema.SealStatus); err != nil {
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
}

// TestFlipSerializesConcurrentFlips reproduces the double-tab / double-click
// race (the UI offers every legal transition as its own pressable key, so two
// can be in flight at once): two goroutines flip the same note to two
// different target statuses concurrently. Flip holds the Lifecycle's lock for
// its whole duration, so exactly one of the two must succeed and the loser
// must get a real, actionable refusal (ErrStale — the winner already moved
// the file) rather than a false success or an interleaved write.
func TestFlipSerializesConcurrentFlips(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lifecycle := newLifecycle(t, root, loadContract(t))

	writeNote(t, root, lessonContent("draft"))

	var errReady, errArchived error
	var wg sync.WaitGroup
	wg.Go(func() { errReady = lifecycle.Flip(testRel, "draft", schema.SealStatus) })
	wg.Go(func() { errArchived = lifecycle.Flip(testRel, "draft", "archived") })
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
	final := readNote(t, root)
	if want := "status: " + wantStatus; !strings.Contains(final, want) {
		t.Errorf("file after concurrent flips = %q, want it to contain %q", final, want)
	}
}

// TestFlipSucceedsWithoutARepository locks the write face to plain files: a
// governed folder that is no git repository accepts a legal transition and the
// note is rewritten in place.
func TestFlipSucceedsWithoutARepository(t *testing.T) {
	t.Parallel()
	root := t.TempDir() // deliberately not a git repository
	lifecycle := newLifecycle(t, root, loadContract(t))

	original := lessonContent("draft")
	writeNote(t, root, original)

	if err := lifecycle.Flip(testRel, "draft", schema.SealStatus); err != nil {
		t.Fatalf("Flip() in a plain folder = %v, want nil", err)
	}
	want := strings.Replace(original, "status: draft", "status: "+schema.SealStatus, 1)
	if got := readNote(t, root); got != want {
		t.Errorf("note after flip = %q, want %q", got, want)
	}
}

// TestFlipSucceedsOnAnUncommittedNote locks the other half of the same rule: a
// note that exists only in the working tree — freshly produced, never
// committed — is as writable as any other. The flip's own concurrency checks
// still guard the write; version control state does not.
func TestFlipSucceedsOnAnUncommittedNote(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	lifecycle := newLifecycle(t, root, loadContract(t))

	committed := lessonContent("draft")
	writeNote(t, root, committed)
	commitAll(t, root)
	edited := committed + "<!-- an uncommitted edit -->\n"
	writeNote(t, root, edited)

	if err := lifecycle.Flip(testRel, "draft", schema.SealStatus); err != nil {
		t.Fatalf("Flip() on an uncommitted note = %v, want nil", err)
	}
	want := strings.Replace(edited, "status: draft", "status: "+schema.SealStatus, 1)
	if got := readNote(t, root); got != want {
		t.Errorf("note after flip = %q, want the uncommitted edit kept and only the status line changed (want %q)", got, want)
	}
}

// TestFlipLeavesAnExistingRepositoryUntouched pins what the write face no
// longer does: when the vault happens to be a git repository, a flip changes
// the file and nothing else — no commit, no staging. The vault's history is
// whatever its owner's own practice produces.
func TestFlipLeavesAnExistingRepositoryUntouched(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	lifecycle := newLifecycle(t, root, loadContract(t))

	original := lessonContent("draft")
	writeNote(t, root, original)
	commitAll(t, root)
	before := commitCount(t, root)

	if err := lifecycle.Flip(testRel, "draft", schema.SealStatus); err != nil {
		t.Fatalf("Flip() = %v, want nil", err)
	}
	want := strings.Replace(original, "status: draft", "status: "+schema.SealStatus, 1)
	if got := readNote(t, root); got != want {
		t.Errorf("note after flip = %q, want %q", got, want)
	}
	if after := commitCount(t, root); after != before {
		t.Errorf("commit count = %d, want unchanged %d (the flip records nothing)", after, before)
	}
	porcelain := runGit(t, root, "status", "--porcelain")
	if !strings.Contains(porcelain, "L05.md") {
		t.Errorf("git status --porcelain = %q, want the rewritten note left uncommitted", porcelain)
	}
	staged := strings.TrimSpace(runGit(t, root, "diff", "--cached", "--name-only"))
	if staged != "" {
		t.Errorf("staged paths = %q, want nothing staged", staged)
	}
}

// TestTransitionsIgnoreOwnerLists locks the demotion of lifecycle owner lists
// to declarative data: a transition whose from-list admits the current status
// is offered whoever the owner list names, including a stage no human owns.
func TestTransitionsIgnoreOwnerLists(t *testing.T) {
	t.Parallel()

	const contractText = `schema_version = "1"

[enums]
type = ["doc"]

[enums.status]
note = ["a", "b"]

[artifacts]
non_instance_dirs = []

[[lifecycle]]
status = "a"
applies_to = ["doc"]
from = []
owner = ["agent"]

[[lifecycle]]
status = "b"
applies_to = ["doc"]
from = ["a"]
owner = ["agent"]
`
	contractPath := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err := os.WriteFile(contractPath, []byte(contractText), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) = %v", contractPath, err)
	}
	contract, err := schema.LoadFile(contractPath)
	if err != nil {
		t.Fatalf("LoadFile(%q) = %v", contractPath, err)
	}
	lifecycle := newLifecycle(t, t.TempDir(), contract)

	got := lifecycle.View().Transitions("Writing/doc.md", "doc", "a")
	if diff := cmp.Diff([]string{"b"}, got); diff != "" {
		t.Errorf("Transitions() mismatch (-want +got):\n%s", diff)
	}
}
