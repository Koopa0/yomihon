package note_test

import (
	"crypto/sha256"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/shell"
)

// The two policies a figure is served under, written out as literals rather
// than read from the code that builds them, so a weakened policy cannot pass
// by being compared with itself.
const (
	scriptlessFigurePolicy = "sandbox; default-src 'none'; base-uri 'none'; connect-src 'none'; " +
		"font-src 'none'; form-action 'none'; frame-ancestors 'self'; frame-src 'none'; " +
		"img-src 'none'; media-src 'none'; object-src 'none'; script-src 'none'; " +
		"script-src-attr 'none'; style-src 'unsafe-inline'; worker-src 'none'"
	// The hashes are of "document.title='one'" and "document.title='two'",
	// computed with openssl dgst -sha256 -binary | base64.
	scriptedFigurePolicy = "sandbox allow-scripts; default-src 'none'; base-uri 'none'; connect-src 'none'; " +
		"font-src 'none'; form-action 'none'; frame-ancestors 'self'; frame-src 'none'; " +
		"img-src 'none'; media-src 'none'; object-src 'none'; " +
		"script-src 'sha256-Pk6kGZ53C/j3zM8I8k1JSRE+kVBXsjQz8VZcsbbwJ2M=' 'sha256-vOHfxP/+BHNtkvm4fGKqR1utplD8WcQxho6/xxY4f0s='; " +
		"script-src-attr 'none'; style-src 'unsafe-inline'; worker-src 'none'"
)

const (
	scriptlessFigure = "<!doctype html><title>still</title><p>no script</p>\n"
	scriptedFigure   = "<!doctype html><title>moving</title><script>document.title='one'</script><p>x</p><script>document.title='two'</script>\n"
	refusedFigure    = "<!doctype html><noscript><iframe src=\"http://outside.test/\"></iframe></noscript>\n"
	figureHost       = "# Host\n\n![[still.html]]\n\n![[moving.html|640x300]]\n\n![[still.html|A still figure]]\n\n![[refused.html]]\n"
)

// figureVault is a vault holding one figure of each kind and a note that
// embeds them.
func figureVault(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "vault")
	mkdir(t, filepath.Join(root, "Figures"))
	write(t, filepath.Join(root, "README.md"), []byte("home\n"))
	write(t, filepath.Join(root, "host.md"), []byte(figureHost))
	write(t, filepath.Join(root, "Figures", "still.html"), []byte(scriptlessFigure))
	write(t, filepath.Join(root, "Figures", "moving.html"), []byte(scriptedFigure))
	write(t, filepath.Join(root, "Figures", "refused.html"), []byte(refusedFigure))
	return root
}

// newFigureServer serves root through the production policy seam, with
// approvals read from marks (nil for a process that keeps none, as the public
// instance's sample vault has none).
func newFigureServer(t *testing.T, root string, marks *mark.File) *httptest.Server {
	t.Helper()
	log := slog.New(slog.DiscardHandler)
	store, source := newSnapshotStore(t, root, log, nil, schema.Ungoverned())
	authority := func() schema.Authority { return schema.CaptureAuthority(nil, schema.Ungoverned()) }
	var approved func(string, [sha256.Size]byte) (render.Approval, error)
	if marks != nil {
		approved = func(rel string, sum [sha256.Size]byte) (render.Approval, error) {
			ok, lapsed, err := marks.FigureApproved(rel, sum)
			switch {
			case err != nil:
				return render.Unapproved, err
			case ok:
				return render.Approved, nil
			case lapsed:
				return render.Lapsed, nil
			}
			return render.Unapproved, nil
		}
		store.ApproveFiguresBy(approved)
	}
	mux := http.NewServeMux()
	note.New(&note.Sources{
		Source:    source,
		VaultName: shell.VaultName(source.Name()),
		Authority: authority,
		Snapshot:  store.Current,

		Continuation:   noMark,
		FigureApproved: approved,
		Log:            log,
	}).Register(mux)
	srv := httptest.NewServer(origin.Protect(mux))
	t.Cleanup(srv.Close)
	return srv
}

func approvalsFor(t *testing.T) *mark.File {
	t.Helper()
	marks, err := mark.New(t.TempDir(), "/vaults/figures")
	if err != nil {
		t.Fatal(err)
	}
	return marks
}

// TestAFigureWithoutApprovalIsServedWithoutScript holds the default tier. A
// figure is a page of its own bytes under the sandbox every raw markup file
// gets — no script, no request of any kind — and it is never a download, or
// the frame would show nothing. A figure with scripts and no approval gets
// exactly the same, which is all the public instance, holding no approvals,
// ever serves.
func TestAFigureWithoutApprovalIsServedWithoutScript(t *testing.T) {
	t.Parallel()
	root := figureVault(t)
	for name, marks := range map[string]*mark.File{
		"no approvals kept":          nil,
		"approvals kept, none given": approvalsFor(t),
	} {
		srv := newFigureServer(t, root, marks)
		for path, bytes := range map[string]string{
			"Figures/still.html":  scriptlessFigure,
			"Figures/moving.html": scriptedFigure,
		} {
			code, header, body := fetch(t, srv.Client(), srv.URL+"/embed/"+path)
			if code != http.StatusOK {
				t.Fatalf("%s: GET /embed/%s = %d, want 200", name, path, code)
			}
			for key, want := range map[string]string{
				"Content-Security-Policy": scriptlessFigurePolicy,
				"Content-Type":            "text/html; charset=utf-8",
				"X-Content-Type-Options":  "nosniff",
				"Cache-Control":           "no-store",
				"Content-Disposition":     "",
				"Location":                "",
			} {
				if got := header.Get(key); got != want {
					t.Errorf("%s: GET /embed/%s %s = %q, want %q", name, path, key, got, want)
				}
			}
			if body != bytes {
				t.Errorf("%s: GET /embed/%s body = %q, want the file's bytes", name, path, body)
			}
		}
	}
}

// TestAnApprovedFigureRunsOnlyItsOwnScripts holds the second tier: the bytes
// approved are served with a sandbox that admits scripts and a script-src that
// names each of their inline scripts by hash and nothing else.
func TestAnApprovedFigureRunsOnlyItsOwnScripts(t *testing.T) {
	t.Parallel()
	root := figureVault(t)
	marks := approvalsFor(t)
	if err := marks.ApproveFigure("Figures/moving.html", sha256.Sum256([]byte(scriptedFigure)), time.Now()); err != nil {
		t.Fatal(err)
	}
	srv := newFigureServer(t, root, marks)
	code, header, body := fetch(t, srv.Client(), srv.URL+"/embed/Figures/moving.html")
	if code != http.StatusOK || body != scriptedFigure {
		t.Fatalf("GET /embed/Figures/moving.html = %d %q, want 200 and the file's bytes", code, body)
	}
	if got := header.Get("Content-Security-Policy"); got != scriptedFigurePolicy {
		t.Errorf("approved figure policy = %q\nwant %q", got, scriptedFigurePolicy)
	}

	// One character changes and the approval names bytes that are gone.
	write(t, filepath.Join(root, "Figures", "moving.html"), []byte(strings.Replace(scriptedFigure, "'two'", "'tw0'", 1)))
	_, header, _ = fetch(t, srv.Client(), srv.URL+"/embed/Figures/moving.html")
	if got := header.Get("Content-Security-Policy"); got != scriptlessFigurePolicy {
		t.Errorf("an edited figure kept its approval: policy = %q", got)
	}
}

// TestUnreadableApprovalsServeNoScript: damage to the record is no approval.
func TestUnreadableApprovalsServeNoScript(t *testing.T) {
	t.Parallel()
	root := figureVault(t)
	marks := approvalsFor(t)
	if err := marks.ApproveFigure("Figures/moving.html", sha256.Sum256([]byte(scriptedFigure)), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marks.ApprovalsPath(), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := newFigureServer(t, root, marks)
	_, header, _ := fetch(t, srv.Client(), srv.URL+"/embed/Figures/moving.html")
	if got := header.Get("Content-Security-Policy"); got != scriptlessFigurePolicy {
		t.Errorf("figure policy over an unreadable record = %q, want the scriptless one", got)
	}
	_, _, page := fetch(t, srv.Client(), srv.URL+"/notes/host.md")
	if !strings.Contains(page, "The approval record could not be read") && !strings.Contains(page, "核可紀錄無法讀取") {
		t.Error("the page does not say the approval record could not be read")
	}
}

// TestARefusedFigureSendsNoneOfItsBytes: a figure whose markup reaches out on
// its own is answered with the reason and none of the file.
func TestARefusedFigureSendsNoneOfItsBytes(t *testing.T) {
	t.Parallel()
	srv := newFigureServer(t, figureVault(t), nil)
	code, header, body := fetch(t, srv.Client(), srv.URL+"/embed/Figures/refused.html")
	if code != http.StatusUnprocessableEntity {
		t.Errorf("GET /embed/Figures/refused.html = %d, want 422", code)
	}
	if strings.Contains(body, "outside.test") || strings.Contains(body, "<noscript") {
		t.Errorf("the refusal carries the figure's bytes: %q", body)
	}
	if got := header.Get("Content-Security-Policy"); got != scriptlessFigurePolicy {
		t.Errorf("refusal policy = %q, want the scriptless sandbox", got)
	}
	if !strings.Contains(body, "<iframe>") {
		t.Errorf("the refusal does not name what it refused: %q", body)
	}
}

func TestTheFigureRouteServesOnlyFigures(t *testing.T) {
	t.Parallel()
	root := figureVault(t)
	write(t, filepath.Join(root, "Figures", "drawing.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`))
	srv := newFigureServer(t, root, nil)
	for _, path := range []string{"host.md", "Figures/drawing.svg", "Figures/missing.html", "Figures"} {
		if code, _, _ := fetch(t, srv.Client(), srv.URL+"/embed/"+path); code != http.StatusNotFound {
			t.Errorf("GET /embed/%s = %d, want 404", path, code)
		}
	}
}

// TestTheReadingPageFramesEachFigure pins the frame the page draws: an empty
// sandbox for every figure but an approved one, which admits scripts and
// nothing more; a title; no referrer; no permissions; the size the embed
// names as a fixed height; and a placeholder, not a frame, for a refused one.
func TestTheReadingPageFramesEachFigure(t *testing.T) {
	t.Parallel()
	root := figureVault(t)
	marks := approvalsFor(t)

	srv := newFigureServer(t, root, marks)
	_, _, page := fetch(t, srv.Client(), srv.URL+"/notes/host.md")
	for _, want := range []string{
		`<iframe class="figure-embed__frame" src="/embed/Figures/still.html" title="still.html" sandbox="" referrerpolicy="no-referrer" allow="" height="480"></iframe>`,
		`<iframe class="figure-embed__frame" src="/embed/Figures/moving.html" title="moving.html" sandbox="" referrerpolicy="no-referrer" allow="" width="640" height="300"></iframe>`,
		`<iframe class="figure-embed__frame" src="/embed/Figures/still.html" title="A still figure" sandbox="" referrerpolicy="no-referrer" allow="" height="480"></iframe>`,
		`這個圖解有腳本，目前沒有執行`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the reading page does not hold\n%s", want)
		}
	}
	if strings.Contains(page, `src="/embed/Figures/refused.html"`) {
		t.Error("a refused figure was framed")
	}
	if strings.Contains(page, "allow-scripts") || strings.Contains(page, "data-figure-scripts") {
		t.Error("an unapproved figure's frame admits scripts")
	}

	if err := marks.ApproveFigure("Figures/moving.html", sha256.Sum256([]byte(scriptedFigure)), time.Now()); err != nil {
		t.Fatal(err)
	}
	_, _, page = fetch(t, srv.Client(), srv.URL+"/notes/host.md")
	const approved = `<iframe class="figure-embed__frame" src="/embed/Figures/moving.html" title="moving.html" sandbox="allow-scripts" referrerpolicy="no-referrer" allow="" width="640" height="300" data-figure-scripts></iframe>`
	if !strings.Contains(page, approved) {
		t.Errorf("the approved figure's frame is not\n%s", approved)
	}
	if strings.Count(page, "allow-scripts") != 1 {
		t.Errorf("allow-scripts appears %d times on the page, want once", strings.Count(page, "allow-scripts"))
	}
	if strings.Contains(page, "目前沒有執行") {
		t.Error("the approved figure still says its scripts are not running")
	}
	if strings.Contains(page, "approve-figure") {
		t.Error("the page names the approval command, which a visitor to a public instance cannot run")
	}

	// An approval of other bytes at the same path is one that lapsed when
	// the file changed, and the page says so instead of how to approve.
	if err := marks.ApproveFigure("Figures/moving.html", sha256.Sum256([]byte(scriptedFigure+" ")), time.Now()); err != nil {
		t.Fatal(err)
	}
	_, _, page = fetch(t, srv.Client(), srv.URL+"/notes/host.md")
	if !strings.Contains(page, "核可已失效") || strings.Contains(page, "allow-scripts") {
		t.Error("a lapsed approval is not said, or its frame still admits scripts")
	}
}

// TestAHoverCardNeverFramesAFigure: a card shows a note the reader has not
// opened, so an approved figure in it is a stand-in and not a frame — nothing
// of the figure is fetched, and none of its scripts runs, because a link to
// its note was passed over.
func TestAHoverCardNeverFramesAFigure(t *testing.T) {
	t.Parallel()
	root := figureVault(t)
	marks := approvalsFor(t)
	if err := marks.ApproveFigure("Figures/moving.html", sha256.Sum256([]byte(scriptedFigure)), time.Now()); err != nil {
		t.Fatal(err)
	}
	srv := newFigureServer(t, root, marks)
	_, _, page := fetch(t, srv.Client(), srv.URL+"/notes/host.md")
	if !strings.Contains(page, `sandbox="allow-scripts"`) {
		t.Fatal("the opened note does not frame its approved figure, so this test proves nothing about the card")
	}
	code, _, card := fetch(t, srv.Client(), srv.URL+"/preview/host.md")
	if code != http.StatusOK {
		t.Fatalf("GET /preview/host.md = %d, want 200", code)
	}
	for _, refused := range []string{"<iframe", "figure-embed__frame", "/embed/", "allow-scripts"} {
		if strings.Contains(card, refused) {
			t.Errorf("the hover card holds %q:\n%s", refused, card)
		}
	}
	if strings.Count(card, "這裡無法顯示") != 4 {
		t.Errorf("the hover card shows %d figure stand-ins, want one for each of the four embeds:\n%s", strings.Count(card, "這裡無法顯示"), card)
	}
}

// newApproveAllServer serves root with every figure approved, the worst case
// for a page that shows one note inside another.
func newApproveAllServer(t *testing.T, root string, contract *schema.Contract) *httptest.Server {
	t.Helper()
	log := slog.New(slog.DiscardHandler)
	governance := schema.Ungoverned()
	if contract != nil {
		governance = contract.Governance()
	}
	store, source := newSnapshotStore(t, root, log, contract, governance)
	authority := func() schema.Authority { return schema.CaptureAuthority(contract, governance) }
	approveAll := func(string, [sha256.Size]byte) (render.Approval, error) { return render.Approved, nil }
	store.ApproveFiguresBy(approveAll)
	mux := http.NewServeMux()
	note.New(&note.Sources{
		Source:    source,
		Contract:  contract,
		VaultName: shell.VaultName(source.Name()),
		Authority: authority,
		Snapshot:  store.Current,

		Continuation:   noMark,
		FigureApproved: approveAll,
		Log:            log,
	}).Register(mux)
	srv := httptest.NewServer(origin.Protect(mux))
	t.Cleanup(srv.Close)
	return srv
}

// framesNoFigure fails when page frames a figure at all.
func framesNoFigure(t *testing.T, where, page string) {
	t.Helper()
	for _, refused := range []string{"figure-embed__frame", "/embed/", "allow-scripts"} {
		if strings.Contains(page, refused) {
			t.Errorf("%s holds %q: a figure is framed outside its own note's page", where, refused)
		}
	}
	if !strings.Contains(page, "這裡無法顯示") {
		t.Errorf("%s shows no figure stand-in, so the figure was not reached and this proves nothing", where)
	}
}

// TestAConceptSheetNeverFramesAFigure: a lesson carries the sheets of the
// concepts it cites, drawn into the lesson's own page; a figure in one is the
// concept note's, so on the lesson's page it is a stand-in.
func TestAConceptSheetNeverFramesAFigure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "Concepts", "japanese"))
	mkdir(t, filepath.Join(root, "Writing", "lessons", "japanese"))
	write(t, filepath.Join(root, "Concepts", "japanese", "は.md"),
		[]byte("---\ntype: concept\n---\n# は (主題助詞)\n\nMarks the topic.\n\n![[moving.html]]\n"))
	write(t, filepath.Join(root, "Concepts", "japanese", "moving.html"), []byte(scriptedFigure))
	write(t, filepath.Join(root, "Writing", "lessons", "japanese", "L01.md"),
		[]byte("---\ntype: lesson\n---\n# L01\n\nThe particle [[は]] marks the topic.\n"))
	srv := newApproveAllServer(t, root, loadContract(t))
	_, _, concept := fetch(t, srv.Client(), srv.URL+"/notes/Concepts/japanese/%E3%81%AF.md")
	if !strings.Contains(concept, `sandbox="allow-scripts"`) {
		t.Fatal("the concept's own page does not frame its approved figure, so this test proves nothing")
	}
	code, _, page := fetch(t, srv.Client(), srv.URL+"/notes/Writing/lessons/japanese/L01.md")
	if code != http.StatusOK || !strings.Contains(page, "data-concept-sheet") {
		t.Fatalf("GET the lesson = %d without a concept sheet", code)
	}
	framesNoFigure(t, "the lesson's concept sheet", page)
}

// TestTheComparePageNeverFramesAFigure: two notes side by side are read on a
// page that is neither note's own.
func TestTheComparePageNeverFramesAFigure(t *testing.T) {
	t.Parallel()
	root := figureVault(t)
	write(t, filepath.Join(root, "other.md"), []byte("# Other\n\nplain\n"))
	srv := newApproveAllServer(t, root, nil)
	code, _, page := fetch(t, srv.Client(), srv.URL+"/compare/host.md?with=other.md")
	if code != http.StatusOK || !strings.Contains(page, "plain") {
		t.Fatalf("GET /compare/host.md?with=other.md = %d without both notes", code)
	}
	framesNoFigure(t, "the compare page", page)
}
