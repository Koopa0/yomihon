package status_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io/fs"
	"log/slog"
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
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/vaultfs"
)

// testRel is the vault-relative path every fixture note in this package
// uses. Fixing it keeps the fixtures and their assertions simple; the path
// itself is never what's under test.
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

func newWriter(t *testing.T, root string, contract *schema.Contract) *status.Writer {
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
	writer, err := status.Open(reader, contract, contract.Governance(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("status.Open(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if closeErr := writer.Close(); closeErr != nil {
			t.Errorf("Writer.Close() error = %v", closeErr)
		}
	})
	return writer
}

// newVault creates a temp directory that is also a git repository, for the
// tests proving a flip leaves version-control state untouched: the write
// face must change the note file and nothing else, whether or not the vault
// happens to be under git. The identity config is scoped to the repo and
// commit.gpgsign is forced off so seeding commits never depend on the host's
// global git config or GPG agent.
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
// diskIdentity is the content identity of the fixture bytes as written —
// what a page rendered from exactly those bytes would embed in its
// transition forms.
func diskIdentity(content string) [sha256.Size]byte {
	return vault.ContentIdentity([]byte(content))
}

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
	reader, err := vaultfs.Open(root)
	if err != nil {
		t.Fatalf("vaultfs.Open(%q) error = %v", root, err)
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

	writer, err := status.Open(reader, nil, schema.Ungoverned(), slog.New(slog.DiscardHandler))
	if writer != nil {
		t.Cleanup(func() {
			if closeErr := writer.Close(); closeErr != nil {
				t.Errorf("Writer.Close() error = %v", closeErr)
			}
		})
	}
	if err == nil || !strings.Contains(err.Error(), "vault root changed") {
		t.Fatalf("status.Open(replaced root, slog.New(slog.DiscardHandler)) = (%v, %v), want nil and root-changed error", writer, err)
	}
}

func TestFlipRefusesSymlinkTarget(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writer := newWriter(t, root, loadContract(t))

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

	err := writer.Flip(t.Context(), testRel, "draft", schema.SealStatus, [sha256.Size]byte{})
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
	writer := newWriter(t, root, loadContract(t))

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

	err := writer.Flip(t.Context(), testRel, "draft", schema.SealStatus, [sha256.Size]byte{})
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
	writer := newWriter(t, root, loadContract(t))
	notePath := filepath.Join(root, filepath.FromSlash(testRel))
	if err := os.MkdirAll(notePath, 0o750); err != nil {
		t.Fatalf("mkdir target directory: %v", err)
	}
	markerPath := filepath.Join(notePath, "marker")
	if err := os.WriteFile(markerPath, []byte("unchanged"), 0o600); err != nil {
		t.Fatalf("write directory marker: %v", err)
	}

	err := writer.Flip(t.Context(), testRel, "draft", schema.SealStatus, [sha256.Size]byte{})
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

func TestFlipClassifiesNonInstanceBeforeFilesystem(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	contract := loadContract(t)
	writer := newWriter(t, root, contract)

	nonInstanceRel := "System/templates/Loud lesson.md"
	path := filepath.Join(root, filepath.FromSlash(nonInstanceRel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir template directory: %v", err)
	}
	original := lessonContent("draft")
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write template note: %v", err)
	}

	err := writer.Flip(t.Context(), nonInstanceRel, "draft", schema.SealStatus, [sha256.Size]byte{})
	if !errors.Is(err, status.ErrNonInstance) {
		t.Fatalf("Flip(non-instance) = %v, want %v", err, status.ErrNonInstance)
	}
	got, readErr := os.ReadFile(path) // #nosec G304 -- path is a fixed test-relative file under this test's TempDir
	if readErr != nil {
		t.Fatalf("read template note: %v", readErr)
	}
	if diff := cmp.Diff(original, string(got)); diff != "" {
		t.Errorf("non-instance bytes changed (-want +got):\n%s", diff)
	}

	err = writer.Flip(t.Context(), "System/templates/Missing.md", "draft", schema.SealStatus, [sha256.Size]byte{})
	if !errors.Is(err, status.ErrNonInstance) {
		t.Errorf("Flip(nonexistent non-instance) = %v, want %v before stat", err, status.ErrNonInstance)
	}
	err = writer.Flip(t.Context(), "System/temporary/../templates/Normalized.md", "draft", schema.SealStatus, [sha256.Size]byte{})
	if !errors.Is(err, status.ErrNonInstance) {
		t.Errorf("Flip(normalized non-instance) = %v, want %v before stat", err, status.ErrNonInstance)
	}

	err = writer.Flip(t.Context(), "System/templates-old/Missing.md", "draft", schema.SealStatus, [sha256.Size]byte{})
	if errors.Is(err, status.ErrNonInstance) {
		t.Errorf("Flip(component-boundary sibling) = %v, must reach filesystem instead of non-instance gate", err)
	}
}

// TestFlipRefusesADifferentlySpelledOnDiskName locks the write face to the
// name the directory actually holds rather than the one the request typed. On
// a case-insensitive filesystem a request for "L06.md" opens the entry spelled
// "L06.MD", which the scan reads as a resource; the flip has to refuse before
// it rewrites bytes the reading side classifies as something other than the
// requested note.
func TestFlipRefusesADifferentlySpelledOnDiskName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writer := newWriter(t, root, loadContract(t))

	const onDiskRel = "Writing/lessons/japanese/L06.MD"
	const requestedRel = "Writing/lessons/japanese/L06.md"
	original := lessonContent("draft")
	writeVaultFile(t, root, onDiskRel, original)

	onDisk := filepath.Join(root, filepath.FromSlash(onDiskRel))
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(requestedRel))); err != nil {
		t.Skipf("this filesystem keeps the two spellings apart, so the bypass cannot arise here: %v", err)
	}

	err := writer.Flip(t.Context(), requestedRel, "draft", schema.SealStatus, [sha256.Size]byte{})
	if !errors.Is(err, status.ErrNonInstance) {
		t.Fatalf("Flip(%q) against on-disk %q = %v, want %v", requestedRel, onDiskRel, err, status.ErrNonInstance)
	}
	got, readErr := os.ReadFile(onDisk) // #nosec G304 -- a fixed in-test path under this test's TempDir
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
	writer := newWriter(t, root, loadContract(t))

	writeNote(t, root, lessonContent("draft"))

	err := writer.Flip(t.Context(), "Writing/lessons/japanese/Absent.md", "draft", schema.SealStatus, [sha256.Size]byte{})
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Flip(missing note) = %v, want an error wrapping %v", err, fs.ErrNotExist)
	}
	if errors.Is(err, status.ErrNonInstance) {
		t.Errorf("Flip(missing note) = %v, must not report a missing note as a resource", err)
	}
}

// TestFlipRefusesNonInstancePaths locks the write face to the same note
// definition the reading scan uses: a note is a file whose path ends in
// ".md" and carries no dot-prefixed component, and it is the file the vault
// holds under exactly the requested spelling. Everything else is a resource.
// A resource carrying note-shaped frontmatter and a legal transition must not
// receive a committed status flip — that would mint a note-lifecycle receipt
// for a file the reading face itself refuses to offer a write form for. The
// classification folds case, because on a case-insensitive filesystem every
// spelling of a path opens the same file and a rule that read the spelling
// literally would let one of them through.
func TestFlipRefusesNonInstancePaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		onDisk  string // where the bytes are written
		request string // the path handed to Flip; empty means onDisk
	}{
		{name: "hidden note", onDisk: "Writing/lessons/japanese/.hidden-lesson.md"},
		{name: "hidden directory", onDisk: "Writing/.drafts/japanese/L05.md"},
		{name: "vault configuration directory", onDisk: ".obsidian/plugins/note.md"},
		{name: "not markdown", onDisk: "Writing/lessons/japanese/L05.txt"},
		{
			name:    "lower-case spelling of an excluded directory",
			onDisk:  "System/templates/Card.md",
			request: "system/templates/Card.md",
		},
		{
			name:    "mixed-case spelling of an excluded directory",
			onDisk:  "System/templates/Card.md",
			request: "SYSTEM/Templates/Card.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writer := newWriter(t, root, loadContract(t))

			original := lessonContent("draft")
			writeVaultFile(t, root, tt.onDisk, original)

			request := tt.onDisk
			if tt.request != "" {
				request = tt.request
			}
			err := writer.Flip(t.Context(), request, "draft", schema.SealStatus, [sha256.Size]byte{})
			if !errors.Is(err, status.ErrNonInstance) {
				t.Fatalf("Flip(%q) = %v, want %v", request, err, status.ErrNonInstance)
			}
			got, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(tt.onDisk))) // #nosec G304 -- a fixed in-test path under this test's TempDir
			if readErr != nil {
				t.Fatalf("read %s: %v", tt.onDisk, readErr)
			}
			if diff := cmp.Diff(original, string(got)); diff != "" {
				t.Errorf("bytes changed before the refusal (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFlipValidatesPathBeforeClosure(t *testing.T) {
	t.Parallel()
	writer := newWriter(t, t.TempDir(), nil)
	for _, rel := range []string{"", ".", "..", "../outside.md", "/absolute.md", `System\templates\T.md`} {
		err := writer.Flip(t.Context(), rel, "draft", schema.SealStatus, [sha256.Size]byte{})
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
			writer := newWriter(t, t.TempDir(), tt.contract)
			if !writer.View().Closed() {
				t.Fatal("Closed() = false, want artifact-policy closure")
			}
			if got, want := writer.View().Diagnostic(), tt.want; got != want {
				t.Errorf("WriteDiagnostic() = %q, want %q", got, want)
			}
			if got := writer.View().Order(); got != nil {
				t.Errorf("Order() = %v while artifact policy closes instance projections, want nil", got)
			}
			err := writer.Flip(t.Context(), testRel, "draft", schema.SealStatus, [sha256.Size]byte{})
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
			writer := newWriter(t, root, loadContract(t))

			onDisk := lessonContent(tt.onDiskStatus)
			writeNote(t, root, onDisk)

			err := writer.Flip(t.Context(), testRel, tt.from, tt.to, diskIdentity(onDisk))
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
			writer := newWriter(t, root, loadContract(t))

			writeNote(t, root, tt.content)

			// A missing status retains the declared note type, and draft is a
			// declared starting point, so the transition reaches the surgical
			// rewrite and reports the malformed line count — which is the
			// thing under test. Duplicate YAML keys invalidate the parsed
			// frontmatter, including its type. Lifecycle validation therefore
			// fails closed before the surgical rewrite.
			err := writer.Flip(t.Context(), testRel, "", "draft", diskIdentity(tt.content))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Flip() = %v, want %v", err, tt.wantErr)
			}
			if got := readNote(t, root); got != tt.content {
				t.Errorf("file was touched:\ngot:  %q\nwant: %q", got, tt.content)
			}
		})
	}
}

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
		{
			// An anchor on the status value that another field aliases. Two
			// separate things refuse it, and removing either one on its own
			// leaves this row passing: the value span treats "&" as a YAML
			// indicator and reports no span, and the rewritten bytes are
			// parsed again before anything is written, where the severed
			// anchor shows up as a dangling alias. The alias has to follow
			// the anchor, so this row spells its frontmatter out rather than
			// using block().
			name: "status value carrying an aliased anchor",
			content: "---\n" +
				"title: L05\n" +
				"type: lesson\n" +
				"status: &s draft\n" +
				"domain: *s\n" +
				"created: 2026-06-01\n" +
				"updated: 2026-06-01\n" +
				"---\n" +
				"\nbody\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writer := newWriter(t, root, loadContract(t))
			writeNote(t, root, tt.content)

			observed, err := writer.ObservedStatus(t.Context(), testRel)
			if err != nil || observed != "draft" {
				t.Fatalf("ObservedStatus() = (%q, %v), want the reader to see draft", observed, err)
			}

			err = writer.Flip(t.Context(), testRel, "draft", schema.SealStatus, diskIdentity(tt.content))
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
	writer := newWriter(t, root, nil)

	if !writer.View().Closed() {
		t.Fatal("Closed() = false, want true for a nil contract")
	}
	if got := writer.View().Transitions(testRel, "lesson", "draft"); got != nil {
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
			if err := writer.Flip(t.Context(), tt.rel, tt.from, tt.to, [sha256.Size]byte{}); !errors.Is(err, status.ErrClosed) {
				t.Errorf("Flip(%q, %q, %q) = %v, want %v", tt.rel, tt.from, tt.to, err, status.ErrClosed)
			}
		})
	}
}

func TestClosed(t *testing.T) {
	t.Parallel()
	if !newWriter(t, t.TempDir(), nil).View().Closed() {
		t.Error("Closed() = false, want true for nil contract")
	}
	if newWriter(t, t.TempDir(), loadContract(t)).View().Closed() {
		t.Error("Closed() = true, want false for a loaded contract")
	}
}

func TestOrderDistinguishesUnavailableCoreFromEmptyGroup(t *testing.T) {
	t.Parallel()
	if got := newWriter(t, t.TempDir(), nil).View().Order(); got != nil {
		t.Errorf("Order() with unavailable core = %v, want nil", got)
	}
	got := newWriter(t, t.TempDir(), loadContractWithEmptyDefaultStatusGroup(t)).View().Order()
	if got == nil {
		t.Error("Order() with valid core and explicit empty default group = nil, want available empty slice")
	}
	if len(got) != 0 {
		t.Errorf("Order() with explicit empty default group = %v, want empty", got)
	}
}

func TestTransitions(t *testing.T) {
	t.Parallel()
	writer := newWriter(t, t.TempDir(), loadContract(t))

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
			got := writer.View().Transitions(tt.relPath, tt.noteType, tt.current)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Transitions(%q, %q, %q) mismatch (-want +got):\n%s", tt.relPath, tt.noteType, tt.current, diff)
			}
		})
	}
}

// TestKnownStatus pins the declared-value check the reading page's
// out-of-enum flag rests on: a value in the type's list is known, everything
// else — an unlisted value, an undeclared type, an empty value, a closed
// view — is not.
func TestCanReturn(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		closed   bool // a view opened with no contract answers nothing at all
		noteType string
		from     string
		to       string
		want     bool
	}{
		// Cross-checked by hand against testdata/contract.toml's lifecycle
		// table: draft is entered from imported and ready, ready from draft,
		// archived from anywhere, and nothing enters imported or leaves
		// archived.
		{name: "ready walks straight back to draft", noteType: "lesson", from: "draft", to: "ready", want: true},
		{name: "nothing walks back from archived to draft", noteType: "lesson", from: "draft", to: "archived", want: false},
		{name: "nothing re-enters imported", noteType: "lesson", from: "imported", to: "draft", want: false},
		{name: "staying put is trivially returnable", noteType: "lesson", from: "draft", to: "draft", want: true},
		{name: "an empty origin claims nothing", noteType: "lesson", from: "", to: "archived", want: false},
		{name: "an empty destination claims nothing", noteType: "lesson", from: "draft", to: "", want: false},
		{name: "a closed view claims nothing", closed: true, noteType: "lesson", from: "draft", to: "ready", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			contract := loadContract(t)
			if tt.closed {
				contract = nil
			}
			view := newWriter(t, t.TempDir(), contract).View()
			if got := view.CanReturn(tt.noteType, tt.from, tt.to); got != tt.want {
				t.Errorf("CanReturn(%q, %q, %q) = %t, want %t", tt.noteType, tt.from, tt.to, got, tt.want)
			}
		})
	}
}

// TestCanReturnFollowsAChainOfTransitions pins that the answer is
// reachability, not the presence of a direct reverse edge: trade draft's
// entry from ready for one from archived, and the walk back from ready runs
// through archived — two offered presses — so ready stays returnable even
// though no edge points from ready to draft any more.
func TestCanReturnFollowsAChainOfTransitions(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read test contract: %v", err)
	}
	modified := strings.Replace(string(data), `from = ["imported", "ready"]`, `from = ["imported", "archived"]`, 1)
	if modified == string(data) {
		t.Fatal("return-path edge replacement did not apply")
	}
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	if writeErr := os.WriteFile(path, []byte(modified), 0o600); writeErr != nil { // #nosec G703 -- fixed basename under t.TempDir
		t.Fatalf("write modified contract: %v", writeErr)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile(%q) = %v", path, err)
	}
	view := newWriter(t, t.TempDir(), contract).View()
	if !view.CanReturn("lesson", "draft", "ready") {
		t.Error(`CanReturn("lesson", "draft", "ready") = false, want true: ready walks back through archived to draft`)
	}
	if !view.CanReturn("lesson", "draft", "archived") {
		t.Error(`CanReturn("lesson", "draft", "archived") = false, want true: archived walks straight back to draft`)
	}
}

// TestCanReturnExcludesPublishedAsAPath pins the one subtlety in the walk: an
// edge into published is not a step the write face could ever offer, so a
// return that exists only by way of published does not exist here — even
// where the contract legalises both halves of that detour.
func TestCanReturnExcludesPublishedAsAPath(t *testing.T) {
	t.Parallel()
	const contractText = `schema_version = "1"

[enums]
type = ["doc"]

[enums.status]
note = ["draft", "ready", "published"]

[artifacts]
non_instance_dirs = []

[[lifecycle]]
status = "draft"
applies_to = ["doc"]
from = ["published"]
owner = ["agent"]

[[lifecycle]]
status = "ready"
applies_to = ["doc"]
from = ["draft"]
owner = ["agent"]

[[lifecycle]]
status = "published"
applies_to = ["doc"]
from = ["ready"]
owner = ["agent"]
`
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err := os.WriteFile(path, []byte(contractText), 0o600); err != nil { // #nosec G703 -- fixed basename under t.TempDir
		t.Fatalf("write test contract: %v", err)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile(%q) = %v", path, err)
	}
	view := newWriter(t, t.TempDir(), contract).View()
	if view.CanReturn("doc", "draft", "ready") {
		t.Error(`CanReturn("doc", "draft", "ready") = true, want false: the only walk back runs through published, which no control can enter`)
	}
}

func TestKnownStatus(t *testing.T) {
	t.Parallel()

	if newWriter(t, t.TempDir(), nil).View().KnownStatus("lesson", "draft") {
		t.Error("KnownStatus on a closed write face = true, want false")
	}
	view := newWriter(t, t.TempDir(), loadContract(t)).View()
	tests := []struct {
		name     string
		noteType string
		status   string
		want     bool
	}{
		{name: "declared value", noteType: "lesson", status: "draft", want: true},
		{name: "unlisted value", noteType: "lesson", status: "這是草稿", want: false},
		{name: "undeclared type", noteType: "mystery", status: "draft", want: false},
		{name: "empty value", noteType: "lesson", status: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := view.KnownStatus(tt.noteType, tt.status); got != tt.want {
				t.Errorf("KnownStatus(%q, %q) = %v, want %v", tt.noteType, tt.status, got, tt.want)
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
			root := t.TempDir()
			writer := newWriter(t, root, loadContract(t))

			writeNote(t, root, tt.content)

			if err := writer.Flip(t.Context(), testRel, "draft", schema.SealStatus, diskIdentity(tt.content)); err != nil {
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
// the install window leaves behind: a dot-prefixed temp file, invisible to
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
	writer := newWriter(t, root, loadContract(t))

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

	if err := writer.Flip(t.Context(), testRel, "draft", schema.SealStatus, diskIdentity(original)); err != nil {
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
	writer := newWriter(t, root, loadContract(t))

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

	if err := writer.Flip(t.Context(), testRel, "draft", schema.SealStatus, diskIdentity(original)); err != nil {
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
	writer := newWriter(t, root, loadContract(t))

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

	if err := writer.Flip(t.Context(), testRel, "draft", schema.SealStatus, diskIdentity(original)); err != nil {
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
// different target statuses concurrently. Flip holds the Writer's lock for
// its whole duration, so exactly one of the two must succeed and the loser
// must get a real, actionable refusal (ErrStale — the winner already moved
// the file) rather than a false success or an interleaved write.
func TestFlipSerializesConcurrentFlips(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writer := newWriter(t, root, loadContract(t))

	writeNote(t, root, lessonContent("draft"))

	var errReady, errArchived error
	var wg sync.WaitGroup
	wg.Go(func() {
		errReady = writer.Flip(t.Context(), testRel, "draft", schema.SealStatus, diskIdentity(lessonContent("draft")))
	})
	wg.Go(func() {
		errArchived = writer.Flip(t.Context(), testRel, "draft", "archived", diskIdentity(lessonContent("draft")))
	})
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
	writer := newWriter(t, root, loadContract(t))

	original := lessonContent("draft")
	writeNote(t, root, original)

	if err := writer.Flip(t.Context(), testRel, "draft", schema.SealStatus, diskIdentity(original)); err != nil {
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
	writer := newWriter(t, root, loadContract(t))

	committed := lessonContent("draft")
	writeNote(t, root, committed)
	commitAll(t, root)
	edited := committed + "<!-- an uncommitted edit -->\n"
	writeNote(t, root, edited)

	if err := writer.Flip(t.Context(), testRel, "draft", schema.SealStatus, diskIdentity(edited)); err != nil {
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
	writer := newWriter(t, root, loadContract(t))

	original := lessonContent("draft")
	writeNote(t, root, original)
	commitAll(t, root)
	before := commitCount(t, root)

	if err := writer.Flip(t.Context(), testRel, "draft", schema.SealStatus, diskIdentity(original)); err != nil {
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
	writer := newWriter(t, t.TempDir(), contract)

	got := writer.View().Transitions("Writing/doc.md", "doc", "a")
	if diff := cmp.Diff([]string{"b"}, got); diff != "" {
		t.Errorf("Transitions() mismatch (-want +got):\n%s", diff)
	}
}

// publishableContract loads a contract whose from-lists make published
// reachable from ready, alongside an ordinary retirement edge. The write
// face must keep refusing the published target even when the contract
// admits it: the value records a completed publication, and nothing in
// yomihon can attest one.
func publishableContract(t *testing.T) *schema.Contract {
	t.Helper()
	const contractText = `schema_version = "1"

[enums]
type = ["doc"]

[enums.status]
note = ["draft", "ready", "published", "archived"]

[artifacts]
non_instance_dirs = []

[[lifecycle]]
status = "draft"
applies_to = ["doc"]
from = []
owner = ["agent"]

[[lifecycle]]
status = "ready"
applies_to = ["doc"]
from = ["draft"]
owner = ["agent"]

[[lifecycle]]
status = "published"
applies_to = ["doc"]
from = ["ready"]
owner = ["agent"]

[[lifecycle]]
status = "archived"
applies_to = ["*"]
from = ["*"]
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
	return contract
}

// TestTransitionsNeverOfferPublished locks the render half of the published
// reservation: even when the contract's from-lists admit ready→published,
// the offered set omits it — published records a completed publication, and
// no interactive control may claim one happened.
func TestTransitionsNeverOfferPublished(t *testing.T) {
	t.Parallel()
	writer := newWriter(t, t.TempDir(), publishableContract(t))

	got := writer.View().Transitions("Writing/doc.md", "doc", "ready")
	if diff := cmp.Diff([]string{"archived"}, got); diff != "" {
		t.Errorf("Transitions(ready) mismatch (-want +got):\n%s", diff)
	}
}

// TestFlipRefusesPublishedTarget locks the write half of the published
// reservation: a POST whose target is published is refused before the note
// is touched, even when the contract's from-lists admit the transition.
func TestFlipRefusesPublishedTarget(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writer := newWriter(t, root, publishableContract(t))

	const rel = "Writing/doc.md"
	original := "---\ntitle: Doc\ntype: doc\nstatus: ready\n---\n\nBody.\n"
	writeVaultFile(t, root, rel, original)

	err := writer.Flip(t.Context(), rel, "ready", schema.PublishedStatus, [sha256.Size]byte{})
	if !errors.Is(err, status.ErrPublishedReserved) {
		t.Fatalf("Flip(to=published) = %v, want %v", err, status.ErrPublishedReserved)
	}
	got, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) // #nosec G304 -- a fixed in-test path under this test's TempDir
	if readErr != nil {
		t.Fatalf("read note: %v", readErr)
	}
	if string(got) != original {
		t.Errorf("note after refused flip = %q, want untouched %q", got, original)
	}
}

// TestTheSweepSaysWhatItSetAside holds the one thing this package reports
// without being asked. A flip killed mid-write leaves a file beside the note;
// an hour later the next flip in that directory moves it aside rather than
// deleting it, because deleting could destroy the only copy of a note the
// crash caught in the middle. Kept means kept forever, and the name is
// dot-prefixed, so nothing a reader opens will ever show it.
//
// So the sweep says so, once, naming both the note and the file it kept.
// Without that the design's honest half — we will not delete your bytes —
// arrived as a file nobody knew was there.
func TestTheSweepSaysWhatItSetAside(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	reader, err := vaultfs.Open(root)
	if err != nil {
		t.Fatalf("vaultfs.Open(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	var logged bytes.Buffer
	contract := loadContract(t)
	writer, err := status.Open(reader, contract, contract.Governance(),
		slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelWarn})))
	if err != nil {
		t.Fatalf("status.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := writer.Close(); closeErr != nil {
			t.Errorf("Writer.Close() error = %v", closeErr)
		}
	})

	original := lessonContent("draft")
	writeNote(t, root, original)
	dir := filepath.Dir(filepath.Join(root, filepath.FromSlash(testRel)))
	const middle = "AAAAABBBBBCCCCCDDDDDEEEEEF"
	abandoned := filepath.Join(dir, ".yomihon-status-"+middle+".tmp")
	if err := os.WriteFile(abandoned, []byte("bytes only a person can identify"), 0o600); err != nil {
		t.Fatalf("plant the abandoned temp: %v", err)
	}
	aged := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(abandoned, aged, aged); err != nil {
		t.Fatalf("age the abandoned temp: %v", err)
	}

	if err := writer.Flip(t.Context(), testRel, "draft", schema.SealStatus, diskIdentity(original)); err != nil {
		t.Fatalf("Flip() = %v, want nil", err)
	}

	said := logged.String()
	for _, want := range []string{testRel, ".yomihon-orphaned-" + middle + ".keep"} {
		if !strings.Contains(said, want) {
			t.Errorf("the sweep kept a file without naming %q; it said:\n%s", want, said)
		}
	}

	// The control: an ordinary flip with nothing to set aside says nothing at
	// all, or the line above would be noise on every write.
	logged.Reset()
	after := strings.Replace(original, "status: draft", "status: "+schema.SealStatus, 1)
	if err := writer.Flip(t.Context(), testRel, schema.SealStatus, "archived", diskIdentity(after)); err != nil {
		t.Fatalf("second Flip() = %v, want nil", err)
	}
	if logged.Len() != 0 {
		t.Errorf("a flip with nothing abandoned still reported:\n%s", logged.String())
	}
}

// TestConstructorsRefuseAWiringBugTheSameWay covers every nil this package's
// two constructors cannot work without. A nil reader used to come back as an
// ordinary error while its five siblings panicked, which offered callers a
// recovery from something no caller can recover from: there is no second
// vault to try. Open's error return stays for the failures that are real —
// a root that will not open, a root that moved while it was being pinned.
func TestConstructorsRefuseAWiringBugTheSameWay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		call func(t *testing.T)
		want string
	}{
		{
			name: "a writer with no vault to write into",
			call: func(t *testing.T) {
				t.Helper()
				_, _ = status.Open(nil, nil, schema.Ungoverned(), slog.New(slog.DiscardHandler)) //nolint:errcheck // the call panics before it returns
			},
			want: "status: Open requires a non-nil Reader",
		},
		{
			name: "a route with no writer behind it",
			call: func(t *testing.T) {
				t.Helper()
				status.NewHandler(nil, func() pages.Shell { return pages.Shell{} }, slog.New(slog.DiscardHandler))
			},
			want: "status: NewHandler requires a non-nil Writer",
		},
		{
			name: "a route with no shell to draw",
			call: func(t *testing.T) {
				t.Helper()
				status.NewHandler(&status.Writer{}, nil, slog.New(slog.DiscardHandler))
			},
			want: "status: NewHandler requires a non-nil shell provider",
		},
		{
			name: "a route with nowhere to report",
			call: func(t *testing.T) {
				t.Helper()
				status.NewHandler(&status.Writer{}, func() pages.Shell { return pages.Shell{} }, nil)
			},
			want: "status: NewHandler requires a non-nil logger",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if got := recover(); got != tt.want {
					t.Errorf("panic = %v, want %q", got, tt.want)
				}
			}()
			tt.call(t)
		})
	}
}
