// Package shell projects the navigation and lifecycle state shared by every
// full-page reading surface. It owns the cross-feature projection; the value
// it produces is nav.Shell, which belongs to the navigation model and knows
// nothing about where its parts were gathered from.
//
// One function in a directory of its own looks like a package that should be
// folded into a neighbour, and there is no neighbour to fold it into. Beside
// the value it returns is impossible: the generation already imports the
// navigation model, so the model importing it back would not compile, and the
// model reaching the write face as well would make what a projection is out of
// what one is made from. On the generation it would compile, and it would point
// the read side at the write face, which inverts the model. In the reading
// feature it would be out of reach of every other face that draws a rail. What
// is left is a package every consumer may import, which is this one.
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
