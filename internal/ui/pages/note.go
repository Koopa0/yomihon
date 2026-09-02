package pages

import (
	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// The reading page's own small answers: what a fault is called, what claim
// the date makes, which attributes a column carries, and which of the write
// face's states this note is in. They are here rather than beside the markup
// because Go written inside a template reaches the compiler only as generated
// output, which every linter in this repository is told to skip.

// diagKindLabel names a rendering diagnostic in the reader's language. A kind
// without a name here arrives as its raw slug rather than a dead page: the
// diagnostics exist to report trouble, and the panel failing on an unnamed
// kind would be the reporter breaking on the news.
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

// noteDateLabel names the claim the metarow's date makes: the author's own
// declared update, or the file's recorded change time when the note declares
// none a date can be read from.
func noteDateLabel(v *NoteView) string {
	if v.UpdatedFromFile {
		return wording.FileChangedOn.In(v.Lang)
	}
	return wording.UpdatedOn.In(v.Lang)
}

// articleLanguageAttrs states an article's language only where the note
// declared one and the vault contract gave that declaration authority. A note
// that declared nothing contributes no attribute, so the article inherits the
// language the surrounding page is written in.
//
// Naming a language yomihon does not know would be worse than saying nothing:
// the undetermined tag is a positive claim that halts inheritance for
// everything beneath it, and no reader, voice, or font picker is any better
// off for it. Inheriting is the truthful default for a vault whose prose is
// Traditional Chinese, and a note written in another language names that
// language in its own frontmatter.
func articleLanguageAttrs(tag string) templ.Attributes {
	if tag == "" {
		return nil
	}
	return templ.Attributes{"lang": tag}
}

// freshnessAttrs marks the reading column as one the client may watch: the path
// to ask about, the identity of the bytes printed inside it, and the status
// printed beside the title — the one value the identity leaves out, so without
// its own stamp a flip on disk would never reach an open page. They sit on the
// column rather than on the article because the notice they lead to belongs
// beside the article, not within it. All are withheld when
// the page is already saying its own words may be behind the file, because two
// differently worded notices about the same doubt help nobody, and when there is
// no identity to compare against. Where the attributes are absent the client
// finds nothing to watch and the page stays exactly as the server sent it.
func freshnessAttrs(v *NoteView, lang wording.Lang) templ.Attributes {
	if v.Stale || v.RelPath == "" || v.ContentIdentity == "" {
		return nil
	}
	// The sentences travel with the watch rather than living in the script. The
	// dictionary is the server's, and a second copy of these words inside a
	// JavaScript file would be a second place to translate them and a second
	// place for them to fall out of step.
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
	// The transcluded stamp is absent, not empty, for a page that pulled in
	// nothing: absence is what keeps that page's polling ask — and the
	// server's answer to it — exactly as narrow as before the stamp existed.
	if v.TranscludedIdentity != "" {
		attrs["data-freshness-embeds"] = v.TranscludedIdentity
	}
	return attrs
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

// hasAids reports whether this note carries reading aids — a table of
// contents or any diagnostic. The right rail exists to hold reading aids:
// when a note has none, the rail's column is dropped entirely and the write
// face appears in the fixed-bottom status bar instead, so the reader never
// sits beside a tall gutter holding a lone status card.
func (v *NoteView) hasAids() bool {
	return len(v.TOC) > 0 || v.Diagnostic != "" || len(v.RenderDiagnostics) > 0 || v.citedByShown()
}

// citedByShown reports whether the answer about what links here means anything
// on this page — either there are citations, or the library has links at all
// and their absence is itself the finding. Counting it among the aids matters
// twice: a rail holding only this block is not an empty rail, and the narrow
// projection has to carry it. Left out of both, a note whose sole aid was its
// backlinks lost them at every width, not only the narrow ones.
func (v *NoteView) citedByShown() bool {
	return len(v.CitedBy) > 0 || v.VaultHasLinks
}

// frontmatterDoorLine is the empty state's honest escape hatch: nothing leads
// onward from here through this interface, and recovery is a hand edit of the
// frontmatter — through the editor link the page already carries, when it
// does. Yomihon states the door; it never walks through it.
func frontmatterDoorLine(v *NoteView) string {
	if v.ObsidianHref == "" {
		return wording.EditFrontmatterToRecover.In(v.Lang)
	}
	return wording.EditFrontmatterToRecoverWithLink.In(v.Lang)
}

// diagnosticAddress spells an address the way its author wrote it. The
// diagnostic keeps the note name and the part after "#" apart, because other
// readers match on the name by itself and a joined string matches none of them;
// putting them back together is a decision about what to show, and it is made
// here so every panel shows the same thing.
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
func statusState(v *NoteView) string {
	switch {
	case v.NonInstance:
		return "non-instance"
	case v.WriteDiagnostic != "":
		return "unavailable"
	default:
		return "instance"
	}
}

// showsFlipReceipt reports whether this reading has a change to state.
func (v *NoteView) showsFlipReceipt() bool {
	return v.Governed && v.FlippedFrom != "" && v.Status != "" && v.FlippedFrom != v.Status
}
