package semantic

import "errors"

// ActiveGenerationState describes the active role observed by an explicit
// build while it owned the writer lease. The zero value means no authoritative
// observation was possible.
type ActiveGenerationState uint8

const (
	ActiveGenerationNotInspected ActiveGenerationState = iota
	ActiveGenerationAbsent
	ActiveGenerationPreservedUsable
	ActiveGenerationPreservedUnusable
)

// StagingGenerationState describes the physical staging role observed by an
// explicit build while it owned the writer lease. The zero value means no
// authoritative observation was possible.
type StagingGenerationState uint8

const (
	StagingGenerationNotInspected StagingGenerationState = iota
	StagingGenerationAbsent
	StagingGenerationIncompatible
	StagingGenerationResumable
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
