package pages

import (
	"strconv"
	"strings"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// diagKindLabel names a rendering diagnostic in the reader's language. A kind
// without a name here arrives as its raw slug rather than a dead page.
func diagKindLabel(kind render.DiagnosticKind, lang wording.Lang) string {
	switch kind {
	case render.DiagWikilinkBroken:
		return wording.DiagLinkNoTarget.In(lang)
	case render.DiagWikilinkTitleOnly:
		return wording.DiagLinkTitleOnly.In(lang)
	case render.DiagTitleTruncatedAtHash:
		return wording.DiagTitleCut.In(lang)
	case render.DiagWikilinkAmbiguous:
		return wording.DiagLinkManyTargets.In(lang)
	case render.DiagUnknownCallout:
		return wording.DiagUnknownCallout.In(lang)
	case render.DiagRiskyFence:
		return wording.DiagRiskyFence.In(lang)
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
	case render.DiagCommentUnclosed:
		return wording.DiagCommentUnclosed.In(lang)
	default:
		return string(kind)
	}
}

// noteDateLabel names the claim the metarow's date makes: the author's declared
// update, or the file's recorded change time where the note declares none.
func noteDateLabel(v *NoteView, lang wording.Lang) string {
	if v.UpdatedFromFile {
		return wording.FileChangedOn.In(lang)
	}
	return wording.UpdatedOn.In(lang)
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
// to ask about, the identity of the bytes printed inside it, and the status
// beside the title, which the identity leaves out. They are withheld from a page
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
		"data-freshness-status":      v.Status,
		"data-freshness-newversion":  wording.FreshnessNewVersion.In(lang),
		"data-freshness-reload":      wording.FreshnessReload.In(lang),
		"data-freshness-preparing":   wording.FreshnessPreparing.In(lang),
		"data-freshness-gone":        wording.FreshnessGone.In(lang),
		"data-freshness-searchtitle": wording.FreshnessSearchTitle.In(lang),
		"data-freshness-holdtitle":   wording.FreshnessHoldPreparingTitle.In(lang),
		"data-freshness-holddetail":  wording.FreshnessHoldPreparingDetail.In(lang),
		"data-freshness-gonetitle":   wording.FreshnessHoldGoneTitle.In(lang),
		"data-freshness-writehold":   wording.FreshnessWriteHold.In(lang),
	}
	// Absent rather than empty for a page that pulled in nothing, which keeps
	// that page's polling ask as narrow as it was before the stamp existed.
	if v.TranscludedIdentity != "" {
		attrs["data-freshness-embeds"] = v.TranscludedIdentity
	}
	return attrs
}

// readAloudAttrs carries the read-aloud bar's words to the page that will grow
// one. The browser builds the bar, and a sentence built there would be in
// whichever language the script was written in. They are withheld from a page
// with nothing to read aloud, on the condition the script itself uses.
func readAloudAttrs(v *NoteView, lang wording.Lang) templ.Attributes {
	if !strings.Contains(v.BodyHTML, speakButtonMarker) {
		return nil
	}
	return templ.Attributes{
		"data-readaloud-controls":    wording.ReadAloudControls.In(lang),
		"data-readaloud-speed":       wording.ReadAloudSpeed.In(lang),
		"data-readaloud-rate":        wording.ReadAloudRateFmt.In(lang),
		"data-readaloud-stop":        wording.ReadAloudStop.In(lang),
		"data-readaloud-stopthis":    wording.ReadAloudStopThis.In(lang),
		"data-readaloud-stopped":     wording.ReadAloudStopped.In(lang),
		"data-readaloud-playing":     wording.ReadAloudPlaying.In(lang),
		"data-readaloud-finished":    wording.ReadAloudFinished.In(lang),
		"data-readaloud-unavailable": wording.ReadAloudUnavailable.In(lang),
	}
}

// speakButtonMarker is the attribute the renderer writes and the script reads,
// so asking for it asks the same question the script asks.
const speakButtonMarker = `data-tts="`

// diagCount is the diagnostics rail's badge number: the frontmatter diagnostic
// (0 or 1) plus every render diagnostic.
func (v *NoteView) diagCount() int {
	n := len(v.RenderDiagnostics)
	if v.Diagnostic != "" {
		n++
	}
	return n
}

// hasAids reports whether this note carries reading aids. With none, the right
// rail's column is dropped and the write face moves to the bottom bar, so no
// reader sits beside a tall gutter holding a lone status card.
func (v *NoteView) hasAids() bool {
	return len(v.TOC) > 0 || v.Diagnostic != "" || len(v.RenderDiagnostics) > 0 || v.citedByShown()
}

// citedByShown reports whether the answer about what links here means anything
// on this page: either there are citations, or the library has links at all and
// their absence is itself the finding. It counts among the aids, so a rail
// holding only this block is not an empty one.
func (v *NoteView) citedByShown() bool {
	return len(v.CitedBy) > 0 || v.VaultHasLinks
}

// frontmatterDoorLine is the empty state's escape hatch: recovery is a hand edit
// of the frontmatter, through the editor link the page carries when it has one.
func frontmatterDoorLine(v *NoteView, lang wording.Lang) string {
	if v.ObsidianHref == "" {
		return wording.EditFrontmatterToRecover.In(lang)
	}
	return wording.EditFrontmatterToRecoverWithLink.In(lang)
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

// faceState is which state one note puts the write face in. The rail panel and
// the bottom bar offer the same states in the same order and read this one
// answer, so neither keeps rules of its own. The states are ordered by which
// fact overrides which: ungoverned first, then a face that could not be opened,
// then the ways a status can fail to be readable, then the two readings of an
// empty set, then a readable status with moves to offer.
type faceState uint8

const (
	faceNonInstance faceState = iota
	faceWriteUnavailable
	faceNoFrontmatter
	faceStatusUnknown
	faceStatusNotText
	faceStatusUnreadable
	faceOutsideScope
	faceNoTransitions
	faceTransitions
)

// statusFace decides which state a note puts the write face in. An empty set
// has two readings and they stay apart: the declared knowledge layer withheld
// the moves, or the schema defines none onward from this status.
func statusFace(v *NoteView) faceState {
	switch {
	case v.NonInstance:
		return faceNonInstance
	case v.WriteDiagnostic != "":
		return faceWriteUnavailable
	case v.NoFrontmatter:
		return faceNoFrontmatter
	case v.StatusUnknown:
		return faceStatusUnknown
	case v.Status == "" && v.StatusNotText:
		return faceStatusNotText
	case v.Status == "":
		return faceStatusUnreadable
	case v.OutsideKnowledgeScope && len(v.Transitions) == 0:
		return faceOutsideScope
	case len(v.Transitions) == 0:
		return faceNoTransitions
	default:
		return faceTransitions
	}
}

// token is the value both faces stamp on themselves for the client and the
// stylesheet. It is coarser than the state it comes from: the outside asks only
// whether the folder governs this note and whether the face opened.
func (f faceState) token() string {
	switch f {
	case faceNonInstance:
		return "non-instance"
	case faceWriteUnavailable:
		return "unavailable"
	case faceNoFrontmatter, faceStatusUnknown, faceStatusNotText, faceStatusUnreadable,
		faceOutsideScope, faceNoTransitions, faceTransitions:
		return "instance"
	}
	panic("pages: unknown write-face state: " + strconv.Itoa(int(f)))
}

// showsStatusFace reports whether the write face has anything to show about this
// note. Both faces ask it themselves rather than trusting whoever draws them. A
// folder that governs nothing has no lifecycle; a frontmatter fault leaves no
// status to report. A note outside the governed set stays named through both,
// that classification coming from its path rather than its frontmatter.
func (v *NoteView) showsStatusFace() bool {
	return v.Governed && (v.Diagnostic == "" || v.NonInstance)
}

// showsFlipReceipt reports whether this reading has a change to state.
func (v *NoteView) showsFlipReceipt() bool {
	return v.Governed && v.FlippedFrom != "" && v.Status != "" && v.FlippedFrom != v.Status
}

// schemaNoticesID names the block of schema findings the transition controls
// describe themselves by. One page renders at most one such block, which is what
// lets the id be fixed.
const schemaNoticesID = "schema-notices"

// schemaNoticesRef is the description a transition submit carries beside the
// findings, so a control announced on its own still says what the amber notices
// say to a sighted reader. The attribute is absent on a page with no findings,
// so no control describes itself by an element that is not in the document.
func schemaNoticesRef(v *NoteView) templ.Attributes {
	if len(v.SchemaNotices) == 0 {
		return nil
	}
	return templ.Attributes{"aria-describedby": schemaNoticesID}
}
