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
	lifecycleStatuses        string
	navigationAvailable      bool
	navigationDiagnostic     string
	artifactAvailable        bool
	artifactDiagnostic       string
	privacyAvailable         bool
	privacyDiagnostic        string
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
		lifecycleStatuses:    stageStatuses(s),
		navigationAvailable:  s.NavigationRoles().Available(),
		navigationDiagnostic: s.NavigationRoles().Diagnostic(),
		artifactAvailable:    s.ArtifactPolicy().Available(),
		artifactDiagnostic:   s.ArtifactPolicy().Diagnostic(),
		privacyAvailable:     s.PrivacyPolicy().Available(),
		privacyDiagnostic:    s.PrivacyPolicy().Diagnostic(),
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

// stageStatuses names the lifecycle rows a decode kept, in the order it kept
// them. The row count alone let two decodes that kept different rows of the
// same length compare equal, which is the shape of the defect this comparison
// exists to catch.
func stageStatuses(contract *Contract) string {
	names := make([]string, len(contract.stages))
	for i, stage := range contract.stages {
		names[i] = stage.Status
	}
	return strings.Join(names, ",")
}

func classifyDecodeError(err error) string {
	switch message := err.Error(); {
	case message == `missing required key "schema_version"`:
		return "missing-version"
	case strings.HasPrefix(message, "unsupported schema_version "):
		return "unsupported-version"
	case strings.HasPrefix(message, "unknown core keys: "):
		return "unknown-core"
	case strings.HasPrefix(message, "contract keys differ only by letter case: "):
		return "folded-keys"
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
		[]byte(foldedKeyContract("top-level")),
		[]byte(foldedKeyContract("nested-table")),
		[]byte(foldedKeyContract("array-of-tables")),
		[]byte(foldedKeyContract("key-in-table")),
		[]byte(foldedKeyContract("map-valued-section")),
		[]byte(foldedKeyContract("quoted-key")),
		[]byte(foldedKeyContract("inline-table-array")),
		[]byte(foldedKeyContract("row-in-table-array")),
		[]byte(foldedKeyContract("two-pairs")),
		[]byte(foldedKeyContract("lowercases-together-only")),
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

// foldedKeyContract writes a contract that spells one key twice, differing
// only in the letters' case, so the decoder's second-chance folded match has
// two candidates for one field.
// foldedKeyContract writes a contract that spells one key twice, differing
// only in the letters' case, so the decoder's second-chance folded match has
// two candidates for one field.
//
// The two non-ASCII shapes are built from their code points rather than typed,
// because U+017F reads as an f and U+0130 as an I, and a reviewer has to be
// able to see which letter a case is about.
func foldedKeyContract(shape string) string {
	const (
		longS   = "\u017f" // ſ, which the decoder folds to s
		dottedI = "\u0130" // İ, which the decoder does not fold to i
	)
	switch shape {
	case "top-level":
		return strings.Replace(validContractV1,
			"aligned_with = \"Note-Contract.md\"\n",
			"aligned_with = \"Note-Contract.md\"\nAligned_With = \"Other.md\"\n", 1)
	case "nested-table":
		return validContractV1 + "\n[Navigation]\npath_types = [\"moc\"]\n"
	case "array-of-tables":
		return validContractV1 + "\n[[Lifecycle]]\nstatus = \"ready\"\napplies_to = [\"*\"]\nfrom = [\"draft\"]\nowner = []\n"
	case "key-in-table":
		return strings.Replace(validContractV1,
			"required = [\"title\", \"type\"]\n",
			"required = [\"title\", \"type\"]\nRequired = [\"nothing\"]\n", 1)
	case "map-valued-section":
		return strings.Replace(validContractV1,
			"note = [\"draft\", \"archived\"]\n",
			"note = [\"draft\", \"archived\"]\nNote = [\"draft\"]\n", 1)
	case "quoted-key":
		// A key outside the ASCII set has to be quoted to be a key at all, and
		// this pair separates the decoder's own folding from a lowercasing
		// comparison: the two fold together and do not lowercase together.
		return strings.Replace(validContractV1,
			"slug_pattern = \"^[a-z]+$\"\n",
			"slug_pattern = \"^[a-z]+$\"\n\""+longS+"lug_pattern\" = \"^[0-9]+$\"\n", 1)
	case "two-pairs":
		// Two folded pairs in two different tables, so the order they are
		// reported in is observable.
		withFields := strings.Replace(validContractV1,
			"required = [\"title\", \"type\"]\n",
			"required = [\"title\", \"type\"]\nRequired = [\"nothing\"]\n", 1)
		return strings.Replace(withFields,
			"note = [\"draft\", \"archived\"]\n",
			"note = [\"draft\", \"archived\"]\nNote = [\"draft\"]\n", 1)
	case "row-in-table-array":
		// Both spellings inside one row of an array of tables: the ambiguity is
		// in the row, not among the row headers.
		return strings.Replace(validContractV1,
			"[[lifecycle]]\nstatus = \"archived\"\n",
			"[[lifecycle]]\nstatus = \"archived\"\nStatus = \"ready\"\n", 1)
	case "inline-table-array":
		// Written as an array of inline tables, the lifecycle rows arrive as a
		// different Go type from the same rows written as tables.
		head, _, _ := strings.Cut(validContractV1, "[[lifecycle]]")
		return strings.Replace(head,
			"generated_at_must_match = true\n",
			"generated_at_must_match = true\nlifecycle = [{status = \"draft\", Status = \"archived\", applies_to = [\"lesson\"], from = [], owner = []}]\n", 1)
	case "lowercases-together-only":
		// The other direction: these two lowercase to the same string and the
		// decoder still keeps them apart, so they are two unknown keys.
		return strings.Replace(validContractV1,
			"aligned_with = \"Note-Contract.md\"\n",
			"aligned_with = \"Note-Contract.md\"\ninitial = true\n\""+dottedI+"nitial\" = true\n", 1)
	}
	panic("unknown shape " + shape)
}

func TestDecodeRefusesKeysThatFoldTogether(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		shape string
		names [2]string
	}{
		{"top-level", [2]string{`"Aligned_With"`, `"aligned_with"`}},
		{"nested-table", [2]string{`"Navigation"`, `"navigation"`}},
		{"array-of-tables", [2]string{`"Lifecycle"`, `"lifecycle"`}},
		{"key-in-table", [2]string{`"fields.Required"`, `"fields.required"`}},
		{"map-valued-section", [2]string{`"enums.status.Note"`, `"enums.status.note"`}},
		{"quoted-key", [2]string{`"rules.slug_pattern"`, "\"rules.\u017flug_pattern\""}},
		{"inline-table-array", [2]string{`"lifecycle.Status"`, `"lifecycle.status"`}},
		{"row-in-table-array", [2]string{`"lifecycle.Status"`, `"lifecycle.status"`}},
	} {
		t.Run(tc.shape, func(t *testing.T) {
			t.Parallel()

			_, err := decodeContract([]byte(foldedKeyContract(tc.shape)), policySource{})
			if err == nil {
				t.Fatal("decodeContract() error = nil, want a refusal naming both spellings")
			}
			if got := classifyDecodeError(err); got != "folded-keys" {
				t.Fatalf("classifyDecodeError() = %q, want %q (error was %v)", got, "folded-keys", err)
			}
			for _, name := range tc.names {
				if !strings.Contains(err.Error(), name) {
					t.Errorf("refusal %q does not name %s", err.Error(), name)
				}
			}
		})
	}
}

func TestDecodeOfFoldedKeysIsDeterministic(t *testing.T) {
	t.Parallel()

	for _, shape := range []string{
		"top-level", "nested-table", "array-of-tables", "key-in-table",
		"map-valued-section", "quoted-key", "inline-table-array", "row-in-table-array",
		"two-pairs",
	} {
		t.Run(shape, func(t *testing.T) {
			t.Parallel()

			data := []byte(foldedKeyContract(shape))
			_, first := decodeContract(data, policySource{})
			if first == nil {
				t.Fatal("decodeContract() error = nil, want a refusal")
			}
			for i := range 200 {
				_, err := decodeContract(data, policySource{})
				if err == nil || err.Error() != first.Error() {
					t.Fatalf("decode %d differed:\nfirst: %v\nnow:   %v", i, first, err)
				}
			}
		})
	}
}

// TestDecodeKeepsSpellingsTheDecoderTellsApart pins the other side of the
// refusal: a spelling the decoder resolves to one field, and a spelling it
// treats as a different key entirely, both stay as they were.
func TestDecodeKeepsSpellingsTheDecoderTellsApart(t *testing.T) {
	t.Parallel()

	t.Run("one folded spelling per table still binds", func(t *testing.T) {
		t.Parallel()

		// The second lifecycle row writes Status, and nothing else in that row
		// folds to it, so the row binds the way it always did.
		data := strings.Replace(validContractV1,
			"[[lifecycle]]\nstatus = \"archived\"\n",
			"[[lifecycle]]\nStatus = \"archived\"\n", 1)
		contract, err := decodeContract([]byte(data), policySource{})
		if err != nil {
			t.Fatalf("decodeContract() error = %v, want the contract to load", err)
		}
		if got, want := contract.StageCount(), 2; got != want {
			t.Errorf("StageCount() = %d, want %d", got, want)
		}
	})

	t.Run("dotted capital I does not fold to i", func(t *testing.T) {
		t.Parallel()

		// These two keys lowercase to the same string, and the decoder still
		// binds them separately, so what is owed here is the refusal for a key
		// nobody knows, not the one for a spelling nobody can tell apart.
		_, err := decodeContract([]byte(foldedKeyContract("lowercases-together-only")), policySource{})
		if err == nil {
			t.Fatal("decodeContract() error = nil, want the unknown-key refusal")
		}
		if got, want := classifyDecodeError(err), "unknown-core"; got != want {
			t.Fatalf("classifyDecodeError() = %q, want %q (error was %v)", got, want, err)
		}
	})

	t.Run("the shipped contract vocabulary still loads", func(t *testing.T) {
		t.Parallel()

		if _, err := decodeContract([]byte(validContractV1), policySource{}); err != nil {
			t.Fatalf("decodeContract(valid) error = %v", err)
		}
	})
}
