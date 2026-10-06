package lexical

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

func TestFrontmatterTagsFindBelowBody(t *testing.T) {
	t.Parallel()
	idx := NewIndex([]Document{
		DocumentFromNote(vault.Parse("Notes/A-tag.md", []byte("---\ntitle: Quiet\ntags: [haiku]\n---\nUnrelated prose.\n"))),
		DocumentFromNote(vault.Parse("Notes/Z-body.md", []byte("---\ntitle: Verse\n---\nA #haiku in the body.\n"))),
	}, schema.ArtifactPolicy{})
	for _, query := range []string{"haiku", "#haiku"} {
		t.Run(query, func(t *testing.T) {
			t.Parallel()
			got := paths(searchResults(t, idx, Parse(query)))
			want := []string{"Notes/Z-body.md", "Notes/A-tag.md"}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("caught: Search(%q) frontmatter tags/body order mismatch (-want +got):\n%s", query, diff)
			}
		})
	}
}

func TestFrontmatterTagEvidence(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, frontmatter, body, query string
		want                           []Result
	}{
		{name: "original first answering tag", frontmatter: "tags: [other, 'Haiku anthology', 'haiku later']", query: "haiku", want: []Result{{RelPath: "Notes/Quiet.md", Title: "Quiet", Tag: "Haiku anthology"}}},
		{name: "hash shorthand", frontmatter: "tags: [haiku]", query: "#haiku", want: []Result{{RelPath: "Notes/Quiet.md", Title: "Quiet", Tag: "haiku"}}},
		{name: "nfc case and width", frontmatter: "tags: [\"E\u0301cole ＡBC\"]", query: "#école abc", want: []Result{{RelPath: "Notes/Quiet.md", Title: "Quiet", Tag: "École ＡBC"}}},
		{name: "one tag holds all tokens", frontmatter: "tags: ['red blue']", query: "#red #blue", want: []Result{{RelPath: "Notes/Quiet.md", Title: "Quiet", Tag: "red blue"}}},
		{name: "tokens do not combine tags", frontmatter: "tags: [red, blue]", query: "red blue", want: []Result{}},
		{name: "quoted whitespace", frontmatter: "tags: [\"Colour\\nTheory\"]", query: "\"#colour theory\"", want: []Result{{RelPath: "Notes/Quiet.md", Title: "Quiet", Tag: "Colour\nTheory"}}},
		{name: "mixed members", frontmatter: "tags: [true, 12, {name: haiku}, haiku]", query: "haiku", want: []Result{{RelPath: "Notes/Quiet.md", Title: "Quiet", Tag: "haiku"}}},
		{name: "absent", query: "haiku", want: []Result{}},
		{name: "empty", frontmatter: "tags: []", query: "haiku", want: []Result{}},
		{name: "scalar", frontmatter: "tags: haiku", query: "haiku", want: []Result{}},
		{name: "map", frontmatter: "tags: {name: haiku}", query: "haiku", want: []Result{}},
		{name: "nontext", frontmatter: "tags: [true, 12]", query: "haiku", want: []Result{}},
		{name: "malformed yaml", frontmatter: "tags: [haiku", query: "haiku", want: []Result{}},
		{name: "hash only", frontmatter: "tags: [haiku]", query: "#", want: []Result{}},
		{name: "empty normalized token", frontmatter: "tags: [haiku]", query: "# haiku", want: []Result{}},
		{name: "empty query", frontmatter: "tags: [haiku]", query: "", want: nil},
		{name: "tag and body never combine", frontmatter: "tags: [haiku]", body: "poetry", query: "haiku poetry", want: []Result{}},
		{name: "tag and alias never combine", frontmatter: "tags: [haiku]\naliases: [poetry]", query: "haiku poetry", want: []Result{}},
		{name: "body wins", frontmatter: "tags: [haiku]", body: "haiku", query: "haiku", want: []Result{{RelPath: "Notes/Quiet.md", Title: "Quiet", Snippet: "haiku", Landing: "haiku", LandingBare: "haiku"}}},
		{name: "malformed frontmatter keeps body", frontmatter: "tags: [haiku", body: "haiku", query: "haiku", want: []Result{{RelPath: "Notes/Quiet.md", Title: "Quiet", Snippet: "haiku", Landing: "haiku", LandingBare: "haiku"}}},
		{name: "topic wins", frontmatter: "topics: [haiku]\ntags: [haiku]", query: "haiku", want: []Result{{RelPath: "Notes/Quiet.md", Title: "Quiet", Topic: "haiku"}}},
		{name: "alias wins", frontmatter: "aliases: [haiku]\ntags: [haiku]", query: "haiku", want: []Result{{RelPath: "Notes/Quiet.md", Title: "Quiet", Alias: "haiku"}}},
		{name: "title wins", frontmatter: "tags: [quiet]", query: "quiet", want: []Result{{RelPath: "Notes/Quiet.md", Title: "Quiet"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			note := vault.Parse("Notes/Quiet.md", []byte("---\ntitle: Quiet\n"+tt.frontmatter+"\n---\n"+tt.body))
			idx := NewIndex([]Document{DocumentFromNote(note)}, schema.ArtifactPolicy{})
			got := searchResults(t, idx, Parse(tt.query))
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("caught: Search(%q) tag evidence mismatch (-want +got):\n%s", tt.query, diff)
			}
		})
	}
}

func TestTagShorthandLeavesOtherEvidenceLiteral(t *testing.T) {
	t.Parallel()
	idx := NewIndex([]Document{
		DocumentFromNote(vault.Parse("Notes/Bare.md", []byte("---\ntitle: haiku\n---\nhaiku"))),
		DocumentFromNote(vault.Parse("Notes/Tagged.md", []byte("---\ntitle: Quiet\ntags: [haiku]\n---\nUnrelated."))),
		DocumentFromNote(vault.Parse("Notes/Literal.md", []byte("---\ntitle: Literal\n---\n#haiku"))),
	}, schema.ArtifactPolicy{})
	want := []string{"Notes/Literal.md", "Notes/Tagged.md"}
	if diff := cmp.Diff(want, paths(searchResults(t, idx, Parse("#haiku")))); diff != "" {
		t.Errorf("caught: hash shorthand leaked to other fields (-want +got):\n%s", diff)
	}
}

func TestTagCapabilityAndFileExclusion(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		policy schema.ArtifactPolicy
	}{
		{name: "no contract"},
		{name: "valid", policy: validArtifactPolicy(t)},
		{name: "unavailable", policy: invalidArtifactPolicy(t)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := DocumentFromNote(vault.Parse("Notes/Quiet.md", []byte("---\ntitle: Quiet\ntags: [haiku]\n---\nUnrelated.")))
			idx := NewIndex([]Document{doc, {RelPath: "Other.txt", Title: "Other", Tags: []string{"haiku"}, File: true}}, tt.policy)
			want := []Result{{RelPath: "Notes/Quiet.md", Title: "Quiet", Tag: "haiku"}}
			if diff := cmp.Diff(want, searchResults(t, idx, Parse("haiku"))); diff != "" {
				t.Errorf("caught: tag capability/file exclusion mismatch (-want +got):\n%s", diff)
			}
			if tt.name == "unavailable" {
				if _, err := idx.Search(Parse("haiku type:concept"), -1); !errors.Is(err, ErrMetadataUnavailable) {
					t.Errorf("Search(haiku type:concept) error = %v, want ErrMetadataUnavailable", err)
				}
			}
		})
	}
}

func TestMarkTagHitsSharesMatchingAndSpelling(t *testing.T) {
	t.Parallel()
	for _, query := range []string{"haiku", "#haiku", "＃ＨＡＩＫＵ"} {
		t.Run(query, func(t *testing.T) {
			t.Parallel()
			want := []HitRun{{Text: "Haiku", Hit: true}, {Text: " anthology"}}
			if diff := cmp.Diff(want, MarkTagHits("Haiku anthology", Parse(query).Tokens())); diff != "" {
				t.Errorf("caught: MarkTagHits(%q) mismatch (-want +got):\n%s", query, diff)
			}
		})
	}
	if got := MarkTagHits("haiku", Parse("#").Tokens()); got != nil {
		t.Errorf("MarkTagHits(#) = %+v, want nil", got)
	}
}

func TestTagIndexOwnsItsCapturedValues(t *testing.T) {
	t.Parallel()
	tags := []string{"Haiku"}
	idx := NewIndex([]Document{{RelPath: "Quiet.md", Title: "Quiet", Tags: tags}}, schema.ArtifactPolicy{})
	tags[0] = "Changed"
	want := []Result{{RelPath: "Quiet.md", Title: "Quiet", Tag: "Haiku"}}
	if diff := cmp.Diff(want, searchResults(t, idx, Parse("haiku"))); diff != "" {
		t.Errorf("captured tag values changed with source slice (-want +got):\n%s", diff)
	}
}
