// Package shell projects the navigation and lifecycle state shared by every
// full-page reading surface. It owns the cross-feature projection, while the
// pages package owns the resulting presentation value.
package shell

import (
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/ui/pages"
)

// Project derives one shell from one vault snapshot and the two immutable
// request authorities captured by the composition point. The projector performs
// no source reads of its own. Instance-derived navigation and counts close
// together when either authority can no longer be honoured — not merely when it
// is absent, because a folder that declared nothing excluded nothing and its
// projections are answerable.
func Project(
	lifecycle status.View,
	policy schema.ArtifactPolicy,
	snap *snapshot.View,
) pages.Shell {
	governed := lifecycle.Governed()
	projected := pages.Shell{Nav: snap.Navigation(), Governed: governed}
	// The two authorities are sampled at different instants, so a projection
	// stays open only while both are still answerable. Either one refusing
	// closes the shared navigation, whichever was captured first.
	if claim := lifecycle.Claim(); !claim.Trustworthy() {
		return projected.WithoutInstanceProjections(claim)
	}
	if !policy.Trustworthy() {
		return projected.WithoutInstanceProjections(policy.Claim())
	}
	return projected
}
