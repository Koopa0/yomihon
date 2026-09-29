package schema

import (
	"strings"
	"testing"
)

func TestAnswerTypeRequiresExplicitEnumMember(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name        string
		declaration string
		want        string
	}{
		{name: "absent"},
		{name: "declared", declaration: `answer_type = "lesson"`, want: "lesson"},
		{name: "another member", declaration: `answer_type = "moc"`, want: "moc"},
		{name: "empty", declaration: `answer_type = ""`},
		{name: "undeclared", declaration: `answer_type = "note"`},
		{name: "case differs", declaration: `answer_type = "Lesson"`},
		{name: "list", declaration: `answer_type = ["lesson"]`},
		{name: "boolean", declaration: `answer_type = true`},
		{name: "number", declaration: `answer_type = 1`},
		{name: "table", declaration: `answer_type = { type = "lesson" }`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := answerContract(tt.declaration)
			contract, err := loadContractBytes(t, []byte(data))
			if err != nil {
				t.Fatalf("LoadFile() error = %v", err)
			}
			if got := contract.NavigationRoles().AnswerType(); got != tt.want {
				t.Errorf("AnswerType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAnswerTypeDoesNotReplaceOtherNavigationDeclarations(t *testing.T) {
	t.Parallel()

	for _, declaration := range []string{"", `answer_type = "lesson"`} {
		t.Run(declaration, func(t *testing.T) {
			t.Parallel()
			contract, err := loadContractBytes(t, []byte(answerContract(declaration)))
			if err != nil {
				t.Fatal(err)
			}
			roles := contract.NavigationRoles()
			if !roles.Available() || !roles.IsPathType("study-path") || !roles.IsMapType("moc") {
				t.Error("answer role changed the declared path/map roles")
			}
			if !contract.JournalDir().Contains("Diary/today.md") {
				t.Error("answer role changed the declared journal directory")
			}
		})
	}
}

func TestAnswerTypeUnknownNavigationKeyClosesRole(t *testing.T) {
	t.Parallel()
	contract, err := loadContractBytes(t, []byte(answerContract("answer_type = \"lesson\"\nanswer_types = [\"moc\"]")))
	if err != nil {
		t.Fatal(err)
	}
	if got := contract.NavigationRoles().AnswerType(); got != "" {
		t.Errorf("AnswerType() = %q despite unknown navigation key", got)
	}
}

func TestAnswerTypeHasNoFabricatedAuthority(t *testing.T) {
	t.Parallel()
	for _, contract := range []*Contract{nil, {}} {
		if got := contract.NavigationRoles().AnswerType(); got != "" {
			t.Errorf("AnswerType() = %q without a decoded declaration", got)
		}
	}
}

func TestAnswerTypeAcceptsDeclaredUnicodeWord(t *testing.T) {
	t.Parallel()
	data := strings.Replace(validContractV1, `type = ["lesson", "study-path", "moc"]`, `type = ["lesson", "study-path", "moc", "réponse"]`, 1)
	data = strings.Replace(data, "[navigation]\n", "[navigation]\nanswer_type = \"réponse\"\n", 1)
	contract, err := loadContractBytes(t, []byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if got := contract.NavigationRoles().AnswerType(); got != "r\u00e9ponse" {
		t.Errorf("AnswerType() = %q, want declared enum member", got)
	}
}

func answerContract(declaration string) string {
	return strings.Replace(validContractV1, "[navigation]\n", "[navigation]\njournal_dir = \"Diary\"\n"+declaration+"\n", 1)
}
