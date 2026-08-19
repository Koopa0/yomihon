package snapshot_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/vault"
)

// lockedSource is the vault with some of its files held shut. It answers scans
// normally — the files are all there, and their sizes and times are visible —
// and refuses to hand over the bytes of the paths named in locked, which is
// what a permission, a lock held by another program, and a failing disk all
// look like from here.
type lockedSource struct {
	*vault.Reader

	locked atomic.Pointer[[]string]
}

func (s *lockedSource) ReadFile(ctx context.Context, entry vault.Entry) ([]byte, error) {
	if paths := s.locked.Load(); paths != nil {
		for _, path := range *paths {
			if path == entry.Path() {
				return nil, errors.New("locked by another program")
			}
		}
	}
	return s.Reader.ReadFile(ctx, entry)
}

func (s *lockedSource) lock(paths ...string) { s.locked.Store(&paths) }

func writeVaultNote(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// governedVault puts the shared contract fixture in root and loads it, so the
// reading page carries its write face and the status a carried note shows can
// be told apart from the one on disk.
func governedVault(t *testing.T, root string) *schema.Contract {
	t.Helper()
	declaration, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read contract fixture: %v", err)
	}
	full := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	if mkdirErr := os.MkdirAll(filepath.Dir(full), 0o750); mkdirErr != nil {
		t.Fatalf("mkdir for the contract: %v", mkdirErr)
	}
	if writeErr := os.WriteFile(full, declaration, 0o600); writeErr != nil {
		t.Fatalf("write the contract: %v", writeErr)
	}
	contract, err := schema.Load(root)
	if err != nil {
		t.Fatalf("schema.Load: %v", err)
	}
	return contract
}

func getPage(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url) //nolint:noctx // the request is to this test's own server and outlives nothing
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close %s body: %v", url, closeErr)
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s body: %v", url, err)
	}
	return resp.StatusCode, string(body)
}

// TestDegradedGenerationTellsTheTruthOnEveryReadingSurface drives a folder
// through the state the retention bound creates — a generation published
// without sources it could not re-read — and reads back what a person actually
// sees. Every claim here is one the pages would otherwise make silently: home
// says the folder was not read whole, the health page names each source and
// when the folder was last seen entire, the carried note renders its last
// known words and says they are the last known ones, work written during the
// failure is readable, and a file the folder has never had a reading of still
// gets the page that sends the reader to clear its permission.
func TestDegradedGenerationTellsTheTruthOnEveryReadingSurface(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const (
		carried = "Concepts/Carried.md"
		fresh   = "Concepts/Fresh.md"
		never   = "Concepts/Never.md"
	)
	writeVaultNote(t, root, carried,
		"---\ntitle: Carried\ntype: concept\nstatus: seedling\n---\nthe words read before the file shut\n")
	writeVaultNote(t, root, "Concepts/Base.md", "---\ntitle: Base\ntype: concept\nstatus: seedling\n---\nbase\n")
	contract := governedVault(t, root)

	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close: %v", closeErr)
		}
	})
	source := &lockedSource{Reader: reader}

	log := slog.New(slog.DiscardHandler)
	store, err := snapshot.New(t.Context(), source, log, contract, contract.Governance())
	if err != nil {
		t.Fatalf("snapshot.New: %v", err)
	}
	lifecycle, err := status.Open(reader, contract, contract.Governance())
	if err != nil {
		t.Fatalf("status.Open: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := lifecycle.Close(); closeErr != nil {
			t.Errorf("Lifecycle.Close: %v", closeErr)
		}
	})
	mux := http.NewServeMux()
	note.New(&note.Dependencies{
		Source:   reader,
		Status:   lifecycle.View,
		Snapshot: store.Current,
		Provenance: func(context.Context, string, [sha256.Size]byte) (string, error) {
			return "", nil
		},
		WriteBlock:     lifecycle.WriteBlockReason,
		ObservedStatus: lifecycle.ObservedStatus,
		Log:            log,
	}).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	// The folder was read whole once. Now one note shuts, another note the
	// folder has never had a reading of appears already shut, and the reader
	// keeps working: a third note is written while neither can be read.
	whole := store.Current().Freshness().BuiltAt
	writeVaultNote(t, root, carried,
		"---\ntitle: Carried\ntype: concept\nstatus: growing\n---\nwords written after the file shut\n")
	writeVaultNote(t, root, never, "---\ntitle: Never\ntype: concept\nstatus: seedling\n---\nnever read\n")
	writeVaultNote(t, root, fresh,
		"---\ntitle: Fresh\ntype: concept\nstatus: seedling\n---\nwritten while the folder was degraded\n")
	source.lock(carried, never)

	base := time.Now()
	clock := base
	store.SetClock(func() time.Time { return clock })
	for tick := range 2*snapshot.DegradeAfter + 1 {
		clock = base.Add(time.Duration(tick) * 2 * time.Second)
		store.Rescan(t.Context())
	}
	if reported := store.Current().Freshness(); reported.Complete {
		t.Fatalf("the folder never degraded, so no surface below is under test: %+v", reported)
	}

	code, home := getPage(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", code, http.StatusOK)
	}
	if i := strings.Index(home, "讀不進來"); i < 0 {
		t.Error("home page presents a folder it could not read whole as if it had")
	} else if !strings.Contains(home[i:], carried) || !strings.Contains(home[i:], never) {
		t.Error("home notice does not name both sources the folder could not read")
	}

	code, health := getPage(t, srv.URL+"/health")
	if code != http.StatusOK {
		t.Fatalf("GET /health status = %d, want %d", code, http.StatusOK)
	}
	if i := strings.Index(health, "讀不進來的檔案"); i < 0 {
		t.Error("health page has no blocked-sources section for a degraded generation")
	} else if !strings.Contains(health[i:], carried) || !strings.Contains(health[i:], never) {
		t.Error("health blocked-sources section does not name both unreadable sources")
	}
	if !strings.Contains(health, "最後一次完整讀取是 "+whole.Format("2006-01-02 15:04")) {
		t.Error("health page does not say when the folder was last read whole")
	}
	if strings.Contains(health, "啟動到現在還沒有一次完整的讀取") {
		t.Error("health page says the folder has never been read whole, which it was before the file shut")
	}

	code, page := getPage(t, srv.URL+"/notes/"+carried)
	if code != http.StatusOK {
		t.Fatalf("GET the carried note status = %d, want %d", code, http.StatusOK)
	}
	if !strings.Contains(page, "the words read before the file shut") {
		t.Error("the carried note does not render the last words the folder read")
	}
	if !strings.Contains(page, "data-note-stale") {
		t.Error("the carried note is presented as freshly read; a reader would take it for the file's current words")
	}
	// The write face is never derived from a carried copy. The status panel
	// asks the file itself what its status line says, so a copy this
	// generation could not re-read cannot put a stale value under a control
	// that would act on it.
	if !strings.Contains(page, "ui-status--growing") {
		t.Error("the carried note's status face does not show the status the file itself carries")
	}
	if strings.Contains(page, "ui-status--seedling") {
		t.Error("the carried note's status face shows the carried copy's status, which a transition would be built from")
	}

	code, page = getPage(t, srv.URL+"/notes/"+fresh)
	if code != http.StatusOK {
		t.Fatalf("GET the note written during the failure status = %d, want %d", code, http.StatusOK)
	}
	if !strings.Contains(page, "written while the folder was degraded") {
		t.Error("a note written while one file was unreadable is not readable")
	}
	if strings.Contains(page, "data-note-stale") {
		t.Error("a note read in this generation is marked as a copy that could not be re-read")
	}

	code, page = getPage(t, srv.URL+"/notes/"+never)
	if code != http.StatusNotFound {
		t.Fatalf("GET the never-read note status = %d, want %d", code, http.StatusNotFound)
	}
	if !strings.Contains(page, "檔案存在") {
		t.Error("a file the folder has never had a reading of does not get the page saying it exists but could not be read")
	}
}
