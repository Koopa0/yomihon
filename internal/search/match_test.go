package search

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/vault"
)

// paths pulls the RelPaths out of results, in result order, for assertions.
func paths(results []Result) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.RelPath
	}
	return out
}

// filterFixture is a small index exercising each filter and the folder
// boundary. The RelPaths are chosen so "Writing-old/..." sorts BEFORE
// "Writing/..." ('-' 0x2D < '/' 0x2F), which pins the rel_path result order.
func filterFixture() *Index {
	return BuildFromDocs([]Doc{
		{RelPath: "Writing/Kafka.md", Title: "Kafka Basics", NoteType: "lesson", Domain: "golang", Status: "draft", Slug: "kafka-basics", Topics: []string{"messaging", "distributed"}, PlainText: "Kafka is a distributed log; 50% done."},
		{RelPath: "Writing-old/Legacy.md", Title: "Legacy", NoteType: "note", Domain: "golang", Status: "archived", Slug: "legacy", Topics: []string{"messaging"}, PlainText: "older kafka streams notes"},
		{RelPath: "Concepts/Focus.md", Title: "深度工作", NoteType: "concept", Domain: "meta", Status: "evergreen", Topics: []string{"focus"}, PlainText: "深度工作 需要 專注"},
	})
}

func TestSearchFilters(t *testing.T) {
	t.Parallel()
	idx := filterFixture()

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"type equality", "type:lesson", []string{"Writing/Kafka.md"}},
		{"status equality", "status:archived", []string{"Writing-old/Legacy.md"}},
		{"domain matches two in rel_path order", "domain:golang", []string{"Writing-old/Legacy.md", "Writing/Kafka.md"}},
		{"slug equality", "slug:kafka-basics", []string{"Writing/Kafka.md"}},
		{"topic membership single", "topic:focus", []string{"Concepts/Focus.md"}},
		{"topic membership two", "topic:messaging", []string{"Writing-old/Legacy.md", "Writing/Kafka.md"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := paths(idx.Search(Parse(tt.query)))
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Search(%q) paths mismatch (-want +got):\n%s", tt.query, diff)
			}
		})
	}
}

// TestSearchFolderBoundary pins the "/"-boundary rule: folder:Writing must not
// match Writing-old/, and a trailing slash is equivalent.
func TestSearchFolderBoundary(t *testing.T) {
	t.Parallel()
	idx := filterFixture()

	want := []string{"Writing/Kafka.md"}
	if got := paths(idx.Search(Parse("folder:Writing"))); !cmp.Equal(want, got) {
		t.Errorf("folder:Writing = %v, want %v (must exclude Writing-old/)", got, want)
	}
	if got := paths(idx.Search(Parse("folder:Writing/"))); !cmp.Equal(want, got) {
		t.Errorf("folder:Writing/ = %v, want %v (trailing slash is equivalent)", got, want)
	}
}

func TestSearchTokens(t *testing.T) {
	t.Parallel()
	idx := filterFixture()

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"two cjk tokens AND in title", "深度 工作", []string{"Concepts/Focus.md"}},
		{"multi-token AND in body", "kafka distributed", []string{"Writing/Kafka.md"}},
		{"literal percent matches literal percent", "50%", []string{"Writing/Kafka.md"}},
		{"bare percent matches percent substring", "%", []string{"Writing/Kafka.md"}},
		{"repeated key is jointly unsatisfiable", "type:a type:b", []string{}},
		{"no match", "nonexistentxyz", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := paths(idx.Search(Parse(tt.query)))
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Search(%q) paths mismatch (-want +got):\n%s", tt.query, diff)
			}
		})
	}
}

// TestSearchOrdering pins the two-bucket order: title hits (rel_path-ordered)
// before body hits (rel_path-ordered). a.md and c.md match in the title; b.md
// only in the body.
func TestSearchOrdering(t *testing.T) {
	t.Parallel()
	idx := BuildFromDocs([]Doc{
		{RelPath: "a.md", Title: "Kafka Guide", PlainText: "intro"},
		{RelPath: "b.md", Title: "Streaming", PlainText: "a kafka pipeline"},
		{RelPath: "c.md", Title: "Kafka Intro", PlainText: "more"},
	})
	got := paths(idx.Search(Parse("kafka")))
	want := []string{"a.md", "c.md", "b.md"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Search(kafka) order mismatch (-want +got):\n%s", diff)
	}
}

// TestSearchNFDContent pins that NFC-form query text matches NFD-form content.
// The body holds NFD が ("が" = か + U+3099); the query is NFC が ("が"). Both
// sides fold to NFC, so the match succeeds — this is the whole reason fold
// normalizes NFC on both index and query.
func TestSearchNFDContent(t *testing.T) {
	t.Parallel()
	idx := BuildFromDocs([]Doc{
		{RelPath: "n.md", Title: "note", PlainText: "reading \u304b\u3099 today"},
	})
	got := paths(idx.Search(Parse("\u304c")))
	if diff := cmp.Diff([]string{"n.md"}, got); diff != "" {
		t.Errorf("NFC query against NFD content mismatch (-want +got):\n%s", diff)
	}
}

// TestSearchEmptyQuery pins that an empty or whitespace query returns nothing,
// while a pure-filter query (no bare token) is legal structured browsing.
func TestSearchEmptyQuery(t *testing.T) {
	t.Parallel()
	idx := filterFixture()

	if got := idx.Search(Parse("")); got != nil {
		t.Errorf("empty query returned %v, want nil", got)
	}
	if got := idx.Search(Parse("   \t ")); got != nil {
		t.Errorf("whitespace-only query returned %v, want nil", got)
	}
	if got := paths(idx.Search(Parse("type:lesson"))); len(got) != 1 {
		t.Errorf("pure-filter query type:lesson = %v, want one match", got)
	}
}

// TestCountByTypeStatus tallies notes by (type, status) together — the pairing
// CountByStatus cannot express, since two notes sharing a status but not a type
// must stay separate. The type/status words are invented so the count is what is
// proven, not any real vault's vocabulary; a note missing a field falls in that
// field's "" bucket.
func TestCountByTypeStatus(t *testing.T) {
	t.Parallel()
	idx := BuildFromDocs([]Doc{
		{RelPath: "a.md", NoteType: "lesson", Status: "s1"},
		{RelPath: "b.md", NoteType: "lesson", Status: "s1"},
		{RelPath: "c.md", NoteType: "lesson", Status: "s2"},
		{RelPath: "d.md", NoteType: "concept", Status: "s1"},
		{RelPath: "e.md", NoteType: "", Status: ""},
	})
	got := idx.CountByTypeStatus()
	want := map[TypeStatus]int{
		{Type: "lesson", Status: "s1"}:  2,
		{Type: "lesson", Status: "s2"}:  1,
		{Type: "concept", Status: "s1"}: 1,
		{Type: "", Status: ""}:          1,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("CountByTypeStatus() mismatch (-want +got):\n%s", diff)
	}
}

// TestBuildFromDocsDeterministic pins that building the same input twice yields
// byte-identical indexes (input order does not leak into the result — entries
// are sorted by RelPath).
func TestBuildFromDocsDeterministic(t *testing.T) {
	t.Parallel()
	docs := []Doc{
		{RelPath: "b.md", Title: "B", PlainText: "beta"},
		{RelPath: "a.md", Title: "A", Topics: []string{"x"}, PlainText: "alpha"},
		{RelPath: "c.md", Title: "C", PlainText: "gamma"},
	}
	first := BuildFromDocs(docs)
	second := BuildFromDocs(docs)
	if diff := cmp.Diff(first, second, cmp.AllowUnexported(Index{}, entry{})); diff != "" {
		t.Errorf("BuildFromDocs is not deterministic (-first +second):\n%s", diff)
	}
}

// TestFrontmatterExcludedFromPlainText pins the frontmatter-exclusion rule
// end to end:
// a value that lives only in frontmatter (a created date) is not in plain_text
// and is not an indexed field, so a bare-token search for it hits nothing —
// while the body text and the structured fields still match.
func TestFrontmatterExcludedFromPlainText(t *testing.T) {
	t.Parallel()
	raw := []byte("---\ntitle: My Note\ntype: concept\ndomain: golang\ncreated: 2020-01-15\n---\n\nThe body mentions widgets.\n")
	n := vault.Parse("Concepts/My Note.md", raw)
	idx := BuildFromDocs([]Doc{docFromNote(n)})

	if got := idx.Search(Parse("2020")); len(got) != 0 {
		t.Errorf("frontmatter date leaked into plain_text: search 2020 = %v", paths(got))
	}
	if got := paths(idx.Search(Parse("widgets"))); len(got) != 1 {
		t.Errorf("body term widgets = %v, want one match", got)
	}
	if got := paths(idx.Search(Parse("domain:golang"))); len(got) != 1 {
		t.Errorf("domain:golang = %v, want one match", got)
	}
}
