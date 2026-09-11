package snapshot

import (
	"testing"
)

// TestSearchIndexCarriesDeclaredArticleLanguage locks the only place the
// search index learns a note's language: indexDocuments copies the resolved
// BCP 47 tag onto each lexical.Document before the index is built.
func TestSearchIndexCarriesDeclaredArticleLanguage(t *testing.T) {
	t.Parallel()

	const rel = "Writing/lessons/japanese/L01.md"
	root := t.TempDir()
	writeNote(t, root, rel, `---
title: L01
type: lesson
domain: japanese
status: draft
lang: ja
created: 2026-06-01
updated: 2026-06-01
---

body
`)
	contract := testContract(t, root)
	store, _ := newTestStore(t, root, contract)
	gen := store.Current()
	if gen == nil {
		t.Fatal("store.Current() = nil, want a built generation")
	}

	results := snapshotSearch(t, gen.Search(), "L01")
	if len(results) != 1 {
		t.Fatalf("Search(L01) = %d results, want 1", len(results))
	}
	if results[0].RelPath != rel {
		t.Fatalf("Search(L01) path = %q, want %q", results[0].RelPath, rel)
	}
	if results[0].Language != "ja" {
		t.Errorf("indexed language = %q, want ja", results[0].Language)
	}
}
