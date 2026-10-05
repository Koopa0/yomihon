package graph_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
)

func TestMarkdownResolutionNamesAllInterpretations(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name        string
		owner       string
		destination string
		files       []string
		want        graph.MarkdownResolution
	}{
		{name: "three interpretations one file", owner: "Notes/here.md", destination: "Other.md", files: []string{"Notes/Other.md"}, want: graph.MarkdownResolution{Resolution: graph.Resolution{Kind: graph.KindUnique, RelPath: "Notes/Other.md"}, Local: true, Relative: "Notes/Other.md"}},
		{name: "repeated capture one file", owner: "Notes/here.md", destination: "Other.md", files: []string{"Notes/Other.md", "Notes/Other.md"}, want: graph.MarkdownResolution{Resolution: graph.Resolution{Kind: graph.KindUnique, RelPath: "Notes/Other.md"}, Local: true, Relative: "Notes/Other.md"}},
		{name: "relative root and shortest all differ", owner: "Notes/here.md", destination: "Other.md", files: []string{"Notes/Other.md", "Other.md", "Else/Other.md"}, want: graph.MarkdownResolution{Resolution: graph.Resolution{Kind: graph.KindAmbiguous, Candidates: []string{"Else/Other.md", "Notes/Other.md", "Other.md"}}, Local: true, Relative: "Notes/Other.md"}},
		{name: "qualified suffix", owner: "Inbox/here.md", destination: "DB/Other.md", files: []string{"Area/DB/Other.md", "Area/NotDB/Other.md"}, want: graph.MarkdownResolution{Resolution: graph.Resolution{Kind: graph.KindUnique, RelPath: "Area/DB/Other.md"}, Local: true, Relative: "Inbox/DB/Other.md"}},
		{name: "qualified missing is not a basename", owner: "Inbox/here.md", destination: "Absent/Other.md", files: []string{"Area/Other.md"}, want: graph.MarkdownResolution{Local: true, Relative: "Inbox/Absent/Other.md"}},
		{name: "encoded delimiter remains path", owner: "Inbox/here.md", destination: "A%23B%3FC.md?at=1#part", files: []string{"Area/A#B?C.md"}, want: graph.MarkdownResolution{Resolution: graph.Resolution{Kind: graph.KindUnique, RelPath: "Area/A#B?C.md"}, Local: true, Relative: "Inbox/A#B?C.md", Suffix: "?at=1#part"}},
		{name: "decode once", owner: "Inbox/here.md", destination: "A%2520B.md", files: []string{"Area/A%20B.md", "Area/A B.md"}, want: graph.MarkdownResolution{Resolution: graph.Resolution{Kind: graph.KindUnique, RelPath: "Area/A%20B.md"}, Local: true, Relative: "Inbox/A%20B.md"}},
		{name: "literal plus", owner: "Inbox/here.md", destination: "A+B.md", files: []string{"Area/A+B.md", "Area/A B.md"}, want: graph.MarkdownResolution{Resolution: graph.Resolution{Kind: graph.KindUnique, RelPath: "Area/A+B.md"}, Local: true, Relative: "Inbox/A+B.md"}},
		{name: "qualified colon filename", owner: "Inbox/here.md", destination: "Docs/a%3Ab.md", files: []string{"Docs/a:b.md"}, want: graph.MarkdownResolution{Resolution: graph.Resolution{Kind: graph.KindUnique, RelPath: "Docs/a:b.md"}, Local: true, Relative: "Inbox/Docs/a:b.md"}},
		{name: "qualified literal colon filename", owner: "Inbox/here.md", destination: "Docs/a:b.md", files: []string{"Docs/a:b.md"}, want: graph.MarkdownResolution{Resolution: graph.Resolution{Kind: graph.KindUnique, RelPath: "Docs/a:b.md"}, Local: true, Relative: "Inbox/Docs/a:b.md"}},
		{name: "URI scheme remains nonlocal", owner: "Inbox/here.md", destination: "custom+v1:data.md", files: []string{"custom+v1:data.md"}, want: graph.MarkdownResolution{}},
		{name: "decoded URI scheme is invalid local input", owner: "Inbox/here.md", destination: "custom%3Adata.md", files: []string{"custom:data.md"}, want: graph.MarkdownResolution{Local: true, Invalid: true}},
		{name: "canonical case and NFC", owner: "Inbox/here.md", destination: "cafe%CC%81.md", files: []string{"Area/Café.md"}, want: graph.MarkdownResolution{Resolution: graph.Resolution{Kind: graph.KindUnique, RelPath: "Area/Café.md"}, Local: true, Relative: "Inbox/café.md"}},
		{name: "outside is not rescued", owner: "Inbox/here.md", destination: "../../Other.md", files: []string{"Area/Other.md"}, want: graph.MarkdownResolution{Local: true, Outside: true, Relative: "../Other.md"}},
		{name: "invalid encoding", owner: "Inbox/here.md", destination: "A%2.md", want: graph.MarkdownResolution{Local: true, Invalid: true}},
		{name: "fragment only", owner: "Inbox/here.md", destination: "#part", want: graph.MarkdownResolution{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			idx := graph.BuildFromNotes(nil, tt.files)
			got := idx.ResolveMarkdown(tt.owner, tt.destination, func(string) bool { return true }, func(p string) bool { return slices.Contains(tt.files, p) })
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Markdown resolution (-want +got):\n%s", diff)
			}
			reversed := slices.Clone(tt.files)
			slices.Reverse(reversed)
			other := graph.BuildFromNotes(nil, reversed).ResolveMarkdown(tt.owner, tt.destination, func(string) bool { return true }, func(p string) bool { return slices.Contains(tt.files, p) })
			if diff := cmp.Diff(tt.want, other); diff != "" {
				t.Errorf("reverse inventory (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMarkdownPrivacyPrecedesEveryMembershipProbe(t *testing.T) {
	t.Parallel()
	for _, destination := range []string{"../Restricted/Other.md", "Restricted/Other.md", "Other.md"} {
		t.Run(destination, func(t *testing.T) {
			t.Parallel()
			idx := graph.BuildFromNotes(nil, []string{"Public/Other.md", "Restricted/Other.md"})
			got := idx.ResolveMarkdown("Public/here.md", destination, func(p string) bool { return !strings.HasPrefix(p, "Restricted/") }, func(p string) bool {
				t.Errorf("membership %q was observed before all interpretations were authorized", p)
				return true
			})
			if !got.Withheld || got.Kind != graph.KindUnresolved || got.RelPath != "" || len(got.Candidates) != 0 {
				t.Errorf("privacy refusal = %+v, want no disclosed resolution", got)
			}
		})
	}
}

func TestMarkdownAbsentPrivateReadingStillWithholdsThePublicFile(t *testing.T) {
	t.Parallel()
	idx := graph.BuildFromNotes(nil, []string{"Public/Restricted/Other.md"})
	got := idx.ResolveMarkdown("Public/here.md", "Restricted/Other.md", func(p string) bool {
		return !strings.HasPrefix(p, "Restricted/")
	}, func(p string) bool {
		t.Errorf("membership %q was observed despite an absent private root reading", p)
		return true
	})
	if !got.Withheld || got.Kind != graph.KindUnresolved || got.RelPath != "" || len(got.Candidates) != 0 {
		t.Errorf("absent private reading = %+v, want no disclosed resolution", got)
	}
}

func FuzzMarkdownPath(f *testing.F) {
	for _, seed := range []string{"A.md", "A%20B.md", "A%2520B.md", "%", "../../A.md", "https://host/A.md", "#part", "a%00.md"} {
		f.Add(seed)
	}
	idx := graph.BuildFromNotes(nil, []string{"Notes/A.md", "Notes/A B.md"})
	f.Fuzz(func(t *testing.T, destination string) {
		result := idx.ResolveMarkdown("Inbox/source.md", destination, func(string) bool { return true }, func(string) bool { return true })
		if result.Kind == graph.KindUnique && result.RelPath != "Notes/A.md" && result.RelPath != "Notes/A B.md" {
			t.Errorf("resolver invented uncaptured path: %+v", result)
		}
	})
}
