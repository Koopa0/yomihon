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
	err := configDir.Classify(folder)
	if err == nil {
		return 0
	}
	overlap, ok := errors.AsType[*confdir.OverlapError](err)
	if !ok {
		t.Fatalf("Classify(%q) = %v; want apart, or an *OverlapError", folder, err)
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

// Classify answers a folder apart from the configuration directory with no
// error, one that overlaps it with an *OverlapError that says how, and one it
// cannot compare with the error that stopped it, which is never taken for an
// overlap.
func TestClassifyAnswersApartAnOverlapOrWhatStoppedIt(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	mkdirs(t, base, "home/.config/yomihon/vault-dir", "home/.config/notes", "home/notes", "home/.config2", "elsewhere")
	configDir := confdir.At(filepath.Join(base, "home", ".config"))
	home := filepath.Join(base, "home")
	sep := string(filepath.Separator)
	dangling := filepath.Join(base, "dangling")
	linkErr := os.Symlink(filepath.Join(base, "gone"), dangling)
	file := filepath.Join(base, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name      string
		configDir confdir.ConfigDir
		folder    string
		how       confdir.Overlap
		unknown   bool
		unixOnly  bool
	}{
		{name: "no configuration directory", folder: home},
		{name: "a folder beside it", configDir: configDir, folder: filepath.Join(home, "notes")},
		{name: "a folder elsewhere", configDir: configDir, folder: filepath.Join(base, "elsewhere")},
		{name: "a folder whose name begins with its name", configDir: configDir, folder: filepath.Join(home, ".config2")},
		{name: "the configuration directory", configDir: configDir, folder: filepath.Join(home, ".config"), how: confdir.FolderIsIt},
		{name: "it with a trailing separator", configDir: configDir, folder: filepath.Join(home, ".config") + sep, how: confdir.FolderIsIt},
		{name: "it spelled through ..", configDir: configDir, folder: filepath.Join(home, "notes") + sep + ".." + sep + ".config", how: confdir.FolderIsIt},
		{name: "the folder that holds it", configDir: configDir, folder: home, how: confdir.FolderHoldsIt},
		{name: "a folder further up", configDir: configDir, folder: base, how: confdir.FolderHoldsIt},
		{name: "a folder inside it", configDir: configDir, folder: filepath.Join(home, ".config", "notes"), how: confdir.FolderIsInside},
		{name: "yomihon's own directory", configDir: configDir, folder: filepath.Join(home, ".config", "yomihon"), how: confdir.FolderIsInside},
		{name: "a folder inside yomihon's own directory", configDir: configDir, folder: filepath.Join(home, ".config", "yomihon", "vault-dir"), how: confdir.FolderIsInside},
		{name: "a folder that is not there", configDir: configDir, folder: filepath.Join(base, "absent"), unknown: true},
		{name: "a link to a folder that is not there", configDir: configDir, folder: dangling, unknown: true},
		{name: "a configuration directory under a file", configDir: confdir.At(filepath.Join(file, ".config")), folder: home, unknown: true, unixOnly: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			switch {
			case tt.folder == dangling && linkErr != nil:
				t.Skipf("no symbolic links here: %v", linkErr)
			case tt.unixOnly && runtime.GOOS == "windows":
				t.Skip("Windows reads a name below a file as one that is not there")
			}
			err := tt.configDir.Classify(tt.folder)
			overlap, isOverlap := errors.AsType[*confdir.OverlapError](err)
			switch {
			case tt.unknown && (err == nil || isOverlap):
				t.Errorf("Classify(%q) = %v, want the error that stopped the comparison", tt.folder, err)
			case !tt.unknown && tt.how == 0 && err != nil:
				t.Errorf("Classify(%q) = %v, want apart", tt.folder, err)
			case tt.how != 0 && (!isOverlap || overlap.How != tt.how):
				t.Errorf("Classify(%q) = %v, want an *OverlapError of how %d", tt.folder, err, tt.how)
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
	if refusal := configDir.Classify(link); refusal == nil || !strings.Contains(refusal.Error(), vault.Tilde(leads)+")") {
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
		t.Errorf("Classify(%q) overlaps as %d, want FolderHoldsIt: the configuration directory is %s", deep, got, filepath.Join(deep, "conf"))
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
		t.Errorf("Classify(%q) overlaps as %d, want FolderHoldsIt", upper, got)
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
		t.Errorf("Classify(%q) overlaps as %d, want FolderHoldsIt", folder, got)
	}
}

// What cannot be looked at cannot be told apart, and is not taken for apart.
func TestAFolderThatCannotBeToldApartIsNotApart(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	mkdirs(t, base, "locked/home/.config", "locked/notes")
	locked := filepath.Join(base, "locked")
	configDir := confdir.At(filepath.Join(locked, "home", ".config"))
	notes := filepath.Join(locked, "notes")
	unreadableDir(t, locked, 0o000)
	err := configDir.Classify(notes)
	if _, isOverlap := errors.AsType[*confdir.OverlapError](err); err == nil || isOverlap {
		t.Errorf("Classify under a directory that cannot be searched = %v, want the error that stopped it", err)
	}
	// The folder can be looked at and the configuration directory cannot.
	elsewhere := mkdirs(t, base, "elsewhere")
	err = configDir.Classify(elsewhere)
	if _, isOverlap := errors.AsType[*confdir.OverlapError](err); err == nil || isOverlap {
		t.Errorf("Classify of a configuration directory that cannot be looked at = %v, want the error that stopped it", err)
	}
}
