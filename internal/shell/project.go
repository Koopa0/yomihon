// Package shell projects the navigation and lifecycle state shared by every
// full-page reading surface. It stands alone rather than beside the nav.Shell
// it returns because the navigation model cannot import the generation and the
// write face that feed it, and every reading face imports this.
package shell

import (
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
)

// Project derives one shell from one vault snapshot and the lifecycle view
// captured for the same request, reading no source of its own. It takes the
// artifact authority from the snapshot rather than from its caller so that the
// signature cannot express a mismatched pair.
func Project(lifecycle status.Authority, snap *snapshot.Generation) nav.Shell {
	policy := snap.ArtifactPolicy()
	governed := lifecycle.Governed()
	projected := nav.Shell{Nav: snap.Navigation(), Governed: governed}
	// Either authority refusing closes the instance-derived navigation and
	// counts: the two are sampled at different instants and both must answer.
	if claim := lifecycle.Claim(); !claim.Trustworthy() {
		return projected.WithoutInstanceProjections(claim)
	}
	if !policy.Trustworthy() {
		return projected.WithoutInstanceProjections(policy.Claim())
	}
	return projected
}
