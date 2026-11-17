package main

import (
	"context"
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

	"github.com/koopa0/yomihon/internal/mark/vaults"
)

// nextBlock hands each test a block of ports of its own. The vaults a test adds
// take the ports above its starting one, and the list probes them by binding, so
// two tests whose starting ports sit side by side would take each other's.
var nextBlock atomic.Int32

const portsPerTest = 40

// someFreePort is the starting port for a test, with the forty above it left to
// that test alone. It is one nothing holds now.
func someFreePort(t *testing.T) int {
	t.Helper()
	for {
		port := 20000 + portsPerTest*int(nextBlock.Add(1))
		if port+portsPerTest > 65000 {
			t.Fatal("no block of ports is left for this test")
		}
		if portFree(t.Context(), port) {
			return port
		}
	}
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

func TestTheFirstVaultMovesToTheStartingPortWhenNoVaultIsOnIt(t *testing.T) {
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
	r.address(t)
	if moved := idOf(t, listOf(t, configDir), old); moved.Port != start {
		t.Errorf("the first vault is on %d, was on %d; want it on the starting port, %d", moved.Port, pre.Port, start)
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

func TestAVaultWhosePortSomethingElseTookIsMovedToAnotherAndTheListSaysSo(t *testing.T) {
	t.Parallel()
	configDir := t.TempDir()
	start := someFreePort(t)
	pre := listOf(t, configDir)
	if _, _, err := pre.Add(newVaultFolder(t), start); err != nil {
		t.Fatal(err)
	}
	moved, _, err := pre.Add(newVaultFolder(t), start)
	if err != nil {
		t.Fatal(err)
	}
	// Something else takes the port the list gave it, while yomihon is not
	// running.
	var lc net.ListenConfig
	squatter, err := lc.Listen(t.Context(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(moved.Port)))
	if err != nil {
		t.Skipf("the port is not free to take: %v", err)
	}
	t.Cleanup(func() { _ = squatter.Close() }) //nolint:errcheck // the port is released either way

	r := startRunning(t, configDir, newVaultFolder(t), start)
	r.address(t)
	if r.mountOf(moved.ID) == nil {
		t.Fatalf("the vault was not served on any port")
	}
	list, err := pre.List()
	if err != nil {
		t.Fatal(err)
	}
	for i := range list {
		if list[i].ID == moved.ID && list[i].Port == moved.Port {
			t.Errorf("the list still gives the vault the port something else holds, %d", moved.Port)
		}
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
