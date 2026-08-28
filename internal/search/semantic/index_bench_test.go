package semantic

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

const (
	benchmarkSemanticChunks        = 6_496
	benchmarkSemanticChunksPerNote = 13
)

var benchmarkSemanticRanks []Result

func BenchmarkSemanticTopK(b *testing.B) {
	for _, dimension := range []int{1_536, 3_072} {
		b.Run(fmt.Sprintf("dim-%d", dimension), func(b *testing.B) {
			rows := benchmarkChunkVectors(b, benchmarkSemanticChunks, dimension)
			index, err := newOwnedIndex(rows, dimension)
			if err != nil {
				b.Fatal(err)
			}
			query := make([]float32, dimension)
			query[0] = 1

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				benchmarkSemanticRanks, err = index.TopNotes(query, nil, 50)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func benchmarkChunkVectors(b *testing.B, chunks, dimension int) []ChunkVector {
	b.Helper()
	rows := make([]ChunkVector, chunks)
	for i := range rows {
		note := i / benchmarkSemanticChunksPerNote
		relPath := fmt.Sprintf("Writing/benchmark-%04d.md", note)
		vector := make([]float32, dimension)
		vector[i%dimension] = 1
		row := ChunkVector{
			RelPath:       relPath,
			NoteHash:      sha256.Sum256([]byte(relPath)),
			Ordinal:       uint32(i % benchmarkSemanticChunksPerNote), // #nosec G115 -- fixed benchmark modulus is non-negative and below uint32
			SubmittedHash: sha256.Sum256(fmt.Appendf(nil, "%s:%d", relPath, i)),
			Vector:        vector,
		}
		rows[i] = bindChunkVector(&row)
	}
	return rows
}
