package search

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

func validArtifactPolicy(tb testing.TB) schema.ArtifactPolicy {
	tb.Helper()
	contract, err := schema.LoadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		tb.Fatalf("schema.LoadFile: %v", err)
	}
	return contract.ArtifactPolicy()
}

func invalidArtifactPolicy(tb testing.TB) schema.ArtifactPolicy {
	tb.Helper()
	return artifactPolicyFromSection(tb, "[artifacts]\nnon_instance_dirs = [\".\"]\n")
}

func incompleteArtifactPolicy(tb testing.TB) schema.ArtifactPolicy {
	tb.Helper()
	return artifactPolicyFromSection(tb, "[artifacts]\n")
}

func artifactPolicyFromSection(tb testing.TB, section string) schema.ArtifactPolicy {
	tb.Helper()
	path := filepath.Join(tb.TempDir(), "vault-schema.toml")
	contract := `schema_version = "1"

[enums]
type = ["concept"]

[enums.status]
note = ["draft"]

` + section + `
[[lifecycle]]
status = "draft"
applies_to = ["concept"]
from = []
owner = ["koopa"]
`
	if err := os.WriteFile(path, []byte(contract), 0o600); err != nil {
		tb.Fatalf("os.WriteFile: %v", err)
	}
	loaded, err := schema.LoadFile(path)
	if err != nil {
		tb.Fatalf("schema.LoadFile: %v", err)
	}
	return loaded.ArtifactPolicy()
}

// paths pulls the RelPaths out of results, in result order, for assertions.
func paths(results []Result) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.RelPath
	}
	return out
}

func searchResults(tb testing.TB, idx *Index, q Query) []Result {
	tb.Helper()
	results, err := idx.Search(q)
	if err != nil {
		tb.Fatalf("Search() error: %v", err)
	}
	return results
}

func TestSearchNonInstanceCapability(t *testing.T) {
	t.Parallel()

	idx := BuildFromDocs([]Doc{
		{RelPath: "Concepts/Instance.md", Title: "Instance", NoteType: "concept", Domain: "meta", Status: "draft", Slug: "instance", Topics: []string{"cards"}, PlainText: "shared needle"},
		{RelPath: "System/templates/Card.md", Title: "Template Card", NoteType: "concept", Domain: "meta", Status: "draft", Slug: "card", Topics: []string{"cards"}, PlainText: "shared template needle"},
	}, validArtifactPolicy(t))

	tests := []struct {
		name       string
		query      string
		wantPaths  []string
		wantStatus string
	}{
		{name: "metadata excludes noninstance", query: "type:concept", wantPaths: []string{"Concepts/Instance.md"}, wantStatus: "draft"},
		{name: "mixed metadata and text excludes noninstance", query: "type:concept template", wantPaths: []string{}},
		{name: "folder and metadata exclude noninstance", query: "folder:System/templates type:concept", wantPaths: []string{}},
		{name: "text includes noninstance without badge", query: "template", wantPaths: []string{"System/templates/Card.md"}},
		{name: "folder includes noninstance without badge", query: "folder:System/templates", wantPaths: []string{"System/templates/Card.md"}},
		{name: "text and folder include noninstance without badge", query: "template folder:System/templates", wantPaths: []string{"System/templates/Card.md"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := idx.Search(Parse(tt.query))
			if err != nil {
				t.Fatalf("Search(%q) error: %v", tt.query, err)
			}
			if diff := cmp.Diff(tt.wantPaths, paths(got)); diff != "" {
				t.Errorf("Search(%q) paths mismatch (-want +got):\n%s", tt.query, diff)
			}
			for _, result := range got {
				if result.Status != tt.wantStatus {
					t.Errorf("Search(%q) result status = %q, want %q", tt.query, result.Status, tt.wantStatus)
				}
			}
		})
	}
}

func TestSearchUnavailableMetadataCapability(t *testing.T) {
	t.Parallel()

	policies := []struct {
		name   string
		policy schema.ArtifactPolicy
	}{
		{name: "missing", policy: schema.ArtifactPolicy{}},
		{name: "invalid", policy: invalidArtifactPolicy(t)},
		{name: "incomplete", policy: incompleteArtifactPolicy(t)},
	}
	metadataQueries := []string{
		"type:concept",
		"status:",
		"domain:meta",
		"topic:cards",
		"slug:note",
		"needle type:concept",
		"folder:Concepts type:concept",
	}
	plainQueries := []string{"needle", "folder:Concepts", "needle folder:Concepts"}

	for _, policyTest := range policies {
		t.Run(policyTest.name, func(t *testing.T) {
			t.Parallel()
			idx := BuildFromDocs([]Doc{{
				RelPath: "Concepts/Note.md", Title: "Note", NoteType: "concept", Domain: "meta", Status: "draft", Slug: "note", Topics: []string{"cards"}, PlainText: "needle",
			}}, policyTest.policy)

			for _, query := range metadataQueries {
				results, err := idx.Search(Parse(query))
				if !errors.Is(err, ErrMetadataUnavailable) {
					t.Errorf("Search(%q) = (%v, %v), want ErrMetadataUnavailable", query, results, err)
					continue
				}
				if got, want := err.Error(), policyTest.policy.Diagnostic(); got != want {
					t.Errorf("Search(%q) error = %q, want %q", query, got, want)
				}
			}

			for _, query := range plainQueries {
				results, err := idx.Search(Parse(query))
				if err != nil {
					t.Errorf("Search(%q) error: %v", query, err)
					continue
				}
				if diff := cmp.Diff([]string{"Concepts/Note.md"}, paths(results)); diff != "" {
					t.Errorf("Search(%q) paths mismatch (-want +got):\n%s", query, diff)
				}
				if len(results) == 1 && results[0].Status != "" {
					t.Errorf("Search(%q) status = %q, want no metadata badge", query, results[0].Status)
				}
			}
		})
	}
}

func TestZeroValueIndexMetadataUnavailable(t *testing.T) {
	t.Parallel()

	idx := &Index{}
	wantDiagnostic := (schema.ArtifactPolicy{}).Diagnostic()
	if got, err := idx.Search(Parse("type:concept")); !errors.Is(err, ErrMetadataUnavailable) {
		t.Errorf("zero Index.Search(type:concept) = (%v, %v), want ErrMetadataUnavailable", got, err)
	} else if err.Error() != wantDiagnostic {
		t.Errorf("zero Index.Search(type:concept) error = %q, want %q", err.Error(), wantDiagnostic)
	}
	if got, err := idx.CountByStatus(); !errors.Is(err, ErrMetadataUnavailable) {
		t.Errorf("zero Index.CountByStatus() = (%v, %v), want ErrMetadataUnavailable", got, err)
	} else if err.Error() != wantDiagnostic {
		t.Errorf("zero Index.CountByStatus() error = %q, want %q", err.Error(), wantDiagnostic)
	}
	if got, err := idx.CountByTypeStatus(); !errors.Is(err, ErrMetadataUnavailable) {
		t.Errorf("zero Index.CountByTypeStatus() = (%v, %v), want ErrMetadataUnavailable", got, err)
	} else if err.Error() != wantDiagnostic {
		t.Errorf("zero Index.CountByTypeStatus() error = %q, want %q", err.Error(), wantDiagnostic)
	}
}

// filterFixture is a small index exercising each filter and the folder
// boundary. The RelPaths are chosen so "Writing-old/..." sorts BEFORE
// "Writing/..." ('-' 0x2D < '/' 0x2F), which pins the rel_path result order.
func filterFixture(tb testing.TB) *Index {
	tb.Helper()
	return BuildFromDocs([]Doc{
		{RelPath: "Writing/Kafka.md", Title: "Kafka Basics", NoteType: "lesson", Domain: "golang", Status: "draft", Slug: "kafka-basics", Topics: []string{"messaging", "distributed"}, PlainText: "Kafka is a distributed log; 50% done."},
		{RelPath: "Writing-old/Legacy.md", Title: "Legacy", NoteType: "note", Domain: "golang", Status: "archived", Slug: "legacy", Topics: []string{"messaging"}, PlainText: "older kafka streams notes"},
		{RelPath: "Concepts/Focus.md", Title: "深度工作", NoteType: "concept", Domain: "meta", Status: "evergreen", Topics: []string{"focus"}, PlainText: "深度工作 需要 專注"},
	}, validArtifactPolicy(tb))
}

func TestSearchFilters(t *testing.T) {
	t.Parallel()
	idx := filterFixture(t)

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
			got := paths(searchResults(t, idx, Parse(tt.query)))
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
	idx := filterFixture(t)

	want := []string{"Writing/Kafka.md"}
	if got := paths(searchResults(t, idx, Parse("folder:Writing"))); !cmp.Equal(want, got) {
		t.Errorf("folder:Writing = %v, want %v (must exclude Writing-old/)", got, want)
	}
	if got := paths(searchResults(t, idx, Parse("folder:Writing/"))); !cmp.Equal(want, got) {
		t.Errorf("folder:Writing/ = %v, want %v (trailing slash is equivalent)", got, want)
	}
}

func TestSearchTokens(t *testing.T) {
	t.Parallel()
	idx := filterFixture(t)

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
			got := paths(searchResults(t, idx, Parse(tt.query)))
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
	}, validArtifactPolicy(t))
	got := paths(searchResults(t, idx, Parse("kafka")))
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
	}, validArtifactPolicy(t))
	got := paths(searchResults(t, idx, Parse("\u304c")))
	if diff := cmp.Diff([]string{"n.md"}, got); diff != "" {
		t.Errorf("NFC query against NFD content mismatch (-want +got):\n%s", diff)
	}
}

// TestSearchEmptyQuery pins that an empty or whitespace query returns nothing,
// while a pure-filter query (no bare token) is legal structured browsing.
func TestSearchEmptyQuery(t *testing.T) {
	t.Parallel()
	idx := filterFixture(t)

	if got := searchResults(t, idx, Parse("")); got != nil {
		t.Errorf("empty query returned %v, want nil", got)
	}
	if got := searchResults(t, idx, Parse("   \t ")); got != nil {
		t.Errorf("whitespace-only query returned %v, want nil", got)
	}
	if got := paths(searchResults(t, idx, Parse("type:lesson"))); len(got) != 1 {
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
		{RelPath: "System/templates/Card.md", NoteType: "lesson", Status: "s1"},
	}, validArtifactPolicy(t))
	got, err := idx.CountByTypeStatus()
	if err != nil {
		t.Fatalf("CountByTypeStatus() error: %v", err)
	}
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

func TestCountByStatusExcludesNonInstances(t *testing.T) {
	t.Parallel()

	idx := BuildFromDocs([]Doc{
		{RelPath: "Concepts/Empty.md", NoteType: "concept", Status: ""},
		{RelPath: "Concepts/Draft.md", NoteType: "concept", Status: "draft"},
		{RelPath: "System/templates/Card.md", NoteType: "concept", Status: "draft"},
	}, validArtifactPolicy(t))
	want := map[string]int{"": 1, "draft": 1}
	got, err := idx.CountByStatus()
	if err != nil {
		t.Fatalf("CountByStatus() error: %v", err)
	}
	if !cmp.Equal(want, got) {
		t.Errorf("CountByStatus() = %v, want %v", got, want)
	}
}

func TestCountsUnavailableWithoutArtifactPolicy(t *testing.T) {
	t.Parallel()

	policies := []struct {
		name   string
		policy schema.ArtifactPolicy
	}{
		{name: "missing", policy: schema.ArtifactPolicy{}},
		{name: "invalid", policy: invalidArtifactPolicy(t)},
		{name: "incomplete", policy: incompleteArtifactPolicy(t)},
	}
	for _, tt := range policies {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			idx := BuildFromDocs([]Doc{{RelPath: "Concepts/Note.md", NoteType: "concept", Status: "draft"}}, tt.policy)

			if got, err := idx.CountByStatus(); !errors.Is(err, ErrMetadataUnavailable) {
				t.Errorf("CountByStatus() = (%v, %v), want ErrMetadataUnavailable", got, err)
			} else if err.Error() != tt.policy.Diagnostic() {
				t.Errorf("CountByStatus() error = %q, want %q", err.Error(), tt.policy.Diagnostic())
			}
			if got, err := idx.CountByTypeStatus(); !errors.Is(err, ErrMetadataUnavailable) {
				t.Errorf("CountByTypeStatus() = (%v, %v), want ErrMetadataUnavailable", got, err)
			} else if err.Error() != tt.policy.Diagnostic() {
				t.Errorf("CountByTypeStatus() error = %q, want %q", err.Error(), tt.policy.Diagnostic())
			}
		})
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
	policy := validArtifactPolicy(t)
	first := BuildFromDocs(docs, policy)
	second := BuildFromDocs(docs, policy)
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
	idx := BuildFromDocs([]Doc{docFromNote(n)}, validArtifactPolicy(t))

	if got := searchResults(t, idx, Parse("2020")); len(got) != 0 {
		t.Errorf("frontmatter date leaked into plain_text: search 2020 = %v", paths(got))
	}
	if got := paths(searchResults(t, idx, Parse("widgets"))); len(got) != 1 {
		t.Errorf("body term widgets = %v, want one match", got)
	}
	if got := paths(searchResults(t, idx, Parse("domain:golang"))); len(got) != 1 {
		t.Errorf("domain:golang = %v, want one match", got)
	}
}
