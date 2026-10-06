package graph_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
)

func TestMarkdownResolutionIsRelativeAndExact(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name        string
		owner       string
		destination string
		files       []string
		want        graph.MarkdownResolution
	}{
		{name: "relative existing", owner: "Notes/here.md", destination: "Other.md", files: []string{"Notes/Other.md"}, want: graph.MarkdownResolution{Checkable: true, Resolution: graph.Resolution{Kind: graph.KindUnique, RelPath: "Notes/Other.md"}, Local: true, Relative: "Notes/Other.md"}},
		{name: "repeated capture one file", owner: "Notes/here.md", destination: "Other.md", files: []string{"Notes/Other.md", "Notes/Other.md"}, want: graph.MarkdownResolution{Checkable: true, Resolution: graph.Resolution{Kind: graph.KindUnique, RelPath: "Notes/Other.md"}, Local: true, Relative: "Notes/Other.md"}},
		{name: "relative root and shortest all differ", owner: "Notes/here.md", destination: "Other.md", files: []string{"Notes/Other.md", "Other.md", "Else/Other.md"}, want: graph.MarkdownResolution{Checkable: true, Resolution: graph.Resolution{Kind: graph.KindUnique, RelPath: "Notes/Other.md"}, Local: true, Relative: "Notes/Other.md"}},
		{name: "qualified suffix", owner: "Inbox/here.md", destination: "DB/Other.md", files: []string{"Area/DB/Other.md", "Area/NotDB/Other.md"}, want: graph.MarkdownResolution{Checkable: true, Resolution: graph.Resolution{}, Local: true, Relative: "Inbox/DB/Other.md"}},
		{name: "qualified missing is not a basename", owner: "Inbox/here.md", destination: "Absent/Other.md", files: []string{"Area/Other.md"}, want: graph.MarkdownResolution{Checkable: true, Local: true, Relative: "Inbox/Absent/Other.md"}},
		{name: "encoded delimiter remains path", owner: "Inbox/here.md", destination: "A%23B%3FC.md?at=1#part", files: []string{"Inbox/A#B?C.md"}, want: graph.MarkdownResolution{Checkable: true, Resolution: graph.Resolution{Kind: graph.KindUnique, RelPath: "Inbox/A#B?C.md"}, Local: true, Relative: "Inbox/A#B?C.md", Suffix: "?at=1#part"}},
		{name: "decode once", owner: "Inbox/here.md", destination: "A%2520B.md", files: []string{"Inbox/A%20B.md", "Inbox/A B.md"}, want: graph.MarkdownResolution{Checkable: true, Resolution: graph.Resolution{Kind: graph.KindUnique, RelPath: "Inbox/A%20B.md"}, Local: true, Relative: "Inbox/A%20B.md"}},
		{name: "literal plus", owner: "Inbox/here.md", destination: "A+B.md", files: []string{"Inbox/A+B.md", "Inbox/A B.md"}, want: graph.MarkdownResolution{Checkable: true, Resolution: graph.Resolution{Kind: graph.KindUnique, RelPath: "Inbox/A+B.md"}, Local: true, Relative: "Inbox/A+B.md"}},
		{name: "qualified colon filename", owner: "Inbox/here.md", destination: "Docs/a%3Ab.md", files: []string{"Inbox/Docs/a:b.md"}, want: graph.MarkdownResolution{Checkable: true, Resolution: graph.Resolution{Kind: graph.KindUnique, RelPath: "Inbox/Docs/a:b.md"}, Local: true, Relative: "Inbox/Docs/a:b.md"}},
		{name: "qualified literal colon filename", owner: "Inbox/here.md", destination: "Docs/a:b.md", files: []string{"Inbox/Docs/a:b.md"}, want: graph.MarkdownResolution{Checkable: true, Resolution: graph.Resolution{Kind: graph.KindUnique, RelPath: "Inbox/Docs/a:b.md"}, Local: true, Relative: "Inbox/Docs/a:b.md"}},
		{name: "URI scheme remains nonlocal", owner: "Inbox/here.md", destination: "custom+v1:data.md", files: []string{"custom+v1:data.md"}, want: graph.MarkdownResolution{}},
		{name: "decoded URI scheme is invalid local input", owner: "Inbox/here.md", destination: "custom%3Adata.md", files: []string{"custom:data.md"}, want: graph.MarkdownResolution{Local: true, Invalid: true}},
		{name: "canonical case and NFC", owner: "Inbox/here.md", destination: "cafe%CC%81.md", files: []string{"Inbox/café.md"}, want: graph.MarkdownResolution{Checkable: true, Resolution: graph.Resolution{Kind: graph.KindUnique, RelPath: "Inbox/café.md"}, Local: true, Relative: "Inbox/café.md"}},
		{name: "outside is not rescued", owner: "Inbox/here.md", destination: "../../Other.md", files: []string{"Area/Other.md"}, want: graph.MarkdownResolution{Checkable: true, Local: true, Outside: true, Relative: "../Other.md"}},
		{name: "invalid encoding", owner: "Inbox/here.md", destination: "A%2.md", want: graph.MarkdownResolution{Local: true, Invalid: true}},
		{name: "wrong case is missing", owner: "Inbox/here.md", destination: "Caf%C3%A9.md", files: []string{"Inbox/café.md"}, want: graph.MarkdownResolution{Checkable: true, Local: true, Relative: "Inbox/Café.md"}},
		{name: "root is excluded", owner: "Inbox/here.md", destination: "/Other.md", files: []string{"Other.md"}, want: graph.MarkdownResolution{}},
		{name: "vault root is not rescued", owner: "Inbox/here.md", destination: "Root.md", files: []string{"Root.md"}, want: graph.MarkdownResolution{Checkable: true, Local: true, Relative: "Inbox/Root.md"}},
		{name: "basename is not rescued", owner: "Inbox/here.md", destination: "Other.md", files: []string{"Area/Other.md"}, want: graph.MarkdownResolution{Checkable: true, Local: true, Relative: "Inbox/Other.md"}},
		{name: "fragment only", owner: "Inbox/here.md", destination: "#part", want: graph.MarkdownResolution{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := graph.ResolveMarkdown(tt.owner, tt.destination, func(string) bool { return true }, func(p string) bool { return slices.Contains(tt.files, p) })
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Markdown resolution (-want +got):\n%s", diff)
			}
			reversed := slices.Clone(tt.files)
			slices.Reverse(reversed)
			other := graph.ResolveMarkdown(tt.owner, tt.destination, func(string) bool { return true }, func(p string) bool { return slices.Contains(reversed, p) })
			if diff := cmp.Diff(tt.want, other); diff != "" {
				t.Errorf("reverse inventory (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMarkdownPrivacyPrecedesEveryMembershipProbe(t *testing.T) {
	t.Parallel()
	for _, destination := range []string{"../Restricted/Other.md", "../.hidden/Other.md"} {
		t.Run(destination, func(t *testing.T) {
			t.Parallel()
			got := graph.ResolveMarkdown("Public/here.md", destination, func(p string) bool { return !strings.HasPrefix(p, "Restricted/") }, func(p string) bool {
				t.Errorf("membership %q was observed before all interpretations were authorized", p)
				return true
			})
			if !got.Withheld || got.Kind != graph.KindUnresolved || got.RelPath != "" || len(got.Candidates) != 0 {
				t.Errorf("privacy refusal = %+v, want no disclosed resolution", got)
			}
		})
	}
}

func TestMarkdownUnrelatedPrivateCopyDoesNotWithholdLocal(t *testing.T) {
	t.Parallel()
	files := []string{"Public/Restricted/Other.md", "Restricted/Other.md"}
	got := graph.ResolveMarkdown("Public/here.md", "Restricted/Other.md", func(p string) bool { return !strings.HasPrefix(p, "Restricted/") }, func(p string) bool { return slices.Contains(files, p) })
	if got.Withheld || got.Kind != graph.KindUnique || got.RelPath != "Public/Restricted/Other.md" {
		t.Errorf("relative local path influenced by unrelated private copy: %+v", got)
	}
}

func FuzzMarkdownPath(f *testing.F) {
	for _, seed := range []string{"A.md", "A%20B.md", "A%2520B.md", "%", "../../A.md", "https://host/A.md", "#part", "a%00.md"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, destination string) {
		result := graph.ResolveMarkdown("Inbox/source.md", destination, func(string) bool { return true }, func(p string) bool { return p == "Notes/A.md" || p == "Notes/A B.md" })
		if result.Kind == graph.KindUnique && result.RelPath != "Notes/A.md" && result.RelPath != "Notes/A B.md" {
			t.Errorf("resolver invented uncaptured path: %+v", result)
		}
	})
}
