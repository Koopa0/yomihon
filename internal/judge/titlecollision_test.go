package judge

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestTitleCollisionWithholdsBeforeCounting holds that the holders of a shared
// title are filtered before they are counted or named: a withheld holder is
// absent from every byte, and a title left with one describable holder is no
// collision.
func TestTitleCollisionWithholdsBeforeCounting(t *testing.T) {
	t.Parallel()
	out := runCheck(t, judgeFixtureRootWithPrivacy(t, "testdata/vault-titlecollision", "Private"))
	if bytes.Contains(out, []byte("Private/")) {
		t.Errorf("a withheld holder leaked into the report:\n%s", out)
	}
	members := map[string][]string{}
	for _, f := range decodeFindings(t, out) {
		if f.RuleID == "link.title_not_alias" && f.Target != nil {
			members[*f.Target] = f.CollisionMembers
		}
	}
	want := map[string][]string{
		"Shared Title": {"Notes/one.md", "Notes/two.md"},
		"Half Private": nil,
	}
	if diff := cmp.Diff(want, members); diff != "" {
		t.Errorf("link.title_not_alias members by target (-want +got):\n%s", diff)
	}
}
