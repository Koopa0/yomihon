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
	"github.com/koopa0/yomihon/internal/wording"
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
// seconds, and returns the status code beside the answer.
func askFreshness(t *testing.T, client *http.Client, srvURL, identity string) (code int, answer string) {
	t.Helper()
	return askFreshnessURL(t, client, srvURL+"/freshness/"+freshRel+"?identity="+identity)
}

func askFreshnessURL(t *testing.T, client *http.Client, full string) (code int, answer string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, full, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest error = %v", err)
	}
	resp, err := client.Do(req)
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

	code, got := askFreshness(t, srv.Client(), srv.URL, identityOf(body))
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
	code, got := askFreshness(t, srv.Client(), srv.URL, identityOf("# Watched\n\nAn older draft.\n"))
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "stale" {
		t.Fatalf("freshness with the generation caught up = %q, want %q", got, "stale")
	}

	// Now the disk moves ahead of the generation. Reloading would render the
	// same bytes again, so the invitation is withheld.
	writeFreshNote(t, root, "# Watched\n\nJust saved in Obsidian.\n")
	code, got = askFreshness(t, srv.Client(), srv.URL, identityOf(rendered))
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
	code, got := askFreshness(t, srv.Client(), srv.URL, identityOf(body))
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "gone" {
		t.Errorf("freshness of a removed note = %q, want %q", got, "gone")
	}
}

// TestFreshnessNeverCallsAnUnreadableFileGone holds the line that what could
// not be confirmed does not become a confirmed fact. A
// passing read failure must not reach a reader as a deletion.
func TestFreshnessNeverCallsAnUnreadableFileGone(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const body = "# Watched\n\nStill on disk.\n"
	writeFreshNote(t, root, body)
	srv := newServer(t, root)
	lockNote(t, filepath.Join(root, filepath.FromSlash(freshRel)))

	code, got := askFreshness(t, srv.Client(), srv.URL, identityOf(body))
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

// TestFreshnessAnnouncesAFrontmatterOnlyEdit pins that the identity covers
// every byte: an author who removes the draft flag and touches nothing else
// has changed the note, and a page left open is told so once a reload would
// show it.
func TestFreshnessAnnouncesAFrontmatterOnlyEdit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const rendered = "---\ntype: lesson\ndraft: true\n---\n\nBody.\n"
	writeFreshNote(t, root, "---\ntype: lesson\n---\n\nBody.\n")
	srv := newServer(t, root)

	code, got := askFreshness(t, srv.Client(), srv.URL, identityOf(rendered))
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "stale" {
		t.Errorf("freshness after the draft flag alone was removed = %q, want %q", got, "stale")
	}
}

// TestReadingPageStampsNoStatusBesideItsIdentity pins the shape of the ask:
// the identity is the whole claim, so the page carries no second value for a
// field the identity already covers.
func TestReadingPageStampsNoStatusBesideItsIdentity(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const body = "---\ntype: lesson\n---\n# watched\n\nBody.\n"
	writeFreshNote(t, root, body)
	srv := newServerWithContract(t, root, loadContract(t))

	code, page := get(t, srv.Client(), srv.URL+"/notes/"+freshRel)
	if code != http.StatusOK {
		t.Fatalf("GET the reading page status = %d, want %d", code, http.StatusOK)
	}
	if !strings.Contains(page, `data-freshness-identity="`+identityOf(body)+`"`) {
		t.Errorf("the reading page does not stamp the identity of every byte it rendered")
	}
	if strings.Contains(page, "data-freshness-status") {
		t.Error("the reading page still stamps a status beside its identity")
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
			code, _ := askFreshness(t, srv.Client(), srv.URL, tt.identity)
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
	resp, err := srv.Client().Do(req)
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

	code, got := askFreshness(t, srv.Client(), srv.URL, identityOf(body))
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
func pageEmbedsStamp(t *testing.T, client *http.Client, srvURL string) string {
	t.Helper()
	code, page := get(t, client, srvURL+"/notes/"+freshRel)
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
func askFreshnessEmbeds(t *testing.T, client *http.Client, srvURL, identity, embeds string) (code int, answer string) {
	t.Helper()
	return askFreshnessURL(t, client,
		srvURL+"/freshness/"+freshRel+"?identity="+identity+"&embeds="+url.QueryEscape(embeds))
}

// TestReadingPageStampsWhatItTranscluded pins the stamp's presence to the
// page's own citations: a host that embeds another note's section carries the
// digest, and a note that embeds nothing carries no such attribute — so the
// narrow ask, and the endpoint work that answers it, exist only where an embed
// does.
func TestReadingPageStampsWhatItTranscluded(t *testing.T) {
	t.Parallel()
	srv := newEmbedServer(t, embedSourceBody)
	stamp := pageEmbedsStamp(t, srv.Client(), srv.URL)
	if len(stamp) != 64 {
		t.Errorf("transcluded stamp = %q, want 64 hex digits", stamp)
	}

	plainRoot := t.TempDir()
	writeFreshNote(t, plainRoot, "# Watched\n\nNo excerpt here.\n")
	plainSrv := newServer(t, plainRoot)
	code, page := get(t, plainSrv.Client(), plainSrv.URL+"/notes/"+freshRel)
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
	pageStamp := pageEmbedsStamp(t, before.Client(), before.URL)
	freshStamp := pageEmbedsStamp(t, after.Client(), after.URL)
	if pageStamp == freshStamp {
		t.Fatal("editing the embedded section did not move the stamp, so these servers cannot stand for two generations")
	}

	code, got := askFreshnessEmbeds(t, after.Client(), after.URL, identityOf(embedHostBody), pageStamp)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "stale" {
		t.Errorf("freshness of a page whose embedded section moved on = %q, want %q", got, "stale")
	}

	code, got = askFreshnessEmbeds(t, after.Client(), after.URL, identityOf(embedHostBody), freshStamp)
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
	pageStamp := pageEmbedsStamp(t, before.Client(), before.URL)
	if outsideStamp := pageEmbedsStamp(t, outside.Client(), outside.URL); outsideStamp != pageStamp {
		t.Fatalf("an edit outside the embedded section moved the stamp (%q then %q)", pageStamp, outsideStamp)
	}

	code, got := askFreshnessEmbeds(t, outside.Client(), outside.URL, identityOf(embedHostBody), pageStamp)
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
	pageStamp := pageEmbedsStamp(t, before.Client(), before.URL)

	goneRoot := t.TempDir()
	writeFreshNote(t, goneRoot, embedHostBody)
	goneSrv := newServer(t, goneRoot)

	code, got := askFreshnessEmbeds(t, goneSrv.Client(), goneSrv.URL, identityOf(embedHostBody), pageStamp)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "stale" {
		t.Errorf("freshness of a page whose embedded source left the vault = %q, want %q", got, "stale")
	}
}

// newMissingEmbedServer builds a server over the watched host alone, so its
// embed names a note nobody has written and the page says so where the excerpt
// would have stood.
func newMissingEmbedServer(t *testing.T) *httptest.Server {
	t.Helper()
	root := t.TempDir()
	writeFreshNote(t, root, embedHostBody)
	return newServer(t, root)
}

// embedSourceWithoutTheAddress is the source note written without the section
// the host's embed names: the note is there, and none of it is shown.
const embedSourceWithoutTheAddress = "intro\n\n# Tail\n\nOutside words.\n"

// TestFreshnessAnnouncesAMissingEmbedTargetArriving is what an open page owes a
// reader whose agent is writing while they read: the page opened saying there
// is no such note, someone wrote it, and the words are there to be had. The
// host file never changed, so nothing but the stamp can carry that news.
func TestFreshnessAnnouncesAMissingEmbedTargetArriving(t *testing.T) {
	t.Parallel()
	missing := newMissingEmbedServer(t)
	pageStamp := pageEmbedsStamp(t, missing.Client(), missing.URL)

	after := newEmbedServer(t, embedSourceBody)
	code, got := askFreshnessEmbeds(t, after.Client(), after.URL, identityOf(embedHostBody), pageStamp)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "stale" {
		t.Errorf("freshness of a page whose missing embed target was written = %q, want %q", got, "stale")
	}

	// A page opened against that same generation is level with it, so the
	// answer above came from the stamp moving and not from the comparison
	// having been made to answer stale for every page carrying one.
	code, got = askFreshnessEmbeds(t, after.Client(), after.URL, identityOf(embedHostBody),
		pageEmbedsStamp(t, after.Client(), after.URL))
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "unchanged" {
		t.Errorf("freshness of a page already showing the excerpt = %q, want %q", got, "unchanged")
	}
}

// TestFreshnessSeparatesAMissingNoteFromAMissingAddress holds the two absences
// apart. A note written without the section the embed names moves the page from
// one sentence to the other, and a reader who is told a note does not exist,
// then told the note exists and this address in it does not, has been told two
// different things — so a stamp that collapsed them would leave the second
// unsaid, and the address arriving after that would be compared against the
// wrong state.
func TestFreshnessSeparatesAMissingNoteFromAMissingAddress(t *testing.T) {
	t.Parallel()
	missing := newMissingEmbedServer(t)
	withoutAddress := newEmbedServer(t, embedSourceWithoutTheAddress)
	expanded := newEmbedServer(t, embedSourceBody)

	noNote := pageEmbedsStamp(t, missing.Client(), missing.URL)
	noAddress := pageEmbedsStamp(t, withoutAddress.Client(), withoutAddress.URL)
	whole := pageEmbedsStamp(t, expanded.Client(), expanded.URL)
	if noNote == noAddress || noNote == whole || noAddress == whole {
		t.Fatalf("the three states a page can show for one embed stamped %q, %q and %q; two of them are one digest",
			noNote, noAddress, whole)
	}

	// The note arrives without the address the embed names: the page's sentence
	// changes, so its answer has to.
	code, got := askFreshnessEmbeds(t, withoutAddress.Client(), withoutAddress.URL, identityOf(embedHostBody), noNote)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "stale" {
		t.Errorf("freshness of a page told there is no such note, once the note exists without the address = %q, want %q",
			got, "stale")
	}

	// The address is written into the note that was already there.
	code, got = askFreshnessEmbeds(t, expanded.Client(), expanded.URL, identityOf(embedHostBody), noAddress)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "stale" {
		t.Errorf("freshness of a page whose withheld address was written = %q, want %q", got, "stale")
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

	code, got := askFreshness(t, after.Client(), after.URL, identityOf(embedHostBody))
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
			code, _ := askFreshnessEmbeds(t, srv.Client(), srv.URL, identityOf(embedHostBody), tt.embeds)
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
	stamp := pageEmbedsStamp(t, srv.Client(), srv.URL)
	writeFreshNote(t, root, embedHostBody+"\nA line the generation has not seen.\n")

	code, got := askFreshnessEmbeds(t, srv.Client(), srv.URL, identityOf(embedHostBody), stamp)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}
	if got != "preparing" {
		t.Errorf("freshness with the generation behind the disk = %q, want %q", got, "preparing")
	}
}

// TestTheFreshnessEndpointRefusesInTheReadersLanguage covers the three ways
// this endpoint answers nothing. A page never reads these bodies — the client
// stops at the status — so the only person who ever meets one arrived by
// assembling the address, and answering them in the router's own English, or
// in a language the reader has already switched away from, is the same
// discourtesy every page here was built to stop.
func TestTheFreshnessEndpointRefusesInTheReadersLanguage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFreshNote(t, root, "# Watched\n\nBody.\n")
	srv := newServer(t, root)
	good := identityOf("# Watched\n\nBody.\n")

	tests := []struct {
		name   string
		target string
		code   int
		want   string
	}{
		{
			name:   "an address that names no note to watch",
			target: srv.URL + "/freshness/Writing/absent.txt?identity=" + good,
			code:   http.StatusNotFound,
			want:   wording.FreshnessNotWatchable.In(wording.En),
		},
		{
			name:   "an identity that is not a digest",
			target: srv.URL + "/freshness/" + freshRel + "?identity=nonsense",
			code:   http.StatusBadRequest,
			want:   "identity must be 64 hex digits",
		},
		{
			name:   "an embed stamp that is not a digest",
			target: srv.URL + "/freshness/" + freshRel + "?identity=" + good + "&embeds=nonsense",
			code:   http.StatusBadRequest,
			want:   "embeds must be 64 hex digits",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code, body := askFreshnessInLanguage(t, srv.Client(), tt.target, wording.En)
			if code != tt.code {
				t.Fatalf("status = %d, want %d (body %q)", code, tt.code, body)
			}
			if !strings.Contains(body, tt.want) {
				t.Errorf("body = %q, want it to contain %q", body, tt.want)
			}
			if strings.Contains(body, "404 page not found") {
				t.Errorf("body = %q, which is the router's own English", body)
			}
		})
	}
}

// askFreshnessInLanguage asks as a reader who has chosen one, which is the
// only way the wiring under test is reached at all.
func askFreshnessInLanguage(t *testing.T, client *http.Client, full string, lang wording.Lang) (code int, answer string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, full, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest error = %v", err)
	}
	req.Header.Set("Cookie", wording.CookieName+"="+string(lang))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s error = %v", full, err)
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
