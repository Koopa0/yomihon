// Package report serves the reports face: the daily-briefing HTML under
// System/reports/, presented verbatim inside a sandboxed iframe. A requested
// <name> must match the snapshot's already-enumerated report allowlist, and the
// resulting canonical path must resolve to an Entry in that same snapshot. The
// process-owned vault Reader refreshes the Entry through its observed raw
// filesystem spelling and validates every directory and file identity before
// any response bytes are committed. A name that is not an enumerated briefing
// — unknown, a .md report (those are ordinary notes, served at /notes/), or any
// traversal-shaped string — simply does not match. The shell page renders
// header + sidebar + title + one iframe; the /raw endpoint writes the fully read
// file bytes unchanged (no templ, no renderer, correct charset), so a briefing
// round-trips byte-for-byte. The iframe carries a bare sandbox: authored scripts
// and automatic features do not run, and the document lands in an opaque origin.
// That sandbox is enforced on
// the /raw resource itself with a Content-Security-Policy sandbox header — not
// only through the shell's iframe attribute — so the containment holds however a
// briefing is loaded (framed by the shell, framed cross-origin, or opened
// top-level), never depending on the embedder. The resource policy permits
// self-contained inline styles, SVG, and data assets but no script or
// automatic network fetch. An explicit user navigation remains a user action,
// not something this automatic-egress boundary claims to suppress. yomihon
// styles only the frame — the renderer never touches the content inside it.
package report

import (
	"context"
	"io/fs"
	"log/slog"
	"strings"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vaultfs"
)

const briefingRoot = "System/reports/daily-briefing"

// Handler serves the reports face through one process-owned vault Reader.
type Handler struct {
	source  *vaultfs.Reader
	current func() *snapshot.Generation
	shell   func(*snapshot.Generation) pages.Shell
	log     *slog.Logger
}

// New wires the reports feature. Every dependency must be non-nil: a nil is a
// wiring bug that must fail here, not on the first request.
func New(
	source *vaultfs.Reader,
	current func() *snapshot.Generation,
	shell func(*snapshot.Generation) pages.Shell,
	log *slog.Logger,
) *Handler {
	if source == nil {
		panic("report: New requires a non-nil Source")
	}
	if current == nil {
		panic("report: New requires a non-nil Snapshot provider")
	}
	if shell == nil {
		panic("report: New requires a non-nil Shell provider")
	}
	if log == nil {
		panic("report: New requires a non-nil Log")
	}
	return &Handler{source: source, current: current, shell: shell, log: log}
}

// resolveReport looks a requested report name up against the report allowlist:
// the .html briefings nav already enumerated under System/reports/daily-briefing/
// (nav.Report.Briefing). The request's <name> is compared, never joined, so a
// name that is not an enumerated briefing (an unknown file, a .md report served
// at /notes/, or any traversal-shaped string) simply does not match. The
// matched RelPath is separately confined by readReport; neither boundary
// stands in for the other.
func resolveReport(model *nav.Model, name string) (nav.Report, bool) {
	if model == nil {
		return nav.Report{}, false
	}
	for _, rep := range model.Reports() {
		if rep.Briefing && rep.Name == name {
			return rep, true
		}
	}
	return nav.Report{}, false
}

func readReport(
	ctx context.Context,
	source *vaultfs.Reader,
	view *snapshot.Generation,
	relPath string,
) ([]byte, error) {
	name, ok := strings.CutPrefix(relPath, briefingRoot+"/")
	if !ok || name == "" || strings.Contains(name, "/") {
		return nil, fs.ErrNotExist
	}
	entry, ok := view.Entry(relPath)
	if !ok {
		return nil, fs.ErrNotExist
	}
	entry, err := source.Refresh(entry)
	if err != nil {
		return nil, err
	}
	return source.ReadFile(ctx, entry)
}
