package note

import (
	"fmt"
	"net/http"
	"path"
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// compare shows two notes at once, the path naming the first and the query the
// second. It reads; nothing here writes, and neither column carries a control
// that could.
//
// The page needs two notes it can render. Where it has only one — the query
// named nothing, named this same note, or named something this generation
// cannot show as a note — the reader is sent to the note the path named, which
// answers for it exactly as it always does: a note, a stand-in page for another
// kind of file, or the page that says nothing is there. That keeps one
// classification in one place instead of two routes that can come to disagree.
func (h *Handler) compare(w http.ResponseWriter, r *http.Request) {
	lang := origin.Language(r)
	rel := vault.NormalizeNFC(r.PathValue("path"))
	authority := h.sources.Status()
	snap := h.sources.Snapshot().Capture()
	if !servable(rel) {
		h.showNotFound(w, r, r.URL.Path, authority, snap)
		return
	}
	with := vault.NormalizeNFC(r.URL.Query().Get(pages.CompareWithParam))
	a, aOK := readableNote(snap, rel)
	b, bOK := readableNote(snap, with)
	if !aOK || !bOK || with == rel {
		h.readAlone(w, r, rel)
		return
	}

	viewA, refsA := h.reading(r, rel, &a, snap, authority, lang, comparePrefixA)
	viewB, refsB := h.reading(r, with, &b, snap, authority, lang, comparePrefixB)
	view := pages.CompareView{
		A: viewA,
		B: viewB,
		// One set of sheets for the page: a concept both lessons cite is one
		// document, cited twice.
		Concepts: loadConcepts(snap, mergedConceptRefs(refsA, refsB), lang),
	}

	title := fmt.Sprintf(wording.CompareTitleFmt.In(lang), a.Title, b.Title)
	if err := pages.Compare(view, layouts.ChromeFromRequest(r, title)).Render(r.Context(), w); err != nil {
		h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write compare page",
			"path", rel, "with", with, "error", err)
	}
}

// The two id spaces a compare page hands out, one per column, so the places
// inside one note never answer for the other's. They are short because every
// name inside a column carries one.
const (
	comparePrefixA = "a-"
	comparePrefixB = "b-"
)

// readAlone sends a reader who asked for a pair to the one note that is
// certainly there. Nothing about the answer is kept: a note the vault does not
// hold yet is one somebody may be writing now, and the pair that could not be
// shown this second can be readable on the next reload.
func (h *Handler) readAlone(w http.ResponseWriter, r *http.Request, rel string) {
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, pages.VaultHref("/notes/", rel), http.StatusFound)
}

// readableNote is one note this generation can render as an article: a markdown
// path the contract did not tell the scan to skip, within the bound a body is
// read up to, and captured with a body to show. It is the same set of questions
// the note's own page asks before it renders one, asked here so a column is
// never opened over a file that has no article.
func readableNote(snap *snapshot.Generation, rel string) (snapshot.Reading, bool) {
	if rel == "" || !servable(rel) || !vault.IsMarkdown(rel) || snap.SkipsNote(rel) {
		return snapshot.Reading{}, false
	}
	if entry, isFile := snap.Entry(rel); isFile && entry.Size() > render.MaxSourceBytes {
		return snapshot.Reading{}, false
	}
	return snap.Note(rel)
}

// mergedConceptRefs is the concepts the page cites, each once, in the order the
// two columns reach them. The order decides which sheet is rendered in which
// region, so it comes from the page rather than from a map.
func mergedConceptRefs(first, second []string) []string {
	if len(second) == 0 {
		return first
	}
	merged := slices.Clone(first)
	for _, ref := range second {
		if !slices.Contains(merged, ref) {
			merged = append(merged, ref)
		}
	}
	return merged
}

// pairOffer is the one other note this one is offered to be read beside, or the
// zero value where none is.
//
// Two relations can name a partner, and they are not equals. A declaration is
// the author's own claim: either note wrote the other in based_on, and the value
// placed exactly one note. A shared filename is this program's inference: the
// two sit in one folder, one filename opens with the other's, and the two
// declare different languages — which is what a translation looks like from
// outside. So a declaration wins wherever there is exactly one, and the
// inference answers only where no declaration does.
//
// Either way, more than one candidate is no offer. Choosing between two would
// be a guess, and this surface never picks one of several names; a reader can
// still type the address for the pair they meant.
func pairOffer(snap *snapshot.Generation, model *nav.Model, n *snapshot.Reading) nav.NoteRef {
	if snap == nil || n == nil || n.RelPath == "" {
		return nav.NoteRef{}
	}
	declared := declaredPartners(snap, n.RelPath)
	if len(declared) > 0 {
		if len(declared) == 1 {
			return declared[0]
		}
		return nav.NoteRef{}
	}
	if inferred := translatedSiblings(model, n); len(inferred) == 1 {
		return inferred[0]
	}
	return nav.NoteRef{}
}

// declaredPartners is every note a based_on declaration ties to this one, in
// either direction and each named once: the sources this note declared, then
// the notes that declared this one. Only a value that placed exactly one note
// is a partner — an unresolved or ambiguous one is the author's own text, and
// this page never turns text into an address.
func declaredPartners(snap *snapshot.Generation, rel string) []nav.NoteRef {
	partners := make([]nav.NoteRef, 0, 2)
	seen := map[string]bool{rel: true}
	for _, ref := range slices.Concat(snap.BasedOn(rel), snap.BasedOnBy(rel)) {
		if ref.RelPath == "" || seen[ref.RelPath] {
			continue
		}
		seen[ref.RelPath] = true
		if partner, ok := snap.Note(ref.RelPath); ok {
			ref.Language = partner.Language
		}
		partners = append(partners, ref)
	}
	return partners
}

// translatedSiblings is every note in this one's folder whose filename opens
// with this one's, or whose own filename this one opens with, and that declares
// a different language. The filename is the stem because a title is not a name
// anything in this vault resolves by, and the declared language is what tells a
// translation from a note that merely starts with the same words — so no list of
// language suffixes is invented here for a vault to keep in step with.
func translatedSiblings(model *nav.Model, n *snapshot.Reading) []nav.NoteRef {
	if model == nil || n.Language == "" {
		return nil
	}
	stem := noteStem(n.RelPath)
	if stem == "" {
		return nil
	}
	_, siblings := model.Siblings(n.RelPath)
	var translated []nav.NoteRef
	for _, sibling := range siblings {
		if sibling.RelPath == n.RelPath || sibling.Language == "" || sibling.Language == n.Language {
			continue
		}
		other := noteStem(sibling.RelPath)
		if other == "" || (!strings.HasPrefix(other, stem) && !strings.HasPrefix(stem, other)) {
			continue
		}
		translated = append(translated, sibling)
	}
	return translated
}

// noteStem is the filename a wikilink would resolve by, folded the way every
// reader of this vault folds a name.
func noteStem(relPath string) string {
	return graph.NormalizeKey(strings.TrimSuffix(path.Base(relPath), ".md"))
}
