package confdir_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/vault"
)

// overlapOf is how Classify says folder and configDir overlap, 0 for apart, or
// the test stops on any other answer.
func overlapOf(t *testing.T, configDir confdir.ConfigDir, folder string) confdir.Overlap {
	t.Helper()
	relation, err := configDir.Classify(folder)
	if relation == confdir.Apart && err == nil {
		return 0
	}
	overlap, ok := errors.AsType[*confdir.OverlapError](err)
	if relation != confdir.Overlaps || !ok {
		t.Fatalf("Classify(%q) = %v, %v; want apart, or overlaps with an *OverlapError", folder, relation, err)
	}
	return overlap.How
}

// mkdirs makes each directory under base and answers the path of the first.
func mkdirs(t *testing.T, base string, rels ...string) string {
	t.Helper()
	for _, rel := range rels {
		if err := os.MkdirAll(filepath.Join(base, filepath.FromSlash(rel)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(base, filepath.FromSlash(rels[0]))
}

func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
}

func TestAFolderAndTheConfigurationDirectoryOneInsideTheOtherOverlap(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	mkdirs(t, base, "home/.config/yomihon/vault-dir", "home/.config/notes", "home/notes", "home/.config2", "elsewhere")
	configDir := confdir.At(filepath.Join(base, "home", ".config"))
	sep := string(filepath.Separator)
	for _, tt := range []struct {
		name   string
		folder string
		want   confdir.Overlap
	}{
		{"a folder beside the configuration directory", filepath.Join(base, "home", "notes"), 0},
		{"a folder elsewhere", filepath.Join(base, "elsewhere"), 0},
		{"a folder whose name begins with the configuration directory's", filepath.Join(base, "home", ".config2"), 0},
		{"the configuration directory itself", filepath.Join(base, "home", ".config"), confdir.FolderIsIt},
		{"the configuration directory with a trailing separator", filepath.Join(base, "home", ".config") + sep, confdir.FolderIsIt},
		{"the configuration directory spelled through ..", filepath.Join(base, "home", "notes") + sep + ".." + sep + ".config", confdir.FolderIsIt},
		{"the folder that holds it", filepath.Join(base, "home"), confdir.FolderHoldsIt},
		{"a folder further up", base, confdir.FolderHoldsIt},
		{"a folder inside it", filepath.Join(base, "home", ".config", "notes"), confdir.FolderIsInside},
		{"a folder inside yomihon's own directory", filepath.Join(base, "home", ".config", "yomihon", "vault-dir"), confdir.FolderIsInside},
		{"yomihon's own directory", filepath.Join(base, "home", ".config", "yomihon"), confdir.FolderIsInside},
	} {
		if got := overlapOf(t, configDir, tt.folder); got != tt.want {
			t.Errorf("%s: Classify(%q) overlaps as %d, want %d", tt.name, tt.folder, got, tt.want)
		}
	}
}

// Classify is the one answer every caller acts on, so each of its three answers
// comes with the error that goes with it, and no other: apart with none, overlaps
// with an *OverlapError that says how, and unknown with what stopped the
// comparison, which is never taken for an overlap.
func TestClassifyAnswersApartOverlapsOrUnknownWithItsError(t *testing.T) {
	t.Parallel()
	type fixture struct {
		configDir confdir.ConfigDir
		folder    string
	}
	// under makes home/.config, home/notes and home/.config/notes under a
	// directory of the case's own, and names the folder rel under home.
	under := func(rel string) func(*testing.T) fixture {
		return func(t *testing.T) fixture {
			t.Helper()
			home := mkdirs(t, t.TempDir(), "home", "home/.config/notes", "home/notes", "home/.config2")
			return fixture{confdir.At(filepath.Join(home, ".config")), filepath.Join(home, filepath.FromSlash(rel))}
		}
	}
	noConfigDir := func(t *testing.T) fixture {
		t.Helper()
		return fixture{confdir.ConfigDir{}, t.TempDir()}
	}
	danglingLink := func(t *testing.T) fixture {
		t.Helper()
		f := under("dangling")(t)
		symlink(t, filepath.Join(filepath.Dir(f.folder), "gone"), f.folder)
		return f
	}
	configDirUnderAFile := func(t *testing.T) fixture {
		t.Helper()
		base := t.TempDir()
		notes := mkdirs(t, base, "notes")
		file := filepath.Join(base, "home")
		if err := os.WriteFile(file, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		return fixture{confdir.At(filepath.Join(file, ".config")), notes}
	}
	for _, tt := range []struct {
		name     string
		unixOnly bool
		fixture  func(t *testing.T) fixture
		want     confdir.Relation
		how      confdir.Overlap
	}{
		{name: "no configuration directory", fixture: noConfigDir, want: confdir.Apart},
		{name: "a folder beside it", fixture: under("notes"), want: confdir.Apart},
		{name: "a folder whose name begins with its name", fixture: under(".config2"), want: confdir.Apart},
		{name: "the configuration directory", fixture: under(".config"), want: confdir.Overlaps, how: confdir.FolderIsIt},
		{name: "the folder that holds it", fixture: under("."), want: confdir.Overlaps, how: confdir.FolderHoldsIt},
		{name: "a folder inside it", fixture: under(".config/notes"), want: confdir.Overlaps, how: confdir.FolderIsInside},
		{name: "a folder that is not there", fixture: under("absent"), want: confdir.Unknown},
		{name: "a link to a folder that is not there", fixture: danglingLink, want: confdir.Unknown},
		{name: "a configuration directory under a file", unixOnly: true, fixture: configDirUnderAFile, want: confdir.Unknown},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.unixOnly && runtime.GOOS == "windows" {
				t.Skip("Windows reads a name below a file as one that is not there")
			}
			f := tt.fixture(t)
			relation, err := f.configDir.Classify(f.folder)
			if relation != tt.want {
				t.Fatalf("Classify(%q) = %v, %v; want %v", f.folder, relation, err, tt.want)
			}
			overlap, isOverlap := errors.AsType[*confdir.OverlapError](err)
			switch tt.want {
			case confdir.Apart:
				if err != nil {
					t.Errorf("Classify(%q) = apart with %v, want no error", f.folder, err)
				}
			case confdir.Overlaps:
				if !isOverlap || overlap.How != tt.how {
					t.Errorf("Classify(%q) = overlaps with %v, want an *OverlapError of how %d", f.folder, err, tt.how)
				}
			case confdir.Unknown:
				if err == nil || isOverlap {
					t.Errorf("Classify(%q) = unknown with %v, want the error that stopped the comparison", f.folder, err)
				}
			}
		})
	}
}

// A configuration directory that is not there yet still has the directories
// above it, and a folder that is one of them holds it the moment the first write
// makes it.
func TestAConfigurationDirectoryNotThereYetIsHeldByTheFoldersAboveIt(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	mkdirs(t, base, "home/notes")
	configDir := confdir.At(filepath.Join(base, "home", ".config", "deeper"))
	if got := overlapOf(t, configDir, filepath.Join(base, "home")); got != confdir.FolderHoldsIt {
		t.Errorf("a folder above a configuration directory not made yet overlaps as %d, want FolderHoldsIt", got)
	}
	if got := overlapOf(t, configDir, filepath.Join(base, "home", "notes")); got != 0 {
		t.Errorf("a folder beside a configuration directory not made yet overlaps as %d, want apart", got)
	}
	if _, err := os.Stat(filepath.Join(base, "home", ".config")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("telling them apart made %s: %v", filepath.Join(base, "home", ".config"), err)
	}
}

// A vault that is a link to a folder above the configuration directory is that
// folder.
func TestAVaultLinkedToAFolderAboveTheConfigurationDirectoryHoldsIt(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	mkdirs(t, base, "home/.config", "links")
	link := filepath.Join(base, "links", "vault")
	symlink(t, filepath.Join(base, "home"), link)
	configDir := confdir.At(filepath.Join(base, "home", ".config"))
	if got := overlapOf(t, configDir, link); got != confdir.FolderHoldsIt {
		t.Errorf("Classify(%q), a link to the folder above it, overlaps as %d, want FolderHoldsIt", link, got)
	}
	leads, err := filepath.EvalSymlinks(filepath.Join(base, "home"))
	if err != nil {
		t.Fatal(err)
	}
	if _, refusal := configDir.Classify(link); refusal == nil || !strings.Contains(refusal.Error(), vault.Tilde(leads)+")") {
		t.Errorf("the refusal says %v, which does not name where the link leads", refusal)
	}
}

// HOME reached through a link: the configuration directory is spelled under the
// link, and the folder the reader types is the real one.
func TestAConfigurationDirectoryReachedThroughALinkedHomeIsWhereTheLinkLeads(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	home := mkdirs(t, base, "real/home", "real/home/.config/notes")
	link := filepath.Join(base, "home")
	symlink(t, home, link)
	configDir := confdir.At(filepath.Join(link, ".config"))
	if got := overlapOf(t, configDir, home); got != confdir.FolderHoldsIt {
		t.Errorf("the real home overlaps as %d with a configuration directory under a link to it, want FolderHoldsIt", got)
	}
	if got := overlapOf(t, configDir, filepath.Join(home, ".config", "notes")); got != confdir.FolderIsInside {
		t.Errorf("a folder inside the real configuration directory overlaps as %d, want FolderIsInside", got)
	}
	if got := overlapOf(t, configDir, filepath.Join(base, "real")); got != confdir.FolderHoldsIt {
		t.Errorf("the folder above the real home overlaps as %d, want FolderHoldsIt", got)
	}
}

// "link/.." is the folder above where the link leads, not the folder beside the
// link, so a configuration directory spelled that way is not where cleaning the
// spelling would put it.
func TestAConfigurationDirectorySpelledThroughALinkAndDotDotIsWhereTheSystemPutsIt(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	mkdirs(t, base, "deep/inner", "deep/conf", "beside/conf")
	deep := filepath.Join(base, "deep")
	link := filepath.Join(base, "beside", "link")
	symlink(t, filepath.Join(base, "deep", "inner"), link)
	sep := string(filepath.Separator)
	configDir := confdir.At(link + sep + ".." + sep + "conf")
	if got := overlapOf(t, configDir, deep); got != confdir.FolderHoldsIt {
		t.Errorf("Apart(%q) overlaps as %d, want FolderHoldsIt: the configuration directory is %s", deep, got, filepath.Join(deep, "conf"))
	}
	if got := overlapOf(t, configDir, filepath.Join(base, "beside")); got != 0 {
		t.Errorf("the folder beside the link overlaps as %d, want apart: cleaning the spelling is not where the system puts it", got)
	}
}

// On a filesystem that ignores case, another case of a folder's name is that
// folder.
func TestAFolderSpelledInAnotherCaseIsTheSameFolderWhereCaseIsIgnored(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	mkdirs(t, base, "Home/.config")
	upper := filepath.Join(base, "HOME")
	if _, err := os.Stat(upper); err != nil {
		t.Skipf("this filesystem tells %s from %s, so there is no second spelling to try", upper, filepath.Join(base, "Home"))
	}
	configDir := confdir.At(filepath.Join(base, "Home", ".config"))
	if got := overlapOf(t, configDir, upper); got != confdir.FolderHoldsIt {
		t.Errorf("Apart(%q) overlaps as %d, want FolderHoldsIt", upper, got)
	}
	if got := overlapOf(t, configDir, filepath.Join(base, "HOME", ".CONFIG")); got != confdir.FolderIsIt {
		t.Errorf("the configuration directory in another case overlaps as %d, want FolderIsIt", got)
	}
}

// A drive letter is a name in either case on Windows.
func TestADriveLetterInAnotherCaseIsTheSameDrive(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "windows" {
		t.Skip("drive letters are Windows's")
	}
	base := t.TempDir()
	mkdirs(t, base, "home/.config")
	volume := filepath.VolumeName(base)
	if len(volume) != 2 || volume[1] != ':' {
		t.Skipf("the temporary directory %s is not on a lettered drive", base)
	}
	other := strings.ToLower(volume)
	if other == volume {
		other = strings.ToUpper(volume)
	}
	configDir := confdir.At(filepath.Join(base, "home", ".config"))
	folder := other + filepath.Join(base, "home")[len(volume):]
	if got := overlapOf(t, configDir, folder); got != confdir.FolderHoldsIt {
		t.Errorf("Apart(%q) overlaps as %d, want FolderHoldsIt", folder, got)
	}
}

func TestNoConfigurationDirectoryIsApartFromEveryFolder(t *testing.T) {
	t.Parallel()
	if relation, err := (confdir.ConfigDir{}).Classify(t.TempDir()); relation != confdir.Apart || err != nil {
		t.Errorf("the zero ConfigDir.Classify = %v, %v; want apart, nil", relation, err)
	}
}

// What cannot be looked at cannot be told apart, and is not taken for apart.
func TestAFolderThatCannotBeToldApartIsNotApart(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	configDir := confdir.At(filepath.Join(base, "home", ".config"))
	if relation, err := configDir.Classify(filepath.Join(base, "absent")); relation != confdir.Unknown || err == nil {
		t.Errorf("Classify of a folder that is not there = %v, %v; want unknown with an error", relation, err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	mkdirs(t, base, "locked/home/.config", "locked/notes")
	locked := filepath.Join(base, "locked")
	configDir = confdir.At(filepath.Join(locked, "home", ".config"))
	notes := filepath.Join(locked, "notes")
	if err := os.Chmod(locked, 0o000); err != nil { // #nosec G302 -- the test's own temporary directory
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(locked, 0o700); err != nil { // #nosec G302 -- restores the mode the test removed so the directory can be cleaned up
			t.Errorf("restore the directory's mode: %v", err)
		}
	})
	if _, err := os.Stat(notes); err == nil {
		t.Skip("this user looks inside a directory it may not search (a privileged user), so the fixture shows nothing")
	}
	relation, err := configDir.Classify(notes)
	if relation != confdir.Unknown || err == nil {
		t.Fatalf("Classify under a directory that cannot be searched = %v, %v; want unknown with an error", relation, err)
	}
	if _, isOverlap := errors.AsType[*confdir.OverlapError](err); isOverlap {
		t.Errorf("Classify under a directory that cannot be searched = %v, want the error that stopped it", err)
	}
	// The folder can be looked at and the configuration directory cannot.
	elsewhere := mkdirs(t, base, "elsewhere")
	relation, err = configDir.Classify(elsewhere)
	if relation != confdir.Unknown || err == nil {
		t.Fatalf("Classify of a configuration directory under a directory that cannot be searched = %v, %v; want unknown with an error", relation, err)
	}
	if _, isOverlap := errors.AsType[*confdir.OverlapError](err); isOverlap {
		t.Errorf("Classify of a configuration directory that cannot be looked at = %v, want the error that stopped it", err)
	}
}

func TestTheRefusalNamesBothAndSaysWhatToDo(t *testing.T) {
	t.Parallel()
	r := t.TempDir()
	conf, inside := filepath.Join(r, "conf"), filepath.Join(r, "conf", "n")
	for _, tt := range []struct {
		how    confdir.Overlap
		folder string
		want   []string
	}{
		{confdir.FolderIsIt, conf, []string{vault.Tilde(conf) + " is the configuration directory where yomihon keeps your marks", "open the folder your notes are in"}},
		{confdir.FolderHoldsIt, r, []string{vault.Tilde(r) + " holds " + vault.Tilde(conf) + ", the configuration directory", "not one that holds it"}},
		{confdir.FolderIsInside, inside, []string{vault.Tilde(inside) + " is inside " + vault.Tilde(conf) + ", the configuration directory", "move the folder out of the configuration directory, or open another"}},
	} {
		said := (&confdir.OverlapError{Folder: tt.folder, Resolved: tt.folder, ConfigDir: conf, How: tt.how}).Error()
		for _, want := range tt.want {
			if !strings.Contains(said, want) {
				t.Errorf("the refusal for %d says %q, want it to say %q", tt.how, said, want)
			}
		}
		if strings.Contains(said, "(") {
			t.Errorf("the refusal for %d names a resolved folder that is the one given: %q", tt.how, said)
		}
	}
}
