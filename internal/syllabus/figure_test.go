package syllabus_test

import (
	"bytes"
	"crypto/sha256"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/syllabus"
)

// TestACoursePageNeverFramesAFigure: an approved figure framed anywhere but
// its own note's page runs where its reader never opened it. The course page's
// opening and the listening page both show notes' words inside a page of their
// own, so no figure there is framed, with every figure approved.
func TestACoursePageNeverFramesAFigure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lessons := filepath.Join(root, "Writing", "lessons", "golang")
	maps := filepath.Join(root, "Maps")
	for _, dir := range []string{lessons, maps} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, body string) {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	const figure = "<p>x</p><script>document.title='moving'</script>"
	write(filepath.Join(lessons, "moving.html"), figure)
	write(filepath.Join(maps, "course.html"), figure)
	write(filepath.Join(lessons, "First.md"), "---\ntitle: First\ntype: lesson\ndomain: golang\nstatus: ready\n"+
		"created: 2026-06-01\nupdated: 2026-06-01\n---\n\nopening\n\n<!-- read-aloud: ja -->\nいち。![[moving.html]]\n")
	write(filepath.Join(maps, "Path.md"), "---\ntitle: Go path\ntype: study-path\ndomain: golang\nstatus: evergreen\n"+
		"created: 2026-06-01\nupdated: 2026-06-01\n---\n\nThe course opens here. ![[course.html]]\n\n"+
		"## line | Line | 線 {sequence=primary}\n\n- [[First]]\n")

	contract, err := schema.LoadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatal(err)
	}
	store := newGenerationStore(t, root, contract)
	store.ApproveFiguresBy(func(string, [sha256.Size]byte) (render.Approval, error) { return render.Approved, nil })
	mux := http.NewServeMux()
	syllabus.New(func() syllabus.RequestSnapshot {
		snap := store.Current().Capture()
		return syllabus.RequestSnapshot{
			Shell:      nav.Shell{Nav: snap.Navigation(), Governed: true},
			Generation: snap,
			Authority:  lessonTypes("lesson"),
		}
	}, slog.New(slog.DiscardHandler)).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	for _, address := range []string{"/listen/Maps/Path.md", "/courses/Maps/Path.md"} {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+address, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		res, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var page bytes.Buffer
		if _, err := page.ReadFrom(res.Body); err != nil {
			t.Fatal(err)
		}
		if closeErr := res.Body.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		if res.StatusCode != http.StatusOK {
			t.Fatalf("GET %s = %d", address, res.StatusCode)
		}
		for _, refused := range []string{"figure-embed__frame", "/embed/", "allow-scripts"} {
			if strings.Contains(page.String(), refused) {
				t.Errorf("GET %s holds %q: a lesson's figure is framed outside its own note's page", address, refused)
			}
		}
		// The listening page drops a marked paragraph holding a figure whole,
		// since every figure's markup holds a paragraph of its own; only the
		// course page shows the stand-in.
		if address == "/courses/Maps/Path.md" && !strings.Contains(page.String(), "這裡無法顯示") {
			t.Errorf("GET %s shows no figure stand-in, so the figure was not reached and this proves nothing", address)
		}
	}
}
