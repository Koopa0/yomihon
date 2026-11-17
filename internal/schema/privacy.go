package schema

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/koopa0/yomihon/internal/vault"
)

// undeclaredPrivacyDiagnostic is said only about a contract that exists and
// left the section out. Egress stays closed either way: permission is positive
// authority and silence is not permission.
const undeclaredPrivacyDiagnostic = "the rules file declares no privacy policy; agent-facing output disabled until it does"

const stalePrivacyDiagnostic = "vault privacy policy source changed after startup; agent-facing output disabled until restart"

type privacySection struct {
	NeverEgressDirs []string `toml:"never_egress_dirs"`
}

// PrivacyPolicy is the vault contract's fail-closed egress capability. Its zero
// value is unavailable, and callers ask the positive EgressAllowed question so
// a missing or invalid policy cannot be mistaken for permission. It is the one
// capability where an unclaimed declaration and a rejected one behave alike;
// only the reporting differs.
//
// What it binds is the agent-facing output — the check, coverage, exists and
// marks commands, which are its only consumers. The reading pages serve
// every note the folder holds and never ask this question, so a note withheld
// from a command is still served to the reader at the keyboard. Binding the
// pages too would be a different decision, not a repair to this one.
type PrivacyPolicy struct {
	state *privacyPolicyState
}

type privacyPolicyState struct {
	neverEgressDirs []string
	source          policySource
	claim           Claim
	// undeclared marks the one rejection whose repair can be named without
	// quoting the contract: the section is missing, so there is nothing of the
	// vault's own to quote.
	undeclared bool
	// absence is the reader that found no rules file, set only on the policy
	// of a folder that carries none. It is what ValidateSource asks again.
	absence *vault.Reader
	stale   atomic.Bool
	// nearby is set with stale when the policy was withdrawn because a walk of
	// the folder entered one that holds a rules file.
	nearby atomic.Bool
}

// Claim reports how far the privacy declaration got.
func (p PrivacyPolicy) Claim() Claim {
	if p.state == nil {
		return Claim{}
	}
	if p.state.stale.Load() {
		return changedClaim(stalePrivacyDiagnostic)
	}
	return p.state.claim
}

// Available reports whether the contract declared a valid privacy policy.
func (p PrivacyPolicy) Available() bool {
	return p.Claim().held()
}

// Undeclared reports whether the contract exists and left the [privacy]
// section out. It is the one refusal a command may explain in full, since the
// repair — declaring the section — quotes nothing the vault wrote.
func (p PrivacyPolicy) Undeclared() bool {
	return p.state != nil && p.state.undeclared && !p.state.stale.Load()
}

// RulesFileNearby reports whether the policy was withdrawn because a folder the
// command read holds a rules file, which the policy was taken without knowing:
// the withheld ground it names is in the output already. It is the one
// withdrawal a command may name, since it quotes nothing the vault wrote.
func (p PrivacyPolicy) RulesFileNearby() bool {
	return p.state != nil && p.state.nearby.Load()
}

// Trustworthy reports whether the never-egress set may be reasoned over.
func (p PrivacyPolicy) Trustworthy() bool {
	return p.Claim().Trustworthy()
}

// Diagnostic explains why the privacy policy could not be honoured. It is empty
// when the policy was read cleanly and empty when nothing declared one.
func (p PrivacyPolicy) Diagnostic() string {
	return p.Claim().Diagnostic()
}

// EgressAllowed reports whether rel is a valid vault-relative path outside
// every never-egress directory. The unavailable and malformed-path cases are
// deliberately false: permission is positive authority, never the absence of
// a deny match.
func (p PrivacyPolicy) EgressAllowed(rel string) bool {
	if !p.Available() || rel == "" || strings.Contains(rel, `\`) {
		return false
	}
	cleaned := path.Clean(vault.NormalizeNFC(rel))
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || path.IsAbs(cleaned) {
		return false
	}
	for _, dir := range p.state.neverEgressDirs {
		if pathHasFoldedPrefix(cleaned, dir) {
			return false
		}
	}
	return true
}

// WithheldDirs is the declared never_egress_dirs, normalized as EgressAllowed
// compares them, or nil while the declaration is not held. The slice is the
// caller's own.
func (p PrivacyPolicy) WithheldDirs() []string {
	if !p.Available() {
		return nil
	}
	return slices.Clone(p.state.neverEgressDirs)
}

// SameDirName reports whether two path components name the same directory. It
// is the one comparison every vault directory scope makes, so no scope drifts
// from another on how a directory name is spelled.
//
// Case folds because a case-insensitive filesystem opens the same file under
// any case spelling; strings.EqualFold folds per rune, so ß matches ẞ but never
// "ss". Composition is deliberately not folded: a scan reports composed paths,
// so a contract spelling a directory in decomposed form matches nothing.
func SameDirName(a, b string) bool {
	return strings.EqualFold(a, b)
}

// pathHasFoldedPrefix reports whether rel is dir itself or a path below it,
// comparing whole components with [SameDirName]. The fold is unconditional,
// including on case-sensitive filesystems. Exclusion policies therefore exclude
// all case aliases, and domain-root validation rejects declarations that would
// become ambiguous on a case-insensitive filesystem.
func pathHasFoldedPrefix(rel, dir string) bool {
	relComponents := strings.Split(rel, "/")
	dirComponents := strings.Split(dir, "/")
	if len(relComponents) < len(dirComponents) {
		return false
	}
	for index, component := range dirComponents {
		if !SameDirName(relComponents[index], component) {
			return false
		}
	}
	return true
}

// PrivacyWithoutRules is the policy of a folder in which reader found no rules
// file: nothing is withheld, so every path may be said. It is not the zero
// value, which stays unavailable for every other way a policy can be missing —
// a rules file that could not be read withholds ground this process never
// learned, and must go on withholding all of it. Only a reader's own lookup
// finding nothing at the rules path may make this, and ValidateSource looks
// again, so a rules file that appears while a command runs withdraws it.
func PrivacyWithoutRules(reader *vault.Reader) PrivacyPolicy {
	return PrivacyPolicy{state: &privacyPolicyState{claim: heldClaim(), absence: reader}}
}

// WithoutRules reports whether p is the policy of a folder that carries no
// rules file, so a command can say that nothing is withheld because nothing was
// declared.
func (p PrivacyPolicy) WithoutRules() bool {
	return p.state != nil && p.state.absence != nil && p.Available()
}

// revalidateAbsence asks again whether the rules file is still not there, at
// the root and in every folder the reader's walks have entered since: those
// walks are what the output was read from, so a vault moved in after the first
// look and before they reached it is found here, in the listing that found its
// notes.
func (p PrivacyPolicy) revalidateAbsence() PrivacyPolicy {
	_, err := p.state.absence.Lookup(ContractRelPath)
	switch {
	case err == nil:
		// A rules file has appeared since this policy was taken. What it
		// withholds is not known here, so nothing may go out under a policy that
		// came before it.
		p.state.stale.Store(true)
		return p
	case errors.Is(err, fs.ErrNotExist):
	default:
		return PrivacyPolicy{state: &privacyPolicyState{claim: unreadableClaim(err)}}
	}
	held, watchErr := p.state.absence.Watched()
	switch {
	case held:
		p.state.nearby.Store(true)
		p.state.stale.Store(true)
	case watchErr != nil:
		return PrivacyPolicy{state: &privacyPolicyState{claim: unreadableClaim(watchErr)}}
	}
	return p
}

// ValidateSource returns p only while the contract source it was derived from
// is unchanged. Copies of one Contract share a one-way stale latch, so once any
// consumer observes drift all agent-facing output stays unavailable. A source
// that cannot be read at all withholds this caller's answer and nothing more:
// it is no evidence the declaration was withdrawn, and the next caller reads
// the file again.
func (p PrivacyPolicy) ValidateSource() PrivacyPolicy {
	if p.state == nil || !p.state.claim.held() || p.state.stale.Load() {
		return p
	}
	if p.state.absence != nil {
		return p.revalidateAbsence()
	}
	same, err := p.state.source.reread()
	if err != nil {
		return PrivacyPolicy{state: &privacyPolicyState{claim: unreadableClaim(err)}}
	}
	if !same {
		p.state.stale.Store(true)
	}
	return p
}

func derivePrivacyPolicy(section *privacySection, dirsDefined bool, source policySource) PrivacyPolicy {
	// An existing contract that omits the section left an input out; a folder
	// with no contract yields the silent zero value.
	if section == nil {
		return PrivacyPolicy{state: &privacyPolicyState{claim: Rejected(undeclaredPrivacyDiagnostic), undeclared: true}}
	}
	if !dirsDefined {
		return PrivacyPolicy{state: &privacyPolicyState{claim: Rejected(`invalid privacy policy: missing required key "never_egress_dirs"`)}}
	}
	dirs := make([]string, 0, len(section.NeverEgressDirs))
	for _, original := range section.NeverEgressDirs {
		normalized, ok := normalizeDeclaredDir(original)
		if !ok {
			return invalidPrivacyPolicy(original)
		}
		dirs = append(dirs, normalized)
	}
	return PrivacyPolicy{state: &privacyPolicyState{
		neverEgressDirs: dirs,
		source:          source,
		claim:           heldClaim(),
	}}
}

func invalidPrivacyPolicy(value string) PrivacyPolicy {
	return PrivacyPolicy{state: &privacyPolicyState{
		claim: Rejected(fmt.Sprintf("invalid privacy policy: never_egress_dirs contains %q", value)),
	}}
}
