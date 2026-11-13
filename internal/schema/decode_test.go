package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validContract = `schema_version = "2"

[folders]
checked = ["Writing"]
templates = ["System/templates"]
journal = "Diary"
domain_from = ["Writing"]
skipped_files = ["README.md"]

[enums]
type = ["note"]
domain = ["golang"]
level = ["beginner"]

[fields]
common = ["type", "aliases", "created"]
required = ["type"]

[types.lesson]
role = "lesson"
fields = ["slug", "level"]
slug_pattern = "^[a-z]+$"

[types.study-path]
role = "course"

[types.moc]
role = "map"

[rules]
planned_gap_marks = ["Gaps"]

[privacy]
never_egress_dirs = ["Private"]
`

func TestDynamicContractMapKeysDecode(t *testing.T) {
	t.Parallel()

	data := strings.Replace(validContract, "[types.moc]\n", "[types.\"réponse\"]\nfields = [\"based_on\"]\n\n[types.moc]\n", 1)
	contract, err := loadContractBytes(t, []byte(data))
	if err != nil {
		t.Fatalf("LoadFile(dynamic map keys) error = %v", err)
	}
	if !contract.DeclaresType("réponse") {
		t.Error(`DeclaresType("réponse") = false, want the table name read as a type`)
	}
}

func TestContractVersion(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		from string
		to   string
		want string
	}{
		{
			name: "earlier",
			from: `schema_version = "2"`,
			to:   `schema_version = "1"`,
			want: `unsupported schema_version "1"; supported version is "2"`,
		},
		{
			name: "later",
			from: `schema_version = "2"`,
			to:   `schema_version = "3"`,
			want: `unsupported schema_version "3"; supported version is "2"`,
		},
		{
			name: "empty",
			from: `schema_version = "2"`,
			to:   `schema_version = ""`,
			want: `unsupported schema_version ""; supported version is "2"`,
		},
		{
			name: "missing",
			from: "schema_version = \"2\"\n",
			to:   "",
			want: `missing required key "schema_version"`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data := []byte(strings.Replace(validContract, tt.from, tt.to, 1))
			_, err := decodeContract(data, policySource{})
			if err == nil {
				t.Fatalf("decodeContract(%s schema_version) error = nil, want %q", tt.name, tt.want)
			}
			if got := err.Error(); got != tt.want {
				t.Errorf("decodeContract(%s schema_version) error = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestCoreDecodeErrorsCloseContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "wrong core type",
			data: []byte(strings.Replace(validContract, `schema_version = "2"`, `schema_version = 2`, 1)),
		},
		{
			name: "malformed TOML",
			data: []byte(strings.Replace(validContract, `schema_version = "2"`, `schema_version = ["2"`, 1)),
		},
		{
			name: "wrong folders type",
			data: []byte(strings.Replace(validContract, `journal = "Diary"`, `journal = ["Diary"]`, 1)),
		},
		{
			name: "wrong type table type",
			data: []byte(strings.Replace(validContract, `role = "course"`, `role = ["course"]`, 1)),
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

	for _, tt := range []struct {
		name string
		from string
		to   string
		want string
	}{
		{
			name: "top level",
			from: "schema_version = \"2\"\n",
			to:   "schema_version = \"2\"\nunknown_zeta = true\nunknown_alpha = true\n",
			want: `unknown keys: "unknown_alpha", "unknown_zeta"`,
		},
		{
			name: "folders",
			from: "journal = \"Diary\"\n",
			to:   "journal = \"Diary\"\nunknown_zeta = true\nunknown_alpha = true\n",
			want: `unknown keys: "folders.unknown_alpha", "folders.unknown_zeta"`,
		},
		{
			name: "a type table",
			from: "role = \"course\"\n",
			to:   "role = \"course\"\nunknown_zeta = true\nunknown_alpha = true\n",
			want: `unknown keys: "types.study-path.unknown_alpha", "types.study-path.unknown_zeta"`,
		},
		{
			name: "fields",
			from: "required = [\"type\"]\n",
			to:   "required = [\"type\"]\nunknown_zeta = true\nunknown_alpha = true\n",
			want: `unknown keys: "fields.unknown_alpha", "fields.unknown_zeta"`,
		},
		{
			name: "rules",
			from: "planned_gap_marks = [\"Gaps\"]\n",
			to:   "planned_gap_marks = [\"Gaps\"]\nunknown_zeta = true\nunknown_alpha = true\n",
			want: `unknown keys: "rules.unknown_alpha", "rules.unknown_zeta"`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data := []byte(strings.Replace(validContract, tt.from, tt.to, 1))
			_, err := loadContractBytes(t, data)
			if err == nil {
				t.Fatal("LoadFile(unknown keys) error = nil, want hard error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("LoadFile(unknown keys) error = %q, want substring %q", err, tt.want)
			}
		})
	}
}

func TestUnknownPrivacyKeysInvalidateOnlyPrivacy(t *testing.T) {
	t.Parallel()

	data := []byte(strings.Replace(
		validContract,
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
	if !s.DeclaresType("lesson") {
		t.Error(`DeclaresType("lesson") = false, want the core vocabulary loaded`)
	}
}

func TestPrivacyTypeErrorInvalidatesOnlyPrivacy(t *testing.T) {
	t.Parallel()

	data := []byte(strings.Replace(
		validContract,
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
	if !s.DeclaresType("lesson") {
		t.Error(`DeclaresType("lesson") = false, want the core vocabulary loaded`)
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
	errClass             string
	version              string
	typeRoles            string
	navigationAvailable  bool
	navigationDiagnostic string
	artifactAvailable    bool
	artifactDiagnostic   string
	privacyAvailable     bool
	privacyDiagnostic    string
	journalAvailable     bool
	journalDiagnostic    string
}

func classifyDecode(data []byte) decodeClassification {
	s, err := decodeContract(data, policySource{})
	if err != nil {
		return decodeClassification{errClass: classifyDecodeError(err)}
	}
	return decodeClassification{
		version:              s.Version(),
		typeRoles:            typeRoles(s),
		navigationAvailable:  s.NavigationRoles().Available(),
		navigationDiagnostic: s.NavigationRoles().Diagnostic(),
		artifactAvailable:    s.ArtifactPolicy().Available(),
		artifactDiagnostic:   s.ArtifactPolicy().Diagnostic(),
		privacyAvailable:     s.PrivacyPolicy().Available(),
		privacyDiagnostic:    s.PrivacyPolicy().Diagnostic(),
		journalAvailable:     s.JournalDir().Available(),
		journalDiagnostic:    s.JournalDir().Diagnostic(),
	}
}

// typeRoles names the types a decode kept with their roles, in the order it
// kept them. The count alone let two decodes that kept different types of the
// same length compare equal, which is the shape of the defect this comparison
// exists to catch.
func typeRoles(contract *Contract) string {
	names := make([]string, len(contract.types))
	for i, def := range contract.types {
		names[i] = def.name + ":" + string(def.role)
	}
	return strings.Join(names, ",")
}

func classifyDecodeError(err error) string {
	switch message := err.Error(); {
	case message == `missing required key "schema_version"`:
		return "missing-version"
	case strings.HasPrefix(message, "unsupported schema_version "):
		return "unsupported-version"
	case strings.HasPrefix(message, "unknown keys: "):
		return "unknown-core"
	case strings.HasPrefix(message, "contract keys differ only by letter case: "):
		return "folded-keys"
	case strings.HasPrefix(message, "contract nests deeper than "):
		return "too-deep"
	case strings.HasPrefix(message, "contract is larger than "):
		return "too-large"
	case strings.HasPrefix(message, "toml:"):
		return "toml-decode"
	default:
		return message
	}
}

func FuzzDecodeContractDeterministic(f *testing.F) {
	seeds := [][]byte{
		nil,
		[]byte(validContract),
		[]byte(`schema_version = "2"`),
		[]byte(`schema_version = "1"`),
		[]byte("not = [valid"),
		[]byte(strings.Replace(validContract, "schema_version = \"2\"\n", "schema_version = \"2\"\nunknown = true\n", 1)),
		[]byte(strings.Replace(validContract, "journal = \"Diary\"\n", "journal = \"Diary\"\nunknown = true\n", 1)),
		[]byte(strings.Replace(validContract, "role = \"course\"\n", "role = \"course\"\nunknown = true\n", 1)),
		[]byte(strings.Replace(validContract, "never_egress_dirs = [\"Private\"]\n", "never_egress_dirs = [\"Private\"]\nunknown = true\n", 1)),
		[]byte(strings.Replace(validContract, `role = "course"`, `role = "lesson"`, 1)),
		[]byte(strings.Replace(validContract, `role = "course"`, `role = "unknown"`, 1)),
		[]byte(strings.Replace(validContract, `templates = ["System/templates"]`, `templates = ["../x"]`, 1)),
		[]byte(strings.Replace(validContract, `journal = "Diary"`, `journal = "../Diary"`, 1)),
		[]byte(strings.Replace(validContract, `never_egress_dirs = ["Private"]`, `never_egress_dirs = "Private"`, 1)),
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
	// These seeds exercise deterministic decoding, not a regression-red oracle.
	for _, roots := range []string{
		`["Writing/lessons"]`, `["Archive/studies"]`,
		`["Writing", "Writing/lessons"]`, `["Writing/lessons", "Writing"]`,
		`["Writing/lessons", "Writing/lessonship"]`, `["Writing/../lessons"]`,
		`["Writing/\u0000lessons"]`,
	} {
		seeds = append(seeds, []byte(strings.Replace(semanticallyValidContract,
			`domain_from = ["Writing"]`, `domain_from = `+roots, 1)))
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
//
// The two non-ASCII shapes are built from their code points rather than typed,
// because U+017F reads as an f and U+0130 as an I, and a reviewer has to be
// able to see which letter a case is about.
func foldedKeyContract(shape string) string {
	const (
		longS   = "\u017f" // ſ, which the decoder folds to s
		dottedI = "\u0130" // İ, which the decoder does not fold to i
	)
	const versionLine = "schema_version = \"2\"\n"
	switch shape {
	case "top-level":
		return strings.Replace(validContract, versionLine, versionLine+"Schema_version = \"2\"\n", 1)
	case "nested-table":
		return validContract + "\n[Folders]\nchecked = [\"Writing\"]\n"
	case "array-of-tables":
		return validContract + "\n[[rows]]\nx = 1\n\n[[Rows]]\nx = 2\n"
	case "key-in-table":
		return strings.Replace(validContract,
			"required = [\"type\"]\n",
			"required = [\"type\"]\nRequired = [\"aliases\"]\n", 1)
	case "map-valued-section":
		return strings.Replace(validContract,
			"domain = [\"golang\"]\n",
			"domain = [\"golang\"]\nDomain = [\"rust\"]\n", 1)
	case "quoted-key":
		// A key outside the ASCII set has to be quoted to be a key at all, and
		// this pair separates the decoder's own folding from a lowercasing
		// comparison: the two fold together and do not lowercase together.
		return strings.Replace(validContract,
			"slug_pattern = \"^[a-z]+$\"\n",
			"slug_pattern = \"^[a-z]+$\"\n\""+longS+"lug_pattern\" = \"^[0-9]+$\"\n", 1)
	case "two-pairs":
		// Two folded pairs in two different tables, so the order they are
		// reported in is observable.
		withFields := strings.Replace(validContract,
			"required = [\"type\"]\n",
			"required = [\"type\"]\nRequired = [\"aliases\"]\n", 1)
		return strings.Replace(withFields,
			"domain = [\"golang\"]\n",
			"domain = [\"golang\"]\nDomain = [\"rust\"]\n", 1)
	case "row-in-table-array":
		// Both spellings inside one row of an array of tables: the ambiguity is
		// in the row, not among the row headers.
		return validContract + "\n[[rows]]\nstatus = 1\nStatus = 2\n"
	case "inline-table-array":
		// Written as an array of inline tables, the rows arrive as a different
		// Go type from the same rows written as tables.
		return strings.Replace(validContract, versionLine,
			versionLine+"rows = [{status = \"draft\", Status = \"archived\"}]\n", 1)
	case "lowercases-together-only":
		// The other direction: these two lowercase to the same string and the
		// decoder still keeps them apart, so they are two unknown keys.
		return strings.Replace(validContract, versionLine,
			versionLine+"initial = true\n\""+dottedI+"nitial\" = true\n", 1)
	}
	panic("unknown shape " + shape)
}

func TestDecodeRefusesKeysThatFoldTogether(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		shape string
		names [2]string
	}{
		{"top-level", [2]string{`"Schema_version"`, `"schema_version"`}},
		{"nested-table", [2]string{`"Folders"`, `"folders"`}},
		{"array-of-tables", [2]string{`"Rows"`, `"rows"`}},
		{"key-in-table", [2]string{`"fields.Required"`, `"fields.required"`}},
		{"map-valued-section", [2]string{`"enums.Domain"`, `"enums.domain"`}},
		{"quoted-key", [2]string{`"types.lesson.slug_pattern"`, "\"types.lesson.\u017flug_pattern\""}},
		{"inline-table-array", [2]string{`"rows.Status"`, `"rows.status"`}},
		{"row-in-table-array", [2]string{`"rows.Status"`, `"rows.status"`}},
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

		// The folders table writes Checked, and nothing else in that table
		// folds to it, so the key binds the way it always did.
		data := strings.Replace(validContract, `checked = ["Writing"]`, `Checked = ["Writing"]`, 1)
		contract, err := decodeContract([]byte(data), policySource{})
		if err != nil {
			t.Fatalf("decodeContract() error = %v, want the contract to load", err)
		}
		if got := contract.KnowledgeScope().Declared(); len(got) != 1 || got[0] != "Writing" {
			t.Errorf("KnowledgeScope().Declared() = %q, want [Writing]", got)
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

		if _, err := decodeContract([]byte(validContract), policySource{}); err != nil {
			t.Fatalf("decodeContract(valid) error = %v", err)
		}
	})
}
