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
	"github.com/koopa0/yomihon/internal/snapshot"
)

// mapVault writes the one map the listing draws, declaring the language its
// title is written in. Two of these, differing in both, are what tells the two
// readings apart on the page.
func mapVault(t *testing.T, title, tag string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "Maps")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir Maps: %v", err)
	}
	body := "---\ntitle: " + title + "\ntype: topic-map\ndomain: golang\nstatus: evergreen\n" +
		"created: 2026-06-01\nupdated: 2026-06-01\nlang: " + tag + "\n---\n\n## Branch\n\n- [[Somewhere]]\n"
	if err := os.WriteFile(filepath.Join(dir, "Map.md"), []byte(body), 0o600); err != nil {
		t.Fatalf("write the map: %v", err)
	}
	return root
}

// rowTitleSpan returns the first listing row's title element, whole, so a
// failure states the name and the language as the reader met them rather than
// reporting that some expected string was missing.
func rowTitleSpan(t *testing.T, page string) string {
	t.Helper()
	const open = `<span class="y-row__title"`
	const closing = `</span>`
	at := strings.Index(page, open)
	if at < 0 {
		t.Fatalf("the listing drew no row title; page = %q", page)
	}
	end := strings.Index(page[at:], closing)
	if end < 0 {
		t.Fatalf("the row title is unterminated; from it the page reads %q", page[at:])
	}
	return page[at : at+end+len(closing)]
}

// TestTheMapIndexReadsOneGeneration holds the map listing to a single reading
// of the published snapshot. A row's name comes from the navigation model and
// the language it is written in comes from the note itself; taken from two
// readings, a rebuild landing between them pairs one version's title with
// another version's language, and the row states something about the vault that
// was never true of it.
func TestTheMapIndexReadsOneGeneration(t *testing.T) {
	t.Parallel()
	log := slog.New(slog.DiscardHandler)
	contract := loadHomeContract(t)
	storeA, sourceA := newSnapshotStore(t, mapVault(t, "Japanese title", "ja"), log, contract, contract.Governance())
	storeB, _ := newSnapshotStore(t, mapVault(t, "English title", "en"), log, contract, contract.Governance())
	writer := openStatusWriter(t, sourceA, contract, contract.Governance())

	for _, tt := range []struct {
		name string
		// published answers each reading of the pointer the request makes.
		published func() func() *snapshot.Generation
	}{
		{
			// The rebuild lands between the first reading and any second one,
			// which is the moment the page has to survive.
			name: "a rebuild between readings",
			published: func() func() *snapshot.Generation {
				readings := 0
				return func() *snapshot.Generation {
					readings++
					if readings > 1 {
						return storeB.Current()
					}
					return storeA.Current()
				}
			},
		},
		{
			// The control: nothing is published while the page is assembled, so
			// the row below is what the listing draws when no rebuild can
			// confuse it.
			name: "no rebuild at all",
			published: func() func() *snapshot.Generation {
				return storeA.Current
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mux := http.NewServeMux()
			note.New(&note.Sources{
				Source:         sourceA,
				Status:         writer.Authority,
				Snapshot:       tt.published(),
				ObservedStatus: writer.ObservedStatus,
				ConsumeReceipt: writer.ConsumeReceipt,
				Continuation:   noMark,
				Log:            log,
			}).Register(mux)
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			code, page := get(t, srv.Client(), srv.URL+"/maps")
			if code != http.StatusOK {
				t.Fatalf("GET /maps status = %d, want 200", code)
			}
			const want = `<span class="y-row__title" lang="ja">Japanese title</span>`
			if got := rowTitleSpan(t, page); got != want {
				t.Errorf("the map row draws two vault versions at once:\n got %s\nwant %s", got, want)
			}
		})
	}
}
