package schema

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

const semanticallyValidContract = `schema_version = "1"
aligned_with = "human-doctrine.md"
generated_at_must_match = true

[enums]
type = ["inbox", "lesson", "concept", "system"]
domain = ["golang"]
source_kind = ["book"]
source_provider = ["publisher"]
level = ["beginner"]
map_kind = ["topic"]

[enums.status]
note = ["draft", "archived"]
lesson = ["draft", "archived"]
system = ["active", "draft", "archived"]

[fields]
required = ["title", "type"]
required_inbox = ["title"]
domain_exempt_types = ["system"]
known = ["title", "type", "based_on", "source_locator", "related"]
lesson_only = ["slug", "evolution_predecessor", "evolution_successors"]

[fields.status_group]
lesson = ["lesson"]
system = ["system"]

[rules]
domain_equals_folder_under = ["Writing"]
concept_requires_provenance = ["based_on", "source_locator"]
slug_pattern = "^[a-z]+$"

[scan]
knowledge_dirs = ["Writing"]
skip_basenames = ["README.md"]

[navigation]
path_types = []
map_types = []

[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = []

[supersession]
predecessor_field = "evolution_predecessor"
successor_field = "evolution_successors"
general_link_field = "related"
archived_status = "archived"

[[lifecycle]]
status = "draft"
applies_to = ["*"]
from = []
owner = ["koopa"]

[[lifecycle]]
status = "archived"
applies_to = ["*"]
from = ["*"]
owner = []
`

func TestCoreSemanticsEnums(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		from    string
		to      string
		wantErr string
	}{
		{
			name:    "type empty element",
			from:    `type = ["inbox", "lesson", "concept", "system"]`,
			to:      `type = ["inbox", "", "concept", "system"]`,
			wantErr: "enums.type: empty value",
		},
		{
			name:    "domain duplicate",
			from:    `domain = ["golang"]`,
			to:      `domain = ["golang", "golang"]`,
			wantErr: `enums.domain: duplicate value "golang"`,
		},
		{
			name:    "source kind empty element",
			from:    `source_kind = ["book"]`,
			to:      `source_kind = [""]`,
			wantErr: "enums.source_kind: empty value",
		},
		{
			name:    "source provider duplicate",
			from:    `source_provider = ["publisher"]`,
			to:      `source_provider = ["publisher", "publisher"]`,
			wantErr: `enums.source_provider: duplicate value "publisher"`,
		},
		{
			name:    "level empty element",
			from:    `level = ["beginner"]`,
			to:      `level = [""]`,
			wantErr: "enums.level: empty value",
		},
		{
			name:    "map kind duplicate",
			from:    `map_kind = ["topic"]`,
			to:      `map_kind = ["topic", "topic"]`,
			wantErr: `enums.map_kind: duplicate value "topic"`,
		},
		{
			name:    "status empty element",
			from:    `note = ["draft", "archived"]`,
			to:      `note = ["draft", ""]`,
			wantErr: `enums.status."note": empty value`,
		},
		{
			name:    "status duplicate",
			from:    `note = ["draft", "archived"]`,
			to:      `note = ["draft", "draft"]`,
			wantErr: `enums.status."note": duplicate value "draft"`,
		},
		{
			name:    "empty status group name",
			from:    `system = ["active", "draft", "archived"]`,
			to:      `"" = ["active", "draft", "archived"]`,
			wantErr: "enums.status: empty group name",
		},
		{
			name: "status map errors are sorted",
			from: `[enums.status]
note = ["draft", "archived"]
lesson = ["draft", "archived"]
system = ["active", "draft", "archived"]`,
			to: `[enums.status]
zeta = ["duplicate", "duplicate"]
note = ["draft", "archived"]
lesson = ["draft", "archived"]
alpha = ["duplicate", "duplicate"]
system = ["active", "draft", "archived"]`,
			wantErr: `enums.status."alpha": duplicate value "duplicate"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data := replaceContractText(t, semanticallyValidContract, tt.from, tt.to)
			_, err := decodeContract([]byte(data), policySource{})
			if err == nil {
				t.Fatalf("decodeContract() error = nil, want substring %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("decodeContract() error = %q, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestCoreSemanticsFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		from    string
		to      string
		wantErr string
	}{
		{
			name:    "required empty element",
			from:    `required = ["title", "type"]`,
			to:      `required = ["title", ""]`,
			wantErr: "fields.required: empty value",
		},
		{
			name:    "required duplicate",
			from:    `required = ["title", "type"]`,
			to:      `required = ["title", "title"]`,
			wantErr: `fields.required: duplicate value "title"`,
		},
		{
			name:    "required inbox empty element",
			from:    `required_inbox = ["title"]`,
			to:      `required_inbox = [""]`,
			wantErr: "fields.required_inbox: empty value",
		},
		{
			name:    "required inbox duplicate",
			from:    `required_inbox = ["title"]`,
			to:      `required_inbox = ["title", "title"]`,
			wantErr: `fields.required_inbox: duplicate value "title"`,
		},
		{
			name:    "known empty element",
			from:    `known = ["title", "type", "based_on", "source_locator", "related"]`,
			to:      `known = ["title", "type", "based_on", "", "related"]`,
			wantErr: "fields.known: empty value",
		},
		{
			name:    "known duplicate",
			from:    `known = ["title", "type", "based_on", "source_locator", "related"]`,
			to:      `known = ["title", "type", "based_on", "based_on", "related"]`,
			wantErr: `fields.known: duplicate value "based_on"`,
		},
		{
			name:    "lesson only empty element",
			from:    `lesson_only = ["slug", "evolution_predecessor", "evolution_successors"]`,
			to:      `lesson_only = ["slug", "", "evolution_successors"]`,
			wantErr: "fields.lesson_only: empty value",
		},
		{
			name:    "lesson only duplicate",
			from:    `lesson_only = ["slug", "evolution_predecessor", "evolution_successors"]`,
			to:      `lesson_only = ["slug", "slug", "evolution_successors"]`,
			wantErr: `fields.lesson_only: duplicate value "slug"`,
		},
		{
			name:    "domain exempt empty element",
			from:    `domain_exempt_types = ["system"]`,
			to:      `domain_exempt_types = [""]`,
			wantErr: "fields.domain_exempt_types: empty value",
		},
		{
			name:    "domain exempt duplicate",
			from:    `domain_exempt_types = ["system"]`,
			to:      `domain_exempt_types = ["system", "system"]`,
			wantErr: `fields.domain_exempt_types: duplicate value "system"`,
		},
		{
			name:    "required outside known",
			from:    `required = ["title", "type"]`,
			to:      `required = ["title", "missing"]`,
			wantErr: `fields.required: value "missing" is not listed in fields.known`,
		},
		{
			name:    "required inbox outside known",
			from:    `required_inbox = ["title"]`,
			to:      `required_inbox = ["missing"]`,
			wantErr: `fields.required_inbox: value "missing" is not listed in fields.known`,
		},
		{
			name:    "known overlaps lesson only",
			from:    `known = ["title", "type", "based_on", "source_locator", "related"]`,
			to:      `known = ["title", "type", "based_on", "source_locator", "related", "slug"]`,
			wantErr: `fields.known and fields.lesson_only overlap at "slug"`,
		},
		{
			name:    "domain exempt outside type enum",
			from:    `domain_exempt_types = ["system"]`,
			to:      `domain_exempt_types = ["missing"]`,
			wantErr: `fields.domain_exempt_types: value "missing" is not listed in enums.type`,
		},
		{
			name:    "required inbox without inbox type",
			from:    `type = ["inbox", "lesson", "concept", "system"]`,
			to:      `type = ["lesson", "concept", "system"]`,
			wantErr: `fields.required_inbox requires enums.type to contain "inbox"`,
		},
		{
			name:    "lesson only without lesson type",
			from:    `type = ["inbox", "lesson", "concept", "system"]`,
			to:      `type = ["inbox", "concept", "system"]`,
			wantErr: `fields.lesson_only requires enums.type to contain "lesson"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data := replaceContractText(t, semanticallyValidContract, tt.from, tt.to)
			_, err := decodeContract([]byte(data), policySource{})
			if err == nil {
				t.Fatalf("decodeContract() error = nil, want substring %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("decodeContract() error = %q, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestCoreSemanticsStatusGroups(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		from    string
		to      string
		wantErr string
	}{
		{
			name:    "missing default note group",
			from:    "note = [\"draft\", \"archived\"]\n",
			to:      "",
			wantErr: `enums.status: missing required group "note"`,
		},
		{
			name:    "empty mapping group name",
			from:    `system = ["system"]`,
			to:      `"" = ["system"]`,
			wantErr: "fields.status_group: empty group name",
		},
		{
			name:    "mapping group is not a status enum",
			from:    `system = ["system"]`,
			to:      `other = ["system"]`,
			wantErr: `fields.status_group: unknown status group "other"`,
		},
		{
			name:    "mapped list has empty element",
			from:    `lesson = ["lesson"]`,
			to:      `lesson = [""]`,
			wantErr: `fields.status_group."lesson": empty value`,
		},
		{
			name:    "mapped list has duplicate",
			from:    `lesson = ["lesson"]`,
			to:      `lesson = ["lesson", "lesson"]`,
			wantErr: `fields.status_group."lesson": duplicate value "lesson"`,
		},
		{
			name:    "mapped type is unknown",
			from:    `lesson = ["lesson"]`,
			to:      `lesson = ["missing"]`,
			wantErr: `fields.status_group."lesson": value "missing" is not listed in enums.type`,
		},
		{
			name:    "type is assigned to two groups",
			from:    `system = ["system"]`,
			to:      `system = ["system", "lesson"]`,
			wantErr: `fields.status_group: type "lesson" is assigned to both "lesson" and "system"`,
		},
		{
			name: "map errors are sorted",
			from: `[fields.status_group]
lesson = ["lesson"]
system = ["system"]`,
			to: `[fields.status_group]
zeta = ["system"]
lesson = ["lesson"]
alpha = ["system"]
system = ["system"]`,
			wantErr: `fields.status_group: unknown status group "alpha"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data := replaceContractText(t, semanticallyValidContract, tt.from, tt.to)
			_, err := decodeContract([]byte(data), policySource{})
			if err == nil {
				t.Fatalf("decodeContract() error = nil, want substring %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("decodeContract() error = %q, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestCoreSemanticsRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		from    string
		to      string
		wantErr string
	}{
		{
			name:    "domain folder empty element",
			from:    `domain_equals_folder_under = ["Writing"]`,
			to:      `domain_equals_folder_under = [""]`,
			wantErr: "rules.domain_equals_folder_under: empty value",
		},
		{
			name:    "domain folder duplicate",
			from:    `domain_equals_folder_under = ["Writing"]`,
			to:      `domain_equals_folder_under = ["Writing", "Writing"]`,
			wantErr: `rules.domain_equals_folder_under: duplicate value "Writing"`,
		},
		{
			name:    "domain folder has backslash",
			from:    `domain_equals_folder_under = ["Writing"]`,
			to:      `domain_equals_folder_under = ["Writing\\notes"]`,
			wantErr: `rules.domain_equals_folder_under: unsafe relative directory path "Writing\\notes"`,
		},
		{
			name:    "domain folder is current directory",
			from:    `domain_equals_folder_under = ["Writing"]`,
			to:      `domain_equals_folder_under = ["."]`,
			wantErr: `rules.domain_equals_folder_under: unsafe relative directory path "."`,
		},
		{
			name:    "domain folder is parent directory",
			from:    `domain_equals_folder_under = ["Writing"]`,
			to:      `domain_equals_folder_under = [".."]`,
			wantErr: `rules.domain_equals_folder_under: unsafe relative directory path ".."`,
		},
		{
			name:    "domain folder is absolute",
			from:    `domain_equals_folder_under = ["Writing"]`,
			to:      `domain_equals_folder_under = ["/Writing"]`,
			wantErr: `rules.domain_equals_folder_under: unsafe relative directory path "/Writing"`,
		},
		{
			name:    "domain folder contains NUL",
			from:    `domain_equals_folder_under = ["Writing"]`,
			to:      `domain_equals_folder_under = ["\u0000"]`,
			wantErr: `rules.domain_equals_folder_under: unsafe relative directory path "\x00"`,
		},
		{
			name:    "concept provenance empty element",
			from:    `concept_requires_provenance = ["based_on", "source_locator"]`,
			to:      `concept_requires_provenance = ["based_on", ""]`,
			wantErr: "rules.concept_requires_provenance: empty value",
		},
		{
			name:    "concept provenance duplicate",
			from:    `concept_requires_provenance = ["based_on", "source_locator"]`,
			to:      `concept_requires_provenance = ["based_on", "based_on"]`,
			wantErr: `rules.concept_requires_provenance: duplicate value "based_on"`,
		},
		{
			name:    "concept provenance unknown field",
			from:    `concept_requires_provenance = ["based_on", "source_locator"]`,
			to:      `concept_requires_provenance = ["based_on", "missing"]`,
			wantErr: `rules.concept_requires_provenance: value "missing" is not listed in fields.known`,
		},
		{
			name:    "concept provenance lesson field",
			from:    `concept_requires_provenance = ["based_on", "source_locator"]`,
			to:      `concept_requires_provenance = ["based_on", "slug"]`,
			wantErr: `rules.concept_requires_provenance: value "slug" is listed in fields.lesson_only`,
		},
		{
			name:    "concept type without provenance",
			from:    `concept_requires_provenance = ["based_on", "source_locator"]`,
			to:      `concept_requires_provenance = []`,
			wantErr: `rules.concept_requires_provenance must not be empty when enums.type contains "concept"`,
		},
		{
			name:    "lesson type without slug pattern",
			from:    `slug_pattern = "^[a-z]+$"`,
			to:      `slug_pattern = ""`,
			wantErr: `rules.slug_pattern must not be empty when enums.type contains "lesson"`,
		},
		{
			name:    "lesson type with invalid slug pattern",
			from:    `slug_pattern = "^[a-z]+$"`,
			to:      `slug_pattern = "["`,
			wantErr: "rules.slug_pattern: invalid regular expression",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data := replaceContractText(t, semanticallyValidContract, tt.from, tt.to)
			_, err := decodeContract([]byte(data), policySource{})
			if err == nil {
				t.Fatalf("decodeContract() error = nil, want substring %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("decodeContract() error = %q, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestCoreSemanticsScan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		from    string
		to      string
		wantErr string
	}{
		{
			name:    "knowledge directory empty element",
			from:    `knowledge_dirs = ["Writing"]`,
			to:      `knowledge_dirs = [""]`,
			wantErr: "scan.knowledge_dirs: empty value",
		},
		{
			name:    "knowledge directory duplicate",
			from:    `knowledge_dirs = ["Writing"]`,
			to:      `knowledge_dirs = ["Writing", "Writing"]`,
			wantErr: `scan.knowledge_dirs: duplicate value "Writing"`,
		},
		{
			name:    "knowledge directory has slash",
			from:    `knowledge_dirs = ["Writing"]`,
			to:      `knowledge_dirs = ["Writing/notes"]`,
			wantErr: `scan.knowledge_dirs: unsafe top-level component "Writing/notes"`,
		},
		{
			name:    "knowledge directory has backslash",
			from:    `knowledge_dirs = ["Writing"]`,
			to:      `knowledge_dirs = ["Writing\\notes"]`,
			wantErr: `scan.knowledge_dirs: unsafe top-level component "Writing\\notes"`,
		},
		{
			name:    "knowledge directory is current directory",
			from:    `knowledge_dirs = ["Writing"]`,
			to:      `knowledge_dirs = ["."]`,
			wantErr: `scan.knowledge_dirs: unsafe top-level component "."`,
		},
		{
			name:    "knowledge directory is parent directory",
			from:    `knowledge_dirs = ["Writing"]`,
			to:      `knowledge_dirs = [".."]`,
			wantErr: `scan.knowledge_dirs: unsafe top-level component ".."`,
		},
		{
			name:    "knowledge directory is absolute",
			from:    `knowledge_dirs = ["Writing"]`,
			to:      `knowledge_dirs = ["/Writing"]`,
			wantErr: `scan.knowledge_dirs: unsafe top-level component "/Writing"`,
		},
		{
			name:    "knowledge directory contains NUL",
			from:    `knowledge_dirs = ["Writing"]`,
			to:      `knowledge_dirs = ["\u0000"]`,
			wantErr: `scan.knowledge_dirs: unsafe top-level component "\x00"`,
		},
		{
			name:    "skip basename empty element",
			from:    `skip_basenames = ["README.md"]`,
			to:      `skip_basenames = [""]`,
			wantErr: "scan.skip_basenames: empty value",
		},
		{
			name:    "skip basename duplicate",
			from:    `skip_basenames = ["README.md"]`,
			to:      `skip_basenames = ["README.md", "README.md"]`,
			wantErr: `scan.skip_basenames: duplicate value "README.md"`,
		},
		{
			name:    "skip basename has slash",
			from:    `skip_basenames = ["README.md"]`,
			to:      `skip_basenames = ["System/README.md"]`,
			wantErr: `scan.skip_basenames: invalid basename "System/README.md"`,
		},
		{
			name:    "skip basename has backslash",
			from:    `skip_basenames = ["README.md"]`,
			to:      `skip_basenames = ["System\\README.md"]`,
			wantErr: `scan.skip_basenames: invalid basename "System\\README.md"`,
		},
		{
			name:    "skip basename is current directory",
			from:    `skip_basenames = ["README.md"]`,
			to:      `skip_basenames = ["."]`,
			wantErr: `scan.skip_basenames: invalid basename "."`,
		},
		{
			name:    "skip basename is parent directory",
			from:    `skip_basenames = ["README.md"]`,
			to:      `skip_basenames = [".."]`,
			wantErr: `scan.skip_basenames: invalid basename ".."`,
		},
		{
			name:    "skip basename contains NUL",
			from:    `skip_basenames = ["README.md"]`,
			to:      `skip_basenames = ["\u0000"]`,
			wantErr: `scan.skip_basenames: invalid basename "\x00"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data := replaceContractText(t, semanticallyValidContract, tt.from, tt.to)
			_, err := decodeContract([]byte(data), policySource{})
			if err == nil {
				t.Fatalf("decodeContract() error = nil, want substring %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("decodeContract() error = %q, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestCoreSemanticsSupersession(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		from    string
		to      string
		wantErr string
	}{
		{
			name:    "missing section is legal",
			from:    supersessionContractSection,
			to:      "",
			wantErr: "",
		},
		{
			name:    "empty predecessor field",
			from:    `predecessor_field = "evolution_predecessor"`,
			to:      `predecessor_field = ""`,
			wantErr: `supersession.predecessor_field must not be empty`,
		},
		{
			name:    "empty successor field",
			from:    `successor_field = "evolution_successors"`,
			to:      `successor_field = ""`,
			wantErr: `supersession.successor_field must not be empty`,
		},
		{
			name:    "empty general link field",
			from:    `general_link_field = "related"`,
			to:      `general_link_field = ""`,
			wantErr: `supersession.general_link_field must not be empty`,
		},
		{
			name:    "empty archived status",
			from:    `archived_status = "archived"`,
			to:      `archived_status = ""`,
			wantErr: `supersession.archived_status must not be empty`,
		},
		{
			name:    "predecessor is not lesson only",
			from:    `predecessor_field = "evolution_predecessor"`,
			to:      `predecessor_field = "title"`,
			wantErr: `supersession.predecessor_field: value "title" is not listed in fields.lesson_only`,
		},
		{
			name:    "successor is not lesson only",
			from:    `successor_field = "evolution_successors"`,
			to:      `successor_field = "title"`,
			wantErr: `supersession.successor_field: value "title" is not listed in fields.lesson_only`,
		},
		{
			name:    "general link is not known",
			from:    `general_link_field = "related"`,
			to:      `general_link_field = "missing"`,
			wantErr: `supersession.general_link_field: value "missing" is not listed in fields.known`,
		},
		{
			name:    "predecessor equals successor",
			from:    `successor_field = "evolution_successors"`,
			to:      `successor_field = "evolution_predecessor"`,
			wantErr: `supersession fields "predecessor_field" and "successor_field" must be distinct`,
		},
		{
			name:    "predecessor equals general link",
			from:    `general_link_field = "related"`,
			to:      `general_link_field = "evolution_predecessor"`,
			wantErr: `supersession fields "predecessor_field" and "general_link_field" must be distinct`,
		},
		{
			name:    "successor equals general link",
			from:    `general_link_field = "related"`,
			to:      `general_link_field = "evolution_successors"`,
			wantErr: `supersession fields "successor_field" and "general_link_field" must be distinct`,
		},
		{
			name:    "archive token is not legal for every type",
			from:    `archived_status = "archived"`,
			to:      `archived_status = "active"`,
			wantErr: `supersession.archived_status: value "active" is not legal for type "inbox"`,
		},
		{
			name:    "archive token is unknown",
			from:    `archived_status = "archived"`,
			to:      `archived_status = "missing"`,
			wantErr: `supersession.archived_status: value "missing" is not listed in enums.status`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data := replaceContractText(t, semanticallyValidContract, tt.from, tt.to)
			_, err := decodeContract([]byte(data), policySource{})
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("decodeContract() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("decodeContract() error = nil, want substring %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("decodeContract() error = %q, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestCoreSemanticsGreenControls(t *testing.T) {
	t.Parallel()

	t.Run("cross status group reuse and optional enums", func(t *testing.T) {
		t.Parallel()

		data := semanticallyValidContract
		for _, replacement := range []struct {
			from string
			to   string
		}{
			{from: `domain = ["golang"]`, to: `domain = []`},
			{from: `source_kind = ["book"]`, to: `source_kind = []`},
			{from: `source_provider = ["publisher"]`, to: `source_provider = []`},
			{from: `level = ["beginner"]`, to: `level = []`},
			{from: `map_kind = ["topic"]`, to: `map_kind = []`},
		} {
			data = replaceContractText(t, data, replacement.from, replacement.to)
		}
		if _, err := decodeContract([]byte(data), policySource{}); err != nil {
			t.Fatalf("decodeContract(empty optional enums and repeated status token) error = %v", err)
		}
	})

	t.Run("empty optional rule and scan lists without governed types", func(t *testing.T) {
		t.Parallel()

		data := semanticallyValidContract
		for _, replacement := range []struct {
			from string
			to   string
		}{
			{
				from: `type = ["inbox", "lesson", "concept", "system"]`,
				to:   `type = ["inbox", "system"]`,
			},
			{
				from: `lesson_only = ["slug", "evolution_predecessor", "evolution_successors"]`,
				to:   `lesson_only = []`,
			},
			{from: "lesson = [\"lesson\"]\n", to: ""},
			{
				from: `concept_requires_provenance = ["based_on", "source_locator"]`,
				to:   `concept_requires_provenance = []`,
			},
			{from: `slug_pattern = "^[a-z]+$"`, to: `slug_pattern = ""`},
			{from: `domain_equals_folder_under = ["Writing"]`, to: `domain_equals_folder_under = []`},
			{from: `knowledge_dirs = ["Writing"]`, to: `knowledge_dirs = []`},
			{from: `skip_basenames = ["README.md"]`, to: `skip_basenames = []`},
			{from: supersessionContractSection, to: ""},
		} {
			data = replaceContractText(t, data, replacement.from, replacement.to)
		}
		if _, err := decodeContract([]byte(data), policySource{}); err != nil {
			t.Fatalf("decodeContract(empty optional core lists) error = %v", err)
		}
	})

	t.Run("absent capabilities remain degraded", func(t *testing.T) {
		t.Parallel()

		data := semanticallyValidContract
		for _, section := range []string{
			"[navigation]\npath_types = []\nmap_types = []\n\n",
			"[artifacts]\nnon_instance_dirs = []\n\n",
			"[privacy]\nnever_egress_dirs = []\n\n",
		} {
			data = replaceContractText(t, data, section, "")
		}
		s, err := decodeContract([]byte(data), policySource{})
		if err != nil {
			t.Fatalf("decodeContract(absent capabilities) error = %v", err)
		}
		if s.NavigationRoles().Available() {
			t.Error("NavigationRoles().Available() = true, want false")
		}
		if s.ArtifactPolicy().Available() {
			t.Error("ArtifactPolicy().Available() = true, want false")
		}
		if s.PrivacyPolicy().Available() {
			t.Error("PrivacyPolicy().Available() = true, want false")
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

	t.Run("coordination metadata remains decode only", func(t *testing.T) {
		t.Parallel()

		data := replaceContractText(
			t,
			semanticallyValidContract,
			`aligned_with = "human-doctrine.md"`,
			`aligned_with = ""`,
		)
		data = replaceContractText(t, data, "generated_at_must_match = true", "generated_at_must_match = false")
		if _, err := decodeContract([]byte(data), policySource{}); err != nil {
			t.Fatalf("decodeContract(decode-only coordination metadata) error = %v", err)
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
			err:  errors.New(`fields.status_group: unknown status group "alpha"`),
			want: `fields.status_group: unknown status group "alpha"`,
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

const supersessionContractSection = `[supersession]
predecessor_field = "evolution_predecessor"
successor_field = "evolution_successors"
general_link_field = "related"
archived_status = "archived"

`

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
		overlap    = "rules.domain_equals_folder_under: overlapping roots"
		unsafePath = "rules.domain_equals_folder_under: unsafe relative directory path"
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
		{name: "duplicate", roots: `["Writing", "Writing"]`, wantErr: `rules.domain_equals_folder_under: duplicate value "Writing"`},
		{name: "nested duplicate", roots: `["Writing/lessons", "Writing/lessons"]`, wantErr: `rules.domain_equals_folder_under: duplicate value "Writing/lessons"`},
		{name: "empty element", roots: `[""]`, wantErr: "rules.domain_equals_folder_under: empty value"},
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
				`domain_equals_folder_under = ["Writing"]`, `domain_equals_folder_under = `+tt.roots)
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
			if diff := cmp.Diff(tt.want, got.Definition().Rules.DomainEqualsFolderUnder); diff != "" {
				t.Errorf("loaded roots mismatch (-want +got):\n%s", diff)
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
