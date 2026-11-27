package schema

import (
	"strings"
	"testing"
)

func TestNavigationRolesNameTheLessonTypeOnlyWhenTheContractGivesTheRole(t *testing.T) {
	t.Parallel()

	const lessonRole = "[types.lesson]\nrole = \"lesson\"\n"
	if strings.Count(validContract, lessonRole) != 1 {
		t.Fatal("the base contract no longer gives lesson the lesson role")
	}
	lessonless := strings.Replace(validContract, lessonRole, "[types.lesson]\n", 1)
	renamed := strings.Replace(validContract, lessonRole, "[types.module]\nrole = \"lesson\"\n", 1)

	for _, tt := range []struct {
		name   string
		data   string
		want   map[string]bool
		holder string
	}{
		{
			name:   "a contract that gives the role to lesson",
			data:   validContract,
			want:   map[string]bool{"lesson": true, "study-path": false, "moc": false, "": false},
			holder: "lesson",
		},
		{
			name:   "a contract that gives it to another type",
			data:   renamed,
			want:   map[string]bool{"module": true, "lesson": false, "": false},
			holder: "module",
		},
		{
			name: "a contract that gives it to none",
			data: lessonless,
			want: map[string]bool{"lesson": false, "note": false, "": false},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			contract, err := loadContractBytes(t, []byte(tt.data))
			if err != nil {
				t.Fatalf("LoadFile() error = %v", err)
			}
			roles := contract.NavigationRoles()
			for noteType, want := range tt.want {
				if got := roles.IsLessonType(noteType); got != want {
					t.Errorf("IsLessonType(%q) = %t, want %t", noteType, got, want)
				}
			}
			name, declared := contract.LessonType()
			wantName := tt.holder
			if wantName == "" {
				wantName = "lesson"
			}
			if name != wantName || declared != (tt.holder != "") {
				t.Errorf("LessonType() = (%q, %t), want (%q, %t)", name, declared, wantName, tt.holder != "")
			}
		})
	}
}

func TestTheLessonTypeSurvivesRolesThatCannotBeHonoured(t *testing.T) {
	t.Parallel()

	data := strings.Replace(validContract, "[rules]\n", "[types.extra]\nrole = \"answers\"\n\n[rules]\n", 1)
	contract, err := loadContractBytes(t, []byte(data))
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	if contract.NavigationRoles().Available() {
		t.Fatal("an unknown role left the role sets open, so this case proves nothing about the lesson type")
	}
	if name, declared := contract.LessonType(); name != "lesson" || !declared {
		t.Errorf("LessonType() = (%q, %t) after the roles were rejected, want (\"lesson\", true)", name, declared)
	}
	if !CaptureAuthority(contract, contract.Governance()).IsLessonType("lesson") {
		t.Error("Authority.IsLessonType(lesson) = false after the roles were rejected, want the reading aids kept")
	}
}

// TestAuthorityReadsTheLessonTypeHoweverItIsComposed holds the lesson type to
// the spelling rule every other type lookup follows: a note read off a
// filesystem that decomposes its names still gets the reading aids.
func TestAuthorityReadsTheLessonTypeHoweverItIsComposed(t *testing.T) {
	t.Parallel()

	const composed, decomposed = "le\u00e7on", "lec\u0327on"
	data := strings.Replace(validContract, "[types.lesson]\n", "[types.\""+composed+"\"]\n", 1)
	contract, err := loadContractBytes(t, []byte(data))
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	authority := CaptureAuthority(contract, contract.Governance())
	for _, noteType := range []string{composed, decomposed} {
		if !authority.IsLessonType(noteType) {
			t.Errorf("Authority.IsLessonType(%+q) = false, want true", noteType)
		}
	}
}

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

func TestAFolderWithNoContractReadsTheBuiltInAnswerType(t *testing.T) {
	t.Parallel()
	var none *Contract
	if got, want := none.NavigationRoles().AnswerType(), "thought"; got != want {
		t.Errorf("AnswerType() = %q, want %q", got, want)
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

// A role only one type may hold, given to two, leaves no answer to which of
// them is meant, so the roles are refused as a whole.
func TestARoleTwoTypesHoldRefusesTheRoles(t *testing.T) {
	t.Parallel()

	contract, err := loadContractBytes(t, []byte(answerContract("[types.lecture]\nrole = \"lesson\"\nfields = [\"slug\"]\n")))
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	if contract.NavigationRoles().Available() {
		t.Error("NavigationRoles().Available() = true for a lesson role two types hold")
	}
}
