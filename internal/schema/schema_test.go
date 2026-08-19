package schema_test

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

func TestContractExposesNoFields(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeFor[schema.Contract]()
	for field := range typ.Fields() {
		if field.IsExported() {
			t.Errorf("schema.Contract exposes field %s; authority must be method-only", field.Name)
		}
	}
}

func TestDefinitionIsDetached(t *testing.T) {
	t.Parallel()

	want := loadFixture(t).Definition()
	contract := loadFixture(t)
	mutated := contract.Definition()

	mutated.Enums.Type[0] = "changed"
	mutated.Enums.Domain[0] = "changed"
	mutated.Enums.SourceKind[0] = "changed"
	mutated.Enums.SourceProvider[0] = "changed"
	mutated.Enums.Level[0] = "changed"
	mutated.Enums.MapKind[0] = "changed"
	mutated.Enums.Status["note"][0] = "changed"

	mutated.Fields.Required[0] = "changed"
	mutated.Fields.RequiredInbox[0] = "changed"
	mutated.Fields.DomainExempt[0] = "changed"
	mutated.Fields.Known[0] = "changed"
	mutated.Fields.LessonOnly[0] = "changed"
	mutated.Fields.StatusGroup["lesson"][0] = "changed"

	mutated.Rules.DomainEqualsFolderUnder[0] = "changed"
	mutated.Rules.ConceptRequiresProvenance[0] = "changed"
	mutated.Scan.KnowledgeDirs[0] = "changed"
	mutated.Scan.SkipBasenames[0] = "changed"

	if diff := cmp.Diff(want, contract.Definition()); diff != "" {
		t.Errorf("Definition() changed after caller mutation (-want +got):\n%s", diff)
	}
}

// TestInboxRequiredFieldsIsDetached asserts a caller that edits the returned
// list edits its own copy, the same guarantee Definition() gives.
func TestInboxRequiredFieldsIsDetached(t *testing.T) {
	t.Parallel()

	contract := loadFixture(t)
	_, want, _ := contract.InboxRequiredFields()
	_, mutated, _ := contract.InboxRequiredFields()
	if len(mutated) == 0 {
		t.Fatal("fixture declares no inbox required fields; the detachment claim would be vacuous")
	}
	mutated[0] = "changed"

	_, got, _ := contract.InboxRequiredFields()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("InboxRequiredFields() changed after caller mutation (-want +got):\n%s", diff)
	}
}

func TestZeroContractCarriesNoAuthority(t *testing.T) {
	t.Parallel()

	var contract schema.Contract
	if got := contract.Version(); got != "" {
		t.Errorf("Version() = %q, want empty", got)
	}
	if got := contract.StageCount(); got != 0 {
		t.Errorf("StageCount() = %d, want 0", got)
	}
	if got := contract.StatusGroup(""); got != "" {
		t.Errorf(`StatusGroup("") = %q, want empty`, got)
	}
	if got := contract.Statuses(""); got != nil {
		t.Errorf(`Statuses("") = %v, want nil`, got)
	}
	if _, ok := contract.Stage("lesson", "draft"); ok {
		t.Error("Stage() on zero Contract = true, want false")
	}
	if err := contract.Transition("lesson", "draft", "ready", "koopa"); !errors.Is(err, schema.ErrUnknownStatus) {
		t.Errorf("Transition() on zero Contract = %v, want %v", err, schema.ErrUnknownStatus)
	}
	if contract.NavigationRoles().Available() {
		t.Error("NavigationRoles() on zero Contract is available")
	}
	if contract.ArtifactPolicy().Available() {
		t.Error("ArtifactPolicy() on zero Contract is available")
	}
	if contract.PrivacyPolicy().Available() {
		t.Error("PrivacyPolicy() on zero Contract is available")
	}
}

// The testdata contract is a loader fixture, not a second schema: runtime
// code only ever reads the real vault contract.
func loadFixture(t *testing.T) *schema.Contract {
	t.Helper()
	s, err := schema.LoadFile(filepath.Join("testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("LoadFile(testdata/contract.toml) = %v", err)
	}
	return s
}

func loadContractText(t *testing.T, navigation, artifacts string) *schema.Contract {
	t.Helper()
	return loadContractTextWithPrivacy(t, navigation, artifacts, "")
}

func loadContractTextWithPrivacy(t *testing.T, navigation, artifacts, privacy string) *schema.Contract {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err := os.WriteFile(path, []byte(contractText(navigation, artifacts, privacy)), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) = %v", path, err)
	}
	s, err := schema.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile(%q) = %v", path, err)
	}
	return s
}

func loadSemanticRootContract(t *testing.T, artifacts, privacy string) (string, *schema.Contract) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll(%q) = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contractText("", artifacts, privacy)), 0o600); err != nil { // #nosec G703 -- path is rooted in t.TempDir
		t.Fatalf("WriteFile(%q) = %v", path, err)
	}
	s, err := schema.Load(root)
	if err != nil {
		t.Fatalf("Load(%q) = %v", root, err)
	}
	return root, s
}

func contractText(navigation, artifacts, privacy string) string {
	return `schema_version = "1"

[enums]
type = ["lesson", "study-path", "moc", "topic-map"]

[enums.status]
note = ["draft"]

[fields]
required = ["title", "type"]
known = ["title", "type"]

[rules]
slug_pattern = "^[a-z]+$"

[scan]
knowledge_dirs = ["Writing"]
` + navigation + artifacts + privacy + `
[[lifecycle]]
status = "draft"
applies_to = ["*"]
from = []
owner = ["koopa"]
`
}

func TestLoadFileDerivesValidCapabilities(t *testing.T) {
	t.Parallel()

	s := loadContractTextWithPrivacy(t, `
[navigation]
path_types = ["study-path"]
map_types = ["moc", "topic-map"]
`, `
[artifacts]
non_instance_dirs = ["System/templates"]
`, `
[privacy]
never_egress_dirs = ["Private"]
`)

	roles := s.NavigationRoles()
	if !roles.Available() {
		t.Fatalf("NavigationRoles().Available() = false, diagnostic %q", roles.Diagnostic())
	}
	if !roles.IsPathType("study-path") {
		t.Error("NavigationRoles().IsPathType(\"study-path\") = false, want true")
	}
	if !roles.IsMapType("moc") || !roles.IsMapType("topic-map") {
		t.Error("NavigationRoles().IsMapType() = false for a declared map type, want true")
	}
	if roles.IsPathType("lesson") || roles.IsMapType("lesson") {
		t.Error("NavigationRoles() classifies undeclared type \"lesson\", want neither role")
	}

	policy := s.ArtifactPolicy()
	if !policy.Available() {
		t.Fatalf("ArtifactPolicy().Available() = false, diagnostic %q", policy.Diagnostic())
	}
	if !policy.IsNonInstance("System/templates/card.md") {
		t.Error("ArtifactPolicy().IsNonInstance(\"System/templates/card.md\") = false, want true")
	}

	privacy := s.PrivacyPolicy()
	if !privacy.Available() {
		t.Fatalf("PrivacyPolicy().Available() = false, diagnostic %q", privacy.Diagnostic())
	}
	if privacy.EgressAllowed("Private/note.md") {
		t.Error("PrivacyPolicy().EgressAllowed(\"Private/note.md\") = true, want false")
	}
	if !privacy.EgressAllowed("Public/note.md") {
		t.Error("PrivacyPolicy().EgressAllowed(\"Public/note.md\") = false, want true")
	}
}

func TestCorpusPolicyFingerprintHashesNormalizedSemantics(t *testing.T) {
	t.Parallel()

	firstRoot, _ := loadSemanticRootContract(t, `
[artifacts]
non_instance_dirs = ["System/templates", "System/./generated", "System/templates"]
`, `
[privacy]
never_egress_dirs = ["Private/journal", "Private", "Private/journal"]
`)
	firstReader, first := loadPinnedSemanticContract(t, firstRoot)
	secondRoot, _ := loadSemanticRootContract(t, `
[artifacts]
non_instance_dirs = ["System/generated", "System/templates"]
`, `
[privacy]
never_egress_dirs = ["Private", "Private/journal"]
`)
	secondReader, second := loadPinnedSemanticContract(t, secondRoot)
	firstHash, firstOK := schema.CorpusPolicyFingerprint(first.ArtifactPolicy(), first.PrivacyPolicy())
	secondHash, secondOK := schema.CorpusPolicyFingerprint(second.ArtifactPolicy(), second.PrivacyPolicy())
	if !firstOK || !secondOK {
		t.Fatal("CorpusPolicyFingerprint() unavailable for valid capabilities")
	}
	if firstHash != secondHash {
		t.Error("equivalent normalized policy sets produced different fingerprints")
	}
	firstSource, firstSourceOK := schema.PolicySourceFingerprint(firstReader, first.ArtifactPolicy(), first.PrivacyPolicy())
	secondSource, secondSourceOK := schema.PolicySourceFingerprint(secondReader, second.ArtifactPolicy(), second.PrivacyPolicy())
	if !firstSourceOK || !secondSourceOK {
		t.Fatal("PolicySourceFingerprint() unavailable for capabilities from one valid contract")
	}
	if firstSource == secondSource {
		t.Error("different exact contract bytes produced the same policy-source fingerprint")
	}
	if got, ok := schema.PolicySourceFingerprint(firstReader, first.ArtifactPolicy(), second.PrivacyPolicy()); ok || got != ([32]byte{}) {
		t.Errorf("mixed-source policy fingerprint = (%x, %v), want zero, false", got, ok)
	}

	changed := loadContractTextWithPrivacy(t, "", `
[artifacts]
non_instance_dirs = ["System/generated", "System/templates"]
`, `
[privacy]
never_egress_dirs = ["Other"]
`)
	changedHash, changedOK := schema.CorpusPolicyFingerprint(changed.ArtifactPolicy(), changed.PrivacyPolicy())
	if !changedOK || changedHash == firstHash {
		t.Error("eligibility change did not change corpus-policy fingerprint")
	}

	if got, ok := schema.CorpusPolicyFingerprint(schema.ArtifactPolicy{}, first.PrivacyPolicy()); ok || got != ([32]byte{}) {
		t.Errorf("unavailable capability fingerprint = (%x, %v), want zero, false", got, ok)
	}
}

func TestPolicySourceFingerprintRejectsCapabilitiesLoadedFromAnotherRoot(t *testing.T) {
	t.Parallel()

	root, pathLoaded := loadSemanticRootContract(t, `
[artifacts]
non_instance_dirs = ["System/templates"]
`, `
[privacy]
never_egress_dirs = ["Private"]
`)
	reader, loaded := loadPinnedSemanticContract(t, root)
	if _, ok := schema.PolicySourceFingerprint(reader, loaded.ArtifactPolicy(), loaded.PrivacyPolicy()); !ok {
		t.Fatal("PolicySourceFingerprint() rejected capabilities loaded from the selected root")
	}
	if got, ok := schema.PolicySourceFingerprint(reader, pathLoaded.ArtifactPolicy(), pathLoaded.PrivacyPolicy()); ok || got != ([32]byte{}) {
		t.Fatalf("PolicySourceFingerprint(path-loaded policies) = (%x, %t), want zero, false", got, ok)
	}
	otherRoot, _ := loadSemanticRootContract(t, `
[artifacts]
non_instance_dirs = ["System/templates"]
`, `
[privacy]
never_egress_dirs = ["Private"]
`)
	otherReader, _ := loadPinnedSemanticContract(t, otherRoot)
	if got, ok := schema.PolicySourceFingerprint(otherReader, loaded.ArtifactPolicy(), loaded.PrivacyPolicy()); ok || got != ([32]byte{}) {
		t.Fatalf("PolicySourceFingerprint(other root) = (%x, %t), want zero, false", got, ok)
	}
}

func loadPinnedSemanticContract(t *testing.T, root string) (*vault.Reader, *schema.Contract) {
	t.Helper()
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	contract, err := schema.LoadReader(t.Context(), reader)
	if err != nil {
		t.Fatalf("LoadReader(%q) error = %v", root, err)
	}
	return reader, contract
}

func TestLoadReaderPinsContractAuthorityToTheSelectedVault(t *testing.T) {
	t.Parallel()
	root, _ := loadSemanticRootContract(t, `
[artifacts]
non_instance_dirs = ["System/templates"]
`, `
[privacy]
never_egress_dirs = ["Private"]
`)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	contract, err := schema.LoadReader(t.Context(), reader)
	if err != nil {
		t.Fatalf("LoadReader() error = %v", err)
	}

	oldRoot := root + "-selected"
	if renameErr := os.Rename(root, oldRoot); renameErr != nil {
		t.Fatalf("rename selected root: %v", renameErr)
	}
	replacement := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	if mkdirErr := os.MkdirAll(filepath.Dir(replacement), 0o750); mkdirErr != nil {
		t.Fatalf("mkdir replacement contract parent: %v", mkdirErr)
	}
	if writeErr := os.WriteFile(replacement, []byte("not the selected contract\n"), 0o600); writeErr != nil {
		t.Fatalf("write replacement contract: %v", writeErr)
	}
	if !contract.PrivacyPolicy().ValidateSource().Available() {
		t.Fatal("root-path replacement changed authority behind the pinned reader")
	}

	selectedContract := filepath.Join(oldRoot, filepath.FromSlash(schema.ContractRelPath))
	data, err := os.ReadFile(selectedContract) // #nosec G304 -- selectedContract is rooted in t.TempDir
	if err != nil {
		t.Fatalf("read selected contract: %v", err)
	}
	if err := os.WriteFile(selectedContract, append(data, '\n'), 0o600); err != nil { // #nosec G703 -- selectedContract is rooted in t.TempDir
		t.Fatalf("change selected contract: %v", err)
	}
	if contract.PrivacyPolicy().ValidateSource().Available() {
		t.Fatal("policy remained available after the selected contract changed")
	}
}

func TestLoadReaderRejectsSymlinkedContract(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	contractPath := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	if err := os.MkdirAll(filepath.Dir(contractPath), 0o750); err != nil {
		t.Fatalf("mkdir contract parent: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "contract.toml")
	if err := os.WriteFile(outside, []byte(contractText("", "", "")), 0o600); err != nil {
		t.Fatalf("write outside contract: %v", err)
	}
	if err := os.Symlink(outside, contractPath); err != nil {
		t.Fatalf("symlink contract: %v", err)
	}
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	if contract, err := schema.LoadReader(t.Context(), reader); err == nil || contract != nil {
		t.Fatalf("LoadReader(symlink) = (%v, %v), want nil error", contract, err)
	}
}

func TestArtifactPolicySourceDriftLatchesAcrossCopies(t *testing.T) {
	t.Parallel()

	root, contract := loadSemanticRootContract(t, `
[artifacts]
non_instance_dirs = ["System/templates"]
`, `
[privacy]
never_egress_dirs = ["Private"]
`)
	path := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	original, err := os.ReadFile(path) // #nosec G304 -- path is rooted in t.TempDir
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	first := contract.ArtifactPolicy()
	second := contract.ArtifactPolicy()
	if err = os.WriteFile(path, append(original, '\n'), 0o600); err != nil { // #nosec G703 -- path is rooted in t.TempDir
		t.Fatalf("WriteFile(changed contract) error = %v", err)
	}
	if first.ValidateSource().Available() {
		t.Fatal("first policy copy remained available after source drift")
	}
	if err = os.WriteFile(path, original, 0o600); err != nil { // #nosec G703 -- path is rooted in t.TempDir
		t.Fatalf("WriteFile(restored contract) error = %v", err)
	}
	if second.ValidateSource().Available() {
		t.Error("second policy copy reopened after source restoration, want shared latch")
	}
	const want = "vault artifact policy source changed after startup; instance projections disabled until restart"
	if got := second.Diagnostic(); got != want {
		t.Errorf("second policy diagnostic = %q, want %q", got, want)
	}

	reloaded, err := schema.Load(root)
	if err != nil {
		t.Fatalf("Load(restored contract) error = %v", err)
	}
	if !reloaded.ArtifactPolicy().Available() {
		t.Errorf("freshly loaded policy remained unavailable: %s", reloaded.ArtifactPolicy().Diagnostic())
	}
}

func TestArtifactPolicyCaptureIsAnImmutableRequestSnapshot(t *testing.T) {
	t.Parallel()

	root, contract := loadSemanticRootContract(t, `
[artifacts]
non_instance_dirs = ["System/templates"]
`, `
[privacy]
never_egress_dirs = ["Private"]
`)
	captured := contract.ArtifactPolicy().Capture()
	if !captured.Available() || !captured.IsNonInstance("System/templates/Example.md") {
		t.Fatalf("Capture() did not preserve the valid artifact classification")
	}

	path := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	original, err := os.ReadFile(path) // #nosec G304 -- path is rooted in t.TempDir
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if err = os.WriteFile(path, append(original, '\n'), 0o600); err != nil { // #nosec G703 -- path is rooted in t.TempDir
		t.Fatalf("WriteFile(changed contract) error = %v", err)
	}
	if contract.ArtifactPolicy().ValidateSource().Available() {
		t.Fatal("source-bound policy remained available after drift")
	}
	if !captured.ValidateSource().Available() || !captured.IsNonInstance("System/templates/Example.md") {
		t.Error("captured request policy changed after its source drifted")
	}
}

func TestArtifactPolicySourceDriftLatchIsConcurrentSafe(t *testing.T) {
	t.Parallel()

	root, contract := loadSemanticRootContract(t, `
[artifacts]
non_instance_dirs = ["System/templates"]
`, `
[privacy]
never_egress_dirs = ["Private"]
`)
	path := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	original, err := os.ReadFile(path) // #nosec G304 -- path is rooted in t.TempDir
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if err = os.WriteFile(path, append(original, '\n'), 0o600); err != nil { // #nosec G703 -- path is rooted in t.TempDir
		t.Fatalf("WriteFile(changed contract) error = %v", err)
	}

	policies := make([]schema.ArtifactPolicy, 32)
	for i := range policies {
		policies[i] = contract.ArtifactPolicy()
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range policies {
		policy := policies[i]
		wg.Go(func() {
			<-start
			for range 128 {
				if i%2 == 0 {
					_ = policy.ValidateSource().Available()
					continue
				}
				_ = policy.Available()
				_ = policy.Diagnostic()
			}
		})
	}
	close(start)
	wg.Wait()

	for i, policy := range policies {
		if policy.Available() {
			t.Errorf("policy %d remained available after concurrent source-drift validation", i)
		}
	}
}

func TestPrivacyPolicySourceDriftLatchesAcrossCopies(t *testing.T) {
	t.Parallel()

	root, contract := loadSemanticRootContract(t, `
[artifacts]
non_instance_dirs = ["System/templates"]
`, `
[privacy]
never_egress_dirs = ["Private"]
`)
	path := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	original, err := os.ReadFile(path) // #nosec G304 -- path is rooted in t.TempDir
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	first := contract.PrivacyPolicy()
	second := contract.PrivacyPolicy()
	if err = os.WriteFile(path, append(original, '\n'), 0o600); err != nil { // #nosec G703 -- path is rooted in t.TempDir
		t.Fatalf("WriteFile(changed contract) error = %v", err)
	}
	if first.ValidateSource().Available() {
		t.Fatal("first privacy policy copy remained available after source drift")
	}
	if err = os.WriteFile(path, original, 0o600); err != nil { // #nosec G703 -- path is rooted in t.TempDir
		t.Fatalf("WriteFile(restored contract) error = %v", err)
	}
	if second.ValidateSource().Available() {
		t.Error("second privacy policy copy reopened after source restoration, want shared latch")
	}
	const want = "vault privacy policy source changed after startup; agent-facing output disabled until restart"
	if got := second.Diagnostic(); got != want {
		t.Errorf("second privacy policy diagnostic = %q, want %q", got, want)
	}

	reloaded, err := schema.Load(root)
	if err != nil {
		t.Fatalf("Load(restored contract) error = %v", err)
	}
	if !reloaded.PrivacyPolicy().Available() {
		t.Errorf("freshly loaded privacy policy remained unavailable: %s", reloaded.PrivacyPolicy().Diagnostic())
	}
}

func TestPrivacyPolicySourceDriftLatchIsConcurrentSafe(t *testing.T) {
	t.Parallel()

	root, contract := loadSemanticRootContract(t, `
[artifacts]
non_instance_dirs = ["System/templates"]
`, `
[privacy]
never_egress_dirs = ["Private"]
`)
	path := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	original, err := os.ReadFile(path) // #nosec G304 -- path is rooted in t.TempDir
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if err = os.WriteFile(path, append(original, '\n'), 0o600); err != nil { // #nosec G703 -- path is rooted in t.TempDir
		t.Fatalf("WriteFile(changed contract) error = %v", err)
	}

	policies := make([]schema.PrivacyPolicy, 32)
	for i := range policies {
		policies[i] = contract.PrivacyPolicy()
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range policies {
		policy := policies[i]
		wg.Go(func() {
			<-start
			for range 128 {
				if i%2 == 0 {
					_ = policy.ValidateSource().Available()
					continue
				}
				_ = policy.Available()
				_ = policy.Diagnostic()
			}
		})
	}
	close(start)
	wg.Wait()

	for i, policy := range policies {
		if policy.Available() {
			t.Errorf("privacy policy %d remained available after concurrent source-drift validation", i)
		}
	}
}

func TestLoadFileMissingOptionalSections(t *testing.T) {
	t.Parallel()

	s := loadContractText(t, "", "")
	roles := s.NavigationRoles()
	if roles.Available() {
		t.Fatal("NavigationRoles().Available() = true, want false")
	}
	const wantNavigation = "contract declares no navigation roles; Paths and Maps disabled until it does"
	if got := roles.Diagnostic(); got != wantNavigation {
		t.Errorf("NavigationRoles().Diagnostic() = %q, want %q", got, wantNavigation)
	}
	policy := s.ArtifactPolicy()
	if policy.Available() {
		t.Fatal("ArtifactPolicy().Available() = true, want false")
	}
	const wantArtifact = "contract declares no artifact policy; instance projections disabled until it does"
	if got := policy.Diagnostic(); got != wantArtifact {
		t.Errorf("ArtifactPolicy().Diagnostic() = %q, want %q", got, wantArtifact)
	}

	privacy := s.PrivacyPolicy()
	if privacy.Available() {
		t.Fatal("PrivacyPolicy().Available() = true, want false")
	}
	const wantPrivacy = "contract declares no privacy policy; agent-facing output disabled until it does"
	if got := privacy.Diagnostic(); got != wantPrivacy {
		t.Errorf("PrivacyPolicy().Diagnostic() = %q, want %q", got, wantPrivacy)
	}
	if privacy.EgressAllowed("Public/note.md") {
		t.Error("unavailable PrivacyPolicy().EgressAllowed() = true, want fail-closed false")
	}
	for _, trustworthy := range []struct {
		name string
		got  bool
	}{
		{"navigation roles", roles.Trustworthy()},
		{"artifact policy", policy.Trustworthy()},
		{"privacy policy", privacy.Trustworthy()},
	} {
		if trustworthy.got {
			t.Errorf("%s of a contract that omitted the section is trustworthy; the operator meant to govern something yomihon cannot read", trustworthy.name)
		}
	}
}

// TestZeroCapabilitiesAreSilentAndFailClosed pins the folder that carries no
// contract at all. Nothing asserted governance there, so nothing is reported —
// but reporting nothing is not permission: egress and the classification of a
// governed instance still require a declaration that was actually read.
func TestZeroCapabilitiesAreSilentAndFailClosed(t *testing.T) {
	t.Parallel()

	var roles schema.NavigationRoles
	var policy schema.ArtifactPolicy
	var privacy schema.PrivacyPolicy

	for _, silent := range []struct {
		name string
		got  string
	}{
		{"NavigationRoles", roles.Diagnostic()},
		{"ArtifactPolicy", policy.Diagnostic()},
		{"PrivacyPolicy", privacy.Diagnostic()},
	} {
		if silent.got != "" {
			t.Errorf("zero %s.Diagnostic() = %q, want silence: nothing ever claimed this", silent.name, silent.got)
		}
	}
	if !roles.Trustworthy() || !policy.Trustworthy() || !privacy.Trustworthy() {
		t.Error("zero capability is untrustworthy; an undeclared set is the empty set, and a projection over it is answerable")
	}
	if roles.Available() || policy.Available() || privacy.Available() {
		t.Error("zero capability reports Available; silence is not a held declaration")
	}
	if privacy.EgressAllowed("Public/note.md") {
		t.Error("zero PrivacyPolicy allows egress; permission is positive authority, never the absence of a deny")
	}
	if policy.IsNonInstance("System/templates/Card.md") {
		t.Error("zero ArtifactPolicy excludes a path it was never told to exclude")
	}
	if roles.IsPathType("study-path") || roles.IsMapType("moc") {
		t.Error("zero NavigationRoles classifies a type nothing declared")
	}
}

// TestStaleArtifactPolicyStopsClassifying pins the one state where the
// trustworthiness gate carries real data behind it. A policy that was read
// cleanly and whose source bytes then changed still holds the directory list it
// was built from; answering from that list would classify a note against
// exclusions the operator has since rewritten. The gate is what stops it, so it
// is asserted directly rather than left to every caller's own check.
func TestStaleArtifactPolicyStopsClassifying(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	text := contractText("", "\n[artifacts]\nnon_instance_dirs = [\"System/templates\"]\n", "")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatalf("write contract: %v", err)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	policy := contract.ArtifactPolicy()
	const template = "System/templates/Card.md"
	if !policy.IsNonInstance(template) {
		t.Fatalf("IsNonInstance(%q) = false while the declaration is held", template)
	}

	if err := os.WriteFile(path, []byte(text+"\n"), 0o600); err != nil {
		t.Fatalf("change contract source: %v", err)
	}
	stale := policy.ValidateSource()

	if stale.Trustworthy() {
		t.Fatal("a policy whose source changed is still trustworthy")
	}
	if stale.IsNonInstance(template) {
		t.Errorf("IsNonInstance(%q) = true from a policy whose source it can no longer vouch for", template)
	}
	if stale.Diagnostic() == "" {
		t.Error("a latched-stale policy says nothing; drift after startup is news")
	}
}

func TestPrivacyPolicyRejectsOmittedRequiredKey(t *testing.T) {
	t.Parallel()

	s := loadContractTextWithPrivacy(t, "", "", "\n[privacy]\n")
	policy := s.PrivacyPolicy()
	if policy.Available() {
		t.Fatal("PrivacyPolicy().Available() = true, want false")
	}
	const want = `invalid privacy policy: missing required key "never_egress_dirs"`
	if got := policy.Diagnostic(); got != want {
		t.Errorf("PrivacyPolicy().Diagnostic() = %q, want %q", got, want)
	}
	if policy.EgressAllowed("Public/note.md") {
		t.Error("invalid PrivacyPolicy().EgressAllowed() = true, want fail-closed false")
	}
}

func TestPrivacyPolicyNormalizesAndMatchesComponentPrefixes(t *testing.T) {
	t.Parallel()

	s := loadContractTextWithPrivacy(t, "", "", `
[privacy]
never_egress_dirs = ["Private/./Journal", "私有"]
`)
	policy := s.PrivacyPolicy()
	if !policy.Available() {
		t.Fatalf("PrivacyPolicy().Available() = false, diagnostic %q", policy.Diagnostic())
	}

	tests := []struct {
		path string
		want bool
	}{
		{path: "Private/Journal", want: false},
		{path: "Private/Journal/note.md", want: false},
		{path: "private/JOURNAL/note.md", want: false},
		{path: "Private/Journal-old/note.md", want: true},
		{path: "private/journal-old/note.md", want: true},
		{path: "私有/note.md", want: false},
		{path: "Public/note.md", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			if got := policy.EgressAllowed(tt.path); got != tt.want {
				t.Errorf("PrivacyPolicy().EgressAllowed(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestPrivacyPolicyRejectsUnsafeDirectories(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"", `.`, `..`, `../Private`, `/Private`, `Private\\Journal`} {
		t.Run(strconv.Quote(value), func(t *testing.T) {
			t.Parallel()
			section := fmt.Sprintf("\n[privacy]\nnever_egress_dirs = [%s]\n", strconv.Quote(value))
			s := loadContractTextWithPrivacy(t, "", "", section)
			policy := s.PrivacyPolicy()
			if policy.Available() {
				t.Fatal("PrivacyPolicy().Available() = true, want false")
			}
			if got := policy.Diagnostic(); !strings.Contains(got, strconv.Quote(value)) {
				t.Errorf("PrivacyPolicy().Diagnostic() = %q, want original value named", got)
			}
			if policy.EgressAllowed("Public/note.md") {
				t.Error("invalid PrivacyPolicy().EgressAllowed() = true, want fail-closed false")
			}
		})
	}
}

func TestCapabilitiesRejectOmittedRequiredKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		navigation     string
		artifacts      string
		capability     func(*schema.Contract) (bool, string)
		otherAvailable func(*schema.Contract) bool
		wantDiagnostic string
	}{
		{
			name: "navigation path types",
			navigation: `
[navigation]
map_types = ["moc"]
`,
			artifacts: `
[artifacts]
non_instance_dirs = ["System/templates"]
`,
			capability: func(s *schema.Contract) (bool, string) {
				roles := s.NavigationRoles()
				return roles.Available(), roles.Diagnostic()
			},
			otherAvailable: func(s *schema.Contract) bool { return s.ArtifactPolicy().Available() },
			wantDiagnostic: `invalid navigation roles: missing required key "path_types"; Paths and Maps disabled`,
		},
		{
			name: "navigation map types",
			navigation: `
[navigation]
path_types = ["study-path"]
`,
			artifacts: `
[artifacts]
non_instance_dirs = ["System/templates"]
`,
			capability: func(s *schema.Contract) (bool, string) {
				roles := s.NavigationRoles()
				return roles.Available(), roles.Diagnostic()
			},
			otherAvailable: func(s *schema.Contract) bool { return s.ArtifactPolicy().Available() },
			wantDiagnostic: `invalid navigation roles: missing required key "map_types"; Paths and Maps disabled`,
		},
		{
			name: "navigation both keys",
			navigation: `
[navigation]
`,
			artifacts: `
[artifacts]
non_instance_dirs = ["System/templates"]
`,
			capability: func(s *schema.Contract) (bool, string) {
				roles := s.NavigationRoles()
				return roles.Available(), roles.Diagnostic()
			},
			otherAvailable: func(s *schema.Contract) bool { return s.ArtifactPolicy().Available() },
			wantDiagnostic: `invalid navigation roles: missing required keys "path_types", "map_types"; Paths and Maps disabled`,
		},
		{
			name: "artifact directories",
			navigation: `
[navigation]
path_types = ["study-path"]
map_types = ["moc"]
`,
			artifacts: `
[artifacts]
`,
			capability: func(s *schema.Contract) (bool, string) {
				policy := s.ArtifactPolicy()
				return policy.Available(), policy.Diagnostic()
			},
			otherAvailable: func(s *schema.Contract) bool { return s.NavigationRoles().Available() },
			wantDiagnostic: `invalid artifact policy: missing required key "non_instance_dirs"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := loadContractText(t, tt.navigation, tt.artifacts)
			available, diagnostic := tt.capability(s)
			if available {
				t.Fatal("capability Available() = true with omitted required key, want false")
			}
			if diagnostic != tt.wantDiagnostic {
				t.Errorf("capability Diagnostic() = %q, want %q", diagnostic, tt.wantDiagnostic)
			}
			if !tt.otherAvailable(s) {
				t.Error("independent capability Available() = false, want true")
			}
		})
	}
}

func TestNavigationRolesRejectUnknownPathType(t *testing.T) {
	t.Parallel()

	s := loadContractText(t, `
[navigation]
path_types = ["unknown-path"]
map_types = ["moc"]
`, `
[artifacts]
non_instance_dirs = ["System/templates"]
`)
	roles := s.NavigationRoles()
	if roles.Available() {
		t.Fatal("NavigationRoles().Available() = true, want false")
	}
	if got := roles.Diagnostic(); !strings.Contains(got, `"unknown-path"`) {
		t.Errorf("NavigationRoles().Diagnostic() = %q, want offending type named", got)
	}
	if roles.IsMapType("moc") {
		t.Error("NavigationRoles().IsMapType(\"moc\") = true after invalid path type, want entire role set unavailable")
	}
	if !s.ArtifactPolicy().Available() {
		t.Errorf("ArtifactPolicy().Available() = false after invalid navigation roles, diagnostic %q", s.ArtifactPolicy().Diagnostic())
	}
}

func TestNavigationRolesRejectUnknownMapType(t *testing.T) {
	t.Parallel()

	s := loadContractText(t, `
[navigation]
path_types = ["study-path"]
map_types = ["unknown-map"]
`, `
[artifacts]
non_instance_dirs = ["System/templates"]
`)
	roles := s.NavigationRoles()
	if roles.Available() {
		t.Fatal("NavigationRoles().Available() = true, want false")
	}
	if got := roles.Diagnostic(); !strings.Contains(got, `"unknown-map"`) {
		t.Errorf("NavigationRoles().Diagnostic() = %q, want offending type named", got)
	}
	if roles.IsPathType("study-path") {
		t.Error("NavigationRoles().IsPathType(\"study-path\") = true after invalid map type, want entire role set unavailable")
	}
}

func TestNavigationRolesRejectDuplicatePathType(t *testing.T) {
	t.Parallel()

	s := loadContractText(t, `
[navigation]
path_types = ["study-path", "study-path"]
map_types = ["moc"]
`, `
[artifacts]
non_instance_dirs = ["System/templates"]
`)
	roles := s.NavigationRoles()
	if roles.Available() {
		t.Fatal("NavigationRoles().Available() = true, want false")
	}
	if got := roles.Diagnostic(); !strings.Contains(got, `"study-path"`) {
		t.Errorf("NavigationRoles().Diagnostic() = %q, want duplicate type named", got)
	}
	if roles.IsMapType("moc") {
		t.Error("NavigationRoles().IsMapType(\"moc\") = true after duplicate path type, want entire role set unavailable")
	}
}

func TestNavigationRolesRejectDuplicateMapType(t *testing.T) {
	t.Parallel()

	s := loadContractText(t, `
[navigation]
path_types = ["study-path"]
map_types = ["moc", "moc"]
`, `
[artifacts]
non_instance_dirs = ["System/templates"]
`)
	roles := s.NavigationRoles()
	if roles.Available() {
		t.Fatal("NavigationRoles().Available() = true, want false")
	}
	if got := roles.Diagnostic(); !strings.Contains(got, `"moc"`) {
		t.Errorf("NavigationRoles().Diagnostic() = %q, want duplicate type named", got)
	}
	if roles.IsPathType("study-path") {
		t.Error("NavigationRoles().IsPathType(\"study-path\") = true after duplicate map type, want entire role set unavailable")
	}
}

func TestNavigationRolesRejectPathMapOverlap(t *testing.T) {
	t.Parallel()

	s := loadContractText(t, `
[navigation]
path_types = ["study-path"]
map_types = ["study-path", "moc"]
`, `
[artifacts]
non_instance_dirs = ["System/templates"]
`)
	roles := s.NavigationRoles()
	if roles.Available() {
		t.Fatal("NavigationRoles().Available() = true, want false")
	}
	if got := roles.Diagnostic(); !strings.Contains(got, `"study-path"`) {
		t.Errorf("NavigationRoles().Diagnostic() = %q, want overlapping type named", got)
	}
	if roles.IsMapType("moc") || roles.IsPathType("study-path") {
		t.Error("NavigationRoles() retained a partial role after path/map overlap, want entire role set unavailable")
	}
}

func TestArtifactPolicyRejectsEmptyDirectory(t *testing.T) {
	t.Parallel()

	s := loadContractText(t, `
[navigation]
path_types = ["study-path"]
map_types = ["moc"]
`, `
[artifacts]
non_instance_dirs = [""]
`)
	policy := s.ArtifactPolicy()
	if policy.Available() {
		t.Fatal("ArtifactPolicy().Available() = true, want false")
	}
	if got := policy.Diagnostic(); !strings.Contains(got, `""`) {
		t.Errorf("ArtifactPolicy().Diagnostic() = %q, want offending empty value named", got)
	}
	if !s.NavigationRoles().Available() {
		t.Errorf("NavigationRoles().Available() = false after invalid artifact policy, diagnostic %q", s.NavigationRoles().Diagnostic())
	}
}

func TestArtifactPolicyRejectsCurrentDirectory(t *testing.T) {
	t.Parallel()

	s := loadContractText(t, `
[navigation]
path_types = []
map_types = []
`, `
[artifacts]
non_instance_dirs = ["."]
`)
	policy := s.ArtifactPolicy()
	if policy.Available() {
		t.Fatal("ArtifactPolicy().Available() = true, want false")
	}
	if got := policy.Diagnostic(); !strings.Contains(got, `"."`) {
		t.Errorf("ArtifactPolicy().Diagnostic() = %q, want offending value named", got)
	}
}

func TestArtifactPolicyRejectsParentDirectory(t *testing.T) {
	t.Parallel()

	s := loadContractText(t, `
[navigation]
path_types = []
map_types = []
`, `
[artifacts]
non_instance_dirs = [".."]
`)
	policy := s.ArtifactPolicy()
	if policy.Available() {
		t.Fatal("ArtifactPolicy().Available() = true, want false")
	}
	if got := policy.Diagnostic(); !strings.Contains(got, `".."`) {
		t.Errorf("ArtifactPolicy().Diagnostic() = %q, want offending value named", got)
	}
}

func TestArtifactPolicyRejectsAbsoluteDirectory(t *testing.T) {
	t.Parallel()

	s := loadContractText(t, `
[navigation]
path_types = []
map_types = []
`, `
[artifacts]
non_instance_dirs = ["/System/templates"]
`)
	policy := s.ArtifactPolicy()
	if policy.Available() {
		t.Fatal("ArtifactPolicy().Available() = true, want false")
	}
	if got := policy.Diagnostic(); !strings.Contains(got, `"/System/templates"`) {
		t.Errorf("ArtifactPolicy().Diagnostic() = %q, want offending value named", got)
	}
}

func TestArtifactPolicyRejectsBackslashDirectory(t *testing.T) {
	t.Parallel()

	s := loadContractText(t, `
[navigation]
path_types = []
map_types = []
`, `
[artifacts]
non_instance_dirs = ["System\\templates"]
`)
	policy := s.ArtifactPolicy()
	if policy.Available() {
		t.Fatal("ArtifactPolicy().Available() = true, want false")
	}
	if got := policy.Diagnostic(); !strings.Contains(got, `System\\templates`) {
		t.Errorf("ArtifactPolicy().Diagnostic() = %q, want offending value named", got)
	}
}

func TestArtifactPolicyRejectsNonLocalNormalizedDirectory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		directory string
	}{
		{name: "slash normalizes to current directory", directory: "./"},
		{name: "components normalize to current directory", directory: "a/.."},
		{name: "parent prefix", directory: "../outside"},
		{name: "components normalize to parent prefix", directory: "a/../../outside"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := loadContractText(t, `
[navigation]
path_types = []
map_types = []
`, fmt.Sprintf(`
[artifacts]
non_instance_dirs = [%s]
`, strconv.Quote(tt.directory)))
			policy := s.ArtifactPolicy()
			if policy.Available() {
				t.Fatal("ArtifactPolicy().Available() = true, want false")
			}
			if got, want := policy.Diagnostic(), strconv.Quote(tt.directory); !strings.Contains(got, want) {
				t.Errorf("ArtifactPolicy().Diagnostic() = %q, want original offending value %s named", got, want)
			}
		})
	}
}

func TestArtifactPolicyAllowsLocalPathAfterNormalization(t *testing.T) {
	t.Parallel()

	s := loadContractText(t, `
[navigation]
path_types = []
map_types = []
`, `
[artifacts]
non_instance_dirs = ["a/../b"]
`)
	policy := s.ArtifactPolicy()
	if !policy.Available() {
		t.Fatalf("ArtifactPolicy().Available() = false, diagnostic %q", policy.Diagnostic())
	}
	if !policy.IsNonInstance("b/card.md") {
		t.Error("ArtifactPolicy().IsNonInstance(\"b/card.md\") = false, want normalized directory to match")
	}
}

func TestArtifactPolicyNormalizesAndMatchesComponentPrefixes(t *testing.T) {
	t.Parallel()

	s := loadContractText(t, `
[navigation]
path_types = []
map_types = []
`, `
[artifacts]
non_instance_dirs = ["System//./templates", "Cafe\u0301/models"]
`)
	policy := s.ArtifactPolicy()
	if !policy.Available() {
		t.Fatalf("ArtifactPolicy().Available() = false, diagnostic %q", policy.Diagnostic())
	}
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "normalized directory", path: "System/templates", want: true},
		{name: "normalized child", path: "System/templates/card.md", want: true},
		{name: "component sibling", path: "System/templates-old/card.md", want: false},
		{name: "NFC query", path: "Café/models/card.md", want: true},
		{name: "NFD query", path: "Café/models/card.md", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := policy.IsNonInstance(tt.path); got != tt.want {
				t.Errorf("ArtifactPolicy().IsNonInstance(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// TestDirectoryPoliciesShareCaseFoldedIdentity pins the one path identity both
// directory policies answer from. On a case-insensitive filesystem any case
// spelling of a path opens the same file, so membership in a declared
// directory cannot depend on the spelling: a lowercase request for a protected
// template is the protected template. The privacy policy has always folded
// case for exactly this reason; the artifact policy answers the same question
// and folds identically, through the same function, so the two cannot drift.
func TestDirectoryPoliciesShareCaseFoldedIdentity(t *testing.T) {
	t.Parallel()

	s := loadContractTextWithPrivacy(t, "", `
[artifacts]
non_instance_dirs = ["System/templates"]
`, `
[privacy]
never_egress_dirs = ["Private"]
`)
	policy := s.ArtifactPolicy()
	if !policy.Available() {
		t.Fatalf("ArtifactPolicy().Available() = false, diagnostic %q", policy.Diagnostic())
	}
	privacy := s.PrivacyPolicy()
	if !privacy.Available() {
		t.Fatalf("PrivacyPolicy().Available() = false, diagnostic %q", privacy.Diagnostic())
	}

	artifactTests := []struct {
		path string
		want bool
	}{
		{path: "System/templates/Card.md", want: true},
		{path: "system/templates/card.md", want: true},
		{path: "SYSTEM/Templates/Card.md", want: true},
		{path: "system/templates", want: true},
		{path: "system/templates-old/card.md", want: false},
		{path: "Writing/lessons/L05.md", want: false},
	}
	for _, tt := range artifactTests {
		t.Run("artifact/"+tt.path, func(t *testing.T) {
			t.Parallel()
			if got := policy.IsNonInstance(tt.path); got != tt.want {
				t.Errorf("ArtifactPolicy().IsNonInstance(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}

	privacyTests := []struct {
		path string
		want bool
	}{
		{path: "Private/x.md", want: false},
		{path: "private/x.md", want: false},
		{path: "PRIVATE/x.md", want: false},
		{path: "Public/x.md", want: true},
	}
	for _, tt := range privacyTests {
		t.Run("privacy/"+tt.path, func(t *testing.T) {
			t.Parallel()
			if got := privacy.EgressAllowed(tt.path); got != tt.want {
				t.Errorf("PrivacyPolicy().EgressAllowed(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestCapabilitiesAllowEmptyLists(t *testing.T) {
	t.Parallel()

	s := loadContractText(t, `
[navigation]
path_types = []
map_types = []
`, `
[artifacts]
non_instance_dirs = []
`)
	roles := s.NavigationRoles()
	if !roles.Available() {
		t.Fatalf("NavigationRoles().Available() = false, diagnostic %q", roles.Diagnostic())
	}
	if got := roles.Diagnostic(); got != "" {
		t.Errorf("NavigationRoles().Diagnostic() = %q with explicit empty lists, want empty", got)
	}
	if roles.IsPathType("study-path") || roles.IsMapType("moc") {
		t.Error("NavigationRoles() classifies a type with empty role lists, want neither role")
	}
	policy := s.ArtifactPolicy()
	if !policy.Available() {
		t.Fatalf("ArtifactPolicy().Available() = false, diagnostic %q", policy.Diagnostic())
	}
	if got := policy.Diagnostic(); got != "" {
		t.Errorf("ArtifactPolicy().Diagnostic() = %q with explicit empty list, want empty", got)
	}
	if policy.IsNonInstance("System/templates/card.md") {
		t.Error("ArtifactPolicy().IsNonInstance() = true with empty directory list, want false")
	}
}

func TestNavigationRolesRemainDerivedAfterDefinitionMutation(t *testing.T) {
	t.Parallel()

	s := loadContractText(t, `
[navigation]
path_types = ["study-path"]
map_types = ["moc"]
`, `
[artifacts]
non_instance_dirs = ["System/templates"]
`)
	definition := s.Definition()
	definition.Enums.Type = []string{"lesson"}

	roles := s.NavigationRoles()
	if !roles.IsPathType("study-path") || !roles.IsMapType("moc") {
		t.Error("NavigationRoles() changed after mutating Definition.Enums.Type, want load-time derived membership")
	}
}

func TestCapabilitiesExposeNoMutableBackingCollections(t *testing.T) {
	t.Parallel()

	for _, capability := range []any{schema.NavigationRoles{}, schema.ArtifactPolicy{}, schema.PrivacyPolicy{}} {
		typ := reflect.TypeOf(capability)
		for field := range typ.Fields() {
			if field.IsExported() && (field.Type.Kind() == reflect.Map || field.Type.Kind() == reflect.Slice) {
				t.Errorf("%s.%s exposes mutable %s backing state", typ.Name(), field.Name, field.Type.Kind())
			}
		}
	}
}

func TestSealStatusPinned(t *testing.T) {
	t.Parallel()

	if got := schema.SealStatus; got != "ready" {
		t.Errorf("SealStatus = %q, want %q", got, "ready")
	}
}

// TestStatusValuesAreNeverHardcodedOutsideSchema guards the single-source rule
// for the status state machine. The legal status values are defined once, in
// the vault contract that this package alone reads; no other package in this
// module — cmd included — may pin one of those values as a string literal in
// its logic. Naming the value as a const, at any scope, is allowed — that is a
// single named reference, not a second copy of the value set — so only literals
// used directly in expressions are flagged. The judge package is exempt: it
// reproduces, byte for byte, a frozen external contract whose rule constants
// happen to be status values, pinned by its golden files rather than derived
// from this contract. Generated files are exempt too, except the templ
// outputs: they embed each template source's expression literals
// deterministically, so scanning them covers the templates without parsing
// templ syntax. The forbidden set is the union of the fixture contract and the
// example contract shipped for new vaults, so a value present in either is
// guarded and the test behaves the same on every machine, with or without the
// real vault. That union is also the guard's whole reach: an operator's own
// contract may declare a value neither file lists, and such a value would pass
// unguarded — which is why the fixture carries every status word the product
// itself names, the koopa-only publication value included.
// The set is the bare status words, so an unrelated literal equal
// to one — a log line, a class name — would also trip this guard; that
// trade-off is accepted, since restricting the match to literals compared
// against a status-typed value is far harder to express and the words rarely
// collide.
func TestStatusValuesAreNeverHardcodedOutsideSchema(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	forbidden := map[string]bool{}
	for _, contractPath := range []string{
		filepath.Join("testdata", "contract.toml"),
		filepath.Join(root, "examples", "vault-schema.toml"),
	} {
		s, err := schema.LoadFile(contractPath)
		if err != nil {
			t.Fatalf("LoadFile(%q) = %v", contractPath, err)
		}
		for _, group := range s.Definition().Enums.Status {
			for _, value := range group {
				forbidden[value] = true
			}
		}
	}

	fset := token.NewFileSet()
	var violations []string
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// schema owns the contract; judge reproduces a frozen external one
			// whose rule constants happen to be status values. node_modules
			// holds runner-local lint tooling, not module sources.
			switch {
			case path == filepath.Join(root, ".git"),
				d.Name() == "node_modules",
				path == filepath.Join(root, "internal", "schema"),
				path == filepath.Join(root, "internal", "judge"):
				return filepath.SkipDir
			}
			// A directory carrying its own go.mod is a separate module, so its
			// sources are not the ones this rule governs and a file there that
			// fails to parse would fail this test for a reason outside the tree
			// it speaks for. The boundary is the go.mod, not a directory name,
			// so a second nested module needs no edit here.
			if path != root {
				if _, statErr := os.Stat(filepath.Join(path, "go.mod")); statErr == nil {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.ParseComments|parser.SkipObjectResolution)
		if perr != nil {
			return fmt.Errorf("parse %s: %w", path, perr)
		}
		if ast.IsGenerated(f) && !strings.HasSuffix(path, "_templ.go") {
			return nil
		}
		// A status value on the right of a const declaration names the value in
		// one place; that is the allowed form, at any scope. A single pre-order
		// walk records each const's literal positions before it reaches them as
		// children, so a package-level or function-local const is exempt while
		// every other literal is flagged.
		named := map[token.Pos]bool{}
		ast.Inspect(f, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.GenDecl:
				if node.Tok != token.CONST {
					return true
				}
				for _, spec := range node.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for _, value := range vs.Values {
						if lit, ok := value.(*ast.BasicLit); ok {
							named[lit.ValuePos] = true
						}
					}
				}
			case *ast.BasicLit:
				if node.Kind != token.STRING || named[node.ValuePos] {
					return true
				}
				value, uerr := strconv.Unquote(node.Value)
				if uerr != nil || !forbidden[value] {
					return true
				}
				violations = append(violations, fmt.Sprintf("%s: %q", fset.Position(node.ValuePos), value))
			}
			return true
		})
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk module: %v", walkErr)
	}
	if len(violations) > 0 {
		t.Errorf("a status value is defined only in the vault contract, but these string literals hardcode one outside internal/schema — name it as a const or move the decision behind this package:\n%s",
			strings.Join(violations, "\n"))
	}
}

// moduleRoot walks up from the test's working directory to the directory
// holding go.mod, so the status-literal guard covers the whole module — cmd,
// tools, and the templ outputs — rather than only the tree beneath internal/.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("module root: go.mod not found above the working directory")
		}
		dir = parent
	}
}

func TestStatusGroup(t *testing.T) {
	t.Parallel()
	s := loadFixture(t)

	tests := []struct {
		noteType string
		want     string
	}{
		{"lesson", "lesson"},
		{"system", "system"},
		{"guide", "system"},
		{"concept", "note"},
		{"writing", "note"},
	}
	for _, tt := range tests {
		t.Run(tt.noteType, func(t *testing.T) {
			t.Parallel()
			if got := s.StatusGroup(tt.noteType); got != tt.want {
				t.Errorf("StatusGroup(%q) = %q, want %q", tt.noteType, got, tt.want)
			}
		})
	}
}

func TestStatuses(t *testing.T) {
	t.Parallel()
	s := loadFixture(t)

	got := s.Statuses("lesson")
	want := []string{"imported", "draft", "ready", "published", "archived"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Statuses(\"lesson\") mismatch (-want +got):\n%s", diff)
	}
}

func TestTransition(t *testing.T) {
	t.Parallel()
	s := loadFixture(t)

	tests := []struct {
		name    string
		typ     string
		from    string
		to      string
		actor   string
		wantErr error
	}{
		{"lesson imported→draft by claude", "lesson", "imported", "draft", "claude", nil},
		{"lesson draft→ready by koopa", "lesson", "draft", "ready", "koopa", nil},
		{"ready is koopa-only", "lesson", "draft", "ready", "claude", schema.ErrOwnerForbidden},
		{"skip a stage", "lesson", "imported", "ready", "koopa", schema.ErrIllegalTransition},
		{"archived from anywhere", "concept", "seedling", "archived", "koopa", nil},
		{"initial captured", "transcript", "", "captured", "hermes", nil},
		{"non-initial needs predecessor", "source-note", "", "cleaned", "claude", schema.ErrIllegalTransition},
		{"undefined status for type", "concept", "seedling", "ready", "koopa", schema.ErrUnknownStatus},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := s.Transition(tt.typ, tt.from, tt.to, tt.actor)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Transition(%q, %q, %q, %q) = %v, want %v",
					tt.typ, tt.from, tt.to, tt.actor, err, tt.wantErr)
			}
		})
	}
}

// TestAdvanceableBy exercises the "still has a named, owned onward step"
// predicate on a synthetic contract, so it proves the state-machine logic
// rather than any particular vault's status words. The state names here are
// invented (s1…final, retired) precisely so the test cannot pass by accident of
// matching a real status.
func TestAdvanceableBy(t *testing.T) {
	t.Parallel()

	// s1→s2→s3→final is a linear owned path; retired is the "any state" escape
	// (both its from and applies_to are the wildcard), which must never count as
	// an onward step.
	const contract = `schema_version = "1"

[enums]
type = ["doc", "note"]

[enums.status]
note = ["s1", "s2", "s3", "final", "retired"]

[[lifecycle]]
status = "s1"
applies_to = ["doc"]
from = []
owner = ["bot"]

[[lifecycle]]
status = "s2"
applies_to = ["doc"]
from = ["s1"]
owner = ["editor"]

[[lifecycle]]
status = "s3"
applies_to = ["doc", "note"]
from = ["s2"]
owner = ["editor", "bot"]

[[lifecycle]]
status = "final"
applies_to = ["*"]
from = ["s3"]
owner = ["editor"]

[[lifecycle]]
status = "retired"
applies_to = ["*"]
from = ["*"]
owner = ["editor", "bot"]
`
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err := os.WriteFile(path, []byte(contract), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) = %v", path, err)
	}
	s, err := schema.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile(%q) = %v", path, err)
	}

	tests := []struct {
		name     string
		noteType string
		status   string
		actor    string
		want     bool
	}{
		{"named edge, owner matches", "doc", "s1", "editor", true},
		{"named edge, owner excluded", "doc", "s1", "bot", false},
		{"named edge, type not in applies_to", "note", "s1", "editor", false},
		{"owner list includes actor", "doc", "s2", "bot", true},
		{"applies_to wildcard admits the type", "doc", "s3", "editor", true},
		{"applies_to wildcard rejects unknown type", "anytype", "s3", "editor", false},
		{"only the wildcard escape remains", "doc", "final", "editor", false},
		{"escape state itself has no named onward step", "doc", "retired", "editor", false},
		{"status defined nowhere", "doc", "nope", "editor", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := s.AdvanceableBy(tt.noteType, tt.status, tt.actor); got != tt.want {
				t.Errorf("AdvanceableBy(%q, %q, %q) = %v, want %v",
					tt.noteType, tt.status, tt.actor, got, tt.want)
			}
		})
	}
}

// TestTouchingTheContractLeavesThePolicyAvailable separates two questions the
// pinned identity check had collapsed into one.
//
// The pinned entry records which file the contract was at startup, and that
// record includes the modification time — so a git checkout, a pull, a restored
// backup or an editor that saves by rename moves it without changing a byte.
// The vault this serves is a git repository and yomihon runs git against it, so
// this is an ordinary event, and treating it as a contract change closed the
// write face until restart while reporting a change that had not happened.
func TestTouchingTheContractLeavesThePolicyAvailable(t *testing.T) {
	t.Parallel()
	root, _ := loadSemanticRootContract(t, `
[artifacts]
non_instance_dirs = ["System/templates"]
`, `
[privacy]
never_egress_dirs = ["Private"]
`)
	_, contract := loadPinnedSemanticContract(t, root)
	if !contract.ArtifactPolicy().ValidateSource().Available() {
		t.Fatal("the policy was unavailable before anything touched the contract")
	}

	path := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat contract: %v", err)
	}
	original, err := os.ReadFile(path) // #nosec G304 -- path is rooted in t.TempDir
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	moved := before.ModTime().Add(2 * time.Second)
	if err = os.Chtimes(path, moved, moved); err != nil {
		t.Fatalf("move the contract's modification time: %v", err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat contract after: %v", err)
	}
	if after.ModTime().Equal(before.ModTime()) {
		t.Fatal("the modification time did not move, so this test proves nothing")
	}
	current, err := os.ReadFile(path) // #nosec G304 -- path is rooted in t.TempDir
	if err != nil {
		t.Fatalf("re-read contract: %v", err)
	}
	if !bytes.Equal(original, current) {
		t.Fatal("the contract's bytes changed, so this is a different test")
	}

	if !contract.ArtifactPolicy().ValidateSource().Available() {
		t.Error("the artifact policy closed after a touch; no byte of the contract moved")
	}
	if !contract.PrivacyPolicy().ValidateSource().Available() {
		t.Error("the privacy policy closed after a touch; no byte of the contract moved")
	}
}

// TestRewritingTheContractClosesThePolicy is the reverse direction, and it is
// the one that must never soften: a contract whose bytes changed under a running
// process is a contract this process cannot vouch for.
func TestRewritingTheContractClosesThePolicy(t *testing.T) {
	t.Parallel()
	root, _ := loadSemanticRootContract(t, `
[artifacts]
non_instance_dirs = ["System/templates"]
`, `
[privacy]
never_egress_dirs = ["Private"]
`)
	_, contract := loadPinnedSemanticContract(t, root)

	path := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	original, err := os.ReadFile(path) // #nosec G304 -- path is rooted in t.TempDir
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	if err = os.WriteFile(path, append(original, '\n'), 0o600); err != nil { // #nosec G703 -- path is rooted in t.TempDir
		t.Fatalf("rewrite contract: %v", err)
	}

	if contract.ArtifactPolicy().ValidateSource().Available() {
		t.Error("the artifact policy stayed open after the contract's bytes changed")
	}
	if contract.PrivacyPolicy().ValidateSource().Available() {
		t.Error("the privacy policy stayed open after the contract's bytes changed")
	}
}
