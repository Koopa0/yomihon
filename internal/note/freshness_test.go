package note_test

import (
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/vault"
)

const freshRel = "Writing/watched.md"

// writeFreshNote puts body at rel under root, creating the folders it needs.
func writeFreshNote(t *testing.T, root, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(freshRel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", full, err)
	}
}

// identityOf is what a reading page carries for these bytes.
func identityOf(body string) string {
	sum := vault.ContentIdentity([]byte(body))
	return hex.EncodeToString(sum[:])
}

// askFreshness performs the request a page open on rel makes every few
// seconds, and returns the status code beside the answer. It carries no
// status, which is the recovery page's ask: that page binds only the bytes a
// refused write saw.
func askFreshness(t *testing.T, srvURL, identity string) (code int, answer string) {
	t.Helper()
	return askFreshnessURL(t, srvURL+"/freshness/"+freshRel+"?identity="+identity)
}

// askFreshnessWithStatus is the reading page's ask: beside the identity it
// carries the status it printed, so the answer covers the pair.
func askFreshnessWithStatus(t *testing.T, srvURL, identity, printed string) (code int, answer string) {
	t.Helper()
	return askFreshnessURL(t,
		srvURL+"/freshness/"+freshRel+"?identity="+identity+"&status="+url.QueryEscape(printed))
}

func askFreshnessURL(t *testing.T, full string) (code int, answer string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, full, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest error = %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /freshness/%s error = %v", freshRel, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("Body.Close() error = %v", closeErr)
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body error = %v", err)
	}
	return resp.StatusCode, string(body)
}

func TestFreshnessSaysUnchangedWhileTheNoteIsUntouched(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const body = "# Watched\n\nThe words a reader is reading.\n"
	writeFreshNote(t, root, body)
	srv := newServer(t, root)

	code, got := askFreshness(t, srv.URL, identityOf(body))
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "unchanged" {
		t.Errorf("freshness of an untouched note = %q, want %q", got, "unchanged")
	}
}

// TestFreshnessOffersReloadOnlyOncePublishedCatchesUp is the invariant behind
// the two answers: a page is invited to reload only when reloading would
// render what the check just saw on disk. While the published generation still
// holds the old bytes, a reload would repeat them, so the answer says a new
// version was detected and withholds the invitation.
func TestFreshnessOffersReloadOnlyOncePublishedCatchesUp(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const rendered = "# Watched\n\nWhat the page rendered.\n"
	writeFreshNote(t, root, rendered)
	// The generation is built here and never rebuilt in this test, so it holds
	// exactly the bytes on disk at this moment.
	srv := newServer(t, root)

	// The published generation agrees with the disk and both differ from what
	// this page rendered: reloading now shows the reader something new.
	code, got := askFreshness(t, srv.URL, identityOf("# Watched\n\nAn older draft.\n"))
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "stale" {
		t.Fatalf("freshness with the generation caught up = %q, want %q", got, "stale")
	}

	// Now the disk moves ahead of the generation. Reloading would render the
	// same bytes again, so the invitation is withheld.
	writeFreshNote(t, root, "# Watched\n\nJust saved in Obsidian.\n")
	code, got = askFreshness(t, srv.URL, identityOf(rendered))
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "preparing" {
		t.Errorf("freshness with the generation behind the disk = %q, want %q", got, "preparing")
	}
}

func TestFreshnessSaysGoneWhenTheFileLeaves(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const body = "# Watched\n\nHere for now.\n"
	writeFreshNote(t, root, body)
	srv := newServer(t, root)

	if err := os.Remove(filepath.Join(root, filepath.FromSlash(freshRel))); err != nil {
		t.Fatalf("Remove error = %v", err)
	}
	code, got := askFreshness(t, srv.URL, identityOf(body))
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "gone" {
		t.Errorf("freshness of a removed note = %q, want %q", got, "gone")
	}
}

// TestFreshnessNeverCallsAnUnreadableFileGone holds the line the write face
// holds: what could not be confirmed does not become a confirmed fact. A
// passing read failure must not reach a reader as a deletion.
func TestFreshnessNeverCallsAnUnreadableFileGone(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const body = "# Watched\n\nStill on disk.\n"
	writeFreshNote(t, root, body)
	srv := newServer(t, root)
	lockNote(t, filepath.Join(root, filepath.FromSlash(freshRel)))

	code, got := askFreshness(t, srv.URL, identityOf(body))
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got == "gone" {
		t.Fatal("an unreadable note is reported as removed; a passing read failure must not become a deletion")
	}
	if got != "unreadable" {
		t.Errorf("freshness of an unreadable note = %q, want %q", got, "unreadable")
	}
}

// TestFreshnessAnnouncesAStatusOnlyRewrite pins the second half of what a
// reading page carries. The identity leaves the frontmatter status value out —
// that exclusion belongs to the write face — so the page states the status it
// printed separately, and a rewrite of that one value is answered stale: a
// page left open keeps showing the value it printed until it reloads.
func TestFreshnessAnnouncesAStatusOnlyRewrite(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const before = "---\ntype: lesson\nstatus: draft\n---\n\nBody.\n"
	const after = "---\ntype: lesson\nstatus: ready\n---\n\nBody.\n"
	writeFreshNote(t, root, before)
	srv := newServer(t, root)
	writeFreshNote(t, root, after)

	code, got := askFreshnessWithStatus(t, srv.URL, identityOf(before), "draft")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "stale" {
		t.Errorf("freshness after a status-only rewrite, asked with the printed status = %q, want %q", got, "stale")
	}
}

// TestFreshnessComparesIdentityAloneWhenNoStatusIsCarried keeps the ask that
// states no status — the recovery page's — on the identity alone: the same
// rewrite the test above announces stays invisible here. Running against the
// same fixture, this also proves the stale answer above came from the status
// pair rather than from the identity, because these bytes still match.
func TestFreshnessComparesIdentityAloneWhenNoStatusIsCarried(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const before = "---\ntype: lesson\nstatus: draft\n---\n\nBody.\n"
	const after = "---\ntype: lesson\nstatus: ready\n---\n\nBody.\n"
	writeFreshNote(t, root, before)
	srv := newServer(t, root)
	writeFreshNote(t, root, after)

	code, got := askFreshness(t, srv.URL, identityOf(before))
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "unchanged" {
		t.Errorf("freshness after a status-only rewrite, asked with no status = %q, want %q", got, "unchanged")
	}
}

// TestFreshnessStaysQuietWhenThePrintedStatusStillHolds is the ask the page
// makes right after its own flip landed and the redirect re-rendered it: the
// stamp already names the new status, so nothing is news. A stale answer here
// would invite a reload that delivers the words already on screen.
func TestFreshnessStaysQuietWhenThePrintedStatusStillHolds(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const before = "---\ntype: lesson\nstatus: draft\n---\n\nBody.\n"
	const after = "---\ntype: lesson\nstatus: ready\n---\n\nBody.\n"
	writeFreshNote(t, root, before)
	srv := newServer(t, root)
	writeFreshNote(t, root, after)

	code, got := askFreshnessWithStatus(t, srv.URL, identityOf(before), "ready")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "unchanged" {
		t.Errorf("freshness asked with the status the disk now carries = %q, want %q", got, "unchanged")
	}
}

// TestFreshnessHoldsStatusNewsBehindThePublishedGate pins the order of the two
// comparisons. When the disk has moved past the published generation, a reload
// would render the old bytes again, and a status difference must not talk over
// that: the answer stays preparing until the generation holds what the disk
// holds.
func TestFreshnessHoldsStatusNewsBehindThePublishedGate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const before = "---\ntype: lesson\nstatus: draft\n---\n\nBody.\n"
	const after = "---\ntype: lesson\nstatus: ready\n---\n\nA new body.\n"
	writeFreshNote(t, root, before)
	// The generation is built here and never rebuilt in this test, so the
	// rewrite below leaves the disk ahead of it.
	srv := newServer(t, root)
	writeFreshNote(t, root, after)

	code, got := askFreshnessWithStatus(t, srv.URL, identityOf(before), "draft")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "preparing" {
		t.Errorf("freshness with the generation behind a body-and-status rewrite = %q, want %q", got, "preparing")
	}
}

// TestFreshnessTreatsABodyEditTheSameWithAStatusCarried holds the behaviour a
// body edit always had: the carried status is a second comparison, not a veto,
// and a page whose bytes moved on is told so even though the status it printed
// still stands.
func TestFreshnessTreatsABodyEditTheSameWithAStatusCarried(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const rendered = "---\ntype: lesson\nstatus: draft\n---\n\nAn older body.\n"
	const onDisk = "---\ntype: lesson\nstatus: draft\n---\n\nBody.\n"
	writeFreshNote(t, root, onDisk)
	// The generation is built from the disk's bytes, so it has already caught
	// up with them and only the page is behind.
	srv := newServer(t, root)

	code, got := askFreshnessWithStatus(t, srv.URL, identityOf(rendered), "draft")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "stale" {
		t.Errorf("freshness after a body edit, asked with the unchanged status = %q, want %q", got, "stale")
	}
}

func TestFreshnessRefusesAnIdentityItCannotHaveIssued(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFreshNote(t, root, "# Watched\n\nBody.\n")
	srv := newServer(t, root)

	for _, tt := range []struct {
		name     string
		identity string
	}{
		{name: "absent", identity: ""},
		{name: "too short", identity: "abcdef"},
		{name: "right width, not hex", identity: strings.Repeat("z", 64)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code, _ := askFreshness(t, srv.URL, tt.identity)
			if code != http.StatusBadRequest {
				t.Errorf("status for a %s identity = %d, want %d", tt.name, code, http.StatusBadRequest)
			}
		})
	}
}

// TestReadingPageSendsNoBannerOfItsOwn holds the shape of the whole feature: the
// notice is the client's, built only once the server has said the file moved on.
// A server that rendered it and left the client to hide it would leave a reader
// without scripting looking at a permanent claim that the page is out of date.
// The watch attributes are asserted alongside, so an absent banner cannot be
// read as proof when the real cause is a feature that is not wired at all.
func TestReadingPageSendsNoBannerOfItsOwn(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFreshNote(t, root, "# Watched\n\nThe words a reader is reading.\n")
	srv := newServer(t, root)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/notes/"+freshRel, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest error = %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET the reading page error = %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("Body.Close() error = %v", closeErr)
		}
	}()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body error = %v", err)
	}
	page := string(raw)

	if !strings.Contains(page, "data-freshness-path=") {
		t.Fatal("the reading page carries no watch attributes, so an absent banner proves nothing")
	}
	if strings.Contains(page, "y-freshness") {
		t.Error("the server rendered the freshness banner; it must exist only after the client is told the file moved on")
	}
}

// TestFreshnessNeverCallsAnUnfindableNoteGone is the other half of the refusal
// above, and it is a different code path: this one never reaches the file at
// all. A folder that cannot be searched makes the note unfindable rather than
// absent, and the two must not be reported as the same thing — a reader whose
// permissions slipped for a moment has not lost a note.
func TestFreshnessNeverCallsAnUnfindableNoteGone(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const body = "# Watched\n\nStill on disk.\n"
	writeFreshNote(t, root, body)
	srv := newServer(t, root)

	// The folder is closed after the generation was built, so the note is in
	// the snapshot and out of reach at once.
	folder := filepath.Dir(filepath.Join(root, filepath.FromSlash(freshRel)))
	if err := os.Chmod(folder, 0o000); err != nil {
		t.Fatalf("chmod folder: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(folder, 0o750); err != nil { // #nosec G302 -- a directory needs its search bit; this restores the mode the test removed
			t.Errorf("restore folder mode: %v", err)
		}
	})
	if _, err := os.ReadFile(filepath.Join(folder, "watched.md")); err == nil { // #nosec G304 -- probing a path inside this test's own TempDir
		t.Skip("mode 000 does not block a directory here (running as a privileged user)")
	}

	code, got := askFreshness(t, srv.URL, identityOf(body))
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got == "gone" {
		t.Fatal("a note behind a closed folder is reported as removed; not being able to look is not the same as nothing being there")
	}
	if got != "unreadable" {
		t.Errorf("freshness of a note behind a closed folder = %q, want %q", got, "unreadable")
	}
}

// The transclusion fixture: a watched host that embeds one section of a
// second note, so the host page's words include an excerpt whose source file
// the host's own identity knows nothing about.
const (
	embedSourceRel  = "Writing/source.md"
	embedHostBody   = "# Watched\n\nHost words.\n\n![[source#Sec]]\n"
	embedSourceBody = "intro\n\n# Sec\n\nInside words.\n\n# Tail\n\nOutside words.\n"
)

// newEmbedServer builds a server over the watched host and one embedded
// source, the source's body chosen per test so two servers can stand for two
// generations of the same vault.
func newEmbedServer(t *testing.T, sourceBody string) *httptest.Server {
	t.Helper()
	root := t.TempDir()
	writeFreshNote(t, root, embedHostBody)
	writeVaultNote(t, root, embedSourceRel, sourceBody)
	return newServer(t, root)
}

// writeVaultNote puts body at rel under root, creating the folders it needs.
func writeVaultNote(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", full, err)
	}
}

var embedsStampRe = regexp.MustCompile(`data-freshness-embeds="([0-9a-f]{64})"`)

// pageEmbedsStamp reads the transcluded stamp off the watched host's own page,
// the way the polling client does.
func pageEmbedsStamp(t *testing.T, srvURL string) string {
	t.Helper()
	code, page := get(t, srvURL+"/notes/"+freshRel)
	if code != http.StatusOK {
		t.Fatalf("GET the reading page status = %d, want %d", code, http.StatusOK)
	}
	m := embedsStampRe.FindStringSubmatch(page)
	if m == nil {
		t.Fatal("the reading page carries no transcluded stamp; the asks below would compare nothing")
	}
	return m[1]
}

// askFreshnessEmbeds is the ask a page carrying a transcluded stamp makes:
// identity and stamp together, status left uncarried to keep the comparison
// under test alone.
func askFreshnessEmbeds(t *testing.T, srvURL, identity, embeds string) (code int, answer string) {
	t.Helper()
	return askFreshnessURL(t,
		srvURL+"/freshness/"+freshRel+"?identity="+identity+"&embeds="+url.QueryEscape(embeds))
}

// TestReadingPageStampsWhatItTranscluded pins the stamp's presence to the
// page's actual pull: a host that embeds another note's section carries the
// digest, and a note that embeds nothing carries no such attribute — so the
// narrow ask, and the endpoint work that answers it, exist only where an
// excerpt does.
func TestReadingPageStampsWhatItTranscluded(t *testing.T) {
	t.Parallel()
	srv := newEmbedServer(t, embedSourceBody)
	stamp := pageEmbedsStamp(t, srv.URL)
	if len(stamp) != 64 {
		t.Errorf("transcluded stamp = %q, want 64 hex digits", stamp)
	}

	plainRoot := t.TempDir()
	writeFreshNote(t, plainRoot, "# Watched\n\nNo excerpt here.\n")
	plainSrv := newServer(t, plainRoot)
	code, page := get(t, plainSrv.URL+"/notes/"+freshRel)
	if code != http.StatusOK {
		t.Fatalf("GET the plain reading page status = %d, want %d", code, http.StatusOK)
	}
	if strings.Contains(page, "data-freshness-embeds") {
		t.Error("a page that transcluded nothing stamps a transcluded identity; the attribute must be absent")
	}
}

// TestFreshnessAnnouncesAnEmbeddedSourceEdit is the defect this stamp exists
// to close: the host file never changed, its identity still matches, and the
// words on the page are stale anyway because they were pulled in from a
// source that moved on. Two servers stand for the two generations — the one
// the page rendered under, and the one a reload would render from.
func TestFreshnessAnnouncesAnEmbeddedSourceEdit(t *testing.T) {
	t.Parallel()
	before := newEmbedServer(t, embedSourceBody)
	after := newEmbedServer(t, strings.Replace(embedSourceBody, "Inside words.", "Inside words, changed.", 1))
	pageStamp := pageEmbedsStamp(t, before.URL)
	freshStamp := pageEmbedsStamp(t, after.URL)
	if pageStamp == freshStamp {
		t.Fatal("editing the embedded section did not move the stamp, so these servers cannot stand for two generations")
	}

	code, got := askFreshnessEmbeds(t, after.URL, identityOf(embedHostBody), pageStamp)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "stale" {
		t.Errorf("freshness of a page whose embedded section moved on = %q, want %q", got, "stale")
	}

	code, got = askFreshnessEmbeds(t, after.URL, identityOf(embedHostBody), freshStamp)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "unchanged" {
		t.Errorf("freshness of a page already showing the current excerpt = %q, want %q", got, "unchanged")
	}
}

// TestFreshnessLeavesTheStampStillWhereTheExcerptDidNotMove is the honesty
// half: the page consumed one section, so an edit elsewhere in the source
// note changes nothing a reload would show, and the answer must stay
// unchanged — an offer that returns the same words teaches a reader to stop
// believing the banner.
func TestFreshnessLeavesTheStampStillWhereTheExcerptDidNotMove(t *testing.T) {
	t.Parallel()
	before := newEmbedServer(t, embedSourceBody)
	outside := newEmbedServer(t, strings.Replace(embedSourceBody, "Outside words.", "Outside words, changed.", 1))
	pageStamp := pageEmbedsStamp(t, before.URL)
	if outsideStamp := pageEmbedsStamp(t, outside.URL); outsideStamp != pageStamp {
		t.Fatalf("an edit outside the embedded section moved the stamp (%q then %q)", pageStamp, outsideStamp)
	}

	code, got := askFreshnessEmbeds(t, outside.URL, identityOf(embedHostBody), pageStamp)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "unchanged" {
		t.Errorf("freshness after an edit outside the consumed excerpt = %q, want %q", got, "unchanged")
	}
}

// TestFreshnessAnnouncesARemovedEmbeddedSource pins the other way an excerpt
// dies: the source file leaves the vault, the embed degrades to a citation
// that resolves nothing, and a reload shows that instead of the words. The
// host's own identity still matches throughout.
func TestFreshnessAnnouncesARemovedEmbeddedSource(t *testing.T) {
	t.Parallel()
	before := newEmbedServer(t, embedSourceBody)
	pageStamp := pageEmbedsStamp(t, before.URL)

	goneRoot := t.TempDir()
	writeFreshNote(t, goneRoot, embedHostBody)
	goneSrv := newServer(t, goneRoot)

	code, got := askFreshnessEmbeds(t, goneSrv.URL, identityOf(embedHostBody), pageStamp)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "stale" {
		t.Errorf("freshness of a page whose embedded source left the vault = %q, want %q", got, "stale")
	}
}

// TestFreshnessComparesTranscludedOnlyWhenCarried keeps the ask that carries
// no stamp — the recovery page's, and every page that transcluded nothing —
// on the identity alone. Run against the very fixture whose stamp comparison
// answers stale, this also proves that answer came from the stamp rather
// than from anything else these servers disagree about.
func TestFreshnessComparesTranscludedOnlyWhenCarried(t *testing.T) {
	t.Parallel()
	after := newEmbedServer(t, strings.Replace(embedSourceBody, "Inside words.", "Inside words, changed.", 1))

	code, got := askFreshness(t, after.URL, identityOf(embedHostBody))
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "unchanged" {
		t.Errorf("freshness asked with no transcluded stamp = %q, want %q", got, "unchanged")
	}
}

// TestFreshnessRefusesATranscludedStampItCannotHaveIssued mirrors the
// identity's own strictness: the stamp is one digest in lower-case hex, and
// anything else is a caller's mistake answered as one, never compared
// leniently into a false invitation.
func TestFreshnessRefusesATranscludedStampItCannotHaveIssued(t *testing.T) {
	t.Parallel()
	srv := newEmbedServer(t, embedSourceBody)
	for _, tt := range []struct {
		name   string
		embeds string
	}{
		{name: "present but empty", embeds: ""},
		{name: "too short", embeds: "abcdef"},
		{name: "right width, not hex", embeds: strings.Repeat("z", 64)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code, _ := askFreshnessEmbeds(t, srv.URL, identityOf(embedHostBody), tt.embeds)
			if code != http.StatusBadRequest {
				t.Errorf("status for a %s stamp = %d, want %d", tt.name, code, http.StatusBadRequest)
			}
		})
	}
}

// TestFreshnessHoldsTranscludedNewsBehindThePublishedGate pins the order of
// the comparisons, the same order the status stamp obeys: while the disk has
// moved past the published generation, a reload would render the old bytes
// again, so a stamp difference must not talk over the preparing answer.
func TestFreshnessHoldsTranscludedNewsBehindThePublishedGate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFreshNote(t, root, embedHostBody)
	writeVaultNote(t, root, embedSourceRel, embedSourceBody)
	// The generation is built here and never rebuilt in this test, so the
	// host rewrite below leaves the disk ahead of it.
	srv := newServer(t, root)
	stamp := pageEmbedsStamp(t, srv.URL)
	writeFreshNote(t, root, embedHostBody+"\nA line the generation has not seen.\n")

	code, got := askFreshnessEmbeds(t, srv.URL, identityOf(embedHostBody), stamp)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "preparing" {
		t.Errorf("freshness with the generation behind the disk = %q, want %q", got, "preparing")
	}
}

// governedFreshBody is a lesson the note testdata contract governs, so the
// reading page reads its status live, offers its transitions, and stamps the
// status it printed.
func governedFreshBody(noteStatus string) string {
	return "---\n" +
		"title: watched\n" +
		"type: lesson\n" +
		"domain: japanese\n" +
		"status: " + noteStatus + "\n" +
		"created: 2026-06-01\n" +
		"updated: 2026-06-01\n" +
		"---\n" +
		"\nBody.\n"
}

// TestReadingPageStampsTheStatusItPrinted pins what the polling asks are built
// from: the watch attributes name the path, the identity of the bytes, and the
// status printed beside the title — the identity alone leaves that one value
// out, so without the stamp a flip on disk would never reach an open page.
func TestReadingPageStampsTheStatusItPrinted(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFreshNote(t, root, governedFreshBody("draft"))
	srv := newServerWithContract(t, root, loadContract(t))

	code, page := get(t, srv.URL+"/notes/"+freshRel)
	if code != http.StatusOK {
		t.Fatalf("GET the reading page status = %d, want %d", code, http.StatusOK)
	}
	if !strings.Contains(page, `data-freshness-status="draft"`) {
		t.Errorf("the reading page does not stamp the status it printed; want %q", `data-freshness-status="draft"`)
	}
}

// TestReadingPageStampsAnEmptyStatusForAStatuslessNote pins the stamp's shape
// for a note that prints no status at all: the attribute is present and empty —
// a claim that nothing is printed — rather than absent. Absent is the recovery
// page's shape, and it means the ask must not cover status at all.
func TestReadingPageStampsAnEmptyStatusForAStatuslessNote(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFreshNote(t, root, "# Watched\n\nBody.\n")
	srv := newServer(t, root)

	code, page := get(t, srv.URL+"/notes/"+freshRel)
	if code != http.StatusOK {
		t.Fatalf("GET the reading page status = %d, want %d", code, http.StatusOK)
	}
	if !strings.Contains(page, `data-freshness-status=""`) {
		t.Errorf("a statusless note does not stamp the empty claim; want %q", `data-freshness-status=""`)
	}
}

// TestOwnFlipRefreshesTheStampThroughTheRedirect walks the write path's own
// round trip: the flip lands, the 303 re-renders the page, and the fresh page
// stamps the status it now prints beside the identity the flip preserved — so
// its polling resumes with the new pair and hears unchanged rather than an
// invitation to reload into the words already on screen.
func TestOwnFlipRefreshesTheStampThroughTheRedirect(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	draft := governedFreshBody("draft")
	writeFreshNote(t, root, draft)
	srv := newServerWithContract(t, root, loadContract(t))
	identity := identityOf(draft)

	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	form := url.Values{
		"path":             {freshRel},
		"from":             {"draft"},
		"to":               {"ready"},
		"content_identity": {identity},
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		srv.URL+"/status", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /status: %v", err)
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Errorf("close response body: %v", closeErr)
	}
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /status status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	location := resp.Header.Get("Location")
	if location == "" {
		t.Fatal("POST /status set no Location; the redirect is the page's way back")
	}

	code, page := get(t, srv.URL+location)
	if code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want %d", location, code, http.StatusOK)
	}
	if !strings.Contains(page, `data-freshness-status="ready"`) {
		t.Errorf("the re-rendered page does not stamp the status the flip installed; want %q", `data-freshness-status="ready"`)
	}
	// A flip rewrites the one value the identity leaves out, so the identity
	// the page carries is the one it carried before.
	if !strings.Contains(page, `data-freshness-identity="`+identity+`"`) {
		t.Errorf("the re-rendered page does not carry the identity the flip preserved; want %q", identity)
	}

	code, got := askFreshnessWithStatus(t, srv.URL, identity, "ready")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "unchanged" {
		t.Errorf("freshness right after the page's own flip re-rendered it = %q, want %q", got, "unchanged")
	}
}
