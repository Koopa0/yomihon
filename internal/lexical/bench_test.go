package lexical

import (
	"fmt"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/render"
)

// BenchmarkSearch measures query matching against a built index. The index is
// built once from a fixed in-memory corpus — the pure, disk-free input the
// build accepts — so the loop times only the match, not the walk or the index
// build. The corpus and query are stable so the number is meaningful across
// runs of a single machine.
func BenchmarkSearch(b *testing.B) {
	idx := NewIndex(benchDocs(), validArtifactPolicy(b))
	q := Parse("kafka concept")
	b.ReportAllocs()
	for b.Loop() {
		if _, err := idx.Search(q, -1); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSearchResultMaterialization times building every result of the
// whole answer the search page asks for, over 537 notes of about 8 KiB that
// open with a d2 fence also holding the query. The notes are projected the
// way DocumentFromNote projects one, emphasis and links included, so a result
// takes the path that skips what the page hides; that path once tabulated
// every character of every note it measured.
func BenchmarkSearchResultMaterialization(b *testing.B) {
	const notes = 537
	body := "# Pipeline\n\n" +
		"```d2\n" +
		"direction: right\n" +
		"Source: \"source\\nowns jobs close\"\n" +
		"err := work(); if err != nil { return err }\n" +
		"```\n\n" +
		strings.Repeat("The *filler* paragraph stays out of the [[way]].\n\n", 120) +
		"A pipeline stage owns jobs and reports an err only after workers return.\n\n" +
		strings.Repeat("and more filler word ", 200)
	projection := render.PlainProjectionOfPage("Pipeline", body)
	docs := make([]Document, notes)
	for i := range docs {
		docs[i] = Document{
			RelPath:      fmt.Sprintf("Notes/n%03d.md", i),
			Title:        fmt.Sprintf("Note %03d", i),
			PlainText:    projection.Text,
			Blocks:       projection.Blocks,
			FenceRanges:  projection.FenceRanges,
			DisplaySpans: projection.DisplaySpans,
			Insertions:   projection.Insertions,
		}
	}
	idx := NewIndex(docs, validArtifactPolicy(b))
	for _, q := range []string{"err", "return"} {
		b.Run(q, func(b *testing.B) {
			query := Parse(q)
			b.ReportAllocs()
			for b.Loop() {
				if _, err := idx.Search(query, -1); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkNewIndex times the build a save pays: the scanner rebuilds on
// every change. The corpus is 1226 copies of two ASCII bodies, 597 of
// them carrying eight fences.
func BenchmarkNewIndex(b *testing.B) {
	docs := repeatedASCIIFenceDocs()
	policy := validArtifactPolicy(b)
	b.ReportAllocs()
	for b.Loop() {
		_ = NewIndex(docs, policy)
	}
}

// repeatedASCIIFenceDocs is the build-cost smoke corpus: 1226 notes,
// 597 of them the same ASCII body carrying eight fences, the rest the
// same unfenced ASCII body. It is not a live vault.
func repeatedASCIIFenceDocs() []Document {
	const notes = 1226
	const fenced = 597
	const fencesPer = 8
	var fencedBody strings.Builder
	chunk := strings.Repeat("The filler paragraph stays out of the way.\n\n", 15)
	for i := range fencesPer {
		fencedBody.WriteString(chunk)
		fencedBody.WriteString("```d2\n")
		fmt.Fprintf(&fencedBody, "fence %d holds err and return\n", i)
		fencedBody.WriteString("```\n\n")
	}
	fencedBody.WriteString(strings.Repeat("and more filler word ", 80))
	fencedText, fencedBlocks, fences := render.PlainBlocks(fencedBody.String())
	proseText, proseBlocks, _ := render.PlainBlocks(strings.Repeat(chunk, fencesPer) + strings.Repeat("and more filler word ", 80))
	docs := make([]Document, notes)
	for i := range docs {
		if i < fenced {
			docs[i] = Document{
				RelPath:     fmt.Sprintf("Notes/n%03d.md", i),
				Title:       fmt.Sprintf("Note %03d", i),
				PlainText:   fencedText,
				Blocks:      fencedBlocks,
				FenceRanges: fences,
			}
			continue
		}
		docs[i] = Document{
			RelPath:   fmt.Sprintf("Notes/n%03d.md", i),
			Title:     fmt.Sprintf("Note %03d", i),
			PlainText: proseText,
			Blocks:    proseBlocks,
		}
	}
	return docs
}

// benchDocs is a small fixed corpus spanning a few domains, one draft in three, with
// the query term present in some notes and absent in others, so matching does
// real work rather than trivially hitting or missing every entry.
func benchDocs() []Document {
	domains := []string{"golang", "japanese", "distributed", "rust", "database"}
	bodies := []string{
		"kafka partitions replicate across brokers for durability",
		"the kana syllabary maps sounds to written symbols",
		"a goroutine is a lightweight thread scheduled by the runtime",
		"ownership and borrowing enforce memory safety without a collector",
		"a b-tree index keeps range scans on sorted keys cheap",
	}
	docs := make([]Document, 0, len(bodies)*3)
	for i, body := range bodies {
		for j := range 3 {
			docs = append(docs, Document{
				RelPath:   "Concepts/" + domains[i] + "/note" + string(rune('A'+j)) + ".md",
				Title:     domains[i] + " concept " + string(rune('A'+j)),
				NoteType:  "concept",
				Domain:    domains[i],
				Draft:     j == 0,
				Slug:      domains[i] + "-" + string(rune('a'+j)),
				PlainText: body + " a concept note about " + domains[i],
			})
		}
	}
	return docs
}

// BenchmarkShownExcerptCentre separates source length from display-span count.
// The hidden-prefix case cannot allocate an atom for each hidden source byte.
func BenchmarkShownExcerptCentre(b *testing.B) {
	for _, size := range []int{4096, 1 << 20} {
		for _, shape := range []string{"hidden-prefix", "dense-spans"} {
			b.Run(fmt.Sprintf("%s/%d", shape, size), func(b *testing.B) {
				doc := shownExcerptFixture(shape, size)
				index := NewIndex([]Document{doc}, validArtifactPolicy(b))
				entry := index.entries[0]
				centre := entry.shownExcerptCentre([]string{"claim"})
				if !centre.found || entry.plainText[centre.start:centre.end] != "claim" {
					b.Fatal("benchmark must select the shown claim within the source ceiling")
				}
				b.ReportAllocs()
				for b.Loop() {
					_ = entry.shownExcerptCentre([]string{"claim"})
				}
			})
		}
	}
}

// shownExcerptFixture is a document of about size bytes whose only shown
// "claim" sits at its end. A hidden prefix is one span over a hidden "claim"
// and the filler after it; dense spans hide every tag of a long run of short
// formatted words, so the selection has thousands of spans to walk.
func shownExcerptFixture(shape string, size int) Document {
	const shown = " Our claim survives."
	if shape == "dense-spans" {
		var text strings.Builder
		var spans []render.DisplaySpan
		for range (size - 64) / 8 {
			at := text.Len()
			text.WriteString("<b>x</b>")
			spans = append(spans, render.DisplaySpan{Start: at, End: at + 3, Hidden: true}, render.DisplaySpan{Start: at + 4, End: at + 8, Hidden: true})
		}
		text.WriteString(shown)
		return Document{RelPath: "fixture.md", PlainText: text.String(), DisplaySpans: spans}
	}
	hidden := "claim" + strings.Repeat("x", size-64)
	return Document{
		RelPath:      "fixture.md",
		PlainText:    hidden + shown,
		DisplaySpans: []render.DisplaySpan{{Start: 0, End: len(hidden), Hidden: true}},
	}
}
