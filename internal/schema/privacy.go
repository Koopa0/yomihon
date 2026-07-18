package schema

import (
	"fmt"
	"path"
	"strings"
	"sync/atomic"

	"github.com/koopa0/yomihon/internal/vault"
)

const missingPrivacyDiagnostic = "contract declares no privacy policy; agent-facing output disabled until it does"

const stalePrivacyDiagnostic = "vault privacy policy source changed after startup; agent-facing output disabled until restart"

type privacySection struct {
	NeverEgressDirs []string `toml:"never_egress_dirs"`
}

// PrivacyPolicy is the vault contract's fail-closed egress capability. Its
// zero value is unavailable. Callers ask the positive EgressAllowed question
// so a missing or invalid policy cannot be mistaken for permission.
type PrivacyPolicy struct {
	state *privacyPolicyState
}

type privacyPolicyState struct {
	neverEgressDirs []string
	source          policySource
	available       bool
	diagnostic      string
	stale           atomic.Bool
}

// Available reports whether the contract declared a valid privacy policy.
func (p PrivacyPolicy) Available() bool {
	return p.state != nil && p.state.available && !p.state.stale.Load()
}

// Diagnostic explains why agent-facing output is unavailable.
func (p PrivacyPolicy) Diagnostic() string {
	if p.Available() {
		return ""
	}
	if p.state != nil && p.state.stale.Load() {
		return stalePrivacyDiagnostic
	}
	if p.state != nil && p.state.diagnostic != "" {
		return p.state.diagnostic
	}
	return missingPrivacyDiagnostic
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

func pathHasFoldedPrefix(rel, dir string) bool {
	relComponents := strings.Split(rel, "/")
	dirComponents := strings.Split(dir, "/")
	if len(relComponents) < len(dirComponents) {
		return false
	}
	for index, component := range dirComponents {
		if !strings.EqualFold(relComponents[index], component) {
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
	if p.state == nil || !p.state.available || p.state.stale.Load() {
		return p
	}
	if !p.state.source.unchanged() {
		p.state.stale.Store(true)
	}
	return p
}

func derivePrivacyPolicy(section *privacySection, dirsDefined bool, source policySource) PrivacyPolicy {
	if section == nil {
		return PrivacyPolicy{}
	}
	if !dirsDefined {
		return PrivacyPolicy{state: &privacyPolicyState{diagnostic: `invalid privacy policy: missing required key "never_egress_dirs"`}}
	}
	dirs := make([]string, 0, len(section.NeverEgressDirs))
	for _, original := range section.NeverEgressDirs {
		if original == "" || strings.Contains(original, `\`) {
			return invalidPrivacyPolicy(original)
		}
		normalized := path.Clean(vault.NormalizeNFC(original))
		if normalized == "." || normalized == ".." || strings.HasPrefix(normalized, "../") || path.IsAbs(normalized) {
			return invalidPrivacyPolicy(original)
		}
		dirs = append(dirs, normalized)
	}
	return PrivacyPolicy{state: &privacyPolicyState{
		neverEgressDirs: dirs,
		source:          source,
		available:       true,
	}}
}

func invalidPrivacyPolicy(value string) PrivacyPolicy {
	return PrivacyPolicy{state: &privacyPolicyState{
		diagnostic: fmt.Sprintf("invalid privacy policy: never_egress_dirs contains %q", value),
	}}
}
