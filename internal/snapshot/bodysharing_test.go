package snapshot

import (
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// Every body product belongs to the same captured source. Borrowing its
// recognition must preserve the whole result and avoid three duplicate reads.
func TestSnapshotBodyProductsShareRecognition(t *testing.T) {
	var fields []string
	for field := range reflect.TypeFor[noteProducts]().Fields() {
		fields = append(fields, field.Name)
	}
	if diff := cmp.Diff([]string{"document", "planned", "links"}, fields); diff != "" {
		t.Fatalf("caught: body product ownership changed (-want +got):\n%s", diff)
	}
	const path = "Notes/Source.md"
	body := "# Source\n\n" + strings.Repeat("## Heading\n\n[[A|Alias]] [manual](Manual.md) `[[Quoted]]`\n\n%% [[Hidden]] %%\n\n- [[B]]\n  - [[C]]\n\n", 16)
	data := []byte("---\ntitle: Source\naliases: [Other]\n---\n" + body)
	lint, err := judge.NewFrontmatterLinter(nil)
	if err != nil {
		t.Fatal(err)
	}
	independent := func() noteRead {
		parsed := vault.Parse(path, data)
		return noteRead{
			parsed: parsed, reading: newReading(parsed, data, schema.ArticleLanguage{}),
			findings: lint.Lint(path, data),
			products: noteProducts{
				document: lexical.DocumentFromNote(parsed),
				planned:  judge.NewPlanned(slices.Values([]string{parsed.Body}), nil),
				links:    judge.LinkTargets(parsed.Body),
			},
		}
	}
	shared := func() noteRead {
		read, fault := deriveNote(path, data, schema.ArticleLanguage{}, nil, lint, nil)
		if fault != nil {
			t.Fatalf("not-applied: body product read panicked: %s", fault.value)
		}
		return read
	}
	if diff := cmp.Diff(independent(), shared(), cmp.AllowUnexported(noteRead{}, noteProducts{}, judge.Planned{}, vault.Note{})); diff != "" {
		t.Fatalf("caught: snapshot body products differ (-want +got):\n%s", diff)
	}
	separate := testing.AllocsPerRun(100, func() { runtime.KeepAlive(independent()) })
	borrowed := testing.AllocsPerRun(100, func() { runtime.KeepAlive(shared()) })
	required := testing.AllocsPerRun(100, func() {
		for range 3 {
			runtime.KeepAlive(graph.ReadBody(body))
		}
	})
	if saved := separate - borrowed; required <= 0 || saved < required {
		t.Errorf("caught: snapshot body recognition repeated separate=%g borrowed=%g saved=%g required-three-reads=%g", separate, borrowed, saved, required)
	}
}
