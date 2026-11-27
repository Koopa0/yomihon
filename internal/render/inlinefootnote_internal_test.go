package render

import "testing"

// TestInlineFootnoteDefinitionsFollowTheNote holds where the expansion writes
// a definition: after the note, including when a caret stands in front of the
// inline form and the reference it becomes reads like another inline note.
func TestInlineFootnoteDefinitionsFollowTheNote(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, body, want string }{
		{"a plain note", "a^[x]\n", "a[^yomihon-inline-footnote-1]\n[^yomihon-inline-footnote-1]: x\n\n"},
		{"a caret before the note", "^^[x]\n", "^[^yomihon-inline-footnote-1]\n[^yomihon-inline-footnote-1]: x\n\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := expandInlineFootnotes(tt.body); got != tt.want {
				t.Errorf("expandInlineFootnotes(%q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}
