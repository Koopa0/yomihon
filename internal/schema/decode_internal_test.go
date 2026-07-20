package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validContractV1 = `schema_version = "1"
aligned_with = "Note-Contract.md"
generated_at_must_match = true

[enums]
type = ["lesson", "study-path", "moc"]

[enums.status]
note = ["draft", "archived"]
lesson = ["draft", "archived"]

[fields]
required = ["title", "type"]
known = ["title", "type", "related"]
lesson_only = ["slug", "evolution_predecessor", "evolution_successors"]

[fields.status_group]
lesson = ["lesson"]

[rules]
slug_pattern = "^[a-z]+$"

[scan]
knowledge_dirs = ["Writing"]

[navigation]
path_types = ["study-path"]
map_types = ["moc"]

[artifacts]
non_instance_dirs = ["System/templates"]

[privacy]
never_egress_dirs = ["Private"]

[supersession]
predecessor_field = "evolution_predecessor"
successor_field = "evolution_successors"
general_link_field = "related"
archived_status = "archived"

[[lifecycle]]
status = "draft"
applies_to = ["lesson", "study-path"]
from = []
owner = ["koopa"]

[[lifecycle]]
status = "archived"
applies_to = ["*"]
from = ["*"]
owner = []
`

func TestCurrentContractVocabularyDecodes(t *testing.T) {
	t.Parallel()

	s, err := loadContractBytes(t, []byte(validContractV1))
	if err != nil {
		t.Fatalf("LoadFile(current schema v1 vocabulary) error = %v", err)
	}
	if got, want := s.metadata.alignedWith, "Note-Contract.md"; got != want {
		t.Errorf("decoded aligned_with = %q, want %q", got, want)
	}
	if !s.metadata.generatedAtMustMatch {
		t.Error("decoded generated_at_must_match = false, want true")
	}
	if s.metadata.supersession == nil {
		t.Fatal("decoded supersession = nil, want retained section")
	}
	if got, want := *s.metadata.supersession, (supersessionSection{
		PredecessorField: "evolution_predecessor",
		SuccessorField:   "evolution_successors",
		GeneralLinkField: "related",
		ArchivedStatus:   "archived",
	}); got != want {
		t.Errorf("decoded supersession = %+v, want %+v", got, want)
	}
}

func TestUnknownSupersessionKeysAreCoreErrors(t *testing.T) {
	t.Parallel()

	data := []byte(strings.Replace(
		validContractV1,
		"archived_status = \"archived\"\n",
		"archived_status = \"archived\"\nunknown_zeta = true\nunknown_alpha = true\n",
		1,
	))
	_, err := loadContractBytes(t, data)
	if err == nil {
		t.Fatal("LoadFile(unknown supersession keys) error = nil, want hard error")
	}
	const want = `unknown core keys: "supersession.unknown_alpha", "supersession.unknown_zeta"`
	if !strings.Contains(err.Error(), want) {
		t.Errorf("LoadFile(unknown supersession keys) error = %q, want substring %q", err, want)
	}
}

func TestDynamicContractMapKeysDecode(t *testing.T) {
	t.Parallel()

	data := strings.Replace(
		validContractV1,
		"note = [\"draft\", \"archived\"]",
		"note = [\"draft\", \"archived\"]\npublication = [\"published\", \"archived\"]",
		1,
	)
	data = strings.Replace(
		data,
		"lesson = [\"lesson\"]",
		"lesson = [\"lesson\"]\npublication = [\"moc\"]",
		1,
	)
	if _, err := loadContractBytes(t, []byte(data)); err != nil {
		t.Fatalf("LoadFile(dynamic map keys) error = %v", err)
	}
}

func TestContractVersion(t *testing.T) {
	t.Parallel()

	t.Run("unsupported", func(t *testing.T) {
		t.Parallel()

		data := []byte(strings.Replace(validContractV1, `schema_version = "1"`, `schema_version = "2"`, 1))
		if _, err := loadContractBytes(t, data); err == nil {
			t.Fatal(`LoadFile(schema_version = "2") error = nil, want unsupported-version error`)
		}
	})

	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		data := []byte(strings.Replace(validContractV1, "schema_version = \"1\"\n", "", 1))
		if _, err := loadContractBytes(t, data); err == nil {
			t.Fatal("LoadFile(missing schema_version) error = nil, want missing-version error")
		}
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		data := []byte(strings.Replace(validContractV1, `schema_version = "1"`, `schema_version = ""`, 1))
		if _, err := loadContractBytes(t, data); err == nil {
			t.Fatal(`LoadFile(schema_version = "") error = nil, want unsupported-version error`)
		}
	})
}

func TestCoreDecodeErrorsCloseContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "wrong core type",
			data: []byte(strings.Replace(
				validContractV1,
				`schema_version = "1"`,
				`schema_version = 1`,
				1,
			)),
		},
		{
			name: "malformed TOML",
			data: []byte(strings.Replace(
				validContractV1,
				`schema_version = "1"`,
				`schema_version = ["1"`,
				1,
			)),
		},
		{
			name: "wrong supersession type",
			data: []byte(strings.Replace(
				validContractV1,
				`archived_status = "archived"`,
				`archived_status = ["archived"]`,
				1,
			)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := loadContractBytes(t, tt.data); err == nil {
				t.Fatal("LoadFile(core decode error) error = nil, want hard error")
			}
		})
	}
}

func TestUnknownCoreKeys(t *testing.T) {
	t.Parallel()

	data := []byte(strings.Replace(
		validContractV1,
		"schema_version = \"1\"\n",
		"schema_version = \"1\"\nunknown_zeta = true\nunknown_alpha = true\n",
		1,
	))
	_, err := loadContractBytes(t, data)
	if err == nil {
		t.Fatal("LoadFile(unknown core keys) error = nil, want hard error")
	}
	const want = `unknown core keys: "unknown_alpha", "unknown_zeta"`
	if !strings.Contains(err.Error(), want) {
		t.Errorf("LoadFile(unknown core keys) error = %q, want substring %q", err, want)
	}
}

func TestUnknownNavigationKeysInvalidateOnlyNavigation(t *testing.T) {
	t.Parallel()

	data := []byte(strings.Replace(
		validContractV1,
		"map_types = [\"moc\"]\n",
		"map_types = [\"moc\"]\nmystery_zeta = true\nmystery_alpha = true\n",
		1,
	))
	s, err := loadContractBytes(t, data)
	if err != nil {
		t.Fatalf("LoadFile(unknown navigation keys) error = %v", err)
	}
	if s.NavigationRoles().Available() {
		t.Fatal("NavigationRoles().Available() = true, want false")
	}
	const want = `invalid navigation roles: unknown keys "navigation.mystery_alpha", "navigation.mystery_zeta"; Paths and Maps disabled`
	if got := s.NavigationRoles().Diagnostic(); got != want {
		t.Errorf("NavigationRoles().Diagnostic() = %q, want %q", got, want)
	}
	if !s.ArtifactPolicy().Available() {
		t.Errorf("ArtifactPolicy().Available() = false, diagnostic %q", s.ArtifactPolicy().Diagnostic())
	}
	if !s.PrivacyPolicy().Available() {
		t.Errorf("PrivacyPolicy().Available() = false, diagnostic %q", s.PrivacyPolicy().Diagnostic())
	}
	if _, ok := s.Stage("lesson", "draft"); !ok {
		t.Error(`Stage("lesson", "draft") = false, want true`)
	}
}

func TestNavigationTypeErrorInvalidatesOnlyNavigation(t *testing.T) {
	t.Parallel()

	data := []byte(strings.Replace(
		validContractV1,
		`path_types = ["study-path"]`,
		`path_types = "study-path"`,
		1,
	))
	s, err := loadContractBytes(t, data)
	if err != nil {
		t.Fatalf("LoadFile(navigation type error) error = %v", err)
	}
	if s.NavigationRoles().Available() {
		t.Fatal("NavigationRoles().Available() = true, want false")
	}
	const want = `invalid navigation roles: key "navigation.path_types" has incompatible TOML type; Paths and Maps disabled`
	if got := s.NavigationRoles().Diagnostic(); got != want {
		t.Errorf("NavigationRoles().Diagnostic() = %q, want %q", got, want)
	}
	if !s.ArtifactPolicy().Available() {
		t.Errorf("ArtifactPolicy().Available() = false, diagnostic %q", s.ArtifactPolicy().Diagnostic())
	}
	if !s.PrivacyPolicy().Available() {
		t.Errorf("PrivacyPolicy().Available() = false, diagnostic %q", s.PrivacyPolicy().Diagnostic())
	}
	if _, ok := s.Stage("lesson", "draft"); !ok {
		t.Error(`Stage("lesson", "draft") = false, want true`)
	}
}

func TestUnknownArtifactKeysInvalidateOnlyArtifacts(t *testing.T) {
	t.Parallel()

	data := []byte(strings.Replace(
		validContractV1,
		"non_instance_dirs = [\"System/templates\"]\n",
		"non_instance_dirs = [\"System/templates\"]\nmystery_zeta = true\nmystery_alpha = true\n",
		1,
	))
	s, err := loadContractBytes(t, data)
	if err != nil {
		t.Fatalf("LoadFile(unknown artifact keys) error = %v", err)
	}
	if s.ArtifactPolicy().Available() {
		t.Fatal("ArtifactPolicy().Available() = true, want false")
	}
	const want = `invalid artifact policy: unknown keys "artifacts.mystery_alpha", "artifacts.mystery_zeta"`
	if got := s.ArtifactPolicy().Diagnostic(); got != want {
		t.Errorf("ArtifactPolicy().Diagnostic() = %q, want %q", got, want)
	}
	if !s.NavigationRoles().Available() {
		t.Errorf("NavigationRoles().Available() = false, diagnostic %q", s.NavigationRoles().Diagnostic())
	}
	if !s.PrivacyPolicy().Available() {
		t.Errorf("PrivacyPolicy().Available() = false, diagnostic %q", s.PrivacyPolicy().Diagnostic())
	}
	if _, ok := s.Stage("lesson", "draft"); !ok {
		t.Error(`Stage("lesson", "draft") = false, want true`)
	}
}

func TestArtifactTypeErrorInvalidatesOnlyArtifacts(t *testing.T) {
	t.Parallel()

	data := []byte(strings.Replace(
		validContractV1,
		`non_instance_dirs = ["System/templates"]`,
		`non_instance_dirs = "System/templates"`,
		1,
	))
	s, err := loadContractBytes(t, data)
	if err != nil {
		t.Fatalf("LoadFile(artifact type error) error = %v", err)
	}
	if s.ArtifactPolicy().Available() {
		t.Fatal("ArtifactPolicy().Available() = true, want false")
	}
	const want = `invalid artifact policy: key "artifacts.non_instance_dirs" has incompatible TOML type`
	if got := s.ArtifactPolicy().Diagnostic(); got != want {
		t.Errorf("ArtifactPolicy().Diagnostic() = %q, want %q", got, want)
	}
	if !s.NavigationRoles().Available() {
		t.Errorf("NavigationRoles().Available() = false, diagnostic %q", s.NavigationRoles().Diagnostic())
	}
	if !s.PrivacyPolicy().Available() {
		t.Errorf("PrivacyPolicy().Available() = false, diagnostic %q", s.PrivacyPolicy().Diagnostic())
	}
	if _, ok := s.Stage("lesson", "draft"); !ok {
		t.Error(`Stage("lesson", "draft") = false, want true`)
	}
}

func TestUnknownPrivacyKeysInvalidateOnlyPrivacy(t *testing.T) {
	t.Parallel()

	data := []byte(strings.Replace(
		validContractV1,
		"never_egress_dirs = [\"Private\"]\n",
		"never_egress_dirs = [\"Private\"]\nmystery_zeta = true\nmystery_alpha = true\n",
		1,
	))
	s, err := loadContractBytes(t, data)
	if err != nil {
		t.Fatalf("LoadFile(unknown privacy keys) error = %v", err)
	}
	if s.PrivacyPolicy().Available() {
		t.Fatal("PrivacyPolicy().Available() = true, want false")
	}
	const want = `invalid privacy policy: unknown keys "privacy.mystery_alpha", "privacy.mystery_zeta"`
	if got := s.PrivacyPolicy().Diagnostic(); got != want {
		t.Errorf("PrivacyPolicy().Diagnostic() = %q, want %q", got, want)
	}
	if !s.NavigationRoles().Available() {
		t.Errorf("NavigationRoles().Available() = false, diagnostic %q", s.NavigationRoles().Diagnostic())
	}
	if !s.ArtifactPolicy().Available() {
		t.Errorf("ArtifactPolicy().Available() = false, diagnostic %q", s.ArtifactPolicy().Diagnostic())
	}
	if _, ok := s.Stage("lesson", "draft"); !ok {
		t.Error(`Stage("lesson", "draft") = false, want true`)
	}
}

func TestPrivacyTypeErrorInvalidatesOnlyPrivacy(t *testing.T) {
	t.Parallel()

	data := []byte(strings.Replace(
		validContractV1,
		`never_egress_dirs = ["Private"]`,
		`never_egress_dirs = "Private"`,
		1,
	))
	s, err := loadContractBytes(t, data)
	if err != nil {
		t.Fatalf("LoadFile(privacy type error) error = %v", err)
	}
	if s.PrivacyPolicy().Available() {
		t.Fatal("PrivacyPolicy().Available() = true, want false")
	}
	const want = `invalid privacy policy: key "privacy.never_egress_dirs" has incompatible TOML type`
	if got := s.PrivacyPolicy().Diagnostic(); got != want {
		t.Errorf("PrivacyPolicy().Diagnostic() = %q, want %q", got, want)
	}
	if !s.NavigationRoles().Available() {
		t.Errorf("NavigationRoles().Available() = false, diagnostic %q", s.NavigationRoles().Diagnostic())
	}
	if !s.ArtifactPolicy().Available() {
		t.Errorf("ArtifactPolicy().Available() = false, diagnostic %q", s.ArtifactPolicy().Diagnostic())
	}
	if _, ok := s.Stage("lesson", "draft"); !ok {
		t.Error(`Stage("lesson", "draft") = false, want true`)
	}
}

func loadContractBytes(t *testing.T, data []byte) (*Contract, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile(%q) = %v", path, err)
	}
	return LoadFile(path)
}

type decodeClassification struct {
	errClass                 string
	version                  string
	lifecycleEntries         int
	navigationAvailable      bool
	navigationDiagnostic     string
	artifactAvailable        bool
	artifactDiagnostic       string
	privacyAvailable         bool
	privacyDiagnostic        string
	alignedWith              string
	generatedAtMustMatch     bool
	supersessionPresent      bool
	supersessionPredecessor  string
	supersessionSuccessor    string
	supersessionGeneralLink  string
	supersessionArchiveState string
}

func classifyDecode(data []byte) decodeClassification {
	s, err := decodeContract(data, policySource{})
	if err != nil {
		return decodeClassification{errClass: classifyDecodeError(err)}
	}
	classification := decodeClassification{
		version:              s.Version(),
		lifecycleEntries:     s.StageCount(),
		navigationAvailable:  s.NavigationRoles().Available(),
		navigationDiagnostic: s.NavigationRoles().Diagnostic(),
		artifactAvailable:    s.ArtifactPolicy().Available(),
		artifactDiagnostic:   s.ArtifactPolicy().Diagnostic(),
		privacyAvailable:     s.PrivacyPolicy().Available(),
		privacyDiagnostic:    s.PrivacyPolicy().Diagnostic(),
		alignedWith:          s.metadata.alignedWith,
		generatedAtMustMatch: s.metadata.generatedAtMustMatch,
	}
	if s.metadata.supersession != nil {
		classification.supersessionPresent = true
		classification.supersessionPredecessor = s.metadata.supersession.PredecessorField
		classification.supersessionSuccessor = s.metadata.supersession.SuccessorField
		classification.supersessionGeneralLink = s.metadata.supersession.GeneralLinkField
		classification.supersessionArchiveState = s.metadata.supersession.ArchivedStatus
	}
	return classification
}

func classifyDecodeError(err error) string {
	switch message := err.Error(); {
	case message == `missing required key "schema_version"`:
		return "missing-version"
	case strings.HasPrefix(message, "unsupported schema_version "):
		return "unsupported-version"
	case strings.HasPrefix(message, "unknown core keys: "):
		return "unknown-core"
	case message == "no lifecycle stages":
		return "missing-lifecycle"
	case strings.HasPrefix(message, "toml:"):
		return "toml-decode"
	default:
		return message
	}
}

func FuzzDecodeContractDeterministic(f *testing.F) {
	seeds := [][]byte{
		nil,
		[]byte(validContractV1),
		[]byte(`schema_version = "2"`),
		[]byte("not = [valid"),
		[]byte(strings.Replace(validContractV1, "schema_version = \"1\"\n", "schema_version = \"1\"\nunknown = true\n", 1)),
		[]byte(strings.Replace(validContractV1, "map_types = [\"moc\"]\n", "map_types = [\"moc\"]\nunknown = true\n", 1)),
		[]byte(strings.Replace(validContractV1, "non_instance_dirs = [\"System/templates\"]\n", "non_instance_dirs = [\"System/templates\"]\nunknown = true\n", 1)),
		[]byte(strings.Replace(validContractV1, "never_egress_dirs = [\"Private\"]\n", "never_egress_dirs = [\"Private\"]\nunknown = true\n", 1)),
		[]byte(strings.Replace(validContractV1, `path_types = ["study-path"]`, `path_types = "study-path"`, 1)),
		[]byte(strings.Replace(validContractV1, `non_instance_dirs = ["System/templates"]`, `non_instance_dirs = "System/templates"`, 1)),
		[]byte(strings.Replace(validContractV1, `never_egress_dirs = ["Private"]`, `never_egress_dirs = "Private"`, 1)),
		[]byte("0B7b11BYCAAXX7=\"8\"\n[enums.status]\n72=\"\"\n0=\"\""),
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		first := classifyDecode(data)
		second := classifyDecode(data)
		if first != second {
			t.Errorf("decode classification changed between identical inputs:\nfirst:  %+v\nsecond: %+v", first, second)
		}
	})
}
