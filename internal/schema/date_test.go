package schema

import (
	"testing"
	"time"
)

func TestAuthoredDateField(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		contract *Contract
		want     string
	}{
		{name: "no contract", contract: nil, want: ""},
		{name: "declares neither", contract: &Contract{definition: Definition{Fields: Fields{Known: []string{"title"}}}}, want: ""},
		{name: "declares created", contract: &Contract{definition: Definition{Fields: Fields{Known: []string{"title", "created"}}}}, want: "created"},
		{name: "declares updated", contract: &Contract{definition: Definition{Fields: Fields{Known: []string{"title", "updated"}}}}, want: "updated"},
		// One field answers for the whole vault, so a shelf's date column
		// carries one meaning. Where both are declared it is the day the note
		// was made, which is what dates a report.
		{name: "declares both", contract: &Contract{definition: Definition{Fields: Fields{Known: []string{"updated", "created"}}}}, want: "created"},
		// A lesson-only declaration cannot describe every note, and a report
		// is not a lesson.
		{name: "lesson-only created", contract: &Contract{definition: Definition{Fields: Fields{LessonOnly: []string{"created"}}}}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dated := tt.contract.AuthoredDate()
			if got := dated.Field(); got != tt.want {
				t.Errorf("AuthoredDate().Field() = %q, want %q", got, tt.want)
			}
			if got := dated.Declared(); got != (tt.want != "") {
				t.Errorf("AuthoredDate().Declared() = %v, want %v", got, tt.want != "")
			}
		})
	}
}

func TestAuthoredDateRequiresContractAuthority(t *testing.T) {
	t.Parallel()
	got, err := (&Contract{}).AuthoredDate().Resolve(map[string]any{"created": "2026-08-31"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got != "" {
		t.Errorf("Resolve() = %q, want no day without a contract declaration", got)
	}
}

func TestAuthoredDateResolve(t *testing.T) {
	t.Parallel()
	dated := (&Contract{definition: Definition{Fields: Fields{Known: []string{"created"}}}}).AuthoredDate()
	tests := []struct {
		name        string
		frontmatter map[string]any
		want        string
		wantErr     bool
	}{
		{name: "missing", frontmatter: nil, want: ""},
		// "created:" with nothing after it decodes to null, which wrote no day
		// to misread.
		{name: "written empty", frontmatter: map[string]any{"created": nil}, want: ""},
		// An unquoted YAML date arrives already decoded as a time.
		{name: "decoded time", frontmatter: map[string]any{"created": time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)}, want: "2026-08-31"},
		{name: "quoted day", frontmatter: map[string]any{"created": "2026-08-31"}, want: "2026-08-31"},
		{name: "a moment is still one day", frontmatter: map[string]any{"created": "2026-08-31T09:30:00Z"}, want: "2026-08-31"},
		// A moment carries a zone, and the day it names is the one its author
		// was writing on.
		{name: "a moment keeps its own day", frontmatter: map[string]any{"created": "2026-08-31T23:30:00+09:00"}, want: "2026-08-31"},
		{name: "unpadded", frontmatter: map[string]any{"created": "2026-8-3"}, want: "", wantErr: true},
		{name: "prose", frontmatter: map[string]any{"created": "soon"}, want: "", wantErr: true},
		{name: "a list of days", frontmatter: map[string]any{"created": []any{"2026-08-31"}}, want: "", wantErr: true},
		{name: "a number", frontmatter: map[string]any{"created": 2026}, want: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := dated.Resolve(tt.frontmatter)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Resolve(%v) error = %v, wantErr %v", tt.frontmatter, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Resolve(%v) = %q, want %q", tt.frontmatter, got, tt.want)
			}
		})
	}
}

// TestAuthoredDateReadsOnlyTheDeclaredField keeps the second field silent
// while the first holds authority: a vault dated by created that also writes
// updated is dated by one of them, not by whichever a note happens to carry.
func TestAuthoredDateReadsOnlyTheDeclaredField(t *testing.T) {
	t.Parallel()
	dated := (&Contract{definition: Definition{Fields: Fields{Known: []string{"created", "updated"}}}}).AuthoredDate()
	got, err := dated.Resolve(map[string]any{"updated": "2026-08-31"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got != "" {
		t.Errorf("Resolve() = %q, want no day: this contract dates a note by %q", got, dated.Field())
	}
}
