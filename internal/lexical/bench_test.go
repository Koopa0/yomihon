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
		_, _, _ = idx.SearchN(q, -1) //nolint:errcheck // benchmark fixture is validated before timing; only ranking cost is measured
	}
}

// BenchmarkSearchResultMaterialization times building the 200-row page the
// handler ships. Landing used to retabulate every character of every note;
// the number is here so a fold-map coming back is visible as allocation,
// not as a later review.
func BenchmarkSearchResultMaterialization(b *testing.B) {
	const notes = 200
	body := "```d2\nneedle in a fence first\n```\n\n" +
		strings.Repeat("The filler paragraph stays out of the way.\n\n", 40) +
		"needle sits here\n\nand more " + strings.Repeat("filler word ", 200)
	text, ends, fences := render.PlainBlocks(body)
	docs := make([]Document, notes)
	for i := range docs {
		docs[i] = Document{
			RelPath:     fmt.Sprintf("Notes/n%03d.md", i),
			Title:       fmt.Sprintf("Note %03d", i),
			PlainText:   text,
			BlockEnds:   ends,
			FenceRanges: fences,
		}
	}
	idx := NewIndex(docs, validArtifactPolicy(b))
	q := Parse("needle")
	b.ReportAllocs()
	for b.Loop() {
		_, _, err := idx.SearchN(q, notes)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSearchLiveVaultSizedFences times the 200-row materialization the
// handler ships, over a corpus the size of the live vault that produced the
// review numbers (537 notes, many of them carrying a d2 fence that also
// holds the query). The queries are the ones the review timed: err, return.
func BenchmarkSearchLiveVaultSizedFences(b *testing.B) {
	idx := NewIndex(liveVaultSizedFenceDocs(), validArtifactPolicy(b))
	for _, q := range []string{"err", "return"} {
		b.Run(q, func(b *testing.B) {
			query := Parse(q)
			b.ReportAllocs()
			for b.Loop() {
				_, _, err := idx.SearchN(query, 200)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func liveVaultSizedFenceDocs() []Document {
	const notes = 537
	fence := "" +
		"```d2\n" +
		"direction: right\n" +
		"Source: \"source\\nowns jobs close\"\n" +
		"err := work(); if err != nil { return err }\n" +
		"```\n"
	prose := "A pipeline stage owns jobs and reports an err only after workers return.\n"
	// ~8 KiB/note: foldRunes still walks every character on each fence
	// occurrence, so a short fixture cannot show the live-vault cost.
	body := fence + strings.Repeat("The filler paragraph stays out of the way.\n\n", 120) + prose +
		strings.Repeat("and more filler word ", 200)
	text, ends, fences := render.PlainBlocks(body)
	docs := make([]Document, notes)
	for i := range docs {
		docs[i] = Document{
			RelPath:     fmt.Sprintf("Notes/n%03d.md", i),
			Title:       fmt.Sprintf("Note %03d", i),
			PlainText:   text,
			BlockEnds:   ends,
			FenceRanges: fences,
		}
	}
	return docs
}

// benchDocs is a small fixed corpus spanning a few domains and statuses, with
// the query term present in some notes and absent in others, so matching does
// real work rather than trivially hitting or missing every entry.
func benchDocs() []Document {
	domains := []string{"golang", "japanese", "distributed", "rust", "database"}
	statuses := []string{"seed", "growing", "evergreen"}
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
				Status:    statuses[j],
				Slug:      domains[i] + "-" + string(rune('a'+j)),
				Topics:    []string{domains[i], "reference"},
				PlainText: body + " a concept note about " + domains[i],
			})
		}
	}
	return docs
}
