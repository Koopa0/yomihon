package schema

import "strconv"

// grant records how far one declaration got. Absence has two meanings and only
// one is news: a folder that never carried a contract asserted nothing, while
// one whose contract omits or mangles a declaration asserted what it cannot
// honour.
type grant int

const (
	// grantUnclaimed means nothing ever asserted this, so there is nothing to
	// report: a projection over an undeclared set is empty and that is true.
	grantUnclaimed grant = iota
	// grantUnresolved means an assertion was made and could not be honoured —
	// unreadable, incomplete, rejected, or stale. This is the one worth saying
	// out loud: a projection over it would silently omit governed material.
	grantUnresolved
	// grantHeld means the declaration was read cleanly and still matches the
	// bytes it was derived from.
	grantHeld
)

// Reason names, in a value a caller can branch on, why a declaration could not
// be honoured. Most rejections carry only an operator's sentence; the
// vault-level one reaches an ordinary reader's page, which is written in a
// language chosen per request rather than when the contract was loaded.
type Reason uint8

const (
	// ReasonUnstated is a rejection with no machine-readable reason: the
	// diagnostic sentence is the whole of what is known.
	ReasonUnstated Reason = iota
	// ReasonContractUnreadable is a contract file that exists and could not be
	// loaded. Cause carries the loader's own error, so a surface can name the
	// fault in whichever language it is speaking.
	ReasonContractUnreadable
)

// String names a rejection reason for a diagnostic, a log line or a panic.
// These words are for an operator reading a machine's output; a reader's own
// sentence comes from the dictionary at the surface. A reason outside the
// constants is a programming error and panics.
func (r Reason) String() string {
	switch r {
	case ReasonUnstated:
		return "unstated"
	case ReasonContractUnreadable:
		return "contract-unreadable"
	default:
		panic("schema: unknown Reason: " + strconv.Itoa(int(r)))
	}
}

// Claim is one declaration's outcome: how far it got, why it failed where a
// caller can act on the answer, and the operator-facing sentence when the
// outcome is news. The zero Claim is unclaimed and silent. A Claim can be
// closed from outside this package but never opened — authority comes from a
// contract that was read, never from a caller asserting it.
type Claim struct {
	outcome    grant
	reason     Reason
	cause      error
	diagnostic string
}

// Rejected returns the claim for a declaration that was made and could not be
// honoured. The closure is a fact about the declaration, not about whether
// anyone wrote a sentence for it: a capability answered from the presence of a
// message is the failure this type exists to remove.
func Rejected(diagnostic string) Claim {
	return Claim{outcome: grantUnresolved, diagnostic: diagnostic}
}

func heldClaim() Claim { return Claim{outcome: grantHeld} }

// A capability asks a Claim one of two questions, and which one is the
// capability's own polarity. Trustworthy asks whether a projection over the set
// may be answered at all; held asks the narrower "is there a declaration here
// to read", which is what a capability needs when acting on silence would
// invent a rule the folder's owner never wrote. held stays unexported so that
// each capability answers it as its own Available and no call site asks the
// other question by accident.

// Claimed reports whether anything ever asserted this at all.
func (c Claim) Claimed() bool { return c.outcome != grantUnclaimed }

func (c Claim) held() bool { return c.outcome == grantHeld }

// Trustworthy reports whether a projection over this declaration's set may be
// answered. It holds for a declaration read cleanly and for one never made — an
// undeclared set is the empty set — and fails only where an assertion was made
// and could not be honoured.
func (c Claim) Trustworthy() bool { return c.outcome != grantUnresolved }

// Diagnostic is the operator-facing sentence, empty unless there is news.
func (c Claim) Diagnostic() string { return c.diagnostic }

// Reason names why this declaration was rejected, where a caller can act on the
// answer rather than only print it. It is ReasonUnstated for a claim that
// holds, for one nothing ever made, and for a rejection with nothing more to
// say.
func (c Claim) Reason() Reason { return c.reason }

// Cause is the error behind the rejection, nil where there is none. It travels
// beside Reason so a surface can name the fault in its own words and still
// quote what the loader said.
func (c Claim) Cause() error { return c.cause }

// Governance is what a folder asserted about its own contract, whether or not
// that assertion could be honoured. A folder carrying no contract governs
// nothing and is not in trouble; one whose contract cannot be read claimed
// governance it failed to deliver.
type Governance struct {
	claim Claim
}

// Ungoverned is the answer for a folder that carries no contract file.
func Ungoverned() Governance { return Governance{} }

// Unreadable records a contract file that exists and could not be loaded,
// which without this is indistinguishable from a folder carrying none. The
// claim carries the reason and the loader's error rather than a finished
// sentence: a contract loads at startup, before any reader has asked for
// anything, so only the surface knows which language to say it in.
func Unreadable(err error) Governance {
	return Governance{claim: unreadableClaim(err)}
}

// unreadableClaim is the outcome for a contract file that exists and could not
// be read, wherever that is discovered: at load, or later when a capability
// re-reads the bytes it was derived from. Both carry the reader's error so a
// surface can name the fault in its own words.
func unreadableClaim(err error) Claim {
	return Claim{
		outcome:    grantUnresolved,
		reason:     ReasonContractUnreadable,
		cause:      err,
		diagnostic: unreadableDiagnostic(err),
	}
}

// unreadableDiagnostic is the operator's own line; a surface speaking to a
// reader uses the reason instead.
func unreadableDiagnostic(err error) string {
	const sentence = "the vault contract could not be read"
	if err == nil {
		return sentence
	}
	return sentence + ": " + err.Error()
}

// Reason names why the contract could not be honoured, empty of meaning unless
// this vault claimed governance and failed to deliver it.
func (g Governance) Reason() Reason { return g.claim.reason }

// Governance reports what this contract asserts. A nil contract governs nothing.
func (c *Contract) Governance() Governance {
	if c == nil {
		return Governance{}
	}
	return Governance{claim: heldClaim()}
}

// Governed reports whether anything claimed authority over this vault. It is
// true for a contract that loaded, for one that could not be read, and for one
// that left a section out: the claim is what governs, not its completeness.
func (g Governance) Governed() bool { return g.claim.Claimed() }

// Trustworthy reports whether the contract's declarations may be projected over.
func (g Governance) Trustworthy() bool { return g.claim.Trustworthy() }

// Diagnostic is the vault-level sentence, empty unless the contract asserted
// governance it could not deliver.
func (g Governance) Diagnostic() string { return g.claim.Diagnostic() }

// Claim returns the vault-level outcome as a capability claim, so a projection
// closed by the contract itself carries the same reason value as one closed by
// a single declaration.
func (g Governance) Claim() Claim { return g.claim }

// Capabilities are the four declarations one process runs on, resolved
// together so no consumer can combine a vault-level fault with a zero
// capability and conclude that nothing was excluded.
type Capabilities struct {
	Navigation NavigationRoles
	Knowledge  KnowledgeScope
	Artifacts  ArtifactPolicy
	Language   ArticleLanguage
}

// Capabilities resolves this contract's declarations against what the folder
// asserted about the contract as a whole. A folder with no contract answers
// with the zero capabilities, every declared set being empty; one that claimed
// authority and could not be read closes every projection, its sets unknown.
//
// Each withheld capability carries the vault-level sentence rather than
// silence. Language is the exception: it has no Available or Diagnostic for a
// claim to feed, so it returns the same "not declared" zero value.
func (c *Contract) Capabilities(g Governance) Capabilities {
	if !g.Trustworthy() {
		return Capabilities{
			Navigation: NavigationRoles{claim: g.claim},
			Knowledge:  KnowledgeScope{claim: g.claim},
			Artifacts:  ArtifactPolicy{state: &artifactPolicyState{claim: g.claim}},
		}
	}
	return Capabilities{
		Navigation: c.NavigationRoles(),
		Knowledge:  c.KnowledgeScope(),
		Artifacts:  c.ArtifactPolicy(),
		Language:   c.ArticleLanguage(),
	}
}
