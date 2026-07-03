// Package search is the vault's in-memory search index and query engine
// (docs/spec.md §3, docs/search-plan.md). It holds one entry per note — the
// few fields search reads (D23) — and answers a deterministic, NFC-folded
// substring query plus six structured filters. It owns the index and the
// query; it does NOT own freshness: the index is one of three models in the
// shared vault Snapshot (internal/snapshot), rebuilt from the vault on change
// (D24, D25). There is no database and no persistent state — the truth is the
// vault files, the index only accelerates (§0.1).
package search

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/koopa0/kurodo/internal/graph"
	"github.com/koopa0/kurodo/internal/render"
	"github.com/koopa0/kurodo/internal/vault"
)

// fold is the single definer of "what counts as a match" (docs/search-plan.md
// §4): NFC then lowercase, applied identically to stored text and to a query
// token. Case folding lives only here — graph.NormalizeNFC supplies the shared
// NFC step so there is no second NFC in the repo (CLAUDE.md predictable-mistake
// #2).
func fold(s string) string {
	return strings.ToLower(graph.NormalizeNFC(s))
}

// Doc is the pure, disk-free input to BuildFromDocs: everything the index needs
// about one note, with PlainText already extracted (render.PlainText). Keeping
// the build pure makes it unit-testable without a vault on disk.
type Doc struct {
	RelPath   string
	Title     string
	NoteType  string
	Domain    string
	Status    string
	Slug      string
	Topics    []string
	PlainText string
}

// entry is one indexed note. The *Fold copies double the note's text in memory
// (a few MB across the corpus) to buy a zero-config, allocation-free match at
// query time — Title/PlainText keep their display form (NFC, original case),
// the *Fold fields are fold()ed for matching, and the structured field values
// are stored NFC-but-case-preserving so a filter is an exact selection of a
// canonical value (docs/search-plan.md §3, R3). There is no content hash:
// change detection is the scanner's job by mtime (§8).
type entry struct {
	RelPath   string
	Title     string
	TitleFold string
	NoteType  string
	Domain    string
	Status    string
	Slug      string
	Topics    []string
	PlainText string
	PlainFold string
}

// Index is the whole in-memory search index: entries kept sorted by RelPath so
// each result bucket is naturally rel_path-ordered without a sort call
// (docs/search-plan.md §6). Read-only once built. Entries are held by pointer
// so a query iterates 8-byte pointers rather than copying each ~180-byte entry.
type Index struct {
	entries []*entry
}

// BuildFromDocs builds an Index from already-extracted note data, with no disk
// access — the pure indexing logic Build delegates to. Entries are sorted by
// RelPath at build time, which is the sole source of result ordering.
func BuildFromDocs(docs []Doc) *Index {
	entries := make([]*entry, 0, len(docs))
	for i := range docs {
		e := entryFromDoc(&docs[i])
		entries = append(entries, &e)
	}
	slices.SortFunc(entries, func(a, b *entry) int {
		return cmp.Compare(a.RelPath, b.RelPath)
	})
	return &Index{entries: entries}
}

// entryFromDoc derives one entry from a Doc, applying the storage rules of
// docs/search-plan.md §3/§4: Title/PlainText stored NFC (display), the *Fold
// copies fold()ed, the structured field values stored NFC (case-preserving).
func entryFromDoc(d *Doc) entry {
	title := graph.NormalizeNFC(d.Title)
	plain := graph.NormalizeNFC(d.PlainText)
	topics := make([]string, len(d.Topics))
	for i, t := range d.Topics {
		topics[i] = graph.NormalizeNFC(t)
	}
	return entry{
		RelPath:   d.RelPath,
		Title:     title,
		TitleFold: fold(title),
		NoteType:  graph.NormalizeNFC(d.NoteType),
		Domain:    graph.NormalizeNFC(d.Domain),
		Status:    graph.NormalizeNFC(d.Status),
		Slug:      graph.NormalizeNFC(d.Slug),
		Topics:    topics,
		PlainText: plain,
		PlainFold: fold(plain),
	}
}

// Build walks root (via vault.List) and indexes every markdown note. A note
// whose read fails is skipped rather than aborting the whole build (wall 4):
// one bad file must not narrow what the rest of the index can find. It is the
// disk entry point the Snapshot rebuild calls.
func Build(root string) (*Index, error) {
	paths, err := vault.List(root)
	if err != nil {
		return nil, fmt.Errorf("search: build index: %w", err)
	}
	var docs []Doc
	for _, p := range paths {
		if !strings.HasSuffix(p, ".md") {
			continue
		}
		n, readErr := vault.ReadNote(root, p)
		if readErr != nil {
			continue // best-effort: a vanished/unreadable note drops out, the rest stand
		}
		docs = append(docs, docFromNote(n))
	}
	return BuildFromDocs(docs), nil
}

// docFromNote extracts a Doc from a parsed note: the structured fields from
// frontmatter and PlainText from the render AST. A note with malformed
// frontmatter (Frontmatter == nil) simply contributes empty structured fields;
// its body text is still indexed.
func docFromNote(n *vault.Note) Doc {
	return Doc{
		RelPath:   n.RelPath,
		Title:     n.Title(),
		NoteType:  n.Type(),
		Domain:    stringField(n, "domain"),
		Status:    n.Status(),
		Slug:      stringField(n, "slug"),
		Topics:    stringsField(n, "topics"),
		PlainText: render.PlainText(n.Body),
	}
}

// Len reports how many notes are indexed.
func (idx *Index) Len() int {
	return len(idx.entries)
}

// stringField reads a string frontmatter value, empty when absent or not a
// string (wall 4: a malformed field costs that field, never the build).
func stringField(n *vault.Note, key string) string {
	if v, ok := n.Frontmatter[key].(string); ok {
		return v
	}
	return ""
}

// stringsField reads a list-of-strings frontmatter value (e.g. topics),
// tolerating any non-list-of-strings shape by returning nil.
func stringsField(n *vault.Note, key string) []string {
	raw, ok := n.Frontmatter[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
