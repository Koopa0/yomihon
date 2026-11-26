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

	"github.com/koopa0/yomihon/internal/egress"
	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/mark/vaults"
	"github.com/koopa0/yomihon/internal/origin"
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
	configDir confdir.ConfigDir
	// store is the list of vaults, or nil where nothing is remembered: with no
	// configuration directory, or with a port left to the system. Then there is
	// one vault, the one given, and no list to watch.
	store *vaults.Store

	mu      sync.Mutex
	mounts  map[string]*mount
	closing bool
	// toldApart is the listed vaults already said to overlap the
	// configuration directory.
	toldApart map[string]bool

	// wait is how long a folder given to the running yomihon is waited for.
	wait time.Duration

	reading sync.WaitGroup
	serving sync.WaitGroup
	// recording is the records of a read that are being written to the list.
	recording sync.WaitGroup
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

func newHost(log *slog.Logger, configDir confdir.ConfigDir, store *vaults.Store) *host {
	return &host{
		log:       log,
		configDir: configDir,
		store:     store,
		mounts:    map[string]*mount{},
		toldApart: map[string]bool{},
		wait:      handOffWait,
		fatal:     make(chan error, 1),
	}
}

// mount answers for entry on its port, binding the port unless the caller
// already has. The read of the vault starts at the first request, or now when
// eager, which is for the vault the reader just typed. Its failure is then the
// process's.
func (h *host) mount(ctx context.Context, entry *vaults.Entry, listener net.Listener, eager bool) error {
	// Checked before the port is bound: a vault listed before yomihon refused
	// such a folder is not served, and stays listed for `yomihon vaults remove`.
	if overlap := h.overlaps(entry); overlap != nil {
		if listener != nil {
			return errors.Join(overlap, listener.Close())
		}
		return overlap
	}
	if listener == nil {
		var err error
		if listener, err = listenLoopback(ctx, strconv.Itoa(entry.Port)); err != nil {
			return &refusal{cause: err, msg: fmt.Sprintf("%s is on port %d, which something else holds (%v); stop that, or take the vault out of the list with `yomihon vaults remove` and open its folder again, which gives it another port",
				vault.Tilde(entry.Path), entry.Port, errors.Unwrap(err))}
		}
	}
	m := &mount{entry: *entry, listener: listener, progress: new(snapshot.Progress), served: make(chan error, 1)}
	m.front = newStartup(shell.VaultName(entry.Path), vault.Tilde(entry.Path), m.progress, scanGrace, h.log)
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

// touchTimeout bounds reading the rules file for the name the list keeps.
const touchTimeout = 10 * time.Second

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
	h.touch(ctx, m)
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
		if r.Method == http.MethodGet && r.Header.Get("Sec-Fetch-Dest") == "document" && origin.LoopbackRequest(r) {
			h.touch(r.Context(), m)
		}
		next.ServeHTTP(w, r)
	})
}

// touch records the read, and what the vault is called now, without making the
// page wait for the list's lock: at most one record is being made at a time, and
// one that finds the vault no longer listed has nothing to say, because a vault
// the reader removed goes on being served until the process ends.
func (h *host) touch(ctx context.Context, m *mount) {
	if h.store == nil || !m.touching.CompareAndSwap(false, true) {
		return
	}
	// The record outlives the request that asked for it, which may end first, and
	// not the host: the host waits for it before it ends, so nothing writes to the
	// list after that. A record asked for once the host is ending is dropped. The
	// group is added to under the lock stopServing closes under, so every record is
	// begun before the wait for them: an Add that raced the Wait would be a data
	// race, and Wait could return before the record it missed is written.
	ctx = context.WithoutCancel(ctx)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closing {
		m.touching.Store(false)
		return
	}
	h.recording.Go(func() {
		defer m.touching.Store(false)
		h.recordRead(ctx, m)
	})
}

// recordRead writes the read, and the name the list keeps, to the list.
func (h *host) recordRead(ctx context.Context, m *mount) {
	name := ""
	if site := m.site.Load(); site != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, touchTimeout)
		defer cancel()
		name = site.listName(ctx)
	}
	if _, err := h.store.Touch(m.entry.ID, name, time.Now()); err != nil && !errors.Is(err, vaults.ErrUnknown) && m.touchWarned.CompareAndSwap(false, true) {
		h.log.Warn("could not record the read in the list of vaults", "vault", m.entry.Path, "error", err)
	}
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
// made to say so rather than serving it twice. A vault whose port something
// else holds is told and left where it is: its address is the one the reader's
// browser has, and it is not given another.
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
		h.notServed(entry, err)
	}
}

// overlaps is why the vault is not to be served because it and the
// configuration directory are one inside the other, or nil. Anything else that
// stops the comparison, such as a folder that is gone, is for the read of the
// vault to say, as it was before the comparison was made.
func (h *host) overlaps(entry *vaults.Entry) error {
	err := h.configDir.Apart(entry.Path)
	if _, overlaps := errors.AsType[*confdir.OverlapError](err); overlaps {
		return err
	}
	return nil
}

// notServed says why a listed vault is not served. A vault that overlaps the
// configuration directory is said once, since the list is read again every few
// seconds and the answer does not change while it is listed.
func (h *host) notServed(entry *vaults.Entry, err error) {
	if _, overlaps := errors.AsType[*confdir.OverlapError](err); overlaps {
		h.mu.Lock()
		told := h.toldApart[entry.ID]
		h.toldApart[entry.ID] = true
		h.mu.Unlock()
		if !told {
			h.log.Warn("not serving a vault in the list; it stays listed until `yomihon vaults remove` takes it out", "vault", entry.Path, "reason", err.Error())
		}
		return
	}
	h.log.Warn("could not serve the vault", "vault", entry.Path, "port", entry.Port, "error", err)
}

// stopServing waits for every listener to finish serving and for every read to
// end. Nothing can be mounted, started or recorded after it begins; the records
// already begun are waited for by endRecords.
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

// endRecords waits for every record of a read that stopServing let begin to be
// written. A record waits up to the list's lock timeout behind another process,
// so the wait follows the announcement and the return of a second interrupt to the
// process, and not the shutdown of the listeners.
func (h *host) endRecords() { h.recording.Wait() }

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
	h.endRecords()
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
	if start < vaults.MinPort {
		return &refusal{msg: fmt.Sprintf("YOMIHON_PORT %d is one of the system's ports; to keep a list of vaults it must be 0, or an integer from %d to 65535", start, vaults.MinPort)}
	}
	if running, ok := h.running(ctx); ok {
		return h.giveFolder(ctx, cfg, running, out)
	}
	listener, err := listenLoopback(ctx, cfg.port)
	if err != nil {
		// A yomihon that started a moment ago holds the port before it has said
		// so, and the same moment shows it.
		if running, ok := h.waitRunning(ctx); ok {
			return h.giveFolder(ctx, cfg, running, out)
		}
		return err
	}
	// The port is bound before the folder is recorded, so that a second yomihon
	// started in the same moment finds this one, and the store is told it is ours
	// so that recording the folder does not take our own listener for another
	// program's.
	h.store.Hold(start)
	serveCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	typed, added, err := h.record(cfg.root, start)
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
		err = h.unlistIfNew(err, &typed, added)
		cancel()
		if !held {
			err = errors.Join(err, listener.Close())
		}
		err = errors.Join(err, h.stopServing())
		h.endRecords()
		return errors.Join(err, h.closeSites())
	}
	if !held {
		// The vault just given is listed on another port, and a listed vault is
		// never moved. The starting port is still held, which is how a second
		// yomihon finds this one, and a browser that opens it is sent to the vault.
		h.serving.Go(func() { h.redirectPort(serveCtx, listener, typed.Port) })
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
		// Asked before the starting port is handed over, which a vault that is
		// not served must not take with it.
		if overlap := h.overlaps(&list[i]); overlap != nil && list[i].ID != typedID {
			h.notServed(&list[i], overlap)
			continue
		}
		var bound net.Listener
		if list[i].Port == start && !held {
			bound, held = listener, true
		}
		if err = h.mount(ctx, &list[i], bound, list[i].ID == typedID); err != nil {
			if list[i].ID == typedID {
				return held, err
			}
			h.notServed(&list[i], err)
		}
	}
	return held, nil
}

// record puts the folder in the list and answers its entry. A vault already
// listed keeps its port; a new one is given the starting port if no vault is on
// it, and the next free one if one is.
func (h *host) record(root string, start int) (entry vaults.Entry, added bool, err error) {
	_, resolved, err := vault.ResolveRoot(root)
	if err != nil {
		return vaults.Entry{}, false, err
	}
	typed, added, err := h.store.Add(resolved, start)
	if err != nil {
		return vaults.Entry{}, false, addError(err)
	}
	return typed, added, nil
}

// addError is what a folder the list would not take is answered with: a refusal
// the reader can act on when the folder overlaps the configuration directory.
func addError(err error) error {
	if _, overlaps := errors.AsType[*confdir.OverlapError](err); overlaps {
		return &refusal{cause: err, msg: err.Error()}
	}
	return fmt.Errorf("add the folder to the list of vaults: %w", err)
}

// unlistIfNew takes the vault just given out of the list when err says its first
// bind failed and this start is what added it. It was never served there, so no
// address of it exists anywhere, and keeping it would send every start after this
// one to the same refusal; a vault already listed may have been served and keeps
// its port. mountAll answers only for the vault just given, so a failed bind in
// err is its own. err is returned as it was when the list cannot be changed, whose
// text says how to do it by hand.
func (h *host) unlistIfNew(err error, entry *vaults.Entry, added bool) error {
	bind, ok := errors.AsType[*listenError](err)
	if !ok || !added {
		return err
	}
	if _, rmErr := h.store.Remove(entry.ID); rmErr != nil {
		h.log.Warn("could not take the folder out of the list again", "folder", entry.Path, "error", rmErr)
		return err
	}
	return &refusal{cause: err, msg: fmt.Sprintf("%s was given port %d and could not bind it (%v); the folder is not kept in the list, so opening it again gives it another port", entry.Path, entry.Port, bind.err)}
}

// redirectPort answers the starting port, when no vault is on it, by sending a
// browser to the vault the reader just opened. It stands behind the same two
// edges as the vaults do, and it names nothing but that one loopback address.
func (h *host) redirectPort(ctx context.Context, listener net.Listener, port int) {
	target := fmt.Sprintf("http://127.0.0.1:%d/", port)
	srv := newHTTPServer(origin.Protect(origin.LoopbackOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
	}))))
	if err := serveHTTP(ctx, srv, listener, shutdownGrace(srv)); err != nil && ctx.Err() == nil {
		h.fail(err)
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
//
// It adds the folder only to hand it over, so a hand-over that does not happen
// takes the folder out of the list again, and says what to do. A record of a
// running yomihon is a process id and a port, and an id the system has given to
// another program makes one of that; this process opens no connection to ask
// the port who it is, so a record that names a live process and a held port
// cannot be told from a yomihon that is slow, until the wait has passed.
func (h *host) giveFolder(ctx context.Context, cfg *config, running vaults.Instance, out io.Writer) error {
	if cfg.portSet && running.Port != cfg.startPort() {
		return &refusal{msg: fmt.Sprintf("yomihon is already running on port %d (process %d) and serves every vault in the list, so YOMIHON_PORT=%d cannot start a second one; "+
			"open the folder without YOMIHON_PORT to give it to that one, or stop it first", running.Port, running.PID, cfg.startPort())}
	}
	_, resolved, err := vault.ResolveRoot(cfg.root)
	if err != nil {
		return err
	}
	entry, added, err := h.store.Add(resolved, running.Port)
	if err != nil {
		return addError(err)
	}
	err = h.handOff(ctx, entry.ID, out)
	if !errors.Is(err, errNotTaken) {
		return err
	}
	if added {
		if _, rmErr := h.store.Remove(entry.ID); rmErr != nil {
			h.log.Warn("could not take the folder out of the list again", "folder", resolved, "error", rmErr)
		}
	}
	return &refusal{msg: fmt.Sprintf("%s says yomihon is running (process %d, port %d) but it did not take the folder within %s, and the folder was not added to the list; "+
		"if yomihon is not running, remove that file and start again, and if it is, nothing on port %d is serving this list",
		h.store.InstanceFile(), running.PID, running.Port, h.wait, running.Port)}
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

// listName is what the list keeps the vault's name as: its README's title, else
// its folder's name. It is the one place that decides, for the list, which of the
// two, and the list is read back by `yomihon vaults` for a program. A README the
// rules file skips is no note and has no title to take (the snapshot says so),
// and one the rules file withholds is not said, so the folder's name stands in;
// so it does when the rules file cannot be honoured, since what it withholds is
// then not known.
func (site *readingSite) listName(ctx context.Context) string {
	folder := shell.VaultName(site.source.Name())
	title := site.snapshots.Current().Capture().DeclaredTitle(snapshot.ReadmeRelPath)
	if title == "" || !egress.Allowed(ctx, site.source, snapshot.ReadmeRelPath) {
		return folder
	}
	return title
}
