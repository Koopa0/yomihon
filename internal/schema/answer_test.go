package schema

import (
	"strings"
	"testing"
)

const thoughtType = "[types.thought]\nrole = \"thought\"\nfields = [\"based_on\"]\n"

func TestAnswerTypeIsTheTypeHoldingTheThoughtRole(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name        string
		declaration string
		want        string
	}{
		{name: "absent"},
		{name: "declared", declaration: thoughtType, want: "thought"},
		{name: "another name", declaration: "[types.reflection]\nrole = \"thought\"\nfields = [\"based_on\"]\n", want: "reflection"},
		{name: "a plain type", declaration: "[types.thought]\nfields = [\"based_on\"]\n"},
		{name: "role in another case", declaration: "[types.thought]\nrole = \"Thought\"\nfields = [\"based_on\"]\n"},
		{name: "role held twice", declaration: thoughtType + "[types.reflection]\nrole = \"thought\"\nfields = [\"based_on\"]\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			contract, err := loadContractBytes(t, []byte(answerContract(tt.declaration)))
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

	for _, declaration := range []string{"", thoughtType} {
		t.Run(strings.TrimSpace(declaration), func(t *testing.T) {
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

func TestAnswerTypeUnknownRoleClosesIt(t *testing.T) {
	t.Parallel()
	data := answerContract(thoughtType + "[types.extra]\nrole = \"answers\"\n")
	contract, err := loadContractBytes(t, []byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if got := contract.NavigationRoles().AnswerType(); got != "" {
		t.Errorf("AnswerType() = %q despite an unknown role beside it", got)
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
	data := answerContract("[types.\"re\u0301ponse\"]\nrole = \"thought\"\nfields = [\"based_on\"]\n")
	contract, err := loadContractBytes(t, []byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if got := contract.NavigationRoles().AnswerType(); got != "r\u00e9ponse" {
		t.Errorf("AnswerType() = %q, want the declared type in its folded spelling", got)
	}
}

func answerContract(declaration string) string {
	return strings.Replace(validContract, "[rules]\n", declaration+"\n[rules]\n", 1)
}
