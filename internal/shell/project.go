// Package shell projects the navigation and lifecycle state shared by every
// full-page reading surface. It owns the cross-feature projection, while the
// pages package owns the resulting presentation value.
//
// One function in a directory of its own looks like a package that should be
// folded into a neighbour, and there is no neighbour to fold it into. Beside
// the presentation value it returns is impossible: the write face renders its
// own recovery page, so status already imports pages, and pages importing
// status would close the loop. On the snapshot it would compile, and it would
// point the read generation at both the write face and the presentation layer,
// which inverts the model. In the reading feature it would be out of reach of
// the write face, which that feature already imports. What is left is a
// package every consumer may import, which is this one.
package shell

import (
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
)

// Project derives one shell from one vault snapshot and the immutable
// lifecycle view captured for the same request. The projector performs no
// source reads of its own, and it takes the artifact authority from the
// snapshot rather than from its caller: the two have to come from one capture
// for the closures below to mean anything, and a signature that cannot express
// the mismatch is worth more than a comment forbidding it.
//
// Instance-derived navigation and counts close together when either authority
// can no longer be honoured — not merely when it is absent, because a folder
// that declared nothing excluded nothing and its projections are answerable.
func Project(lifecycle status.Authority, snap *snapshot.Generation) nav.Shell {
	policy := snap.ArtifactPolicy()
	governed := lifecycle.Governed()
	projected := nav.Shell{Nav: snap.Navigation(), Governed: governed}
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
