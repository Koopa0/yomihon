package status

// The install window — between the byte-for-byte confirmation that the
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

// seedInstallNote lays down a lesson note and returns its absolute path.
func seedInstallNote(t *testing.T, root string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(installRel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir note parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(internalLesson()), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	return path
}

// externalWriters are the two ways a cooperating local editor replaces a
// note's bytes. Both must be survivable: whichever one lands inside the
// install window, its bytes have to be the ones on disk afterwards.
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
			external := strings.Replace(internalLesson(), "body", "external edit", 1)
			if external == internalLesson() {
				t.Fatal("the external edit is identical to the note, so this proves nothing")
			}

			err := lifecycle.flip(installRel, "draft", schema.SealStatus, internalLessonIdentity(), flipHooks{
				beforeInstall: func() { writer.write(t, path, []byte(external)) },
			})

			if !errors.Is(err, ErrConcurrentWrite) {
				t.Errorf("Flip() with an external edit in the install window = %v, want %v", err, ErrConcurrentWrite)
			}
			got, readErr := os.ReadFile(path) // #nosec G304 -- a fixed relative note under this test's TempDir
			if readErr != nil {
				t.Fatalf("read note after the raced flip: %v", readErr)
			}
			if string(got) != external {
				t.Errorf("note after the raced flip =\n%q\nwant the external edit:\n%q", got, external)
			}
			assertNoStatusTemps(t, filepath.Dir(path))
		})
	}
}

// statusEntries lists the install residue in dir: the temporary names a
// flip creates, and the names an abandoned one is moved aside to.
func statusEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read install directory: %v", err)
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

// TestStrandedInstallDeletesNothing covers the outcome that has no clean
// ending: another program edits the note inside the window and putting that
// edit back fails too. Both versions have to survive — one under the note's
// name and one beside it — because the write face cannot tell which of them
// the operator wants, and the entry beside the note is the only copy of the
// other one.
func TestStrandedInstallDeletesNothing(t *testing.T) {
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
	err = replaceRegularFile(parent, rel, rel, &source, []byte("replacement"), installHooks{
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

	if !errors.Is(err, ErrInstallStranded) {
		t.Fatalf("replaceRegularFile() = %v, want %v", err, ErrInstallStranded)
	}
	residue := statusEntries(t, dir)
	if len(residue) != 1 {
		t.Fatalf("install residue = %v, want exactly the entry holding the other version", residue)
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

	err = replaceRegularFile(parent, rel, rel, &source, []byte("replacement"), installHooks{
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

// TestExchangeRefusesANameThatIsNotOneEntry is the whole of the confinement
// argument for the raw directory-relative swap. The descriptor comes from a
// directory the per-component walk already validated, so the swap stays inside
// that directory exactly as long as neither name can reach out of it. Nothing
// else in the install re-checks that, which is why the check is asserted
// here rather than trusted.
func TestExchangeRefusesANameThatIsNotOneEntry(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	parent := internalRoot(t, dir)
	for _, name := range []string{"", ".", "..", "sub/note.md", "/note.md", "../note.md"} {
		if err := exchangeAt(parent, name, "note.md"); !errors.Is(err, errNotSingleComponent) {
			t.Errorf("exchangeAt(from=%q) = %v, want %v", name, err, errNotSingleComponent)
		}
		if err := exchangeAt(parent, "note.md", name); !errors.Is(err, errNotSingleComponent) {
			t.Errorf("exchangeAt(to=%q) = %v, want %v", name, err, errNotSingleComponent)
		}
	}
	// Two ordinary entry names get past the guard and reach the filesystem,
	// so the refusals above are the guard's answer and not a blanket one.
	for _, name := range []string{"first.md", "second.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := exchangeAt(parent, "first.md", "second.md"); err != nil {
		t.Fatalf("exchangeAt() on two ordinary names = %v, want the swap to be attempted", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "first.md")) // #nosec G304 -- a fixed name under this test's TempDir
	if err != nil {
		t.Fatalf("read first.md: %v", err)
	}
	if string(got) != "second.md" {
		t.Errorf("first.md = %q after the swap, want the other entry's bytes", got)
	}
}

// TestProbeFailureDoesNotPinTheFilesystem locks the rule that a rung reached
// by a failure is not remembered. A directory that cannot be written to for a
// moment — a full disk, a permission the owner is in the middle of changing —
// would otherwise teach the whole filesystem that it is only good enough for a
// plain rename, and every later flip on that volume, on any note, would take
// the weakest install for the life of the process.
func TestProbeFailureDoesNotPinTheFilesystem(t *testing.T) {
	dir := t.TempDir()
	parent, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatalf("opening %s: %v", dir, err)
	}
	defer func() {
		if closeErr := parent.Close(); closeErr != nil {
			t.Errorf("closing root: %v", closeErr)
		}
	}()

	key, ok := deviceKey(parent)
	if !ok {
		t.Skip("this platform does not identify the filesystem, so nothing is remembered to begin with")
	}
	// Any earlier probe in this process already answered for this device, and a
	// warm entry would make every assertion below pass without running a probe
	// at all.
	exchangeProbes.Delete(key)

	// Establish what this filesystem can really do, then forget it again, so a
	// failure to reproduce the strong rung later is about the cache and not
	// about the volume.
	earned := selectRung(parent, installHooks{})
	if earned == rungRename {
		t.Skip("this filesystem earns only a plain rename, so there is no stronger answer for a failure to hide")
	}
	exchangeProbes.Delete(key)

	// Take away the probe's ability to write its own throwaway files. The
	// directory is otherwise untouched: 0500 keeps the traverse bit the probe
	// needs to look inside while removing the write bit it needs to create
	// them, which is exactly the failure under test.
	if chmodErr := os.Chmod(dir, 0o500); chmodErr != nil { // #nosec G302 -- a directory mode, not a file mode; this test's own TempDir
		t.Fatalf("making %s unwritable: %v", dir, chmodErr)
	}
	if failed := selectRung(parent, installHooks{}); failed != rungRename {
		t.Fatalf("selectRung on an unwritable directory = %v, want %v; the probe was expected to fail here", failed, rungRename)
	}
	if chmodErr := os.Chmod(dir, 0o700); chmodErr != nil { // #nosec G302 -- restoring the owner-only directory mode t.TempDir created
		t.Fatalf("restoring %s: %v", dir, chmodErr)
	}

	if got := selectRung(parent, installHooks{}); got != earned {
		t.Errorf("selectRung after the directory became writable again = %v, want %v; the momentary failure was remembered and pinned the filesystem to the weakest install", got, earned)
	}
}

// TestInstallRungMatrix runs every install rung the product can select through
// the two race timings a test can place, and states what each one is worth.
// The ladder exists because volumes differ in what they can promise, so the
// promise each rung makes is the thing to lock: a rung whose weakness is
// written down but never executed is a claim nobody has checked.
//
// One cell in the matrix is out of reach from here and is deliberately absent
// rather than approximated: the hardlink rung's second window, between taking
// the extra name and the rename that installs, needs a seam inside the install
// itself. beforeInstall fires before the install is entered, so it places a
// writer in the first window only.
func TestInstallRungMatrix(t *testing.T) {
	t.Parallel()

	const (
		original    = "original"
		replacement = "replacement"
		external    = "another program's edit"
	)

	tests := []struct {
		name string
		rung installRung
		// raced places an external in-place write inside the install window.
		raced bool
		// wantErr is the sentinel the install must report, or nil for success.
		wantErr error
		// wantContent is what the note holds afterwards.
		wantContent string
	}{
		{
			name:        "exchange installs",
			rung:        rungExchange,
			wantContent: replacement,
		},
		{
			name:        "exchange refuses a raced install and keeps the other edit",
			rung:        rungExchange,
			raced:       true,
			wantErr:     ErrConcurrentWrite,
			wantContent: external,
		},
		{
			name:        "hardlink installs",
			rung:        rungHardlink,
			wantContent: replacement,
		},
		{
			name:        "hardlink refuses a raced install and keeps the other edit",
			rung:        rungHardlink,
			raced:       true,
			wantErr:     ErrConcurrentWrite,
			wantContent: external,
		},
		{
			name:        "rename installs",
			rung:        rungRename,
			wantContent: replacement,
		},
		{
			// The weakest rung promises nothing about this window, which is why
			// it is last and why a volume reaches it only when the two above
			// were refused. A plain rename replaces the note whole, so the
			// other edit is lost rather than torn, and no syscall on this path
			// could have reported it. Locking that here makes the cost visible
			// and means an improvement has to be a deliberate edit to this
			// table rather than a silent change of behaviour.
			name:        "rename overwrites a raced install without noticing",
			rung:        rungRename,
			raced:       true,
			wantContent: replacement,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			parent := internalRoot(t, dir)
			const rel = "note.md"
			path := filepath.Join(dir, rel)
			if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
				t.Fatalf("write original: %v", err)
			}
			source, err := readRegularFile(parent, rel, rel)
			if err != nil {
				t.Fatalf("readRegularFile() error = %v", err)
			}

			hooks := installHooks{rung: func() installRung { return tt.rung }}
			if tt.raced {
				hooks.beforeInstall = func() {
					if writeErr := os.WriteFile(path, []byte(external), 0o600); writeErr != nil {
						t.Errorf("external write: %v", writeErr)
					}
				}
			}

			err = replaceRegularFile(parent, rel, rel, &source, []byte(replacement), hooks, func() error { return nil })
			switch {
			case tt.wantErr == nil && err != nil:
				t.Fatalf("replaceRegularFile() = %v, want success", err)
			case tt.wantErr != nil && !errors.Is(err, tt.wantErr):
				t.Fatalf("replaceRegularFile() = %v, want %v", err, tt.wantErr)
			}

			got, readErr := os.ReadFile(path) // #nosec G304 -- a fixed name under t.TempDir
			if readErr != nil {
				t.Fatalf("read note: %v", readErr)
			}
			if string(got) != tt.wantContent {
				t.Errorf("note holds %q, want %q", got, tt.wantContent)
			}
		})
	}
}

// TestOnlyAMeasuredRungIsRemembered covers the other half of the same rule. A
// swap can fail for reasons that say nothing about the driver — an interrupted
// syscall, a moment of I/O trouble — and the probe then falls through to the
// hard link, which succeeds. Remembering that answer pins a volume that can
// swap to the rung that cannot see a replacement arriving mid-install, for the
// rest of the process and for every note on it.
// It cannot run in parallel, and neither can its sibling above: both own the
// process-wide probe cache for their duration, and every flip in this package
// writes to it under the same device key.
func TestOnlyAMeasuredRungIsRemembered(t *testing.T) {
	dir := t.TempDir()
	parent, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatalf("opening %s: %v", dir, err)
	}
	defer func() {
		if closeErr := parent.Close(); closeErr != nil {
			t.Errorf("closing root: %v", closeErr)
		}
	}()

	key, ok := deviceKey(parent)
	if !ok {
		t.Skip("this platform does not identify the filesystem, so nothing is remembered to begin with")
	}
	exchangeProbes.Delete(key)
	earned := selectRung(parent, installHooks{})
	if earned != rungExchange {
		t.Skipf("this filesystem earns %v, so there is no swap for a passing failure to hide", earned)
	}
	exchangeProbes.Delete(key)

	// One refusal the driver did not mean: everything else about the volume is
	// untouched, so the probe falls through to the hard link and answers with
	// the weaker rung.
	degraded := selectRung(parent, installHooks{
		ops: func(base installOps) installOps {
			base.swap = func(string, string) error { return errors.New("interrupted") }
			return base
		},
	})
	if degraded != rungHardlink {
		t.Fatalf("selectRung with a refused swap = %v, want %v; the probe was expected to fall through to the second name", degraded, rungHardlink)
	}

	if got := selectRung(parent, installHooks{}); got != earned {
		t.Errorf("selectRung after a passing swap failure = %v, want %v; the weaker answer was remembered, so every later flip on this volume takes an install that cannot see a concurrent replacement", got, earned)
	}
}
