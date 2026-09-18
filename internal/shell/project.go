// Package shell projects the navigation and lifecycle state shared by every
// full-page reading surface. It stands alone rather than beside the nav.Shell
// it returns because the navigation model cannot import the generation and the
// write face that feed it. The command projects one for the faces that never
// import this package — search and reports are handed the result — while the
// note face builds its own, several times over, from the same call.
package shell

import (
	"path/filepath"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
)

// VaultName reduces the folder the server was pointed at to the name a reader
// knows it by: the last element of its path. A vault contract declares no name
// of its own, so there is nothing else to take.
//
// Three paths yield no name to show — an empty one, the root of the filesystem,
// and a path that is nothing but separators — and all three answer with an
// empty string, so the one surface that says which folder this is has one case
// to word rather than a stray "." or "/" to explain.
func VaultName(path string) string {
	switch name := filepath.Base(filepath.Clean(path)); name {
	case ".", string(filepath.Separator):
		return ""
	default:
		return name
	}
}

// Project derives one shell from one vault snapshot and the lifecycle view
// captured for the same request, reading no source of its own. It takes the
// artifact authority from the snapshot rather than from its caller so that the
// signature cannot express a mismatched pair. vaultName is the folder's own
// name, already reduced to something a reader recognises by VaultName.
func Project(vaultName string, lifecycle status.Authority, snap *snapshot.Generation) nav.Shell {
	policy := snap.ArtifactPolicy()
	governed := lifecycle.Governed()
	found := GatherFindings(lifecycle, snap)
	projected := nav.Shell{
		Nav:      snap.Navigation(),
		Governed: governed,
		Vault: nav.Vault{
			Name:     vaultName,
			Notes:    snap.NoteCount(),
			Findings: found.Total(),
		},
	}
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
