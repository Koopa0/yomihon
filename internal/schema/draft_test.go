package schema_test

import (
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// TestDraftIsTheBooleanTrueAndNothingElse pins what declares a note unfinished:
// the YAML boolean true. Absence, false and every other shape is finished.
func TestDraftIsTheBooleanTrueAndNothingElse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		frontmatter string
		want        bool
	}{
		{"draft: true", true},
		{"draft: True", true},
		{"draft: false", false},
		{"", false},
		{`draft: "true"`, false},
		{"draft: yes", false},
		{"draft: 1", false},
		{"draft:", false},
		{"status: draft", false},
	}
	for _, tt := range tests {
		n := vault.Parse("note.md", []byte("---\ntitle: T\n"+tt.frontmatter+"\n---\nbody\n"))
		if got := schema.Draft(n.Frontmatter); got != tt.want {
			t.Errorf("Draft() with %q = %t, want %t", tt.frontmatter, got, tt.want)
		}
	}
}
