package note

import (
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/vault"
)

const healthLanguageRel = "Writing/lessons/japanese/L01.md"

// TestHealthCarriesDeclaredArticleLanguage locks every health carry site that
// stamps a declared article language onto a note name: unwritten links,
// title-only links, batched refs, and the inline ref schemaFaultLists builds
// before the page lists frontmatter faults.
func TestHealthCarriesDeclaredArticleLanguage(t *testing.T) {
	t.Parallel()

	ref := nav.NoteRef{Name: "L01", RelPath: healthLanguageRel}
	articleLang := func(relPath string) string {
		if relPath == healthLanguageRel {
			return "ja"
		}
		return ""
	}

	for _, tc := range []struct {
		name string
		got  func(t *testing.T) string
	}{
		{
			name: "healthLinks",
			got: func(t *testing.T) string {
				t.Helper()
				out := healthLinks([]snapshot.HealthLink{{From: ref, Target: "Ghost"}}, articleLang)
				if len(out) != 1 {
					t.Fatalf("healthLinks() = %d links, want 1", len(out))
				}
				return out[0].From.Language
			},
		},
		{
			name: "healthTitleLinks",
			got: func(t *testing.T) string {
				t.Helper()
				out := healthTitleLinks([]snapshot.HealthTitleLink{{From: ref, Target: "Ghost", Note: ref}}, articleLang)
				if len(out) != 1 {
					t.Fatalf("healthTitleLinks() = %d links, want 1", len(out))
				}
				if out[0].From.Language != "ja" {
					t.Errorf("healthTitleLinks() From language = %q, want ja", out[0].From.Language)
				}
				return out[0].Note.Language
			},
		},
		{
			name: "noteRefs",
			got: func(t *testing.T) string {
				t.Helper()
				out := noteRefs([]nav.NoteRef{ref}, articleLang)
				if len(out) != 1 {
					t.Fatalf("noteRefs() = %d refs, want 1", len(out))
				}
				return out[0].Language
			},
		},
		{
			name: "schemaFaultLists",
			got: func(t *testing.T) string {
				t.Helper()
				_, faults := schemaFaultLists(healthLanguageSchemaSnapshot(t))
				if len(faults) != 1 {
					t.Fatalf("schemaFaultLists() faults = %d, want 1", len(faults))
				}
				if faults[0].RelPath != healthLanguageRel {
					t.Fatalf("schemaFaultLists() path = %q, want %q", faults[0].RelPath, healthLanguageRel)
				}
				return faults[0].Language
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.got(t); got != "ja" {
				t.Errorf("%s language = %q, want ja", tc.name, got)
			}
		})
	}
}

func healthLanguageSchemaSnapshot(t *testing.T) *snapshot.Generation {
	t.Helper()
	root := t.TempDir()
	writeNote(t, root, healthLanguageRel, `---
title: L01
type: memorandum
domain: golang
status: draft
lang: ja
created: 2026-06-01
updated: 2026-06-01
---

body
`)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	contract, err := schema.LoadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("schema.LoadFile() error = %v", err)
	}
	store, err := snapshot.New(t.Context(), reader, slog.New(slog.DiscardHandler), contract, contract.Governance())
	if err != nil {
		t.Fatalf("snapshot.New() error = %v", err)
	}
	view := store.Current()
	if view == nil {
		t.Fatal("store.Current() = nil, want a built generation")
	}
	return view
}
