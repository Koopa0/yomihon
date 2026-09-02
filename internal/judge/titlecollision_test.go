package judge

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// titleFindingsByTarget maps each link.title_not_alias finding to its link
// target, failing on a duplicate so an assertion can never silently read the
// wrong finding.
func titleFindingsByTarget(t *testing.T, findings []Finding) map[string]Finding {
	t.Helper()
	out := make(map[string]Finding)
	for i := range findings {
		f := findings[i]
		if f.RuleID != "link.title_not_alias" {
			continue
		}
		if f.Target == nil {
			t.Fatalf("link.title_not_alias finding without a target: %+v", f)
		}
		if _, dup := out[*f.Target]; dup {
			t.Fatalf("two link.title_not_alias findings for target %q", *f.Target)
		}
		out[*f.Target] = f
	}
	return out
}

// TestTitleCollisionListsEveryHolder pins what a link to a title several notes
// hold reports: one finding whose collision_members names every holder, whose
// evidence describes the shared title across all of them, and whose suggested
// action favors none — advice naming one holder would send the author to a
// note the link may never have meant.
func TestTitleCollisionListsEveryHolder(t *testing.T) {
	t.Parallel()
	root := judgeFixtureRoot(t, "testdata/vault-titlecollision")
	wantGolden(t, runCheck(t, root), "testdata/golden/titlecollision.jsonl")
	findings, err := Check(root)
	if err != nil {
		t.Fatalf("Check(): %v", err)
	}
	byTarget := titleFindingsByTarget(t, findings)

	shared, ok := byTarget["Shared Title"]
	if !ok {
		t.Fatal("no link.title_not_alias finding for [[Shared Title]]")
	}
	wantMembers := []string{"Notes/one.md", "Notes/two.md", "Private/three.md"}
	if diff := cmp.Diff(wantMembers, shared.CollisionMembers); diff != "" {
		t.Errorf("collision_members mismatch (-want +got):\n%s", diff)
	}
	for _, member := range wantMembers {
		if !strings.Contains(shared.Evidence, member) {
			t.Errorf("evidence %q does not name holder %q; the collision itself is the evidence", shared.Evidence, member)
		}
		if strings.Contains(shared.SuggestedAction, member) {
			t.Errorf("suggested_action %q names %q; the advice must not pick one holder over another", shared.SuggestedAction, member)
		}
	}

	half, ok := byTarget["Half Private"]
	if !ok {
		t.Fatal("no link.title_not_alias finding for [[Half Private]]")
	}
	if diff := cmp.Diff([]string{"Notes/solo.md", "Private/hidden.md"}, half.CollisionMembers); diff != "" {
		t.Errorf("collision_members mismatch for the two-holder title (-want +got):\n%s", diff)
	}
}

// TestTitleCollisionWithholdsBeforeCounting pins the privacy order the alias
// and name collision rules already keep: holders are filtered before anything
// is counted or named, so a withheld holder is absent from every field, and a
// title left with a single describable holder takes the single-holder wording
// as if the withheld note did not exist.
func TestTitleCollisionWithholdsBeforeCounting(t *testing.T) {
	t.Parallel()
	root := judgeFixtureRootWithPrivacy(t, "testdata/vault-titlecollision", "Private")
	wantGolden(t, runCheck(t, root), "testdata/golden/titlecollision-privacy.jsonl")
	findings, err := Check(root)
	if err != nil {
		t.Fatalf("Check(): %v", err)
	}
	byTarget := titleFindingsByTarget(t, findings)

	shared, ok := byTarget["Shared Title"]
	if !ok {
		t.Fatal("no link.title_not_alias finding for [[Shared Title]]")
	}
	if diff := cmp.Diff([]string{"Notes/one.md", "Notes/two.md"}, shared.CollisionMembers); diff != "" {
		t.Errorf("collision_members must hold the describable holders alone (-want +got):\n%s", diff)
	}
	if joined := shared.Evidence + " " + shared.SuggestedAction; strings.Contains(joined, "Private/") {
		t.Errorf("a withheld holder leaked into the finding text: %q", joined)
	}

	half, ok := byTarget["Half Private"]
	if !ok {
		t.Fatal("no link.title_not_alias finding for [[Half Private]]")
	}
	if half.CollisionMembers != nil {
		t.Errorf("a single describable holder must take the single-holder wording; got members %v", half.CollisionMembers)
	}
	if !strings.Contains(half.Evidence, "Notes/solo.md") || !strings.Contains(half.SuggestedAction, "Notes/solo.md") {
		t.Errorf("the single-holder finding must keep naming its one describable holder; evidence %q, action %q",
			half.Evidence, half.SuggestedAction)
	}
	if strings.Contains(half.Evidence+half.SuggestedAction, "Private/") {
		t.Errorf("the single-holder finding leaked the withheld holder: evidence %q, action %q", half.Evidence, half.SuggestedAction)
	}
}
