package pages

import (
	"testing"

	"github.com/koopa0/yomihon/internal/nav"
)

// TestHealthIsNotCleanWhileAnyListHasSomethingIn It holds the one sentence this
// page stakes its credibility on. Whether the folder has nothing to answer for
// is decided by naming every list, so a list added to the page and not to that
// decision produces a page that shows findings and says all is well at once.
//
// Each case sets exactly one list, because a fixture that trips two of them
// would still pass with either one forgotten — which is how a check about a
// conjunction quietly stops checking half of it.
func TestHealthIsNotCleanWhileAnyListHasSomethingInIt(t *testing.T) {
	t.Parallel()

	ref := []nav.NoteRef{{RelPath: "Concepts/a.md", Name: "a"}}
	for _, tc := range []struct {
		name string
		view HealthView
	}{
		{"unwritten citation", HealthView{Unwritten: []HealthLink{{}}}},
		{"title-only citation", HealthView{TitleOnly: []HealthTitleLink{{}}}},
		{"an uncited note", HealthView{IslandCount: 1}},
		{"a shared name", HealthView{Collisions: []HealthCollision{{}}}},
		{"an unreadable source", HealthView{Blocked: []HealthBlockedSource{{}}}},
		{"a status outside its list", HealthView{StatusOutsideEnum: []HealthStatusNote{{}}}},
		{"frontmatter that cannot be read", HealthView{FrontmatterUnreadable: ref}},
		{"frontmatter the schema rejects", HealthView{SchemaFaults: ref}},
		{"a scope that could not be worked out", HealthView{InstanceScopeUnknown: "why"}},
		{"a vocabulary that could not be read", HealthView{SchemaScopeUnknown: "why"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.view.clean() {
				t.Error("the page would say the folder has nothing to answer for while holding this finding")
			}
		})
	}

	if !(HealthView{}).clean() {
		t.Error("a folder with nothing in any list does not read as clean, so the sentence could never appear")
	}
}
