package status

// The publication window — between the byte-for-byte confirmation that the
// note is unchanged and the moment the rewritten bytes take its name — is the
// one interval a flip cannot revalidate. These tests drive an external writer
// into exactly that interval through the beforeInstall seam, in the two shapes
// a real editor uses: an in-place truncating write, and an atomic
// rename-replace from a sibling temporary file.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

const installRel = "Writing/lessons/japanese/L05.md"

// seedInstallNote lays down a committed lesson and returns its absolute path.
func seedInstallNote(t *testing.T, root string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(installRel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir note parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(internalLesson()), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	internalRunGit(t, root, "add", "-A")
	internalRunGit(t, root, "commit", "-m", "seed lesson")
	return path
}

// externalWriters are the two ways a cooperating local editor replaces a
// note's bytes. Both must be survivable: whichever one lands inside the
// publication window, its bytes have to be the ones on disk afterwards.
var externalWriters = []struct {
	name  string
	write func(t *testing.T, path string, data []byte)
}{
	{
		name: "in place",
		write: func(t *testing.T, path string, data []byte) {
			t.Helper()
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatalf("external in-place write: %v", err)
			}
		},
	},
	{
		name: "rename replace",
		write: func(t *testing.T, path string, data []byte) {
			t.Helper()
			sibling := filepath.Join(filepath.Dir(path), "external-editor-temp")
			if err := os.WriteFile(sibling, data, 0o600); err != nil {
				t.Fatalf("external sibling write: %v", err)
			}
			if err := os.Rename(sibling, path); err != nil {
				t.Fatalf("external rename replace: %v", err)
			}
		},
	},
}

func TestFlipPreservesAnExternalEditRacingTheInstall(t *testing.T) {
	t.Parallel()

	for _, writer := range externalWriters {
		t.Run(writer.name, func(t *testing.T) {
			t.Parallel()

			root, lifecycle := internalVault(t)
			path := seedInstallNote(t, root)
			commitsBefore := internalCommitCount(t, root)
			external := strings.Replace(internalLesson(), "body", "external edit", 1)
			if external == internalLesson() {
				t.Fatal("the external edit is identical to the note, so this proves nothing")
			}

			err := lifecycle.flip(t.Context(), installRel, "draft", schema.SealStatus, flipHooks{
				beforeInstall: func() { writer.write(t, path, []byte(external)) },
			})

			if !errors.Is(err, ErrConcurrentWrite) {
				t.Errorf("Flip() with an external edit in the publication window = %v, want %v", err, ErrConcurrentWrite)
			}
			got, readErr := os.ReadFile(path) // #nosec G304 -- a fixed relative note under this test's TempDir
			if readErr != nil {
				t.Fatalf("read note after the raced flip: %v", readErr)
			}
			if string(got) != external {
				t.Errorf("note after the raced flip =\n%q\nwant the external edit:\n%q", got, external)
			}
			if commitsAfter := internalCommitCount(t, root); commitsAfter != commitsBefore {
				t.Errorf("commit count = %d, want %d: a refused flip records no receipt", commitsAfter, commitsBefore)
			}
			assertNoStatusTemps(t, filepath.Dir(path))
		})
	}
}

// statusEntries lists the publication residue in dir: the temporary names a
// flip creates, and the names an abandoned one is moved aside to.
func statusEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read publication directory: %v", err)
	}
	var found []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), statusTempPrefix) || strings.HasPrefix(entry.Name(), statusOrphanPrefix) {
			found = append(found, entry.Name())
		}
	}
	return found
}

// TestWriteTempNameShapeIsTheOneTheSweepRecognizes ties the sweep's matcher to
// the names writeTemp actually produces. If crypto/rand's Text ever returns a
// different length or alphabet, the sweep would stop recognizing yomihon's own
// temps and this is where that shows up.
func TestWriteTempNameShapeIsTheOneTheSweepRecognizes(t *testing.T) {
	t.Parallel()

	name := tempName()
	middle, ok := writeTempMiddle(name)
	if !ok {
		t.Fatalf("writeTempMiddle(%q) = _, false, want the sweep to recognize a name writeTemp created", name)
	}
	if len(middle) != tempRandomLength {
		t.Errorf("random middle of %q is %d characters, want %d", name, len(middle), tempRandomLength)
	}
	for _, decoy := range []string{
		".yomihon-status-SHORT.tmp",
		".yomihon-status-" + strings.ToLower(middle) + ".tmp",
		".yomihon-status-" + middle + ".keep",
		"yomihon-status-" + middle + ".tmp",
		".yomihon-orphaned-" + middle + ".keep",
	} {
		if _, matched := writeTempMiddle(decoy); matched {
			t.Errorf("writeTempMiddle(%q) matched, want a name outside the shape to be left alone", decoy)
		}
	}
}

// TestProbeRefusesAFilesystemThatOnlyReportsSuccess is the gate the exchange
// rung stands on. Some volumes accept the swap request, return success, and
// perform a destructive plain rename instead — trusting the return value there
// would destroy exactly the version the exchange exists to keep. The probe
// therefore believes only what it can read back afterwards.
func TestProbeRefusesAFilesystemThatOnlyReportsSuccess(t *testing.T) {
	t.Parallel()

	unsupported := errors.New("operation not supported")
	tests := []struct {
		name string
		ops  func(parent *os.Root, base installOps) installOps
		want installRung
	}{
		{
			name: "this vault's filesystem",
			ops:  func(_ *os.Root, base installOps) installOps { return base },
			want: rungExchange,
		},
		{
			name: "success that destroys one entry",
			ops: func(parent *os.Root, base installOps) installOps {
				base.swap = func(from, to string) error { return parent.Rename(from, to) }
				return base
			},
			want: rungHardlink,
		},
		{
			name: "success that swaps nothing",
			ops: func(_ *os.Root, base installOps) installOps {
				base.swap = func(string, string) error { return nil }
				return base
			},
			want: rungHardlink,
		},
		{
			name: "exchange refused, second names allowed",
			ops: func(_ *os.Root, base installOps) installOps {
				base.swap = func(string, string) error { return unsupported }
				return base
			},
			want: rungHardlink,
		},
		{
			name: "exchange and second names both refused",
			ops: func(_ *os.Root, base installOps) installOps {
				base.swap = func(string, string) error { return unsupported }
				base.link = func(string, string) error { return unsupported }
				return base
			},
			want: rungRename,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			parent := internalRoot(t, dir)
			if got := probeRung(parent, tt.ops(parent, rootOps(parent))); got != tt.want {
				t.Errorf("probeRung() = %v, want %v", got, tt.want)
			}
			if residue := statusEntries(t, dir); len(residue) != 0 {
				t.Errorf("probe left %v behind, want every probe file cleaned up", residue)
			}
		})
	}
}

// TestStrandedPublicationDeletesNothing covers the outcome that has no clean
// ending: another program edits the note inside the window and putting that
// edit back fails too. Both versions have to survive — one under the note's
// name and one beside it — because the write face cannot tell which of them
// the operator wants, and the entry beside the note is the only copy of the
// other one.
func TestStrandedPublicationDeletesNothing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	parent := internalRoot(t, dir)
	const rel = "note.md"
	path := filepath.Join(dir, rel)
	const original = "original"
	const external = "another program's edit"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write original: %v", err)
	}
	source, err := readRegularFile(parent, rel, rel)
	if err != nil {
		t.Fatalf("readRegularFile() error = %v", err)
	}

	swaps := 0
	err = replaceRegularFile(parent, rel, rel, &source, []byte("replacement"), publishHooks{
		rung: func() installRung { return rungExchange },
		beforeInstall: func() {
			if writeErr := os.WriteFile(path, []byte(external), 0o600); writeErr != nil {
				t.Fatalf("external write: %v", writeErr)
			}
		},
		ops: func(base installOps) installOps {
			swap := base.swap
			base.swap = func(from, to string) error {
				swaps++
				if swaps > 1 {
					return errors.New("the volume stopped accepting the swap")
				}
				return swap(from, to)
			}
			return base
		},
	}, func() error { return nil })

	if !errors.Is(err, ErrPublicationStranded) {
		t.Fatalf("replaceRegularFile() = %v, want %v", err, ErrPublicationStranded)
	}
	residue := statusEntries(t, dir)
	if len(residue) != 1 {
		t.Fatalf("publication residue = %v, want exactly the entry holding the other version", residue)
	}
	if !strings.Contains(err.Error(), residue[0]) {
		t.Errorf("error %q does not name %q, and nothing else tells the operator where the other version is", err, residue[0])
	}
	kept, readErr := os.ReadFile(filepath.Join(dir, residue[0])) // #nosec G304 -- an entry this test just listed under its own TempDir
	if readErr != nil {
		t.Fatalf("read the retained version: %v", readErr)
	}
	if string(kept) != external {
		t.Errorf("retained version = %q, want the other program's bytes %q", kept, external)
	}
}

// TestRetainedHardlinkInstallPutsBackAnInPlaceEdit exercises the fallback
// rung, for volumes whose driver cannot swap two entries atomically. Keeping a
// second name for the current version catches an editor that writes through
// the file it already has open, which is what this covers. It does not catch an
// editor that replaces the note's name after that second name is taken — that
// edit is lost, which is why this rung is the fallback and why the guarantee
// is stated per rung rather than once for the whole write face.
func TestRetainedHardlinkInstallPutsBackAnInPlaceEdit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	parent := internalRoot(t, dir)
	const rel = "note.md"
	path := filepath.Join(dir, rel)
	const external = "another program's edit"
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatalf("write original: %v", err)
	}
	source, err := readRegularFile(parent, rel, rel)
	if err != nil {
		t.Fatalf("readRegularFile() error = %v", err)
	}

	err = replaceRegularFile(parent, rel, rel, &source, []byte("replacement"), publishHooks{
		rung: func() installRung { return rungHardlink },
		beforeInstall: func() {
			if writeErr := os.WriteFile(path, []byte(external), 0o600); writeErr != nil {
				t.Fatalf("external write: %v", writeErr)
			}
		},
	}, func() error { return nil })

	if !errors.Is(err, ErrConcurrentWrite) {
		t.Fatalf("replaceRegularFile() on the fallback rung = %v, want %v", err, ErrConcurrentWrite)
	}
	if !strings.Contains(err.Error(), rungHardlink.String()) {
		t.Errorf("error %q does not name the rung it ran on, so the guarantee cannot be bound to it", err)
	}
	got, readErr := os.ReadFile(path) // #nosec G304 -- a fixed name under this test's TempDir
	if readErr != nil {
		t.Fatalf("read note: %v", readErr)
	}
	if string(got) != external {
		t.Errorf("note = %q, want the other program's bytes %q", got, external)
	}
	if residue := statusEntries(t, dir); len(residue) != 0 {
		t.Errorf("residue = %v, want the retained name cleared once the edit was put back", residue)
	}
}
