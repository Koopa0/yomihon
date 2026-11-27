package schema_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

func loadTextError(t *testing.T, text string) error {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) = %v", path, err)
	}
	_, err := schema.LoadFile(path)
	return err
}

// TestTheReferenceContractLoads reads back every declaration of the contract
// the author's own vault carries, so a loader that drops or reorders any part
// of the shape it was written for fails here first.
func TestTheReferenceContractLoads(t *testing.T) {
	t.Parallel()

	contract, err := schema.LoadFile(filepath.Join("testdata", "reference.toml"))
	if err != nil {
		t.Fatalf("LoadFile(testdata/reference.toml) = %v", err)
	}
	want := schema.Definition{
		Types: []schema.TypeDefinition{
			{
				Name:        "lesson",
				Role:        schema.RoleLesson,
				Required:    []string{"slug"},
				Fields:      []string{"slug", "level", "prerequisites", "verified_on", "verified_with", "based_on"},
				SlugPattern: `^(C[1-8]-(L[1-4]|D)-[0-9]{2}(-part-(0[2-9]|[1-9][0-9]))?|C4-題庫-P[0-9]{2}|C6-L1-EXAM|C7-L1-模擬考)$`,
			},
			{Name: "study-path", Role: schema.RoleCourse},
			{Name: "outline"},
		},
		Enums: []schema.EnumDefinition{
			{Field: "domain", Values: []string{"go", "sql", "system-design", "dsa", "llm", "ai", "backend", "rust"}},
			{Field: "level", Values: []string{"基礎", "中階", "資深", "前沿", "深入專題"}},
		},
		Fields: schema.FieldDefinition{Common: []string{"type", "aliases", "draft", "lang", "created"}},
		Rules: schema.Rules{
			PlannedGapMarks:    schema.DefaultPlannedGapMarks(),
			PlannedInlineMarks: schema.DefaultPlannedInlineMarks(),
		},
		Folders: schema.Folders{
			Checked:             []string{"courses"},
			Templates:           []string{"prompts"},
			DomainFrom:          []string{"courses"},
			FrontmatterOptional: true,
		},
	}
	if diff := cmp.Diff(want, contract.Definition()); diff != "" {
		t.Errorf("Definition() mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"lesson", "study-path", "outline"}, contract.EnumValues("type")); diff != "" {
		t.Errorf("EnumValues(type) mismatch (-want +got):\n%s", diff)
	}

	roles := contract.NavigationRoles()
	if !roles.Available() || !roles.IsPathType("study-path") || !roles.IsLessonType("lesson") || roles.IsMapType("outline") || roles.AnswerType() != "" {
		t.Errorf("NavigationRoles() = available %t, path study-path %t, lesson %t, map outline %t, answer %q; want the course and lesson roles alone",
			roles.Available(), roles.IsPathType("study-path"), roles.IsLessonType("lesson"), roles.IsMapType("outline"), roles.AnswerType())
	}
	if pattern, ok := contract.SlugPattern("lesson"); !ok || !pattern.MatchString("C2-L1-04") || pattern.MatchString("c2-l1-04") {
		t.Error("SlugPattern(lesson) does not hold the module slug the reference contract writes")
	}
	if !contract.ArtifactPolicy().IsNonInstance("prompts/review.md") {
		t.Error("ArtifactPolicy().IsNonInstance(prompts/review.md) = false, want the template folder held")
	}
	if !contract.KnowledgeScope().Includes("courses/go/a.md") || contract.KnowledgeScope().Includes("prompts/a.md") {
		t.Error("KnowledgeScope() does not hold courses alone")
	}
	if got := contract.Domains().Of("courses/go/lesson.md"); got != "go" {
		t.Errorf("Domains().Of(courses/go/lesson.md) = %q, want %q", got, "go")
	}
	if contract.RequiresFrontmatter() {
		t.Error("RequiresFrontmatter() = true, want false: frontmatter_optional = true")
	}
	if !contract.PrivacyPolicy().Available() {
		t.Errorf("PrivacyPolicy().Available() = false, diagnostic %q", contract.PrivacyPolicy().Diagnostic())
	}
	if !contract.AuthoredDate().Declared() || !contract.ArticleLanguage().Declared() {
		t.Error("the created and lang fields in fields.common were not given authority")
	}
}

const standingContract = `[fields]
common = ["type", "lang", "created"]
required = ["type", "created"]

[types.lesson]
role = "lesson"
required = ["slug", "created"]
fields = ["slug", "level"]
slug_pattern = "^[a-z]+(-[a-z]+)*$"

[types.study-path]
role = "course"

[types.concept]

[types.note]

[rules]
concept_requires_provenance = ["based_on"]
`

func TestFieldStanding(t *testing.T) {
	t.Parallel()

	contract := loadText(t, standingContract)
	tests := []struct {
		noteType string
		key      string
		want     schema.FieldStanding
	}{
		{"lesson", "slug", schema.FieldAllowed},
		{"lesson", "lang", schema.FieldAllowed},
		{"note", "lang", schema.FieldAllowed},
		{"", "lang", schema.FieldAllowed},
		{"concept", "based_on", schema.FieldAllowed},
		{"note", "slug", schema.FieldUnknown},
		{"note", "based_on", schema.FieldUnknown},
		{"lesson", "title", schema.FieldUnknown},
		{"lesson", "domain", schema.FieldUnknown},
		{"lesson", "status", schema.FieldUnknown},
		{"", "slug", schema.FieldUnknown},
		{"undeclared", "slug", schema.FieldTypeDependent},
		{"undeclared", "based_on", schema.FieldTypeDependent},
		{"Lesson", "slug", schema.FieldTypeDependent},
		{"undeclared", "title", schema.FieldUnknown},
		{"lesson", "lesson", schema.FieldUnknown},
	}
	for _, key := range []string{"type", "draft", "tags", "aliases", "cssclasses"} {
		for _, noteType := range []string{"lesson", "note", "", "undeclared"} {
			tests = append(tests, struct {
				noteType string
				key      string
				want     schema.FieldStanding
			}{noteType, key, schema.FieldAllowed})
		}
	}
	for _, tt := range tests {
		if got := contract.FieldStanding(tt.noteType, tt.key); got != tt.want {
			t.Errorf("FieldStanding(%q, %q) = %v, want %v", tt.noteType, tt.key, got, tt.want)
		}
	}
}

func TestRequiredFields(t *testing.T) {
	t.Parallel()

	contract := loadText(t, standingContract)
	for _, tt := range []struct {
		noteType string
		want     []string
	}{
		{"lesson", []string{"type", "created", "slug"}},
		{"note", []string{"type", "created"}},
		{"", []string{"type", "created"}},
		{"undeclared", []string{"type", "created"}},
	} {
		if diff := cmp.Diff(tt.want, contract.RequiredFields(tt.noteType)); diff != "" {
			t.Errorf("RequiredFields(%q) mismatch (-want +got):\n%s", tt.noteType, diff)
		}
	}
}

func TestSlugPattern(t *testing.T) {
	t.Parallel()

	contract := loadText(t, standingContract)
	pattern, ok := contract.SlugPattern("lesson")
	if !ok {
		t.Fatal(`SlugPattern("lesson") = false, want the declared pattern`)
	}
	for slug, want := range map[string]bool{"go-basics": true, "Go-basics": false, "go_basics": false, "": false} {
		if got := pattern.MatchString(slug); got != want {
			t.Errorf("SlugPattern(lesson).MatchString(%q) = %t, want %t", slug, got, want)
		}
	}
	for _, noteType := range []string{"note", "study-path", "undeclared", ""} {
		if pattern, ok := contract.SlugPattern(noteType); ok || pattern != nil {
			t.Errorf("SlugPattern(%q) = (%v, %t), want (nil, false)", noteType, pattern, ok)
		}
	}
}

func TestEnumKeysAreFieldsOrTheDomain(t *testing.T) {
	t.Parallel()

	contract := loadText(t, "[enums]\ndomain = [\"golang\"]\nlevel = [\"basic\"]\n")
	if diff := cmp.Diff([]string{"golang"}, contract.EnumValues("domain")); diff != "" {
		t.Errorf("EnumValues(domain) mismatch (-want +got):\n%s", diff)
	}
	for _, key := range []string{"title", "status", "tags", "topics"} {
		err := loadTextError(t, "[enums]\n"+key+" = [\"x\"]\n")
		want := `enums.` + key + `: "` + key + `" is not a field yomihon defines or derives`
		if err == nil || !strings.HasSuffix(err.Error(), want) {
			t.Errorf("LoadFile([enums] %s) error = %v, want it to end with %q", key, err, want)
		}
	}
}

// TestTheTypeListIsTheEnumThenTheTables holds the one order the type list has:
// the words enums.type writes, then each table not already named, as written.
func TestTheTypeListIsTheEnumThenTheTables(t *testing.T) {
	t.Parallel()

	contract := loadText(t, `[enums]
type = ["zeta", "alpha"]

[types.mid]

[types.alpha]
role = "map"

[types.beta]
`)
	want := []string{"zeta", "alpha", "mid", "beta"}
	if diff := cmp.Diff(want, contract.EnumValues("type")); diff != "" {
		t.Errorf("EnumValues(type) mismatch (-want +got):\n%s", diff)
	}
	var names []string
	for _, def := range contract.Definition().Types {
		names = append(names, def.Name)
	}
	if diff := cmp.Diff(want, names); diff != "" {
		t.Errorf("Definition().Types names mismatch (-want +got):\n%s", diff)
	}
	if !contract.NavigationRoles().IsMapType("alpha") {
		t.Error("the table for a type enums.type also names lost its role")
	}
}

// TestATypeIsOneTypeHoweverEachPlaceComposesIt holds the type list to the
// spelling rule: enums.type and a [types] table naming one word, one of them
// decomposed, declare one type, and the table's role is that type's.
func TestATypeIsOneTypeHoweverEachPlaceComposesIt(t *testing.T) {
	t.Parallel()

	contract := loadText(t, `[enums]
type = ["Cafe\u0301"]

[types."Caf\u00e9"]
role = "map"
`)
	if diff := cmp.Diff([]string{"Caf\u00e9"}, contract.EnumValues("type")); diff != "" {
		t.Errorf("EnumValues(type) mismatch (-want +got):\n%s", diff)
	}
	if !contract.NavigationRoles().IsMapType("Caf\u00e9") {
		t.Error("the table for a type enums.type also names lost its role")
	}

	err := loadTextError(t, "[types.\"Caf\u00e9\"]\nrole = \"map\"\n\n[types.\"Cafe\u0301\"]\n")
	if err == nil || !strings.Contains(err.Error(), "types: duplicate type") {
		t.Errorf("two tables composing one word: error = %v, want types: duplicate type", err)
	}
}

func TestDomainsComeFromTheFolder(t *testing.T) {
	t.Parallel()

	contract := loadText(t, "[folders]\ndomain_from = [\"courses\", \"Writing/lessons\"]\n")
	domains := contract.Domains()
	for path, want := range map[string]string{
		"courses/go/a.md":                "go",
		"courses/go/deep/a.md":           "go",
		"Writing/lessons/japanese/L1.md": "japanese",
		"courses/a.md":                   "",
		"Writing/go/a.md":                "",
		"Notes/go/a.md":                  "",
	} {
		if got := domains.Of(path); got != want {
			t.Errorf("Domains().Of(%q) = %q, want %q", path, got, want)
		}
	}
	if diff := cmp.Diff([]string{"courses", "Writing/lessons"}, domains.Roots()); diff != "" {
		t.Errorf("Domains().Roots() mismatch (-want +got):\n%s", diff)
	}
}

// TestDatesAndLanguageNeedFieldsCommon holds where the two vault-wide readings
// get their authority: fields.common, which describes every note, and never a
// single type's list.
func TestDatesAndLanguageNeedFieldsCommon(t *testing.T) {
	t.Parallel()

	common := loadText(t, "[fields]\ncommon = [\"type\", \"lang\", \"created\"]\n")
	if !common.AuthoredDate().Declared() {
		t.Error("AuthoredDate().Declared() = false with created in fields.common, want true")
	}
	if !common.ArticleLanguage().Declared() {
		t.Error("ArticleLanguage().Declared() = false with lang in fields.common, want true")
	}

	oneType := loadText(t, "[fields]\ncommon = [\"type\"]\n\n[types.lesson]\nfields = [\"lang\", \"created\"]\n")
	if oneType.AuthoredDate().Declared() {
		t.Error("AuthoredDate().Declared() = true with created on one type only, want false")
	}
	if oneType.ArticleLanguage().Declared() {
		t.Error("ArticleLanguage().Declared() = true with lang on one type only, want false")
	}
}
