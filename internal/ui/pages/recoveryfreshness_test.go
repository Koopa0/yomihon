package pages

import "testing"

func TestRecoveryFreshnessAttrsNeedBothFacts(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		view  StatusRecoveryView
		watch bool
	}{
		{
			name:  "a refusal that knows the note and the version",
			view:  StatusRecoveryView{NotePath: "Writing/note.md", NoteIdentity: "3f2a"},
			watch: true,
		},
		{
			name:  "a refusal with no version to wait for",
			view:  StatusRecoveryView{NotePath: "Writing/note.md"},
			watch: false,
		},
		{
			name:  "a refusal that names no note",
			view:  StatusRecoveryView{NoteIdentity: "3f2a"},
			watch: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := len(recoveryFreshnessAttrs(tt.view)) > 0; got != tt.watch {
				t.Errorf("recoveryFreshnessAttrs() watches = %t, want %t", got, tt.watch)
			}
		})
	}
}
