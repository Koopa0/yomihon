package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/koopa0/yomihon/internal/asset"
	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/report"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/shell"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/syllabus"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
)

// readingSite is the production HTTP composition and the snapshot scanner it
// serves. Keeping listener lifecycle out of this type lets acceptance tests
// drive the exact production handler without opening the command's port.
type readingSite struct {
	handler   http.Handler
	snapshots *snapshot.Store
	lifecycle *status.Lifecycle
	source    *vault.Reader
	cancel    context.CancelFunc
	watchers  sync.WaitGroup
	requestMu sync.Mutex
	closing   bool
	requests  sync.WaitGroup
	closeOnce sync.Once
	closeErr  error
}

func newReadingSite(ctx context.Context, root string, log *slog.Logger) (_ *readingSite, resultErr error) {
	source, err := vault.Open(root)
	if err != nil {
		return nil, fmt.Errorf("open vault source: %w", err)
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, source.Close())
		}
	}()

	contract, err := schema.LoadReader(ctx, source)
	var governance schema.Governance
	switch {
	case schema.ContractAbsent(err):
		// A folder that carries no contract is not a folder in trouble. It has
		// no lifecycle to govern, which is the ordinary shape of any directory
		// yomihon is pointed at, so it is reported as the fact it is: reading
		// and search are whole, the write face has nothing to write against.
		log.Info("no vault contract; reading and search only, no write face",
			"path", schema.ContractRelPath)
		contract = nil
		governance = schema.Ungoverned()
	case err != nil:
		// A contract that exists and cannot be read is the opposite case: this
		// folder claimed governance and then failed to deliver it. Carrying that
		// distinction past startup is what keeps the two folders from rendering
		// the same page.
		log.Warn("vault contract unavailable; write face is closed (fail-closed)", "error", err)
		contract = nil
		governance = schema.Unreadable(err)
	default:
		log.Info("vault contract loaded",
			"version", contract.Version(), "lifecycle_stages", contract.StageCount())
		governance = contract.Governance()
	}

	lifecycle, err := status.Open(source, contract, governance, log)
	if err != nil {
		return nil, fmt.Errorf("open status lifecycle: %w", err)
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, lifecycle.Close())
		}
	}()
	store, err := snapshot.New(ctx, source, log, contract, governance)
	if err != nil {
		return nil, err
	}
	projectShell := func(statusView status.View, snap *snapshot.View) pages.Shell {
		return shell.Project(statusView, snap.ArtifactPolicy(), snap)
	}
	shellForSnapshot := func(snap *snapshot.View) pages.Shell {
		return projectShell(lifecycle.View(), snap)
	}
	shellProvider := func() pages.Shell {
		statusView := lifecycle.View()
		return projectShell(statusView, store.Current().Capture())
	}
	searchProvider := func() search.RequestSnapshot {
		statusView := lifecycle.View()
		snap := store.Current().Capture()
		return search.RequestSnapshot{Index: snap.Search(), Shell: projectShell(statusView, snap), Status: statusView}
	}

	mux := http.NewServeMux()
	note.New(&note.Dependencies{
		Source:         source,
		Status:         lifecycle.View,
		Snapshot:       store.Current,
		ObservedStatus: lifecycle.ObservedStatus,
		Log:            log,
	}).Register(mux)
	status.NewHandler(lifecycle, shellProvider, log).Register(mux)
	search.NewHandler(searchProvider, log).Register(mux)
	syllabus.New(shellProvider, log).Register(mux)
	report.New(source, store.Current, shellForSnapshot, log).Register(mux)
	asset.Register(mux)

	handler := http.NewCrossOriginProtection().Handler(mux)
	watchCtx, cancel := context.WithCancel(ctx)
	site := &readingSite{
		handler:   origin.LoopbackOnly(origin.Protect(handler)),
		snapshots: store,
		lifecycle: lifecycle,
		source:    source,
		cancel:    cancel,
	}
	site.watchers.Go(func() { site.snapshots.Run(watchCtx) })
	return site, nil
}

func (site *readingSite) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	site.requestMu.Lock()
	if site.closing {
		site.requestMu.Unlock()
		http.Error(w, "server is shutting down", http.StatusServiceUnavailable)
		return
	}
	site.requests.Add(1)
	site.requestMu.Unlock()
	defer site.requests.Done()

	site.handler.ServeHTTP(w, r)
}

func (site *readingSite) close() error {
	if site == nil {
		return nil
	}
	site.closeOnce.Do(func() {
		// Closing the admission gate before Wait prevents a concurrent Add from
		// racing with Wait when no request has entered yet. Requests already in
		// flight keep their rooted capabilities until their handlers return,
		// even when http.Server.Shutdown has reached its deadline.
		site.requestMu.Lock()
		site.closing = true
		site.requestMu.Unlock()
		if site.cancel != nil {
			site.cancel()
		}
		site.watchers.Wait()
		site.requests.Wait()
		if site.lifecycle != nil {
			site.closeErr = errors.Join(site.closeErr, site.lifecycle.Close())
		}
		if site.source != nil {
			site.closeErr = errors.Join(site.closeErr, site.source.Close())
		}
	})
	return site.closeErr
}
