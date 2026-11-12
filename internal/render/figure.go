package render

import (
	"fmt"
	"html"
	"path"
	"strconv"
	"strings"

	"github.com/koopa0/yomihon/internal/figure"
	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/wording"
)

// Figures answers, for a figure a page embeds, what the generation's read of
// its bytes found and whether the owner approved those bytes to run.
type Figures interface {
	Figure(relPath string) FigureState
}

// Approval is where the owner's approval of one figure stands.
type Approval uint8

const (
	// Unapproved is a figure nobody approved, or whose approvals could not be
	// read.
	Unapproved Approval = iota
	// Approved is an approval of exactly the bytes inspected.
	Approved
	// Lapsed is an approval of the same path and other bytes: the figure
	// changed after it was approved.
	Lapsed
)

// String names an approval's standing for a log line or a crash.
func (a Approval) String() string {
	switch a {
	case Unapproved:
		return "unapproved"
	case Approved:
		return "approved"
	case Lapsed:
		return "lapsed"
	default:
		return "approval(" + strconv.Itoa(int(a)) + ")"
	}
}

// FigureState is one figure as a page is about to draw it.
//
// It decides the frame's sandbox attribute and the words beside it, not what
// the figure may do: the frame's document is fetched separately, read again
// there, and served under its own content policy, and a browser applies the
// narrower of that policy and this attribute. A page drawn before the file
// changed can therefore show a frame that admits scripts around a document
// that does not, and never the other way round.
type FigureState struct {
	// Inspection is what the generation's read of the file found.
	Inspection figure.Inspection
	// Size is the file's size as the generation saw it.
	Size int64
	// Read is false when the generation holds no read of the file. The frame
	// is drawn without script and the document route decides on its own read.
	Read bool
	// Approval is where the owner's approval of those bytes stands.
	Approval Approval
	// ApprovalsUnreadable means the approvals could not be read, so none was
	// honoured.
	ApprovalsUnreadable bool
}

// FigureHeight is the height of a figure frame whose embed names no height.
// A figure cannot measure itself without script, so the author's |WxH is the
// only other answer; this one is tall enough for a small diagram with a few
// controls.
const FigureHeight = 480

// figureSize reads the |W or |WxH an embed may carry. The height is a fixed
// pixel height, not a ratio: a figure whose text reflows grows taller as its
// frame narrows, which a ratio would clip exactly where the figure is tallest.
func figureSize(display string) (width, height string, ok bool) {
	if !imagePixelSize.MatchString(display) {
		return "", "", false
	}
	width, height, both := strings.Cut(display, "x")
	if !both {
		height = strconv.Itoa(FigureHeight)
	}
	return width, height, true
}

// figureEmbed draws ![[name.html]]: a refused figure as a stand-in with the
// reason beside it, and any other as a frame onto the document route. The
// frame has an empty sandbox, so no script, no same-origin access, no forms,
// no popups and no navigation of the page around it; an approved figure's
// frame admits scripts and nothing more.
func figureEmbed(relPath string, link graph.Wikilink, state *FigureState, col *collector) string {
	lang := col.page.lang
	title := path.Base(relPath)
	width, height := "", strconv.Itoa(FigureHeight)
	if link.Aliased {
		if w, h, sized := figureSize(link.Display); sized {
			width, height = w, h
		} else if strings.TrimSpace(link.Display) != "" {
			title = link.Display
		}
	}
	standIn := fmt.Sprintf(wording.FigureStandInFmt.In(lang),
		`<a href="`+attributeEscaper.Replace(notesHref(relPath))+`">`+html.EscapeString(title)+`</a>`)
	if col.page.figureStandIns {
		return `<div class="figure-embed figure-embed--stand-in"><p class="embed-media">` + standIn + `</p></div>`
	}

	if state.Read && state.Inspection.Refusal != figure.Served {
		finding := FigureFinding(&state.Inspection, state.Size)
		finding.Target = link.Target
		col.report(&finding)
		return `<div class="figure-embed figure-embed--refused"><p class="embed-media">` + standIn + `</p>` +
			`<p class="embed__note figure-embed__note">` + html.EscapeString(FigureFindingNote(finding.Kind, finding.Detail, lang)) + `</p></div>`
	}

	scripted := state.Read && state.Inspection.Scripted()
	runs := scripted && state.Approval == Approved && !state.ApprovalsUnreadable
	sandbox, marker := "", ""
	if runs {
		sandbox, marker = "allow-scripts", ` data-figure-scripts`
	}
	sized := ` height="` + height + `"`
	if width != "" {
		sized = ` width="` + width + `"` + sized
	}
	var b strings.Builder
	b.WriteString(`<div class="figure-embed">`)
	fmt.Fprintf(&b, `<iframe class="figure-embed__frame" src="%s" title="%s" sandbox="%s" referrerpolicy="no-referrer" allow=""%s%s></iframe>`,
		attributeEscaper.Replace(figureHref(relPath)), html.EscapeString(title), sandbox, sized, marker)
	b.WriteString(`<p class="figure-embed__print">` + standIn + `</p>`)
	if scripted && !runs {
		b.WriteString(`<p class="embed__note figure-embed__note">` + html.EscapeString(idleSentence(state).In(lang)) + `</p>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

// idleSentence says why a figure's scripts are not running.
func idleSentence(state *FigureState) wording.Phrase {
	switch {
	case state.ApprovalsUnreadable:
		return wording.FigureApprovalsUnreadable
	case state.Approval == Lapsed:
		return wording.FigureApprovalLapsed
	default:
		return wording.FigureScriptsIdle
	}
}

// FigureFinding is the finding a refused figure is reported as: one of the
// five figure kinds, carrying what its note names. size is the file's size,
// which a figure refused for being too large is refused by.
func FigureFinding(in *figure.Inspection, size int64) Diagnostic {
	switch in.Refusal {
	case figure.TooLarge:
		return Diagnostic{Kind: DiagFigureTooLarge, Detail: figureByteSize(size),
			Message: "figure is " + strconv.FormatInt(size, 10) + " bytes, over the " + strconv.Itoa(figure.MaxBytes) + " byte limit"}
	case figure.NotText, figure.UTF16:
		return Diagnostic{Kind: DiagFigureEncoding, Message: "figure is not UTF-8 text"}
	case figure.NestedFrame:
		return Diagnostic{Kind: DiagFigureNestedFrame, Detail: in.Detail, Message: "figure contains " + in.Detail}
	case figure.LoadsElsewhere:
		return Diagnostic{Kind: DiagFigureLoadsElsewhere, Detail: in.Detail, Message: "figure contains " + in.Detail}
	case figure.ScriptOutsideText:
		return Diagnostic{Kind: DiagFigureScriptOutside, Detail: in.Detail, Message: "figure has " + in.Detail}
	default:
		panic("render: unknown figure.Refusal: " + in.Refusal.String())
	}
}

// FigureFindingNote is the note for one figure finding, in lang: the same
// sentence on the page, in the findings rail, and in the frame.
func FigureFindingNote(kind DiagnosticKind, detail string, lang wording.Lang) string {
	switch kind {
	case DiagFigureTooLarge:
		return fmt.Sprintf(wording.DiagFigureTooLargeNote.In(lang), detail, figureByteSize(figure.MaxBytes))
	case DiagFigureEncoding:
		return wording.DiagFigureEncodingNote.In(lang)
	case DiagFigureNestedFrame:
		return fmt.Sprintf(wording.DiagFigureNestedFrameNote.In(lang), detail)
	case DiagFigureLoadsElsewhere:
		return fmt.Sprintf(wording.DiagFigureLoadsElsewhereNote.In(lang), detail)
	case DiagFigureScriptOutside:
		return fmt.Sprintf(wording.DiagFigureScriptOutsideNote.In(lang), detail)
	default:
		panic("render: unknown figure DiagnosticKind: " + string(kind))
	}
}

// IsFigureFinding reports whether kind is one of the five figure findings.
func IsFigureFinding(kind DiagnosticKind) bool {
	switch kind {
	case DiagFigureTooLarge, DiagFigureEncoding, DiagFigureNestedFrame, DiagFigureLoadsElsewhere, DiagFigureScriptOutside:
		return true
	default:
		return false
	}
}

// figureByteSize writes a size in MiB with one decimal, the unit the figure
// limit is stated in, which reads the same in both languages.
func figureByteSize(n int64) string {
	mib := float64(n) / (1 << 20)
	if mib == float64(int64(mib)) {
		return strconv.FormatInt(int64(mib), 10) + " MiB"
	}
	return strconv.FormatFloat(mib, 'f', 1, 64) + " MiB"
}

// figureHref is the document route a figure's frame loads.
func figureHref(p string) string {
	return "/embed/" + escapedVaultPath(p)
}
