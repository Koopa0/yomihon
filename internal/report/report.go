// Package report serves the daily-briefing HTML under System/reports/ inside a
// sandboxed iframe. A requested name matches a briefing the snapshot already
// enumerated or it matches nothing; the raw endpoint writes the file's bytes
// unchanged under a sandbox policy set on the resource itself, so containment
// does not depend on the embedder.
package report

import (
	"context"
	"io/fs"
	"log/slog"
	"strings"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/vaultfs"
)

const briefingRoot = "System/reports/daily-briefing"

// RequestSnapshot is the reading generation and the shell state captured
// together from one atomic vault generation. Two separate captures could name
// a report the rail beside it has never heard of.
type RequestSnapshot struct {
	Generation *snapshot.Generation
	Shell      nav.Shell
}

// Handler serves the reports face through one process-owned vault Reader.
type Handler struct {
	source   *vaultfs.Reader
	snapshot func() RequestSnapshot
	log      *slog.Logger
}

// New wires the reports feature. Every dependency must be non-nil: a nil is a
// wiring bug that must fail here, not on the first request.
func New(source *vaultfs.Reader, snapshotProvider func() RequestSnapshot, log *slog.Logger) *Handler {
	if source == nil {
		panic("report: New requires a non-nil Source")
	}
	if snapshotProvider == nil {
		panic("report: New requires a non-nil Snapshot provider")
	}
	if log == nil {
		panic("report: New requires a non-nil Log")
	}
	return &Handler{source: source, snapshot: snapshotProvider, log: log}
}

// resolveReport matches a requested name against the briefings nav enumerated.
// The name is compared, never joined, so nothing outside that set can match.
//
// A vault holds its names composed and a request can carry either spelling of
// the same letter, so the name is composed before it is compared. Composition
// is canonical: it cannot introduce a separator or a dot segment, and the
// comparison it feeds is still against an enumerated set.
func resolveReport(model *nav.Model, name string) (nav.Report, bool) {
	if model == nil {
		return nav.Report{}, false
	}
	name = vault.NormalizeNFC(name)
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
