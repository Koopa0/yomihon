package syllabus_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/syllabus"
	"github.com/koopa0/yomihon/internal/vault"
)

// pathVault writes one study-path and the single lesson it lists, with the
// study-path declaring the language its title is written in. Two of these,
// differing in both, are what tells one reading of the vault from another on
// the listing page.
func pathVault(t *testing.T, title, tag string) string {
	t.Helper()
	root := t.TempDir()
	lessonDir := filepath.Join(root, "Writing", "lessons", "golang")
	if err := os.MkdirAll(lessonDir, 0o750); err != nil {
		t.Fatalf("mkdir the lesson folder: %v", err)
	}
	lesson := "---\ntitle: Slices\ntype: lesson\ndomain: golang\nstatus: ready\n" +
		"created: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(lessonDir, "Slices.md"), []byte(lesson), 0o600); err != nil {
		t.Fatalf("write the lesson: %v", err)
	}
	mapsDir := filepath.Join(root, "Maps")
	if err := os.MkdirAll(mapsDir, 0o750); err != nil {
		t.Fatalf("mkdir Maps: %v", err)
	}
	path := "---\ntitle: " + title + "\ntype: study-path\ndomain: golang\nstatus: evergreen\n" +
		"created: 2026-06-01\nupdated: 2026-06-01\nlang: " + tag + "\n---\n\n" +
		"## data | Data | 資料\n\n### text | Text | 文字 {sequence=primary}\n\n- [[Slices]]\n"
	if err := os.WriteFile(filepath.Join(mapsDir, "Path.md"), []byte(path), 0o600); err != nil {
		t.Fatalf("write the study-path: %v", err)
	}
	return root
}

// newGenerationStore publishes one generation over root and never runs the
// watcher, so the only publication in a test is the one the test performs.
func newGenerationStore(t *testing.T, root string, contract *schema.Contract) *snapshot.Store {
	t.Helper()
	source, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if closeErr := source.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	store, err := snapshot.New(t.Context(), source, slog.New(slog.DiscardHandler), contract, contract.Governance())
	if err != nil {
		t.Fatalf("snapshot.New: %v", err)
	}
	return store
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

// TestThePathIndexReadsOneGeneration holds the study-path listing to a single
// reading of the published snapshot. A row's name comes from the navigation
// model and the language it is written in comes from the note itself; taken
// from two readings, a rebuild landing between them pairs one version's title
// with another version's language, and the row states something about the vault
// that was never true of it.
func TestThePathIndexReadsOneGeneration(t *testing.T) {
	t.Parallel()
	contract, err := schema.LoadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("schema.LoadFile = %v", err)
	}
	storeA := newGenerationStore(t, pathVault(t, "Japanese title", "ja"), contract)
	storeB := newGenerationStore(t, pathVault(t, "English title", "en"), contract)
	answer := func(store *snapshot.Store) (nav.Shell, *snapshot.Generation) {
		snap := store.Current().Capture()
		return nav.Shell{Nav: snap.Navigation(), Governed: true}, snap
	}

	for _, tt := range []struct {
		name string
		// published answers each reading of the pointer the request makes.
		published func() func() (nav.Shell, *snapshot.Generation)
	}{
		{
			// The rebuild lands between the first reading and any second one,
			// which is the moment the page has to survive.
			name: "a rebuild between readings",
			published: func() func() (nav.Shell, *snapshot.Generation) {
				readings := 0
				return func() (nav.Shell, *snapshot.Generation) {
					readings++
					if readings > 1 {
						return answer(storeB)
					}
					return answer(storeA)
				}
			},
		},
		{
			// The control: nothing is published while the page is assembled, so
			// the row below is what the listing draws when no rebuild can
			// confuse it.
			name: "no rebuild at all",
			published: func() func() (nav.Shell, *snapshot.Generation) {
				return func() (nav.Shell, *snapshot.Generation) { return answer(storeA) }
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mux := http.NewServeMux()
			syllabus.New(tt.published(), slog.New(slog.DiscardHandler)).Register(mux)
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			code, page := get(t, srv.Client(), srv.URL+"/paths")
			if code != http.StatusOK {
				t.Fatalf("GET /paths status = %d, want 200", code)
			}
			const want = `<span class="y-row__title" lang="ja">Japanese title</span>`
			if got := rowTitleSpan(t, page); got != want {
				t.Errorf("the study-path row draws two vault versions at once:\n got %s\nwant %s", got, want)
			}
		})
	}
}
