package schema

// Authority is one immutable reading of the vault contract, captured once per
// request: whether anything claimed governance over the folder, how far that
// claim got, and the artifact policy as its source stood at capture. Its query
// methods perform no I/O, so one request derives every contract decision from
// the same sample.
type Authority struct {
	contract   *Contract
	policy     ArtifactPolicy
	governance Governance
}

// CaptureAuthority reads the contract's artifact policy against its source
// once and returns the view one request answers from. A nil contract is either
// a folder that never claimed authority or one whose contract could not be
// read; governance says which.
func CaptureAuthority(contract *Contract, governance Governance) Authority {
	if contract == nil {
		return Authority{governance: governance}
	}
	return Authority{contract: contract, policy: contract.ArtifactPolicy().Capture(), governance: governance}
}

// Governed reports whether anything claimed authority over this vault. A false
// answer is not a failure: the folder simply has no contract.
func (a Authority) Governed() bool {
	return a.governance.Governed()
}

// Claim reports how far the contract's authority got. A policy whose source
// changed after startup closes the view with its own claim.
func (a Authority) Claim() Claim {
	if a.contract != nil && !a.policy.Available() {
		return a.policy.Claim()
	}
	return a.governance.Claim()
}

// Closed reports whether this captured view cannot classify note instances.
func (a Authority) Closed() bool {
	return a.contract == nil || !a.policy.Available()
}

// Diagnostic explains why this captured view is closed. It is empty both when
// the contract reads cleanly and when nothing ever claimed authority.
func (a Authority) Diagnostic() string {
	return a.Claim().Diagnostic()
}

// Policy is the artifact policy this view captured.
func (a Authority) Policy() ArtifactPolicy {
	return a.policy
}

// IsLessonType reports whether noteType is the type this vault files its
// course members as. The name comes from the contract layer, which spells it
// once; whether this folder declares the type is deliberately not asked, since
// a note that calls itself a lesson gets the reading aids either way and a
// folder with no contract would otherwise lose them.
func (a Authority) IsLessonType(noteType string) bool {
	lesson, _ := a.contract.LessonType()
	return noteType == lesson
}
