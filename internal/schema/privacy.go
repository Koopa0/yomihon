package schema

import (
	"fmt"
	"path"
	"strings"
	"sync/atomic"

	"github.com/koopa0/yomihon/internal/vault"
)

// undeclaredPrivacyDiagnostic is said only about a contract that exists and
// left the section out. A folder carrying no contract asserted nothing, so it
// reports nothing — egress stays closed either way, because permission is
// positive authority and silence is not permission.
const undeclaredPrivacyDiagnostic = "contract declares no privacy policy; agent-facing output disabled until it does"

const stalePrivacyDiagnostic = "vault privacy policy source changed after startup; agent-facing output disabled until restart"

type privacySection struct {
	NeverEgressDirs []string `toml:"never_egress_dirs"`
}

// PrivacyPolicy is the vault contract's fail-closed egress capability. Its
// zero value is unavailable. Callers ask the positive EgressAllowed question
// so a missing or invalid policy cannot be mistaken for permission.
//
// Egress is the one place where an unclaimed declaration and a rejected one
// behave identically: sending bytes off the machine is an act, and without a
// held declaration yomihon cannot know what must never leave. Only the
// reporting differs — a rejected declaration is news, an absent one is not.
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

// SameDirName reports whether two path components name the same directory.
// It is the one comparison every vault directory scope makes — the privacy
// policy's never-egress set, the artifact policy's non-instance set, and the
// knowledge layer's first-segment membership — so no scope can drift from
// another on how a directory name is spelled.
//
// Case folds because a case-insensitive filesystem opens the same file under
// any case spelling, which makes a scope that depended on the spelling depend
// on a coincidence: the same note would be governed or ungoverned according to
// which case its owner happened to type into the contract. strings.EqualFold
// applies Unicode simple case folding per rune, so ß matches ẞ but never "ss".
//
// Composition is a separate dimension and is deliberately not folded here: the
// decomposed and composed spellings of one name stay different components. A
// scan reports composed paths, so a contract that spells a directory in
// decomposed form matches nothing — which the knowledge layer now reports
// rather than silently applying to no file.
func SameDirName(a, b string) bool {
	return strings.EqualFold(a, b)
}

// pathHasFoldedPrefix reports whether rel is dir itself or a path below it,
// comparing whole components with SameDirName. It is the single directory
// membership identity for both vault directory policies — the privacy policy's
// never-egress set and the artifact policy's non-instance set — so the two
// cannot drift apart on any normalization dimension.
//
// The fold is unconditional, which costs something on a case-sensitive
// filesystem: two genuinely distinct sibling directories differing only in
// case are treated as one, so a declared "System/templates" also covers a
// separate "system/templates" that the scan would otherwise govern. Both
// policies are exclusion sets, so the error is always toward excluding more —
// a refusal or a withheld artifact, never a write to the wrong file or an
// unintended egress — and reading a spelling out of the classification is the
// direction that cannot be recovered from.
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

// ValidateSource returns p only while the exact contract source from which it
// was derived is unchanged. Every copy derived from one Contract shares the
// same one-way stale latch, so once any consumer observes drift, all
// agent-facing output remains unavailable until a freshly loaded Contract
// replaces it.
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
