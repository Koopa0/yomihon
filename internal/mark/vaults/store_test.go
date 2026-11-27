package vaults

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/mark/confdir"
)

// folder is a directory standing for a vault, named so a test can tell it from
// another.
func folder(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

// storeIn is a Store over configDir that reads the clock a test sets and gives
// up on a held lock quickly.
func storeIn(t *testing.T, configDir string, free func(int) bool) (*Store, *time.Time) {
	t.Helper()
	s, err := Open(confdir.At(configDir), free)
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return clock }
	s.wait = 200 * time.Millisecond
	return s, &clock
}

// startPort is where a test's vaults begin.
const startPort = 8800

func mustAdd(t *testing.T, s *Store, root string) Entry {
	t.Helper()
	e, _, err := s.Add(root, startPort)
	if err != nil {
		t.Fatalf("Add(%q): %v", root, err)
	}
	return e
}

func TestTheFirstVaultTakesTheStartingPortAndEachNextTheFirstFreeOneAbove(t *testing.T) {
	t.Parallel()
	s, _ := storeIn(t, t.TempDir(), nil)

	first := mustAdd(t, s, folder(t, "a"))
	second := mustAdd(t, s, folder(t, "b"))
	third := mustAdd(t, s, folder(t, "c"))
	if first.Port != 8800 || second.Port != 8801 || third.Port != 8802 {
		t.Fatalf("ports = %d, %d, %d, want 8800, 8801, 8802", first.Port, second.Port, third.Port)
	}
}

func TestAPortSomethingElseHoldsIsSkippedAndTheChoiceIsWrittenDown(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	s, _ := storeIn(t, config, func(port int) bool { return port != 8801 && port != 8802 })

	mustAdd(t, s, folder(t, "a"))
	second := mustAdd(t, s, folder(t, "b"))
	if second.Port != 8803 {
		t.Fatalf("with 8801 and 8802 held by other programs the second vault took %d, want 8803", second.Port)
	}

	// A new process finds the port where it was left, whatever is free now.
	again, _ := storeIn(t, config, nil)
	list, err := again.List()
	if err != nil {
		t.Fatal(err)
	}
	if i := slices.IndexFunc(list, func(e Entry) bool { return e.ID == second.ID }); i < 0 || list[i].Port != 8803 {
		t.Fatalf("a new process reads %+v, want the second vault still on 8803", list)
	}
}

func TestAVaultTypedAgainKeepsItsPortAndIsNotNew(t *testing.T) {
	t.Parallel()
	s, clock := storeIn(t, t.TempDir(), nil)
	dir := folder(t, "a")

	first, added, err := s.Add(dir, 8800)
	if err != nil || !added {
		t.Fatalf("first Add = %+v, added %v, %v", first, added, err)
	}
	*clock = clock.Add(time.Hour)
	again, added, err := s.Add(dir, 9900)
	if err != nil || added {
		t.Fatalf("second Add = %+v, added %v, %v; want the same vault, not new", again, added, err)
	}
	if again.ID != first.ID || again.Port != 8800 {
		t.Errorf("typed again, the vault is %q on %d, want %q on 8800", again.ID, again.Port, first.ID)
	}
	if !again.LastRead.After(first.LastRead) {
		t.Errorf("typed again an hour later, last read is %v, want later than %v", again.LastRead, first.LastRead)
	}
}

func TestAStartingPortThatIsNotOneIsRefused(t *testing.T) {
	t.Parallel()
	s, _ := storeIn(t, t.TempDir(), nil)
	for _, start := range []int{0, -1, 80, MinPort - 1, 65536} {
		if _, _, err := s.Add(folder(t, "a"), start); !errors.Is(err, ErrStartPort) {
			t.Errorf("Add with start %d = %v, want ErrStartPort", start, err)
		}
	}
}

func TestEveryPortFromTheStartUpBeingTakenIsAnError(t *testing.T) {
	t.Parallel()
	s, _ := storeIn(t, t.TempDir(), func(int) bool { return false })
	if _, _, err := s.Add(folder(t, "a"), 65530); !errors.Is(err, ErrNoPort) {
		t.Fatalf("Add with no free port = %v, want ErrNoPort", err)
	}
}

func TestAVaultIsIdentifiedByItsResolvedFolder(t *testing.T) {
	t.Parallel()
	s, _ := storeIn(t, t.TempDir(), nil)
	dir := folder(t, "real")
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Skipf("symlinks are not available here: %v", err)
	}

	first := mustAdd(t, s, dir)
	second, added, err := s.Add(link, 8800)
	if err != nil {
		t.Fatal(err)
	}
	if added || second.ID != first.ID {
		t.Fatalf("a folder and a symlink to it are %q and %q (new: %v), want one vault", first.ID, second.ID, added)
	}
}

func TestAVaultsIDIsTheNameOfItsDirectoryOfMarks(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	s, _ := storeIn(t, config, nil)
	e := mustAdd(t, s, folder(t, "a"))

	marks, err := confdir.VaultDir(confdir.At(config), e.Path)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(marks.Path()) != e.ID {
		t.Fatalf("the vault's marks are kept in %q but its id is %q: adding it again after removing it would not find them", filepath.Base(marks.Path()), e.ID)
	}
}

func TestAVaultIsNamedByItsFolderUntilItIsCalledSomethingElse(t *testing.T) {
	t.Parallel()
	s, _ := storeIn(t, t.TempDir(), nil)
	e := mustAdd(t, s, folder(t, "mneme"))
	if e.Name != "mneme" {
		t.Fatalf("a new vault is called %q, want its folder's name", e.Name)
	}
}

func TestTwoProcessesAddingAtOnceBothLandWithTheirOwnPorts(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	const processes, each = 2, 6
	stores := make([]*Store, processes)
	for i := range stores {
		stores[i], _ = storeIn(t, config, nil)
		stores[i].wait = 30 * time.Second
	}

	var wg sync.WaitGroup
	folders := make([]string, 0, processes*each)
	for i := range processes {
		for j := range each {
			dir := folder(t, "v"+strconv.Itoa(i)+"-"+strconv.Itoa(j))
			folders = append(folders, dir)
			wg.Go(func() {
				if _, _, err := stores[i].Add(dir, 8800); err != nil {
					t.Errorf("Add(%q): %v", dir, err)
				}
			})
		}
	}
	wg.Wait()

	list, err := stores[0].List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != len(folders) {
		t.Fatalf("%d vaults in the list after %d were added at once: a change was lost", len(list), len(folders))
	}
	ports := map[int]bool{}
	for _, e := range list {
		if ports[e.Port] {
			t.Fatalf("two vaults share port %d", e.Port)
		}
		ports[e.Port] = true
	}
}

func TestTheListIsOrderedByLastReadWithTheLatestFirst(t *testing.T) {
	t.Parallel()
	s, clock := storeIn(t, t.TempDir(), nil)
	a := mustAdd(t, s, folder(t, "a"))
	*clock = clock.Add(time.Hour)
	b := mustAdd(t, s, folder(t, "b"))
	*clock = clock.Add(time.Hour)
	c := mustAdd(t, s, folder(t, "c"))
	*clock = clock.Add(time.Hour)
	if _, err := s.Touch(a.ID, "", *clock); err != nil {
		t.Fatal(err)
	}

	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range list {
		got = append(got, e.ID)
	}
	if want := []string{a.ID, c.ID, b.ID}; !slices.Equal(got, want) {
		t.Fatalf("order = %v, want a, c, b: the one read last first", got)
	}
}

func TestALastReadIsWrittenAtMostOnceAMinuteAndANewNameAtOnce(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	s, clock := storeIn(t, config, nil)
	e := mustAdd(t, s, folder(t, "a"))
	start := *clock

	// A page loaded seconds later writes nothing, and costs no file access.
	if err := os.Chmod(filepath.Join(config, "yomihon", listFile), 0o000); err != nil {
		t.Skipf("cannot make the list unreadable here: %v", err)
	}
	defer os.Chmod(filepath.Join(config, "yomihon", listFile), 0o600) //nolint:errcheck // restoring a temporary directory's file
	if written, err := s.Touch(e.ID, "", start.Add(30*time.Second)); err != nil || written {
		t.Fatalf("Touch 30s later = written %v, %v; want it answered from memory", written, err)
	}
	if written, err := s.Touch(e.ID, "a", start.Add(30*time.Second)); err != nil || written {
		t.Fatalf("Touch with the name it already has = written %v, %v; want nothing", written, err)
	}
	if err := os.Chmod(filepath.Join(config, "yomihon", listFile), 0o600); err != nil {
		t.Fatal(err)
	}

	if written, err := s.Touch(e.ID, "", start.Add(2*time.Minute)); err != nil || !written {
		t.Fatalf("Touch 2 minutes later = written %v, %v; want a write", written, err)
	}
	if written, err := s.Touch(e.ID, "A Better Name", start.Add(2*time.Minute+time.Second)); err != nil || !written {
		t.Fatalf("Touch with a new name = written %v, %v; want a write at once", written, err)
	}
	list, _ := s.List() //nolint:errcheck // asserted on below
	if len(list) != 1 || list[0].Name != "A Better Name" || !list[0].LastRead.Equal(start.Add(2*time.Minute)) {
		t.Fatalf("list = %+v, want the new name and the last read of the first write after the minute", list)
	}
}

func TestTouchOfAVaultNotInTheListIsAnError(t *testing.T) {
	t.Parallel()
	s, clock := storeIn(t, t.TempDir(), nil)
	if _, err := s.Touch(confdir.VaultID("/nowhere"), "x", *clock); !errors.Is(err, ErrUnknown) {
		t.Fatalf("Touch of an unknown id = %v, want ErrUnknown", err)
	}
}

func TestANameIsKeptAsOneLineOfValidTextThatIsNotTooLong(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ in, want string }{
		{"  two   words\t\n here ", "two words here"},
		{"bell\x07here", "bell here"},
		{"bad\xffbytes", "bad�bytes"},
		{"", ""},
	} {
		if got := cleanName(tt.in, ""); got != tt.want {
			t.Errorf("cleanName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
	long := cleanName(repeat("字", 500), "")
	if len(long) > maxNameBytes || len([]rune(long)) == 0 {
		t.Errorf("a long name is %d bytes, want 1..%d", len(long), maxNameBytes)
	}
	if got := cleanName("", "/vaults/notes"); got != "notes" {
		t.Errorf("an empty name with a path = %q, want the folder's name", got)
	}
}

func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for range n {
		out = append(out, s...)
	}
	return string(out)
}

func TestARemovedVaultComesBackWithTheSameIDAndPort(t *testing.T) {
	t.Parallel()
	s, _ := storeIn(t, t.TempDir(), nil)
	mustAdd(t, s, folder(t, "a"))
	b := mustAdd(t, s, folder(t, "b"))

	gone, err := s.Remove(b.ID)
	if err != nil || gone.ID != b.ID {
		t.Fatalf("Remove = %+v, %v", gone, err)
	}
	if list, _ := s.List(); len(list) != 1 { //nolint:errcheck // asserted on the length
		t.Fatalf("after Remove the list holds %d vaults, want 1", len(list))
	}
	back, err := s.Restore(b.ID)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if back.ID != b.ID || back.Port != b.Port || back.Path != b.Path || back.Name != b.Name {
		t.Fatalf("Restore = %+v, want %+v back as it was", back, b)
	}
	if list, _ := s.List(); len(list) != 2 { //nolint:errcheck // asserted on the length
		t.Fatalf("after Restore the list holds %d vaults, want 2", len(list))
	}
}

func TestRestoreOnlyBringsBackAVaultThisStoreRemovedAndStillRemembers(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	s, _ := storeIn(t, config, nil)
	kept := mustAdd(t, s, folder(t, "kept"))
	removedElsewhere := mustAdd(t, s, folder(t, "elsewhere"))

	other, _ := storeIn(t, config, nil)
	if _, err := other.Remove(removedElsewhere.ID); err != nil {
		t.Fatal(err)
	}
	before := readFile(t, filepath.Join(config, "yomihon", listFile))

	for name, id := range map[string]string{
		"a path":                          "/Users/someone/notes",
		"a path that is a folder here":    folder(t, "never-added"),
		"an id it never held":             confdir.VaultID("/never/seen"),
		"an id one still in the list has": kept.ID,
		"an id another process removed":   removedElsewhere.ID,
		"nothing":                         "",
	} {
		if got, err := s.Restore(id); !errors.Is(err, ErrNotRemoved) {
			t.Errorf("Restore(%s) = %+v, %v; want ErrNotRemoved", name, got, err)
		}
	}
	after := readFile(t, filepath.Join(config, "yomihon", listFile))
	if !bytes.Equal(before, after) {
		t.Errorf("a refused Restore changed the list:\nbefore %s\nafter  %s", before, after)
	}
}

func TestOnlyTheRecentRemovalsAreRemembered(t *testing.T) {
	t.Parallel()
	s, _ := storeIn(t, t.TempDir(), nil)
	var ids []string
	for i := range rememberRemoved + 1 {
		e := mustAdd(t, s, folder(t, "v"+strconv.Itoa(i)))
		ids = append(ids, e.ID)
		if _, err := s.Remove(e.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Restore(ids[0]); !errors.Is(err, ErrNotRemoved) {
		t.Errorf("the oldest removal is still remembered after %d more: %v", rememberRemoved, err)
	}
	if _, err := s.Restore(ids[len(ids)-1]); err != nil {
		t.Errorf("the latest removal was forgotten: %v", err)
	}
}

func TestARestoredVaultWhosePortWasGivenAwayTakesTheNextFreeOne(t *testing.T) {
	t.Parallel()
	s, _ := storeIn(t, t.TempDir(), nil)
	mustAdd(t, s, folder(t, "a"))
	b := mustAdd(t, s, folder(t, "b"))
	if _, err := s.Remove(b.ID); err != nil {
		t.Fatal(err)
	}
	c := mustAdd(t, s, folder(t, "c"))
	if c.Port != b.Port {
		t.Fatalf("the new vault took %d, want the port the removed one left, %d", c.Port, b.Port)
	}
	back, err := s.Restore(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.Port != c.Port+1 {
		t.Fatalf("the restored vault is on %d, want the next port after the one given away, %d", back.Port, c.Port+1)
	}
}

func TestAVaultAddedAgainIsNoLongerRememberedAsRemoved(t *testing.T) {
	t.Parallel()
	s, _ := storeIn(t, t.TempDir(), nil)
	dir := folder(t, "a")
	e := mustAdd(t, s, dir)
	if _, err := s.Remove(e.ID); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, s, dir)
	if _, err := s.Restore(e.ID); !errors.Is(err, ErrNotRemoved) {
		t.Fatalf("Restore of a vault the reader added again = %v, want ErrNotRemoved", err)
	}
}

func TestADamagedListIsSavedNotDeletedAndReadAsEmpty(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	s, _ := storeIn(t, config, nil)
	home := homeAt(t, config)
	if err := confdir.Replace(home, listFile, []byte("{ this is not a list")); err != nil {
		t.Fatal(err)
	}

	if list, err := s.List(); !errors.Is(err, ErrDamaged) || len(list) != 0 {
		t.Fatalf("List of a damaged file = %v, %v; want no vaults and ErrDamaged", list, err)
	}
	if _, err := os.Stat(home.Join(damagedFile)); err == nil {
		t.Fatal("reading a damaged list saved it; only a change may")
	}

	e := mustAdd(t, s, folder(t, "a"))
	if got := readFile(t, home.Join(damagedFile)); string(got) != "{ this is not a list" {
		t.Fatalf("the damaged list was not kept: %q", got)
	}
	list, err := s.List()
	if err != nil || len(list) != 1 || list[0].ID != e.ID {
		t.Fatalf("after a change the list is %v, %v; want a fresh one with the new vault", list, err)
	}
}

func TestAnEntryThatClaimsAnotherVaultsIDIsLeftOutAndTheRestAreKept(t *testing.T) {
	t.Parallel()
	s, _ := storeIn(t, t.TempDir(), nil)
	good := entryAt("/v/good", 8802)
	stolen := Entry{ID: confdir.VaultID("/some/other/vault"), Path: entryAt("/v/stolen", 8801).Path, Name: "x", Port: 8801}
	data, err := encode([]Entry{stolen, good})
	if err != nil {
		t.Fatal(err)
	}
	if err = confdir.Replace(s.dir, listFile, data); err != nil {
		t.Fatal(err)
	}
	list, err := s.List()
	if err != nil || len(list) != 1 || list[0].ID != good.ID || list[0].Port != 8802 {
		t.Fatalf("List = %+v, %v; want only the good entry, on its port: the other would hand a vault another's marks", list, err)
	}
}

func TestTheLaterOfTwoEntriesOnOnePortOrWithOneIDIsLeftOut(t *testing.T) {
	t.Parallel()
	for name, list := range map[string][]Entry{
		"one port twice": {entryAt("/v/a", 8800), entryAt("/v/b", 8800)},
		"one id twice":   {entryAt("/v/a", 8800), entryAt("/v/a", 8801)},
	} {
		data, err := encode(list)
		if err != nil {
			t.Fatal(err)
		}
		got, skipped, err := decode(data)
		if err != nil || len(got) != 1 || got[0] != list[0] || len(skipped) != 1 {
			t.Errorf("%s: decode = %+v, %v, %v; want the first entry kept and the second left out", name, got, skipped, err)
		}
	}
}

func entryAt(path string, port int) Entry {
	path = filepath.FromSlash(path)
	if !filepath.IsAbs(path) {
		path, _ = filepath.Abs(path) //nolint:errcheck // a test path
	}
	return Entry{ID: confdir.VaultID(path), Path: path, Name: filepath.Base(path), Port: port}
}

func TestChangedLooksAtTheFileWithoutReadingIt(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	s, _ := storeIn(t, config, nil)
	if changed, err := s.Changed(); err != nil || changed {
		t.Fatalf("Changed with no list = %v, %v; want false", changed, err)
	}
	mustAdd(t, s, folder(t, "a"))
	if changed, err := s.Changed(); err != nil || changed {
		t.Fatalf("Changed after this process's own change = %v, %v; want false", changed, err)
	}

	// Another process changes it; the poller learns from the file's length and
	// time alone, and finds out what is wrong with it only when it reads.
	if err := confdir.Replace(homeAt(t, config), listFile, []byte("not a list at all, and longer")); err != nil {
		t.Fatal(err)
	}
	if changed, err := s.Changed(); err != nil || !changed {
		t.Fatalf("Changed after another process wrote it = %v, %v; want true and no error for what is inside", changed, err)
	}
	if _, err := os.Stat(filepath.Join(config, "yomihon", damagedFile)); err == nil {
		t.Fatal("Changed saved the list as damaged, so it parsed it")
	}
	if _, err := s.List(); !errors.Is(err, ErrDamaged) {
		t.Fatalf("List = %v, want ErrDamaged", err)
	}
	if changed, err := s.Changed(); err != nil || changed {
		t.Fatalf("Changed after it was read = %v, %v; want false", changed, err)
	}
}

func TestAChangeLeavesOnlyTheListAndItsLockInTheDirectory(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	s, _ := storeIn(t, config, nil)
	mustAdd(t, s, folder(t, "a"))
	mustAdd(t, s, folder(t, "b"))

	entries, err := os.ReadDir(filepath.Join(config, "yomihon"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if want := []string{listFile, lockFile}; !slices.Equal(names, want) {
		t.Fatalf("the directory holds %v, want only %v: no temporary file or anything else left behind", names, want)
	}
	for _, name := range names {
		if got, want := fileMode(t, filepath.Join(config, "yomihon", name)), os.FileMode(0o600); runtimeHasModes() && got != want {
			t.Errorf("%s is mode %v, want %v: what the reader has opened is theirs alone", name, got, want)
		}
	}
}

// readFile is a file of this test's own temporary directory.
func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path) // #nosec G304 -- a file of this test's own temporary directory
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

// deadPID is the id of a process that has ended.
func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^$") // #nosec G204 G702 -- this test binary, run again so that it ends at once
	if err := cmd.Run(); err != nil {
		t.Fatalf("run a process that ends at once: %v", err)
	}
	return cmd.ProcessState.Pid()
}

func TestALockHeldByAnotherIsWaitedForAndThenGivenUpOn(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	s, _ := storeIn(t, config, nil)
	held, err := confdir.Acquire(t.Context(), homeAt(t, config), lockFile, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = held.Release() }) //nolint:errcheck // the temporary directory goes with it

	_, _, err = s.Add(folder(t, "a"), 8800)
	if !errors.Is(err, confdir.ErrLocked) {
		t.Fatalf("Add while another holds the lock = %v, want ErrLocked", err)
	}
	if list, listErr := s.List(); listErr != nil || len(list) != 0 {
		t.Fatalf("a refused Add changed the list: %v, %v", list, listErr)
	}
}

// treeOf is every file and directory under root with what each holds, so two
// calls can be compared byte for byte.
func treeOf(t *testing.T, root string) map[string]string {
	t.Helper()
	tree := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() {
			tree[rel+string(filepath.Separator)] = ""
			return nil
		}
		tree[rel] = string(readFile(t, path))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

// TestTheListWritesNothingOutsideItsOwnDirectoryUnderTheConfigurationDirectory
// is promise 1 for the list of vaults: nothing is written into a vault being
// read, or anywhere else beside the reader's own directory. It does everything
// the list does, in a tree that holds a vault, the configuration directory and
// a neighbour of both, and asks that the tree be the same afterwards except
// under yomihon in the configuration directory.
func TestTheListWritesNothingOutsideItsOwnDirectoryUnderTheConfigurationDirectory(t *testing.T) {
	t.Parallel()
	world := t.TempDir()
	config := filepath.Join(world, "config")
	vaultDir := filepath.Join(world, "notes")
	neighbour := filepath.Join(world, "elsewhere")
	for _, dir := range []string{config, vaultDir, neighbour, filepath.Join(vaultDir, "Writing")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{
		filepath.Join(vaultDir, "README.md"),
		filepath.Join(vaultDir, "Writing", "alpha.md"),
		filepath.Join(neighbour, "keep.txt"),
	} {
		if err := os.WriteFile(file, []byte("# "+filepath.Base(file)+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	before := treeOf(t, world)

	s, clock := storeIn(t, config, nil)
	e := mustAdd(t, s, vaultDir)
	other := mustAdd(t, s, folder(t, "other"))
	*clock = clock.Add(time.Hour)
	if _, err := s.Touch(e.ID, "Notes", *clock); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Remove(other.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Restore(other.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteInstance(Instance{PID: os.Getpid(), Port: startPort, Started: *clock}); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveInstance(os.Getpid()); err != nil {
		t.Fatal(err)
	}
	// A list that cannot be read is saved as damaged, which is a write of its own.
	if err := os.WriteFile(filepath.Join(config, "yomihon", listFile), []byte("damaged"), 0o600); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, s, vaultDir)

	after := treeOf(t, world)
	home := "config" + string(filepath.Separator) + "yomihon"
	for path, content := range after {
		if path == home+string(filepath.Separator) || strings.HasPrefix(path, home+string(filepath.Separator)) {
			continue
		}
		was, existed := before[path]
		if !existed {
			t.Errorf("%s was created, and it is not under the reader's own directory", path)
		} else if was != content {
			t.Errorf("%s was changed, and it is not under the reader's own directory", path)
		}
	}
	for path := range before {
		if _, kept := after[path]; !kept {
			t.Errorf("%s was removed", path)
		}
	}
	if _, wrote := after[home+string(filepath.Separator)]; !wrote {
		t.Fatal("nothing was written under the reader's own directory, so this saw none of what the list does")
	}
}

func TestSetPortMovesAVaultButNotOntoAnotherVaultsPort(t *testing.T) {
	t.Parallel()
	s, _ := storeIn(t, t.TempDir(), nil)
	a := mustAdd(t, s, folder(t, "a"))
	b := mustAdd(t, s, folder(t, "b"))

	if err := s.SetPort(b.ID, 9100); err != nil {
		t.Fatalf("SetPort: %v", err)
	}
	if err := s.SetPort(b.ID, a.Port); !errors.Is(err, ErrPortTaken) {
		t.Errorf("SetPort onto another vault's port = %v, want ErrPortTaken", err)
	}
	if err := s.SetPort(confdir.VaultID("/never/seen"), 9101); !errors.Is(err, ErrUnknown) {
		t.Errorf("SetPort of an unknown id = %v, want ErrUnknown", err)
	}
	for _, port := range []int{0, 80, MinPort - 1} {
		if err := s.SetPort(b.ID, port); !errors.Is(err, ErrStartPort) {
			t.Errorf("SetPort(%d) = %v, want ErrStartPort", port, err)
		}
	}
	list, _ := s.List() //nolint:errcheck // asserted on below
	for _, e := range list {
		if e.ID == b.ID && e.Port != 9100 {
			t.Errorf("the vault is on %d, want 9100", e.Port)
		}
	}
}

// A vault already listed is never moved: not by the starting port, not by
// another vault being added. The saved address of each is what the reader's
// browser has, and it has to keep opening that vault.
func TestNoVaultIsEverMovedByAnotherBeingAdded(t *testing.T) {
	t.Parallel()
	s, _ := storeIn(t, t.TempDir(), nil)
	first := mustAdd(t, s, folder(t, "a"))
	s.Hold(startPort + 7)
	typed, added, err := s.Add(folder(t, "b"), startPort+7)
	if err != nil || !added {
		t.Fatalf("Add = %+v, %v, %v", typed, added, err)
	}
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	ports := map[string]int{}
	for _, e := range list {
		ports[e.ID] = e.Port
	}
	if ports[first.ID] != first.Port {
		t.Errorf("the vault listed first moved from %d to %d", first.Port, ports[first.ID])
	}
	if ports[typed.ID] != startPort+7 {
		t.Errorf("the new vault is on %d, want the starting port %d the process holds", ports[typed.ID], startPort+7)
	}
}

// The process that holds the starting port gives it to the vault it is about to
// record, which a probe cannot say is free because the probe finds the process
// itself in the way. A vault already on the port keeps it.
func TestAPortThisProcessHoldsIsFreeForTheVaultItRecordsUnlessAVaultHasIt(t *testing.T) {
	t.Parallel()
	held := map[int]bool{startPort: true, startPort + 1: true}
	s, _ := storeIn(t, t.TempDir(), func(port int) bool { return !held[port] })

	if got := mustAdd(t, s, folder(t, "a")); got.Port != startPort+2 {
		t.Fatalf("without Hold the first vault is on %d, want %d: the probe finds the process's own port in use", got.Port, startPort+2)
	}

	s2, _ := storeIn(t, t.TempDir(), func(port int) bool { return !held[port] })
	s2.Hold(startPort)
	first := mustAdd(t, s2, folder(t, "a"))
	second := mustAdd(t, s2, folder(t, "b"))
	if first.Port != startPort {
		t.Errorf("the first vault is on %d, want the port the process holds, %d", first.Port, startPort)
	}
	if second.Port != startPort+2 {
		t.Errorf("the second vault is on %d, want %d: the held port is the first vault's now, and %d is held by something else", second.Port, startPort+2, startPort+1)
	}
}

func mkdirHome(s *Store) error { return os.MkdirAll(s.dir.Path(), 0o700) }

// homeAt is the reader's directory under configDir, the one kind of directory the
// package under test is given to work in.
func homeAt(t *testing.T, configDir string) confdir.Dir {
	t.Helper()
	home, err := confdir.Home(confdir.At(configDir))
	if err != nil {
		t.Fatal(err)
	}
	return home
}

func writeList(t *testing.T, configDir, body string) *Store {
	t.Helper()
	s, _ := storeIn(t, configDir, nil)
	if err := mkdirHome(s); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.dir.Join(listFile), []byte(body), 0o600); err != nil { // #nosec G703 -- a file under t.TempDir
		t.Fatal(err)
	}
	return s
}

// An entry the list would not have written is left out, and not given another
// port; the list is not damaged for it.
func TestAnEntryOutsideTheUnprivilegedPortsOrAtTheTopOfAFilesystemIsLeftOut(t *testing.T) {
	t.Parallel()
	folderPath := folder(t, "a")
	for _, test := range []struct {
		name, path string
		port       int
	}{
		{"a port of the system's", folderPath, 80},
		{"the last system port", folderPath, MinPort - 1},
		{"the top of the filesystem", string(filepath.Separator), 9000},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			body := `{"version":1,"vaults":[{"id":"` + confdir.VaultID(test.path) + `","path":` + strconv.Quote(test.path) + `,"name":"x","last_read":"2026-11-18T09:00:00Z","port":` + strconv.Itoa(test.port) + `}]}`
			s := writeList(t, t.TempDir(), body)
			if list, err := s.List(); err != nil || len(list) != 0 {
				t.Errorf("List = %+v, %v; want no vaults and no error", list, err)
			}
		})
	}
}

// A list longer than any list is, is saved like every other damaged list,
// and the next change starts a fresh one.
func TestAnOversizedListIsSavedNotLeftToBlockEveryChange(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	s := writeList(t, configDir, strings.Repeat("x", maxListBytes+1))
	entry, added, err := s.Add(folder(t, "a"), startPort)
	if err != nil || !added {
		t.Fatalf("Add on top of an oversized list = %+v, %v, %v; want it taken in", entry, added, err)
	}
	if _, err = os.Stat(s.dir.Join(damagedFile)); err != nil {
		t.Errorf("the oversized list was not saved as damaged: %v", err)
	}
	if list, listErr := s.List(); listErr != nil || len(list) != 1 {
		t.Errorf("after the change the list holds %+v, %v; want the one vault", list, listErr)
	}
}

// The ports are what the reader's tabs and history follow. One entry that cannot
// be used does not take the others off the ports they have.
func TestOneEntryThatCannotBeUsedDoesNotMoveTheOthers(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	var said []string
	s := writeListReporting(t, configDir, &said, "")
	a, b := entryAt("/v/a", 8801), entryAt("/v/b", 8802)
	bad := `{"id":"x","path":"/","name":"root","last_read":"2026-11-18T09:00:00Z","port":8800}`
	body := string(mustEncode(t, []Entry{a, b}))
	body = strings.Replace(body, `"vaults": [`, `"vaults": [`+bad+`,`, 1)
	if err := os.WriteFile(s.dir.Join(listFile), []byte(body), 0o600); err != nil { // #nosec G703 -- a file under t.TempDir
		t.Fatal(err)
	}

	list, err := s.List()
	if err != nil || len(list) != 2 {
		t.Fatalf("List = %+v, %v; want the two good vaults", list, err)
	}
	for _, e := range list {
		if want := map[string]int{a.ID: 8801, b.ID: 8802}[e.ID]; e.Port != want {
			t.Errorf("%s is on %d, want %d", e.Path, e.Port, want)
		}
	}
	if len(said) != 1 || !strings.Contains(said[0], "1 entry that could not be used") {
		t.Fatalf("said %q; want one line about the entry left out", said)
	}
	if _, listErr := s.List(); listErr != nil || len(said) != 1 {
		t.Fatalf("reading the same file again said %q; once for each state of the file", said)
	}

	// A change keeps the list as it was, and writes the rest on the ports they had.
	mustAdd(t, s, folder(t, "c"))
	kept := readFile(t, s.dir.Join(damagedFile))
	if string(kept) != body {
		t.Errorf("the list as it was is not kept whole: %q", kept)
	}
	after, err := s.List()
	if err != nil || len(after) != 3 {
		t.Fatalf("after the change List = %+v, %v; want three vaults", after, err)
	}
	for _, e := range after {
		if want, held := map[string]int{a.ID: 8801, b.ID: 8802}[e.ID]; held && e.Port != want {
			t.Errorf("%s moved to %d, want %d", e.Path, e.Port, want)
		}
	}
	if len(said) != 2 || !strings.Contains(said[1], s.dir.Join(damagedFile)) {
		t.Fatalf("said %q; the second line names the file the list is kept as", said)
	}
}

func TestAnEntryOfTheWrongShapeIsLeftOutAndAFileThatIsNotAListIsStillDamaged(t *testing.T) {
	t.Parallel()
	good := string(mustEncode(t, []Entry{entryAt("/v/a", 8801)}))
	for name, test := range map[string]struct {
		body    string
		damaged bool
	}{
		"a port that is a string":    {strings.Replace(good, `"vaults": [`, `"vaults": [{"id":"i","path":"/v/x","name":"x","last_read":"2026-11-18T09:00:00Z","port":"88"},`, 1), false},
		"an entry that is null":      {strings.Replace(good, `"vaults": [`, `"vaults": [null,`, 1), false},
		"a field it does not have":   {strings.Replace(good, `"vaults": [`, `"vaults": [{"id":"i","path":"/v/x","name":"x","last_read":"2026-11-18T09:00:00Z","port":8803,"extra":1},`, 1), false},
		"a version it does not read": {strings.Replace(good, `"version": 1`, `"version": 2`, 1), true},
		"vaults that is not a list":  {`{"version":1,"vaults":{}}`, true},
	} {
		s := writeList(t, t.TempDir(), test.body)
		list, err := s.List()
		if test.damaged != errors.Is(err, ErrDamaged) || (!test.damaged && (err != nil || len(list) != 1)) {
			t.Errorf("%s: List = %+v, %v; want damaged = %v and otherwise the one good vault", name, list, err, test.damaged)
		}
	}
}

// What a list was is not written over by the next list that cannot be used: the
// first stays, each later one has a name of its own, and past the last name only
// the last is replaced.
func TestASecondDamagedListDoesNotReplaceTheFirst(t *testing.T) {
	t.Parallel()
	var said []string
	s := writeListReporting(t, t.TempDir(), &said, "first, not a list")
	mustAdd(t, s, folder(t, "a"))
	for i := range damagedCopies + 2 {
		if err := os.WriteFile(s.dir.Join(listFile), []byte("not a list, number "+strconv.Itoa(i)), 0o600); err != nil { // #nosec G703 -- a file under t.TempDir
			t.Fatal(err)
		}
		mustAdd(t, s, folder(t, "b"+strconv.Itoa(i)))
	}
	if got := readFile(t, s.dir.Join(damagedName(1))); string(got) != "first, not a list" {
		t.Errorf("the first damaged list saved was replaced: %q", got)
	}
	if got := readFile(t, s.dir.Join(damagedName(2))); string(got) != "not a list, number 0" {
		t.Errorf("the second was not kept under a name of its own: %q", got)
	}
	if got := readFile(t, s.dir.Join(damagedName(damagedCopies))); string(got) != "not a list, number "+strconv.Itoa(damagedCopies+1) {
		t.Errorf("once every name is taken the last is replaced, and it holds %q", got)
	}
	entries, err := os.ReadDir(s.dir.Path())
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "vaults.damaged") {
			count++
		}
	}
	if count != damagedCopies {
		t.Errorf("%d lists are kept, want %d and no more", count, damagedCopies)
	}
	if want := damagedCopies + 3; len(said) != want { // the list it began with, and each round's
		t.Errorf("said %d lines for %d damaged lists saved: %q", len(said), want, said)
	}
}

// A change that changes nothing writes nothing, and so saves nothing as damaged: one
// copy of the list for every page load would fill the directory.
func TestAChangeThatChangesNothingSavesNothingAsDamaged(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]func(t *testing.T) (string, Entry){
		"a list with an entry that cannot be used": func(t *testing.T) (string, Entry) {
			t.Helper()
			good := entryAt("/v/good", 8802)
			good.LastRead = time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
			text := strings.Replace(string(mustEncode(t, []Entry{good})), `"vaults": [`, `"vaults": [{"id":"x","path":"/","name":"x","last_read":"2026-11-18T09:00:00Z","port":8801},`, 1)
			return text, good
		},
		"a list that is not a list": func(*testing.T) (string, Entry) { return "not a list", entryAt("/v/gone", 8801) },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			text, entry := body(t)
			s := writeList(t, t.TempDir(), text)
			for range 3 {
				// The vault was read a moment ago, or is not there: nothing is written.
				_, _ = s.Touch(entry.ID, entry.Name, time.Date(2026, 11, 18, 9, 0, 1, 0, time.UTC)) //nolint:errcheck // an unlisted vault is an error, and that is not what is looked at
			}
			entries, err := os.ReadDir(s.dir.Path())
			if err != nil {
				t.Fatal(err)
			}
			for _, found := range entries {
				if strings.HasPrefix(found.Name(), "vaults.damaged") {
					t.Errorf("%s was made for a change that wrote nothing", found.Name())
				}
			}
			if kept := readFile(t, s.dir.Join(listFile)); string(kept) != text {
				t.Errorf("the list was written: %q", kept)
			}
		})
	}
}

func mustEncode(t *testing.T, list []Entry) []byte {
	t.Helper()
	data, err := encode(list)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// writeListReporting is writeList for a Store that says what it did into said.
func writeListReporting(t *testing.T, configDir string, said *[]string, body string) *Store {
	t.Helper()
	s, err := Open(confdir.At(configDir), nil, Report(func(line string) { *said = append(*said, line) }))
	if err != nil {
		t.Fatal(err)
	}
	s.wait = 200 * time.Millisecond
	if err = mkdirHome(s); err != nil {
		t.Fatal(err)
	}
	if body != "" {
		if err = os.WriteFile(s.dir.Join(listFile), []byte(body), 0o600); err != nil { // #nosec G703 -- a file under t.TempDir
			t.Fatal(err)
		}
	}
	return s
}

// A folder that holds the configuration directory, is it, or is inside it never
// joins the list, and asking leaves the list as it was.
func TestAFolderThatOverlapsTheConfigurationDirectoryIsNotAdded(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	configDir := filepath.Join(base, "home", ".config")
	inside := filepath.Join(configDir, "notes")
	if err := os.MkdirAll(inside, 0o700); err != nil {
		t.Fatal(err)
	}
	s, _ := storeIn(t, configDir, nil)
	beside := folder(t, "notes")
	mustAdd(t, s, beside)
	listed := filepath.Join(configDir, "yomihon", listFile)
	before, err := os.ReadFile(listed) // #nosec G304 -- the test's own temporary directory
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{filepath.Join(base, "home"), configDir, inside} {
		entry, added, addErr := s.Add(root, startPort)
		if _, overlaps := errors.AsType[*confdir.OverlapError](addErr); !overlaps {
			t.Errorf("Add(%q) = %+v, %v, %v; want an *OverlapError", root, entry, added, addErr)
		}
	}
	after, err := os.ReadFile(listed) // #nosec G304 -- the test's own temporary directory
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("the list changed when no folder was added:\n before %s\n after  %s", before, after)
	}
}
