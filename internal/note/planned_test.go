package note

import (
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/vaultfs"
	"github.com/koopa0/yomihon/internal/wording"
)

// plainVaultView builds one generation over root — an ungoverned folder, which
// is all the classifier needs: planning is a convention of the prose, not of
// the contract.
func plainVaultView(t *testing.T, root string) *snapshot.View {
	t.Helper()
	reader, err := vaultfs.Open(root)
	if err != nil {
		t.Fatalf("vaultfs.Open(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	store, err := snapshot.New(t.Context(), reader, slog.New(slog.DiscardHandler), nil, schema.Governance{})
	if err != nil {
		t.Fatalf("snapshot.New() error = %v", err)
	}
	view := store.Current()
	if view == nil {
		t.Fatal("store.Current() = nil, want a built generation")
	}
	return view
}

// The vault is written forward: a note lists the concepts it still owes under a
// heading marked as a gap, then links those names from wherever they belong,
// long before the target files exist. Those links resolve to nothing and are
// still not faults, and the reading page has to tell them apart from a link
// that names nothing anyone ever intended to write — the same distinction the
// adjudicator draws, reached the same way.
func TestFaultsKeepsOnlyUnplannedBrokenLinks(t *testing.T) {
	root := t.TempDir()
	writeNote(t, root, "Maps/plan.md", `---
title: Plan
---

## 缺口帳

- Consistent Hashing
- Vector Clocks（暫定）
`)
	// Written elsewhere, against names the ledger above has declared.
	writeNote(t, root, "Notes/tracked.md", `---
title: Tracked
---

Partitioning depends on [[Consistent Hashing]] and [[Vector Clocks]].
`)
	// The same shape of link, against a name nothing has declared.
	writeNote(t, root, "Notes/unplanned.md", `---
title: Unplanned
---

This cites [[Byzantine Broadcast]], which no note has said it will write.
`)

	snap := plainVaultView(t, root)

	tests := []struct {
		name       string
		relPath    string
		body       string
		wantBroken []string
	}{
		{
			name:    "links naming planned concepts are not faults",
			relPath: "Notes/tracked.md",
			body:    "Partitioning depends on [[Consistent Hashing]] and [[Vector Clocks]].\n",
		},
		{
			name:       "a link naming nothing anyone planned stays a fault",
			relPath:    "Notes/unplanned.md",
			body:       "This cites [[Byzantine Broadcast]], which no note has said it will write.\n",
			wantBroken: []string{"Byzantine Broadcast"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered := snap.Render(tt.relPath, tt.body, wording.ZhHant)
			// The render pass must have seen every link as unresolved, or the
			// filter below would be reported as working while never running.
			if len(brokenTargets(rendered.Diagnostics)) == 0 {
				t.Fatalf("Render(%q) produced no broken-link diagnostic to classify", tt.relPath)
			}
			got := brokenTargets(faults(rendered.Diagnostics, snap))
			if !slices.Equal(got, tt.wantBroken) {
				t.Errorf("faults(Render(%q)) broken targets = %v, want %v", tt.relPath, got, tt.wantBroken)
			}
		})
	}
}

// A wikilink target that does resolve can still fail to render — an embed whose
// body this generation did not capture reports the same diagnostic kind — and
// that failure has nothing to do with planning. Naming such a target in the gap
// ledger must not silence it.
func TestFaultsKeepsResolvedTargetsThatFailedToRender(t *testing.T) {
	root := t.TempDir()
	writeNote(t, root, "Maps/plan.md", `---
title: Plan
---

## 缺口帳

- Consistent Hashing
`)
	writeNote(t, root, "Notes/Consistent Hashing.md", "---\ntitle: Consistent Hashing\n---\n\nWritten after all.\n")

	snap := plainVaultView(t, root)

	// The note now exists, so the ledger entry is stale — resolution, asked
	// first, is what keeps the planned name from deciding anything here.
	if snap.TrackedForwardReference("Consistent Hashing") {
		t.Errorf("TrackedForwardReference(%q) = true for a name that resolves to a written note, want false", "Consistent Hashing")
	}

	diags := []render.Diagnostic{{
		Kind:    render.DiagWikilinkBroken,
		Target:  "Consistent Hashing",
		Message: `embed target "Notes/Consistent Hashing.md" is unavailable in the captured generation`,
	}}
	if got := len(faults(diags, snap)); got != 1 {
		t.Errorf("faults() kept %d diagnostics for a resolved-but-unrendered target, want 1", got)
	}
}

func brokenTargets(diags []render.Diagnostic) []string {
	var out []string
	for _, d := range diags {
		if d.Kind == render.DiagWikilinkBroken {
			out = append(out, d.Target)
		}
	}
	return out
}

func writeNote(t *testing.T, root, relPath, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", full, err)
	}
}
