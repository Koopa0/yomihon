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
