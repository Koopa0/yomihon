package note_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/wording"
)

// A reader sees why type-only fields cannot be judged in the selected
// language, while authored values remain escaped text.
func TestTypeDependentNotePage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	fixture, openErr := os.OpenRoot(root)
	if openErr != nil {
		t.Fatalf("open fixture root: %v", openErr)
	}
	t.Cleanup(func() {
		if closeErr := fixture.Close(); closeErr != nil {
			t.Errorf("close fixture root: %v", closeErr)
		}
	})
	contractBytes, err := os.ReadFile(filepath.Join("..", "judge", "testdata", "vault-type-dependent", schema.ContractRelPath))
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	if mkdirErr := fixture.MkdirAll(filepath.Dir(schema.ContractRelPath), 0o750); mkdirErr != nil {
		t.Fatalf("create contract directory: %v", mkdirErr)
	}
	if writeErr := fixture.WriteFile(schema.ContractRelPath, contractBytes, 0o600); writeErr != nil {
		t.Fatalf("write fixture contract: %v", writeErr)
	}
	const rel = "Notes/InvalidType.md"
	const target = "Lesson<script>&"
	if mkdirErr := fixture.MkdirAll("Notes", 0o750); mkdirErr != nil {
		t.Fatalf("create notes directory: %v", mkdirErr)
	}
	if writeErr := fixture.WriteFile(rel, []byte("---\ntitle: Invalid type\ntype: 'Lesson<script>&'\nlevel: fundamental\nslug: invalid-type\nextra: yes\n---\n\nBody.\n"), 0o600); writeErr != nil {
		t.Fatalf("write fixture note: %v", writeErr)
	}
	contract, err := schema.Load(root)
	if err != nil {
		t.Fatalf("schema.Load() error = %v", err)
	}
	log := slog.New(slog.DiscardHandler)
	store, source := newSnapshotStore(t, root, log, contract, contract.Governance())
	writer := openStatusWriter(t, source, contract, contract.Governance())
	mux := http.NewServeMux()
	note.New(&note.Sources{
		Source: source, Status: writer.Authority, Snapshot: store.Current,
		ObservedStatus: writer.ObservedStatus, ConsumeReceipt: writer.ConsumeReceipt, Continuation: noMark, Log: log,
	}).Register(mux)
	for _, tt := range []struct {
		lang   wording.Lang
		want   string
		absent string
	}{
		{lang: wording.En, want: "type-only fields cannot be judged until", absent: "類型限定欄位要等"},
		{lang: wording.ZhHant, want: "類型限定欄位要等", absent: "type-only fields cannot be judged until"},
	} {
		t.Run(string(tt.lang), func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes/"+rel, http.NoBody)
			request.Header.Set("Cookie", wording.CookieName+"="+string(tt.lang))
			mux.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("GET note = %d, want %d", recorder.Code, http.StatusOK)
			}
			page := recorder.Body.String()
			for _, want := range []string{tt.want, "<code>type</code>", "<code>Lesson&lt;script&gt;&amp;</code>", "<code>extra</code>"} {
				if !strings.Contains(page, want) {
					t.Errorf("caught: type-dependent note page omits %q", want)
				}
			}
			for _, unwanted := range []string{tt.absent, "<code>slug</code>", "<code>level</code>", "<code>schema.type_dependent</code>", target} {
				if strings.Contains(page, unwanted) {
					t.Errorf("caught: type-dependent note page contains %q", unwanted)
				}
			}
		})
	}
}
