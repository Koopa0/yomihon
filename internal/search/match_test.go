package search

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/ui/pages"
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

// undeclaredArtifactPolicy is a contract that loaded and left [artifacts] out
// altogether. It is not the zero value: the zero value belongs to a folder that
// carries no contract, which excluded nothing on purpose, while this one meant
// to exclude directories yomihon cannot name.
func undeclaredArtifactPolicy(tb testing.TB) schema.ArtifactPolicy {
	tb.Helper()
	return artifactPolicyFromSection(tb, "")
}

func artifactPolicyFromSection(tb testing.TB, section string) schema.ArtifactPolicy {
	tb.Helper()
	path := filepath.Join(tb.TempDir(), "vault-schema.toml")
	contract := `schema_version = "1"

[enums]
type = ["concept"]

[enums.status]
note = ["draft"]

[fields]
known = ["based_on"]

[rules]
concept_requires_provenance = ["based_on"]

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

func searchResults(tb testing.TB, idx *Index, q *Query) []Result {
	tb.Helper()
	results, err := idx.Search(q)
	if err != nil {
		tb.Fatalf("Search() error: %v", err)
	}
	return results
}

func TestSearchSnippetRequiresBodyEvidence(t *testing.T) {
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
	if text[0].RelPath != "title-and-body.md" || text[0].Snippet == "" {
		t.Errorf("title+body result = %+v, want body evidence", text[0])
	}
	if text[1].RelPath != "title-only.md" || text[1].Snippet != "" {
		t.Errorf("title-only result = %+v, want no snippet", text[1])
	}

	filtered := searchResults(t, idx, Parse("type:concept"))
	if len(filtered) != 1 || filtered[0].Snippet != "" {
		t.Errorf("pure-filter result = %+v, want no snippet", filtered)
	}
}

func TestSearchNonInstanceCapability(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
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
		{name: "undeclared", policy: undeclaredArtifactPolicy(t)},
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
			idx := NewIndex([]Document{{
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

func TestWithArtifactPolicyBindsIndependentMetadataAuthority(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{{
		RelPath:  "Concepts/Note.md",
		Title:    "Note",
		NoteType: "concept",
		Status:   "draft",
	}}, validArtifactPolicy(t))
	closed := idx.WithArtifactPolicy(undeclaredArtifactPolicy(t))

	openResults, err := idx.Search(Parse("Note"))
	if err != nil {
		t.Fatalf("original Search(Note) error = %v", err)
	}
	if len(openResults) != 1 || openResults[0].Status != "draft" {
		t.Fatalf("original Search(Note) = %+v, want draft metadata badge", openResults)
	}
	closedResults, err := closed.Search(Parse("Note"))
	if err != nil {
		t.Fatalf("closed Search(Note) error = %v", err)
	}
	if len(closedResults) != 1 || closedResults[0].Status != "" {
		t.Errorf("closed Search(Note) = %+v, want lexical result without metadata badge", closedResults)
	}
	if _, err := closed.Search(Parse("status:draft")); !errors.Is(err, ErrMetadataUnavailable) {
		t.Errorf("closed Search(status:draft) error = %v, want ErrMetadataUnavailable", err)
	}
	if _, err := idx.Search(Parse("status:draft")); err != nil {
		t.Errorf("original Search(status:draft) error = %v, want functional copy not to mutate original", err)
	}
}

// TestZeroValueIndexAnswersNothingWithoutInventingAFault pins the zero Index,
// which holds no documents and no declared exclusions. Every projection over it
// is empty, and empty is the honest answer — the index has nothing to withhold,
// so it does not report a capability failure it did not have.
func TestZeroValueIndexAnswersNothingWithoutInventingAFault(t *testing.T) {
	t.Parallel()

	idx := &Index{}
	results, err := idx.Search(Parse("type:concept"))
	if err != nil {
		t.Errorf("zero Index.Search(type:concept) error = %v, want nil", err)
	}
	if len(results) != 0 {
		t.Errorf("zero Index.Search(type:concept) = %v, want no results", results)
	}
	typeCounts, err := idx.CountByTypeStatus()
	if err != nil || len(typeCounts) != 0 {
		t.Errorf("zero Index.CountByTypeStatus() = (%v, %v), want an empty tally and no error", typeCounts, err)
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
		{RelPath: "Concepts/Note.md", Title: "Note", NoteType: "concept", Status: "draft"},
	}, policy)

	_, err := idx.Search(Parse("status:draft"))
	if !errors.Is(err, ErrMetadataUnavailable) {
		t.Fatalf("Search(status:draft) error = %v, want ErrMetadataUnavailable", err)
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
		{RelPath: "Concepts/Note.md", Title: "Note", NoteType: "concept", Status: "draft", PlainText: "needle"},
		{RelPath: "System/templates/Card.md", Title: "Card", NoteType: "lesson", PlainText: "needle"},
	}, schema.ArtifactPolicy{})

	results, err := idx.Search(Parse("type:concept"))
	if err != nil {
		t.Fatalf("Search(type:concept) with an unclaimed policy error = %v, want an answer", err)
	}
	if diff := cmp.Diff([]string{"Concepts/Note.md"}, paths(results)); diff != "" {
		t.Errorf("Search(type:concept) paths mismatch (-want +got):\n%s", diff)
	}
	// Nothing declared System/templates as an artifact, so it is an ordinary
	// note and a plain query reaches it.
	plain, err := idx.Search(Parse("needle"))
	if err != nil {
		t.Fatalf("Search(needle) error = %v", err)
	}
	if len(plain) != 2 {
		t.Errorf("Search(needle) = %v, want both notes: nothing was excluded", paths(plain))
	}
	counts, err := idx.CountByTypeStatus()
	if err != nil {
		t.Fatalf("CountByTypeStatus() with an unclaimed policy error = %v", err)
	}
	if got := counts[TypeStatus{Type: "concept", Status: "draft"}]; got != 1 {
		t.Errorf("CountByTypeStatus()[concept/draft] = %d, want 1", got)
	}
}

// filterFixture is a small index exercising each filter and the folder
// boundary. The RelPaths are chosen so "Writing-old/..." sorts BEFORE
// "Writing/..." ('-' 0x2D < '/' 0x2F), which pins the rel_path result order.
func filterFixture(tb testing.TB) *Index {
	tb.Helper()
	return NewIndex([]Document{
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

// TestSearchOrdering pins the opening of the six-group order: a note's title
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

// TestTheSixAnswerGroupsComeBackInRankedOrder pins the whole of the result
// order in one query. The six groups are the contract — a note's title, a
// note's body, the same two over vault files, then the notes and files matched
// only by where they live — and the six fixtures are laid out so their path
// order is the exact reverse of their group order, which is what makes the
// assertion discriminate between the two.
func TestTheSixAnswerGroupsComeBackInRankedOrder(t *testing.T) {
	t.Parallel()
	idx := NewIndex([]Document{
		{RelPath: "a-kafka/data.txt", Title: "data.txt", PlainText: "opaque", File: true},
		{RelPath: "b-kafka/inside.md", Title: "Inside", PlainText: "unrelated"},
		{RelPath: "c/notes.txt", Title: "notes.txt", PlainText: "mentions kafka once", File: true},
		{RelPath: "d/kafka.txt", Title: "kafka.txt", PlainText: "plain", File: true},
		{RelPath: "e/note.md", Title: "Streaming", PlainText: "a kafka pipeline"},
		{RelPath: "f/note.md", Title: "Kafka guide", PlainText: "nothing else"},
	}, validArtifactPolicy(t))

	want := []string{
		"f/note.md",         // a note named by the query
		"e/note.md",         // a note whose prose says it
		"d/kafka.txt",       // a file named by the query
		"c/notes.txt",       // a file whose characters say it
		"b-kafka/inside.md", // a note only its address matches
		"a-kafka/data.txt",  // a file only its address matches
	}
	if diff := cmp.Diff(want, paths(searchResults(t, idx, Parse("kafka")))); diff != "" {
		t.Errorf("Search(kafka) group order mismatch (-want +got):\n%s", diff)
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

// TestSearchNFDContent pins that NFC-form query text matches NFD-form content.
// The body holds NFD が ("が" = か + U+3099); the query is NFC が ("が"). Both
// sides fold to NFC, so the match succeeds — this is the whole reason fold
// normalizes NFC on both index and query.
func TestSearchNFDContent(t *testing.T) {
	t.Parallel()
	idx := NewIndex([]Document{
		{RelPath: "n.md", Title: "note", PlainText: "reading \u304b\u3099 today"},
	}, validArtifactPolicy(t))
	got := paths(searchResults(t, idx, Parse("\u304c")))
	if diff := cmp.Diff([]string{"n.md"}, got); diff != "" {
		t.Errorf("NFC query against NFD content mismatch (-want +got):\n%s", diff)
	}
}

// TestFoldIsSimpleLowercaseNotFullCaseFold pins what fold is: NFC then simple
// lowercase, nothing wider. NFC leaves character width alone, so fullwidth \uff21
// folds to fullwidth \uff41 and never meets an ASCII a, and halfwidth \uff76 never
// meets \u30ab \u2014 a compatibility normalization would merge both. The lowercase is
// the simple per-character mapping, so \u00df stays \u00df rather than expanding to ss
// (\u1e9e still lowers to \u00df; that much is the simple mapping). What a reader
// feels: an ASCII query does not find fullwidth text, and ss does not find \u00df.
func TestFoldIsSimpleLowercaseNotFullCaseFold(t *testing.T) {
	t.Parallel()
	folds := []struct {
		name string
		in   string
		want string
	}{
		{"fullwidth capital lowers within its width", "\uff21", "\uff41"},
		{"halfwidth katakana stays halfwidth", "\uff76", "\uff76"},
		{"sharp s does not expand", "\u00df", "\u00df"},
		{"capital sharp s lowers to sharp s", "\u1e9e", "\u00df"},
	}
	for _, tt := range folds {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := fold(tt.in); got != tt.want {
				t.Errorf("fold(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}

	idx := NewIndex([]Document{
		{RelPath: "wide.md", Title: "wide", PlainText: "\uff21\uff22\uff23 \u5927\u5beb"},
		{RelPath: "river.md", Title: "river", PlainText: "der Flu\u00df heute"},
	}, validArtifactPolicy(t))
	for _, query := range []string{"abc", "ss"} {
		if got := paths(searchResults(t, idx, Parse(query))); len(got) != 0 {
			t.Errorf("Search(%q) = %v, want no hits under a simple fold", query, got)
		}
	}
	// The zeroes above prove nothing if the fixture text is unreachable, so
	// reach each note by the form the fold does preserve.
	if got := paths(searchResults(t, idx, Parse("\uff42\uff43"))); !slices.Equal(got, []string{"wide.md"}) {
		t.Errorf("Search(\uff42\uff43) = %v, want the fullwidth text found by a fullwidth query", got)
	}
	if got := paths(searchResults(t, idx, Parse("Flu\u00df"))); !slices.Equal(got, []string{"river.md"}) {
		t.Errorf("Search(Flu\u00df) = %v, want the sharp-s text found by a sharp-s query", got)
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

// TestCountByTypeStatus tallies notes by (type, status) together — the pairing a
// status-keyed tally cannot express, since two notes sharing a status but not a
// type must stay separate. It also pins the two edges the pair carries alone: a
// note outside the governed instance set is left out, and a note missing a field
// falls in that field's "" bucket. The type/status words are invented so the
// count is what is proven, not any real vault's vocabulary.
func TestCountByTypeStatus(t *testing.T) {
	t.Parallel()
	idx := NewIndex([]Document{
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

func TestCountsUnavailableWithoutArtifactPolicy(t *testing.T) {
	t.Parallel()

	policies := []struct {
		name   string
		policy schema.ArtifactPolicy
	}{
		{name: "undeclared", policy: undeclaredArtifactPolicy(t)},
		{name: "invalid", policy: invalidArtifactPolicy(t)},
		{name: "incomplete", policy: incompleteArtifactPolicy(t)},
	}
	for _, tt := range policies {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			idx := NewIndex([]Document{{RelPath: "Concepts/Note.md", NoteType: "concept", Status: "draft"}}, tt.policy)

			if got, err := idx.CountByTypeStatus(); !errors.Is(err, ErrMetadataUnavailable) {
				t.Errorf("CountByTypeStatus() = (%v, %v), want ErrMetadataUnavailable", got, err)
			} else if err.Error() != tt.policy.Diagnostic() {
				t.Errorf("CountByTypeStatus() error = %q, want %q", err.Error(), tt.policy.Diagnostic())
			}
		})
	}
}

// TestNewIndexDeterministicUnderAnyInputOrder pins that input order does not
// leak into the built index: every permutation of the same documents yields an
// identical index, with entries in strictly increasing RelPath order. Strict
// increase is the invariant the determinism rests on — the build sorts with an
// unstable sort, which is total only because real inputs never repeat a path
// (the scanner refuses canonical-path collisions before a document is made).
// Building one slice twice cannot catch any of this; permuting can.
func TestNewIndexDeterministicUnderAnyInputOrder(t *testing.T) {
	t.Parallel()
	docs := []Document{
		{RelPath: "b.md", Title: "B", PlainText: "beta"},
		{RelPath: "a.md", Title: "A", Topics: []string{"x"}, PlainText: "alpha"},
		{RelPath: "d.txt", Title: "d.txt", PlainText: "delta", File: true},
		{RelPath: "c.md", Title: "C", PlainText: "gamma"},
	}
	policy := validArtifactPolicy(t)
	first := NewIndex(docs, policy)
	for i := 1; i < len(first.entries); i++ {
		if first.entries[i-1].RelPath >= first.entries[i].RelPath {
			t.Fatalf("entries[%d..%d] = %q, %q: not strictly increasing by RelPath",
				i-1, i, first.entries[i-1].RelPath, first.entries[i].RelPath)
		}
	}
	for _, perm := range permutations(len(docs)) {
		shuffled := make([]Document, 0, len(docs))
		for _, at := range perm {
			shuffled = append(shuffled, docs[at])
		}
		got := NewIndex(shuffled, policy)
		if diff := cmp.Diff(
			first,
			got,
			cmp.AllowUnexported(Index{}, entry{}),
			cmpopts.IgnoreFields(Index{}, "policy"),
		); diff != "" {
			t.Fatalf("NewIndex over input order %v differs (-first +permuted):\n%s", perm, diff)
		}
	}
}

// permutations returns every ordering of the indexes 0..n-1.
func permutations(n int) [][]int {
	var out [][]int
	var build func(rest, chosen []int)
	build = func(rest, chosen []int) {
		if len(rest) == 0 {
			out = append(out, slices.Clone(chosen))
			return
		}
		for i, v := range rest {
			remaining := slices.Concat(rest[:i], rest[i+1:])
			build(remaining, slices.Concat(chosen, []int{v}))
		}
	}
	all := make([]int, n)
	for i := range all {
		all[i] = i
	}
	build(all, nil)
	return out
}

func TestIndexMetadataClosesWhenPolicySourceDrifts(t *testing.T) {
	t.Parallel()

	contractBytes, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read contract fixture: %v", err)
	}
	contractPath := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err = os.WriteFile(contractPath, contractBytes, 0o600); err != nil { // #nosec G703 -- path is a fixed basename under t.TempDir
		t.Fatalf("write contract: %v", err)
	}
	contract, err := schema.LoadFile(contractPath)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	policy := contract.ArtifactPolicy()
	idx := NewIndex([]Document{{
		RelPath:   "Concepts/A.md",
		Title:     "A",
		Status:    "seedling",
		PlainText: "needle",
	}}, policy)

	if err = os.WriteFile(contractPath, append(contractBytes, '\n'), 0o600); err != nil { // #nosec G703 -- path is a fixed basename under t.TempDir
		t.Fatalf("change contract: %v", err)
	}
	if policy.ValidateSource().Available() {
		t.Fatal("policy remained available after source drift")
	}
	if err = os.WriteFile(contractPath, contractBytes, 0o600); err != nil { // #nosec G703 -- path is a fixed basename under t.TempDir
		t.Fatalf("restore contract: %v", err)
	}

	if _, countErr := idx.CountByTypeStatus(); !errors.Is(countErr, ErrMetadataUnavailable) {
		t.Errorf("CountByTypeStatus() error = %v, want %v", countErr, ErrMetadataUnavailable)
	}
	query := Parse("needle")
	results, err := idx.Search(query)
	if err != nil {
		t.Fatalf("bare-text Search() error = %v", err)
	}
	if len(results) != 1 || results[0].RelPath != "Concepts/A.md" {
		t.Errorf("bare-text Search() results = %v, want readable corpus preserved", results)
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
			results, err := idx.Search(Parse(tt.query))
			if err != nil {
				t.Fatalf("Search(%q) error = %v", tt.query, err)
			}
			paths := make([]string, 0, len(results))
			for _, r := range results {
				paths = append(paths, r.RelPath)
			}
			if diff := cmp.Diff(tt.want, paths); diff != "" {
				t.Errorf("Search(%q) paths mismatch (-want +got):\n%s", tt.query, diff)
			}
		})
	}
}

// TestBoundedSearchKeepsTheOpeningStretch pins the two properties the bounded
// variant owes its callers: the total is the count the unbounded answer would
// have, and the kept results are exactly that answer's opening stretch — same
// hits, same order, same snippets. A cap that reordered or resampled would
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

	all, err := idx.Search(Parse("needle"))
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(all) != 10 || all[0].RelPath != "Notes/z-title.md" {
		t.Fatalf("unbounded answer = %d results opening with %q, want 10 opening with the title hit", len(all), all[0].RelPath)
	}

	bounded, total, err := idx.search(Parse("needle"), 4)
	if err != nil {
		t.Fatalf("search() error = %v", err)
	}
	if total != 10 {
		t.Errorf("total = %d, want 10", total)
	}
	if diff := cmp.Diff(all[:4], bounded); diff != "" {
		t.Errorf("bounded results are not the opening stretch (-unbounded[:4] +bounded):\n%s", diff)
	}

	if _, countOnly, countErr := idx.search(Parse("needle"), 0); countErr != nil || countOnly != 10 {
		t.Errorf("count-only search = (total %d, %v), want (10, nil)", countOnly, countErr)
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
}

// fileCorpusFixture is one vault of notes plus the files that sit beside them.
// The rel paths are chosen so a purely path-ordered index would interleave
// them — "Notes/…" sorts between "Concepts/…" and "Writing/…" — which is what
// makes the ordering assertions below able to fail.
func fileCorpusFixture(tb testing.TB) (notesOnly, widened *Index) {
	tb.Helper()
	notes := []Document{
		{RelPath: "Concepts/Focus.md", Title: "Focus", NoteType: "concept", Status: "evergreen", PlainText: "a note about the flamingo"},
		{RelPath: "Writing/Kafka.md", Title: "Kafka", NoteType: "lesson", Status: "draft", PlainText: "another flamingo mention"},
	}
	files := []Document{
		DocumentFromFile("Notes/todo.txt", []byte("renew the passport before flamingo season")),
		DocumentFromFile("Writing/article.html", []byte("<h1>flamingo</h1>")),
	}
	policy := validArtifactPolicy(tb)
	return NewIndex(notes, policy), NewIndex(append(append([]Document{}, notes...), files...), policy)
}

// TestSearchFindsAFileByItsText is the whole point of indexing files. A reader
// who opened todo.txt from the rail, read it, and then searched for a word in
// it used to get nothing back and no reason — the shape that makes someone
// conclude the file was deleted and type it again.
func TestSearchFindsAFileByItsText(t *testing.T) {
	t.Parallel()
	_, widened := fileCorpusFixture(t)

	results := searchResults(t, widened, Parse("passport"))
	want := []Result{{
		RelPath: "Notes/todo.txt",
		Title:   "todo.txt",
		Snippet: "renew the passport before flamingo season",
		File:    true,
	}}
	if diff := cmp.Diff(want, results); diff != "" {
		t.Errorf("Search(passport) mismatch (-want +got):\n%s", diff)
	}
}

// TestNoteResultsOpenTheWidenedResults is the shape lock. Files are appended to
// what a vault of notes alone answers, never interleaved with it: widening what
// can be found must not move what could already be found, or every reader's
// muscle memory for "it is the third one" breaks on a change that was supposed
// to add.
func TestNoteResultsOpenTheWidenedResults(t *testing.T) {
	t.Parallel()
	notesOnly, widened := fileCorpusFixture(t)

	for _, query := range []string{"flamingo", "a", "status:draft", "folder:Writing"} {
		t.Run(query, func(t *testing.T) {
			t.Parallel()
			before := searchResults(t, notesOnly, Parse(query))
			after := searchResults(t, widened, Parse(query))
			if len(after) < len(before) {
				t.Fatalf("Search(%q) returned %d results over the widened corpus, fewer than the %d notes alone answered",
					query, len(after), len(before))
			}
			if diff := cmp.Diff(before, after[:len(before)]); diff != "" {
				t.Errorf("Search(%q) moved a note result (-notes only +widened prefix):\n%s", query, diff)
			}
		})
	}
}

// TestFileEntriesAnswerNoMetadataProjection keeps every lifecycle tally the
// tally it was. A file carries no frontmatter, so it is not an instance of
// anything, and letting one into a status count or a "type:" filter would make
// Home's figures disagree with the contract that defines them.
func TestFileEntriesAnswerNoMetadataProjection(t *testing.T) {
	t.Parallel()
	notesOnly, widened := fileCorpusFixture(t)

	beforeTypes, err := notesOnly.CountByTypeStatus()
	if err != nil {
		t.Fatalf("notes-only CountByTypeStatus() error: %v", err)
	}
	afterTypes, err := widened.CountByTypeStatus()
	if err != nil {
		t.Fatalf("widened CountByTypeStatus() error: %v", err)
	}
	if diff := cmp.Diff(beforeTypes, afterTypes); diff != "" {
		t.Errorf("CountByTypeStatus changed when files joined the corpus (-notes only +widened):\n%s", diff)
	}

	for _, query := range []string{"type:lesson", "status:draft", "flamingo status:draft"} {
		if got := paths(searchResults(t, widened, Parse(query))); slices.Contains(got, "Notes/todo.txt") {
			t.Errorf("Search(%q) answered with a file, which declares no metadata: %v", query, got)
		}
	}
}

// TestFileEntriesRemainReachableByFolder is the other side of that exclusion. A
// folder is a fact about where a file sits, not a value it declared, so
// narrowing by one must keep reaching files — otherwise "exclude files from
// filters" would have been written one step too wide.
func TestFileEntriesRemainReachableByFolder(t *testing.T) {
	t.Parallel()
	_, widened := fileCorpusFixture(t)

	got := paths(searchResults(t, widened, Parse("folder:Notes")))
	if diff := cmp.Diff([]string{"Notes/todo.txt"}, got); diff != "" {
		t.Errorf("Search(folder:Notes) mismatch (-want +got):\n%s", diff)
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

	// The hit is also shown, and the marks on it are cut from the note's own
	// bytes: an offset located in the folded copy that is carried back wrong
	// paints the highlight over the neighbouring words.
	var wrapped, deep Result
	for _, r := range got {
		switch r.RelPath {
		case "wrapped.md":
			wrapped = r
		case "deep.md":
			deep = r
		}
	}
	// A window opened at the top of a long note would show the reader filler
	// and mark nothing, which reads as a hit on a note that does not contain
	// the phrase.
	if !strings.Contains(deep.Snippet, "semantic retrieval") {
		t.Errorf("snippet of the long note = %q, want the window around the phrase", deep.Snippet)
	}
	if wrapped.Snippet != "daily semantic retrieval log" {
		t.Errorf("snippet = %q, want the wrapped sentence on one line", wrapped.Snippet)
	}
	want := []pages.SnippetRun{
		{Text: "daily "},
		{Text: "semantic retrieval", Hit: true},
		{Text: " log"},
	}
	if diff := cmp.Diff(want, markHits(wrapped.Snippet, query.Tokens())); diff != "" {
		t.Errorf("markHits over the wrapped hit mismatch (-want +got):\n%s", diff)
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

// A topic can carry a space, and the way to name one is the same convention
// every search box uses: quote the value. Quoting had exempted the whole
// field from being read as a filter, so the idiom looked supported and
// quietly ran as body text over notes that never write their own topics down.
func TestQuotedFilterValueStillFilters(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
		{RelPath: "a.md", Title: "A", Topics: []string{"functional programming"}, PlainText: "one"},
		{RelPath: "b.md", Title: "B", Topics: []string{"functional"}, PlainText: "two"},
		{RelPath: "c.md", Title: "C", PlainText: "topic:functional programming written out in the body"},
	}, schema.ArtifactPolicy{})

	got := paths(searchResults(t, idx, Parse(`topic:"functional programming"`)))
	if diff := cmp.Diff([]string{"a.md"}, got); diff != "" {
		t.Errorf(`Search(topic:"functional programming") mismatch (-want +got):`+"\n%s", diff)
	}

	// The other direction stands unchanged: quote the whole field and it is
	// text, matched wherever those characters sit together.
	quoted := paths(searchResults(t, idx, Parse(`"topic:functional programming"`)))
	if diff := cmp.Diff([]string{"c.md"}, quoted); diff != "" {
		t.Errorf(`Search("topic:functional programming") mismatch (-want +got):`+"\n%s", diff)
	}
}
