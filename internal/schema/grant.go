package schema

import "strconv"

// grant records how far one declaration got. Absence of a declaration has two
// meanings and only one of them is news: a folder that never carried a contract
// asserted nothing, while a contract that exists and then omits, mangles, or
// contradicts a declaration asserted something yomihon could not honour.
type grant int

const (
	// grantUnclaimed means nothing ever asserted this. The declared set is
	// empty, a projection over an empty set is empty, and empty is the true
	// answer — so there is nothing to report.
	grantUnclaimed grant = iota
	// grantUnresolved means an assertion was made and then could not be
	// honoured: unreadable, incomplete, rejected, or gone stale. This is the
	// one case worth saying out loud, because a projection built over it would
	// silently answer with material the operator meant to govern.
	grantUnresolved
	// grantHeld means the declaration was read cleanly and still matches the
	// bytes it was derived from.
	grantHeld
)

// Reason names, in a value a caller can branch on, why a declaration could not
// be honoured. Most rejections carry an operator's sentence and nothing else:
// they are read on the machine that owns the folder, by the person who wrote
// it. The vault-level one is different, because it is what an ordinary reader
// is shown on an ordinary page, and a reader's page is written in the reader's
// own language — which is a fact about a request, not about a contract that
// was loaded before any request existed.
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
// These words are yomihon's own, for an operator reading a machine's output —
// the sentence a reader sees is chosen from the dictionary at the surface, and
// this is not it.
//
// A reason outside the constants is a programming error and stops here naming
// its number.
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
// outcome is news. The zero Claim is unclaimed and silent.
//
// A Claim can be closed from outside this package but never opened: authority
// comes from a contract that was read, never from a caller asserting it.
type Claim struct {
	outcome    grant
	reason     Reason
	cause      error
	diagnostic string
}

// Rejected returns the claim for a declaration that was made and could not be
// honoured. Every caller today has something to say, but the closure does not
// depend on that: it is a fact about the declaration, not about whether anyone
// wrote a sentence for it. A capability answered from the presence of a message
// is the failure this type exists to remove, so the two are kept apart even
// where they happen to coincide.
func Rejected(diagnostic string) Claim {
	return Claim{outcome: grantUnresolved, diagnostic: diagnostic}
}

func heldClaim() Claim { return Claim{outcome: grantHeld} }

// A capability asks a Claim one of two questions, and which one it asks is the
// capability's own polarity, not a detail of its implementation.
//
// Trustworthy is the gate for "may I answer a projection over this set at
// all". It holds both for a declaration read cleanly and for one nobody ever
// made, because an undeclared set is the empty set and a projection over an
// empty set is answerable. It fails only where an assertion was made and could
// not be honoured, because then the set the operator intended is unknown.
//
// held is the narrower question, "is there a declaration here to read", and is
// the one a capability asks when acting on silence would be inventing a rule
// the folder's owner never wrote — egress, above all, where permission is
// positive authority and an absent declaration is not permission. It is not
// exported: each capability answers it as its own Available, in the words of
// what it governs, and two exported names for one question is how a call site
// comes to ask the other one by accident.

// Claimed reports whether anything ever asserted this at all.
func (c Claim) Claimed() bool { return c.outcome != grantUnclaimed }

func (c Claim) held() bool { return c.outcome == grantHeld }

// Trustworthy reports whether a projection over this declaration's set may be
// answered. It holds for a declaration that was read cleanly and for one that
// was never made — an undeclared set is the empty set. It fails only when an
// assertion was made and could not be honoured, because then the set the
// operator intended is unknown.
func (c Claim) Trustworthy() bool { return c.outcome != grantUnresolved }

// Diagnostic is the operator-facing sentence, empty unless there is news.
func (c Claim) Diagnostic() string { return c.diagnostic }

// Reason names why this declaration was rejected, where the answer is one a
// caller can act on rather than only print. It is ReasonUnstated for a claim
// that holds, for one nothing ever made, and for a rejection whose sentence is
// all there is to say.
func (c Claim) Reason() Reason { return c.reason }

// Cause is the error behind the rejection, nil where there is none to hand
// back. It travels beside Reason so a surface can name the fault in its own
// words and still quote what the loader actually said.
func (c Claim) Cause() error { return c.cause }

// Governance is what a folder asserted about its own contract, independent of
// whether that assertion could be honoured. It is the vault-level fact every
// capability question sits under: a folder that carries no contract governs
// nothing and is not in trouble, while a folder whose contract cannot be read
// claimed governance it then failed to deliver.
type Governance struct {
	claim Claim
}

// Ungoverned is the answer for a folder that carries no contract file.
func Ungoverned() Governance { return Governance{} }

// Unreadable records a contract file that exists and could not be loaded. The
// distinction is the one loud fact for the whole vault: without it, a folder
// with a broken contract is indistinguishable from a folder with none.
//
// The claim carries the reason and the loader's error rather than a finished
// sentence. A contract is loaded once, at startup, before any reader has asked
// for anything — so a sentence written here would be written in whichever
// language was guessed then, and a reader who reads in the other one would
// find it sitting under a label in theirs. The surface that shows it knows the
// language; this does not.
func Unreadable(err error) Governance {
	return Governance{claim: Claim{
		outcome:    grantUnresolved,
		reason:     ReasonContractUnreadable,
		cause:      err,
		diagnostic: unreadableDiagnostic(err),
	}}
}

// unreadableDiagnostic is the operator's own line, in the language every other
// diagnostic this package produces is written in. A surface speaking to a
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
// true for a contract that loaded and for one that could not be read, and it
// stays true for a contract that loaded and left a section out — the claim of
// authority is what makes a vault governed, not the completeness of the claim.
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
// together. They are resolved as a set because a consumer that combined a
// vault-level fault with one zero capability of its own would conclude that
// nothing was excluded — the conclusion this type exists to make unavailable.
type Capabilities struct {
	Navigation NavigationRoles
	Knowledge  KnowledgeScope
	Artifacts  ArtifactPolicy
	Language   ArticleLanguage
}

// Capabilities resolves this contract's declarations against what the folder
// asserted about the contract as a whole.
//
// A contract that loaded answers for itself. A folder with no contract answers
// with the zero capabilities: it declared nothing, every declared set is empty,
// and a projection over an empty set is answerable. A contract that claimed
// authority and could not be read answers with none of that — the sets it would
// have carried are unknown, not empty, so every projection over them closes.
// Resolving this in one place is what stops a consumer from combining a
// vault-level fault with a zero capability and concluding that nothing was
// excluded.
//
// Each withheld capability carries the vault-level sentence, not silence: a
// surface that can only report through one capability — a search answering a
// metadata filter, say — would otherwise have to choose between claiming zero
// results and saying nothing at all, and zero results is a lie. Surfaces that
// can show two of them collapse the repetition themselves.
//
// Language is the one exception, and returns its plain zero value instead: it
// carries no Available or Diagnostic of its own for a claim to feed, so a
// withheld generation would leave that claim with nothing to report through —
// the same "not declared" state Resolve already returns for a folder with no
// contract at all.
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
