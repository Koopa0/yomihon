package note

import (
	"errors"
	"log/slog"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
)

// homeStatusView opens one lifecycle over an empty folder under the supplied
// authorities and returns its read-only view.
func homeStatusView(t *testing.T, contract *schema.Contract, governance schema.Governance) status.Authority {
	t.Helper()
	reader, err := vault.Open(t.TempDir())
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
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
	return lifecycle.Authority()
}

// A vault whose contract file exists and cannot be read is governed and shut at
// the same time: it asserted a vocabulary and then failed to deliver one, so it
// answers "not declared" to every value put to it. The folder index renders the
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
		view       status.Authority
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
			recent, _ := recentShelfNotes(notes, true, tt.view, nil)
			if len(recent) != len(notes) {
				t.Fatalf("recentShelfNotes returned %d rows, want %d", len(recent), len(notes))
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
				t.Errorf("recentShelfNotes flagged %d rows, want %d", flags, tt.wantFlags)
			}
		})
	}
}

func TestRecentShelfNotesCarryDeclaredLanguage(t *testing.T) {
	t.Parallel()
	lookup := pages.ArticleLanguageFor(func(relPath string) string {
		if relPath == "Writing/lessons/japanese/L01.md" {
			return "ja"
		}
		return ""
	})
	notes := []nav.NoteSummary{{
		Title:   "L01 わたしは学生です",
		RelPath: "Writing/lessons/japanese/L01.md",
		Type:    "lesson",
	}}
	recent, _ := recentShelfNotes(notes, false, status.Authority{}, lookup)
	if len(recent) != 1 {
		t.Fatalf("recentShelfNotes returned %d rows, want 1", len(recent))
	}
	if recent[0].Language != "ja" {
		t.Errorf("recent row language = %q, want ja", recent[0].Language)
	}
}

// A fresh clone stamps every file with one moment, so the tie-break is what
// the recent shelf shows. It must be the vault's one reading order, the same
// a folder lists, not a byte order that puts Untitled 2 before Untitled and
// TODO before index.
func TestRecentShelfNotesBreakTiesInReadingOrder(t *testing.T) {
	t.Parallel()
	stamp := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var notes []nav.NoteSummary
	for _, rel := range []string{"Letters/Zed.md", "Letters/Untitled 2.md", "Letters/TODO.md", "Letters/Untitled.md", "Letters/index.md"} {
		notes = append(notes, nav.NoteSummary{Title: "Same", RelPath: rel, Modified: stamp})
	}
	recent, _ := recentShelfNotes(notes, false, status.Authority{}, nil)
	got := make([]string, 0, len(recent))
	for _, n := range recent {
		got = append(got, n.RelPath)
	}
	want := []string{"Letters/index.md", "Letters/TODO.md", "Letters/Untitled.md", "Letters/Untitled 2.md", "Letters/Zed.md"}
	if !slices.Equal(got, want) {
		t.Errorf("caught: recent shelf tie order = %q, want %q", got, want)
	}
}
