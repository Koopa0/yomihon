package layouts

import (
	"cmp"
	"context"
	"net/http"
	"net/url"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/asset"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/wording"
)

// Chrome is the per-request shell state rendered server-side: the page title
// and the persisted theme / furigana / text-size / typeface / single-key-
// shortcut signals stamped onto the root so the correct state paints on the
// first byte (no FOUC).
// CSS reacts to these attributes (see assets/css/*); the client runtime flips
// the ones its own controls govern, along with their cookies. The
// server-rendered page remains usable without enhancement.
type Chrome struct {
	// SearchQuery is the query a search page answers, written in the header's
	// entry in place of its prompt so the page that holds the answer says what
	// it was asked. Every other page leaves it empty.
	SearchQuery string
	// Title is the page's own title, which the tab shows before the vault's
	// name. It is empty for the vault's own desk, which the tab names by the
	// vault alone.
	Title string
	// Vault is what the vault is called to a reader, which tells one vault's
	// tab from another's. ChromeFromRequest takes it from the request.
	Vault string
	// TitleAlone makes Title the whole document title, for a page that stands
	// before a vault can be asked what it is called.
	TitleAlone bool
	Nonce      string // per-response nonce for application-owned scripts
	// Assets selects the version values in static resource addresses. Its
	// zero value binds them to the real registered bytes.
	Assets asset.Versions
	// Theme is the reader's stored choice: "light", "dark", or "" when they
	// never made one. A choice is stamped on the root so the first paint is
	// already right; no choice leaves the root unstamped and the stylesheet's
	// system-preference block decides.
	Theme string
	// Ruby is whether furigana are shown: "on" or "off". The root carries it so
	// the first paint already matches.
	Ruby string
	// Readings is whether the page in front of the reader carries furigana,
	// and so whether the header offers the switch for them. The page decides
	// it from what it rendered, as it decides Mark; every other page leaves it
	// false and draws no such switch.
	Readings bool
	// Diagrams is whether the page carries a diagram the browser will draw, and
	// so whether its head says how tall each was drawn the last time this tab
	// had it. The page decides it from what it rendered, as it decides
	// Readings.
	Diagrams bool
	// TextSize scales the reading column: "m" (the default measure), "l", or
	// "xl". It tunes presentation only — the charter's bounded reading
	// preferences — and the chrome around the article keeps its own size.
	TextSize string
	// Font is the typeface the reading column is set in: "serif", "sans",
	// "kai", or "" when the reader never chose. A choice is stamped on the
	// root, as the theme is; no choice leaves the attribute off and the
	// stylesheet's base value answers.
	Font string
	// Rail is whether the left navigation column is shown above the drawer
	// width: "open" or "collapsed". The root carries it so the first paint is
	// already the column the reader left; it has no effect where the column is
	// a drawer.
	Rail string
	// SectionButtons is when the buttons beside a section are drawn: "always"
	// or "hover". The root carries it, so a stylesheet answering it needs no
	// script and the first paint is already as the reader left it.
	SectionButtons string
	// Position is whether a note opens where the reader last stopped in it:
	// "on" or "off". The root carries it, so the head can tell before the first
	// frame whether a note is held back for the place it will open at.
	Position                  string
	SingleKeyShortcutsEnabled bool // plain /, [ and ? are enabled
	// Lang is the language the interface speaks for this request, and the
	// language the document declares. The article inside it may declare its
	// own; that answers what the note is written in, which is a different
	// question from what the interface is saying.
	Lang wording.Lang
	// ReturnTo is this request's own address — path and query — carried by the
	// controls the server answers, so the redirect afterwards lands the reader
	// back on the page they were reading. The server validates it as a local
	// path before following it.
	ReturnTo string
	// Path is this request's own path, which is not always where ReturnTo
	// points: the reading choices return to the page that led to them. The
	// header's own destinations read it to say so when the reader is on one.
	Path string
	// Mark is the note this request is reading, where the reader can be
	// offered to keep the place they stopped at. Nil on every page that is not
	// such a note, and the header then draws no such control. The page decides
	// it, because whether a note can be marked is the reading page's question
	// and an answer worked out a second time here would be a second answer.
	Mark *MarkOffer
	// FilePath is the full path of the note file this request is reading, which
	// the header offers to copy with the line of the section being read, for
	// the reader's own editor. It is never the ~ form, because the reader
	// pastes it back. Empty on every page that reads no note, and the header
	// then draws no such control.
	FilePath string
	// Reading is the note this request is reading, named the way the browser's
	// storage names it, so that the place the reader stops at is kept against
	// this note in this vault and nowhere else. Nil on every page that is not
	// a note, and the root then names no note.
	Reading *ReadingNote
}

// ReadingNote is a note as the browser keeps a reader's place in it: the
// vault's id and the note's path inside the vault. The id is there because a
// vault that is removed gives its port to the next vault opened, and an origin
// and a path alone would hand that vault's note of the same name an old place.
// The id is derived from the vault's location and spells none of it.
type ReadingNote struct {
	Vault string
	Path  string
}

// MarkOffer is the note a reading place would be kept in: its address, the
// identity of the bytes the reader is looking at, and the route the control
// posts to. The three travel together because the post needs all of them —
// an address with no identity keeps a place in a note that may since have been
// rewritten, and neither is worth anything without somewhere to send it.
// Kept is whether the place already kept is in this very note, which the
// control draws as the reader's own mark.
type MarkOffer struct {
	Path     string
	Identity string
	Endpoint string
	Kept     bool
}

// documentTitle is the text of the document's <title>: the page's title, then
// the vault's name, so that two vaults open in one window are told apart by the
// end of the tab and two tabs of one vault by its start. A page that is the
// vault itself, or whose title is the vault's name, says it once. It is here
// rather than in the template for the reason singleKeyShortcutsState is.
func documentTitle(title, vault string, alone bool) string {
	switch {
	case alone:
		return title
	case title == "" || title == vault:
		return cmp.Or(vault, "yomihon")
	case vault == "":
		return title
	default:
		return title + " — " + vault
	}
}

type vaultNameKey struct{}

// VaultNamed answers every request with what the vault is called, asked of name
// only when a page is drawn, so that a request for a file or a fragment does
// not ask. The name is the one the shell shows, from the generation that is
// current when the page is built.
func VaultNamed(name func() string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), vaultNameKey{}, name)))
	})
}

func vaultOf(r *http.Request) string {
	if name, ok := r.Context().Value(vaultNameKey{}).(func() string); ok {
		return name()
	}
	return ""
}

// singleKeyShortcutsState is the word the root carries for the single-key
// preference. It is here rather than beside the markup because Go written
// inside a template reaches the compiler only as generated output, which every
// linter in this repository is told to skip.
func singleKeyShortcutsState(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}

// ChromeFromRequest builds the page chrome from the request: the page title
// plus the reading choices the browser carries, so the root element renders
// the correct state on the first byte (no FOUC). Every one of those choices is
// read through the same table, which names the values each cookie may carry;
// anything else falls to that row's answer — input hygiene, since a cookie is
// user-controllable.
//
// The theme's and the typeface's answers are deliberately empty rather than a
// named default: a reader who never chose has expressed no preference there,
// and the stylesheet answers an unstamped root with its own base value — the
// system's own preference, in the theme's case. Both stored themes are
// honored, because an explicit light choice must keep beating a dark system.
//
// It takes no shell: what the chrome is built from is the request and nothing
// else, and a snapshot projection passed alongside would say the two were
// related when they never were.
func ChromeFromRequest(r *http.Request, title string) Chrome {
	return Chrome{
		Title:                     title,
		Vault:                     vaultOf(r),
		Lang:                      origin.Language(r),
		Nonce:                     origin.Nonce(r.Context()),
		Theme:                     read(r, themeChoice),
		Ruby:                      read(r, rubyChoice),
		TextSize:                  read(r, textSizeChoice),
		Font:                      read(r, fontChoice),
		Rail:                      read(r, railChoice),
		SectionButtons:            read(r, sectionButtonsChoice),
		Position:                  read(r, positionChoice),
		SingleKeyShortcutsEnabled: read(r, shortcutsChoice) == "on",
		// The request's own address, so a control the server answers can bring
		// the reader back to this page afterwards. Only an address a GET can
		// revisit qualifies: a page rendered by a POST names a target, not a
		// place, so it falls back to Home — and a page that knows a better
		// return, as the recovery page knows its note, overrides this
		// afterwards.
		ReturnTo: returnableAddress(r),
		Path:     r.URL.Path,
	}
}

// destinationAttrs marks a header link to one of the interface's own pages,
// at path, as the page the reader is on when the request's own path is that
// page, which is what aria-current names; any other page leaves the link
// unmarked.
func destinationAttrs(here, path string) templ.Attributes {
	if here != path {
		return nil
	}
	return templ.Attributes{"aria-current": "page"}
}

// returnableAddress is the address a control the server answers sends a reader
// back to: a GET's own path and query, or Home when the page came from a POST,
// whose address names a target rather than a place a reader can revisit.
func returnableAddress(r *http.Request) string {
	if r.Method == http.MethodGet {
		return r.URL.RequestURI()
	}
	return "/"
}

// preferencesHref builds the address of the page a reader sets their reading
// choices on, carrying where they are so that page can send them back there. A
// reader already on it carries nothing: an address pointing at its own page
// needs no return, and one carrying its own would nest another copy of itself
// on every visit. Only the encoding happens here — whether the address names
// somewhere this site can return to is answered where it is followed.
func preferencesHref(returnTo string) string {
	if returnTo == "" || returnTo == "/preferences" {
		return "/preferences"
	}
	return "/preferences?from=" + url.QueryEscape(returnTo)
}
