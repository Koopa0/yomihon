package note_test

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/shell"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/vault"
)

// benchNoteCount is the folder the desk is measured against: larger than the
// low-thousands the reading server is built for, so a number taken here is an
// upper bound on what a real folder costs rather than a hopeful one.
const benchNoteCount = 5000

// BenchmarkDesk measures one whole GET of the desk against that folder, which
// is the page a reader opens most and the one that has to carry every shared
// projection before it renders a word.
func BenchmarkDesk(b *testing.B) {
	srv := benchServer(b)
	client := srv.Client()
	b.ReportAllocs()
	for b.Loop() {
		req, err := http.NewRequestWithContext(b.Context(), http.MethodGet, srv.URL+"/", http.NoBody)
		if err != nil {
			b.Fatalf("build GET /: %v", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			b.Fatalf("GET /: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			b.Fatalf("GET / = %d, want 200", resp.StatusCode)
		}
		// The page is read to the end, because a benchmark that measured only
		// the headers would leave the rendering it is about out of the number.
		if _, err := io.Copy(io.Discard, resp.Body); err != nil {
			b.Fatalf("read body: %v", err)
		}
		if err := resp.Body.Close(); err != nil {
			b.Fatalf("close body: %v", err)
		}
	}
}

// benchServer serves one folder of benchNoteCount notes through the production
// composition. Every note declares a type, a domain and a status the contract
// knows, which is the ordinary shape of a folder somebody keeps; every fourth
// cites a name nothing answers to, so the health gathering behind the rail's
// foot reports something rather than walking an empty folder.
func benchServer(b *testing.B) *httptest.Server {
	b.Helper()
	root := b.TempDir()
	for i := range benchNoteCount {
		rel := fmt.Sprintf("Writing/lessons/L%04d.md", i)
		body := fmt.Sprintf(
			"---\ntitle: L%04d\ntype: lesson\ndomain: japanese\nstatus: draft\n"+
				"created: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n",
			i,
		)
		if i%4 == 0 {
			body += fmt.Sprintf("cites [[Nobody wrote %04d]]\n", i)
		}
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			b.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			b.Fatalf("write %s: %v", rel, err)
		}
	}
	contract, err := schema.LoadFile(filepath.Join("..", "shell", "testdata", "contract.toml"))
	if err != nil {
		b.Fatalf("schema.LoadFile() error = %v", err)
	}
	reader, err := vault.Open(root)
	if err != nil {
		b.Fatalf("vault.Open() error = %v", err)
	}
	b.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			b.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	log := slog.New(slog.DiscardHandler)
	writer, err := status.Open(reader, contract, contract.Governance(), log)
	if err != nil {
		b.Fatalf("status.Open() error = %v", err)
	}
	b.Cleanup(func() {
		if closeErr := writer.Close(); closeErr != nil {
			b.Errorf("Writer.Close() error = %v", closeErr)
		}
	})
	store, err := snapshot.New(b.Context(), reader, log, contract, contract.Governance())
	if err != nil {
		b.Fatalf("snapshot.New() error = %v", err)
	}
	mux := http.NewServeMux()
	note.New(&note.Sources{
		Source:         reader,
		VaultName:      shell.VaultName(reader.Name()),
		Status:         writer.Authority,
		Snapshot:       store.Current,
		ObservedStatus: writer.ObservedStatus,
		ConsumeReceipt: writer.ConsumeReceipt,
		Log:            log,
	}).Register(mux)
	srv := httptest.NewServer(mux)
	b.Cleanup(srv.Close)
	return srv
}
