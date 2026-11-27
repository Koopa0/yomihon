package schema_test

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

func TestDefinitionIsDetached(t *testing.T) {
	t.Parallel()

	want := loadFixture(t).Definition()
	contract := loadFixture(t)
	mutated := contract.Definition()

	lesson := slices.IndexFunc(mutated.Types, func(def schema.TypeDefinition) bool { return def.Name == "lesson" })
	if lesson < 0 || len(mutated.Types[lesson].Fields) == 0 || len(mutated.Enums) == 0 {
		t.Fatal("fixture declares no lesson fields or enums; the detachment claim would be vacuous")
	}
	mutated.Types[0].Name = "changed"
	mutated.Types[lesson].Fields[0] = "changed"
	mutated.Types[lesson].SlugPattern = "changed"
	mutated.Enums[0].Values[0] = "changed"

	mutated.Fields.Common[0] = "changed"
	mutated.Fields.Required[0] = "changed"

	mutated.Rules.ConceptRequiresProvenance[0] = "changed"
	mutated.Rules.PlannedGapMarks[0] = "changed"
	mutated.Rules.PlannedInlineMarks[0] = "changed"

	mutated.Folders.Checked[0] = "changed"
	mutated.Folders.Templates[0] = "changed"
	mutated.Folders.DomainFrom[0] = "changed"
	mutated.Folders.SkippedFiles[0] = "changed"
	mutated.Folders.Journal = "changed"

	if diff := cmp.Diff(want, contract.Definition()); diff != "" {
		t.Errorf("Definition() changed after caller mutation (-want +got):\n%s", diff)
	}
}

func TestSkipsBasenameMatchesTheDeclaredFilenameOnly(t *testing.T) {
	t.Parallel()

	contract := loadFixture(t)
	for _, tt := range []struct {
		path string
		want bool
	}{
		{path: "README.md", want: true},
		{path: "Notes/README.md", want: true},
		{path: "Notes/Kept.md", want: false},
		{path: "readme.md", want: false},
		{path: "", want: false},
	} {
		if got := contract.SkipsBasename(tt.path); got != tt.want {
			t.Errorf("SkipsBasename(%q) = %v, want %v", tt.path, got, tt.want)
		}
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

// navigationTypes gives each navigation role one type, and the map role two.
const navigationTypes = `
[types.lesson]
role = "lesson"
fields = ["slug"]

[types.study-path]
role = "course"

[types.moc]
role = "map"

[types.topic-map]
role = "map"
`

func loadContractText(t *testing.T, folders string) *schema.Contract {
	t.Helper()
	return loadContractTextWithPrivacy(t, folders, "")
}

func loadContractTextWithPrivacy(t *testing.T, folders, privacy string) *schema.Contract {
	t.Helper()
	return loadText(t, contractText(folders, navigationTypes, privacy))
}

func loadText(t *testing.T, text string) *schema.Contract {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) = %v", path, err)
	}
	s, err := schema.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile(%q) = %v", path, err)
	}
	return s
}

// The two sections below are what every caller of loadSemanticRootContract
// writes; a caller that needs a different contract writes one.
const (
	semanticFolderLines    = "templates = [\"System/templates\"]\n"
	semanticPrivacySection = `
[privacy]
never_egress_dirs = ["Private"]
`
)

// loadSemanticRootContract writes the semantic contract into a vault of its own
// and loads it the way the served path does, through a reader pinned to that
// vault.
func loadSemanticRootContract(t *testing.T) (string, *schema.Contract) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll(%q) = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contractText(semanticFolderLines, navigationTypes, semanticPrivacySection)), 0o600); err != nil { // #nosec G703 -- path is rooted in t.TempDir
		t.Fatalf("WriteFile(%q) = %v", path, err)
	}
	return root, loadPinnedSemanticContract(t, root)
}

// contractText writes a contract whose [folders] table carries the knowledge
// layer and the given lines, followed by the given type tables and privacy
// section.
func contractText(folders, types, privacy string) string {
	return `[folders]
checked = ["Writing"]
` + folders + `
[fields]
common = ["type"]
` + types + privacy
}

func TestLoadFileDerivesValidCapabilities(t *testing.T) {
	t.Parallel()

	s := loadContractTextWithPrivacy(t, `templates = ["System/templates"]`, `
[privacy]
never_egress_dirs = ["Private"]
`)

	roles := s.NavigationRoles()
	if !roles.Available() {
		t.Fatalf("NavigationRoles().Available() = false, diagnostic %q", roles.Claim().Diagnostic())
	}
	if !roles.IsPathType("study-path") {
		t.Error("NavigationRoles().IsPathType(\"study-path\") = false, want true")
	}
	if !roles.IsMapType("moc") || !roles.IsMapType("topic-map") {
		t.Error("NavigationRoles().IsMapType() = false for a declared map type, want true")
	}
	if roles.IsPathType("lesson") || roles.IsMapType("lesson") {
		t.Error("NavigationRoles() classifies the lesson type as a path or a map, want neither role")
	}
	if !roles.IsLessonType("lesson") {
		t.Error("NavigationRoles().IsLessonType(\"lesson\") = false, want true")
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

// loadPinnedSemanticContract loads the contract of the vault at root through a
// reader opened on it, which stays open until the test ends.
func loadPinnedSemanticContract(t *testing.T, root string) *schema.Contract {
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
	return contract
}

func TestLoadReaderPinsContractAuthorityToTheSelectedVault(t *testing.T) {
	t.Parallel()
	root, _ := loadSemanticRootContract(t)
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
	if err := os.WriteFile(outside, []byte(contractText("", navigationTypes, "")), 0o600); err != nil {
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

// sourceCheck validates one copy of a source-bound capability and reports
// whether it is available and the claim it answers with.
type sourceCheck func() (available bool, claim schema.Claim)

func artifactCheck(c *schema.Contract) sourceCheck {
	policy := c.ArtifactPolicy()
	return func() (bool, schema.Claim) {
		validated := policy.ValidateSource()
		return validated.Available(), validated.Claim()
	}
}

func privacyCheck(c *schema.Contract) sourceCheck {
	policy := c.PrivacyPolicy()
	return func() (bool, schema.Claim) {
		validated := policy.ValidateSource()
		return validated.Available(), validated.Claim()
	}
}

// sourceBound is each capability that answers only while the contract's bytes
// are the ones it was derived from.
var sourceBound = []struct {
	name  string
	check func(*schema.Contract) sourceCheck
}{
	{"artifact policy", artifactCheck},
	{"privacy policy", privacyCheck},
}

// TestADriftedSourceClosesEveryCopyUntilRestart holds the one-way latch: once
// any copy sees the contract's bytes change, every copy of that capability
// stays closed, putting the bytes back reopens nothing, and only a fresh load
// reads the contract again. What a page says to a reader is chosen from the
// reason, so the reason is the changed-contract one.
func TestADriftedSourceClosesEveryCopyUntilRestart(t *testing.T) {
	t.Parallel()
	for _, tt := range sourceBound {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root, contract := loadSemanticRootContract(t)
			path := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
			original, err := os.ReadFile(path) // #nosec G304 -- path is rooted in t.TempDir
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}
			first, second := tt.check(contract), tt.check(contract)
			if err = os.WriteFile(path, append(original, '\n'), 0o600); err != nil { // #nosec G703 -- path is rooted in t.TempDir
				t.Fatalf("WriteFile(changed contract) error = %v", err)
			}
			if available, _ := first(); available {
				t.Fatal("the first copy stayed available after its source changed")
			}
			if err = os.WriteFile(path, original, 0o600); err != nil { // #nosec G703 -- path is rooted in t.TempDir
				t.Fatalf("WriteFile(restored contract) error = %v", err)
			}
			available, claim := second()
			if available || claim.Diagnostic() == "" || claim.Reason() != schema.ReasonContractChanged {
				t.Errorf("second copy after the bytes were put back = (%t, %q, %v), want a closed copy saying the contract changed",
					available, claim.Diagnostic(), claim.Reason())
			}
			if available, claim := tt.check(loadPinnedSemanticContract(t, root))(); !available {
				t.Errorf("a fresh load stayed unavailable: %s", claim.Diagnostic())
			}
		})
	}
}

// TestAnUnreadableSourceRefusesOneRequestWithoutLatching separates a contract
// that cannot be read from one that changed. A save-by-rename leaves the name
// absent for a few microseconds, and reading that instant as drift would shut
// the capability for the life of the process over bytes nobody touched. The
// request that meets the gap is refused, and the next one asks again.
func TestAnUnreadableSourceRefusesOneRequestWithoutLatching(t *testing.T) {
	t.Parallel()
	for _, tt := range sourceBound {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root, contract := loadSemanticRootContract(t)
			path := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
			check := tt.check(contract)
			if available, claim := check(); !available {
				t.Fatalf("unavailable before the source moved: %s", claim.Diagnostic())
			}
			away := path + ".away"
			if err := os.Rename(path, away); err != nil {
				t.Fatalf("Rename(%q, %q) = %v", path, away, err)
			}
			available, claim := check()
			if available || claim.Trustworthy() || claim.Diagnostic() == "" || claim.Reason() != schema.ReasonContractUnreadable {
				t.Errorf("while the source cannot be read = (%t, trustworthy %t, %q, %v), want a refusal that says the file could not be read",
					available, claim.Trustworthy(), claim.Diagnostic(), claim.Reason())
			}
			if err := os.Rename(away, path); err != nil {
				t.Fatalf("Rename(%q, %q) = %v", away, path, err)
			}
			if available, claim := check(); !available {
				t.Errorf("stayed shut after the identical file came back: %s", claim.Diagnostic())
			}
		})
	}
}

func TestArtifactPolicyCaptureIsAnImmutableRequestSnapshot(t *testing.T) {
	t.Parallel()

	root, contract := loadSemanticRootContract(t)
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

func TestASourceDriftLatchIsConcurrentSafe(t *testing.T) {
	t.Parallel()

	root, contract := loadSemanticRootContract(t)
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

// TestLoadFileMissingOptionalSections pins what a contract that writes no
// template folders, no role and no privacy section declares. The first two are
// empty sets read cleanly; the privacy section is the one a governed vault owes,
// so its absence is the fail-closed sentence.
func TestLoadFileMissingOptionalSections(t *testing.T) {
	t.Parallel()

	s := loadText(t, contractText("", "", ""))
	roles := s.NavigationRoles()
	if !roles.Available() || !roles.Trustworthy() {
		t.Fatalf("NavigationRoles() of a contract with no roles: available %t, trustworthy %t, want both", roles.Available(), roles.Trustworthy())
	}
	if got := roles.Claim().Diagnostic(); got != "" {
		t.Errorf("NavigationRoles().Diagnostic() = %q, want empty", got)
	}
	if roles.IsPathType("study-path") || roles.IsMapType("moc") || roles.AnswerType() != "" {
		t.Error("NavigationRoles() of a contract with no roles classifies a type")
	}
	policy := s.ArtifactPolicy()
	if !policy.Available() || !policy.Trustworthy() {
		t.Fatalf("ArtifactPolicy() of a contract with no template folders: available %t, trustworthy %t, want both", policy.Available(), policy.Trustworthy())
	}
	if got := policy.Diagnostic(); got != "" {
		t.Errorf("ArtifactPolicy().Diagnostic() = %q, want empty", got)
	}
	if policy.IsNonInstance("System/templates/card.md") {
		t.Error("ArtifactPolicy().IsNonInstance() = true with no template folders, want false")
	}

	privacy := s.PrivacyPolicy()
	if privacy.Available() {
		t.Fatal("PrivacyPolicy().Available() = true, want false")
	}
	if !privacy.Undeclared() || privacy.Diagnostic() == "" {
		t.Errorf("PrivacyPolicy() = undeclared %t, diagnostic %q; want the omission said out loud", privacy.Undeclared(), privacy.Diagnostic())
	}
	if privacy.EgressAllowed("Public/note.md") {
		t.Error("unavailable PrivacyPolicy().EgressAllowed() = true, want fail-closed false")
	}
	if privacy.Trustworthy() {
		t.Error("privacy policy of a contract that omitted the section is trustworthy; the operator meant to govern something yomihon cannot read")
	}
}

func TestPrivacyPolicyRejectsOmittedRequiredKey(t *testing.T) {
	t.Parallel()

	s := loadContractTextWithPrivacy(t, "", "\n[privacy]\n")
	policy := s.PrivacyPolicy()
	if policy.Available() {
		t.Fatal("PrivacyPolicy().Available() = true, want false")
	}
	if got := policy.Diagnostic(); !strings.Contains(got, `"never_egress_dirs"`) {
		t.Errorf("PrivacyPolicy().Diagnostic() = %q, want the missing key named", got)
	}
	if policy.EgressAllowed("Public/note.md") {
		t.Error("invalid PrivacyPolicy().EgressAllowed() = true, want fail-closed false")
	}
}

func TestPrivacyPolicyNormalizesAndMatchesComponentPrefixes(t *testing.T) {
	t.Parallel()

	s := loadContractTextWithPrivacy(t, "", `
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

	const value = `Private/../Notes`
	s := loadContractTextWithPrivacy(t, "", fmt.Sprintf("\n[privacy]\nnever_egress_dirs = [%s]\n", strconv.Quote(value)))
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
}

// TestNavigationRolesRejectRoleAnomalies holds where a role the reader cannot
// honour lands: the role sets close with a sentence naming the types, and the
// contract still loads, so every other capability stays open.
func TestNavigationRolesRejectRoleAnomalies(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		types string
		named []string
	}{
		{
			name:  "unknown role",
			types: navigationTypes + "\n[types.extra]\nrole = \"chapter\"\n",
			named: []string{`"extra"`, `"chapter"`},
		},
		{
			name:  "two lesson types",
			types: navigationTypes + "\n[types.module]\nrole = \"lesson\"\nfields = [\"slug\"]\n",
			named: []string{`"lesson"`, `"module"`},
		},
		{
			name:  "two thought types",
			types: navigationTypes + "\n[types.thought]\nrole = \"thought\"\nfields = [\"based_on\"]\n\n[types.reflection]\nrole = \"thought\"\nfields = [\"based_on\"]\n",
			named: []string{`"thought"`, `"reflection"`},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := loadText(t, contractText(`templates = ["System/templates"]`, tt.types, ""))
			roles := s.NavigationRoles()
			if roles.Available() || roles.Trustworthy() {
				t.Fatalf("NavigationRoles(): available %t, trustworthy %t, want neither", roles.Available(), roles.Trustworthy())
			}
			for _, name := range tt.named {
				if got := roles.Claim().Diagnostic(); !strings.Contains(got, name) {
					t.Errorf("NavigationRoles().Diagnostic() = %q, want %s named", got, name)
				}
			}
			if roles.IsPathType("study-path") || roles.IsMapType("moc") || roles.IsLessonType("lesson") || roles.AnswerType() != "" {
				t.Error("NavigationRoles() kept part of a role set it rejected, want the entire set unavailable")
			}
			if !s.ArtifactPolicy().Available() {
				t.Errorf("ArtifactPolicy().Available() = false after rejected roles, diagnostic %q", s.ArtifactPolicy().Diagnostic())
			}
		})
	}
}

func TestJournalDirIsUnclaimedWhenUndeclared(t *testing.T) {
	t.Parallel()

	s := loadContractText(t, `templates = ["System/templates"]`)
	journal := s.JournalDir()
	if journal.Claim().Claimed() {
		t.Fatal("JournalDir() claims a declaration the contract never wrote")
	}
	if journal.Available() {
		t.Fatal("JournalDir().Available() = true, want false")
	}
	if !journal.Claim().Trustworthy() {
		t.Error("an undeclared journal is untrustworthy; empty is the true answer")
	}
	if journal.Contains("Diary/today.md") {
		t.Error("JournalDir().Contains() = true with no journal, want empty projection")
	}
	if !s.NavigationRoles().Available() {
		t.Errorf("NavigationRoles().Available() = false after optional journal omitted, diagnostic %q", s.NavigationRoles().Claim().Diagnostic())
	}
}

func TestJournalDirProjectsTheDeclaredDirectory(t *testing.T) {
	t.Parallel()

	s := loadContractText(t, `journal = "Diary"`)
	journal := s.JournalDir()
	if !journal.Available() {
		t.Fatalf("JournalDir().Available() = false, diagnostic %q", journal.Claim().Diagnostic())
	}
	if !journal.Contains("Diary/today.md") {
		t.Error("JournalDir().Contains(Diary/today.md) = false, want true")
	}
	if journal.Contains("Notes/today.md") {
		t.Error("JournalDir().Contains(Notes/today.md) = true, want false")
	}
}

func TestJournalDirRejectsInvalidDirectory(t *testing.T) {
	t.Parallel()

	s := loadContractText(t, `journal = "../Diary"`)
	journal := s.JournalDir()
	if journal.Available() {
		t.Fatal("JournalDir().Available() = true, want false")
	}
	if got := journal.Claim().Diagnostic(); !strings.Contains(got, `../Diary`) {
		t.Errorf("JournalDir().Diagnostic() = %q, want offending value named", got)
	}
	if journal.Contains("Diary/today.md") {
		t.Error("a rejected journal directory still contains a path")
	}
	if !s.NavigationRoles().Available() {
		t.Errorf("NavigationRoles().Available() = false after invalid journal, diagnostic %q", s.NavigationRoles().Claim().Diagnostic())
	}
}

func TestArtifactPolicyRejectsEmptyDirectory(t *testing.T) {
	t.Parallel()

	s := loadContractText(t, `templates = [""]`)
	policy := s.ArtifactPolicy()
	if policy.Available() {
		t.Fatal("ArtifactPolicy().Available() = true, want false")
	}
	if got := policy.Diagnostic(); !strings.Contains(got, `""`) {
		t.Errorf("ArtifactPolicy().Diagnostic() = %q, want offending empty value named", got)
	}
	if !s.NavigationRoles().Available() {
		t.Errorf("NavigationRoles().Available() = false after invalid artifact policy, diagnostic %q", s.NavigationRoles().Claim().Diagnostic())
	}
}

func TestArtifactPolicyNormalizesAndMatchesComponentPrefixes(t *testing.T) {
	t.Parallel()

	s := loadContractText(t, `templates = ["System//./templates", "Cafe\u0301/models"]`)
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
		{name: "NFD query", path: "Cafe\u0301/models/card.md", want: true},
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
// template is the protected template. Both policies answer the question
// through the same function, so the two cannot drift.
func TestDirectoryPoliciesShareCaseFoldedIdentity(t *testing.T) {
	t.Parallel()

	s := loadContractTextWithPrivacy(t, `templates = ["System/templates"]`, `
[privacy]
never_egress_dirs = ["Private", "Straße"]
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
		// The fold is Unicode simple case folding, per rune: the capital ẞ is
		// the same letter as ß, while the two-letter "ss" spelling is a
		// different string — a full case fold would accept it, and the shared
		// identity deliberately does not.
		{path: "Straße/x.md", want: false},
		{path: "STRAẞE/x.md", want: false},
		{path: "Strasse/x.md", want: true},
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

// TestKnowledgeScopeFoldsOnlyTheFirstSegment pins how narrow knowledge-layer
// membership is and where it deliberately stops being narrow. The first path
// segment alone is consulted — never a deeper one, never a prefix of one — and
// it is compared with the fold every vault directory scope shares: a
// case-insensitive filesystem opens one folder under every case spelling of its
// name, so a membership that turned on the spelling would govern a note or
// leave it ungoverned by coincidence.
//
// Composition stays a separate dimension: the loader composes every declared
// folder and the scan reports composed paths, so the fold is over case alone.
func TestKnowledgeScopeFoldsOnlyTheFirstSegment(t *testing.T) {
	t.Parallel()

	const declared = `checked = ["Writing"]`
	const widened = `checked = ["Writing", "だ体"]`
	text := strings.Replace(contractText("", "", ""), declared, widened, 1)
	if !strings.Contains(text, widened) {
		t.Fatalf("fixture drift: the base contract no longer declares %s", declared)
	}
	scope := loadText(t, text).KnowledgeScope()
	if !scope.Available() {
		t.Fatal("KnowledgeScope().Available() = false, want a declared scope")
	}
	if diff := cmp.Diff([]string{"Writing", "だ体"}, scope.Declared()); diff != "" {
		t.Errorf("KnowledgeScope().Declared() mismatch (-want +got):\n%s", diff)
	}

	tests := []struct {
		path string
		want bool
	}{
		{path: "Writing/lessons/L05.md", want: true},
		{path: "Writing", want: true},
		// One folder, whichever case its owner typed into the contract.
		{path: "writing/lessons/L05.md", want: true},
		{path: "WRITING/lessons/L05.md", want: true},
		// The whole first segment, never a prefix of it.
		{path: "Writings/L05.md", want: false},
		// Deeper segments are not consulted.
		{path: "Other/Writing/L05.md", want: false},
		{path: "だ体/note.md", want: true},
		// The decomposed spelling of the same name stays a different string:
		// the fold is over case, and composition is the scan's business, which
		// reports one composed spelling for every path it observed.
		{path: "た\u3099体/note.md", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			if got := scope.Includes(tt.path); got != tt.want {
				t.Errorf("KnowledgeScope().Includes(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// TestKnowledgeScopeIncludesEverythingWhenUndeclared pins the empty-set
// polarity: omitting folders.checked and writing [] are the same
// declaration — no layer — and no layer excludes nothing.
func TestKnowledgeScopeIncludesEverythingWhenUndeclared(t *testing.T) {
	t.Parallel()

	const declared = `checked = ["Writing"]`
	tests := []struct {
		name string
		from string
		to   string
	}{
		{name: "omitted", from: declared + "\n", to: ""},
		{name: "empty list", from: declared, to: `checked = []`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			text := strings.Replace(contractText("", "", ""), tt.from, tt.to, 1)
			if text == contractText("", "", "") && tt.to != declared {
				t.Fatalf("fixture drift: the base contract no longer declares %s", declared)
			}
			scope := loadText(t, text).KnowledgeScope()
			if scope.Available() {
				t.Fatal("KnowledgeScope().Available() = true, want no declared layer")
			}
			if got := scope.Declared(); len(got) != 0 {
				t.Errorf("KnowledgeScope().Declared() = %v, want empty", got)
			}
			for _, p := range []string{"Writing/lessons/L05.md", "System/notes/Idea.md", "Away/Loose.md"} {
				if !scope.Includes(p) {
					t.Errorf("KnowledgeScope().Includes(%q) = false, want true when no layer is declared", p)
				}
			}
		})
	}
}

// TestPrivacyResolvesDottedPathsWhileArtifactMembershipDoesNot is a
// characterization of a live asymmetry, not an endorsement of it. The privacy
// policy cleans a path before matching, so a dotted spelling is judged by
// where it lands, and an escaping spelling is refused outright. Artifact
// membership normalizes only the Unicode form and compares the spelling it
// was given, so a dotted route into a declared artifact directory is not
// classified as non-instance. Whether both ends should share one fail-closed
// identity is an open design question; until it is ruled, this keeps the
// asymmetry from drifting unnoticed.
func TestPrivacyResolvesDottedPathsWhileArtifactMembershipDoesNot(t *testing.T) {
	t.Parallel()

	s := loadContractTextWithPrivacy(t, `templates = ["System/templates", "だ体"]`, `
[privacy]
never_egress_dirs = ["Private", "だ体"]
`)
	artifact := s.ArtifactPolicy()
	if !artifact.Available() {
		t.Fatalf("ArtifactPolicy().Available() = false, diagnostic %q", artifact.Diagnostic())
	}
	privacy := s.PrivacyPolicy()
	if !privacy.Available() {
		t.Fatalf("PrivacyPolicy().Available() = false, diagnostic %q", privacy.Diagnostic())
	}

	privacyTests := []struct {
		path string
		want bool
	}{
		// Cleaned first: the dotted spelling lands inside the protected
		// directory and is refused.
		{path: "Writing/../Private/x.md", want: false},
		// Cleaned to a public spelling: allowed.
		{path: "Writing/../Writing/x.md", want: true},
		// An escaping spelling is refused outright, wherever it points.
		{path: "../Private/x.md", want: false},
		// A trailing slash cleans away.
		{path: "Private/", want: false},
		// The decomposed spelling is normalized before matching.
		{path: "た\u3099体/x.md", want: false},
	}
	for _, tt := range privacyTests {
		t.Run("privacy/"+tt.path, func(t *testing.T) {
			t.Parallel()
			if got := privacy.EgressAllowed(tt.path); got != tt.want {
				t.Errorf("PrivacyPolicy().EgressAllowed(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}

	artifactTests := []struct {
		path string
		want bool
	}{
		{path: "System/templates/card.md", want: true},
		// The asymmetric cell: the dotted spelling resolves under a declared
		// directory, but membership never resolves it, so the classification
		// fails open toward instance.
		{path: "Writing/../System/templates/card.md", want: false},
		// A trailing slash still matches: the empty last component is beyond
		// the compared prefix.
		{path: "System/templates/", want: true},
		// The decomposed spelling is normalized before matching, same as the
		// privacy side.
		{path: "た\u3099体/x.md", want: true},
	}
	for _, tt := range artifactTests {
		t.Run("artifact/"+tt.path, func(t *testing.T) {
			t.Parallel()
			if got := artifact.IsNonInstance(tt.path); got != tt.want {
				t.Errorf("ArtifactPolicy().IsNonInstance(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestCapabilitiesAllowEmptyLists(t *testing.T) {
	t.Parallel()

	s := loadText(t, contractText(`templates = []`, "\n[types.note]\n", ""))
	roles := s.NavigationRoles()
	if !roles.Available() {
		t.Fatalf("NavigationRoles().Available() = false, diagnostic %q", roles.Claim().Diagnostic())
	}
	if got := roles.Claim().Diagnostic(); got != "" {
		t.Errorf("NavigationRoles().Diagnostic() = %q with no role given, want empty", got)
	}
	if roles.IsPathType("note") || roles.IsMapType("note") || roles.IsLessonType("note") {
		t.Error("NavigationRoles() classifies a type no role was given to, want neither role")
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

// TestTouchingTheContractLeavesThePolicyAvailable separates two questions the
// pinned identity check had collapsed into one.
//
// The pinned entry records which file the contract was at startup, and that
// record includes the modification time — so a git checkout, a pull, a restored
// backup or an editor that saves by rename moves it without changing a byte.
// The vault this serves commonly lives under version control and sync tools,
// so this is an ordinary event, and treating it as a contract change closed
// the write face until restart while reporting a change that had not happened.
func TestTouchingTheContractLeavesThePolicyAvailable(t *testing.T) {
	t.Parallel()
	root, contract := loadSemanticRootContract(t)
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

// TestANilContractAnswersAsAnUngovernedVault states the type's nil policy as
// one table, over every exported method rather than the ones somebody thought
// of. Both reachable ways a vault ends up ungoverned — no contract file, and a
// contract file that could not be read — hand every consumer a nil *Contract,
// so a method that panicked on one would be a fault reachable from a folder
// with nothing wrong with it.
//
// The method list is read out of the package source, so a method added without
// a row here fails at this test rather than at whichever consumer meets the
// nil first.
func TestANilContractAnswersAsAnUngovernedVault(t *testing.T) {
	t.Parallel()

	var c *schema.Contract
	// Each row answers with the complaint to make, or empty when the method
	// answered as a vault no contract governs.
	answers := map[string]func() string{
		"Definition": func() string {
			if diff := cmp.Diff(schema.Definition{}, c.Definition()); diff != "" {
				return "Definition() mismatch (-want +got):\n" + diff
			}
			return ""
		},
		"PlannedMarks": func() string {
			heading, inline := c.PlannedMarks()
			if heading != nil || inline != nil {
				return fmt.Sprintf("PlannedMarks() = (%v, %v), want nil lists: no contract declared no marks", heading, inline)
			}
			return ""
		},
		"ScanBoundary": func() string {
			if c.ScanBoundary().OutsideScan("courses/C1/lab/main.go") {
				return "ScanBoundary() excludes courses/C1/lab/main.go, want a vault no contract governs to exclude nothing"
			}
			return ""
		},
		"EnumValues": func() string {
			for _, field := range []string{"type", "domain", "level", "unknown"} {
				if got := c.EnumValues(field); got != nil {
					return fmt.Sprintf("EnumValues(%q) = %v, want nil", field, got)
				}
			}
			return ""
		},
		"FieldStanding": func() string {
			for _, key := range []string{"type", "draft", "tags", "slug", "title"} {
				if got := c.FieldStanding("lesson", key); got != schema.FieldUnknown {
					return fmt.Sprintf("FieldStanding(lesson, %q) = %v, want FieldUnknown: no contract lists a field here", key, got)
				}
			}
			return ""
		},
		"RequiredFields": func() string {
			if got := c.RequiredFields("lesson"); got != nil {
				return fmt.Sprintf("RequiredFields(lesson) = %v, want nil", got)
			}
			return ""
		},
		"SlugPattern": func() string {
			if pattern, ok := c.SlugPattern("lesson"); ok || pattern != nil {
				return fmt.Sprintf("SlugPattern(lesson) = (%v, %t), want (nil, false)", pattern, ok)
			}
			return ""
		},
		"Domains": func() string {
			domains := c.Domains()
			if got := domains.Of("Concepts/golang/a.md"); got != "" || domains.Roots() != nil {
				return fmt.Sprintf("Domains() = roots %v, Of() = %q, want no root and no domain", domains.Roots(), got)
			}
			return ""
		},
		"ConceptType": func() string {
			if _, declared := c.ConceptType(); declared {
				return "ConceptType() reports a declared concept corpus"
			}
			return ""
		},
		"LessonType": func() string {
			if name, declared := c.LessonType(); declared || name == "" {
				return fmt.Sprintf("LessonType() = (%q, %t), want the name it looks for and false", name, declared)
			}
			return ""
		},
		"RequiresFrontmatter": func() string {
			if c.RequiresFrontmatter() {
				return "RequiresFrontmatter() faults a note that carries none"
			}
			return ""
		},
		"SkipBasenames": func() string {
			if got := c.SkipBasenames(); got != nil {
				return fmt.Sprintf("SkipBasenames() = %v, want nil", got)
			}
			return ""
		},
		"SkipsBasename": func() string {
			if c.SkipsBasename("README.md") {
				return "SkipsBasename() skips a filename a vault nothing governs never declared"
			}
			return ""
		},
		"DeclaresType": func() string {
			if c.DeclaresType("lesson") {
				return "DeclaresType() claims a type vocabulary"
			}
			return ""
		},
		"Capabilities": func() string {
			caps := c.Capabilities(schema.Ungoverned())
			if !caps.Navigation.Available() || !caps.Artifacts.Available() {
				return "Capabilities() withholds the built-in roles or the artifact policy from a vault nothing governs"
			}
			if caps.Knowledge.Available() || caps.Journal.Available() {
				return "Capabilities() claims a declared knowledge layer or journal for a vault nothing governs"
			}
			for name, claim := range map[string]schema.Claim{
				"Navigation": caps.Navigation.Claim(), "Artifacts": caps.Artifacts.Claim(),
				"Journal": caps.Journal.Claim(),
			} {
				if claim.Diagnostic() != "" || !claim.Trustworthy() {
					return fmt.Sprintf("Capabilities().%s reports a withheld declaration where none was ever made", name)
				}
			}
			return ""
		},
		"Governance": func() string {
			if c.Governance().Governed() {
				return "Governance() claims authority over the vault"
			}
			return ""
		},
		"NavigationRoles": func() string {
			roles := c.NavigationRoles()
			for _, word := range []string{"course", "study-path"} {
				if !roles.IsPathType(word) {
					return fmt.Sprintf("NavigationRoles().IsPathType(%q) = false, want the built-in course type", word)
				}
			}
			for _, word := range []string{"index", "moc", "map"} {
				if !roles.IsMapType(word) {
					return fmt.Sprintf("NavigationRoles().IsMapType(%q) = false, want the built-in index type", word)
				}
			}
			for _, word := range []string{"unit", "lesson"} {
				if !roles.IsLessonType(word) {
					return fmt.Sprintf("NavigationRoles().IsLessonType(%q) = false, want the built-in unit type", word)
				}
			}
			if roles.IsPathType("note") || roles.IsMapType("note") || roles.IsLessonType("note") {
				return "NavigationRoles() gives an ordinary note type a role"
			}
			if got := roles.AnswerType(); got != "thought" {
				return fmt.Sprintf("NavigationRoles().AnswerType() = %q, want the built-in %q", got, "thought")
			}
			return ""
		},
		"KnowledgeScope": func() string {
			if c.KnowledgeScope().Available() {
				return "KnowledgeScope() claims a declared knowledge layer"
			}
			return ""
		},
		"ArtifactPolicy": func() string {
			policy := c.ArtifactPolicy()
			if !policy.Available() || policy.IsNonInstance("System/templates/x.md") {
				return "ArtifactPolicy() must be available and hold no note apart from the instances"
			}
			return ""
		},
		"PrivacyPolicy": func() string {
			policy := c.PrivacyPolicy()
			if policy.Available() || policy.EgressAllowed("Notes/a.md") {
				return "PrivacyPolicy() permits egress with no declaration behind it"
			}
			return ""
		},
		"JournalDir": func() string {
			journal := c.JournalDir()
			if journal.Available() || journal.Contains("Diary/today.md") {
				return "JournalDir() claims a declared journal directory"
			}
			return ""
		},
		"ArticleLanguage": func() string {
			tag, err := c.ArticleLanguage().Resolve(map[string]any{"lang": "ja"})
			if tag != "" || err != nil {
				return fmt.Sprintf("ArticleLanguage().Resolve() = (%q, %v), want no tag and no fault: the field has no authority here", tag, err)
			}
			return ""
		},
		"AuthoredDate": func() string {
			dated := c.AuthoredDate()
			day, err := dated.Resolve(map[string]any{"created": "2026-08-31"})
			if dated.Declared() || day != "" || err != nil {
				return fmt.Sprintf("AuthoredDate() declared %t and resolves (%q, %v), want no field and no day: no contract dates a note here",
					dated.Declared(), day, err)
			}
			return ""
		},
	}

	byPointer, byValue := exportedContractMethods(t)
	for _, name := range byValue {
		t.Errorf("Contract.%s takes the contract by value, so calling it on the nil that every ungoverned vault "+
			"hands out dereferences before the body runs; declare it on *Contract and give it a row here", name)
	}
	for _, name := range byPointer {
		check, covered := answers[name]
		if !covered {
			t.Errorf("(*Contract).%s has no row here: state what it answers for a vault no contract governs", name)
			continue
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if complaint := check(); complaint != "" {
				t.Error(complaint)
			}
		})
	}
	for name := range answers {
		if !slices.Contains(byPointer, name) {
			t.Errorf("this table states an answer for (*Contract).%s, which the package does not declare", name)
		}
	}
}

// exportedContractMethods reads the names of every exported method on Contract
// out of the package source, so no hand-kept list can fall behind the type. The
// two receiver forms come back apart because they differ in exactly what this
// test is about: a pointer receiver reaches its body holding a nil contract and
// can answer for it, and a value receiver never reaches its body at all.
func exportedContractMethods(t *testing.T) (byPointer, byValue []string) {
	t.Helper()
	fset := token.NewFileSet()
	dir, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("list the package source: %v", err)
	}
	for _, entry := range dir {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		parsed, parseErr := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", name, parseErr)
		}
		for _, decl := range parsed.Decls {
			fn, isFunc := decl.(*ast.FuncDecl)
			if !isFunc || fn.Recv == nil || len(fn.Recv.List) != 1 || !fn.Name.IsExported() {
				continue
			}
			receiver := fn.Recv.List[0].Type
			pointer := false
			if star, isStar := receiver.(*ast.StarExpr); isStar {
				receiver, pointer = star.X, true
			}
			ident, isIdent := receiver.(*ast.Ident)
			if !isIdent || ident.Name != "Contract" {
				continue
			}
			if pointer {
				byPointer = append(byPointer, fn.Name.Name)
			} else {
				byValue = append(byValue, fn.Name.Name)
			}
		}
	}
	if len(byPointer) == 0 {
		t.Fatal("no exported *Contract methods were found; the scan would prove nothing")
	}
	slices.Sort(byPointer)
	slices.Sort(byValue)
	return byPointer, byValue
}
