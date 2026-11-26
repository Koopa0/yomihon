package note

import (
	"cmp"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/koopa0/yomihon/internal/drill"
	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/shell"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/wording"
)

// HasPlace reports whether rel is a note this snapshot lets a reader open and
// anchor is a place that note renders: one of its headings, the title standing
// in for an opening heading, or a slot-machine card of its drill. An empty
// anchor names the note as a whole. Heading ids do not depend on the interface
// language, so any one renders the same set.
func (h *Handler) HasPlace(rel, anchor string) bool {
	return newPlaceResolver(h.sources.Snapshot().Capture()).hasPlace(rel, anchor)
}

// Reading is what a mark is placed in: the words of the note as its page shows
// them in lang, taken from the sanitised HTML the renderer makes and not from
// the Markdown, because a reader selects from the page. It is false for a note
// the vault does not let a reader open.
func (h *Handler) Reading(rel string, lang wording.Lang) (mark.Page, bool) {
	snap := h.sources.Snapshot().Capture()
	n, ok := readableNote(snap, rel)
	if !ok {
		return mark.Page{}, false
	}
	return mark.Page{
		Text:     render.TextContent(snap.Render(rel, n.Body, lang).HTML),
		Identity: hex.EncodeToString(n.ContentIdentity[:]),
	}, true
}

// Units is the notes the course at course walks, as the sidebar and the course
// page read it: the shell's own projection of the navigation model, so rules
// that close the courses close this too. The main line and the side
// branches are apart because a count of what is finished takes the first alone.
// ok is false where no course answers to course.
func (h *Handler) Units(course string) (mainLine, branches []string, ok bool) {
	snap := h.sources.Snapshot().Capture()
	visible := shell.Project(h.sources.VaultName, h.sources.Authority(), snap)
	onMain, onBranches, ok := visible.Nav.CourseUnits(course)
	if !ok {
		return nil, nil, false
	}
	return relPaths(onMain), relPaths(onBranches), true
}

// Rows is what a course counts, as the sidebar and the course page read it, from
// the same projection as Units: how many rows its main line plans, those not yet
// written included, and the notes of those a reader can open, one for each row.
// ok is false where no course answers to course.
func (h *Handler) Rows(course string) (planned int, rows []string, ok bool) {
	snap := h.sources.Snapshot().Capture()
	visible := shell.Project(h.sources.VaultName, h.sources.Authority(), snap)
	planned, onMain, ok := visible.Nav.CourseRows(course)
	if !ok {
		return 0, nil, false
	}
	return planned, relPaths(onMain), true
}

func relPaths(refs []nav.NoteRef) []string {
	paths := make([]string, len(refs))
	for i := range refs {
		paths[i] = refs[i].RelPath
	}
	return paths
}

// placeResolver owns a request's captured generation and lazily resolves each
// shown note once. A note's complete rendered place set is built at most once.
type placeResolver struct {
	snap  *snapshot.Generation
	notes map[string]*resolvedPlaces
}

type resolvedPlaces struct {
	reading  snapshot.Reading
	readable bool
	anchors  map[string]resolvedPlace
	// title is the anchor the title inherited from an opening heading, if any.
	title string
}

type resolvedPlace struct {
	label pages.RowPlace
	card  int
	// heading is the words the place's heading reads, the title for the anchor
	// the title inherited from an opening heading, and empty for a card.
	heading string
}

func newPlaceResolver(snap *snapshot.Generation) *placeResolver {
	return &placeResolver{snap: snap, notes: make(map[string]*resolvedPlaces)}
}

func (p *placeResolver) note(rel string) *resolvedPlaces {
	if resolved, ok := p.notes[rel]; ok {
		return resolved
	}
	n, ok := readableNote(p.snap, rel)
	resolved := &resolvedPlaces{reading: n, readable: ok}
	p.notes[rel] = resolved
	return resolved
}

func (p *placeResolver) hasPlace(rel, anchor string) bool {
	n := p.note(rel)
	if !n.readable {
		return false
	}
	if anchor == "" {
		return true
	}
	if n.anchors == nil {
		n.anchors = make(map[string]resolvedPlace)
		result := p.snap.Render(rel, n.reading.Body, wording.ZhHant)
		if result.TitleAnchor != "" {
			n.anchors[result.TitleAnchor] = resolvedPlace{heading: strings.TrimSpace(n.reading.Title)}
			n.title = result.TitleAnchor
		}
		for _, heading := range result.TOC {
			n.anchors[heading.ID] = resolvedPlace{label: pages.RowPlace{Text: heading.Text, Language: n.reading.Language}, heading: heading.Text}
		}
		if sidecar, found := p.snap.Slots().Lookup(n.reading.Slug); found {
			for i, pattern := range sidecar.Patterns {
				n.anchors[pages.SlotPatternID(i)] = resolvedPlace{
					label: pages.RowPlace{Text: drill.AbstractTemplate(pattern.Template), Language: "ja"}, card: i + 1,
				}
			}
		}
	}
	_, found := n.anchors[anchor]
	return found
}

// titleAnchor is the anchor rel's title inherited, once its places are
// resolved; empty when the title took over no heading.
func (p *placeResolver) titleAnchor(rel string) string {
	return p.note(rel).title
}

func (p *placeResolver) label(rel, anchor string, lang wording.Lang) pages.RowPlace {
	if !p.hasPlace(rel, anchor) {
		return pages.RowPlace{}
	}
	place := p.note(rel).anchors[anchor]
	if place.card > 0 && place.label.Text == "" {
		return pages.RowPlace{Text: fmt.Sprintf(wording.PracticeCardLabelFmt.In(lang), place.card), Language: lang.Tag()}
	}
	return place.label
}

// opening is the first sentence of the section anchor names, read from the note
// as it stands, and empty for the whole note, for the title's anchor and for a
// practice card, which has no fixed text: the sentence on a card is a
// combination made when the page is opened.
func (p *placeResolver) opening(rel, anchor string) string {
	if anchor == "" || !p.hasPlace(rel, anchor) || anchor == p.titleAnchor(rel) {
		return ""
	}
	n := p.note(rel)
	place := n.anchors[anchor]
	if place.heading == "" || place.card > 0 {
		return ""
	}
	return render.SectionSentence(n.reading.Body, place.heading)
}

// MarkedLocation is what one marked location names in a generation, in the
// words a program reading the open list is given.
type MarkedLocation struct {
	// Lost says what is gone, when anything is.
	Lost Loss
	// AnchorText is what the page shows for the place the anchor names: a
	// heading's words, the title for the anchor the title inherited from an
	// opening heading, or a practice card's pattern. It is empty for the whole
	// note and for a lost location.
	AnchorText string
	// Wikilink is the vault's own link to the location, brackets included,
	// written as a thought cites its source. It is empty wherever it could name
	// somewhere else: a note whose path a link cannot spell back to it, a
	// heading whose link text another heading of the note shares, and a lost
	// location. A practice card and the title's anchor have no heading a link
	// can name, so theirs is the note's own.
	Wikilink string
}

// Loss is what a marked location no longer finds.
type Loss int

const (
	// LostNothing is a location whose note renders the place.
	LostNothing Loss = iota
	// LostNote is a location whose note this generation does not let a reader
	// open.
	LostNote
	// LostLocation is a location whose note is there and renders no place the
	// anchor names.
	LostLocation
)

// String names a loss for a log line or a panic.
func (l Loss) String() string {
	switch l {
	case LostNothing:
		return "nothing"
	case LostNote:
		return "note"
	case LostLocation:
		return "location"
	default:
		panic("note: unknown Loss: " + strconv.Itoa(int(l)))
	}
}

// LocateMark resolves one marked location against snap through the same
// place set the shelf and the mark route accept. A card's words are given in
// lang where the card has no pattern of its own to show.
func LocateMark(snap *snapshot.Generation, rel, anchor string, lang wording.Lang) MarkedLocation {
	return MarkLocator(snap, lang)(rel, anchor)
}

// MarkLocator is LocateMark for a list of marks: each note is read and rendered
// once however many marks it holds, so the answers are the same and the cost
// is not paid once for each mark. The function it returns is not safe to call
// from more than one goroutine.
func MarkLocator(snap *snapshot.Generation, lang wording.Lang) func(rel, anchor string) MarkedLocation {
	places := newPlaceResolver(snap)
	return func(rel, anchor string) MarkedLocation { return locateMark(snap, places, rel, anchor, lang) }
}

func locateMark(snap *snapshot.Generation, places *placeResolver, rel, anchor string, lang wording.Lang) MarkedLocation {
	n := places.note(rel)
	if !n.readable {
		return MarkedLocation{Lost: LostNote}
	}
	if !places.hasPlace(rel, anchor) {
		return MarkedLocation{Lost: LostLocation}
	}
	var located MarkedLocation
	cites := thoughtSourceAvailable(snap, rel)
	whole := ""
	if cites {
		whole = wikilinkTo(rel, "")
	}
	if anchor == "" {
		located.Wikilink = whole
		return located
	}
	place := n.anchors[anchor]
	if place.heading == "" || anchor == places.titleAnchor(rel) {
		located.AnchorText = cmp.Or(place.heading, places.label(rel, anchor, lang).Text)
		located.Wikilink = whole
		return located
	}
	located.AnchorText = place.heading
	link := headingLinkKey(place.heading)
	if !cites || link == "" {
		return located
	}
	for id, other := range n.anchors {
		if id != anchor && other.heading != "" && headingLinkKey(other.heading) == link {
			return located
		}
	}
	located.Wikilink = wikilinkTo(rel, place.heading)
	return located
}

// NoteWikilink is the vault's own link to the whole of rel, brackets
// included, or empty when a link cannot spell its path back to it.
func NoteWikilink(snap *snapshot.Generation, rel string) string {
	if !thoughtSourceAvailable(snap, rel) {
		return ""
	}
	return wikilinkTo(rel, "")
}

func wikilinkTo(rel, sectionText string) string {
	return "[[" + thoughtCitation(rel, sectionText) + "]]"
}

// headingLinkKey is the heading as a link names it, folded the way two
// headings could be mistaken for one another by an editor matching it.
func headingLinkKey(text string) string {
	return graph.FoldFragment(headingLinkText(text))
}
