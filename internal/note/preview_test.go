package note_test

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// The excerpt fixtures. Each sentinel is a phrase no other line carries, so
// asking whether a response holds one is asking exactly where the cut landed
// and never whether two passages happen to share words.
const (
	previewBeforeSentinel = "SENTINEL the opening words above every section"
	previewInsideSentinel = "SENTINEL the words the addressed section owns"
	previewAfterSentinel  = "SENTINEL the words the section after it owns"
	previewBlockSentinel  = "SENTINEL the words the marked block owns"
)

// previewTarget is the note a card is opened on: two sections named plainly,
// an opening passage belonging to neither, and a block address, so a cut can
// be observed from all four sides.
const previewTarget = "---\ntitle: Target\n---\n\n" +
	previewBeforeSentinel + "\n\n" +
	"## Addressed\n\n" +
	previewInsideSentinel + "\n\n" +
	previewBlockSentinel + " ^marked\n\n" +
	"## The next one\n\n" +
	previewAfterSentinel + "\n"

const previewTargetRel = "Notes/target.md"

// writePreviewVault lays down the note a card is opened on and the note that
// embeds the same address, so the two cuts can be compared against each other
// rather than against a slice this file worked out on its own.
func writePreviewVault(t *testing.T, root string) {
	t.Helper()
	writeVaultNote(t, root, previewTargetRel, previewTarget)
	writeVaultNote(t, root, "Notes/host.md",
		"---\ntitle: Host\n---\n\nThe host body.\n\n![[target#Addressed]]\n")
}

// previewResponse is one card's answer, read whole.
type previewResponse struct {
	code    int
	body    string
	cache   string
	content string
}

// askPreview performs the request one hover makes. section is the fragment the
// hovered link's own address carries; empty asks for the note itself.
func askPreview(t *testing.T, srvURL, rel, section string, lang wording.Lang) previewResponse {
	t.Helper()
	full := srvURL + "/preview/" + rel
	if section != "" {
		full += "?section=" + url.QueryEscape(section)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, full, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest error = %v", err)
	}
	req.Header.Set("Cookie", wording.CookieName+"="+string(lang))
	resp, err := http.DefaultClient.Do(req)
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
	return previewResponse{
		code:    resp.StatusCode,
		body:    string(body),
		cache:   resp.Header.Get("Cache-Control"),
		content: resp.Header.Get("Content-Type"),
	}
}

// getPage reads a whole page, for the embed the card's cut is compared with.
func getPage(t *testing.T, full string) (code int, page string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, full, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest error = %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
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

// TestTheCardShowsTheExcerptAnEmbedOfTheSameAddressShows is the lock that keeps
// one rule for cutting a section. The card and the embed are two callers of it,
// and the cheapest way for them to diverge is for one of them to grow its own
// copy — so neither is compared against a slice written down here. They are
// compared against each other, sentinel by sentinel.
//
// It is a lock on the cut, not on what either does with an address that matches
// nothing: the embed widens to the whole note and says so in the article, and
// the card refuses. The address here is one both find.
func TestTheCardShowsTheExcerptAnEmbedOfTheSameAddressShows(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writePreviewVault(t, root)
	srv := newServer(t, root)

	card := askPreview(t, srv.URL, previewTargetRel, "addressed", wording.ZhHant)
	if card.code != http.StatusOK {
		t.Fatalf("card status = %d, want %d; body = %s", card.code, http.StatusOK, card.body)
	}
	code, host := getPage(t, srv.URL+"/notes/Notes/host.md")
	if code != http.StatusOK {
		t.Fatalf("host page status = %d, want %d", code, http.StatusOK)
	}
	// Without this the agreement below would hold over a page that expanded
	// nothing, where both sides carry none of the target's words.
	if !strings.Contains(host, previewInsideSentinel) {
		t.Fatalf("the host page expanded no excerpt of the target, so there is no cut to compare with:\n%s", host)
	}

	for _, sentinel := range []string{previewBeforeSentinel, previewInsideSentinel, previewAfterSentinel} {
		inCard := strings.Contains(card.body, sentinel)
		inEmbed := strings.Contains(host, sentinel)
		if inCard != inEmbed {
			t.Errorf("the card and an embed of the same address disagree about %q: card holds it = %t, embed holds it = %t",
				sentinel, inCard, inEmbed)
		}
	}
}

// TestTheCardCutsAtTheSectionTheLinkAddressed states the cut in its own right,
// so the agreement above cannot pass by both sides showing the whole note.
func TestTheCardCutsAtTheSectionTheLinkAddressed(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writePreviewVault(t, root)
	srv := newServer(t, root)

	card := askPreview(t, srv.URL, previewTargetRel, "addressed", wording.ZhHant)
	if card.code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", card.code, http.StatusOK, card.body)
	}
	if !strings.Contains(card.body, previewInsideSentinel) {
		t.Errorf("the card does not carry the addressed section's own words:\n%s", card.body)
	}
	for _, outside := range []string{previewBeforeSentinel, previewAfterSentinel} {
		if strings.Contains(card.body, outside) {
			t.Errorf("the card carries %q, which sits outside the section the link addressed:\n%s", outside, card.body)
		}
	}
}

// TestACardWithNoSectionShowsTheNoteFromTheTop covers the link written at a
// whole note, which is most of them.
func TestACardWithNoSectionShowsTheNoteFromTheTop(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writePreviewVault(t, root)
	srv := newServer(t, root)

	card := askPreview(t, srv.URL, previewTargetRel, "", wording.ZhHant)
	if card.code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", card.code, http.StatusOK, card.body)
	}
	for _, sentinel := range []string{previewBeforeSentinel, previewInsideSentinel, previewAfterSentinel} {
		if !strings.Contains(card.body, sentinel) {
			t.Errorf("a card asked for the whole note is missing %q:\n%s", sentinel, card.body)
		}
	}
}

// TestACardOnABlockAddressShowsThatBlock covers the other fragment a link can
// carry. The address arrives folded, with its caret, exactly as the anchor
// stamped it.
func TestACardOnABlockAddressShowsThatBlock(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writePreviewVault(t, root)
	srv := newServer(t, root)

	card := askPreview(t, srv.URL, previewTargetRel, "^marked", wording.ZhHant)
	if card.code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", card.code, http.StatusOK, card.body)
	}
	if !strings.Contains(card.body, previewBlockSentinel) {
		t.Errorf("the card does not carry the marked block's own words:\n%s", card.body)
	}
	if strings.Contains(card.body, previewAfterSentinel) {
		t.Errorf("the card reaches past the marked block:\n%s", card.body)
	}
}

// TestAnAddressThatNamesNothingAnswersWithASentenceNotAnEmptyCard keeps the
// card's one refusal readable. A card that opens empty, or does not open at
// all, is indistinguishable from a hover the page never registered.
func TestAnAddressThatNamesNothingAnswersWithASentenceNotAnEmptyCard(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writePreviewVault(t, root)
	writeVaultNote(t, root, "Notes/plain.txt", "not markdown at all\n")
	writeVaultNote(t, root, ".obsidian/workspace.md", "---\ntitle: Hidden\n---\n\nNot served.\n")
	srv := newServer(t, root)

	for _, tt := range []struct {
		name string
		rel  string
	}{
		{name: "a path this generation holds no note for", rel: "Notes/absent.md"},
		{name: "a path that is not markdown", rel: "Notes/plain.txt"},
		{name: "a path under a dot-leading folder", rel: ".obsidian/workspace.md"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			card := askPreview(t, srv.URL, tt.rel, "", wording.ZhHant)
			if card.code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", card.code, http.StatusNotFound)
			}
			if !strings.Contains(card.body, wording.PreviewNoNote.In(wording.ZhHant)) {
				t.Errorf("the card says nothing about why it is empty:\n%s", card.body)
			}
		})
	}
}

// TestACardAnsweringInTheReadersLanguage keeps the card's own sentence out of
// the script and inside the dictionary, where both languages of it live.
func TestACardAnsweringInTheReadersLanguage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writePreviewVault(t, root)
	srv := newServer(t, root)

	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		card := askPreview(t, srv.URL, "Notes/absent.md", "", lang)
		if !strings.Contains(card.body, wording.PreviewNoNote.In(lang)) {
			t.Errorf("a reader who asked for %s is answered in another language:\n%s", lang, card.body)
		}
		other := wording.En
		if lang == wording.En {
			other = wording.ZhHant
		}
		if strings.Contains(card.body, wording.PreviewNoNote.In(other)) {
			t.Errorf("a reader who asked for %s is also handed the %s sentence:\n%s", lang, other, card.body)
		}
	}
}

// TestTheExcerptKeepsTheLanguageItsAuthorWroteIt keeps a reading voice from
// announcing one language's words in another's. The card sits on a page whose
// language is its own, and the note it shows need not share it.
func TestTheExcerptKeepsTheLanguageItsAuthorWroteIt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeVaultNote(t, root, "Writing/japanese.md",
		"---\ntitle: Japanese\ntype: writing\ndomain: japanese\nstatus: draft\nlang: ja\n---\n\nHonest body.\n")
	srv := newServerWithContract(t, root, loadContract(t))

	card := askPreview(t, srv.URL, "Writing/japanese.md", "", wording.ZhHant)
	if card.code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", card.code, http.StatusOK, card.body)
	}
	if !strings.Contains(card.body, `lang="ja"`) {
		t.Errorf("the excerpt carries no language of its own, so it is announced as the page's:\n%s", card.body)
	}
}

// TestACardIsNeverAHeldAnswer states the header the live look depends on.
func TestACardIsNeverAHeldAnswer(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writePreviewVault(t, root)
	srv := newServer(t, root)

	card := askPreview(t, srv.URL, previewTargetRel, "", wording.ZhHant)
	if card.cache != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", card.cache, "no-store")
	}
	if !strings.HasPrefix(card.content, "text/html") {
		t.Errorf("Content-Type = %q, want a text/html fragment", card.content)
	}
}

// TestTheCardIsABareFragment holds the shape the client's insertion path
// assumes: the fetched bytes are parsed and one element of them is imported
// into the open page, so a whole document around them would be discarded and a
// script inside them would be an invitation nobody meant to write.
func TestTheCardIsABareFragment(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writePreviewVault(t, root)
	srv := newServer(t, root)

	card := askPreview(t, srv.URL, previewTargetRel, "", wording.ZhHant)
	if !strings.Contains(card.body, "data-preview-body") {
		t.Fatalf("the fragment carries no element the client can import:\n%s", card.body)
	}
	for _, forbidden := range []string{"<script", "<html", "<body", "<!DOCTYPE", "<head"} {
		if strings.Contains(card.body, forbidden) {
			t.Errorf("the fragment carries %q, which a bare fragment has no business holding:\n%s", forbidden, card.body)
		}
	}
}

// TestALongNoteIsCutAndSaysSo keeps the card a taste rather than a transfer,
// and keeps the cut visible: an excerpt that stops without saying so reads as
// the note ending there.
func TestALongNoteIsCutAndSaysSo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const tail = "SENTINEL the words past the budget"
	long := "---\ntitle: Long\n---\n\n" +
		strings.Repeat("A ballast paragraph, one of many, putting bytes between the top of this note and its end.\n\n", 400) +
		tail + "\n"
	writeVaultNote(t, root, "Notes/long.md", long)
	writeVaultNote(t, root, "Notes/short.md", "---\ntitle: Short\n---\n\nA note that fits.\n")
	srv := newServer(t, root)

	cut := askPreview(t, srv.URL, "Notes/long.md", "", wording.ZhHant)
	if cut.code != http.StatusOK {
		t.Fatalf("status = %d, want %d", cut.code, http.StatusOK)
	}
	if strings.Contains(cut.body, tail) {
		t.Errorf("a note past the budget reached the card whole:\n%d bytes", len(cut.body))
	}
	if !strings.Contains(cut.body, wording.PreviewMore.In(wording.ZhHant)) {
		t.Errorf("the card stops short of the note and says nothing about it:\n%s", cut.body)
	}

	whole := askPreview(t, srv.URL, "Notes/short.md", "", wording.ZhHant)
	if strings.Contains(whole.body, wording.PreviewMore.In(wording.ZhHant)) {
		t.Errorf("a card holding the whole note claims it was cut:\n%s", whole.body)
	}
}

// TestASectionTheNoteDoesNotHaveIsRefusedNotWidened holds the card to the place
// the reader named. Showing them the top of the note instead answers a question
// nobody asked, and does it silently: the words that arrive are the note's own,
// so nothing on screen says they are not the section the link promised.
func TestASectionTheNoteDoesNotHaveIsRefusedNotWidened(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writePreviewVault(t, root)
	srv := newServer(t, root)

	for _, tt := range []struct {
		name    string
		section string
		notice  wording.Phrase
	}{
		{name: "a section", section: "nowhere-in-this-note", notice: wording.DiagSectionNote},
		{name: "a block", section: "^nowhere", notice: wording.DiagBlockNote},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			card := askPreview(t, srv.URL, previewTargetRel, tt.section, wording.ZhHant)
			if card.code != http.StatusOK {
				t.Fatalf("status = %d, want %d; the note is there and only the place inside it is not", card.code, http.StatusOK)
			}
			for _, sentinel := range []string{previewBeforeSentinel, previewInsideSentinel, previewAfterSentinel, previewBlockSentinel} {
				if strings.Contains(card.body, sentinel) {
					t.Errorf("the card carries %q, so an address the note does not answer to was widened into words the reader did not ask for:\n%s", sentinel, card.body)
				}
			}
			if !strings.Contains(card.body, tt.notice.In(wording.ZhHant)) {
				t.Errorf("the card does not say the note has no such place:\n%s", card.body)
			}
			if !strings.Contains(card.body, "y-preview__sourcelink") {
				t.Errorf("the card refuses and offers no way on to the note itself:\n%s", card.body)
			}
		})
	}
}

// TestTheCardCarriesNoPlaceAnythingCanBeAddressed keeps the excerpt from
// bringing names into the page that opened it. The card shares one document
// with that page, so every id it carried would be a second element answering to
// a name a heading or a block address there already has, and a fragment naming
// one would reach whichever came first.
func TestTheCardCarriesNoPlaceAnythingCanBeAddressed(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writePreviewVault(t, root)
	srv := newServer(t, root)

	card := askPreview(t, srv.URL, previewTargetRel, "", wording.ZhHant)
	if card.code != http.StatusOK {
		t.Fatalf("status = %d, want %d", card.code, http.StatusOK)
	}
	// The same note as a page of its own, which is where a heading is a place
	// something can be addressed. Without this the check below would hold over
	// a fixture whose headings the renderer never named.
	code, page := getPage(t, srv.URL+"/notes/"+previewTargetRel)
	if code != http.StatusOK {
		t.Fatalf("the note's own page returned %d, want %d", code, http.StatusOK)
	}
	if !strings.Contains(page, `id="addressed"`) {
		t.Fatalf("the note's own page names none of its headings, so there is nothing for the card to have dropped:\n%s", page)
	}
	if strings.Contains(card.body, `id="`) {
		t.Errorf("the card brings a name of its own into the page that opened it:\n%s", card.body)
	}
}

// TestTheCardNamesTheNoteItIsShowing holds the one thing adjacency to the link
// cannot supply. A link written at an alias shows words that are not the note's
// name, and a card cut at a section opens on a heading that says nothing about
// whose section it is — so without this line a reader has no way to confirm
// they are looking at what they aimed for.
func TestTheCardNamesTheNoteItIsShowing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writePreviewVault(t, root)
	srv := newServer(t, root)

	whole := askPreview(t, srv.URL, previewTargetRel, "", wording.ZhHant)
	if whole.code != http.StatusOK {
		t.Fatalf("status = %d, want %d", whole.code, http.StatusOK)
	}
	// The name and the destination in one assertion: a name that is not the way
	// on, or a way on that is not named, each leave the reader where they were.
	const named = `<a class="y-preview__sourcelink" href="/notes/Notes/target.md">Target`
	if !strings.Contains(whole.body, named) {
		t.Errorf("the card does not name the note it is showing as the way on to it; want %s in:\n%s", named, whole.body)
	}
	if strings.Contains(whole.body, "y-preview__section") {
		t.Errorf("a card of a whole note claims a section:\n%s", whole.body)
	}

	section := askPreview(t, srv.URL, previewTargetRel, "addressed", wording.ZhHant)
	if !strings.Contains(section.body, "Addressed") {
		t.Errorf("a card cut at a section does not name that section:\n%s", section.body)
	}
	if !strings.Contains(section.body, "y-preview__section") {
		t.Errorf("the section is not marked as one, so it reads as part of the note's own name:\n%s", section.body)
	}

	block := askPreview(t, srv.URL, previewTargetRel, "^marked", wording.ZhHant)
	if strings.Contains(block.body, "y-preview__section") {
		t.Errorf("a card cut at a block claims a section it has no heading for:\n%s", block.body)
	}
}
