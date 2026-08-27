package semantic

import "errors"

// ActiveGenerationState describes the active role observed by an explicit
// build while it owned the writer lease. The zero value means no authoritative
// observation was possible.
type ActiveGenerationState uint8

// The active roles a build can report. Together they answer the only question
// a failed build leaves a reader: can searching still be served meanwhile.
const (
	// ActiveGenerationNotInspected means no authoritative look was taken, so
	// this is the absence of an observation rather than one.
	ActiveGenerationNotInspected ActiveGenerationState = iota
	// ActiveGenerationAbsent means there is no active generation at all.
	ActiveGenerationAbsent
	// ActiveGenerationPreservedUsable means an active generation loaded, matches
	// this identity and corpus, and carries a measured search cost, so searching
	// is served from it while the failure is addressed.
	ActiveGenerationPreservedUsable
	// ActiveGenerationPreservedUnusable means an active generation is present
	// but cannot answer for this corpus — it failed to load, or its identity,
	// corpus fingerprint or measurement does not match — so searching waits on a
	// build that succeeds.
	ActiveGenerationPreservedUnusable
)

// StagingGenerationState describes the physical staging role observed by an
// explicit build while it owned the writer lease. The zero value means no
// authoritative observation was possible.
type StagingGenerationState uint8

// The staging roles a build can report. Together they say what a later build
// would find waiting for it, and so what the next action costs.
const (
	// StagingGenerationNotInspected means no authoritative look was taken, so
	// this is the absence of an observation rather than one.
	StagingGenerationNotInspected StagingGenerationState = iota
	// StagingGenerationAbsent means no partial build was left behind.
	StagingGenerationAbsent
	// StagingGenerationIncompatible means what was left behind cannot be
	// continued, so a later build starts over.
	StagingGenerationIncompatible
	// StagingGenerationResumable means a later build continues from what is
	// already embedded rather than paying for the corpus again.
	StagingGenerationResumable
	// StagingGenerationRequiresAuthorization means the staged build has spent
	// its chunk attempt budget. What it needs next is that budget renewed, not
	// another retry, and renewing it is the owner's decision because it is what
	// admits further content to the embedding provider.
	StagingGenerationRequiresAuthorization
)

// BuildError preserves a semantic build failure together with the generation
// facts observed before the action returned. Callers can project those facts
// without reopening a store that may have changed since the failure.
type BuildError struct {
	cause   error
	active  ActiveGenerationState
	staging StagingGenerationState
}

func (e *BuildError) Error() string {
	if e == nil || e.cause == nil {
		return "semantic build failed"
	}
	return e.cause.Error()
}

// Unwrap returns the failure that stopped the build.
func (e *BuildError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// ActiveGeneration returns the active-role state observed by the build.
func (e *BuildError) ActiveGeneration() ActiveGenerationState {
	if e == nil {
		return ActiveGenerationNotInspected
	}
	return e.active
}

// StagingGeneration returns the staging-role state observed by the build.
func (e *BuildError) StagingGeneration() StagingGenerationState {
	if e == nil {
		return StagingGenerationNotInspected
	}
	return e.staging
}

type buildObservation struct {
	active  ActiveGenerationState
	staging StagingGenerationState
}

func (o buildObservation) wrap(err error) error {
	if err == nil {
		return nil
	}
	if existing, ok := errors.AsType[*BuildError](err); ok && existing != nil {
		return err
	}
	return &BuildError{cause: err, active: o.active, staging: o.staging}
}
