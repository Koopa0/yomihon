package snapshot

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestCapturedSnapshotPathSuffix(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for rel, body := range map[string]string{
		"Concepts/Atlas/Page.md":    "---\ntitle: Atlas page\ntype: concept\n---\n## Part\n\nBlock payload. ^piece\n",
		"Concepts/A/Shared/Twin.md": "body\n",
		"Concepts/B/Shared/Twin.md": "body\n",
		"Concepts/Citer.md":         "See [[Atlas/Page]] and [[Atlas/Page#Part]], [[Shared/Twin]], [[Nope/Page]].\n",
		"Concepts/Claim.md":         "---\nbased_on: ['[[Atlas/Page#Part]]', '[[Atlas/Page#^piece]]', '[[Shared/Twin]]']\n---\nClaim.\n",
		"Maps/Ledger.md":            "## Gaps\n\n- Atlas/Page\n- Nope/Owed\n",
	} {
		writeNote(t, root, rel, body)
	}
	store, _ := newTestStore(t, root, testContractWithRules(t, root, `planned_gap_marks = ["Gaps"]`))
	g := store.Current()
	t.Log("invoked: actual suffix resolution contract")
	if diff := cmp.Diff([]nav.NoteRef{{Name: "Citer", RelPath: "Concepts/Citer.md"}}, g.CitedBy("Concepts/Atlas/Page.md")); diff != "" {
		t.Errorf("caught: suffix backlink missing: %s", diff)
	}
	for _, rel := range []string{"Concepts/A/Shared/Twin.md", "Concepts/B/Shared/Twin.md"} {
		if refs := g.CitedBy(rel); len(refs) != 0 {
			t.Errorf("caught: ambiguous suffix became a backlink: %+v", refs)
		}
	}
	wantHealth := []HealthLink{{From: nav.NoteRef{Name: "Citer", RelPath: "Concepts/Citer.md"}, Target: "Nope/Page"}}
	if diff := cmp.Diff(wantHealth, g.Health().Unwritten); diff != "" {
		t.Errorf("caught: suffix health missing-link set changed: %s", diff)
	}
	if g.TrackedForwardReference("Atlas/Page") || !g.TrackedForwardReference("Nope/Owed") {
		t.Errorf("caught: suffix planned-gap classification changed")
	}
	wantBased := []nav.NoteRef{{Name: "Page", RelPath: "Concepts/Atlas/Page.md"}, {Name: "[[Shared/Twin]]"}}
	if diff := cmp.Diff(wantBased, g.BasedOn("Concepts/Claim.md")); diff != "" {
		t.Errorf("caught: suffix source projection missing: %s", diff)
	}
	if diff := cmp.Diff([]nav.NoteRef{{Name: "Claim", RelPath: "Concepts/Claim.md"}}, g.BasedOnBy("Concepts/Atlas/Page.md")); diff != "" {
		t.Errorf("caught: suffix reverse source projection missing: %s", diff)
	}
	sources, diags := g.DeclaredSources("Concepts/Claim.md", wording.En)
	wantSources := []DeclaredSource{{Name: "Page", RelPath: "Concepts/Atlas/Page.md", Locations: []render.SourceLocation{{Label: "Part", Fragment: "part"}, {Label: "^piece", Fragment: "^piece"}}}, {Name: "[[Shared/Twin]]"}}
	if diff := cmp.Diff(wantSources, sources); diff != "" {
		t.Errorf("caught: suffix source locations missing: %s", diff)
	}
	if len(diags) != 0 {
		t.Errorf("source diagnostics = %+v", diags)
	}
}
