package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/mark/vaults"
)

// refusalSays is the part of every refusal of a folder that overlaps the
// configuration directory that does not depend on how it overlaps.
const refusalSays = "the configuration directory where yomihon keeps your marks"

// platformConfigUnder points every variable the platform reads for its
// configuration directory at home, and answers that directory. Not parallel:
// it sets the environment.
func platformConfigUnder(t *testing.T, home string) string {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(home, "AppData")
	case "darwin":
		return filepath.Join(home, "Library", "Application Support")
	default:
		return filepath.Join(home, ".config")
	}
}

// startOn starts the command on root as `yomihon <root>` does, with the
// environment as it is, and answers how it ended. A start that is still
// serving after a moment is stopped and reported, with what it printed.
func startOn(t *testing.T, root, port string) error {
	t.Helper()
	t.Setenv("YOMIHON_PORT", port)
	cfg, err := loadConfig(root)
	if err != nil {
		t.Fatalf("loadConfig(%q) = %v", root, err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	var out, logged bytes.Buffer
	err = serveVault(ctx, cancel, slog.New(slog.NewTextHandler(&logged, nil)), &cfg, &out)
	if ctx.Err() != nil {
		t.Fatalf("yomihon served %s until it was stopped (%v); it printed %q and logged:\n%s", root, err, out.String(), logged.String())
	}
	return err
}

// TestAFolderThatOverlapsTheConfigurationDirectoryIsRefusedAtTheStart holds
// promise 1 where the folder and the configuration directory are one inside the
// other: served, every mark, bookmark or finished note would be a write inside
// the folder being read. Each spelling below is one a reader can type; each is
// refused before anything is listed, bound or written, as a fault the reader
// can correct, both alone and with a list of vaults.
//
// Not parallel: it sets the environment.
func TestAFolderThatOverlapsTheConfigurationDirectoryIsRefusedAtTheStart(t *testing.T) {
	for _, tt := range []struct {
		name string
		// arrange makes the folders and the environment, and answers the folder
		// to type and the configuration directory.
		arrange func(t *testing.T) (root, configDir string)
		want    string
	}{
		{"the home directory, which holds it", func(t *testing.T) (string, string) {
			t.Helper()
			home := filepath.Join(t.TempDir(), "home")
			writeDeskFixture(t, home)
			return home, platformConfigUnder(t, home)
		}, " holds "},
		{"a folder inside it", func(t *testing.T) (string, string) {
			t.Helper()
			home := t.TempDir()
			configDir := platformConfigUnder(t, home)
			root := filepath.Join(configDir, "notes")
			writeDeskFixture(t, root)
			return root, configDir
		}, " is inside "},
		{"the configuration directory itself", func(t *testing.T) (string, string) {
			t.Helper()
			configDir := platformConfigUnder(t, t.TempDir())
			writeDeskFixture(t, configDir)
			return configDir, configDir
		}, " is the configuration directory"},
		{"a link to the home directory", func(t *testing.T) (string, string) {
			t.Helper()
			base := t.TempDir()
			home := filepath.Join(base, "home")
			writeDeskFixture(t, home)
			link := filepath.Join(base, "notes")
			if err := os.Symlink(home, link); err != nil {
				t.Skipf("no symbolic links here: %v", err)
			}
			return link, platformConfigUnder(t, home)
		}, " holds "},
		{"the home directory in another case", func(t *testing.T) (string, string) {
			t.Helper()
			base := t.TempDir()
			home := filepath.Join(base, "Home")
			writeDeskFixture(t, home)
			upper := filepath.Join(base, "HOME")
			if _, err := os.Stat(upper); err != nil {
				t.Skipf("this filesystem tells %s from %s", upper, home)
			}
			return upper, platformConfigUnder(t, home)
		}, " holds "},
		{"the real home, with HOME a link to it", func(t *testing.T) (string, string) {
			t.Helper()
			base := t.TempDir()
			home := filepath.Join(base, "real", "home")
			writeDeskFixture(t, home)
			link := filepath.Join(base, "home")
			if err := os.Symlink(home, link); err != nil {
				t.Skipf("no symbolic links here: %v", err)
			}
			return home, platformConfigUnder(t, link)
		}, " holds "},
	} {
		for _, port := range []string{"0", "listed"} {
			t.Run(tt.name+", port "+port, func(t *testing.T) {
				root, configDir := tt.arrange(t)
				if port == "listed" {
					port = strconv.Itoa(someFreePort(t))
				}
				before := vaultState(t, root)
				_, configBefore := os.Stat(filepath.Join(configDir, "yomihon"))

				err := startOn(t, root, port)
				if operatorFault(err) == nil {
					t.Fatalf("starting on %s = %v, want a refusal the reader can correct", root, err)
				}
				if said := err.Error(); !strings.Contains(said, refusalSays) || !strings.Contains(said, tt.want) {
					t.Errorf("the refusal says %q, want it to say %q and %q", said, refusalSays, tt.want)
				}
				after := vaultState(t, root)
				for rel, was := range before {
					if now, ok := after[rel]; !ok || now != was {
						t.Errorf("%s changed in the folder: before %q, after %q", rel, was, now)
					}
				}
				for rel := range after {
					if _, ok := before[rel]; !ok {
						t.Errorf("%s was created in the folder", rel)
					}
				}
				if _, configAfter := os.Stat(filepath.Join(configDir, "yomihon")); (configBefore == nil) != (configAfter == nil) {
					t.Errorf("yomihon's directory in the configuration directory was %v and is %v", configBefore, configAfter)
				}
			})
		}
	}
}

// The control for the test above: a folder beside the configuration directory
// is served, so a start that is served is told from one that is refused.
//
// Not parallel: it sets the environment.
func TestAFolderBesideTheConfigurationDirectoryIsServed(t *testing.T) {
	base := t.TempDir()
	platformConfigUnder(t, filepath.Join(base, "home"))
	root := filepath.Join(base, "notes")
	writeDeskFixture(t, root)
	t.Setenv("YOMIHON_PORT", "0")
	cfg, err := loadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()
	var out bytes.Buffer
	if err := serveVault(ctx, cancel, slog.New(slog.DiscardHandler), &cfg, &out); err != nil {
		t.Fatalf("serving a folder beside the configuration directory = %v", err)
	}
	if !strings.Contains(out.String(), "http://127.0.0.1:") {
		t.Errorf("serving a folder beside the configuration directory printed %q, want its address", out.String())
	}
}

// Giving a folder to the yomihon already running is opening it too, and it is
// refused the same way, with nothing added to the list.
func TestAFolderThatOverlapsTheConfigurationDirectoryIsNotGivenToTheRunningYomihon(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	start := someFreePort(t)
	r := startRunning(t, configDir, newVaultFolder(t), start)
	r.address(t)
	inside := filepath.Join(configDir, "notes")
	writeRecoverySiteFixture(t, inside)

	cfg := &config{root: inside, port: strconv.Itoa(start), portSet: true, configDir: confdir.At(configDir)}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	err := serveVault(ctx, cancel, slog.New(slog.DiscardHandler), cfg, io.Discard)
	if operatorFault(err) == nil || !strings.Contains(err.Error(), refusalSays) {
		t.Fatalf("giving the running yomihon a folder inside the configuration directory = %v, want the refusal", err)
	}
	list, err := listOf(t, configDir).List()
	if err != nil {
		t.Fatal(err)
	}
	for i := range list {
		if list[i].Path == inside {
			t.Errorf("%s was added to the list", inside)
		}
	}
}

// A vault a yomihon listed before this refusal existed is not served, says why
// once, does not take the starting port with it, and stays in the list so that
// `yomihon vaults remove` takes it out.
func TestAListedVaultThatOverlapsTheConfigurationDirectoryIsNotServedAndStaysListed(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	inside := filepath.Join(configDir, "notes")
	writeRecoverySiteFixture(t, inside)
	start := someFreePort(t)
	// Listed the way a yomihon before this refusal listed it: by a list that
	// does not know this configuration directory, copied over this one's.
	elsewhere := t.TempDir()
	old, _, err := listOf(t, elsewhere).Add(inside, start)
	if err != nil {
		t.Fatalf("list %s elsewhere: %v", inside, err)
	}
	data, err := os.ReadFile(filepath.Join(elsewhere, "yomihon", "vaults.json")) // #nosec G304 -- the test's own temporary directory
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Join(configDir, "yomihon"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(configDir, "yomihon", "vaults.json"), data, 0o600); err != nil { // #nosec G703 -- the test's own temporary directory
		t.Fatal(err)
	}

	r := startRunning(t, configDir, newVaultFolder(t), start)
	r.address(t)
	if r.mountOf(old.ID) != nil {
		t.Errorf("%s, inside the configuration directory, is served", inside)
	}
	if old.Port != start {
		t.Fatalf("the vault listed first is on %d, want the starting port %d", old.Port, start)
	}
	if portFree(t.Context(), start) {
		t.Errorf("the starting port %d is not held, so a second yomihon would not find this one", start)
	}
	r.host.sync(t.Context(), true)
	r.host.sync(t.Context(), true)
	if told := strings.Count(r.log.String(), "not serving a vault in the list"); told != 1 {
		t.Errorf("the host said %d times that it does not serve the vault, want once:\n%s", told, r.log.String())
	}
	if !strings.Contains(r.log.String(), refusalSays) {
		t.Errorf("the host did not say why it does not serve the vault:\n%s", r.log.String())
	}

	var stdout, stderr bytes.Buffer
	if exit := runVaults(t.Context(), []string{removeVerb, inside}, config{configDir: confdir.At(configDir)}, &stdout, &stderr, false); exit != 0 {
		t.Fatalf("yomihon vaults remove %s = %d: %s", inside, exit, stderr.String())
	}
	list, err := listOf(t, configDir).List()
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(list, func(e vaults.Entry) bool { return e.ID == old.ID }) {
		t.Errorf("%s is still listed after it was removed", inside)
	}
}

// Adopting an earlier path's marks writes this folder's marks, and a folder
// that holds the configuration directory is refused before anything is written
// into it.
func TestMarksAdoptRefusesAFolderThatHoldsTheConfigurationDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	putOpenThoughtFile(t, root, "Source.md", "# Source\n\n## Chapter\n\nwords\n")
	configDir := filepath.Join(root, ".config")
	earlier := filepath.Join(t.TempDir(), "earlier")
	writeEarlierMarks(t, configDir, earlier, mark.Mark{ID: markID(1), Kind: mark.Question, RelPath: "Source.md", Anchor: "chapter", At: leftOpenDay(1)})
	before := vaultState(t, root)
	got := leftOpen(t, root, configDir, "--adopt", earlier)
	if got.exit != 2 || !strings.Contains(got.stderr, refusalSays) || !strings.Contains(got.stderr, "nothing was adopted") {
		t.Errorf("yomihon marks --adopt on a folder that holds the configuration directory = exit %d, %q; want 2 and the refusal", got.exit, got.stderr)
	}
	after := vaultState(t, root)
	for rel := range after {
		if before[rel] != after[rel] {
			t.Errorf("%s was written in the folder: before %q, after %q", rel, before[rel], after[rel])
		}
	}
}

// Approving or revoking a figure writes the approvals, and a folder that holds
// the configuration directory is refused before they are opened.
func TestAFigureOfAFolderThatHoldsTheConfigurationDirectoryIsNeitherApprovedNorRevoked(t *testing.T) {
	t.Parallel()
	root, _ := approvalVault(t)
	configDir := filepath.Join(root, ".config")
	for _, command := range []string{revokeFigureCommand, approveFigureCommand} {
		exit, _, errOut := runApproval(t, root, configDir, true, "", command, "Figures/moving.html")
		if exit != 2 || !strings.Contains(errOut, refusalSays) {
			t.Errorf("yomihon %s on a folder that holds the configuration directory = exit %d, %q; want 2 and the refusal", command, exit, errOut)
		}
	}
	if _, err := os.Stat(configDir); !os.IsNotExist(err) {
		t.Errorf("a figure command made the configuration directory inside the folder: %v", err)
	}
}

// A listed folder that is not there when yomihon starts is mounted, so that it
// can say it cannot be read; one that comes back inside the configuration
// directory is compared again when it is read, and is not read.
func TestAListedFolderThatComesBackInsideTheConfigurationDirectoryIsNotRead(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	inside := filepath.Join(configDir, "notes")
	writeRecoverySiteFixture(t, inside)
	start := someFreePort(t)
	elsewhere := t.TempDir()
	entry, _, err := listOf(t, elsewhere).Add(inside, start)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(elsewhere, "yomihon", "vaults.json")) // #nosec G304 -- the test's own temporary directory
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Join(configDir, "yomihon"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(configDir, "yomihon", "vaults.json"), data, 0o600); err != nil { // #nosec G703 -- the test's own temporary directory
		t.Fatal(err)
	}
	if err = os.RemoveAll(inside); err != nil {
		t.Fatal(err)
	}

	r := startRunning(t, configDir, newVaultFolder(t), start)
	r.address(t)
	if r.mountOf(entry.ID) == nil {
		t.Fatalf("%s, gone when yomihon started, is not mounted to say it cannot be read", inside)
	}
	writeRecoverySiteFixture(t, inside)
	address := addressOfEntry(&entry)
	eventually(t, "the folder put back inside the configuration directory to be refused when read", func() bool {
		status, body, _ := httpGet(t, address, "/")
		return status == http.StatusServiceUnavailable && strings.Contains(body, `data-nothing="unreadable"`) && strings.Contains(r.log.String(), refusalSays)
	})
	if r.mountOf(entry.ID).site.Load() != nil {
		t.Errorf("%s was read", inside)
	}
	if _, err := os.Stat(filepath.Join(configDir, "yomihon", entry.ID)); !os.IsNotExist(err) {
		t.Errorf("something was kept for %s: %v", inside, err)
	}
}

// A site is built only for a folder known to be apart from the configuration
// directory the process keeps things in, so one that cannot be compared with
// it, here one under a file, is not read. A folder that could not be compared
// at the start never reaches here with that directory: this is a listed vault,
// read by a yomihon whose configuration directory was usable when it began.
func TestASiteIsNotBuiltForAFolderThatCannotBeToldApartFromTheConfigurationDirectory(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows reads a name below a file as one that is not there")
	}
	base := t.TempDir()
	home := filepath.Join(base, "home")
	if err := os.WriteFile(home, []byte("a file"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "notes")
	writeDeskFixture(t, root)
	site, err := newReadingSite(t.Context(), root, confdir.At(filepath.Join(home, ".config")), slog.New(slog.DiscardHandler))
	if err == nil {
		closeErr := site.close()
		t.Fatalf("newReadingSite with a configuration directory that cannot be compared = a site (close: %v), want the folder not read", closeErr)
	}
	if _, overlaps := errors.AsType[*confdir.OverlapError](err); overlaps {
		t.Errorf("newReadingSite = %v, want the error that stopped the comparison, not an overlap", err)
	}
}

// A configuration directory that cannot be compared with the folder, here one
// under a file, is no reason not to read: the folder is served with nothing
// kept, as with no configuration directory, and the log says why.
//
// Not parallel: it sets the environment.
func TestAConfigurationDirectoryThatCannotBeComparedIsReadWithNothingKept(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home")
	if err := os.WriteFile(home, []byte("a file"), 0o600); err != nil {
		t.Fatal(err)
	}
	platformConfigUnder(t, home)
	root := filepath.Join(base, "notes")
	writeDeskFixture(t, root)
	t.Setenv("YOMIHON_PORT", strconv.Itoa(someFreePort(t)))
	cfg, err := loadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.configDir.IsZero() {
		t.Fatal("the environment names no configuration directory, so this shows nothing")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()
	var out bytes.Buffer
	logged := new(lockedBuffer)
	if err := serveVault(ctx, cancel, slog.New(slog.NewTextHandler(logged, nil)), &cfg, &out); err != nil {
		t.Fatalf("serving with a configuration directory that cannot be compared = %v, want the folder read", err)
	}
	if !strings.Contains(out.String(), "http://127.0.0.1:") {
		t.Errorf("serving printed %q, want the folder's address", out.String())
	}
	if said := logged.String(); !strings.Contains(said, "no configuration directory; reading is unaffected") || !strings.Contains(said, "apart from the configuration directory") {
		t.Errorf("the log does not say that nothing is kept, and why:\n%s", said)
	}
}
