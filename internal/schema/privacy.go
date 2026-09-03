package schema

import (
	"fmt"
	"path"
	"strings"
	"sync/atomic"

	"github.com/koopa0/yomihon/internal/vault"
)

// undeclaredPrivacyDiagnostic is said only about a contract that exists and
// left the section out. Egress stays closed either way: permission is positive
// authority and silence is not permission.
const undeclaredPrivacyDiagnostic = "contract declares no privacy policy; agent-facing output disabled until it does"

const stalePrivacyDiagnostic = "vault privacy policy source changed after startup; agent-facing output disabled until restart"

type privacySection struct {
	NeverEgressDirs []string `toml:"never_egress_dirs"`
}

// PrivacyPolicy is the vault contract's fail-closed egress capability. Its zero
// value is unavailable, and callers ask the positive EgressAllowed question so
// a missing or invalid policy cannot be mistaken for permission. It is the one
// capability where an unclaimed declaration and a rejected one behave alike;
// only the reporting differs.
type PrivacyPolicy struct {
	state *privacyPolicyState
}

type privacyPolicyState struct {
	neverEgressDirs []string
	source          policySource
	claim           Claim
	stale           atomic.Bool
}

// Claim reports how far the privacy declaration got.
func (p PrivacyPolicy) Claim() Claim {
	if p.state == nil {
		return Claim{}
	}
	if p.state.stale.Load() {
		return Rejected(stalePrivacyDiagnostic)
	}
	return p.state.claim
}

// Available reports whether the contract declared a valid privacy policy.
func (p PrivacyPolicy) Available() bool {
	return p.Claim().held()
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
// comparing whole components with SameDirName. The fold is unconditional, so on
// a case-sensitive filesystem two distinct siblings differing only in case are
// treated as one. Both policies are exclusion sets, so that errs toward
// excluding more — never toward an unintended egress.
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

// ValidateSource returns p only while the contract source it was derived from
// is unchanged. Copies of one Contract share a one-way stale latch, so once any
// consumer observes drift all agent-facing output stays unavailable.
func (p PrivacyPolicy) ValidateSource() PrivacyPolicy {
	if p.state == nil || !p.state.claim.held() || p.state.stale.Load() {
		return p
	}
	if !p.state.source.unchanged() {
		p.state.stale.Store(true)
	}
	return p
}

func derivePrivacyPolicy(section *privacySection, dirsDefined bool, source policySource) PrivacyPolicy {
	// An existing contract that omits the section left an input out; a folder
	// with no contract yields the silent zero value.
	if section == nil {
		return PrivacyPolicy{state: &privacyPolicyState{claim: Rejected(undeclaredPrivacyDiagnostic)}}
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
