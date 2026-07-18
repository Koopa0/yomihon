// Package shell projects the navigation and lifecycle state shared by every
// full-page reading surface. It owns the cross-feature projection, while the
// pages package owns the resulting presentation value.
package shell

import (
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/pages"
)

// Lifecycle is the read-only status knowledge needed by the shared shell.
// Implementations are request-scoped views; the projector performs no contract
// reads of its own.
type Lifecycle interface {
	Diagnostic() string
	Advanceable(noteType, status string) bool
}

// Project derives one shell from one vault snapshot and the two immutable
// request authorities captured by the composition point. The projector performs
// no source reads of its own. Instance-derived navigation and counts close
// together when either authority is unavailable.
func Project(
	lifecycle Lifecycle,
	policy schema.ArtifactPolicy,
	snap *snapshot.View,
) pages.Shell {
	projected := pages.Shell{Nav: snap.Navigation()}
	if diagnostic := lifecycle.Diagnostic(); diagnostic != "" {
		return projected.WithoutInstanceProjections(diagnostic)
	}
	if !policy.Available() {
		return projected.WithoutInstanceProjections(policy.Diagnostic())
	}

	count, known := advanceableCount(lifecycle, snap)
	projected.Advanceable = count
	projected.AdvanceableKnown = known
	return projected
}

func advanceableCount(lifecycle Lifecycle, snap *snapshot.View) (count int, known bool) {
	counts, err := snap.Search().CountByTypeStatus()
	if err != nil {
		return 0, false
	}
	for ts, n := range counts {
		if lifecycle.Advanceable(ts.Type, ts.Status) {
			count += n
		}
	}
	return count, true
}
