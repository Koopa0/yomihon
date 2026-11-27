package lexical

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/render"
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
	return artifactPolicyFromSection(tb, "[folders]\ntemplates = [\".\"]\n")
}

// escapingArtifactPolicy declares a templates folder above the vault root, so
// the contract loaded and the policy it meant cannot be honoured.
func escapingArtifactPolicy(tb testing.TB) schema.ArtifactPolicy {
	tb.Helper()
	return artifactPolicyFromSection(tb, "[folders]\ntemplates = [\"../outside\"]\n")
}

// blankArtifactPolicy declares a templates folder with no name.
func blankArtifactPolicy(tb testing.TB) schema.ArtifactPolicy {
	tb.Helper()
	return artifactPolicyFromSection(tb, "[folders]\ntemplates = [\"\"]\n")
}

func artifactPolicyFromSection(tb testing.TB, section string) schema.ArtifactPolicy {
	tb.Helper()
	path := filepath.Join(tb.TempDir(), "vault-schema.toml")
	contract := `` + section + `
[types.concept]

[rules]
concept_requires_provenance = ["based_on"]
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
	for i := range results {
		out[i] = results[i].RelPath
	}
	return out
}

func searchResults(tb testing.TB, idx *Index, q *Query) []Result {
	tb.Helper()
	answer, err := idx.Search(q, -1)
	if err != nil {
		tb.Fatalf("Search() error: %v", err)
	}
	return answer.Results
}

// soleResult is the one result a query is expected to find.
func soleResult(tb testing.TB, idx *Index, query string) Result {
	tb.Helper()
	results := searchResults(tb, idx, Parse(query))
	if len(results) != 1 {
		tb.Fatalf("Search(%q) returned %d results, want 1", query, len(results))
	}
	return results[0]
}

func TestALeadAndALandingRequireBodyEvidence(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
		{RelPath: "title-only.md", Title: "Needle", PlainText: "unrelated body"},
		{RelPath: "title-and-body.md", Title: "Needle Too", PlainText: "needle appears here"},
		{RelPath: "filter-only.md", Title: "Filter", PlainText: "body start", NoteType: "concept"},
	}, validArtifactPolicy(t))

	text := searchResults(t, idx, Parse("needle"))
	if len(text) != 2 {
		t.Fatalf("text results = %+v", text)
	}
	if text[0].RelPath != "title-only.md" || text[0].Lead.Runs != nil || text[0].Landing != "" {
		t.Errorf("title-only result = %+v, want the exact title first with no lead or landing", text[0])
	}
	if text[1].RelPath != "title-and-body.md" || text[1].Lead.Runs == nil || text[1].Landing != "needle" {
		t.Errorf("title+body result = %+v, want body evidence after the exact title", text[1])
	}

	filtered := searchResults(t, idx, Parse("type:concept"))
	if len(filtered) != 1 || filtered[0].Lead.Runs != nil || filtered[0].Landing != "" {
		t.Errorf("pure-filter result = %+v, want no lead or landing", filtered)
	}
}

func TestSearchNonInstanceCapability(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
		{RelPath: "Concepts/Instance.md", Title: "Instance", NoteType: "concept", Domain: "meta", Draft: true, Slug: "instance", PlainText: "shared needle"},
		{RelPath: "System/templates/Card.md", Title: "Template Card", NoteType: "concept", Domain: "meta", Draft: true, Slug: "card", PlainText: "shared template needle"},
	}, validArtifactPolicy(t))

	tests := []struct {
		name      string
		query     string
		wantPaths []string
		wantDraft bool
	}{
		{name: "metadata excludes noninstance", query: "type:concept", wantPaths: []string{"Concepts/Instance.md"}, wantDraft: true},
		{name: "mixed metadata and text excludes noninstance", query: "type:concept template", wantPaths: []string{}},
		{name: "folder and metadata exclude noninstance", query: "folder:System/templates type:concept", wantPaths: []string{}},
		{name: "text includes noninstance without badge", query: "template", wantPaths: []string{"System/templates/Card.md"}},
		{name: "folder includes noninstance without badge", query: "folder:System/templates", wantPaths: []string{"System/templates/Card.md"}},
		{name: "text and folder include noninstance without badge", query: "template folder:System/templates", wantPaths: []string{"System/templates/Card.md"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			answer, err := idx.Search(Parse(tt.query), -1)
			if err != nil {
				t.Fatalf("Search(%q) error: %v", tt.query, err)
			}
			if diff := cmp.Diff(tt.wantPaths, paths(answer.Results)); diff != "" {
				t.Errorf("Search(%q) paths mismatch (-want +results):\n%s", tt.query, diff)
			}
			for _, result := range answer.Results {
				if result.Draft != tt.wantDraft {
					t.Errorf("Search(%q) result draft = %t, want %t", tt.query, result.Draft, tt.wantDraft)
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
		{name: "escaping", policy: escapingArtifactPolicy(t)},
		{name: "invalid", policy: invalidArtifactPolicy(t)},
		{name: "blank", policy: blankArtifactPolicy(t)},
	}
	metadataQueries := []string{
		"type:concept",
		"draft:true",
		"domain:meta",
		"slug:note",
		"needle type:concept",
		"folder:Concepts type:concept",
	}
	plainQueries := []string{"needle", "folder:Concepts", "needle folder:Concepts"}

	for _, policyTest := range policies {
		t.Run(policyTest.name, func(t *testing.T) {
			t.Parallel()
			idx := NewIndex([]Document{{
				RelPath: "Concepts/Note.md", Title: "Note", NoteType: "concept", Domain: "meta", Draft: true, Slug: "note", PlainText: "needle",
			}}, policyTest.policy)

			for _, query := range metadataQueries {
				answer, err := idx.Search(Parse(query), -1)
				if !errors.Is(err, ErrMetadataUnavailable) {
					t.Errorf("Search(%q) = (%v, %v), want ErrMetadataUnavailable", query, answer.Results, err)
					continue
				}
				if got, want := err.Error(), policyTest.policy.Diagnostic(); got != want {
					t.Errorf("Search(%q) error = %q, want %q", query, got, want)
				}
			}

			for _, query := range plainQueries {
				answer, err := idx.Search(Parse(query), -1)
				if err != nil {
					t.Errorf("Search(%q) error: %v", query, err)
					continue
				}
				if diff := cmp.Diff([]string{"Concepts/Note.md"}, paths(answer.Results)); diff != "" {
					t.Errorf("Search(%q) paths mismatch (-want +got):\n%s", query, diff)
				}
				if len(answer.Results) == 1 && answer.Results[0].Draft {
					t.Errorf("Search(%q) marked a draft, want no metadata badge", query)
				}
			}
		})
	}
}

func TestWithArtifactPolicyBindsIndependentMetadataAuthority(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{{
		RelPath:  "Concepts/Note.md",
		Title:    "Note",
		NoteType: "concept",
		Draft:    true,
	}}, validArtifactPolicy(t))
	closed := idx.WithArtifactPolicy(escapingArtifactPolicy(t))

	openAnswer, err := idx.Search(Parse("Note"), -1)
	if err != nil {
		t.Fatalf("original Search(Note) error = %v", err)
	}
	if len(openAnswer.Results) != 1 || !openAnswer.Results[0].Draft {
		t.Fatalf("original Search(Note) = %+v, want draft metadata badge", openAnswer.Results)
	}
	closedAnswer, err := closed.Search(Parse("Note"), -1)
	if err != nil {
		t.Fatalf("closed Search(Note) error = %v", err)
	}
	if len(closedAnswer.Results) != 1 || closedAnswer.Results[0].Draft {
		t.Errorf("closed Search(Note) = %+v, want lexical result without metadata badge", closedAnswer.Results)
	}
	if _, err := closed.Search(Parse("draft:true"), -1); !errors.Is(err, ErrMetadataUnavailable) {
		t.Errorf("closed Search(draft:true) error = %v, want ErrMetadataUnavailable", err)
	}
	if _, err := idx.Search(Parse("draft:true"), -1); err != nil {
		t.Errorf("original Search(draft:true) error = %v, want functional copy not to mutate original", err)
	}
}

// TestZeroValueIndexAnswersNothingWithoutInventingAFault pins the zero Index,
// which holds no documents and no declared exclusions. Every projection over it
// is empty, and empty is the honest answer — the index has nothing to withhold,
// so it does not report a capability failure it did not have.
func TestZeroValueIndexAnswersNothingWithoutInventingAFault(t *testing.T) {
	t.Parallel()

	idx := &Index{}
	answer, err := idx.Search(Parse("type:concept"), -1)
	if err != nil {
		t.Errorf("zero Index.Search(type:concept, -1) error = %v, want nil", err)
	}
	if len(answer.Results) != 0 {
		t.Errorf("zero Index.Search(type:concept, -1) = %v, want no results", answer.Results)
	}
	drafts, err := idx.Search(Parse("draft:true"), -1)
	if err != nil || len(drafts.Results) != 0 {
		t.Errorf("zero Index.Search(draft:true, -1) = (%v, %v), want no results and no error", drafts.Results, err)
	}
}

// TestWithheldCapabilitiesRefuseMetadataWithSomethingToSay pins the hole that
// opened when the vault-level fault was carried separately from the capability
// it closed: the index refused the query, its reason came back empty, and the
// results page rendered the ordinary "nothing found" copy. Zero results and
// "cannot answer" are different sentences, and only one of them is true here.
func TestWithheldCapabilitiesRefuseMetadataWithSomethingToSay(t *testing.T) {
	t.Parallel()

	policy := (*schema.Contract)(nil).Capabilities(schema.Unreadable(errors.New("toml: line 42: expected a key separator"))).Artifacts
	idx := NewIndex([]Document{
		{RelPath: "Concepts/Note.md", Title: "Note", NoteType: "concept", Draft: true},
	}, policy)

	_, err := idx.Search(Parse("draft:true"), -1)
	if !errors.Is(err, ErrMetadataUnavailable) {
		t.Fatalf("Search(draft:true) error = %v, want ErrMetadataUnavailable", err)
	}
	if err.Error() == "" {
		t.Error("the refusal carries no reason; the page has nothing to show but a false empty result")
	}
	if !strings.Contains(err.Error(), "line 42") {
		t.Errorf("the refusal does not name the cause: %q", err.Error())
	}
}

// TestUnclaimedArtifactPolicyFiltersOverRawFrontmatter pins the difference the
// grant split makes to search. A folder that carries no contract excluded no
// directory, so a metadata filter runs over every readable note's own
// frontmatter and answers rather than refusing. What it must not do is dress
// that raw value up as a lifecycle state; that decision belongs to the surface
// that renders it, not to the index.
func TestUnclaimedArtifactPolicyFiltersOverRawFrontmatter(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
		{RelPath: "Concepts/Note.md", Title: "Note", NoteType: "concept", Draft: true, PlainText: "needle"},
		{RelPath: "System/templates/Card.md", Title: "Card", NoteType: "lesson", PlainText: "needle"},
	}, schema.ArtifactPolicy{})

	answer, err := idx.Search(Parse("type:concept"), -1)
	if err != nil {
		t.Fatalf("Search(type:concept) with an unclaimed policy error = %v, want an answer", err)
	}
	if diff := cmp.Diff([]string{"Concepts/Note.md"}, paths(answer.Results)); diff != "" {
		t.Errorf("Search(type:concept) paths mismatch (-want +got):\n%s", diff)
	}
	// Nothing declared System/templates as an artifact, so it is an ordinary
	// note and a plain query reaches it.
	plainAnswer, err := idx.Search(Parse("needle"), -1)
	if err != nil {
		t.Fatalf("Search(needle) error = %v", err)
	}
	if len(plainAnswer.Results) != 2 {
		t.Errorf("Search(needle) = %v, want both notes: nothing was excluded", paths(plainAnswer.Results))
	}
	drafts, err := idx.Search(Parse("draft:true"), -1)
	if err != nil {
		t.Fatalf("Search(draft:true) with an unclaimed policy error = %v", err)
	}
	if diff := cmp.Diff([]string{"Concepts/Note.md"}, paths(drafts.Results)); diff != "" {
		t.Errorf("Search(draft:true) paths mismatch (-want +got):\n%s", diff)
	}
}

// filterFixture is a small index exercising each filter and the folder
// boundary. The RelPaths are chosen so "Writing-old/..." sorts BEFORE
// "Writing/..." ('-' 0x2D < '/' 0x2F), which pins the rel_path result order.
func filterFixture(tb testing.TB) *Index {
	tb.Helper()
	return NewIndex([]Document{
		{RelPath: "Writing/Kafka.md", Title: "Kafka Basics", NoteType: "lesson", Domain: "golang", Draft: true, Slug: "kafka-basics", PlainText: "Kafka is a distributed log; 50% done."},
		{RelPath: "Writing-old/Legacy.md", Title: "Legacy", NoteType: "note", Domain: "golang", Slug: "legacy", PlainText: "older kafka streams notes"},
		{RelPath: "Concepts/Focus.md", Title: "深度工作", NoteType: "concept", Domain: "meta", PlainText: "深度工作 需要 專注"},
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
		{"draft equality", "draft:true", []string{"Writing/Kafka.md"}},
		{"domain matches two in rel_path order", "domain:golang", []string{"Writing/Kafka.md", "Writing-old/Legacy.md"}},
		{"slug equality", "slug:kafka-basics", []string{"Writing/Kafka.md"}},
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

// TestFolderFilterReadsTheSeparatorsTheVaultWrote pins where a folder filter
// takes its boundary: the path's own "/", and nothing that merely folds into
// one. A vault may hold a top-level directory whose name carries a fullwidth
// solidus, and reading that character as a separator answered a reader who had
// restricted a query to Writing with a note that never sat there, while
// offering Writing/Archive as a directory the vault does not have.
func TestFolderFilterReadsTheSeparatorsTheVaultWrote(t *testing.T) {
	t.Parallel()
	idx := NewIndex([]Document{
		{RelPath: "Writing/Real.md", Title: "Real", PlainText: "one"},
		{RelPath: "Writing／Archive/Sibling.md", Title: "Sibling", PlainText: "two"},
		{RelPath: "Notes/Archive/Nested.md", Title: "Nested", PlainText: "three"},
		{RelPath: "Ｗide/One.md", Title: "One", PlainText: "four"},
	}, validArtifactPolicy(t))

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"a folder name ends at a real separator", "folder:Writing", []string{"Writing/Real.md"}},
		{"a directory the vault does not have answers nothing", "folder:Writing/Archive", []string{}},
		{"the name as written reaches what is under it", "folder:Writing／Archive", []string{"Writing／Archive/Sibling.md"}},
		{"a real nested directory still matches", "folder:Notes/Archive", []string{"Notes/Archive/Nested.md"}},
		{"case still does not matter", "folder:writing", []string{"Writing/Real.md"}},
		{"a fullwidth letter in a name still folds", "folder:wide", []string{"Ｗide/One.md"}},
		{"a trailing slash is the same folder", "folder:Writing/", []string{"Writing/Real.md"}},
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

// TestSearchOrdering pins the opening of the group order: a note's title
// hits (rel_path-ordered) before a note's body hits (rel_path-ordered). a.md
// and c.md match in the title; b.md only in the body.
func TestSearchOrdering(t *testing.T) {
	t.Parallel()
	idx := NewIndex([]Document{
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

// TestAnExactTitleLeadsATitleThatOnlyContainsTheQuery is the one tie-break
// inside the title group: a note whose title is the query leads any longer
// title whose path sorts earlier. Fold-equal exact titles come first;
// everything else in the group keeps the vault's reading order. The other
// groups do not move.
func TestAnExactTitleLeadsATitleThatOnlyContainsTheQuery(t *testing.T) {
	t.Parallel()

	// Separate folders make path order oppose title relevance even when
	// filename stems place a shorter name before its longer prefix.
	containing := "a/Go Slice 共享底層陣列.md"
	exact := "z/Go Slice.md"
	if vault.ComparePaths(containing, exact) >= 0 {
		t.Fatal("the containing-title path must sort first, or this fixture cannot catch a missing tie-break")
	}

	idx := NewIndex([]Document{
		{RelPath: containing, Title: "Go Slice 共享底層陣列", PlainText: "a longer title that contains the query"},
		{RelPath: exact, Title: "Go Slice", PlainText: "the note whose title is the query"},
	}, validArtifactPolicy(t))

	got := paths(searchResults(t, idx, Parse("Go Slice")))
	want := []string{exact, containing}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("caught: Search(Go Slice) order mismatch (-want +got):\n%s", diff)
	}
}

// TestTwoExactTitlesKeepReadingOrder pins the tie inside the exact-title
// answer: two notes whose titles are the query stay in the vault's reading
// order relative to each other. A containing title that sorts earlier by path
// still follows both, so the fixture cannot pass on path order alone.
func TestTwoExactTitlesKeepReadingOrder(t *testing.T) {
	t.Parallel()

	containing := "a-containing.md"
	first := "m-exact.md"
	second := "z-exact.md"
	if vault.ComparePaths(containing, first) >= 0 || vault.ComparePaths(first, second) >= 0 {
		t.Fatal("the containing title must sort first by path, then the two exact titles, or this fixture cannot catch a reversed tie")
	}

	idx := NewIndex([]Document{
		{RelPath: containing, Title: "Needle Too", PlainText: "unrelated"},
		{RelPath: first, Title: "Needle", PlainText: "unrelated"},
		{RelPath: second, Title: "Needle", PlainText: "unrelated"},
	}, validArtifactPolicy(t))

	got := paths(searchResults(t, idx, Parse("needle")))
	want := []string{first, second, containing}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Search(needle) order mismatch (-want +got):\n%s", diff)
	}
}

// TestAPureFilterKeepsReadingOrder pins the empty-token arm of raiseExactTitles.
// A pure-filter query lands every match in the title group with no tokens, so
// joining them would make every empty title look exact and float it above notes
// that merely sort earlier by path. The group keeps the vault's reading order.
func TestAPureFilterKeepsReadingOrder(t *testing.T) {
	t.Parallel()
	idx := NewIndex([]Document{
		{RelPath: "a.md", Title: "Named", NoteType: "concept"},
		{RelPath: "z.md", Title: "", NoteType: "concept"},
	}, validArtifactPolicy(t))

	got := paths(searchResults(t, idx, Parse("type:concept")))
	want := []string{"a.md", "z.md"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Search(type:concept) order mismatch (-want +got):\n%s", diff)
	}
}

// TestTheFourAnswerGroupsComeBackInRankedOrder pins the complete group
// order. Paths deliberately reverse the evidence order, so reading order cannot
// accidentally satisfy the rank assertion.
func TestTheFourAnswerGroupsComeBackInRankedOrder(t *testing.T) {
	t.Parallel()
	idx := NewIndex([]Document{
		{RelPath: "b-kafka/inside.md", Title: "Inside", PlainText: "unrelated"},
		{RelPath: "f-tag.md", Title: "Declared", Tags: []string{"kafka"}, PlainText: "unrelated"},
		{RelPath: "h/note.md", Title: "Streaming", PlainText: "a kafka pipeline"},
		{RelPath: "i/note.md", Title: "Kafka guide", PlainText: "nothing else"},
	}, validArtifactPolicy(t))
	want := []string{"i/note.md", "h/note.md", "f-tag.md", "b-kafka/inside.md"}
	if int(bucketCount) != len(want) {
		t.Fatalf("complete answer group count = %d, want %d", bucketCount, len(want))
	}
	if diff := cmp.Diff(want, paths(searchResults(t, idx, Parse("kafka")))); diff != "" {
		t.Errorf("caught: Search(kafka) group order mismatch (-want +got):\n%s", diff)
	}
	limited, err := idx.Search(Parse("kafka"), 0)
	if err != nil || limited.Total != 4 || len(limited.Results) != 0 {
		t.Errorf("Search(kafka, 0) = (%+v, %v), want total 4 with no materialized results", limited, err)
	}
}

// TestAKnowledgeLayerHitLeadsAnOutsideHitInItsGroup is the flatten-time
// knowledge rank. A System note and a knowledge-layer note share one
// evidence group; the System path sorts first so path order cannot pass
// this. The knowledge-layer note leads; the System note stays in the answer. Notes/ sorts before
// System/ under ComparePaths, so the layer note here is Writing/.
func TestAKnowledgeLayerHitLeadsAnOutsideHitInItsGroup(t *testing.T) {
	t.Parallel()

	outside := "System/early.md"
	inside := "Writing/late.md"
	if vault.ComparePaths(outside, inside) >= 0 {
		t.Fatal("the System path must sort first, or this fixture cannot catch a missing knowledge rank")
	}

	idx := NewIndex([]Document{
		{RelPath: outside, Title: "Unrelated", PlainText: "needle in a template", OutsideKnowledge: true},
		{RelPath: inside, Title: "Unrelated", PlainText: "needle in a note"},
	}, validArtifactPolicy(t))

	got := paths(searchResults(t, idx, Parse("needle")))
	want := []string{inside, outside}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Search(needle) order mismatch (-want +got):\n%s", diff)
	}
}

// TestAnExactTitleOutsideTheKnowledgeLayerStillLeadsAPartialTitle pins
// exactness over the layer. A reader who types a file's exact title has
// named it; reading the knowledge layer first is a tie-break among hits of the
// same strength, not a rule over that name. The Notes/ partial sorts
// first by path and is in-layer, so path order and a knowledge-primary sort
// both put it first — only raiseExactTitles after raiseKnowledge puts the
// outside exact title ahead.
func TestAnExactTitleOutsideTheKnowledgeLayerStillLeadsAPartialTitle(t *testing.T) {
	t.Parallel()

	partial := "Notes/partial.md"
	exact := "System/exact.md"
	if vault.ComparePaths(partial, exact) >= 0 {
		t.Fatal("the in-layer partial must sort first by path, or this fixture cannot catch a missing exact-title raise")
	}

	idx := NewIndex([]Document{
		{RelPath: partial, Title: "needle and more", PlainText: "unrelated"},
		{RelPath: exact, Title: "needle", PlainText: "unrelated", OutsideKnowledge: true},
	}, validArtifactPolicy(t))

	got := paths(searchResults(t, idx, Parse("needle")))
	want := []string{exact, partial}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Search(needle) order mismatch (-want +got):\n%s", diff)
	}
}

// TestResultsSortTheWayTheirNumbersRead holds the results list to the same
// reading order the sidebar shows a course in. Comparing code points puts 第10課
// between 第1課 and 第2課, so a reader who found their lessons in one order in
// the rail found them in another in the results, with nothing on either page to
// say which was right.
func TestResultsSortTheWayTheirNumbersRead(t *testing.T) {
	t.Parallel()
	idx := NewIndex([]Document{
		{RelPath: "Writing/第10課.md", Title: "第10課", PlainText: "動詞"},
		{RelPath: "Writing/第9課.md", Title: "第9課", PlainText: "動詞"},
	}, validArtifactPolicy(t))

	want := []string{"Writing/第9課.md", "Writing/第10課.md"}
	if diff := cmp.Diff(want, paths(searchResults(t, idx, Parse("動詞")))); diff != "" {
		t.Errorf("Search(動詞) order mismatch (-want +got):\n%s", diff)
	}
}

// TestSearchReachesBothWidthsFromEitherSpelling is the end-to-end lock: two
// notes hold the same sentence in either width, and either spelling of a word
// in it returns both. On a fold that leaves width alone the sets are disjoint.
func TestSearchReachesBothWidthsFromEitherSpelling(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
		{RelPath: "half.md", Title: "half", PlainText: "3羽と3つ。2週間後に Go の並行。心(こころ)"},
		{RelPath: "full.md", Title: "full", PlainText: "３羽と３つ。２週間後に Ｇｏ の並行。心（こころ）"},
	}, validArtifactPolicy(t))
	want := []string{"full.md", "half.md"}
	for _, query := range []string{
		"3羽", "３羽",
		"3つ", "３つ",
		"2週間後", "２週間後",
		"Go の並行", "Ｇｏ の並行",
		"心(こころ)", "心（こころ）",
	} {
		t.Run(query, func(t *testing.T) {
			t.Parallel()
			got := paths(searchResults(t, idx, Parse(query)))
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("Search(%q) mismatch (-want +got):\n%s", query, diff)
			}
		})
	}
}

// TestSearchFiltersAnswerEitherCase pins that each filter key
// answers the same set for either case of its value. Free text already folded;
// a filter that compared raw made the habit that works on words fail on
// constraints.
func TestSearchFiltersAnswerEitherCase(t *testing.T) {
	t.Parallel()
	idx := filterFixture(t)

	tests := []struct {
		key   string
		lower string
		upper string
		want  []string
	}{
		{"type", "type:lesson", "type:Lesson", []string{"Writing/Kafka.md"}},
		{"draft", "draft:true", "draft:TRUE", []string{"Writing/Kafka.md"}},
		{"domain", "domain:golang", "domain:Golang", []string{"Writing/Kafka.md", "Writing-old/Legacy.md"}},
		{"slug", "slug:kafka-basics", "slug:Kafka-Basics", []string{"Writing/Kafka.md"}},
		{"folder", "folder:Writing", "folder:writing", []string{"Writing/Kafka.md"}},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()
			lower := paths(searchResults(t, idx, Parse(tt.lower)))
			upper := paths(searchResults(t, idx, Parse(tt.upper)))
			if diff := cmp.Diff(tt.want, lower); diff != "" {
				t.Errorf("Search(%q) mismatch (-want +got):\n%s", tt.lower, diff)
			}
			if diff := cmp.Diff(lower, upper); diff != "" {
				t.Errorf("Search(%q) and Search(%q) answered different sets (-lower +upper):\n%s", tt.lower, tt.upper, diff)
			}
		})
	}
}

// TestFolderFilterPinsCaseSiblingOrder locks what folding a path prefix does
// when two sibling directories differ only in case: both answer, and the vault
// reading order decides which leads. "At least one comes back" would pass on
// either behaviour and lock nothing.
func TestFolderFilterPinsCaseSiblingOrder(t *testing.T) {
	t.Parallel()
	idx := NewIndex([]Document{
		{RelPath: "Writing/Kafka.md", Title: "Kafka", PlainText: "one"},
		{RelPath: "writing/notes.md", Title: "Notes", PlainText: "two"},
	}, validArtifactPolicy(t))

	want := []string{"Writing/Kafka.md", "writing/notes.md"}
	for _, query := range []string{"folder:Writing", "folder:writing"} {
		got := paths(searchResults(t, idx, Parse(query)))
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("Search(%q) mismatch (-want +got):\n%s", query, diff)
		}
	}
}

// TestCJKQueryMatchesInsideALongerWord pins substring semantics for CJK text:
// a match is a literal substring with no word segmentation, so 京都 is found
// inside 東京都 even though a reader parsing the words would cut 東京 | 都.
// CJK prose carries no delimiter to segment by, and a substring answer the
// reader can see on the page beats a guessed segmentation they cannot argue
// with.
func TestCJKQueryMatchesInsideALongerWord(t *testing.T) {
	t.Parallel()
	idx := NewIndex([]Document{
		{RelPath: "tokyo.md", Title: "首都", PlainText: "東京都の人口"},
		{RelPath: "kyoto.md", Title: "古都", PlainText: "京都の寺"},
	}, validArtifactPolicy(t))
	got := paths(searchResults(t, idx, Parse("京都")))
	if diff := cmp.Diff([]string{"kyoto.md", "tokyo.md"}, got); diff != "" {
		t.Errorf("Search(京都) mismatch (-want +got):\n%s", diff)
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

// TestSearchQuotedPhraseMatchesAdjacentText pins phrase semantics end to end:
// a quoted span matches only where its words sit together, because the parsed
// phrase is one token and matching is a contiguous substring test. A reader
// who quoted a phrase sitting verbatim in their own note used to get 找不到 —
// the quote characters rode along inside the tokens and matched nothing.
func TestSearchQuotedPhraseMatchesAdjacentText(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
		{RelPath: "Notes/adjacent.md", Title: "Adjacent", PlainText: "daily semantic retrieval log"},
		{RelPath: "Notes/apart.md", Title: "Apart", PlainText: "semantic things sit here and retrieval happens later"},
	}, validArtifactPolicy(t))

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{name: "quoted phrase finds only the adjacent note", query: `"semantic retrieval"`, want: []string{"Notes/adjacent.md"}},
		{name: "corner-bracket phrase finds the same note", query: "「semantic retrieval」", want: []string{"Notes/adjacent.md"}},
		{name: "unquoted terms find both notes", query: "semantic retrieval", want: []string{"Notes/adjacent.md", "Notes/apart.md"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tt.want, paths(searchResults(t, idx, Parse(tt.query)))); diff != "" {
				t.Errorf("Search(%q) paths mismatch (-want +got):\n%s", tt.query, diff)
			}
		})
	}
}

// TestBoundedSearchKeepsTheOpeningStretch pins the two properties the bounded
// variant owes its callers: the total is the count the unbounded answer would
// have, and the kept results are exactly that answer's opening stretch — same
// hits, same order, same leads. A cap that reordered or resampled would
// make the first page of a broad query a different page each time.
func TestBoundedSearchKeepsTheOpeningStretch(t *testing.T) {
	t.Parallel()

	docs := make([]Document, 0, 10)
	for i := range 9 {
		docs = append(docs, Document{
			RelPath:   fmt.Sprintf("Notes/n%d.md", i),
			Title:     fmt.Sprintf("Note %d", i),
			PlainText: "shared needle body",
		})
	}
	// A title hit sorted last by path, so bucket order (title hits first)
	// disagrees with scan order and a cap taken during the scan would keep the
	// wrong hits.
	docs = append(docs, Document{RelPath: "Notes/z-title.md", Title: "needle in the title", PlainText: "no match here"})
	idx := NewIndex(docs, validArtifactPolicy(t))

	answer, err := idx.Search(Parse("needle"), -1)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(answer.Results) != 10 || answer.Results[0].RelPath != "Notes/z-title.md" {
		t.Fatalf("unbounded answer = %d results opening with %q, want 10 opening with the title hit", len(answer.Results), answer.Results[0].RelPath)
	}

	bounded, err := idx.Search(Parse("needle"), 4)
	if err != nil {
		t.Fatalf("search() error = %v", err)
	}
	if bounded.Total != 10 {
		t.Errorf("total = %d, want 10", bounded.Total)
	}
	if diff := cmp.Diff(answer.Results[:4], bounded.Results); diff != "" {
		t.Errorf("bounded results are not the opening stretch (-unbounded[:4] +bounded):\n%s", diff)
	}

	if countOnly, countErr := idx.Search(Parse("needle"), 0); countErr != nil || countOnly.Total != 10 {
		t.Errorf("count-only search = (total %d, %v), want (10, nil)", countOnly.Total, countErr)
	}
}

// TestFrontmatterExcludedFromPlainText pins the frontmatter-exclusion rule
// end to end:
// a value that lives only in frontmatter (a created date) is not in plain_text
// and is not an indexed field, so a bare-token search for it hits nothing —
// while the body text and the structured fields still match.
func TestFrontmatterExcludedFromPlainText(t *testing.T) {
	t.Parallel()
	raw := []byte("---\ntype: concept\ndomain: elsewhere\ncreated: 2020-01-15\n---\n\nThe body mentions widgets.\n")
	n := vault.Parse("Concepts/golang/My Note.md", raw)
	n.Domain = "golang"
	idx := NewIndex([]Document{DocumentFromNote(n)}, validArtifactPolicy(t))

	if got := searchResults(t, idx, Parse("2020")); len(got) != 0 {
		t.Errorf("frontmatter date leaked into plain_text: search 2020 = %v", paths(got))
	}
	if got := paths(searchResults(t, idx, Parse("widgets"))); len(got) != 1 {
		t.Errorf("body term widgets = %v, want one match", got)
	}
	if got := paths(searchResults(t, idx, Parse("domain:golang"))); len(got) != 1 {
		t.Errorf("domain:golang = %v, want one match", got)
	}
	// The domain is the one the note's folder gave it; a domain key written in
	// the frontmatter is no field and narrows nothing.
	if got := paths(searchResults(t, idx, Parse("domain:elsewhere"))); len(got) != 0 {
		t.Errorf("domain:elsewhere = %v, want no match", got)
	}
}

// TestQuotedPhraseCrossesALineBreak fixes the shape of this vault's prose into
// the phrase rule: sentences are hard-wrapped near 80 columns, and the plain
// text an index is built from keeps those breaks, so the two words a reader
// quotes sit on two lines about as often as on one. Asking for the same bytes
// answered a phrase written verbatim in the vault with nothing at all — a
// confident zero, the one answer a reader cannot argue with.
//
// The words still have to be adjacent. A note that holds both words with
// something between them is not what the quotes asked for and stays out.
func TestQuotedPhraseCrossesALineBreak(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
		{RelPath: "wrapped.md", Title: "Wrapped", PlainText: "daily semantic\nretrieval log"},
		{RelPath: "apart.md", Title: "Apart", PlainText: "semantic log\nretrieval notes"},
		{RelPath: "spaced.md", Title: "Spaced", PlainText: "the semantic  retrieval pass"},
		{RelPath: "deep.md", Title: "Deep", PlainText: strings.Repeat("filler ", 60) + "deep semantic\nretrieval marker"},
	}, schema.ArtifactPolicy{})

	query := Parse(`"semantic retrieval"`)
	got := searchResults(t, idx, query)
	if diff := cmp.Diff([]string{"deep.md", "spaced.md", "wrapped.md"}, paths(got)); diff != "" {
		t.Fatalf("Search(%q) mismatch (-want +got):\n%s", `"semantic retrieval"`, diff)
	}

	// The hit is also shown, marked on the note's own bytes, and the lead
	// of a long note opens near the phrase rather than at the top, where it
	// would show filler and mark nothing.
	for _, r := range got {
		switch r.RelPath {
		case "wrapped.md", "deep.md":
			if mark := marked(r.Lead.Runs); mark != "semantic retrieval" {
				t.Errorf("%s: lead marks %q, want the phrase on one line", r.RelPath, mark)
			}
			if r.Landing != "semantic retrieval" {
				t.Errorf("%s: Landing = %q, want the phrase on one line", r.RelPath, r.Landing)
			}
		}
	}
}

// TestQuotedPhraseCrossesABlockBoundary pins the admitted cost of the phrase
// rule through the real corpus text. render.PlainText separates one block from
// the next with a single break — the same separator a hard-wrapped line leaves
// inside a paragraph — so nothing in the stored text tells a wrapped sentence
// apart from a paragraph boundary, and a quoted phrase can join the last words
// of one paragraph to the first words of the next. Answering the wrapped
// sentence is worth the pair of blocks it also joins.
func TestQuotedPhraseCrossesABlockBoundary(t *testing.T) {
	t.Parallel()
	plain := render.PlainText("intro semantic\n\nretrieval outro\n")
	// The pin below is about a block boundary; prove the fixture produced one —
	// two paragraphs joined by exactly one break — before asking search about it.
	if !strings.Contains(plain, "semantic\nretrieval") {
		t.Fatalf("render.PlainText joined the paragraphs as %q, want the two words on either side of one break", plain)
	}
	idx := NewIndex([]Document{
		{RelPath: "blocks.md", Title: "Blocks", PlainText: plain},
	}, validArtifactPolicy(t))
	got := paths(searchResults(t, idx, Parse(`"semantic retrieval"`)))
	if diff := cmp.Diff([]string{"blocks.md"}, got); diff != "" {
		t.Errorf("Search(%q) mismatch (-want +got):\n%s", `"semantic retrieval"`, diff)
	}
}

// The phrase rule still admits a match that spans two blocks. What changes is
// the landing: the first block is a text directive the page can honour, and
// the row says the rest crossed. A wrap inside one paragraph is the control
// — same words, one block, the whole phrase is still the destination.
func TestBodyMatchReportsABlockCrossingLanding(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
		DocumentFromNote(vault.Parse("Notes/Cross paragraph.md", []byte(""+
			"# Cross paragraph\n\n"+
			"The evidence records a bright\ncrimson heron near the tower.\n\n"+
			"The field notebook calls this bird cobalt\n\n"+
			"egret beside the old lighthouse.\n"))),
	}, validArtifactPolicy(t))

	tests := []struct {
		name       string
		query      string
		landing    string
		landingEnd string
		crossing   bool
	}{
		{name: "same-paragraph wrap", query: `"bright crimson"`, landing: "bright crimson"},
		{name: "cross-paragraph phrase", query: `"cobalt egret"`, landing: "cobalt", landingEnd: "egret", crossing: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := soleResult(t, idx, tt.query)
			if got.Landing != tt.landing {
				t.Errorf("Landing = %q, want %q", got.Landing, tt.landing)
			}
			if got.LandingEnd != tt.landingEnd {
				t.Errorf("LandingEnd = %q, want %q", got.LandingEnd, tt.landingEnd)
			}
			if got.BlockCrossing != tt.crossing {
				t.Errorf("BlockCrossing = %v, want %v", got.BlockCrossing, tt.crossing)
			}
		})
	}
}

// A phrase that occupies three blocks must name only the last of them as
// the end term. Returning the start of the note for every block would fold
// the middle into that term, and the browser would again be asked for a
// stretch no single element holds.
func TestAThreeBlockPhraseLandsOnTheLastBlockOnly(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
		DocumentFromNote(vault.Parse("Notes/Three blocks.md", []byte(""+
			"# Three blocks\n\n"+
			"alpha\n\n"+
			"beta\n\n"+
			"gamma\n"))),
	}, validArtifactPolicy(t))
	got := soleResult(t, idx, `"alpha beta gamma"`)
	if got.Landing != "alpha" {
		t.Errorf("Landing = %q, want alpha", got.Landing)
	}
	if got.LandingEnd != "gamma" {
		t.Errorf("LandingEnd = %q, want gamma (only the last block); a start-of-note answer would be %q", got.LandingEnd, "beta gamma")
	}
	if !got.BlockCrossing {
		t.Errorf("BlockCrossing = false, want a three-block crossing")
	}
}

// The far end of a crossing is looked for the way a term standing alone is:
// only where a word begins and where one ends. A phrase that stops inside the
// last block's word used to name that fragment, which the page has nowhere,
// and the browser abandoned the whole directive — leaving a note that opens at
// the top. The stretch goes out grown to the word instead.
//
// It opens where its block opens, so only the far end can move, and it moves
// by one word rather than to the end of the block. A close resting inside a
// script that parts no words with spaces stays where the match left it, for
// the reason the standalone form does: the edges there are a segmentation
// nothing here can find.
func TestACrossingEndTermGrowsToItsWholeWord(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
		DocumentFromNote(vault.Parse("Notes/Far end.md", []byte(""+
			"# Far end\n\n"+
			"alpha\n\n"+
			"beta\n\n"+
			"gamma opens the column.\n\n"+
			"delta\n\n"+
			"<ruby>今日<rt>きょう</rt></ruby>は晴れ、cobaltine が続く。\n"))),
	}, validArtifactPolicy(t))

	tests := []struct {
		name  string
		query string
		last  string
	}{
		{name: "an end already on both edges is left alone", query: `"alpha beta gamma"`, last: "gamma"},
		{name: "an end cut inside a word grows to the word", query: `"alpha beta gamm"`, last: "gamma"},
		{name: "one whose match keeps a single letter grows too", query: `"alpha beta g"`, last: "gamma"},
		{name: "the growth ends with the word, not with the block", query: `"alpha beta gamma op"`, last: "gamma opens"},
		{name: "a script that parts no words with spaces is not grown", query: `"delta 今日"`, last: "今日"},
		{
			// The last block is one the page does not draw the way this text
			// carries it: the reading is spoken over characters the page
			// writes it in among, and this text keeps it aside. The growth
			// cannot reach it — it stops at the first character a word does
			// not join, and every character a reading is written over is one
			// of those — so the far end is grown here as anywhere.
			name:  "a block the page draws differently grows its far end all the same",
			query: `"delta 今日は晴れ、cobaltin"`,
			last:  "今日は晴れ、cobaltine",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := soleResult(t, idx, tt.query)
			if !got.BlockCrossing {
				t.Fatalf("Search(%q) did not cross a block, so it has no far end to name", tt.query)
			}
			if got.LandingEnd != tt.last {
				t.Errorf("LandingEnd = %q, want %q (Landing = %q)", got.LandingEnd, tt.last, got.Landing)
			}
		})
	}
}

// A crossing whose first stretch collapses to nothing travels on its far end
// alone, and a browser then holds that one stretch to both its edges with
// nothing to relax the rule. The growth stays inside the block the match
// closed in: a word cannot be assembled out of two blocks the page draws
// apart, and a note whose blocks part inside a run of letters is what says so.
func TestALoneFarEndGrowsInsideItsOwnBlock(t *testing.T) {
	t.Parallel()

	plain := "abc def    \nghi"
	whole := []render.Block{
		{End: strings.Index(plain, "\n"), Verbatim: true},
		{End: len(plain), Verbatim: true},
	}
	// The same text read as three blocks, the second of which stops one
	// letter short of the run's end. The ceiling is that block end, not the
	// end of the text.
	parted := []render.Block{
		{End: strings.Index(plain, "\n"), Verbatim: true},
		{End: len(plain) - 1, Verbatim: true},
		{End: len(plain), Verbatim: true},
	}

	tests := []struct {
		name, query string
		blocks      []render.Block
		last        string
	}{
		{name: "a far end at a word's end stays", query: `"  ghi"`, blocks: whole, last: "ghi"},
		{name: "the far end grows to the word it was cut inside", query: `"  gh"`, blocks: whole, last: "ghi"},
		{name: "and stops where its own block does", query: `"  gh"`, blocks: parted, last: "gh"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			idx := NewIndex([]Document{{
				RelPath:   "Notes/Gaps.md",
				Title:     "Gaps",
				PlainText: plain,
				Blocks:    tt.blocks,
			}}, validArtifactPolicy(t))

			got := soleResult(t, idx, tt.query)
			if got.Landing != "" || got.LandingBare != "" {
				t.Fatalf("Landing = %q / LandingBare = %q, want both empty: the match opens in the gap after %q",
					got.Landing, got.LandingBare, "def")
			}
			if got.LandingEnd != tt.last {
				t.Errorf("LandingEnd = %q, want %q", got.LandingEnd, tt.last)
			}
		})
	}
}

// A one-character CJK query at the end of a block used to report a crossing
// because the fold drops the break and the next kept rune sits in the next
// block. The match itself does not cross.
func TestACharacterAtABlockEndIsNotACrossing(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
		DocumentFromNote(vault.Parse("Notes/cjk-end.md", []byte(""+
			"# CJK end\n\n"+
			"第一段落結尾鈷\n\n"+
			"鷺在第二段。\n"))),
	}, validArtifactPolicy(t))
	got := soleResult(t, idx, "鈷")
	if got.BlockCrossing {
		t.Errorf("BlockCrossing = true for a one-character match at a block end; landing = %q", got.Landing)
	}
	if got.Landing != "鈷" {
		t.Errorf("Landing = %q, want 鈷", got.Landing)
	}
}

// TestCrossingLandingSurvivesAnNFDCharacter holds the block ends to the
// normalised text: a character that recomposes before a crossing phrase must
// not cost the phrase its block ends.
func TestCrossingLandingSurvivesAnNFDCharacter(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
		DocumentFromNote(vault.Parse("Notes/nfd.md", []byte(""+
			"# NFD\n\n"+
			"caf\u0065\u0301 sits in the opening.\n\n"+
			"The evidence records a bright\n\n"+
			"crimson heron.\n"))),
	}, validArtifactPolicy(t))
	got := soleResult(t, idx, `"bright crimson"`)
	if got.Landing != "bright" || got.LandingEnd != "crimson" || !got.BlockCrossing {
		t.Errorf("Landing = %q / %q crossing = %v, want bright / crimson / true",
			got.Landing, got.LandingEnd, got.BlockCrossing)
	}
}

// A domain can carry a space, and the way to name one is the same convention
// every search box uses: quote the value. Quoting had exempted the whole
// field from being read as a filter, so the idiom looked supported and
// quietly ran as body text over notes that never write their own domain down.
func TestQuotedFilterValueStillFilters(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
		{RelPath: "a.md", Title: "A", Domain: "functional programming", PlainText: "one"},
		{RelPath: "b.md", Title: "B", Domain: "functional", PlainText: "two"},
		{RelPath: "c.md", Title: "C", PlainText: "domain:functional programming written out in the body"},
	}, schema.ArtifactPolicy{})

	got := paths(searchResults(t, idx, Parse(`domain:"functional programming"`)))
	if diff := cmp.Diff([]string{"a.md"}, got); diff != "" {
		t.Errorf(`Search(domain:"functional programming") mismatch (-want +got):`+"\n%s", diff)
	}

	// The other direction stands unchanged: quote the whole field and it is
	// text, matched wherever those characters sit together.
	quoted := paths(searchResults(t, idx, Parse(`"domain:functional programming"`)))
	if diff := cmp.Diff([]string{"c.md"}, quoted); diff != "" {
		t.Errorf(`Search("domain:functional programming") mismatch (-want +got):`+"\n%s", diff)
	}
}

// The words a match follows come out of the match's own block. A phrase that
// opens on the last word of a paragraph gives the browser a one-word stretch
// to find, and it finds the first copy anywhere on the page; the run ahead of
// it is what tells that copy from this one. The run is cut at the block start,
// because a browser reads a leading run as a prefix only where it sits in the
// same block as the term it introduces.
func TestLandingNamesTheWordsTheMatchFollowsInsideItsOwnBlock(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
		DocumentFromNote(vault.Parse("Notes/Prefix.md", []byte(""+
			"# Prefix\n\n"+
			"The decoy cobalt sits in the opening paragraph.\n\n"+
			"The field notebook calls this bird cobalt\n\n"+
			"egret beside the old lighthouse.\n\n"+
			"alpha\n\n"+
			"beta\n"))),
	}, validArtifactPolicy(t))

	tests := []struct {
		name   string
		query  string
		prefix string
	}{
		{
			name:   "a crossing phrase names the last words of its first block",
			query:  `"cobalt egret"`,
			prefix: "calls this bird",
		},
		{
			// The previous block ends "…paragraph." and none of it may be
			// named: a run reaching back over the boundary is a directive the
			// browser throws out, not a longer one.
			name:  "a match that opens its block names nothing",
			query: `"alpha beta"`,
		},
		{
			name:   "an ordinary hit names them too",
			query:  "lighthouse",
			prefix: "beside the old",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := soleResult(t, idx, tt.query)
			if got.LandingPrefix != tt.prefix {
				t.Errorf("LandingPrefix = %q, want %q", got.LandingPrefix, tt.prefix)
			}
			// Every match here ends against white space, a full stop or the end of
			// its block, which are boundaries already: nothing follows to name,
			// and the directives stay as they were.
			if got.LandingSuffix != "" {
				t.Errorf("LandingSuffix = %q, want none", got.LandingSuffix)
			}
		})
	}
}

// A reader who types the middle of a word matches the middle of it, and a
// stretch handed to a browser with nothing ahead of it is looked for only
// where a word begins and where one ends. So the stretch a directive can name
// alone is the match grown out to the edges of the words it lies inside.
//
// Which characters a word is made of is the browser's question, not this
// index's: it joins letters and digits and breaks at everything else, so a
// comma ends a word and a hyphen does too. The run white space alone would
// bound is a different thing and a longer one — in a sentence whose words are
// not parted by spaces it is the whole sentence, and handing a browser that
// finds nothing.
func TestLandingGrowsAMatchToTheWordsAroundIt(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
		DocumentFromNote(vault.Parse("Notes/Bare.md", []byte(""+
			"# Bare\n\n"+
			"The ledger records molybdenum here.\n\n"+
			"A read-aloud button sits below.\n\n"+
			"今日は晴れ、cobaltine が続く。\n"))),
	}, validArtifactPolicy(t))

	tests := []struct {
		name  string
		query string
		bare  string
	}{
		{name: "a match already on both edges is left alone", query: "molybdenum", bare: "molybdenum"},
		{name: "a tail grows back to the head of its word", query: "lybdenum", bare: "molybdenum"},
		{name: "a head grows on to the end of its word", query: "molybden", bare: "molybdenum"},
		{name: "a match inside a word grows at both ends", query: "olybden", bare: "molybdenum"},
		{
			// A browser breaks a word at the hyphen, so the second half of
			// this one already opens where a word does and is not dragged
			// back across it.
			name:  "a hyphen is an edge, not something to grow across",
			query: "aloud",
			bare:  "aloud",
		},
		{
			// The hyphen inside the stretch is not at either end of it, so it
			// stops nothing: both ends still grow to the word they are in.
			name:  "a hyphen inside the stretch stops neither end",
			query: "ead-alo",
			bare:  "read-aloud",
		},
		{
			// White space parts no words here, so the run one would bound
			// reaches back through the comma to the start of the sentence.
			// The word ends at the comma, and so does the growth.
			name:  "a comma bounds the word, not the sentence around it",
			query: "baltine",
			bare:  "cobaltine",
		},
		{
			name:  "a word already whole inside such a sentence is left alone",
			query: "cobaltine",
			bare:  "cobaltine",
		},
		{
			// The edges of a word are a segmentation nothing here can find,
			// so the stretch is handed over exactly as the match left it.
			name:  "a script that parts no words with spaces is not grown",
			query: "晴れ",
			bare:  "晴れ",
		},
		{
			name:  "nor is one whose match ends against a comma",
			query: "れ、",
			bare:  "れ、",
		},
		{
			// Each end is asked on its own, so a match that opens in one
			// script and closes inside a word of the other grows only at the
			// end that can be grown. Nothing is owed here — a stretch opening
			// mid-word in such a script is not found either way — but the two
			// ends are decided separately and this is what that looks like.
			name:  "an end that can grow does, whatever the other end is",
			query: "れ、cobaltin",
			bare:  "れ、cobaltine",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := soleResult(t, idx, tt.query)
			if got.LandingBare != tt.bare {
				t.Errorf("LandingBare = %q, want %q (Landing = %q)", got.LandingBare, tt.bare, got.Landing)
			}
		})
	}
}

// The run is written for a browser to find again in rendered text, so it is
// kept short. Words come off the front whole, and nothing is ever cut inside
// what strings.Fields calls one word: a browser looks for a leading run only
// where the run begins at a word boundary, and in prose that parts no words
// with spaces the boundaries are dictionary ones this index cannot see. Where
// that script leaves a word over the budget the run begins at a boundary every
// segmenter agrees on instead — after punctuation, or at the start of the
// block — and a word in any other script is dropped as it always was.
func TestLandingPrefixKeepsWhatABrowserCanFindAgain(t *testing.T) {
	t.Parallel()

	const (
		// Thirty characters and thirty-one, the two sides of the budget.
		thirty    = "一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十"
		thirtyOne = "零" + thirty
	)
	tests := []struct {
		name string
		run  string
		want string
	}{
		{name: "nothing ahead of the match", run: "", want: ""},
		{name: "fewer words than the budget", run: "calls this", want: "calls this"},
		{name: "the last words, not the first", run: "one two three four five", want: "three four five"},
		{
			// A tail of it would open between two characters the reader's
			// language reads as one word, which a browser looks for and never
			// finds — and a leading run it cannot find costs it the term as
			// well, so the note opens at the top instead of at the match. The
			// block start is a boundary the browser agrees with, so the run
			// goes back to it.
			name: "a paragraph written without spaces and without punctuation is named whole",
			run:  "第一段落的開頭寫得很長很長很長很長很長很長很長很長很長很長很長很長很長很長結尾",
			want: "第一段落的開頭寫得很長很長很長很長很長很長很長很長很長很長很長很長很長很長結尾",
		},
		{
			// The comma is a boundary every segmenter breaks at, and what
			// follows it is inside the budget.
			name: "punctuation inside the budget is where the run begins",
			run:  "第一段落的開頭寫得很長很長很長很長很長很長很長很長很長很長很長，接著在這一行裡面才輪到",
			want: "接著在這一行裡面才輪到",
		},
		{
			name: "the earliest boundary that fits is the one used",
			run:  "第一段落的開頭寫得很長很長很長很長很長很長很長很長很長很長很長，接著在這一行，裡面才輪到",
			want: "接著在這一行，裡面才輪到",
		},
		{
			// Thirty characters exactly after the comma: the budget is a
			// limit, not a trigger.
			name: "a boundary that leaves exactly the budget is inside it",
			run:  "第一段落的開頭寫得很長很長很長很長很長很長，" + thirty,
			want: thirty,
		},
		{
			// One over: the comma leaves thirty-one, outside the budget, but
			// it is still the nearest boundary, so the run begins there.
			name: "a boundary one character outside the budget is still the nearest one",
			run:  "第一段落的開頭寫得很長很長很長很長很長很長，" + thirtyOne,
			want: thirtyOne,
		},
		{
			// The page draws a hard line break as a break, so a run that
			// spanned one would not be text it carries in one piece.
			name: "the run stops at a hard line break",
			run:  "第一行寫得很長很長很長很長很長很長很長很長很長很長很長很長\n第二行也寫得很長很長很長很長很長很長很長很長很長很長很長",
			want: "第二行也寫得很長很長很長很長很長很長很長很長很長很長很長",
		},
		{
			name: "a match opening a line after a hard break names nothing",
			run:  "第一行寫得很長很長很長很長很長很長很長很長很長很長很長很長很長很長很長\n",
			want: "",
		},
		{
			// The middle dot is part of a katakana word; a run cut after it
			// would begin inside one.
			name: "a middle dot between katakana is not a boundary",
			run:  "カフェオレ・ミルクコーヒー・エスプレッソ・ラテマキアート・カプチーノ",
			want: "カフェオレ・ミルクコーヒー・エスプレッソ・ラテマキアート・カプチーノ",
		},
		{
			// 30 characters exactly: the budget is a limit, not a trigger.
			name: "a run at the budget is kept whole",
			run:  thirty,
			want: thirty,
		},
		{
			// 51 characters over three words; dropping the first leaves 26,
			// which is inside the budget, so the second is kept whole.
			name: "words come off the front one at a time, and no further",
			run:  "pneumonoultramicroscopic silicovolcanoconiosis tail",
			want: "silicovolcanoconiosis tail",
		},
		{
			// Halving it would name a run that opens inside a word, which
			// matches nothing, so the row keeps its bare term instead. The
			// term that follows is grown to its own word's edges, so it
			// stands alone.
			name: "one over-long word is dropped rather than halved",
			run:  "pneumonoultramicroscopicsilicovolcanoconiosis",
			want: "",
		},
		{
			name: "an over-long word in a script with spaces is dropped whatever it holds",
			run:  "pneumonoultramicroscopic.silicovolcanoconiosis-pneumono",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := landingPrefix(tt.run); got != tt.want {
				t.Errorf("landingPrefix(%q) = %q, want %q", tt.run, got, tt.want)
			}
		})
	}
}

// The run after a term is the mirror of the run before it. A browser holds a
// term with nothing behind it to end where a word does, and where the script
// parts no words with spaces that is a dictionary boundary this index cannot
// see; a run behind it that itself ends at white space, punctuation or the end
// of the block is one every segmenter agrees on, and releases the term. The
// run ends at the nearest such boundary inside the budget, and with none there
// it is the rest of the block.
func TestLandingSuffixEndsWhereABrowserAgreesAWordEnds(t *testing.T) {
	t.Parallel()

	unbroken := strings.Repeat("一二三四五六七八九十", 4)
	tests := []struct {
		name   string
		before string // the last character of the term the run follows
		rest   string // the block from the end of that term to its own end
		want   string
	}{
		{name: "nothing after the term", before: "e", rest: "", want: ""},
		{name: "a term that ends against white space", before: "e", rest: " here.", want: ""},
		{name: "a term that ends against punctuation", before: "e", rest: ", here.", want: ""},
		{name: "a term that stops inside a word of a script with spaces", before: "n", rest: "um here", want: "um"},
		{name: "a run to a comma", before: "陽", rest: "花の株で、今年も青い花をつけた", want: "花の株で"},
		{name: "a run to a full stop", before: "讀", rest: "用的導言。第二句", want: "用的導言"},
		{name: "a fullwidth comma bounds it in Chinese", before: "陽", rest: "花的株，今年也開了", want: "花的株"},
		{name: "a long run still ends at the nearest boundary", before: "獸", rest: unbroken + "。後續", want: unbroken},
		{name: "a run with no boundary is the rest of the block", before: "獸", rest: unbroken, want: unbroken},
		{name: "a hard line break ends the run", before: "獸", rest: unbroken + "\n後續", want: unbroken},
		{name: "an apostrophe inside a word joins it", before: "n", rest: "'t stop here", want: "'t"},
		{name: "a point inside a number joins it", before: "3", rest: ".14 more", want: ".14"},
		{name: "a point after a word ends it", before: "e", rest: ". More", want: ""},
		{name: "a middle dot between katakana joins them", before: "コ", rest: "・ミルク。", want: "・ミルク"},
		{name: "white space ends the run", before: "獸", rest: "三個\n 字出現。", want: "三個"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			text := tt.before + tt.rest
			if got := landingSuffix(text, len(tt.before), len(text)); got != tt.want {
				t.Errorf("landingSuffix(%q after %q) = %q, want %q", tt.rest, tt.before, got, tt.want)
			}
		})
	}
}

// The reading interface is written in Traditional Chinese and so are the notes
// it was built for, where a match is ordinarily reached after a long run of
// characters nothing has parted into words. A run cut to the budget there
// opens between two characters the reader's language reads as one word; a
// browser looks for such a run, finds it nowhere, and throws away the term it
// introduced along with it, so a note that used to open at its match opens at
// the top.
//
// The run goes back to the nearest boundary every segmenter agrees on that
// leaves it inside the budget — here the fullwidth comma, which breaks a word
// in any language — and the term is followed by the characters up to the full
// stop, so that the match is free to end anywhere. The second row is what
// keeps the first honest: a run inside the budget is still named, and it is
// the one the block opens with, so this test fails just as loudly on a change
// that answers it by naming nothing at all.
func TestLandingNamesNoRunItWouldHaveToCutInsideAWord(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
		DocumentFromNote(vault.Parse("Notes/Ledger.md", []byte(""+
			"# 帳冊\n\n"+
			"那本咖啡色封皮的舊冊子在星期四早晨被翻開來，接著在這一行裡面才輪到獨角獸三個字出現。\n\n"+
			"灰布下面寫著飛馬兩個字。\n"))),
	}, validArtifactPolicy(t))

	tests := []struct {
		name   string
		query  string
		prefix string
		suffix string
	}{
		{
			// Thirty-three characters ahead of the match, three over the
			// budget: a cut at the budget would fall between 咖 and 啡, and
			// the comma leaves eleven.
			name:   "a match reached after a long run begins its run at the comma",
			query:  "獨角獸",
			prefix: "接著在這一行裡面才輪到",
			suffix: "三個字出現",
		},
		{
			name:   "a run that fits is named, because the block opens where it opens",
			query:  "飛馬",
			prefix: "灰布下面寫著",
			suffix: "兩個字",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := soleResult(t, idx, tt.query)
			if got.LandingPrefix != tt.prefix {
				t.Errorf("LandingPrefix = %q, want %q", got.LandingPrefix, tt.prefix)
			}
			if got.LandingSuffix != tt.suffix {
				t.Errorf("LandingSuffix = %q, want %q", got.LandingSuffix, tt.suffix)
			}
		})
	}
}

// The run after the match is measured from the end of the last term a
// directive names, wherever growth left it, and follows that term into the
// block it sits in. A match ending against a boundary already has nothing
// after it to name; one that stops inside a word of a script without spaces
// names the characters up to the next boundary, or the rest of the block.
func TestLandingSuffixFollowsTheTermItIsMeasuredFrom(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
		DocumentFromNote(vault.Parse("Notes/Suffix.md", []byte(""+
			"# Suffix\n\n"+
			"夜が明けるとすぐに窓辺の椅子に腰をおろして空の色を長いあいだ眺めていた老人はその日もよく晴れ、それから静かに茶を淹れた。\n\n"+
			"庭の隅にひっそりと咲いていたのは去年の梅雨に隣の家から分けてもらった紫陽花の株で、今年も青い花をつけた。\n\n"+
			"その朝に限って玄関の外がいつになく明るく空は朝のうちからすっかり晴々として遠くの山の稜線までくっきりと見えていた\n\n"+
			"The clerk kept a ledger of tourmaline samples on the shelf.\n\n"+
			"Cobaltineが続く。\n\n"+
			"<ruby>今日<rt>きょう</rt></ruby>は雨模様、ぼんやりとした窓の外を眺めるだけ。\n"))),
	}, validArtifactPolicy(t))

	tests := []struct {
		name   string
		query  string
		prefix string
		suffix string
	}{
		{
			// The match ends on the comma, a boundary, and it opens inside
			// 晴れ. What the browser needs here is the run before it, which
			// goes back to the block start because nothing in it is a
			// boundary; the run after is named all the same.
			name:   "the report's own case names the run before and the run after",
			query:  "れ、",
			prefix: "夜が明けるとすぐに窓辺の椅子に腰をおろして空の色を長いあいだ眺めていた老人はその日もよく晴",
			suffix: "それから静かに茶を淹れた",
		},
		{
			name:   "a match ending inside a word names the run up to the comma",
			query:  "紫陽",
			prefix: "庭の隅にひっそりと咲いていたのは去年の梅雨に隣の家から分けてもらった",
			suffix: "花の株で",
		},
		{
			name:   "with no boundary before the block ends, the rest of the block is named",
			query:  "晴々",
			prefix: "その朝に限って玄関の外がいつになく明るく空は朝のうちからすっかり",
			suffix: "として遠くの山の稜線までくっきりと見えていた",
		},
		{
			name:   "a Latin word grown to its edges is followed by nothing to name",
			query:  "urmalin",
			prefix: "ledger of to",
		},
		{
			// The word is grown to its edge, and the next character belongs
			// to a script whose words part at no space, which is not a
			// boundary this index is certain of.
			name:   "a Latin word followed directly by kana names the kana up to the full stop",
			query:  "cobaltin",
			suffix: "が続く",
		},
		{
			// The page draws the reading in among the characters it is
			// spoken over, so a run beside the term is not text it carries
			// in one piece: neither side of the match is named.
			name:  "a block the page does not reproduce as written names neither run",
			query: "雨模",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := soleResult(t, idx, tt.query)
			if got.LandingPrefix != tt.prefix {
				t.Errorf("LandingPrefix = %q, want %q", got.LandingPrefix, tt.prefix)
			}
			if got.LandingSuffix != tt.suffix {
				t.Errorf("LandingSuffix = %q, want %q", got.LandingSuffix, tt.suffix)
			}
		})
	}
}

// A crossing match is looked for as one directive, and the run after it
// belongs to the term it follows, which is the far end. Whether the page
// reproduces the block that term sits in decides whether the run is named, and
// the first block's own answer does not: each block is asked for itself.
func TestLandingSuffixOfACrossingMatchFollowsTheFarEnd(t *testing.T) {
	t.Parallel()

	plain := "鍵はすべて銀杏\n並木の奥にある小さな倉庫の錠前に合うと"
	split := strings.Index(plain, "\n")
	tests := []struct {
		name      string
		firstOK   bool
		lastOK    bool
		wantStart string
		suffix    string
	}{
		{name: "both blocks are reproduced as written", firstOK: true, lastOK: true, wantStart: "銀杏", suffix: "前に合うと"},
		{name: "a far block the page draws differently names no run after", firstOK: true, lastOK: false, wantStart: "銀杏"},
		{name: "a first block drawn differently does not withhold the far run", firstOK: false, lastOK: true, wantStart: "銀杏", suffix: "前に合うと"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			idx := NewIndex([]Document{{
				RelPath:   "Notes/Crossing.md",
				Title:     "Crossing",
				PlainText: plain,
				Blocks: []render.Block{
					{End: split, Verbatim: tt.firstOK},
					{End: len(plain), Verbatim: tt.lastOK},
				},
			}}, validArtifactPolicy(t))

			got := soleResult(t, idx, `"銀杏並木の奥にある小さな倉庫の錠"`)
			if !got.BlockCrossing {
				t.Fatalf("BlockCrossing = false, want a crossing")
			}
			if got.Landing != tt.wantStart || got.LandingEnd != "並木の奥にある小さな倉庫の錠" {
				t.Errorf("Landing = %q, LandingEnd = %q, want %q and the far stretch", got.Landing, got.LandingEnd, tt.wantStart)
			}
			if got.LandingSuffix != tt.suffix {
				t.Errorf("LandingSuffix = %q, want %q", got.LandingSuffix, tt.suffix)
			}
		})
	}
}
