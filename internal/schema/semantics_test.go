package schema

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

const semanticallyValidContract = `schema_version = "2"

[folders]
checked = ["Writing"]
ignored = []
templates = []
domain_from = ["Writing"]
skipped_files = ["README.md"]

[enums]
type = ["inbox"]
domain = ["golang"]
level = ["beginner"]

[fields]
common = ["type", "aliases", "lang", "created"]
required = ["type"]

[types.lesson]
role = "lesson"
required = ["slug"]
fields = ["slug", "level", "prerequisites"]
slug_pattern = "^[a-z]+$"

[types.concept]

[types.thought]
role = "thought"
fields = ["based_on"]

[rules]
concept_requires_provenance = ["based_on"]

[privacy]
never_egress_dirs = []
`

// decodeError is what decodeContract says about data, empty when it loads.
func decodeError(t *testing.T, data string) string {
	t.Helper()
	_, err := decodeContract([]byte(data), policySource{})
	if err == nil {
		return ""
	}
	return err.Error()
}

type semanticCase struct {
	name    string
	from    string
	to      string
	wantErr string
}

func runSemanticCases(t *testing.T, tests []semanticCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data := replaceContractText(t, semanticallyValidContract, tt.from, tt.to)
			got := decodeError(t, data)
			if tt.wantErr == "" {
				if got != "" {
					t.Fatalf("decodeContract() error = %q, want nil", got)
				}
				return
			}
			if got == "" {
				t.Fatalf("decodeContract() error = nil, want substring %q", tt.wantErr)
			}
			if !strings.Contains(got, tt.wantErr) {
				t.Errorf("decodeContract() error = %q, want substring %q", got, tt.wantErr)
			}
		})
	}
}

func TestCoreSemanticsEnums(t *testing.T) {
	t.Parallel()

	runSemanticCases(t, []semanticCase{
		{
			name:    "type empty element",
			from:    `type = ["inbox"]`,
			to:      `type = ["inbox", ""]`,
			wantErr: "types: empty type name",
		},
		{
			name:    "type spelled twice once the words are folded",
			from:    `type = ["inbox"]`,
			to:      "type = [\"caf\u00e9\", \"cafe\u0301\"]",
			wantErr: "types: duplicate type \"caf\u00e9\"",
		},
		{
			name:    "domain duplicate",
			from:    `domain = ["golang"]`,
			to:      `domain = ["golang", "golang"]`,
			wantErr: `enums.domain: duplicate value "golang"`,
		},
		{
			name:    "level empty element",
			from:    `level = ["beginner"]`,
			to:      `level = [""]`,
			wantErr: "enums.level: empty value",
		},
		{
			name:    "level duplicate once the words are folded",
			from:    `level = ["beginner"]`,
			to:      "level = [\"caf\u00e9\", \"cafe\u0301\"]",
			wantErr: "enums.level: duplicate value \"caf\u00e9\"",
		},
		{
			name: "enum errors are sorted",
			from: `level = ["beginner"]`,
			to: `level = ["duplicate", "duplicate"]
aliases = ["duplicate", "duplicate"]`,
			wantErr: `enums.aliases: duplicate value "duplicate"`,
		},
	})
}

func TestCoreSemanticsFields(t *testing.T) {
	t.Parallel()

	runSemanticCases(t, []semanticCase{
		{
			name:    "common empty element",
			from:    `common = ["type", "aliases", "lang", "created"]`,
			to:      `common = ["type", ""]`,
			wantErr: "fields.common: empty value",
		},
		{
			name:    "common duplicate",
			from:    `common = ["type", "aliases", "lang", "created"]`,
			to:      `common = ["type", "lang", "lang"]`,
			wantErr: `fields.common: duplicate value "lang"`,
		},
		{
			name:    "required empty element",
			from:    `required = ["type"]`,
			to:      `required = ["type", ""]`,
			wantErr: `fields.required: empty value`,
		},
		{
			name:    "required duplicate",
			from:    `required = ["type"]`,
			to:      `required = ["type", "type"]`,
			wantErr: `fields.required: duplicate value "type"`,
		},
		{
			name:    "required outside common",
			from:    `required = ["type"]`,
			to:      `required = ["type", "slug"]`,
			wantErr: `fields.required: "slug" is not listed in fields.common`,
		},
		{
			name:    "type fields duplicate",
			from:    `fields = ["slug", "level", "prerequisites"]`,
			to:      `fields = ["slug", "level", "level"]`,
			wantErr: `types.lesson.fields: duplicate value "level"`,
		},
		{
			name:    "type required duplicate",
			from:    `required = ["slug"]`,
			to:      `required = ["slug", "slug"]`,
			wantErr: `types.lesson.required: duplicate value "slug"`,
		},
		{
			name:    "type required outside its fields",
			from:    `required = ["slug"]`,
			to:      `required = ["slug", "based_on"]`,
			wantErr: `types.lesson.required: "based_on" is not listed in types.lesson.fields`,
		},
		{
			name:    "type required from common",
			from:    `required = ["slug"]`,
			to:      `required = ["slug", "created"]`,
			wantErr: "",
		},
	})
}

// TestTheFieldListIsClosed holds the closed list where a contract writes it: a
// field yomihon neither defines nor reserves for Obsidian is refused wherever
// the contract lists it, and the two derived names are refused like any other.
func TestTheFieldListIsClosed(t *testing.T) {
	t.Parallel()

	const commonLine = `common = ["type", "aliases", "lang", "created"]`
	const lessonFields = `fields = ["slug", "level", "prerequisites"]`
	var tests []semanticCase
	for _, field := range []string{"title", "domain", "status", "updated", "replaces", "topics", "source_kind"} {
		tests = append(tests,
			semanticCase{
				name:    field + " in common",
				from:    commonLine,
				to:      `common = ["type", "aliases", "lang", "created", "` + field + `"]`,
				wantErr: `fields.common: "` + field + `" is not a field yomihon defines`,
			},
			semanticCase{
				name:    field + " in a type",
				from:    lessonFields,
				to:      `fields = ["slug", "level", "prerequisites", "` + field + `"]`,
				wantErr: `types.lesson.fields: "` + field + `" is not a field yomihon defines`,
			},
		)
	}
	for _, field := range []string{"tags", "aliases", "cssclasses", "draft", "based_on", "verified_on", "verified_with"} {
		tests = append(tests, semanticCase{
			name: field + " is a field a contract may list",
			from: lessonFields,
			to:   `fields = ["slug", "level", "prerequisites", "` + field + `"]`,
		})
	}
	runSemanticCases(t, tests)
}

func TestTheRolesNeedTheirFields(t *testing.T) {
	t.Parallel()

	runSemanticCases(t, []semanticCase{
		{
			name: "a lesson type without slug",
			from: `required = ["slug"]
fields = ["slug", "level", "prerequisites"]
slug_pattern = "^[a-z]+$"`,
			to:      `fields = ["level", "prerequisites"]`,
			wantErr: "types.lesson: the lesson role needs slug in types.lesson.fields",
		},
		{
			name:    "a thought type without based_on",
			from:    "role = \"thought\"\nfields = [\"based_on\"]",
			to:      "role = \"thought\"\nfields = [\"lang\"]",
			wantErr: "types.thought: the thought role needs based_on in types.thought.fields",
		},
		{
			name:    "a thought type given based_on by fields.common",
			from:    "common = [\"type\", \"aliases\", \"lang\", \"created\"]",
			to:      "common = [\"type\", \"aliases\", \"lang\", \"created\", \"based_on\"]",
			wantErr: "",
		},
		{
			name: "a slug pattern on a type without slug",
			from: "[types.concept]\n",
			to: `[types.concept]
slug_pattern = "^[a-z]+$"
`,
			wantErr: "types.concept.slug_pattern: the type does not list slug in fields",
		},
		{
			name:    "an invalid slug pattern",
			from:    `slug_pattern = "^[a-z]+$"`,
			to:      `slug_pattern = "["`,
			wantErr: "types.lesson.slug_pattern: invalid regular expression",
		},
	})
}

func TestCoreSemanticsRules(t *testing.T) {
	t.Parallel()

	const provenance = `concept_requires_provenance = ["based_on"]`
	runSemanticCases(t, []semanticCase{
		{
			name:    "domain folder empty element",
			from:    `domain_from = ["Writing"]`,
			to:      `domain_from = [""]`,
			wantErr: "folders.domain_from: empty value",
		},
		{
			name:    "domain folder duplicate",
			from:    `domain_from = ["Writing"]`,
			to:      `domain_from = ["Writing", "Writing"]`,
			wantErr: `folders.domain_from: duplicate value "Writing"`,
		},
		{
			name:    "domain folder has backslash",
			from:    `domain_from = ["Writing"]`,
			to:      `domain_from = ["Writing\\notes"]`,
			wantErr: `folders.domain_from: unsafe relative directory path "Writing\\notes"`,
		},
		{
			name:    "domain folder is current directory",
			from:    `domain_from = ["Writing"]`,
			to:      `domain_from = ["."]`,
			wantErr: `folders.domain_from: unsafe relative directory path "."`,
		},
		{
			name:    "domain folder is parent directory",
			from:    `domain_from = ["Writing"]`,
			to:      `domain_from = [".."]`,
			wantErr: `folders.domain_from: unsafe relative directory path ".."`,
		},
		{
			name:    "domain folder is absolute",
			from:    `domain_from = ["Writing"]`,
			to:      `domain_from = ["/Writing"]`,
			wantErr: `folders.domain_from: unsafe relative directory path "/Writing"`,
		},
		{
			name:    "domain folder contains NUL",
			from:    `domain_from = ["Writing"]`,
			to:      `domain_from = ["\u0000"]`,
			wantErr: `folders.domain_from: unsafe relative directory path "\x00"`,
		},
		{
			name:    "concept provenance empty element",
			from:    provenance,
			to:      `concept_requires_provenance = ["based_on", ""]`,
			wantErr: "rules.concept_requires_provenance: empty value",
		},
		{
			name:    "concept provenance duplicate",
			from:    provenance,
			to:      `concept_requires_provenance = ["based_on", "based_on"]`,
			wantErr: `rules.concept_requires_provenance: duplicate value "based_on"`,
		},
		{
			name:    "concept type without provenance",
			from:    provenance,
			to:      `concept_requires_provenance = []`,
			wantErr: `rules.concept_requires_provenance must not be empty when the types include "concept"`,
		},
		{
			name:    "planned gap marks empty element",
			from:    provenance,
			to:      provenance + "\nplanned_gap_marks = [\"\"]",
			wantErr: "rules.planned_gap_marks: empty value",
		},
		{
			name:    "planned gap marks duplicate",
			from:    provenance,
			to:      provenance + "\nplanned_gap_marks = [\"Gaps\", \"Gaps\"]",
			wantErr: `rules.planned_gap_marks: duplicate value "Gaps"`,
		},
		{
			name:    "planned inline marks empty element",
			from:    provenance,
			to:      provenance + "\nplanned_inline_marks = [\"\"]",
			wantErr: "rules.planned_inline_marks: empty value",
		},
		{
			name:    "planned inline marks duplicate",
			from:    provenance,
			to:      provenance + "\nplanned_inline_marks = [\"todo\", \"todo\"]",
			wantErr: `rules.planned_inline_marks: duplicate value "todo"`,
		},
	})
}

// TestPlannedMarksAreLoadedFromTheContract holds the loader polarity: a
// contract that omits the keys is loaded with today's dialect defaults, so an
// existing vault does not change behaviour; a contract that writes an empty
// list tracks no mark; a contract that writes English marks keeps those.
func TestPlannedMarksAreLoadedFromTheContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		extra      string
		wantGap    []string
		wantInline []string
	}{
		{
			name:       "omitted keys load today's dialect defaults",
			wantGap:    DefaultPlannedGapMarks(),
			wantInline: DefaultPlannedInlineMarks(),
		},
		{
			name: "declared English heading marks replace the heading default",
			extra: `
planned_gap_marks = ["Gaps"]`,
			wantGap:    []string{"Gaps"},
			wantInline: DefaultPlannedInlineMarks(),
		},
		{
			name: "an empty list is a declaration of none",
			extra: `
planned_gap_marks = []
planned_inline_marks = []`,
			wantGap:    []string{},
			wantInline: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			const provenance = `concept_requires_provenance = ["based_on"]`
			data := semanticallyValidContract
			if tt.extra != "" {
				data = replaceContractText(t, data, provenance, provenance+tt.extra)
			}
			got, err := decodeContract([]byte(data), policySource{})
			if err != nil {
				t.Fatalf("decodeContract() error = %v", err)
			}
			rules := got.Definition().Rules
			if diff := cmp.Diff(tt.wantGap, rules.PlannedGapMarks); diff != "" {
				t.Errorf("PlannedGapMarks mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantInline, rules.PlannedInlineMarks); diff != "" {
				t.Errorf("PlannedInlineMarks mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCoreSemanticsFolders(t *testing.T) {
	t.Parallel()

	const checked = `checked = ["Writing"]`
	const skipped = `skipped_files = ["README.md"]`
	runSemanticCases(t, []semanticCase{
		{name: "checked directory empty element", from: checked, to: `checked = [""]`, wantErr: "folders.checked: empty value"},
		{name: "checked directory duplicate", from: checked, to: `checked = ["Writing", "Writing"]`, wantErr: `folders.checked: duplicate value "Writing"`},
		{name: "checked directory has slash", from: checked, to: `checked = ["Writing/notes"]`, wantErr: `folders.checked: unsafe top-level component "Writing/notes"`},
		{name: "checked directory has backslash", from: checked, to: `checked = ["Writing\\notes"]`, wantErr: `folders.checked: unsafe top-level component "Writing\\notes"`},
		{name: "checked directory is current directory", from: checked, to: `checked = ["."]`, wantErr: `folders.checked: unsafe top-level component "."`},
		{name: "checked directory is parent directory", from: checked, to: `checked = [".."]`, wantErr: `folders.checked: unsafe top-level component ".."`},
		{name: "checked directory is absolute", from: checked, to: `checked = ["/Writing"]`, wantErr: `folders.checked: unsafe top-level component "/Writing"`},
		{name: "checked directory contains NUL", from: checked, to: `checked = ["\u0000"]`, wantErr: `folders.checked: unsafe top-level component "\x00"`},
		{name: "skipped file empty element", from: skipped, to: `skipped_files = [""]`, wantErr: "folders.skipped_files: empty value"},
		{name: "skipped file duplicate", from: skipped, to: `skipped_files = ["README.md", "README.md"]`, wantErr: `folders.skipped_files: duplicate value "README.md"`},
		{name: "skipped file has slash", from: skipped, to: `skipped_files = ["System/README.md"]`, wantErr: `folders.skipped_files: invalid basename "System/README.md"`},
		{name: "skipped file has backslash", from: skipped, to: `skipped_files = ["System\\README.md"]`, wantErr: `folders.skipped_files: invalid basename "System\\README.md"`},
		{name: "skipped file is current directory", from: skipped, to: `skipped_files = ["."]`, wantErr: `folders.skipped_files: invalid basename "."`},
		{name: "skipped file is parent directory", from: skipped, to: `skipped_files = [".."]`, wantErr: `folders.skipped_files: invalid basename ".."`},
		{name: "skipped file contains NUL", from: skipped, to: `skipped_files = ["\u0000"]`, wantErr: `folders.skipped_files: invalid basename "\x00"`},
	})
}

func TestIgnoredFoldersValidation(t *testing.T) {
	t.Parallel()

	const declared = `ignored = []`
	tests := []struct {
		name    string
		to      string
		wantErr string
	}{
		{name: "omitted key is legal", to: ""},
		{name: "empty list is legal", to: declared},
		{name: "nested glob", to: `ignored = ["courses/*/lab"]`},
		{name: "literal path", to: `ignored = ["System/templates"]`},
		{name: "several patterns", to: `ignored = ["courses/*/lab", "*/tpl"]`},
		{name: "empty element", to: `ignored = [""]`, wantErr: "folders.ignored: empty value"},
		{name: "duplicate", to: `ignored = ["lab", "lab"]`, wantErr: `folders.ignored: duplicate value "lab"`},
		{name: "parent segment", to: `ignored = ["../lab"]`, wantErr: `folders.ignored: pattern "../lab" climbs out of the vault`},
		{name: "parent segment in the middle", to: `ignored = ["courses/../lab"]`, wantErr: `folders.ignored: pattern "courses/../lab" is not a clean relative path`},
		{name: "absolute", to: `ignored = ["/courses/lab"]`, wantErr: `folders.ignored: pattern "/courses/lab" is absolute`},
		{name: "trailing slash", to: `ignored = ["courses/lab/"]`, wantErr: `folders.ignored: pattern "courses/lab/" is not a clean relative path`},
		{name: "empty segment", to: `ignored = ["courses//lab"]`, wantErr: `folders.ignored: pattern "courses//lab" is not a clean relative path`},
		{name: "current directory", to: `ignored = ["."]`, wantErr: `folders.ignored: pattern "." is not a clean relative path`},
		{name: "malformed glob", to: `ignored = ["courses/[lab"]`, wantErr: `folders.ignored: pattern "courses/[lab" is not a valid pattern`},
		{name: "NUL", to: `ignored = ["a\u0000b"]`, wantErr: `contains a NUL byte`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data := replaceContractText(t, semanticallyValidContract, declared, tt.to)
			got := decodeError(t, data)
			if tt.wantErr == "" {
				if got != "" {
					t.Fatalf("decodeContract() error = %q, want nil", got)
				}
				return
			}
			if got == "" {
				t.Fatalf("decodeContract() error = nil, want substring %q", tt.wantErr)
			}
			if !strings.Contains(got, tt.wantErr) {
				t.Errorf("decodeContract() error = %q, want substring %q", got, tt.wantErr)
			}
		})
	}
}

// TestScanBoundaryFollowsIgnoredFolders pins what a contract's ignored list
// means once loaded: a directory it names is beyond the scan, a sibling is not,
// and a contract that declares nothing excludes nothing but the dot names.
func TestScanBoundaryFollowsIgnoredFolders(t *testing.T) {
	t.Parallel()

	data := replaceContractText(t, semanticallyValidContract, `ignored = []`, `ignored = ["courses/*/lab"]`)
	contract, err := decodeContract([]byte(data), policySource{})
	if err != nil {
		t.Fatalf("decodeContract() error = %v", err)
	}
	boundary := contract.ScanBoundary()
	for relPath, want := range map[string]bool{
		"courses/C1/lab/main.go":   true,
		"courses/C1/lab/deep/x.go": true,
		"courses/C1/modules/a.md":  false,
		"courses/lab/main.go":      false,
		"courses/C1/x/lab/main.go": false,
		"courses/C1/lab":           false,
		"courses/C1/.hidden/a.md":  true,
		"Notes/lab/main.go":        false,
	} {
		if got := boundary.OutsideScan(relPath); got != want {
			t.Errorf("OutsideScan(%q) = %v, want %v", relPath, got, want)
		}
	}

	plain, err := decodeContract([]byte(semanticallyValidContract), policySource{})
	if err != nil {
		t.Fatalf("decodeContract() error = %v", err)
	}
	if plain.ScanBoundary().OutsideScan("courses/C1/lab/main.go") {
		t.Error("a contract without ignored folders excludes courses/C1/lab/main.go")
	}
	var absent *Contract
	if absent.ScanBoundary().OutsideScan("courses/C1/lab/main.go") {
		t.Error("an ungoverned vault excludes courses/C1/lab/main.go")
	}
}

func TestCoreSemanticsGreenControls(t *testing.T) {
	t.Parallel()

	t.Run("optional enums written empty", func(t *testing.T) {
		t.Parallel()

		data := semanticallyValidContract
		for _, replacement := range []struct {
			from string
			to   string
		}{
			{from: `domain = ["golang"]`, to: `domain = []`},
			{from: `level = ["beginner"]`, to: `level = []`},
		} {
			data = replaceContractText(t, data, replacement.from, replacement.to)
		}
		if got := decodeError(t, data); got != "" {
			t.Fatalf("decodeContract(empty optional enums) error = %q", got)
		}
	})

	t.Run("empty optional rule and folder lists without governed types", func(t *testing.T) {
		t.Parallel()

		data := semanticallyValidContract
		for _, replacement := range []struct {
			from string
			to   string
		}{
			{
				from: "[types.lesson]\nrole = \"lesson\"\nrequired = [\"slug\"]\nfields = [\"slug\", \"level\", \"prerequisites\"]\nslug_pattern = \"^[a-z]+$\"\n\n",
				to:   "",
			},
			{from: "[types.concept]\n\n", to: ""},
			{from: "[types.thought]\nrole = \"thought\"\nfields = [\"based_on\"]\n\n", to: ""},
			{from: `concept_requires_provenance = ["based_on"]`, to: `concept_requires_provenance = []`},
			{from: `domain_from = ["Writing"]`, to: `domain_from = []`},
			{from: `checked = ["Writing"]`, to: `checked = []`},
			{from: `skipped_files = ["README.md"]`, to: `skipped_files = []`},
		} {
			data = replaceContractText(t, data, replacement.from, replacement.to)
		}
		if got := decodeError(t, data); got != "" {
			t.Fatalf("decodeContract(empty optional core lists) error = %q", got)
		}
	})

	t.Run("an omitted privacy section stays degraded while the rest holds", func(t *testing.T) {
		t.Parallel()

		data := replaceContractText(t, semanticallyValidContract, "\n[privacy]\nnever_egress_dirs = []\n", "")
		s, err := decodeContract([]byte(data), policySource{})
		if err != nil {
			t.Fatalf("decodeContract(absent privacy) error = %v", err)
		}
		if s.PrivacyPolicy().Available() || s.PrivacyPolicy().Trustworthy() {
			t.Error("PrivacyPolicy() of a contract that omitted [privacy] is available or trustworthy, want fail-closed")
		}
		if !s.NavigationRoles().Available() {
			t.Errorf("NavigationRoles().Available() = false, diagnostic %q", s.NavigationRoles().Diagnostic())
		}
		if !s.ArtifactPolicy().Available() {
			t.Errorf("ArtifactPolicy().Available() = false, diagnostic %q", s.ArtifactPolicy().Diagnostic())
		}
	})

	t.Run("invalid privacy type does not close core or siblings", func(t *testing.T) {
		t.Parallel()

		data := replaceContractText(
			t,
			semanticallyValidContract,
			`never_egress_dirs = []`,
			`never_egress_dirs = "Private"`,
		)
		s, err := decodeContract([]byte(data), policySource{})
		if err != nil {
			t.Fatalf("decodeContract(invalid privacy type) error = %v", err)
		}
		if s.PrivacyPolicy().Available() {
			t.Error("PrivacyPolicy().Available() = true, want false")
		}
		if !s.NavigationRoles().Available() {
			t.Errorf("NavigationRoles().Available() = false, diagnostic %q", s.NavigationRoles().Diagnostic())
		}
		if !s.ArtifactPolicy().Available() {
			t.Errorf("ArtifactPolicy().Available() = false, diagnostic %q", s.ArtifactPolicy().Diagnostic())
		}
	})
}

func TestClassifyDecodeErrorPreservesSemanticMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "semantic error",
			err:  errors.New(`enums.title: "title" is not a field yomihon defines or derives`),
			want: `enums.title: "title" is not a field yomihon defines or derives`,
		},
		{
			name: "TOML decoder error",
			err:  errors.New(`toml: line 3: incompatible types`),
			want: "toml-decode",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := classifyDecodeError(tt.err); got != tt.want {
				t.Errorf("classifyDecodeError(%q) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}

func replaceContractText(t *testing.T, contract, from, to string) string {
	t.Helper()
	if got := strings.Count(contract, from); got != 1 {
		t.Fatalf("fixture mutation needle count = %d, want 1 for %q", got, from)
	}
	return strings.Replace(contract, from, to, 1)
}

// TestLoadDomainRoots catches slash rejection, unsafe path cleaning, and
// ambiguous folded root selection regardless of declaration order or distance.
func TestLoadDomainRoots(t *testing.T) {
	t.Parallel()
	const (
		overlap    = "folders.domain_from: overlapping roots"
		unsafePath = "folders.domain_from: unsafe relative directory path"
	)
	tests := []struct {
		name    string
		roots   string
		want    []string
		wantErr string
	}{
		{name: "top level", roots: `["Writing"]`, want: []string{"Writing"}},
		{name: "nested", roots: `["Writing/lessons"]`, want: []string{"Writing/lessons"}},
		{name: "renamed nested", roots: `["Archive/studies"]`, want: []string{"Archive/studies"}},
		{name: "empty roots", roots: `[]`, want: []string{}},
		{name: "siblings forward", roots: `["Writing/lessons", "Writing/lessonship"]`, want: []string{"Writing/lessons", "Writing/lessonship"}},
		{name: "siblings reverse", roots: `["Writing/lessonship", "Writing/lessons"]`, want: []string{"Writing/lessonship", "Writing/lessons"}},
		{name: "folded siblings forward", roots: `["Writing/lessons", "writing/lessonship"]`, want: []string{"Writing/lessons", "writing/lessonship"}},
		{name: "folded siblings reverse", roots: `["writing/lessonship", "Writing/lessons"]`, want: []string{"writing/lessonship", "Writing/lessons"}},
		{name: "normalization distinct", roots: `["Caf\u00e9", "Cafe\u0301/studies"]`, want: []string{"Caf\u00e9", "Cafe\u0301/studies"}},
		{name: "unicode expansion distinct", roots: `["Stra\u00dfe", "STRASSE/lessons"]`, want: []string{"Stra\u00dfe", "STRASSE/lessons"}},
		{name: "leading whitespace distinct", roots: `[" Writing", "Writing/lessons"]`, want: []string{" Writing", "Writing/lessons"}},
		{name: "trailing whitespace distinct", roots: `["Writing ", "Writing/lessons"]`, want: []string{"Writing ", "Writing/lessons"}},
		{name: "component whitespace distinct", roots: `["Writing/ lessons", "Writing/lessons/extra"]`, want: []string{"Writing/ lessons", "Writing/lessons/extra"}},
		{name: "ancestor forward", roots: `["Writing", "Writing/lessons"]`, wantErr: overlap},
		{name: "ancestor reverse", roots: `["Writing/lessons", "Writing"]`, wantErr: overlap},
		{name: "nested ancestor forward", roots: `["Writing/lessons", "Writing/lessons/extra"]`, wantErr: overlap},
		{name: "nested ancestor reverse", roots: `["Writing/lessons/extra", "Writing/lessons"]`, wantErr: overlap},
		{name: "folded ancestor forward", roots: `["Writing", "writing/lessons"]`, wantErr: overlap},
		{name: "folded ancestor reverse", roots: `["writing/lessons", "Writing"]`, wantErr: overlap},
		{name: "folded equality", roots: `["Writing", "writing"]`, wantErr: overlap},
		{name: "nested folded ancestor forward", roots: `["Writing/lessons", "writing/LESSONS/extra"]`, wantErr: overlap},
		{name: "nested folded ancestor reverse", roots: `["writing/LESSONS/extra", "Writing/lessons"]`, wantErr: overlap},
		{name: "nested folded equality", roots: `["Writing/lessons", "writing/LESSONS"]`, wantErr: overlap},
		{name: "nonadjacent ancestor forward", roots: `["Writing", "Concepts", "Writing/lessons"]`, wantErr: overlap},
		{name: "nonadjacent ancestor reverse", roots: `["Writing/lessons", "Concepts", "Writing"]`, wantErr: overlap},
		{name: "nonadjacent folded ancestor forward", roots: `["Writing", "Concepts", "writing/lessons"]`, wantErr: overlap},
		{name: "nonadjacent folded ancestor reverse", roots: `["writing/lessons", "Concepts", "Writing"]`, wantErr: overlap},
		{name: "unicode folded ancestor forward", roots: `["\u03a3", "\u03c2/studies"]`, wantErr: overlap},
		{name: "unicode folded ancestor reverse", roots: `["\u03c2/studies", "\u03a3"]`, wantErr: overlap},
		{name: "duplicate", roots: `["Writing", "Writing"]`, wantErr: `folders.domain_from: duplicate value "Writing"`},
		{name: "nested duplicate", roots: `["Writing/lessons", "Writing/lessons"]`, wantErr: `folders.domain_from: duplicate value "Writing/lessons"`},
		{name: "empty element", roots: `[""]`, wantErr: "folders.domain_from: empty value"},
		{name: "dot", roots: `["."]`, wantErr: unsafePath},
		{name: "parent", roots: `[".."]`, wantErr: unsafePath},
		{name: "nested dot", roots: `["Writing/./lessons"]`, wantErr: unsafePath},
		{name: "nested parent", roots: `["Writing/../lessons"]`, wantErr: unsafePath},
		{name: "trailing dot", roots: `["Writing/."]`, wantErr: unsafePath},
		{name: "trailing parent", roots: `["Writing/.."]`, wantErr: unsafePath},
		{name: "absolute", roots: `["/Writing"]`, wantErr: unsafePath},
		{name: "trailing slash", roots: `["Writing/"]`, wantErr: unsafePath},
		{name: "doubled slash", roots: `["Writing//lessons"]`, wantErr: unsafePath},
		{name: "backslash", roots: `["Writing\\lessons"]`, wantErr: unsafePath},
		{name: "nul", roots: `["Writing/\u0000lessons"]`, wantErr: unsafePath},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := replaceContractText(t, semanticallyValidContract,
				`domain_from = ["Writing"]`, `domain_from = `+tt.roots)
			got, err := loadContractBytes(t, []byte(data))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("LoadFile() error = nil, want %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("LoadFile() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadFile() error = %v, want accepted relative roots", err)
			}
			if diff := cmp.Diff(tt.want, got.Definition().Folders.DomainFrom); diff != "" {
				t.Errorf("loaded roots mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.want, got.Domains().Roots()); diff != "" {
				t.Errorf("Domains().Roots() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestDomainFolder catches guessed roots, positional or deepest-folder
// selection, filename selection, and normalization of authored path bytes.
func TestDomainFolder(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		roots []string
		path  string
		want  string
		found bool
	}{
		{name: "top level", roots: []string{"Concepts"}, path: "Concepts/golang/A.md", want: "golang", found: true},
		{name: "nested", roots: []string{"Writing/lessons"}, path: "Writing/lessons/golang/L1.md", want: "golang", found: true},
		{name: "deeper note", roots: []string{"Writing/lessons"}, path: "Writing/lessons/japanese/drills/D1.md", want: "japanese", found: true},
		{name: "renamed nested", roots: []string{"Archive/studies"}, path: "Archive/studies/rust/L1.md", want: "rust", found: true},
		{name: "later root", roots: []string{"Concepts", "Writing/lessons"}, path: "Writing/lessons/golang/L1.md", want: "golang", found: true},
		{name: "undeclared nested", roots: []string{"Concepts"}, path: "Writing/lessons/golang/L1.md"},
		{name: "empty roots", roots: []string{}, path: "Writing/lessons/golang/L1.md"},
		{name: "nil roots", path: "Concepts/golang/A.md"},
		{name: "top level direct child", roots: []string{"Concepts"}, path: "Concepts/Overview.md"},
		{name: "nested direct child", roots: []string{"Writing/lessons"}, path: "Writing/lessons/Overview.md"},
		{name: "sibling prefix", roots: []string{"Writing/lessons"}, path: "Writing/lessons-extra/golang/L1.md"},
		{name: "case distinct", roots: []string{"Writing/lessons"}, path: "writing/lessons/golang/L1.md"},
		{name: "normalization distinct", roots: []string{"Caf\u00e9/studies"}, path: "Cafe\u0301/studies/rust/L1.md"},
		{name: "normalization distinct reverse", roots: []string{"Cafe\u0301/studies"}, path: "Caf\u00e9/studies/rust/L1.md"},
		{name: "unicode match", roots: []string{"Caf\u00e9/studies"}, path: "Caf\u00e9/studies/\u65e5\u672c\u8a9e/L1.md", want: "\u65e5\u672c\u8a9e", found: true},
		{name: "domain bytes retained", roots: []string{"Concepts"}, path: "Concepts/Cafe\u0301/A.md", want: "Cafe\u0301", found: true},
		{name: "root whitespace retained", roots: []string{" Writing /lessons "}, path: " Writing /lessons /golang/L1.md", want: "golang", found: true},
		{name: "root whitespace distinct", roots: []string{"Writing/lessons "}, path: "Writing/lessons/golang/L1.md"},
		{name: "domain whitespace retained", roots: []string{"Concepts"}, path: "Concepts/ golang /A.md", want: " golang ", found: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, found := DomainFolder(tt.roots, tt.path)
			if got != tt.want || found != tt.found {
				t.Errorf("DomainFolder(%q, %q) = (%q, %t), want (%q, %t)", tt.roots, tt.path, got, found, tt.want, tt.found)
			}
		})
	}
}

func TestAnEnumTypeListedTwiceIsRefused(t *testing.T) {
	t.Parallel()
	text := strings.Replace(validContract, `type = ["note"]`, `type = ["note", "note"]`, 1)
	if text == validContract {
		t.Fatal("the contract fixture no longer lists enums.type as the test expects")
	}
	if _, err := decodeContract([]byte(text), policySource{}); err == nil || !strings.Contains(err.Error(), `enums.type: duplicate value "note"`) {
		t.Errorf("decodeContract() error = %v, want enums.type: duplicate value", err)
	}
}
