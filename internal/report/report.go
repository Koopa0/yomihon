// Package report serves the daily-briefing HTML under System/reports/ inside a
// sandboxed iframe. A requested name matches a briefing the snapshot already
// enumerated or it matches nothing; the raw endpoint writes the file's bytes
// unchanged under a sandbox policy set on the resource itself, so containment
// does not depend on the embedder.
package report

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/vault"
)

// MaxSourceBytes bounds the briefing bytes read for the shell and its frame.
const MaxSourceBytes int64 = 16 * 1024 * 1024

// ReadingLimitError refuses a briefing while retaining the observed size and
// the bound the reader applied, so both response surfaces explain it alike.
type ReadingLimitError struct {
	Size  int64
	Limit int64
}

func (e *ReadingLimitError) Error() string {
	return fmt.Sprintf("briefing size %d exceeds reading limit %d", e.Size, e.Limit)
}

// RequestSnapshot is the reading generation and the shell state captured
// together from one atomic vault generation. Two separate captures could name
// a report the rail beside it has never heard of.
type RequestSnapshot struct {
	Generation *snapshot.Generation
	Shell      nav.Shell
}

// Handler serves the reports face through one process-owned vault Reader.
type Handler struct {
	source   *vault.Reader
	snapshot func() RequestSnapshot
	log      *slog.Logger
}

// New wires the reports feature. Every dependency must be non-nil: a nil is a
// wiring bug that must fail here, not on the first request.
func New(source *vault.Reader, snapshotProvider func() RequestSnapshot, log *slog.Logger) *Handler {
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
	source *vault.Reader,
	view *snapshot.Generation,
	relPath string,
) ([]byte, error) {
	// BriefingName is the only briefing-shape test. A second root string here
	// would let a widened redirect land on an empty frame.
	name, ok := nav.BriefingName(relPath)
	if !ok || name == "" {
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
	body, err := source.ReadPrefix(ctx, entry, MaxSourceBytes+1)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > MaxSourceBytes {
		return nil, &ReadingLimitError{Size: int64(len(body)), Limit: MaxSourceBytes}
	}
	return body, nil
}
