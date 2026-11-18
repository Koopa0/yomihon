package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/mark/vaults"
)

// nextBlock hands each test a block of ports of its own. The vaults a test adds
// take the ports above its starting one, and the list probes them by binding, so
// two tests whose starting ports sit side by side would take each other's. The
// first block is taken from the process's own id, since several runs of these
// tests share a machine, and each counting up from the same port would meet.
var nextBlock = func() *atomic.Int32 {
	var first atomic.Int32
	first.Store(int32(os.Getpid() % blocks)) // #nosec G115 -- a number below blocks
	return &first
}()

const (
	portsPerTest = 40
	firstPort    = 20000
	// blocks is how many blocks there are, and they end at 32000. Linux hands
	// the own end of a connection a port from 32768 and macOS and Windows from
	// 49152, and any test's client can be given one, so a block that reached
	// them could have a port taken between the look at it and the bind.
	blocks = 300
)

// someFreePort is the starting port for a test, with the forty above it left to
// that test alone. Every port of the block is one nothing holds now.
//
// The block is claimed before it is looked at: looking is binding, and another
// run's block that is looked at while it is being used has a port it has not
// bound yet taken for the moment of the look.
func someFreePort(t *testing.T) int {
	t.Helper()
	for range blocks {
		port := firstPort + portsPerTest*(int(nextBlock.Add(1))%blocks)
		release, ok := claimBlock(port)
		if !ok {
			continue
		}
		if !blockFree(t.Context(), port) {
			release()
			continue
		}
		t.Cleanup(release)
		return port
	}
	t.Fatal("no block of ports is free for this test")
	return 0
}

// staleClaim is how long a claim on a block stands before it is taken to be one a
// run that was killed left behind.
const staleClaim = 20 * time.Minute

// claimDir is where the claims are made: the user's temporary directory, which
// every run of the tests by that user on the machine shares.
func claimDir() string { return filepath.Join(os.TempDir(), "yomihon-test-ports") }

// claimBlock records, where every run of these tests on this machine can see it,
// that this run has the block that starts at port. Another run that is using
// its blocks at the same time, as several do on a machine that several people
// work on, then takes another block instead of the same ports. A directory is made
// for it because making one either succeeds or does not, whoever else is making it.
// Where nothing can be recorded the block is used unclaimed.
func claimBlock(port int) (release func(), ok bool) {
	dir := claimDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return func() {}, true
	}
	claim := filepath.Join(dir, strconv.Itoa(port))
	for range 2 {
		err := os.Mkdir(claim, 0o750)
		if err == nil {
			return func() { _ = os.Remove(claim) }, true //nolint:errcheck // a claim that stays is taken over when it is stale
		}
		if !errors.Is(err, os.ErrExist) {
			return func() {}, true
		}
		if info, statErr := os.Stat(claim); statErr == nil && time.Since(info.ModTime()) < staleClaim {
			return nil, false
		}
		_ = os.Remove(claim) //nolint:errcheck // another run may have taken it over first, and then this one asks again
	}
	return nil, false
}

// blockFree says whether nothing holds any port of the block that starts at port.
func blockFree(ctx context.Context, port int) bool {
	for offset := range portsPerTest {
		if !portFree(ctx, port+offset) {
			return false
		}
	}
	return true
}

// A block one run has is not given to another, until the first lets go of it or
// is old enough to have been killed.
func TestABlockOfPortsOneRunHasIsNotGivenToAnotherRun(t *testing.T) {
	t.Parallel()
	port := 100000 + os.Getpid() // not a port, so no block of a real test is touched
	release, ok := claimBlock(port)
	if !ok {
		t.Fatal("a block nobody has was not given")
	}
	if _, again := claimBlock(port); again {
		t.Error("a block that is claimed was given again")
	}
	release()
	release, ok = claimBlock(port)
	if !ok {
		t.Fatal("a block that was let go of was not given")
	}
	defer release()

	claim := filepath.Join(claimDir(), strconv.Itoa(port))
	old := time.Now().Add(-2 * staleClaim)
	if err := os.Chtimes(claim, old, old); err != nil {
		t.Fatal(err)
	}
	release2, ok := claimBlock(port)
	if !ok {
		t.Fatal("a claim old enough to be a killed run's was not taken over")
	}
	release2()
}

// holder binds port on this machine's loopback and holds it until the test ends,
// as another program would. It is taken from the test's own block, so that the
// ports beside it are the test's too and not another test's ephemeral ones.
func holder(t *testing.T, port int) {
	t.Helper()
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("hold port %d: %v", port, err)
	}
	t.Cleanup(func() { _ = listener.Close() }) //nolint:errcheck // the port is released either way
}

func newVaultFolder(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeRecoverySiteFixture(t, root)
	return root
}

// listOf opens the list the way the command line does, which is how a second
// process changes it.
func listOf(t *testing.T, configDir string) *vaults.Store {
	t.Helper()
	store, err := vaults.Open(configDir, func(port int) bool { return portFree(t.Context(), port) })
	if err != nil {
		t.Fatalf("open the list: %v", err)
	}
	return store
}

// running is a yomihon serving the list, started the way `yomihon <folder>`
// starts one.
type running struct {
	host   *host
	out    *lockedBuffer
	cancel context.CancelFunc
	done   chan error

	stopped sync.Once
	result  error
}

func startRunning(t *testing.T, configDir, root string, port int) *running {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	logged := new(lockedBuffer)
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("the host's log:\n%s", logged.String())
		}
	})
	r := &running{
		host:   newHost(slog.New(slog.NewTextHandler(logged, nil)), configDir, listOf(t, configDir)),
		out:    new(lockedBuffer),
		cancel: cancel,
		done:   make(chan error, 1),
	}
	r.host.wait = 300 * time.Millisecond
	cfg := &config{root: root, port: strconv.Itoa(port), configDir: configDir}
	go func() { r.done <- r.host.serveListed(ctx, func() {}, cfg, r.out) }()
	t.Cleanup(func() { _ = r.stop(t) }) //nolint:errcheck // a test that cares about the result stopped it itself
	return r
}

var printedAddress = regexp.MustCompile(`http://\S+:\d+/`)

// address is what the process printed, waited for.
func (r *running) address(t *testing.T) string {
	t.Helper()
	var found string
	eventually(t, "the address to be printed", func() bool {
		select {
		case err := <-r.done:
			// The result is the one stop would wait for, and nothing else sends
			// another, so the cleanup that stops the host has to be told it has ended.
			r.stopped.Do(func() { r.result = err })
			t.Fatalf("the host ended before it printed an address: %v", err)
		default:
		}
		found = printedAddress.FindString(r.out.String())
		return found != ""
	})
	return found
}

// stop ends the process and answers how it ended. It can be asked again.
func (r *running) stop(t *testing.T) error {
	t.Helper()
	r.stopped.Do(func() {
		r.cancel()
		select {
		case r.result = <-r.done:
		case <-time.After(30 * time.Second):
			t.Error("the host did not stop")
		}
	})
	return r.result
}

func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (r *running) mountOf(id string) *mount { return r.host.find(id) }

func httpGet(t *testing.T, address, path string) (status int, body string, header http.Header) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, address+strings.TrimPrefix(path, "/"), http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s%s: %v", address, path, err)
	}
	data, err := io.ReadAll(resp.Body)
	if closeErr := resp.Body.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(data), resp.Header
}

func addressOfEntry(e *vaults.Entry) string { return addressOf(e.Port) }

func idOf(t *testing.T, store *vaults.Store, root string) vaults.Entry {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	for i := range list {
		if list[i].Path == resolved {
			return list[i]
		}
	}
	t.Fatalf("%s is not in the list %+v", resolved, list)
	return vaults.Entry{}
}

func TestTheRunningYomihonServesEveryListedVaultOnItsOwnPort(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	a, b, c := newVaultFolder(t), newVaultFolder(t), newVaultFolder(t)
	start := someFreePort(t)
	pre := listOf(t, configDir)
	for _, root := range []string{b, c} {
		if _, _, err := pre.Add(root, start); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}

	r := startRunning(t, configDir, a, start)
	printed := r.address(t)

	list, err := pre.List()
	if err != nil || len(list) != 3 {
		t.Fatalf("list = %+v, %v; want the three vaults", list, err)
	}
	ports := map[int]bool{}
	for i := range list {
		if ports[list[i].Port] {
			t.Fatalf("two vaults on port %d: %+v", list[i].Port, list)
		}
		ports[list[i].Port] = true
		if r.mountOf(list[i].ID) == nil {
			t.Errorf("%s is listed and not served", list[i].Path)
		}
	}
	if typed := idOf(t, pre, a); printed != addressOfEntry(&typed) {
		t.Errorf("printed %s, want the address of the folder just given, %s", printed, addressOfEntry(&typed))
	}
	if first := idOf(t, pre, b); first.Port != start {
		t.Errorf("the first vault is on %d, want the starting port, %d", first.Port, start)
	}

	// The one just given is read at once; the others are read when asked for.
	typed := r.mountOf(idOf(t, pre, a).ID)
	eventually(t, "the vault just given to be read", func() bool { return typed.site.Load() != nil })
	for _, root := range []string{b, c} {
		if r.mountOf(idOf(t, pre, root).ID).site.Load() != nil {
			t.Errorf("%s was read before anybody asked for it", root)
		}
	}
	other := idOf(t, pre, b)
	eventually(t, "a vault to be read when asked for", func() bool {
		status, body, _ := httpGet(t, addressOfEntry(&other), "/")
		return status == http.StatusOK && !strings.Contains(body, "data-scanning")
	})
}

func TestAVaultTheCommandLineAddsIsServedWithinAPoll(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	r := startRunning(t, configDir, newVaultFolder(t), someFreePort(t))
	r.address(t)

	added := newVaultFolder(t)
	entry, isNew, err := listOf(t, configDir).Add(added, 1024)
	if err != nil || !isNew {
		t.Fatalf("Add = %+v, %v, %v", entry, isNew, err)
	}
	eventually(t, "the added vault to be served", func() bool { return r.mountOf(entry.ID) != nil })
	if portFree(t.Context(), entry.Port) {
		t.Errorf("nothing holds port %d, which the list gives the added vault", entry.Port)
	}
}

func TestARemovedVaultIsServedUntilExitAndKeepsItsPortWhenAddedAgain(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	first, second := newVaultFolder(t), newVaultFolder(t)
	start := someFreePort(t)
	r := startRunning(t, configDir, first, start)
	r.address(t)
	cli := listOf(t, configDir)
	entry, _, err := cli.Add(second, start)
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, "the second vault to be served", func() bool { return r.mountOf(entry.ID) != nil })

	if _, err = cli.Remove(entry.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	// Gone from the list at once, and still answering.
	list, err := cli.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list after the removal = %+v, %v; want one vault", list, err)
	}
	time.Sleep(3 * pollEvery)
	if r.mountOf(entry.ID) == nil || portFree(t.Context(), entry.Port) {
		t.Fatalf("the removed vault stopped being served; it stays until the process ends")
	}

	// Added again, it is the same vault on the same port, and the list is made
	// to say so rather than the vault being served twice.
	again, _, err := cli.Add(second, start)
	if err != nil || again.ID != entry.ID {
		t.Fatalf("Add again = %+v, %v", again, err)
	}
	eventually(t, "the list to say the vault is where it is served", func() bool {
		now := idOf(t, cli, second)
		return now.Port == entry.Port
	})
}

func TestASecondYomihonGivesItsFolderToTheRunningOne(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	start := someFreePort(t)
	r := startRunning(t, configDir, newVaultFolder(t), start)
	r.address(t)

	given := newVaultFolder(t)
	second := newHost(slog.New(slog.DiscardHandler), configDir, listOf(t, configDir))
	second.wait = 10 * time.Second
	out := new(lockedBuffer)
	cfg := &config{root: given, port: strconv.Itoa(start), configDir: configDir}
	if err := second.serveListed(t.Context(), func() {}, cfg, out); err != nil {
		t.Fatalf("the second yomihon: %v", err)
	}
	entry := idOf(t, listOf(t, configDir), given)
	if got := strings.TrimSpace(out.String()); got != addressOfEntry(&entry) {
		t.Errorf("the second yomihon printed %q, want the address the running one serves it on, %s", got, addressOfEntry(&entry))
	}
	if r.mountOf(entry.ID) == nil {
		t.Errorf("the running yomihon does not serve the folder it was given")
	}
}

// A listed vault is never moved. The vault just typed is new, so it takes the
// starting port when no vault is on it, and the one listed before stays where
// its address already is.
func TestANewVaultTakesTheStartingPortWhenNoVaultIsOnItAndNoVaultIsMoved(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	old, current := newVaultFolder(t), newVaultFolder(t)
	was := someFreePort(t)
	pre, _, err := listOf(t, configDir).Add(old, was)
	if err != nil {
		t.Fatal(err)
	}

	start := someFreePort(t)
	r := startRunning(t, configDir, current, start)
	if got, want := r.address(t), addressOf(start); got != want {
		t.Errorf("the new vault is at %s, want the starting port, %s", got, want)
	}
	if kept := idOf(t, listOf(t, configDir), old); kept.Port != pre.Port {
		t.Errorf("the vault listed before is on %d, was on %d; a listed vault does not move", kept.Port, pre.Port)
	}
}

// A vault already listed on another port is still served there, and the starting
// port, which the process holds, sends a browser to it.
func TestTheStartingPortSendsABrowserToAVaultListedOnAnotherPort(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	folder := newVaultFolder(t)
	was := someFreePort(t)
	pre, _, err := listOf(t, configDir).Add(folder, was)
	if err != nil {
		t.Fatal(err)
	}

	start := someFreePort(t)
	r := startRunning(t, configDir, folder, start)
	if got, want := r.address(t), addressOf(pre.Port); got != want {
		t.Fatalf("the vault is at %s, want the port the list gave it, %s", got, want)
	}
	if kept := idOf(t, listOf(t, configDir), folder); kept.Port != pre.Port {
		t.Errorf("the vault was moved from %d to %d", pre.Port, kept.Port)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, addressOf(start), http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET the starting port: %v", err)
	}
	if err = resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusTemporaryRedirect || resp.Header.Get("Location") != addressOf(pre.Port) {
		t.Errorf("the starting port answered %d to %q, want a redirect to %s", resp.StatusCode, resp.Header.Get("Location"), addressOf(pre.Port))
	}
	// It stands behind the host-name check the vaults do, so it names a vault only
	// to a request for this machine.
	foreign, err := http.NewRequestWithContext(t.Context(), http.MethodGet, addressOf(start), http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	foreign.Host = "evil.example"
	resp, err = client.Do(foreign)
	if err != nil {
		t.Fatalf("GET the starting port as another host: %v", err)
	}
	if err = resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if resp.Header.Get("Location") != "" {
		t.Errorf("a request for another host name was sent to %q", resp.Header.Get("Location"))
	}
	eventually(t, "the vault to be read", func() bool {
		status, body, _ := httpGet(t, addressOf(pre.Port), "/")
		return status == http.StatusOK && !strings.Contains(body, "data-scanning")
	})
}

// A record that names a live process and a held port is believed until the wait
// has passed, because this process opens no connection to ask the port who it
// is. When nothing there serves the folder, the folder is not left in the list,
// and the refusal names the file that says yomihon is running and the port.
func TestAHandOffNothingTakesLeavesNothingInTheListAndSaysWhichFileToRemove(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	squatted := someFreePort(t)
	holder(t, squatted)
	store := listOf(t, configDir)
	// This test's own process stands for the live process that is not yomihon.
	if err := store.WriteInstance(vaults.Instance{PID: os.Getpid(), Port: squatted, Started: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}

	h := newHost(slog.New(slog.DiscardHandler), configDir, store)
	h.wait = 300 * time.Millisecond
	out := new(lockedBuffer)
	cfg := &config{root: newVaultFolder(t), port: strconv.Itoa(someFreePort(t)), configDir: configDir}
	err := h.serveListed(t.Context(), func() {}, cfg, out)
	fault := operatorFault(err)
	if fault == nil {
		t.Fatalf("serveListed = %v, want a refusal the operator can correct", err)
	}
	for _, want := range []string{store.InstanceFile(), strconv.Itoa(squatted)} {
		if !strings.Contains(fault.Error(), want) {
			t.Errorf("the refusal %q does not name %q", fault.Error(), want)
		}
	}
	if out.String() != "" {
		t.Errorf("an address was printed for a folder nothing served: %q", out.String())
	}
	if list, listErr := store.List(); listErr != nil || len(list) != 0 {
		t.Errorf("the folder nothing took is in the list: %+v, %v", list, listErr)
	}
}

// A folder already in the list was not added by this hand-off, so a hand-off that
// fails does not take it out.
func TestAHandOffThatFailsKeepsAFolderTheListAlreadyHad(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	squatted := someFreePort(t)
	holder(t, squatted)
	store := listOf(t, configDir)
	folder := newVaultFolder(t)
	if _, _, err := store.Add(folder, someFreePort(t)); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteInstance(vaults.Instance{PID: os.Getpid(), Port: squatted, Started: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	h := newHost(slog.New(slog.DiscardHandler), configDir, store)
	h.wait = 300 * time.Millisecond
	cfg := &config{root: folder, port: strconv.Itoa(someFreePort(t)), configDir: configDir}
	if operatorFault(h.serveListed(t.Context(), func() {}, cfg, io.Discard)) == nil {
		t.Fatal("want a refusal")
	}
	if list, listErr := store.List(); listErr != nil || len(list) != 1 {
		t.Errorf("the list holds %+v, %v; want the folder it already had", list, listErr)
	}
}

// A port the reader names for a yomihon that is already running somewhere else
// is not ignored: it cannot start a second one, since two would each serve the
// one list, and the reader is told so before anything is added.
func TestAPortNamedForAnotherYomihonThanTheRunningOneIsRefusedAndNothingIsAdded(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	start := someFreePort(t)
	r := startRunning(t, configDir, newVaultFolder(t), start)
	r.address(t)

	other := someFreePort(t)
	h := newHost(slog.New(slog.DiscardHandler), configDir, listOf(t, configDir))
	cfg := &config{root: newVaultFolder(t), port: strconv.Itoa(other), portSet: true, configDir: configDir}
	err := h.serveListed(t.Context(), func() {}, cfg, io.Discard)
	fault := operatorFault(err)
	if fault == nil || !strings.Contains(fault.Error(), strconv.Itoa(start)) || !strings.Contains(fault.Error(), "YOMIHON_PORT") {
		t.Fatalf("serveListed = %v, want a refusal naming YOMIHON_PORT and port %d", err, start)
	}
	if list, listErr := h.store.List(); listErr != nil || len(list) != 1 {
		t.Errorf("the refused folder was added: %+v, %v", list, listErr)
	}
	if !portFree(t.Context(), other) {
		t.Errorf("a second yomihon is serving on %d", other)
	}
}

// The same port the running one holds is what a reader who always sets
// YOMIHON_PORT gets: the folder is handed over.
func TestTheRunningYomihonsOwnPortNamedAgainStillHandsTheFolderOver(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	start := someFreePort(t)
	r := startRunning(t, configDir, newVaultFolder(t), start)
	r.address(t)

	h := newHost(slog.New(slog.DiscardHandler), configDir, listOf(t, configDir))
	h.wait = 10 * time.Second
	out := new(lockedBuffer)
	cfg := &config{root: newVaultFolder(t), port: strconv.Itoa(start), portSet: true, configDir: configDir}
	if err := h.serveListed(t.Context(), func() {}, cfg, out); err != nil {
		t.Fatalf("serveListed = %v", err)
	}
	if !printedAddress.MatchString(out.String()) {
		t.Errorf("no address was printed: %q", out.String())
	}
}

// A starting port below the unprivileged ports is refused before anything is
// listed, whatever else is running.
func TestAStartingPortOfTheSystemsIsRefused(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	h := newHost(slog.New(slog.DiscardHandler), configDir, listOf(t, configDir))
	cfg := &config{root: newVaultFolder(t), port: "80", portSet: true, configDir: configDir}
	if operatorFault(h.serveListed(t.Context(), func() {}, cfg, io.Discard)) == nil {
		t.Fatal("want a refusal of port 80")
	}
	if list, err := h.store.List(); err != nil || len(list) != 0 {
		t.Errorf("a refused start listed %+v, %v", list, err)
	}
}

func TestAStartingPortAnotherProgramHoldsIsRefusedWithTheSettingThatMovesIt(t *testing.T) {
	t.Parallel()
	var lc net.ListenConfig
	held, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = held.Close() }) //nolint:errcheck // the port is released either way
	heldAt, err := netip.ParseAddrPort(held.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port := int(heldAt.Port())

	configDir := t.TempDir()
	h := newHost(slog.New(slog.DiscardHandler), configDir, listOf(t, configDir))
	h.wait = 100 * time.Millisecond
	cfg := &config{root: newVaultFolder(t), port: strconv.Itoa(port), configDir: configDir}
	err = h.serveListed(t.Context(), func() {}, cfg, io.Discard)
	if operatorFault(err) == nil {
		t.Fatalf("serveListed = %v, want the refusal an operator can correct", err)
	}
	if list, listErr := h.store.List(); listErr != nil || len(list) != 0 {
		t.Errorf("a folder that was never served is in the list: %+v, %v", list, listErr)
	}
}

func TestAVaultWhoseFolderIsGoneAnswersThatItCannotBeRead(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	gone := newVaultFolder(t)
	start := someFreePort(t)
	entry, _, err := listOf(t, configDir).Add(gone, start)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}

	r := startRunning(t, configDir, newVaultFolder(t), start)
	r.address(t)
	eventually(t, "the vault that is gone to answer that it cannot be read", func() bool {
		status, _, _ := httpGet(t, addressOfEntry(&entry), "/")
		return status == http.StatusServiceUnavailable
	})
	// The other vault is not the worse for it.
	if err = r.stop(t); err != nil {
		t.Errorf("the host stopped with %v", err)
	}
}

// "Read again" has to read again. A reader who puts the folder back, or fixes
// its permission, and follows the action gets the vault without restarting
// yomihon, and so does a later page asked for after the failure.
func TestAVaultWhoseFolderIsPutBackIsReadAgainWithoutRestarting(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	gone := newVaultFolder(t)
	start := someFreePort(t)
	entry, _, err := listOf(t, configDir).Add(gone, start)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}

	r := startRunning(t, configDir, newVaultFolder(t), start)
	r.address(t)
	address := addressOfEntry(&entry)
	eventually(t, "the vault that is gone to say it cannot be read", func() bool {
		status, body, _ := httpGet(t, address, "/")
		return status == http.StatusServiceUnavailable && strings.Contains(body, `data-nothing="unreadable"`)
	})

	if err = os.MkdirAll(gone, 0o750); err != nil {
		t.Fatal(err)
	}
	writeRecoverySiteFixture(t, gone)
	eventually(t, "the vault, put back, to be read when its address is asked for again", func() bool {
		status, body, _ := httpGet(t, address, "/")
		return status == http.StatusOK && !strings.Contains(body, "data-scanning") && !strings.Contains(body, `data-nothing="unreadable"`)
	})
}

// A folder reached through a symbolic link is the folder: one id, one set of
// marks, whichever spelling the reader types first.
func TestAFolderTypedThroughALinkIsTheSameVaultAsTheFolder(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	target := newVaultFolder(t)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
	r := startRunning(t, configDir, link, someFreePort(t))
	r.address(t)

	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	list, err := listOf(t, configDir).List()
	if err != nil || len(list) != 1 {
		t.Fatalf("the list holds %+v, %v; want the one vault", list, err)
	}
	if list[0].Path != resolved || list[0].ID != confdir.VaultID(resolved) {
		t.Errorf("the vault is %q (%s), want the resolved folder %q (%s)", list[0].Path, list[0].ID, resolved, confdir.VaultID(resolved))
	}
	again, added, err := listOf(t, configDir).Add(target, 1024)
	if err != nil || added || again.ID != list[0].ID {
		t.Errorf("the folder typed by its own path = %+v, new %v, %v; want the same vault", again, added, err)
	}
}

// A listed vault nobody has opened is read when a reader navigates to it, a card
// or a link on another vault's page included, and not for a picture, a frame or a
// fetch that another page pointed at its port, nor for the reading page's own
// request for how far the read has got.
func TestAnUnreadVaultIsReadWhenNavigatedToAndNotForASubresource(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	start := someFreePort(t)
	entry, _, err := listOf(t, configDir).Add(newVaultFolder(t), start+5)
	if err != nil {
		t.Fatal(err)
	}
	r := startRunning(t, configDir, newVaultFolder(t), start)
	r.address(t)
	address := addressOfEntry(&entry)

	get := func(path string, headers map[string]string) int {
		req, reqErr := http.NewRequestWithContext(t.Context(), http.MethodGet, address+strings.TrimPrefix(path, "/"), http.NoBody)
		if reqErr != nil {
			t.Fatal(reqErr)
		}
		for name, value := range headers {
			req.Header.Set(name, value)
		}
		resp, doErr := (&http.Client{Timeout: 10 * time.Second}).Do(req)
		if doErr != nil {
			t.Fatal(doErr)
		}
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		return resp.StatusCode
	}
	for name, headers := range map[string]map[string]string{
		"a picture of another site":      {"Sec-Fetch-Site": "cross-site", "Sec-Fetch-Dest": "image", "Sec-Fetch-Mode": "no-cors"},
		"a frame of another vault":       {"Sec-Fetch-Site": "same-site", "Sec-Fetch-Dest": "iframe", "Sec-Fetch-Mode": "navigate"},
		"a fetch of another vault":       {"Sec-Fetch-Site": "same-site", "Sec-Fetch-Dest": "empty", "Sec-Fetch-Mode": "cors"},
		"the reading page's own request": {"Sec-Fetch-Site": "same-origin", "Sec-Fetch-Dest": "empty", "Sec-Fetch-Mode": "cors"},
	} {
		for range 2 {
			if status := get("/", headers); status != http.StatusServiceUnavailable {
				t.Fatalf("%s got %d, want the page that says the vault is being read", name, status)
			}
		}
		get(scanAddress, headers)
		if m := r.mountOf(entry.ID); m == nil || m.site.Load() != nil {
			t.Fatalf("the vault was read for %s", name)
		}
	}

	card := map[string]string{"Sec-Fetch-Site": "same-site", "Sec-Fetch-Dest": "document", "Sec-Fetch-Mode": "navigate"}
	if status := get("/", card); status != http.StatusServiceUnavailable && status != http.StatusOK {
		t.Fatalf("a card followed from another vault got %d", status)
	}
	eventually(t, "the vault to be read when navigated to", func() bool {
		m := r.mountOf(entry.ID)
		return m != nil && m.site.Load() != nil
	})
}

// A request that names another host name is refused before the vault is woken
// and before its read is recorded: a page on a rebound name sends no fetch
// metadata, which would pass for an address typed, and its picture would read the
// folder and rewrite the list.
func TestARequestForAnotherHostNameNeitherReadsTheVaultNorRecordsARead(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	start := someFreePort(t)
	folder := newVaultFolder(t)
	pre := listOf(t, configDir)
	entry, _, err := pre.Add(folder, start+5)
	if err != nil {
		t.Fatal(err)
	}
	// The vault was last read long ago, so a record of a read would be written.
	long := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	data, err := json.Marshal(struct {
		Version int            `json:"version"`
		Vaults  []vaults.Entry `json:"vaults"`
	}{1, []vaults.Entry{{ID: entry.ID, Path: entry.Path, Name: entry.Name, LastRead: long, Port: entry.Port}}})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(configDir, "yomihon", "vaults.json"), data, 0o600); err != nil { // #nosec G703 -- a file under t.TempDir
		t.Fatal(err)
	}

	r := startRunning(t, configDir, newVaultFolder(t), start)
	r.address(t)
	address := addressOfEntry(&entry)
	lastRead := func() time.Time { return idOf(t, listOf(t, configDir), folder).LastRead }

	request := func(host string) int {
		req, reqErr := http.NewRequestWithContext(t.Context(), http.MethodGet, address, http.NoBody)
		if reqErr != nil {
			t.Fatal(reqErr)
		}
		req.Host = host
		// What a browser sends for a picture on a page whose name was rebound to
		// this address: nothing a page could be told apart by, and a document load.
		req.Header.Set("Sec-Fetch-Dest", "document")
		resp, doErr := (&http.Client{Timeout: 10 * time.Second}).Do(req)
		if doErr != nil {
			t.Fatal(doErr)
		}
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		return resp.StatusCode
	}
	for _, host := range []string{"rebind.example:" + strconv.Itoa(entry.Port), "rebind.example"} {
		if status := request(host); status != http.StatusForbidden {
			t.Fatalf("a request for %q = %d, want 403", host, status)
		}
	}
	time.Sleep(500 * time.Millisecond) // a record, if one were made, is made at once and in the background
	if m := r.mountOf(entry.ID); m == nil || m.site.Load() != nil {
		t.Fatal("the vault was read for a request that was refused")
	}
	if got := lastRead(); !got.Equal(long) {
		t.Fatalf("a request that was refused rewrote the vault's last read to %v", got)
	}

	// The same request for this machine is what records a read, which shows the
	// record is made when it should be and that the check above could have failed.
	if status := request(net.JoinHostPort("127.0.0.1", strconv.Itoa(entry.Port))); status != http.StatusServiceUnavailable && status != http.StatusOK {
		t.Fatalf("a request for this machine = %d", status)
	}
	eventually(t, "a request for this machine to record a read", func() bool { return lastRead().After(long) })
}

// The record of a read is written before the host ends, and not after: one that
// came later could land in a configuration directory that is being removed, or be
// cut off halfway by the process ending.
func TestStoppingWaitsForTheRecordOfARead(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	start := someFreePort(t)
	folder := newVaultFolder(t)
	entry, _, err := listOf(t, configDir).Add(folder, start+5)
	if err != nil {
		t.Fatal(err)
	}
	// The vault was last read long ago, so a record of a read would be written.
	long := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	data, err := json.Marshal(struct {
		Version int            `json:"version"`
		Vaults  []vaults.Entry `json:"vaults"`
	}{1, []vaults.Entry{{ID: entry.ID, Path: entry.Path, Name: entry.Name, LastRead: long, Port: entry.Port}}})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(configDir, "yomihon", "vaults.json"), data, 0o600); err != nil { // #nosec G703 -- a file under t.TempDir
		t.Fatal(err)
	}
	r := startRunning(t, configDir, newVaultFolder(t), start)
	r.address(t)

	// Another process is changing the list, so the record has to wait its turn.
	home, err := confdir.Home(configDir)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := confdir.Acquire(home, "vaults.lock", []byte(strconv.Itoa(os.Getpid())+" 1 1"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var release sync.Once
	letGo := func() { release.Do(func() { _ = lock.Release() }) } //nolint:errcheck // the lock is a file of this test's directory
	t.Cleanup(letGo)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, addressOfEntry(&entry), http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Sec-Fetch-Dest", "document")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if err = resp.Body.Close(); err != nil {
		t.Fatal(err)
	}

	stopped := make(chan error, 1)
	go func() { stopped <- r.stop(t) }()
	select {
	case err = <-stopped:
		t.Fatalf("the host ended while the record of a read was still waiting to be written: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	letGo()
	if err = <-stopped; err != nil {
		t.Fatalf("stopping: %v", err)
	}
	if got := idOf(t, listOf(t, configDir), folder).LastRead; !got.After(long) {
		t.Errorf("the record was not written before the host ended: last read is %v", got)
	}
}

func TestStoppingLeavesNothingThatSaysYomihonIsRunning(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	r := startRunning(t, configDir, newVaultFolder(t), someFreePort(t))
	address := r.address(t)
	store := listOf(t, configDir)
	if _, ok := store.Running(); !ok {
		t.Fatalf("the running yomihon left no record that it is running")
	}
	if err := r.stop(t); err != nil {
		t.Fatalf("stopping: %v", err)
	}
	if in, ok := store.Running(); ok {
		t.Errorf("a stopped yomihon is still recorded as running: %+v", in)
	}
	port, err := strconv.Atoi(strings.TrimSuffix(address[strings.LastIndex(address, ":")+1:], "/"))
	if err != nil {
		t.Fatal(err)
	}
	if !portFree(t.Context(), port) {
		t.Errorf("port %d is still held after stopping", port)
	}
}

// The second promise, for every listener: a vault the running yomihon serves
// answers on loopback and nowhere else, however it came to be served.
func TestEveryVaultListenerIsLoopbackOnly(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	start := someFreePort(t)
	pre := listOf(t, configDir)
	for range 2 {
		if _, _, err := pre.Add(newVaultFolder(t), start); err != nil {
			t.Fatal(err)
		}
	}
	r := startRunning(t, configDir, newVaultFolder(t), start)
	r.address(t)
	added, _, err := pre.Add(newVaultFolder(t), start)
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, "the vault added while running to be served", func() bool { return r.mountOf(added.ID) != nil })

	list, err := pre.List()
	if err != nil || len(list) != 4 {
		t.Fatalf("list = %+v, %v; want four vaults", list, err)
	}
	for i := range list {
		m := r.mountOf(list[i].ID)
		if m == nil {
			t.Fatalf("%s is listed and not served", list[i].Path)
		}
		tcp, ok := m.listener.Addr().(*net.TCPAddr)
		if !ok || !tcp.IP.Equal(net.IPv4(127, 0, 0, 1)) {
			t.Errorf("%s is served on %v, want 127.0.0.1", list[i].Path, m.listener.Addr())
		}
	}
}

// Nothing that belongs to a vault is kept in a cookie: a cookie is shared by
// every port of the host name, so a vault's state in one would be sent to every
// other vault yomihon serves.
func TestNoVaultsStateIsEverSetAsACookie(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	start := someFreePort(t)
	pre := listOf(t, configDir)
	if _, _, err := pre.Add(newVaultFolder(t), start); err != nil {
		t.Fatal(err)
	}
	r := startRunning(t, configDir, newVaultFolder(t), start)
	r.address(t)

	list, err := pre.List()
	if err != nil {
		t.Fatal(err)
	}
	for i := range list {
		address := addressOfEntry(&list[i])
		own := strings.TrimSuffix(address, "/")
		var sets []string
		for _, path := range []string{"/", "/search?q=a", "/health", "/nothing-here"} {
			var header http.Header
			eventually(t, "the vault to be read", func() bool {
				var status int
				var body string
				status, body, header = httpGet(t, address, path)
				return status != http.StatusServiceUnavailable && !strings.Contains(body, "data-scanning")
			})
			sets = append(sets, header.Values("Set-Cookie")...)
		}
		// The one thing that does set a cookie is a preference, and that is the
		// reader's across every vault, so it is written and read like the rest.
		req, reqErr := http.NewRequestWithContext(t.Context(), http.MethodPost, address+"preferences", strings.NewReader("theme=dark"))
		if reqErr != nil {
			t.Fatal(reqErr)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", own)
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		resp, doErr := (&http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(req)
		if doErr != nil {
			t.Fatal(doErr)
		}
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		sets = append(sets, resp.Header.Values("Set-Cookie")...)
		if !slices.ContainsFunc(sets, func(set string) bool { return strings.HasPrefix(set, "yomihon_theme=dark") }) {
			t.Fatalf("%s: writing a preference set no cookie, so this test has nothing to look at: %q", address, sets)
		}

		for _, set := range sets {
			name, value, _ := strings.Cut(set, "=")
			value, _, _ = strings.Cut(value, ";")
			if !strings.HasPrefix(name, "yomihon_") {
				t.Errorf("%s sets a cookie, %s, that is not one of the reader's preferences", address, name)
			}
			for _, mine := range []string{list[i].ID, list[i].Path, list[i].Name, strconv.Itoa(list[i].Port)} {
				if mine != "" && strings.Contains(value, mine) {
					t.Errorf("%s sets %s with something of the vault in it: %q", address, name, value)
				}
			}
		}
	}
}

// A page of one vault is on the same host name as every other vault, and a
// different port is the same site. The browser says so, and a write across it
// is refused, so a page of one vault cannot change what another keeps.
func TestAWriteFromOneVaultsPageToAnotherVaultsPortIsRefused(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	start := someFreePort(t)
	pre := listOf(t, configDir)
	if _, _, err := pre.Add(newVaultFolder(t), start); err != nil {
		t.Fatal(err)
	}
	r := startRunning(t, configDir, newVaultFolder(t), start)
	r.address(t)
	list, err := pre.List()
	if err != nil || len(list) != 2 {
		t.Fatalf("list = %+v, %v", list, err)
	}
	page, target := list[0], list[1]
	eventually(t, "the target vault to be read", func() bool {
		status, body, _ := httpGet(t, addressOfEntry(&target), "/")
		return status == http.StatusOK && !strings.Contains(body, "data-scanning")
	})

	post := func(headers map[string]string) int {
		t.Helper()
		form := url.Values{"path": {"Maps/study.md"}, "offset": {"0"}, "identity": {strings.Repeat("a", 64)}}
		req, reqErr := http.NewRequestWithContext(t.Context(), http.MethodPost, addressOfEntry(&target)+"bookmark", strings.NewReader(form.Encode()))
		if reqErr != nil {
			t.Fatal(reqErr)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for name, value := range headers {
			req.Header.Set(name, value)
		}
		resp, doErr := (&http.Client{Timeout: 10 * time.Second}).Do(req)
		if doErr != nil {
			t.Fatal(doErr)
		}
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		return resp.StatusCode
	}
	fromPage := strings.TrimSuffix(addressOfEntry(&page), "/")
	if got := post(map[string]string{"Origin": fromPage, "Sec-Fetch-Site": "same-site"}); got != http.StatusForbidden {
		t.Errorf("a write from the page on port %d to port %d = %d, want %d", page.Port, target.Port, got, http.StatusForbidden)
	}
	if got := post(map[string]string{"Origin": fromPage}); got != http.StatusForbidden {
		t.Errorf("a write naming the other port as its origin = %d, want %d", got, http.StatusForbidden)
	}
	// Its own page is let through, so the refusals above are the boundary and
	// not a route that is missing.
	own := strings.TrimSuffix(addressOfEntry(&target), "/")
	if got := post(map[string]string{"Origin": own, "Sec-Fetch-Site": "same-origin"}); got != http.StatusNoContent {
		t.Errorf("a write from the vault's own page = %d, want %d", got, http.StatusNoContent)
	}
}

// A vault whose port something else took while yomihon was not running keeps
// the port in the list: its address is the one the reader's browser has. It is
// told, and left; the vaults that can be served still are.
func TestAVaultWhosePortSomethingElseTookStaysOnItAndTheOthersAreServed(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	start := someFreePort(t)
	pre := listOf(t, configDir)
	if _, _, err := pre.Add(newVaultFolder(t), start); err != nil {
		t.Fatal(err)
	}
	squatted, _, err := pre.Add(newVaultFolder(t), start)
	if err != nil {
		t.Fatal(err)
	}
	var lc net.ListenConfig
	squatter, err := lc.Listen(t.Context(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(squatted.Port)))
	if err != nil {
		t.Skipf("the port is not free to take: %v", err)
	}
	t.Cleanup(func() { _ = squatter.Close() }) //nolint:errcheck // the port is released either way

	r := startRunning(t, configDir, newVaultFolder(t), start+10)
	r.address(t)
	if r.mountOf(squatted.ID) != nil {
		t.Errorf("the vault is served though something else holds its port")
	}
	if kept := idOf(t, listOf(t, configDir), squatted.Path); kept.Port != squatted.Port {
		t.Errorf("the list gives the vault %d, was %d; a listed vault does not move", kept.Port, squatted.Port)
	}
}

// One entry of the list that cannot be used does not take the vaults beside it
// off their ports: they are the addresses the reader's browser has. It is told
// in a line on stderr, and the list as it was is kept beside the new one.
func TestOneEntryThatCannotBeUsedLeavesTheOtherVaultsOnTheirPortsAndIsSaid(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	start := someFreePort(t)
	pre := listOf(t, configDir)
	first, _, err := pre.Add(newVaultFolder(t), start)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := pre.Add(newVaultFolder(t), start)
	if err != nil {
		t.Fatal(err)
	}
	// An entry at the top of the list that is not one: the root of a filesystem,
	// on the port the first vault would take if the others were dropped.
	file := filepath.Join(configDir, "yomihon", "vaults.json")
	raw, err := os.ReadFile(file) // #nosec G304 -- a file under t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	spliced := strings.Replace(string(raw), `"vaults": [`, `"vaults": [{"id":"x","path":"/","name":"root","last_read":"2026-11-18T09:00:00Z","port":`+strconv.Itoa(start-1)+`},`, 1)
	if spliced == string(raw) {
		t.Fatalf("the entry was not put in: %s", raw)
	}
	if err = os.WriteFile(file, []byte(spliced), 0o600); err != nil { // #nosec G703 -- a file under t.TempDir
		t.Fatal(err)
	}

	logged, out := new(lockedBuffer), new(lockedBuffer)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	typed := newVaultFolder(t)
	cfg := &config{root: typed, port: strconv.Itoa(start + 20), configDir: configDir}
	go func() {
		done <- serveVault(ctx, func() {}, slog.New(slog.NewTextHandler(logged, nil)), cfg, out)
	}()
	t.Cleanup(func() {
		cancel()
		if endErr := <-done; endErr != nil {
			t.Logf("the host ended with: %v", endErr)
		}
		if t.Failed() {
			t.Logf("the log:\n%s", logged.String())
		}
	})
	eventually(t, "the address to be printed", func() bool { return printedAddress.MatchString(out.String()) })

	for _, kept := range []vaults.Entry{first, second} {
		if status, _, _ := httpGet(t, addressOfEntry(&kept), "/"); status != http.StatusOK && status != http.StatusServiceUnavailable {
			t.Errorf("%s answers %d at %d", kept.Path, status, kept.Port)
		}
		if now := idOf(t, listOf(t, configDir), kept.Path); now.Port != kept.Port {
			t.Errorf("%s is on %d, was on %d: a listed vault does not move", kept.Path, now.Port, kept.Port)
		}
	}
	if !strings.Contains(logged.String(), "1 entry that could not be used") || !strings.Contains(logged.String(), "vaults.damaged.json") {
		t.Errorf("the log does not say what was left out and where the list is kept:\n%s", logged.String())
	}
	if kept, readErr := os.ReadFile(filepath.Join(configDir, "yomihon", "vaults.damaged.json")); readErr != nil || string(kept) != spliced { // #nosec G304 -- a file under t.TempDir
		t.Errorf("the list as it was is not kept: %v", readErr)
	}
}

// The vault just typed is the one the reader asked for, so its port being
// held is an error they can correct, said with the way to correct it.
func TestTheTypedVaultsPortBeingHeldIsRefusedWithWhatToDo(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	folder := newVaultFolder(t)
	taken := someFreePort(t)
	if _, _, err := listOf(t, configDir).Add(folder, taken); err != nil {
		t.Fatal(err)
	}
	var lc net.ListenConfig
	squatter, err := lc.Listen(t.Context(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(taken)))
	if err != nil {
		t.Skipf("the port is not free to take: %v", err)
	}
	t.Cleanup(func() { _ = squatter.Close() }) //nolint:errcheck // the port is released either way

	h := newHost(slog.New(slog.DiscardHandler), configDir, listOf(t, configDir))
	cfg := &config{root: folder, port: strconv.Itoa(someFreePort(t)), configDir: configDir}
	err = h.serveListed(t.Context(), func() {}, cfg, io.Discard)
	fault := operatorFault(err)
	if fault == nil || !strings.Contains(fault.Error(), strconv.Itoa(taken)) || !strings.Contains(fault.Error(), "yomihon vaults remove") {
		t.Fatalf("serveListed = %v, want a refusal naming port %d and the command that frees it", err, taken)
	}
}

func TestAHostThatIsEndingMountsNothingMore(t *testing.T) {
	t.Parallel()
	h := newHost(slog.New(slog.DiscardHandler), t.TempDir(), nil)
	if err := h.stopServing(); err != nil {
		t.Fatal(err)
	}
	entry := vaults.Entry{ID: strings.Repeat("a", 32), Path: t.TempDir(), Port: someFreePort(t)}
	if err := h.mount(t.Context(), &entry, nil, false); !errors.Is(err, errClosing) {
		t.Fatalf("mount after stopServing = %v, want errClosing", err)
	}
	if !portFree(t.Context(), entry.Port) {
		t.Errorf("the port of a vault that was not mounted is held")
	}
}
