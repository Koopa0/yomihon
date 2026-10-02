package schema

import (
	"strings"
	"testing"
)

func TestNavigationRolesNameTheLessonTypeOnlyWhenTheContractListsIt(t *testing.T) {
	t.Parallel()

	lessonless := strings.NewReplacer(
		`type = ["lesson", "study-path", "moc"]`, `type = ["note", "study-path", "moc"]`,
		"lesson = [\"draft\", \"archived\"]\n", "",
		"[fields.status_group]\nlesson = [\"lesson\"]\n", "",
		`applies_to = ["lesson", "study-path"]`, `applies_to = ["note", "study-path"]`,
		`lesson_only = ["slug", "evolution_predecessor", "evolution_successors"]`, `lesson_only = []`,
		"[supersession]\npredecessor_field = \"evolution_predecessor\"\nsuccessor_field = \"evolution_successors\"\ngeneral_link_field = \"related\"\narchived_status = \"archived\"\n\n", "",
	).Replace(validContractV1)
	if lessonless == validContractV1 {
		t.Fatal("the replacer matched nothing, so the contract below still lists the lesson type")
	}

	for _, tt := range []struct {
		name string
		data string
		want map[string]bool
	}{
		{
			name: "a contract that lists it",
			data: validContractV1,
			want: map[string]bool{"lesson": true, "study-path": false, "moc": false, "": false},
		},
		{
			name: "a contract that does not",
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
		})
	}
}

func TestTheLessonTypeSurvivesANavigationDeclarationThatCannotBeHonoured(t *testing.T) {
	t.Parallel()

	data := strings.Replace(validContractV1, "[navigation]\n", "[navigation]\nanswer_types = [\"moc\"]\n", 1)
	contract, err := loadContractBytes(t, []byte(data))
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	roles := contract.NavigationRoles()
	if roles.Available() {
		t.Fatal("an unknown navigation key left the role sets open, so this case proves nothing about the lesson type")
	}
	if !roles.IsLessonType("lesson") {
		t.Error("IsLessonType(lesson) = false after navigation was rejected, want the lesson type kept")
	}
}

func TestTheZeroNavigationRolesNameNoLessonType(t *testing.T) {
	t.Parallel()

	var roles NavigationRoles
	if roles.IsLessonType("lesson") {
		t.Error("the zero NavigationRoles named a lesson type")
	}
	if (&Contract{}).NavigationRoles().IsLessonType("lesson") {
		t.Error("a contract that decoded nothing named a lesson type")
	}
}
