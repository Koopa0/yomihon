package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"os"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/mark/vaults"
	"github.com/koopa0/yomihon/internal/shell"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/vault"
)

const (
	// pollEvery is how often a running yomihon looks at the list of vaults for
	// one the command line added.
	pollEvery = 500 * time.Millisecond

	// relistEvery is how many polls pass between lists made whether or not the
	// file looks changed. A change made in the instant this process wrote the
	// list itself is stamped as its own and not seen, and this closes that.
	relistEvery = 10

	// handOffWait is how long `yomihon <folder>` waits for the running yomihon
	// to take a folder it was given.
	handOffWait = 3 * time.Second

	// handOffPoll is how often the wait for it looks again.
	handOffPoll = 50 * time.Millisecond
)

// host serves every vault it has been given, each on its own loopback port.
// One running yomihon is the one process a second `yomihon <folder>` hands its
// folder to, so it keeps listening for vaults the command line adds.
//
// A vault is bound as soon as it is known, so its address answers at once with
// the page that says it is being read, and is read when somebody first asks for
// it: twenty listed vaults are not twenty scanners from the first second.
type host struct {
	log       *slog.Logger
	configDir string
	// store is the list of vaults, or nil where nothing is remembered: with no
	// configuration directory, or with a port left to the system. Then there is
	// one vault, the one given, and no list to watch.
	store *vaults.Store

	mu      sync.Mutex
	mounts  map[string]*mount
	closing bool

	// wait is how long a folder given to the running yomihon is waited for.
	wait time.Duration

	reading sync.WaitGroup
	serving sync.WaitGroup
	// fatal carries the failure of a read that has to succeed for the process
	// to be worth running: the vault the reader just typed.
	fatal chan error
}

// mount is one vault being served: the port it answers on, the front that
// answers until it is read, and the site that answers after.
type mount struct {
	entry    vaults.Entry
	listener net.Listener
	front    *startup
	progress *snapshot.Progress
	site     atomic.Pointer[readingSite]
	served   chan error

	touching    atomic.Bool
	touchWarned atomic.Bool
}

func newHost(log *slog.Logger, configDir string, store *vaults.Store) *host {
	return &host{
		log:       log,
		configDir: configDir,
		store:     store,
		mounts:    map[string]*mount{},
		wait:      handOffWait,
		fatal:     make(chan error, 1),
	}
}

// mount answers for entry on its port, binding the port unless the caller
// already has. The read of the vault starts at the first request, or now when
// eager, which is for the vault the reader just typed. Its failure is then the
// process's.
func (h *host) mount(ctx context.Context, entry *vaults.Entry, listener net.Listener, eager bool) error {
	if listener == nil {
		var err error
		if listener, err = listenLoopback(ctx, strconv.Itoa(entry.Port)); err != nil {
			return err
		}
	}
	m := &mount{entry: *entry, listener: listener, progress: new(snapshot.Progress), served: make(chan error, 1)}
	m.front = newStartup(shell.VaultName(entry.Path), entry.Path, m.progress, scanGrace, h.log)
	m.front.whenAsked(func() { h.beginRead(ctx, m, eager) })
	srv := newHTTPServer(h.touching(m, m.front))

	h.mu.Lock()
	if h.closing {
		h.mu.Unlock()
		return errors.Join(errClosing, listener.Close())
	}
	h.mounts[entry.ID] = m
	h.serving.Go(func() { h.serve(ctx, m, srv) })
	h.mu.Unlock()
	h.log.Info("yomihon serving", "addr", listener.Addr().String(), "vault", entry.Path)
	if eager {
		m.front.start()
	}
	return nil
}

// errClosing is the answer to a vault offered to a host that is ending.
var errClosing = errors.New("yomihon is stopping")

// serve answers on the mount's port until the host is stopped. A listener that
// fails on its own is the process's failure, and is told once, as the cause.
func (h *host) serve(ctx context.Context, m *mount, srv *http.Server) {
	err := serveHTTP(ctx, srv, m.listener, shutdownGrace(srv))
	if err != nil && ctx.Err() == nil {
		h.fail(err)
		err = nil
	}
	m.served <- err
}

// fail ends the process with err, the first one told.
func (h *host) fail(err error) {
	select {
	case h.fatal <- err:
	default:
	}
}

// beginRead starts reading a vault, unless the host is already ending, which
// has to be settled under the lock that shutdown takes before it waits.
func (h *host) beginRead(ctx context.Context, m *mount, mustSucceed bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closing {
		return
	}
	h.reading.Go(func() { h.read(ctx, m, mustSucceed) })
}

// read opens the vault and builds its reading site, reporting into the progress
// its page shows.
func (h *host) read(ctx context.Context, m *mount, mustSucceed bool) {
	source, err := vault.Open(m.entry.Path)
	if err != nil {
		h.failed(m, mustSucceed, fmt.Errorf("open vault source: %w", err))
		return
	}
	site, err := composeReadingSite(ctx, source, h.configDir, h.log, m.progress)
	if err != nil {
		// An interrupt while the vault is still being read is the reader
		// stopping, not a failure; the address is already on their screen, so
		// this is the likeliest moment for one.
		if ctx.Err() != nil && errors.Is(err, context.Canceled) {
			h.log.Info("yomihon stopped before the vault was read")
			return
		}
		h.failed(m, mustSucceed, fmt.Errorf("build reading site: %w", err))
		return
	}
	m.site.Store(site)
	m.front.open(site)
	h.touch(m)
}

func (h *host) failed(m *mount, mustSucceed bool, err error) {
	h.log.Error("vault could not be read", "vault", m.entry.Path, "error", err)
	m.front.fail(err)
	if mustSucceed {
		h.fail(err)
	}
}

// touching says to the list that the reader loaded a page of the vault. Only a
// page the browser navigated to counts: a script's fetch, a stylesheet and a
// request from a program are not the reader reading.
func (h *host) touching(m *mount, next http.Handler) http.Handler {
	if h.store == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.Header.Get("Sec-Fetch-Dest") == "document" {
			h.touch(m)
		}
		next.ServeHTTP(w, r)
	})
}

// touch records the read, and what the vault is called now, without making the
// page wait for the list's lock: at most one record is being made at a time, and
// one that finds the vault no longer listed has nothing to say, because a vault
// the reader removed goes on being served until the process ends.
func (h *host) touch(m *mount) {
	if h.store == nil || !m.touching.CompareAndSwap(false, true) {
		return
	}
	name := ""
	if site := m.site.Load(); site != nil {
		name = site.vaultTitle()
	}
	go func() {
		defer m.touching.Store(false)
		if _, err := h.store.Touch(m.entry.ID, name, time.Now()); err != nil && !errors.Is(err, vaults.ErrUnknown) && m.touchWarned.CompareAndSwap(false, true) {
			h.log.Warn("could not record the read in the list of vaults", "vault", m.entry.Path, "error", err)
		}
	}()
}

// watch looks for vaults the command line has added until ctx is done.
func (h *host) watch(ctx context.Context) {
	ticker := time.NewTicker(pollEvery)
	defer ticker.Stop()
	for tick := 1; ; tick++ {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.sync(ctx, tick%relistEvery == 0)
		}
	}
}

// sync mounts every listed vault it is not serving yet. The list is read only
// when its file has changed, or when force says to read it regardless.
func (h *host) sync(ctx context.Context, force bool) {
	changed, err := h.store.Changed()
	if err != nil {
		h.log.Warn("could not look at the list of vaults", "error", err)
		return
	}
	if !changed && !force {
		return
	}
	list, err := h.store.List()
	if err != nil {
		h.log.Warn("could not read the list of vaults", "error", err)
		return
	}
	for i := range list {
		h.ensure(ctx, &list[i])
	}
}

// ensure serves the vault if it is not served. A vault the reader removed and
// then added again is still being served, on the port it had, and the list is
// made to say so rather than serving it twice. A port something else has taken
// since the vault was last served is exchanged for the next free one, and the
// new port is written down.
func (h *host) ensure(ctx context.Context, entry *vaults.Entry) {
	h.mu.Lock()
	have, ok := h.mounts[entry.ID]
	closing := h.closing
	h.mu.Unlock()
	if closing {
		return
	}
	if ok {
		if have.entry.Port != entry.Port {
			if err := h.store.SetPort(entry.ID, have.entry.Port); err != nil {
				h.log.Warn("could not keep the vault on its port", "vault", entry.Path, "port", have.entry.Port, "error", err)
			}
		}
		return
	}
	if err := h.mount(ctx, entry, nil, false); err != nil {
		moved, rebindErr := h.store.Rebind(entry.ID)
		if rebindErr != nil {
			h.log.Warn("could not serve the vault", "vault", entry.Path, "port", entry.Port, "error", errors.Join(err, rebindErr))
			return
		}
		if err = h.mount(ctx, &moved, nil, false); err != nil {
			h.log.Warn("could not serve the vault", "vault", entry.Path, "port", moved.Port, "error", err)
		}
	}
}

// stopServing waits for every listener to finish serving and for every read to
// end. Nothing can be mounted or started after it begins.
func (h *host) stopServing() error {
	h.mu.Lock()
	h.closing = true
	mounts := slices.Collect(maps.Values(h.mounts))
	h.mu.Unlock()

	var result error
	for _, m := range mounts {
		result = errors.Join(result, <-m.served)
	}
	h.serving.Wait()
	h.reading.Wait()
	return result
}

// closeSites closes every site. The wait that closing one makes has no
// deadline, deliberately, so work already accepted is not abandoned.
func (h *host) closeSites() error {
	h.mu.Lock()
	mounts := slices.Collect(maps.Values(h.mounts))
	h.mu.Unlock()

	var result error
	for _, m := range mounts {
		if site := m.site.Load(); site != nil {
			result = errors.Join(result, site.close())
		}
	}
	return result
}

// await holds until the process is told to stop or a failure has to end it, and
// then shuts down. The wait in closeSites is announced, which turns an apparent
// hang into a visible state, which is what the reader who pressed Ctrl-C is
// owed. While the notify handler is installed that wait would also swallow a
// repeated interrupt; restoring default signal delivery the moment serving ends
// lets a second Ctrl-C end a stuck shutdown.
func (h *host) await(ctx context.Context, cancel, stop func()) error {
	var cause error
	select {
	case <-ctx.Done():
	case cause = <-h.fatal:
	}
	cancel()
	result := errors.Join(cause, h.stopServing())
	h.log.Info("yomihon no longer serving; finishing work already accepted")
	stop()
	result = errors.Join(result, h.closeSites())
	if result == nil {
		h.log.Info("yomihon stopped")
	}
	return result
}

// serveOne is a process that serves the one vault it was given and remembers
// nothing: no configuration directory, or a port left to the system.
func (h *host) serveOne(ctx context.Context, stop func(), cfg *config, out io.Writer) error {
	listener, err := listenLoopback(ctx, cfg.port)
	if err != nil {
		return err
	}
	_, resolved, err := vault.ResolveRoot(cfg.root)
	if err != nil {
		return errors.Join(err, listener.Close())
	}
	entry := vaults.Entry{ID: confdir.VaultID(resolved), Path: resolved, Name: shell.VaultName(resolved)}
	if tcp, ok := listener.Addr().(*net.TCPAddr); ok {
		entry.Port = tcp.Port
	}

	serveCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := h.mount(serveCtx, &entry, listener, true); err != nil {
		return err
	}
	h.printAddress(out, listener)
	return h.await(serveCtx, cancel, stop)
}

// serveListed is the yomihon that serves the list. It is the one that holds the
// starting port; a yomihon that finds another holding it gives the folder to
// that one and ends. It serves every vault in the list, reads the one just
// given at once and prints its address, and listens for vaults the command line
// adds.
func (h *host) serveListed(ctx context.Context, stop func(), cfg *config, out io.Writer) error {
	start := cfg.startPort()
	if running, ok := h.running(ctx); ok {
		return h.giveFolder(ctx, cfg.root, running.Port, out)
	}
	listener, err := listenLoopback(ctx, cfg.port)
	if err != nil {
		// A yomihon that started a moment ago holds the port before it has said
		// so, and the same moment shows it.
		if running, ok := h.waitRunning(ctx); ok {
			return h.giveFolder(ctx, cfg.root, running.Port, out)
		}
		return err
	}
	serveCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	typed, err := h.record(cfg.root, start)
	if err != nil {
		return errors.Join(err, listener.Close())
	}
	if err = h.store.WriteInstance(vaults.Instance{PID: os.Getpid(), Port: start, Started: time.Now().UTC()}); err != nil {
		h.log.Warn("could not record that yomihon is running; a second yomihon will not find this one", "error", err)
	}
	defer func() {
		if rmErr := h.store.RemoveInstance(os.Getpid()); rmErr != nil {
			h.log.Warn("could not remove the record that yomihon is running", "error", rmErr)
		}
	}()

	list, err := h.store.List()
	if err != nil {
		h.log.Warn("could not read the list of vaults; serving the one just given", "error", err)
		list = []vaults.Entry{typed}
	}
	held, err := h.mountAll(serveCtx, list, typed.ID, start, listener)
	if err != nil {
		cancel()
		return errors.Join(err, h.stopServing(), h.closeSites())
	}
	if !held {
		// No vault is on the starting port, which is a list that could not be
		// read or written. The port is still held, so a second yomihon finds
		// this one rather than one that is not there.
		h.log.Warn("no vault is on the starting port", "port", start)
		h.serving.Go(func() { h.holdPort(serveCtx, listener) })
	}
	if m := h.find(typed.ID); m != nil {
		h.printAddress(out, m.listener)
	}
	h.serving.Go(func() { h.watch(serveCtx) })
	return h.await(serveCtx, cancel, stop)
}

// mountAll serves every vault of the list, the one just given first so that its
// address is the first the log names. The starting port's listener goes to the
// vault that is on that port, and held says whether one was. Only the vault
// just given failing is an error: another that cannot be served is told and
// left, so one folder that is gone does not stop the rest.
func (h *host) mountAll(ctx context.Context, list []vaults.Entry, typedID string, start int, listener net.Listener) (held bool, err error) {
	if i := slices.IndexFunc(list, func(e vaults.Entry) bool { return e.ID == typedID }); i > 0 {
		list[0], list[i] = list[i], list[0]
	}
	for i := range list {
		var bound net.Listener
		if list[i].Port == start && !held {
			bound, held = listener, true
		}
		if err = h.mountListed(ctx, &list[i], bound, list[i].ID == typedID); err != nil {
			if list[i].ID == typedID {
				return held, err
			}
			h.log.Warn("could not serve the vault", "vault", list[i].Path, "port", list[i].Port, "error", err)
		}
	}
	return held, nil
}

// record puts the folder in the list and makes sure a vault is on the starting
// port, which the running yomihon holds. It answers the folder's entry as it is
// afterwards.
func (h *host) record(root string, start int) (vaults.Entry, error) {
	_, resolved, err := vault.ResolveRoot(root)
	if err != nil {
		return vaults.Entry{}, err
	}
	typed, _, err := h.store.Add(resolved, start)
	if err != nil {
		return vaults.Entry{}, fmt.Errorf("add the folder to the list of vaults: %w", err)
	}
	moved, ok, err := h.store.Anchor(start)
	if err != nil {
		return vaults.Entry{}, fmt.Errorf("put a vault on the starting port: %w", err)
	}
	if ok {
		if moved.ID == typed.ID {
			typed = moved
		} else {
			h.log.Info("the first vault moved to the starting port", "vault", moved.Path, "port", moved.Port)
		}
	}
	return typed, nil
}

// holdPort keeps the starting port bound, answering nothing, until ctx is done.
func (h *host) holdPort(ctx context.Context, listener net.Listener) {
	<-ctx.Done()
	if err := listener.Close(); err != nil {
		h.log.Warn("could not close the starting port", "error", err)
	}
}

// running is the yomihon that is already serving, if one is. Its record names a
// process that is alive and a port that is held: a process id the system has
// handed to another program does not make a running yomihon of it.
func (h *host) running(ctx context.Context) (vaults.Instance, bool) {
	in, ok := h.store.Running()
	if !ok || portFree(ctx, in.Port) {
		return vaults.Instance{}, false
	}
	return in, true
}

// waitRunning waits a moment for a yomihon that has the port but has not yet
// written its record.
func (h *host) waitRunning(ctx context.Context) (vaults.Instance, bool) {
	deadline := time.Now().Add(h.wait)
	for time.Now().Before(deadline) {
		if in, ok := h.running(ctx); ok {
			return in, true
		}
		select {
		case <-ctx.Done():
			return vaults.Instance{}, false
		case <-time.After(handOffPoll):
		}
	}
	return vaults.Instance{}, false
}

// giveFolder adds the folder to the list and waits for the running yomihon to
// serve it, printing the address that does.
func (h *host) giveFolder(ctx context.Context, root string, start int, out io.Writer) error {
	_, resolved, err := vault.ResolveRoot(root)
	if err != nil {
		return err
	}
	entry, _, err := h.store.Add(resolved, start)
	if err != nil {
		return fmt.Errorf("add the folder to the list of vaults: %w", err)
	}
	return h.handOff(ctx, entry.ID, out)
}

// handOff waits for the running yomihon to take the vault, and prints its
// address. The running yomihon holds a vault's port exactly when it has taken
// it, so the wait is for the port to stop being free, and no connection is made
// to find out: this process opens none.
func (h *host) handOff(ctx context.Context, id string, out io.Writer) error {
	deadline := time.Now().Add(h.wait)
	for {
		list, err := h.store.List()
		if err != nil {
			return fmt.Errorf("read the list of vaults: %w", err)
		}
		if i := slices.IndexFunc(list, func(e vaults.Entry) bool { return e.ID == id }); i >= 0 && !portFree(ctx, list[i].Port) {
			_, err = fmt.Fprintf(out, "http://127.0.0.1:%d/\n", list[i].Port)
			return err
		}
		if !time.Now().Before(deadline) {
			return errNotTaken
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(handOffPoll):
		}
	}
}

// errNotTaken is the answer when the running yomihon did not serve a folder it
// was given in the time it has.
var errNotTaken = errors.New("the running yomihon has not taken the folder")

// mountListed serves one vault of the list, moving it to another port if
// something else holds its own.
func (h *host) mountListed(ctx context.Context, entry *vaults.Entry, listener net.Listener, eager bool) error {
	err := h.mount(ctx, entry, listener, eager)
	if err == nil || listener != nil {
		return err
	}
	moved, rebindErr := h.store.Rebind(entry.ID)
	if rebindErr != nil {
		return errors.Join(err, rebindErr)
	}
	*entry = moved
	return h.mount(ctx, entry, nil, eager)
}

func (h *host) find(id string) *mount {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.mounts[id]
}

func (h *host) printAddress(out io.Writer, listener net.Listener) {
	if _, err := fmt.Fprintf(out, "http://%s/\n", listener.Addr()); err != nil {
		h.log.Warn("could not print the address", "error", err)
	}
}

// vaultTitle is what the vault is called now: its README's title, else its
// folder.
func (site *readingSite) vaultTitle() string {
	return site.snapshots.Current().Capture().VaultTitle(shell.VaultName(site.source.Name()))
}
