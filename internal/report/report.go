// Package report serves the reports face: the daily-briefing HTML
// under System/reports/, presented verbatim inside a sandboxed iframe. It is the
// first route in yomihon that serves a raw vault file, so its safety rests on one
// invariant — a requested <name> is only ever looked up against the snapshot's
// already-enumerated report allowlist (the .html briefings nav found under
// System/reports/daily-briefing/, nav.Report.Briefing); request input never
// reaches a filesystem path join. A name that is not an enumerated briefing —
// unknown, a .md report (those are ordinary notes, served at /notes/), or any
// traversal-shaped string — simply does not match, and the handler answers a
// silent 404. The shell page renders header + sidebar + title + one iframe; the
// /raw endpoint writes the file's bytes unchanged (no templ, no renderer,
// correct charset), so a briefing round-trips byte-for-byte. The iframe is
// sandboxed allow-scripts and never allow-same-origin: even with scripts running
// it lands in an opaque origin, the double-layer defense the loopback listener's
// same-origin property pairs with. That sandbox is enforced on
// the /raw resource itself with a Content-Security-Policy sandbox header — not
// only through the shell's iframe attribute — so the containment holds however a
// briefing is loaded (framed by the shell, framed cross-origin, or opened
// top-level), never depending on the embedder. yomihon styles only the frame —
// the renderer never touches the content inside it.
package report

import (
	"log/slog"

	"github.com/koopa0/yomihon/internal/nav"
)

// Deps is everything the reports feature reads from: the current navigation
// model (read fresh per request, since the report allowlist lives behind the
// scanner's atomic snapshot and is rebuilt when the vault changes), the vault
// root the /raw endpoint reads briefing files from, and a
// logger. Nav is a plain closure because "give me the current model" is a
// closure, not a method set — mirroring the reading and syllabus features.
type Deps struct {
	Root string
	Nav  func() *nav.Model
	Log  *slog.Logger
}

// Handler serves the reports face for a vault rooted at Deps.Root.
type Handler struct {
	deps Deps
}

// NewHandler wires the reports feature. Every dependency must be non-nil (and
// Root non-empty): a nil is a wiring bug that must fail here, not on the first
// request.
func NewHandler(d Deps) *Handler {
	if d.Root == "" {
		panic("report: NewHandler requires a non-empty Root")
	}
	if d.Nav == nil {
		panic("report: NewHandler requires a non-nil Nav provider")
	}
	if d.Log == nil {
		panic("report: NewHandler requires a non-nil Log")
	}
	return &Handler{deps: d}
}

// resolveReport looks a requested report name up against the report allowlist:
// the .html briefings nav already enumerated under System/reports/daily-briefing/
// (nav.Report.Briefing). It is the whole of the feature's path safety — the
// request's <name> is compared, never joined — so a name that is not an
// enumerated briefing (an unknown file, a .md report served at /notes/, or any
// traversal-shaped string like "../../Diary/x") simply does not match. The
// RelPath the /raw endpoint then reads comes from the matched entry (the scanner
// produced it, confined to System/reports/), not from request input.
func resolveReport(model *nav.Model, name string) (nav.Report, bool) {
	if model == nil {
		return nav.Report{}, false
	}
	for _, rep := range model.Reports {
		if rep.Briefing && rep.Name == name {
			return rep, true
		}
	}
	return nav.Report{}, false
}
