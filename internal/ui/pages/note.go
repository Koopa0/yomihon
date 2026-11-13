package pages

import (
	"strings"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/lesson"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// diagKindLabel names a rendering diagnostic in the reader's language. A kind
// without a name here arrives as its raw slug rather than a dead page.
func diagKindLabel(kind render.DiagnosticKind, lang wording.Lang) string {
	if render.IsFigureFinding(kind) {
		return figureDiagLabel(kind, lang)
	}
	return renderDiagLabel(kind, lang)
}

// renderDiagLabel names every rendering finding that is not about a figure.
func renderDiagLabel(kind render.DiagnosticKind, lang wording.Lang) string {
	switch kind {
	case render.DiagImageMissing:
		return wording.DiagImageMissing.In(lang)
	case render.DiagWikilinkBroken, render.DiagMarkdownBroken:
		return wording.DiagLinkNoTarget.In(lang)
	case render.DiagWikilinkTitleOnly:
		return wording.DiagLinkTitleOnly.In(lang)
	case render.DiagWikilinkAmbiguous:
		return wording.DiagLinkManyTargets.In(lang)
	case render.DiagUnknownCallout:
		return wording.DiagUnknownCallout.In(lang)
	case render.DiagRiskyFence, render.DiagHighlightFailed:
		return fenceDiagLabel(kind, lang)
	case render.DiagEmbedFragmentMissing:
		return wording.DiagEmbedFragmentGone.In(lang)
	case render.DiagEmbedFragmentRepeated:
		return wording.DiagEmbedFragmentRepeated.In(lang)
	case render.DiagEmbedNotExpanded:
		return wording.DiagEmbedNotExpanded.In(lang)
	case render.DiagRenderFailed:
		return wording.DiagRenderFailed.In(lang)
	case render.DiagLinkFragmentMissing:
		return wording.DiagLinkBlockGone.In(lang)
	case render.DiagLinkSectionMissing:
		return wording.DiagLinkSectionGone.In(lang)
	case render.DiagCommentUnclosed, render.DiagCommentContainerUnclosed:
		return commentDiagLabel(kind, lang)
	default:
		return string(kind)
	}
}

// figureDiagLabel names the five reasons a figure is not shown.
func figureDiagLabel(kind render.DiagnosticKind, lang wording.Lang) string {
	switch kind {
	case render.DiagFigureTooLarge:
		return wording.DiagFigureTooLarge.In(lang)
	case render.DiagFigureEncoding:
		return wording.DiagFigureEncoding.In(lang)
	case render.DiagFigureNestedFrame:
		return wording.DiagFigureNestedFrame.In(lang)
	case render.DiagFigureLoadsElsewhere:
		return wording.DiagFigureLoadsElsewhere.In(lang)
	default:
		return wording.DiagFigureScriptOutside.In(lang)
	}
}

// commentDiagLabel keeps body-wide silence distinct from an unterminated
// comment confined to one Markdown container.
func commentDiagLabel(kind render.DiagnosticKind, lang wording.Lang) string {
	if kind == render.DiagCommentContainerUnclosed {
		return wording.DiagCommentContainerUnclosed.In(lang)
	}
	return wording.DiagCommentUnclosed.In(lang)
}

// fenceDiagLabel names a code-block diagnostic. The two kinds share a switch
// arm so adding highlighting's failure does not push the page's kind table
// over the complexity gate.
func fenceDiagLabel(kind render.DiagnosticKind, lang wording.Lang) string {
	if kind == render.DiagHighlightFailed {
		return wording.DiagHighlightFailed.In(lang)
	}
	return wording.DiagRiskyFence.In(lang)
}

// noteDateLabel names the claim the metarow's date makes: the author's declared
// update, or the file's recorded change time where the note declares none.
func noteDateLabel(v *NoteView, lang wording.Lang) string {
	if v.UpdatedFromFile {
		return wording.FileChangedOn.In(lang)
	}
	return wording.UpdatedOn.In(lang)
}

// headFactsShown reports whether the head has any fact to draw as a
// description list: the same four fields noteFacts gates row by row, so a
// note with none of them draws neither the disclosure nor the open copy. A
// draft always has a path, so its mark is drawn whenever it is owed.
func (v *NoteView) headFactsShown() bool {
	return v.Type != "" || v.Updated != "" || v.Language != "" || v.RelPath != ""
}

// authoredLanguageAttrs states the language the note's author wrote in, only
// where the note declared one and the contract gave that declaration authority.
// It goes on every element whose text is the author's rather than the
// interface's, which is not only the article: the contents list repeats the
// note's own headings from outside it, and the preview card shows the note's
// body somewhere else again. A note that declared nothing contributes no
// attribute and inherits the page's language, because the undetermined tag would
// halt that inheritance for everything beneath it.
func authoredLanguageAttrs(tag string) templ.Attributes {
	if tag == "" {
		return nil
	}
	return templ.Attributes{"lang": tag}
}

// freshnessAttrs marks the reading column as one the client may watch: the path
// to ask about and the identity of the bytes printed inside it. They are withheld from a page
// already saying its words may be behind the file, and from one with no identity
// to compare against; absent, the client finds nothing to watch.
func freshnessAttrs(v *NoteView, lang wording.Lang) templ.Attributes {
	if v.Stale || v.RelPath == "" || v.ContentIdentity == "" {
		return nil
	}
	// The sentences travel with the watch rather than living in the script: a
	// copy there would be a second place to translate them.
	attrs := templ.Attributes{
		"data-freshness-path":        v.RelPath,
		"data-freshness-identity":    v.ContentIdentity,
		"data-freshness-newversion":  wording.FreshnessNewVersion.In(lang),
		"data-freshness-reload":      wording.FreshnessReload.In(lang),
		"data-freshness-preparing":   wording.FreshnessPreparing.In(lang),
		"data-freshness-gone":        wording.FreshnessGone.In(lang),
		"data-freshness-searchtitle": wording.FreshnessSearchTitle.In(lang),
	}
	// Absent rather than empty for a page that reads no embed, which keeps that
	// page's polling ask as narrow as it was before the stamp existed.
	if v.TranscludedIdentity != "" {
		attrs["data-freshness-embeds"] = v.TranscludedIdentity
	}
	return attrs
}

// readAloudAttrs carries the read-aloud bar's words to the page that will grow
// one. The browser builds the bar, and a sentence built there would be in
// whichever language the script was written in. They are withheld from a page
// with nothing to read aloud, on the conditions the script itself uses: every
// speaker the reader can press (a paragraph, a practice card, a table cell)
// reaches the same speech owner, so a lesson that offers only one kind needs
// these words as much as one that offers another.
// It takes the rendered body rather than the note, because the bar belongs to
// any page that carries marked paragraphs and one of them is not a note.
func readAloudAttrs(bodyHTML string, lang wording.Lang) templ.Attributes {
	if !strings.Contains(bodyHTML, speakButtonMarker) &&
		!strings.Contains(bodyHTML, practiceSpeakMarker) {
		return nil
	}
	return templ.Attributes{
		"data-readaloud-cell":        wording.ReadAloudCellFmt.In(lang),
		"data-readaloud-controls":    wording.ReadAloudControls.In(lang),
		"data-readaloud-speed":       wording.ReadAloudSpeed.In(lang),
		"data-readaloud-rate":        wording.ReadAloudRateFmt.In(lang),
		"data-readaloud-stop":        wording.ReadAloudStop.In(lang),
		"data-readaloud-stopthis":    wording.ReadAloudStopThis.In(lang),
		"data-readaloud-stopped":     wording.ReadAloudStopped.In(lang),
		"data-readaloud-playing":     wording.ReadAloudPlaying.In(lang),
		"data-readaloud-finished":    wording.ReadAloudFinished.In(lang),
		"data-readaloud-unavailable": wording.ReadAloudUnavailable.In(lang),
		"data-readaloud-playall":     wording.ReadAloudPlayAll.In(lang),
		"data-readaloud-previous":    wording.ReadAloudPrevious.In(lang),
		"data-readaloud-next":        wording.ReadAloudNext.In(lang),
		"data-readaloud-progress":    wording.ReadAloudProgressFmt.In(lang),
	}
}

// speakButtonMarker is the attribute the renderer writes and the script reads,
// so asking for it asks the same question the script asks.
const speakButtonMarker = `data-tts="`

// practiceSpeakMarker is the sentence-practice card's own speaker. The card is
// spliced into the body before these attributes are decided, so asking the body
// for it asks the same question the script asks. Without it the card's button
// takes the speaking state but keeps its idle label, which reads as a control
// that half works.
const practiceSpeakMarker = `data-slot-action="speak"`

// hasUnclosedComment keeps a hidden draft distinct from a note with no words:
// its diagnostic already explains why the reader cannot see its content.
func (v *NoteView) hasUnclosedComment() bool {
	for _, diagnostic := range v.RenderDiagnostics {
		if diagnostic.Kind == render.DiagCommentUnclosed {
			return true
		}
	}
	return false
}

// diagCount is the diagnostics rail's badge number: the frontmatter diagnostic
// (0 or 1) plus every render diagnostic.
func (v *NoteView) diagCount() int {
	n := len(v.RenderDiagnostics)
	if v.Diagnostic != "" {
		n++
	}
	return n
}

// endShown reports whether the article ends on a block of what stands beside
// it: a note to read beside it, the sources it declared, the notes declared on
// it, the answer about which notes link here, or what the check found in it. A
// note with none of them ends on its text, and the sidebar draws no row that
// leads to an end it does not have.
func (v *NoteView) endShown() bool {
	return v.Pair.RelPath != "" || len(v.BasedOn) > 0 || len(v.DeclaredBy) > 0 || v.citedByShown() || v.diagCount() > 0
}

// endName is the end block's id, which the sidebar's last contents row leads
// to. Authored section ids never contain an underscore, so it cannot collide
// with a heading the note itself carries, and the article's id space keeps two
// notes on one page apart.
const endName = "_y-end"

func (v *NoteView) endID() string { return v.IDPrefix + endName }

// endLabel is the words the sidebar's last contents row carries: the first
// heading the end block itself draws, in the order it draws them, so the row
// and the block it leads to say the same thing. Every one of them is a string
// the block already prints; an end with none to name has no row, so the empty
// answer is never read.
func (v *NoteView) endLabel(lang wording.Lang) string {
	switch {
	case v.Pair.RelPath != "":
		return wording.CompareOffer.In(lang)
	case len(v.BasedOn) > 0:
		return wording.BasedOn.In(lang)
	case len(v.DeclaredBy) > 0:
		return wording.DeclaredBy.In(lang)
	case v.citedByShown():
		return wording.CitedBy.In(lang)
	case v.diagCount() > 0:
		return wording.NoteHealth.In(lang)
	default:
		return ""
	}
}

// railWithContents is the reading rail this page draws: the one the handler
// resolved, with the note's own contents to hang under the row that is the
// note. The rail is built before the page's words are, so what it needs of the
// note arrives here rather than being resolved a second time.
func (v *NoteView) railWithContents() ReadingRail {
	rail := v.ReadingRail
	rail.note = v
	return rail
}

// offersMark reports whether this page can offer to keep the reader's place.
// It needs the note's own address and the identity of the bytes being shown,
// because the mark is both of those and a page missing either would store a
// place that points at nothing in particular.
//
// The control it gates is in the header, which stays put while the note
// scrolls: a copy the reader had to travel to would record where travelling to
// it left them, not where they were reading.
func (v *NoteView) offersMark() bool {
	return v.RelPath != "" && v.ContentIdentity != "" && v.MarkAddress != ""
}

// markOffer is what the header needs to draw the control: the note's address,
// the identity of the bytes on the page, and where the control posts. It
// answers from the predicate above rather than repeating its three tests, so a
// page offers the control or does not and nil is the whole of "does not".
func (v *NoteView) markOffer() *layouts.MarkOffer {
	if !v.offersMark() {
		return nil
	}
	return &layouts.MarkOffer{
		Path:     v.RelPath,
		Identity: v.ContentIdentity,
		Endpoint: v.MarkAddress,
		Kept:     v.PlaceKept,
	}
}

// citedByShown reports whether the answer about what links here means anything
// on this page: either there are citations, or the library has links at all and
// their absence is itself the finding. It counts among the aids, so a rail
// holding only this block is not an empty one.
func (v *NoteView) citedByShown() bool {
	return len(v.CitedBy) > 0 || v.VaultHasLinks
}

// diagnosticAddress spells an address the way its author wrote it. The
// diagnostic keeps the note name and the part after "#" apart, so rejoining
// them is a decision about display, made here so every panel shows the same.
func diagnosticAddress(d *render.Diagnostic) string {
	switch {
	case d.Block != "":
		return d.Target + "#^" + d.Block
	case d.Section != "":
		return d.Target + "#" + d.Section
	default:
		return d.Target
	}
}

// conceptSheetTitle is the words the sheet is named by. The sheet takes its
// accessible name from its visible title, so a concept that arrives without one
// still needs something to be called.
func conceptSheetTitle(title string, lang wording.Lang) string {
	if strings.TrimSpace(title) == "" {
		return wording.ConceptNote.In(lang)
	}
	return title
}

// hasSteps reports whether the foot has anything to draw: a step either way,
// a way off a side branch, or a branch to point at. A foot with none draws no
// landmark at all, so a lesson that stands alone in its course is not handed an
// empty navigation block.
func (v *NoteView) hasSteps() bool {
	return v.Prev.RelPath != "" || v.Next.RelPath != "" ||
		v.StepsBack.RelPath != "" || v.StepsOnward.RelPath != "" || v.StepsToContents ||
		len(v.StepsAsides) > 0
}

// carriesReadings reports whether this note's page holds furigana for the
// header's switch to show and hide: in the rendered body, or in a concept
// sheet the body can open over itself.
func (v *NoteView) carriesReadings() bool {
	return render.CarriesReadings(v.BodyHTML) || conceptsCarryReadings(v.Concepts)
}

// conceptsCarryReadings reports whether any of the concept sheets a page can
// open holds furigana.
func conceptsCarryReadings(concepts []lesson.ConceptDoc) bool {
	for _, c := range concepts {
		if render.CarriesReadings(c.HTML) {
			return true
		}
	}
	return false
}
