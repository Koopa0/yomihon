package schema

import "github.com/koopa0/yomihon/internal/wording"

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

// Claim is one declaration's outcome: how far it got, and the operator-facing
// sentence when the outcome is news. The zero Claim is unclaimed and silent.
//
// A Claim can be closed from outside this package but never opened: authority
// comes from a contract that was read, never from a caller asserting it.
type Claim struct {
	outcome    grant
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

// Claimed reports whether anything ever asserted this at all.
func (c Claim) Claimed() bool { return c.outcome != grantUnclaimed }

// Held reports whether the declaration was read cleanly and is usable.
func (c Claim) Held() bool { return c.outcome == grantHeld }

// Trustworthy reports whether a projection over this declaration's set may be
// answered. It holds for a declaration that was read cleanly and for one that
// was never made — an undeclared set is the empty set. It fails only when an
// assertion was made and could not be honoured, because then the set the
// operator intended is unknown.
func (c Claim) Trustworthy() bool { return c.outcome != grantUnresolved }

// Diagnostic is the operator-facing sentence, empty unless there is news.
func (c Claim) Diagnostic() string { return c.diagnostic }

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
// error text is the one loud sentence for the whole vault: without it, a folder
// with a broken contract is indistinguishable from a folder with none.
func Unreadable(err error) Governance {
	if err == nil {
		return Governance{claim: Rejected(wording.ContractUnreadable.In(wording.ZhHant))}
	}
	return Governance{claim: Rejected(wording.ContractUnreadablePrefix.In(wording.ZhHant) + err.Error())}
}

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
