package snapshot

import (
	"testing"
)

const indexLanguageRel = "Writing/lessons/japanese/L01.md"

// TestSearchIndexCarriesDeclaredArticleLanguage holds the snapshot write that
// teaches the lexical index a note's declared language. A search result is
// the observable surface: the indexed document must carry the tag before any
// handler or template can stamp it onto a listing row.
func TestSearchIndexCarriesDeclaredArticleLanguage(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNote(t, root, indexLanguageRel, `---
title: L01 わたしは学生です
type: lesson
domain: japanese
status: draft
slug: jp-l01
lang: ja
created: 2026-06-01
updated: 2026-06-01
---

わたしは学生です
`)
	store, _ := newTestStore(t, root, testContract(t, root))

	results := snapshotSearch(t, store.Current().Search(), "L01")
	if len(results) != 1 {
		t.Fatalf("Search(L01) = %d results, want 1", len(results))
	}
	if results[0].RelPath != indexLanguageRel {
		t.Fatalf("Search(L01) path = %q, want %q", results[0].RelPath, indexLanguageRel)
	}
	if results[0].Language != "ja" {
		t.Fatalf("indexed language = %q, want ja", results[0].Language)
	}
}
