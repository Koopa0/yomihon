package nav

import (
	"fmt"
	"slices"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// CoreFault names a readable note whose navigation tree could not be built.
// Reason is the navigation failure, independent of any schema or judge finding.
type CoreFault struct {
	Note   NoteRef
	Reason string
}

// CoreFaults returns the omitted navigation entries in captured path order.
// The returned slice belongs to the caller; an absent model has no faults.
func (m *Model) CoreFaults() []CoreFault {
	if m == nil {
		return nil
	}
	return slices.Clone(m.coreFaults)
}

type coreWalkers struct {
	path    func(*vault.Note, *graph.Index, map[string]noteFacts, schema.ArtifactPolicy) Path
	mapping func(*vault.Note, *graph.Index, map[string]noteFacts, schema.ArtifactPolicy) Map
}

// guardCoreWalk contains one entire navigation build before its tree is published.
// Completion is tracked separately so panic(nil) cannot look like a successful walk.
func guardCoreWalk(note NoteRef, walk func()) (fault *CoreFault) {
	completed := false
	defer func() {
		if !completed {
			reason := recover()
			fault = &CoreFault{Note: note, Reason: fmt.Sprintf("navigation build failed: %v", reason)}
		}
	}()
	walk()
	completed = true
	return nil
}
