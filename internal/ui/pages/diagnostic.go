package pages

import (
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// renderDiagnosticSummary says what one rendering fault means and what the page
// did about it, in the reader's language. A kind with no sentence here still
// says something rather than going quiet about a fault.
func renderDiagnosticSummary(kind render.DiagnosticKind, lang wording.Lang) string {
	switch kind {
	case render.DiagImageMissing:
		return wording.DiagImageMissingNote.In(lang)
	case render.DiagWikilinkBroken:
		return wording.DiagUnwrittenNote.In(lang)
	case render.DiagWikilinkTitleOnly:
		return wording.DiagTitleOnlyNote.In(lang)
	case render.DiagTitleTruncatedAtHash:
		return wording.DiagTitleCutNote.In(lang)
	case render.DiagWikilinkAmbiguous:
		return wording.DiagAmbiguousNote.In(lang)
	case render.DiagUnknownCallout:
		return wording.DiagCalloutNote.In(lang)
	case render.DiagRiskyFence:
		return wording.DiagFenceNote.In(lang)
	case render.DiagEmbedFragmentMissing:
		return wording.DiagEmbedNote.In(lang)
	case render.DiagEmbedFragmentRepeated:
		return wording.DiagEmbedManyNote.In(lang)
	case render.DiagEmbedNotExpanded:
		return wording.DiagEmbedDepthNote.In(lang)
	case render.DiagLinkFragmentMissing:
		return wording.DiagBlockNote.In(lang)
	case render.DiagLinkSectionMissing:
		return wording.DiagSectionNote.In(lang)
	case render.DiagCommentUnclosed:
		return wording.DiagCommentNote.In(lang)
	case render.DiagRenderFailed:
		return wording.DiagRenderNote.In(lang)
	default:
		return wording.DiagUnknownNote.In(lang)
	}
}
