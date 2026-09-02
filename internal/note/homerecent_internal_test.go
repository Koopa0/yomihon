package note

import (
	"errors"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/vaultfs"
)

// homeStatusView opens one lifecycle over an empty folder under the supplied
// authorities and returns its read-only view.
func homeStatusView(t *testing.T, contract *schema.Contract, governance schema.Governance) status.View {
	t.Helper()
	reader, err := vaultfs.Open(t.TempDir())
	if err != nil {
		t.Fatalf("vaultfs.Open: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	lifecycle, err := status.Open(reader, contract, governance, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("status.Open: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := lifecycle.Close(); closeErr != nil {
			t.Errorf("Lifecycle.Close() error = %v", closeErr)
		}
	})
	return lifecycle.View()
}

// A vault whose contract file exists and cannot be read is governed and shut at
// the same time: it asserted a vocabulary and then failed to deliver one, so it
// answers "not declared" to every value put to it. The landing page renders the
// recent block in that state — plain reading survives a broken contract — so
// these rows are on screen exactly when no vocabulary can back a finding, and
// whether they accuse anyone is this function's own contract.
func TestRecentHomeNotesAccuseNothingWhenTheContractCannotBeRead(t *testing.T) {
	t.Parallel()
	contract, err := schema.LoadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("schema.LoadFile: %v", err)
	}
	notes := []nav.NoteSummary{
		{Title: "Legal", RelPath: "Concepts/legal.md", Type: "concept", Status: "draft", Modified: time.Unix(2, 0)},
		{Title: "Outside", RelPath: "Concepts/outside.md", Type: "concept", Status: "reviewing", Modified: time.Unix(1, 0)},
	}

	tests := []struct {
		name       string
		view       status.View
		wantFlags  int
		wantStatus bool
	}{
		{
			name:      "a contract is in force and could not be read",
			view:      homeStatusView(t, nil, schema.Unreadable(errors.New("contract unreadable"))),
			wantFlags: 0, wantStatus: true,
		},
		{
			// The control: the same rows under a contract that loaded do carry
			// the finding, so the row above is a restraint rather than a
			// function that never flags anything at all.
			name:      "the contract loaded and declares the vocabulary",
			view:      homeStatusView(t, contract, contract.Governance()),
			wantFlags: 1, wantStatus: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recent, _ := recentHomeNotes(notes, true, tt.view)
			if len(recent) != len(notes) {
				t.Fatalf("recentHomeNotes returned %d rows, want %d", len(recent), len(notes))
			}
			flags := 0
			for _, n := range recent {
				if n.StatusOutsideEnum {
					flags++
				}
				if (n.Status != "") != tt.wantStatus {
					t.Errorf("row %q status = %q, want present = %v", n.RelPath, n.Status, tt.wantStatus)
				}
			}
			if flags != tt.wantFlags {
				t.Errorf("recentHomeNotes flagged %d rows, want %d", flags, tt.wantFlags)
			}
		})
	}
}
