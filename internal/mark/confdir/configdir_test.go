package confdir_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/mark/confdir"
)

// platformNames points every variable os.UserConfigDir reads, on any platform,
// at home, and answers the directory the platform then names. Not parallel:
// it sets the environment, which t.Setenv does not allow in a parallel test.
func platformNames(t *testing.T, home string) string {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("AppData", home)
	switch runtime.GOOS {
	case "windows":
		return home
	case "darwin", "ios":
		return home + "/Library/Application Support"
	case "plan9":
		return home + "/lib"
	default:
		return home + "/.config"
	}
}

// platformHome is the reader's directory inside the configuration directory
// Platform answers, or the test stops.
func platformHome(t *testing.T) confdir.Dir {
	t.Helper()
	configDir, err := confdir.Platform()
	if err != nil {
		t.Fatalf("Platform() = %v, want the directory the environment names", err)
	}
	home, err := confdir.Home(configDir)
	if err != nil {
		t.Fatalf("Home(Platform()) = %v", err)
	}
	return home
}

func TestPlatformIsTheDirectoryThePlatformNames(t *testing.T) {
	named := platformNames(t, t.TempDir())
	if got, want := platformHome(t).Path(), filepath.Join(named, "yomihon"); got != want {
		t.Errorf("Home(Platform()) is at %s, want %s", got, want)
	}
}

// A configuration directory that is not an absolute path is read against the
// folder the process stands in, and yomihon with no folder named serves the
// one it stands in, so its files would be kept inside the folder being read.
// The standard library refuses a relative XDG_CONFIG_HOME on Linux and takes
// HOME and %AppData% as they are; Platform refuses all of them.
func TestPlatformRefusesADirectoryThatIsNotAbsolute(t *testing.T) {
	standing := t.TempDir()
	t.Chdir(standing)
	relatives := []string{".", "relative"}
	if runtime.GOOS == "windows" {
		// Rooted without a drive, and on a drive without its root: neither is
		// absolute on Windows, and each is read against where the process is.
		relatives = append(relatives, `\rooted`, `C:relative`)
	}
	for _, relative := range relatives {
		platformNames(t, relative)
		configDir, err := confdir.Platform()
		if !errors.Is(err, confdir.ErrNotAbsolute) {
			t.Errorf("Platform() with HOME and %%AppData%% %q = %v, want ErrNotAbsolute", relative, err)
		}
		if !configDir.IsZero() {
			t.Errorf("Platform() with HOME and %%AppData%% %q named %v, want no configuration directory", relative, configDir)
		}
	}
	// The standard library's own refusal, which Platform passes on.
	if runtime.GOOS == "linux" {
		t.Setenv("HOME", standing)
		t.Setenv("XDG_CONFIG_HOME", "relative")
		if configDir, err := confdir.Platform(); err == nil || !configDir.IsZero() {
			t.Errorf("Platform() with XDG_CONFIG_HOME relative = %v, %v; want a refusal", configDir, err)
		}
	}
	if entries, err := os.ReadDir(standing); err != nil || len(entries) != 0 {
		t.Errorf("the folder the process stands in holds %v, %v; want nothing", entries, err)
	}
}

func TestPlatformWithNoDirectoryNamedSaysWhy(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("AppData", "")
	t.Setenv("home", "")
	configDir, err := confdir.Platform()
	if err == nil || !configDir.IsZero() {
		t.Fatalf("Platform() with nothing naming a directory = %v, %v; want a refusal", configDir, err)
	}
	if errors.Is(err, confdir.ErrNotAbsolute) {
		t.Errorf("Platform() with nothing naming a directory = %v, want the reason the platform gives rather than ErrNotAbsolute", err)
	}
}

// Platform reads the environment and not the disk, so a directory that is not
// there yet is named, nothing is made by naming it, and the first write makes
// it, as it does on a machine where yomihon has kept nothing.
func TestPlatformNamesADirectoryThatIsNotThereAndMakesNothing(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "absent")
	named := platformNames(t, absent)
	home := platformHome(t)
	if got, want := home.Path(), filepath.Join(named, "yomihon"); got != want {
		t.Errorf("Home(Platform()) is at %s, want %s", got, want)
	}
	if _, err := os.Stat(absent); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("naming the configuration directory left %s as %v, want it still absent", absent, err)
	}
	if err := confdir.Replace(home, "vaults.json", []byte("{}")); err != nil {
		t.Fatalf("the first write into a directory that is not there = %v, want it made", err)
	}
	if data, err := os.ReadFile(filepath.Join(named, "yomihon", "vaults.json")); err != nil || string(data) != "{}" { // #nosec G304 -- a file under the test's own temporary directory
		t.Errorf("the first write left %q, %v where the platform named", data, err)
	}
}

// A directory that cannot be read is named all the same: reading is the
// product, and whether the reader's files can be kept is for the write to
// answer, which it does with the permission error and nothing written.
func TestPlatformNamesADirectoryThatCannotBeReadAndTheWriteSaysSo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not refuse a directory by its permission bits")
	}
	named := platformNames(t, t.TempDir())
	if err := os.MkdirAll(named, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(named, 0o000); err != nil { // #nosec G302 -- the test's own temporary directory
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(named, 0o700); err != nil { // #nosec G302 -- restores the mode the test removed so the directory can be cleaned up
			t.Errorf("restore the directory's mode: %v", err)
		}
	})
	if opened, err := os.Open(named); err == nil { // #nosec G304 -- the test's own temporary directory
		_ = opened.Close() //nolint:errcheck // only whether it opened matters
		t.Skip("this user opens a directory it may not read (a privileged user), so the fixture shows nothing")
	}
	home := platformHome(t)
	if got, want := home.Path(), filepath.Join(named, "yomihon"); got != want {
		t.Errorf("Home(Platform()) is at %s, want %s", got, want)
	}
	if err := confdir.Replace(home, "vaults.json", []byte("{}")); !errors.Is(err, fs.ErrPermission) {
		t.Errorf("a write into a directory that cannot be read = %v, want a permission error", err)
	}
	if _, err := confdir.Read(home, "vaults.json", 1<<10); !errors.Is(err, fs.ErrPermission) {
		t.Errorf("a read in a directory that cannot be read = %v, want a permission error", err)
	}
}

// A configuration directory reached through a symbolic link is named as the
// platform spells it, which is the spelling every message has given, and a
// write lands where the link leads.
func TestPlatformNamesADirectoryReachedThroughALinkAsItIsSpelled(t *testing.T) {
	// A home that is not there yet, so that on Windows, where the configuration
	// directory is %AppData% itself, the link can be made in its place.
	named := platformNames(t, filepath.Join(t.TempDir(), "home"))
	if err := os.MkdirAll(filepath.Dir(named), 0o700); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	if err := os.Symlink(target, named); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
	home := platformHome(t)
	if got, want := home.Path(), filepath.Join(named, "yomihon"); got != want {
		t.Errorf("Home(Platform()) is at %s, want %s, as the link spells it", got, want)
	}
	if err := confdir.Replace(home, "vaults.json", []byte("{}")); err != nil {
		t.Fatalf("a write through the link = %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(target, "yomihon", "vaults.json")); err != nil || string(data) != "{}" { // #nosec G304 -- a file under the test's own temporary directory
		t.Errorf("the write did not land where the link leads: %q, %v", data, err)
	}
}

func TestAtIsTheDirectoryATestNames(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	home, err := confdir.Home(confdir.At(dir))
	if err != nil {
		t.Fatalf("Home(At(%q)) = %v", dir, err)
	}
	if want := filepath.Join(dir, "yomihon"); home.Path() != want {
		t.Errorf("Home(At(%q)) is at %s, want %s", dir, home.Path(), want)
	}
	if confdir.At(dir).IsZero() {
		t.Errorf("At(%q) names no configuration directory", dir)
	}
	if !(confdir.ConfigDir{}).IsZero() {
		t.Error("the zero ConfigDir names a configuration directory")
	}
}

// At is for tests, and a test that hands it a relative path has a mistake in
// it: it panics rather than hand back a directory relative to wherever the test
// runs.
func TestAtRefusesAPathThatIsNotAbsolute(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"", ".", "relative", filepath.Join("a", "b")} {
		func() {
			defer func() {
				if said := recover(); said == nil {
					t.Errorf("At(%q) returned, want a panic", path)
				} else if text, ok := said.(string); !ok || !strings.Contains(text, confdir.ErrNotAbsolute.Error()) {
					t.Errorf("At(%q) panicked with %v, want it to say the path is not absolute", path, said)
				}
			}()
			_ = confdir.At(path)
		}()
	}
}
