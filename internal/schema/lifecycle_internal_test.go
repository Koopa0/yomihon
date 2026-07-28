package schema

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

const lifecycleContract = `schema_version = "1"

[enums]
type = ["article", "lesson", "system"]

[enums.status]
note = ["draft", "review", "archived"]
lesson = ["draft", "ready", "archived"]
system = ["active", "archived"]

[fields]
known = ["related"]
lesson_only = ["predecessor", "successors"]

[fields.status_group]
lesson = ["lesson"]
system = ["system"]

[rules]
slug_pattern = "^[a-z]+$"

[[lifecycle]]
status = "draft"
applies_to = ["article", "lesson"]
from = []
owner = ["editor"]

[[lifecycle]]
status = "review"
applies_to = ["article"]
from = ["draft"]
owner = ["editor"]

[[lifecycle]]
status = "ready"
applies_to = ["lesson"]
from = ["draft"]
owner = ["koopa"]

[[lifecycle]]
status = "active"
applies_to = ["system"]
from = []
owner = ["system"]

[[lifecycle]]
status = "archived"
applies_to = ["*"]
from = ["*"]
owner = []
`

const lifecycleSupersessionSection = `
[supersession]
predecessor_field = "predecessor"
successor_field = "successors"
general_link_field = "related"
archived_status = "archived"
`

func TestSupersessionAccessor(t *testing.T) {
	t.Parallel()

	contract := decodeLifecycleFixture(t, lifecycleContract+lifecycleSupersessionSection)
	got, ok := contract.Supersession()
	if !ok {
		t.Fatal("Supersession() = false, want true")
	}
	want := Supersession{
		PredecessorField: "predecessor",
		SuccessorField:   "successors",
		GeneralLinkField: "related",
		ArchivedStatus:   "archived",
	}
	if got != want {
		t.Errorf("Supersession() = %+v, want %+v", got, want)
	}

	var zero Contract
	if got, ok := zero.Supersession(); ok || got != (Supersession{}) {
		t.Errorf("zero Contract Supersession() = (%+v, %t), want zero, false", got, ok)
	}
}

func TestTOMLLifecycleSlicePresence(t *testing.T) {
	t.Parallel()

	type row struct {
		From  *[]string `toml:"from"`
		Owner *[]string `toml:"owner"`
	}
	var decoded struct {
		Lifecycle []row `toml:"lifecycle"`
	}
	const input = `[[lifecycle]]
from = []
`
	if _, err := toml.Decode(input, &decoded); err != nil {
		t.Fatalf("toml.Decode() error = %v", err)
	}
	if got := len(decoded.Lifecycle); got != 1 {
		t.Fatalf("decoded lifecycle rows = %d, want 1", got)
	}
	if decoded.Lifecycle[0].From == nil {
		t.Fatal("explicit from = [] decoded as nil, want present empty slice")
	}
	if got := len(*decoded.Lifecycle[0].From); got != 0 {
		t.Errorf("decoded from length = %d, want 0", got)
	}
	if decoded.Lifecycle[0].Owner != nil {
		t.Errorf("omitted owner decoded as %v, want nil", *decoded.Lifecycle[0].Owner)
	}
}

func TestLifecycleRequiresExplicitKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		from    string
		wantErr string
	}{
		{
			name:    "status",
			from:    "status = \"draft\"\n",
			wantErr: `lifecycle row 1: missing required key "status"`,
		},
		{
			name:    "applies to",
			from:    "applies_to = [\"article\", \"lesson\"]\n",
			wantErr: `lifecycle row 1: missing required key "applies_to"`,
		},
		{
			name:    "from",
			from:    "from = []\n",
			wantErr: `lifecycle row 1: missing required key "from"`,
		},
		{
			name:    "owner",
			from:    "owner = [\"editor\"]\n",
			wantErr: `lifecycle row 1: missing required key "owner"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data := replaceLifecycleRowText(t, lifecycleContract, 1, tt.from, "")
			assertContractError(t, data, tt.wantErr)
		})
	}
}

func TestLifecycleExplicitEmptyLists(t *testing.T) {
	t.Parallel()

	s := decodeLifecycleFixture(t, lifecycleContract)
	draft, ok := s.Stage("article", "draft")
	if !ok {
		t.Fatal(`Stage("article", "draft") = false, want true`)
	}
	if draft.From == nil || len(draft.From) != 0 {
		t.Errorf("draft From = %#v, want explicit empty slice", draft.From)
	}
	archived, ok := s.Stage("article", "archived")
	if !ok {
		t.Fatal(`Stage("article", "archived") = false, want true`)
	}
	if archived.Owner == nil || len(archived.Owner) != 0 {
		t.Errorf("archived Owner = %#v, want explicit empty slice", archived.Owner)
	}
	if err := s.Transition("article", "review", "archived", "editor"); !errors.Is(err, ErrOwnerForbidden) {
		t.Errorf("Transition(article, review, archived, editor) = %v, want %v", err, ErrOwnerForbidden)
	}
}

func TestLifecycleRowValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		row     int
		from    string
		to      string
		wantErr string
	}{
		{
			name:    "blank status",
			row:     1,
			from:    `status = "draft"`,
			to:      `status = " "`,
			wantErr: "lifecycle row 1: status must not be blank",
		},
		{
			name:    "empty applies to",
			row:     1,
			from:    `applies_to = ["article", "lesson"]`,
			to:      `applies_to = []`,
			wantErr: "lifecycle row 1: applies_to must not be empty",
		},
		{
			name:    "blank applies to",
			row:     1,
			from:    `applies_to = ["article", "lesson"]`,
			to:      `applies_to = ["article", " "]`,
			wantErr: "lifecycle row 1 applies_to: blank value",
		},
		{
			name:    "duplicate applies to",
			row:     1,
			from:    `applies_to = ["article", "lesson"]`,
			to:      `applies_to = ["article", "article"]`,
			wantErr: `lifecycle row 1 applies_to: duplicate value "article"`,
		},
		{
			name:    "mixed applies to wildcard",
			row:     1,
			from:    `applies_to = ["article", "lesson"]`,
			to:      `applies_to = ["*", "article"]`,
			wantErr: `lifecycle row 1 applies_to: wildcard "*" must be the only value`,
		},
		{
			name:    "unknown applies to type",
			row:     1,
			from:    `applies_to = ["article", "lesson"]`,
			to:      `applies_to = ["article", "missing"]`,
			wantErr: `lifecycle row 1 applies_to: type "missing" is not listed in enums.type`,
		},
		{
			name:    "blank predecessor",
			row:     2,
			from:    `from = ["draft"]`,
			to:      `from = [" "]`,
			wantErr: "lifecycle row 2 from: blank value",
		},
		{
			name:    "duplicate predecessor",
			row:     2,
			from:    `from = ["draft"]`,
			to:      `from = ["draft", "draft"]`,
			wantErr: `lifecycle row 2 from: duplicate value "draft"`,
		},
		{
			name:    "mixed predecessor wildcard",
			row:     2,
			from:    `from = ["draft"]`,
			to:      `from = ["*", "draft"]`,
			wantErr: `lifecycle row 2 from: wildcard "*" must be the only value`,
		},
		{
			name:    "unknown predecessor",
			row:     2,
			from:    `from = ["draft"]`,
			to:      `from = ["missing"]`,
			wantErr: `lifecycle row 2 from: status "missing" is not legal for type "article"`,
		},
		{
			name:    "blank owner",
			row:     1,
			from:    `owner = ["editor"]`,
			to:      `owner = [" "]`,
			wantErr: "lifecycle row 1 owner: blank value",
		},
		{
			name:    "duplicate owner",
			row:     1,
			from:    `owner = ["editor"]`,
			to:      `owner = ["editor", "editor"]`,
			wantErr: `lifecycle row 1 owner: duplicate value "editor"`,
		},
		{
			name:    "predecessor invalid for one applicable type",
			row:     5,
			from:    "applies_to = [\"*\"]\nfrom = [\"*\"]",
			to:      "applies_to = [\"article\", \"lesson\"]\nfrom = [\"ready\"]",
			wantErr: `lifecycle row 5 from: status "ready" is not legal for type "article"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data := replaceLifecycleRowText(t, lifecycleContract, tt.row, tt.from, tt.to)
			assertContractError(t, data, tt.wantErr)
		})
	}

	t.Run("unknown actor is legal", func(t *testing.T) {
		t.Parallel()

		data := replaceLifecycleRowText(
			t,
			lifecycleContract,
			1,
			`owner = ["editor"]`,
			`owner = ["unregistered-agent"]`,
		)
		if _, err := decodeContract([]byte(data), policySource{}); err != nil {
			t.Fatalf("decodeContract(unknown nonblank owner) error = %v", err)
		}
	})
}

func TestLifecycleTargetMustBeLegalForEveryApplicableType(t *testing.T) {
	t.Parallel()

	const readyRow = `[[lifecycle]]
status = "ready"
applies_to = ["lesson"]
from = ["draft"]
owner = ["koopa"]

`
	data := replaceContractText(t, lifecycleContract, readyRow, "")
	data = replaceLifecycleRowText(t, data, 1, `status = "draft"`, `status = "ready"`)
	assertContractError(
		t,
		data,
		`lifecycle row 1: status "ready" is not legal for type "article"`,
	)
}

func TestLifecycleRowsMayPartitionAStatus(t *testing.T) {
	t.Parallel()

	t.Run("overlap rejected", func(t *testing.T) {
		t.Parallel()

		data := lifecycleContract + `
[[lifecycle]]
status = "draft"
applies_to = ["lesson"]
from = []
owner = ["koopa"]
`
		assertContractError(
			t,
			data,
			`lifecycle rows 1 and 6 overlap for type "lesson" and status "draft"`,
		)
	})

	t.Run("disjoint rows accepted", func(t *testing.T) {
		t.Parallel()

		data := replaceContractText(
			t,
			lifecycleContract,
			`applies_to = ["article", "lesson"]`,
			`applies_to = ["article"]`,
		)
		data += `
[[lifecycle]]
status = "draft"
applies_to = ["lesson"]
from = []
owner = ["koopa"]
`
		if _, err := decodeContract([]byte(data), policySource{}); err != nil {
			t.Fatalf("decodeContract(disjoint same-status rows) error = %v", err)
		}
	})
}

func TestLifecycleAllowsEnumStatusWithoutAuthorityRow(t *testing.T) {
	t.Parallel()

	data := replaceContractText(
		t,
		lifecycleContract,
		`note = ["draft", "review", "archived"]`,
		`note = ["draft", "review", "parked", "archived"]`,
	)
	if _, err := decodeContract([]byte(data), policySource{}); err != nil {
		t.Fatalf("decodeContract(enum status without lifecycle row) error = %v", err)
	}
}

func TestLifecycleRejectsUnknownRuntimeType(t *testing.T) {
	t.Parallel()

	s := decodeLifecycleFixture(t, lifecycleContract)
	if got := s.StatusGroup(""); got != "note" {
		t.Errorf(`StatusGroup("") = %q, want default group "note"`, got)
	}
	if got := s.StatusGroup("undeclared"); got != "" {
		t.Errorf(`StatusGroup("undeclared") = %q, want unavailable empty result`, got)
	}
	if got := s.Statuses("undeclared"); got != nil {
		t.Errorf(`Statuses("undeclared") = %v, want nil`, got)
	}
	if _, ok := s.Stage("undeclared", "archived"); ok {
		t.Error(`Stage("undeclared", "archived") = true, want false`)
	}
	if s.AdvanceableBy("undeclared", "draft", "editor") {
		t.Error(`AdvanceableBy("undeclared", "draft", "editor") = true, want false`)
	}
	if err := s.Transition("undeclared", "draft", "archived", "editor"); !errors.Is(err, ErrUnknownStatus) {
		t.Errorf("Transition(undeclared, draft, archived, editor) = %v, want %v", err, ErrUnknownStatus)
	}
}

func TestLifecycleWildcardPredecessorIncludesInitialAndDeclaredStates(t *testing.T) {
	t.Parallel()

	data := replaceLifecycleRowText(
		t,
		lifecycleContract,
		5,
		`owner = []`,
		`owner = ["editor"]`,
	)
	s := decodeLifecycleFixture(t, data)
	if err := s.Transition("article", "", "archived", "editor"); err != nil {
		t.Errorf("Transition(article, initial, archived, editor) error = %v", err)
	}
	if err := s.Transition("article", "review", "archived", "editor"); err != nil {
		t.Errorf("Transition(article, review, archived, editor) error = %v", err)
	}
	if err := s.Transition("article", "missing", "archived", "editor"); !errors.Is(err, ErrUnknownStatus) {
		t.Errorf("Transition(article, unknown, archived, editor) = %v, want %v", err, ErrUnknownStatus)
	}
	if err := s.Transition("article", "archived", "archived", "editor"); !errors.Is(err, ErrIllegalTransition) {
		t.Errorf("Transition(article, archived, archived, editor) = %v, want %v", err, ErrIllegalTransition)
	}
}

func TestLifecycleLookupsAreImmutable(t *testing.T) {
	t.Parallel()

	s := decodeLifecycleFixture(t, lifecycleContract)

	statuses := s.Statuses("article")
	if want := []string{"draft", "review", "archived"}; !slices.Equal(statuses, want) {
		t.Fatalf("Statuses(article) = %v, want %v", statuses, want)
	}
	statuses[0] = "changed"

	stage, ok := s.Stage("article", "review")
	if !ok {
		t.Fatal("Stage(article, review) = false, want true")
	}
	stage.From[0] = "changed"
	stage.Owner[0] = "changed"

	definition := s.Definition()
	definition.Enums.Type[0] = "changed"
	definition.Enums.Status["note"][0] = "changed"
	definition.Fields.StatusGroup["lesson"][0] = "changed"

	if got := s.StatusGroup("lesson"); got != "lesson" {
		t.Errorf("StatusGroup(lesson) after detached mutation = %q, want %q", got, "lesson")
	}
	if got, want := s.Statuses("article"), []string{"draft", "review", "archived"}; !slices.Equal(got, want) {
		t.Errorf("Statuses(article) after detached mutation = %v, want %v", got, want)
	}
	gotStage, ok := s.Stage("article", "review")
	if !ok {
		t.Fatal("Stage(article, review) after mutation = false, want true")
	}
	if got, want := gotStage.From, []string{"draft"}; !slices.Equal(got, want) {
		t.Errorf("Stage(article, review).From after mutation = %v, want %v", got, want)
	}
	if got, want := gotStage.Owner, []string{"editor"}; !slices.Equal(got, want) {
		t.Errorf("Stage(article, review).Owner after mutation = %v, want %v", got, want)
	}
	if err := s.Transition("article", "draft", "review", "editor"); err != nil {
		t.Errorf("Transition(article, draft, review, editor) after mutation error = %v", err)
	}
	if !s.AdvanceableBy("article", "draft", "editor") {
		t.Error("AdvanceableBy(article, draft, editor) after mutation = false, want true")
	}
}

func TestLifecycleLookupResultsAreDefensiveCopies(t *testing.T) {
	t.Parallel()

	s := decodeLifecycleFixture(t, lifecycleContract)
	statuses := s.Statuses("article")
	statuses[0] = "changed"
	if got, want := s.Statuses("article"), []string{"draft", "review", "archived"}; !slices.Equal(got, want) {
		t.Errorf("Statuses(article) after return mutation = %v, want %v", got, want)
	}

	stage, ok := s.Stage("article", "review")
	if !ok {
		t.Fatal("Stage(article, review) = false, want true")
	}
	stage.AppliesTo[0] = "changed"
	stage.From[0] = "changed"
	stage.Owner[0] = "changed"
	got, ok := s.Stage("article", "review")
	if !ok {
		t.Fatal("Stage(article, review) after return mutation = false, want true")
	}
	if want := []string{"article"}; !slices.Equal(got.AppliesTo, want) {
		t.Errorf("Stage(article, review).AppliesTo after return mutation = %v, want %v", got.AppliesTo, want)
	}
	if want := []string{"draft"}; !slices.Equal(got.From, want) {
		t.Errorf("Stage(article, review).From after return mutation = %v, want %v", got.From, want)
	}
	if want := []string{"editor"}; !slices.Equal(got.Owner, want) {
		t.Errorf("Stage(article, review).Owner after return mutation = %v, want %v", got.Owner, want)
	}
}

func TestLifecycleRawDecodeRemainsStrict(t *testing.T) {
	t.Parallel()

	t.Run("unknown row key", func(t *testing.T) {
		t.Parallel()

		data := replaceLifecycleRowText(
			t,
			lifecycleContract,
			1,
			`owner = ["editor"]`,
			"owner = [\"editor\"]\nunknown_inside_row = true",
		)
		assertContractError(t, data, `unknown core keys: "lifecycle.unknown_inside_row"`)
	})

	t.Run("wrong row field type", func(t *testing.T) {
		t.Parallel()

		data := replaceLifecycleRowText(
			t,
			lifecycleContract,
			1,
			`status = "draft"`,
			`status = ["draft"]`,
		)
		if _, err := decodeContract([]byte(data), policySource{}); err == nil {
			t.Fatal("decodeContract(wrong lifecycle field type) error = nil, want hard error")
		}
	})
}

func TestSupersessionLifecycleIntegrity(t *testing.T) {
	t.Parallel()

	t.Run("wildcard archive is complete", func(t *testing.T) {
		t.Parallel()

		if _, err := decodeContract([]byte(lifecycleContract+lifecycleSupersessionSection), policySource{}); err != nil {
			t.Fatalf("decodeContract(complete wildcard archive) error = %v", err)
		}
	})

	t.Run("archive status absent from one effective group", func(t *testing.T) {
		t.Parallel()

		data := replaceContractText(
			t,
			lifecycleContract,
			`system = ["active", "archived"]`,
			`system = ["active"]`,
		)
		data = replaceContractText(t, data, `applies_to = ["*"]`, `applies_to = ["article", "lesson"]`)
		assertContractError(
			t,
			data+lifecycleSupersessionSection,
			`supersession.archived_status: value "archived" is not legal for type "system"`,
		)
	})

	t.Run("archive stage absent for one type", func(t *testing.T) {
		t.Parallel()

		data := replaceContractText(
			t,
			lifecycleContract,
			`applies_to = ["*"]`,
			`applies_to = ["article", "lesson"]`,
		)
		assertContractError(
			t,
			data+lifecycleSupersessionSection,
			`supersession.archived_status: no lifecycle row applies to type "system" and status "archived"`,
		)
	})

	t.Run("overlapping archive rows rejected", func(t *testing.T) {
		t.Parallel()

		data := lifecycleContract + `
[[lifecycle]]
status = "archived"
applies_to = ["lesson"]
from = ["*"]
owner = []
` + lifecycleSupersessionSection
		assertContractError(
			t,
			data,
			`lifecycle rows 5 and 6 overlap for type "lesson" and status "archived"`,
		)
	})

	t.Run("incomplete explicit predecessor coverage", func(t *testing.T) {
		t.Parallel()

		data := partitionedArchiveContract(t, `from = ["draft"]`)
		assertContractError(
			t,
			data+lifecycleSupersessionSection,
			`supersession archived stage for type "article" does not accept predecessor "review"`,
		)
	})

	t.Run("partitioned explicit coverage is complete", func(t *testing.T) {
		t.Parallel()

		data := partitionedArchiveContract(t, `from = ["draft", "review"]`)
		if _, err := decodeContract([]byte(data+lifecycleSupersessionSection), policySource{}); err != nil {
			t.Fatalf("decodeContract(partitioned complete archive) error = %v", err)
		}
	})
}

func partitionedArchiveContract(t *testing.T, articleFrom string) string {
	t.Helper()

	const wildcardArchive = `[[lifecycle]]
status = "archived"
applies_to = ["*"]
from = ["*"]
owner = []
`
	replacement := `[[lifecycle]]
status = "archived"
applies_to = ["article"]
` + articleFrom + `
owner = []

[[lifecycle]]
status = "archived"
applies_to = ["lesson"]
from = ["draft", "ready"]
owner = []

[[lifecycle]]
status = "archived"
applies_to = ["system"]
from = ["active"]
owner = []
`
	return replaceContractText(t, lifecycleContract, wildcardArchive, replacement)
}

func decodeLifecycleFixture(t *testing.T, data string) *Contract {
	t.Helper()

	s, err := decodeContract([]byte(data), policySource{})
	if err != nil {
		t.Fatalf("decodeContract() error = %v", err)
	}
	return s
}

func assertContractError(t *testing.T, data, want string) {
	t.Helper()

	_, err := decodeContract([]byte(data), policySource{})
	if err == nil {
		t.Fatalf("decodeContract() error = nil, want substring %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("decodeContract() error = %q, want substring %q", err, want)
	}
}

func replaceLifecycleRowText(t *testing.T, contract string, row int, from, to string) string {
	t.Helper()

	const header = "\n[[lifecycle]]\n"
	parts := strings.Split(contract, header)
	if row < 1 || row >= len(parts) {
		t.Fatalf("lifecycle row = %d, want 1..%d", row, len(parts)-1)
	}
	if got := strings.Count(parts[row], from); got != 1 {
		t.Fatalf("lifecycle row %d mutation needle count = %d, want 1 for %q", row, got, from)
	}
	parts[row] = strings.Replace(parts[row], from, to, 1)
	return strings.Join(parts, header)
}

// TestAdvancesFrom pins the rule the header count and a note's own page must
// share. They disagreed: the page offered two enabled buttons on a lesson the
// header had classified as having nowhere to go, because a stage whose
// predecessor list is the wildcard admits every status and an exact comparison
// never matched it.
func TestAdvancesFrom(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		stage  Stage
		status string
		want   bool
	}{
		{
			name:   "a named predecessor is somewhere to go",
			stage:  Stage{Status: "ready", AppliesTo: []string{"lesson"}, From: []string{"draft"}},
			status: "draft",
			want:   true,
		},
		{
			// The case the header used to miss: the contract says a lesson at
			// any status may be rewritten as a draft, and that is a step
			// forward the note page has always offered.
			name:   "a wildcard predecessor on a typed stage is somewhere to go",
			stage:  Stage{Status: "draft", AppliesTo: []string{"lesson"}, From: []string{"*"}},
			status: "imported",
			want:   true,
		},
		{
			// Reachable from everything by everything is retirement. Counting
			// it would make every note in the vault advanceable and the figure
			// would stop distinguishing anything.
			name:   "the stage every type reaches from every status is not",
			stage:  Stage{Status: "archived", AppliesTo: []string{"*"}, From: []string{"*"}},
			status: "seedling",
			want:   false,
		},
		{
			name:   "a stage cannot follow itself",
			stage:  Stage{Status: "active", AppliesTo: []string{"note"}, From: []string{"*"}},
			status: "active",
			want:   false,
		},
		{
			name:   "an unrelated predecessor is not",
			stage:  Stage{Status: "ready", AppliesTo: []string{"lesson"}, From: []string{"draft"}},
			status: "imported",
			want:   false,
		},
		{
			name:   "an initial stage is not reachable from a status",
			stage:  Stage{Status: "captured", AppliesTo: []string{"inbox"}, From: nil},
			status: "cleaned",
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := advancesFrom(&tt.stage, tt.status); got != tt.want {
				t.Errorf("advancesFrom(%+v, %q) = %t, want %t", tt.stage, tt.status, got, tt.want)
			}
		})
	}
}
