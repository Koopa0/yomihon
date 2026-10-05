package render_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestHeadingPathLinksLandOnTheNamedChild(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, body, heading, id string
		missing                 bool
	}{
		{name: "child", body: "## A\n### B\n", heading: "A#B", id: "b"},
		{name: "second parent", body: "## A\n### B\n## Other\n### B\n", heading: "Other#B", id: "b-2"},
		{name: "occupied suffix", body: "## A\n### B\n### B 2\n## Other\n### B\n", heading: "Other#B", id: "b-3"},
		{name: "repeated whole path", body: "## A\n### B\n## A\n### B\n", heading: "A#B", id: "b"},
		{name: "literal trailing hash", body: "## A#\n", heading: "A#", id: "a"},
		{name: "skipped levels", body: "## A\n###### B\n", heading: "A#B", id: "b"},
		{name: "omitted ancestor", body: "## A\n### Middle\n#### B\n", heading: "A#B", id: "b"},
		{name: "all ancestors", body: "## A\n### Middle\n#### B\n", heading: "A#Middle#B", id: "b"},
		{name: "wrong order", body: "## A\n### Middle\n#### B\n", heading: "Middle#A#B", id: "middle-a-b", missing: true},
		{name: "wrong parent", body: "## A\n### B\n## Other\n### C\n", heading: "A#C", id: "a-c", missing: true},
		{name: "closed parent", body: "## A\n## Other\n### B\n", heading: "A#B", id: "a-b", missing: true},
		{name: "empty component", body: "## A\n### B\n", heading: "A##B", id: "a-b", missing: true},
		{name: "folded names", body: "## がん\n### Résumé\n", heading: "か\u3099ん#RE\u0301SUME\u0301", id: "résumé"},
		{name: "setext", body: "A\n===\n\nB\n---\n", heading: "A#B", id: "b"},
		{name: "quote", body: "> ## A\n> ### B\n", heading: "A#B", id: "b"},
		{name: "list", body: "- ## A\n\n  ### B\n", heading: "A#B", id: "b"},
		{name: "displayed inline words", body: "## *A*\n### [B](https://example.invalid/)\n", heading: "A#B", id: "b"},
		{name: "role and ruby", body: "## A {sequence=primary}\n### <ruby>字<rt>じ</rt></ruby>\n", heading: "A#字", id: "字"},
		{name: "fenced child", body: "## A\n```\n### B\n```\n", heading: "A#B", id: "a-b", missing: true},
		{name: "hidden child", body: "## A\n%%\n### B\n%%\n", heading: "A#B", id: "a-b", missing: true},
		{name: "cycle", body: "## A\n### B\n[[N#A#B]]\n", heading: "A#B", id: "b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRenderer(t, []graph.NoteInput{{RelPath: "N.md"}}, nil, transclusions{"N.md": tt.body})
			got := r.HTML("citer.md", "", "[[N#"+tt.heading+"|child]]", wording.ZhHant)
			class := "wikilink"
			var want []render.DiagnosticKind
			if tt.missing {
				class += " wikilink-degraded"
				want = []render.DiagnosticKind{render.DiagLinkSectionMissing}
			}
			link := `<a href="/notes/N.md#` + tt.id + `" class="` + class + `"`
			if !strings.Contains(got.HTML, link) {
				t.Errorf("HTML(%q) missing %q:\n%s", tt.heading, link, got.HTML)
			}
			var kinds []render.DiagnosticKind
			for _, d := range got.Diagnostics {
				kinds = append(kinds, d.Kind)
			}
			if diff := cmp.Diff(want, kinds); diff != "" {
				t.Errorf("HTML(%q).Diagnostics (-want +got):\n%s", tt.heading, diff)
			}
			if !tt.missing {
				destination := r.HTML("N.md", "", tt.body, wording.ZhHant)
				if !strings.Contains(destination.HTML, ` id="`+tt.id+`"`) {
					t.Errorf("destination lacks link's id %q:\n%s", tt.id, destination.HTML)
				}
			}
		})
	}
}

func TestHeadingPathLeavesUnresolvedAndUncapturedTargetsAlone(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, target string
		notes        []graph.NoteInput
		files        []string
		want         string
	}{
		{name: "uncaptured", notes: []graph.NoteInput{{RelPath: "N.md"}}, want: `<a href="/notes/N.md#a-b" class="wikilink">child</a>`},
		{name: "non markdown", target: "N.txt", files: []string{"N.txt"}, want: `<a href="/notes/N.txt" class="wikilink">child</a>`},
		{name: "ambiguous", notes: []graph.NoteInput{{RelPath: "One/N.md"}, {RelPath: "Two/N.md"}}, want: "wikilink-ambiguous"},
		{name: "missing", want: "wikilink-broken"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			target := tt.target
			if target == "" {
				target = "N"
			}
			got := newRenderer(t, tt.notes, tt.files, nil).HTML("citer.md", "", "[["+target+"#A#B|child]]", wording.En)
			if !strings.Contains(got.HTML, tt.want) {
				t.Errorf("HTML() missing %q:\n%s", tt.want, got.HTML)
			}
			if ds := fragmentDiagnostics(&got); len(ds) != 0 {
				t.Errorf("HTML().section diagnostics = %q, want none without one captured note", ds)
			}
		})
	}
}

func TestHeadingPathPreservesTheAuthoredPreviewAddress(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ body, heading, want string }{
		{body: "## First\n### B\n## Other\n### B\n", heading: "Other#B", want: `<a href="/notes/N.md#b-2" class="wikilink" data-preview-section="Other#B">child</a>`},
		{body: "## A & C\n### B \"word\"\n", heading: "A & C#B \"word\"", want: `<a href="/notes/N.md#b-word" class="wikilink" data-preview-section="A &amp; C#B &#34;word&#34;">child</a>`},
	} {
		t.Run(tt.heading, func(t *testing.T) {
			t.Parallel()
			r := newRenderer(t, []graph.NoteInput{{RelPath: "N.md"}}, nil, transclusions{"N.md": tt.body})
			got := r.HTML("citer.md", "", "[[N#"+tt.heading+"|child]]", wording.En)
			if !strings.Contains(got.HTML, tt.want) {
				t.Errorf("HTML() lost the authored preview path or its escaping; want %q:\n%s", tt.want, got.HTML)
			}
		})
	}
}

func TestHeadingPathIncludesOneLevelOfTheDisplayedTransclusion(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, embed, id string
		missing         bool
	}{
		{name: "whole body", embed: "![[Child]]", id: "b-2"},
		{name: "scoped child", embed: "![[Child#B]]", id: "b-2"},
		{name: "outside excerpt", embed: "![[Child#C]]", id: "a-b", missing: true},
		{name: "nested embed withheld", embed: "![[Nested]]", id: "a-b", missing: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			bodies := transclusions{"N.md": "## Other\n### B\n## A\n" + tt.embed + "\n", "Child.md": "### B\nCHILD\n### C\nOTHER\n", "Nested.md": "![[Child]]\n"}
			r := newRenderer(t, []graph.NoteInput{{RelPath: "N.md"}, {RelPath: "Child.md"}, {RelPath: "Nested.md"}}, nil, bodies)
			got := r.HTML("citer.md", "", "[[N#A#B|child]]", wording.En)
			class := "wikilink"
			if tt.missing {
				class += " wikilink-degraded"
			}
			want := `<a href="/notes/N.md#` + tt.id + `" class="` + class + `"`
			if !strings.Contains(got.HTML, want) {
				t.Errorf("HTML() missing %q:\n%s", want, got.HTML)
			}
		})
	}
}

func TestHeadingPathExcerptsChooseTheFirstMatchingBranch(t *testing.T) {
	t.Parallel()
	body := "## A\n### B\nFIRST\n## Other\n### B\nSECOND\n## Other\n### B\nTHIRD\n"
	for _, tt := range []struct{ heading, want string }{
		{heading: "A#B", want: "### B\nFIRST"},
		{heading: "Other#B", want: "### B\nSECOND"},
		{heading: "A#C", want: ""},
		{heading: "Other#A#B", want: ""},
	} {
		t.Run(tt.heading, func(t *testing.T) {
			t.Parallel()
			got, found := render.Excerpt(body, tt.heading)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Excerpt(%q) (-want +got):\n%s", tt.heading, diff)
			}
			if found != (tt.want != "") {
				t.Errorf("Excerpt(%q).found = %t, want %t", tt.heading, found, tt.want != "")
			}
		})
	}
}
