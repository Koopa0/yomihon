package lexical

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// facetFixture is an index wide enough that a division has something to
// divide: several statuses, several types, several domains, notes that declare
// none of them, a vault file that can declare nothing, a value whose spelling
// differs only in case from another's, a value carrying a space, and a note
// the contract places outside instance metadata while it still declares all
// three fields. The plain text is shared so a bare token reaches every note and a division can be
// taken over the whole of it.
func facetFixture(tb testing.TB) *Index {
	tb.Helper()
	docs := []Document{
		{RelPath: "Writing/Kafka.md", Title: "Kafka", NoteType: "lesson", Domain: "golang", Status: "draft", PlainText: "needle in a log"},
		{RelPath: "Writing/Streams.md", Title: "Streams", NoteType: "lesson", Domain: "golang", Status: "ready", PlainText: "needle in a stream"},
		{RelPath: "Concepts/Focus.md", Title: "Focus", NoteType: "concept", Domain: "meta", Status: "draft", PlainText: "needle and focus"},
		{RelPath: "Concepts/Depth.md", Title: "Depth", NoteType: "concept", Domain: "meta", Status: "Draft", PlainText: "needle and depth"},
		{RelPath: "Notes/Loose.md", Title: "Loose", PlainText: "needle with nothing declared"},
		{RelPath: "Notes/Spaced.md", Title: "Spaced", NoteType: "note", Domain: "field notes", Status: "archived", PlainText: "needle in the field"},
		{RelPath: "Notes/Odd.md", Title: "Odd", NoteType: "note", Domain: `say "what"`, Status: "archived", PlainText: "needle said what"},
		{RelPath: "Assets/plain.txt", Title: "plain.txt", File: true, PlainText: "needle in a file"},
		// A template: the contract places System/templates outside instance
		// metadata, so its frontmatter is not a note's. It carries values of
		// its own precisely so that tallying it would be visible — a narrowed
		// query leaves it out, so a division that counted it would print a
		// number the page it leads to contradicts.
		{RelPath: "System/templates/Card.md", Title: "Card", NoteType: "template", Domain: "scaffolding", Status: "blank", PlainText: "needle in a template"},
	}
	return NewIndex(docs, validArtifactPolicy(tb))
}

// facetQueries are the shapes a division has to be right for: a bare token, a
// phrase, a query already carrying one of the constraints being offered, a
// query carrying a constraint on another key, a pure-filter query, and a query
// whose spelling of a value is not the one the notes used.
var facetQueries = []string{
	"needle",
	`"in a"`,
	"needle status:draft",
	"needle type:concept",
	"type:lesson",
	"needle status:DRAFT",
	"needle domain:golang status:draft",
}

// TestEveryFacetCountIsItsNarrowedQuerysOwnAnswer is the lock the whole column
// rests on. A count beside a value is a promise about the page the value leads
// to, so it is checked by going there: the row's own narrowed query is written
// by the same grammar the page writes it with, parsed back, and run, and the
// number of results it finds has to be the number the row printed.
//
// Nothing here trusts the tally. A count read back out of the counter would
// agree with itself whatever it counted — including over the bounded stretch
// of results rather than over every hit, which is the way this is most likely
// to go wrong and the way a reader would never see.
func TestEveryFacetCountIsItsNarrowedQuerysOwnAnswer(t *testing.T) {
	t.Parallel()
	idx := facetFixture(t)

	checked := 0
	for _, query := range facetQueries {
		t.Run(query, func(t *testing.T) {
			t.Parallel()
			answer, err := idx.Search(Parse(query), -1)
			if err != nil {
				t.Fatalf("Search(%q) error = %v", query, err)
			}
			for _, division := range answer.Facets {
				for _, value := range division.Values {
					narrowed, ok := WithFilter(query, Filter{Key: division.Key, Value: value.Value})
					if !ok {
						// A value this grammar cannot spell is offered without a
						// link, and its count is then a fact about the answer
						// rather than a promise about another page.
						continue
					}
					narrowedAnswer, err := idx.Search(Parse(narrowed), 0)
					if err != nil {
						t.Fatalf("Search(%q) error = %v", narrowed, err)
					}
					if narrowedAnswer.Total != value.Count {
						t.Errorf("%s:%s said %d, but %q finds %d",
							division.Key, value.Value, value.Count, narrowed, narrowedAnswer.Total)
					}
					checked++
				}
			}
		})
	}
	t.Cleanup(func() {
		// Subtests run in parallel, so this reads after they finish. Without it
		// a division that stopped being produced at all would leave every loop
		// above with nothing to iterate and this test would pass having
		// compared nothing.
		if checked < 20 {
			t.Errorf("only %d facet counts were put to their own query, so this check holds almost nothing", checked)
		}
	})
}

// TestAFacetDivisionAccountsForEveryHit pins the other half: a count that is
// right for every value it lists still lies if it leaves values out. Every hit
// is either at one of the division's values or in its unstated cell, so the two
// have to add up to the answer's own total — and a hit that silently fell out
// of the tally shows up here as arithmetic rather than as a missing row nobody
// would notice.
func TestAFacetDivisionAccountsForEveryHit(t *testing.T) {
	t.Parallel()
	idx := facetFixture(t)

	for _, query := range facetQueries {
		t.Run(query, func(t *testing.T) {
			t.Parallel()
			answer, err := idx.Search(Parse(query), -1)
			if err != nil {
				t.Fatalf("Search(%q) error = %v", query, err)
			}
			if len(answer.Facets) != len(facetKeys) {
				t.Fatalf("Search(%q) divided along %d keys, want %d", query, len(answer.Facets), len(facetKeys))
			}
			for _, division := range answer.Facets {
				sum := division.Unstated
				for _, value := range division.Values {
					sum += value.Count
				}
				if sum != answer.Total {
					t.Errorf("%s divides %d hits into %d, so some hit is at no value and in no cell",
						division.Key, answer.Total, sum)
				}
			}
		})
	}
}

// TestAFacetCountsEveryHitAndNotTheBuiltStretch is the bounded-search case
// stated on its own, because it is the one the whole design turns on: results
// are materialized up to a limit and the tally is not. With more matches than
// the limit, a division taken over the built results would come out at exactly
// the limit and look entirely reasonable.
func TestAFacetCountsEveryHitAndNotTheBuiltStretch(t *testing.T) {
	t.Parallel()

	const notes = 40
	const limit = 5
	docs := make([]Document, 0, notes)
	for i := range notes {
		docs = append(docs, Document{
			RelPath:   "Notes/n" + strconv.Itoa(i) + ".md",
			Title:     "n" + strconv.Itoa(i),
			NoteType:  "note",
			Status:    "draft",
			PlainText: "needle",
		})
	}
	idx := NewIndex(docs, validArtifactPolicy(t))

	answer, err := idx.Search(Parse("needle"), limit)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if len(answer.Results) != limit || answer.Total != notes {
		t.Fatalf("Search built %d of %d results, want %d of %d", len(answer.Results), answer.Total, limit, notes)
	}
	draft := facetValueCount(t, answer, StatusFilterKey, "draft")
	if draft != notes {
		t.Errorf("status:draft counted %d, want %d — the tally is reading the built stretch, not the answer", draft, notes)
	}
}

// TestAnUnstatedCellIsNotAValue pins that the hits declaring nothing are kept
// off the values. A note with no domain is not a note in a domain called "",
// and a division that listed it among the values would offer a link to a query
// that cannot be written and would answer nothing.
func TestAnUnstatedCellIsNotAValue(t *testing.T) {
	t.Parallel()
	idx := facetFixture(t)

	answer, err := idx.Search(Parse("needle"), -1)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	for _, division := range answer.Facets {
		if division.Unstated == 0 {
			t.Errorf("%s has no unstated cell, but the fixture holds a note declaring nothing and a file that can declare nothing", division.Key)
		}
		for _, value := range division.Values {
			if value.Value == "" {
				t.Errorf("%s lists an empty value among its values", division.Key)
			}
		}
	}
}

// TestTwoSpellingsOfOneValueAreOneRow pins that values are gathered the way
// matching gathers them. "Draft" and "draft" are one constraint to the index,
// so two rows of one would each print a count the shared narrowed query
// contradicts — which is exactly what the count check above would catch, and
// this says why.
func TestTwoSpellingsOfOneValueAreOneRow(t *testing.T) {
	t.Parallel()
	idx := facetFixture(t)

	answer, err := idx.Search(Parse("needle"), -1)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	var spellings []string
	for _, division := range answer.Facets {
		if division.Key != StatusFilterKey {
			continue
		}
		for _, value := range division.Values {
			spellings = append(spellings, value.Value)
		}
	}
	folded := make(map[string]int)
	for _, spelling := range spellings {
		folded[fold(spelling)]++
	}
	for value, rows := range folded {
		if rows > 1 {
			t.Errorf("%q occupies %d rows, but the index reads them as one constraint", value, rows)
		}
	}
	if folded["draft"] != 1 {
		t.Fatalf("status rows = %v, want one row gathering Draft and draft", spellings)
	}
}

// TestFacetValuesAreOrderedByWeightThenByName pins the order, since it is what
// the recorded pages are recorded with and a map's own order is no order at
// all.
func TestFacetValuesAreOrderedByWeightThenByName(t *testing.T) {
	t.Parallel()
	idx := facetFixture(t)

	answer, err := idx.Search(Parse("needle"), -1)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	for _, division := range answer.Facets {
		for i := 1; i < len(division.Values); i++ {
			previous, current := division.Values[i-1], division.Values[i]
			if previous.Count < current.Count {
				t.Errorf("%s puts %d before %d", division.Key, previous.Count, current.Count)
			}
			if previous.Count == current.Count && fold(previous.Value) > fold(current.Value) {
				t.Errorf("%s puts %q before %q at the same count", division.Key, previous.Value, current.Value)
			}
		}
	}
}

// TestADivisionIsOfferedOnlyWhereItCouldBeHonoured pins the two states where
// no narrowing can be offered at all: a query nothing answers has nothing to
// divide, and a folder whose artifact policy leaves metadata unavailable would
// refuse every narrowed query it invited.
func TestADivisionIsOfferedOnlyWhereItCouldBeHonoured(t *testing.T) {
	t.Parallel()

	idx := facetFixture(t)
	for _, query := range []string{"", "   ", "nothingmatchesthis"} {
		answer, err := idx.Search(Parse(query), -1)
		if err != nil {
			t.Fatalf("Search(%q) error = %v", query, err)
		}
		if len(answer.Facets) != 0 {
			t.Errorf("Search(%q) offered %d divisions over %d hits", query, len(answer.Facets), answer.Total)
		}
	}

	closed := idx.WithArtifactPolicy(undeclaredArtifactPolicy(t))
	answer, err := closed.Search(Parse("needle"), -1)
	if err != nil {
		t.Fatalf("Search on an unavailable policy error = %v", err)
	}
	if answer.Total == 0 {
		t.Fatal("the plain text query found nothing, so this proves nothing about the divisions")
	}
	if len(answer.Facets) != 0 {
		t.Errorf("a folder whose metadata is unavailable offered %d divisions, each leading to a refusal", len(answer.Facets))
	}
}

// TestACarrierTravelsWithTheStatusItHolds pins that a value carries the note
// types holding it. A status is declared per type, so a surface ruling on a
// value needs them, and a value that arrived without them would be ruled on as
// though every type declared it.
func TestACarrierTravelsWithTheStatusItHolds(t *testing.T) {
	t.Parallel()
	idx := facetFixture(t)

	answer, err := idx.Search(Parse("needle"), -1)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	got := map[string][]string{}
	for _, division := range answer.Facets {
		if division.Key != StatusFilterKey {
			continue
		}
		for _, value := range division.Values {
			got[fold(value.Value)] = value.Carriers
		}
	}
	want := map[string][]string{
		"draft":    {"concept", "lesson"},
		"ready":    {"lesson"},
		"archived": {"note"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("status carriers mismatch (-want +got):\n%s", diff)
	}
}

// TestEveryDividedKeyIsAKeyTheGrammarAccepts holds the divisions inside the
// grammar. A key offered as a division that the parser does not read as a
// filter would send the reader to a search that quietly ran their constraint as
// a word.
func TestEveryDividedKeyIsAKeyTheGrammarAccepts(t *testing.T) {
	t.Parallel()

	if len(facetKeys) == 0 {
		t.Fatal("nothing is divided along, so this check holds nothing")
	}
	for _, key := range facetKeys {
		kind, ok := classifyFilterKey(key)
		if !ok {
			t.Errorf("%q is offered as a division and is not a filter this grammar accepts", key)
			continue
		}
		if kind != filterMetadata {
			t.Errorf("%q is offered as a division and asks no metadata of an entry", key)
		}
	}
	if slices.Contains(facetKeys, "folder") || slices.Contains(facetKeys, "topic") {
		t.Error("an open set is offered as a division; a column of two hundred values narrows nothing")
	}
}

// TestEveryDividedKeyReadsItsOwnField pins the switch that reads a value off an
// entry against the list of keys divided along. A key added to that list with
// no arm to read it would tally every hit as stating nothing — a column of one
// empty cell, with no error anywhere.
func TestEveryDividedKeyReadsItsOwnField(t *testing.T) {
	t.Parallel()

	// Each field is marked with its own key, so a switch arm wired to the
	// wrong field names the field it actually read rather than merely failing.
	marked := &entry{
		NoteType: "type as written", NoteTypeFold: "type",
		Status: "status as written", StatusFold: "status",
		Domain: "domain as written", DomainFold: "domain",
	}
	for _, key := range facetKeys {
		written, folded := marked.facetValue(key)
		if folded != key {
			t.Errorf("facetValue(%q) read %q, so that key reads some other field or none", key, folded)
		}
		if written != key+" as written" {
			t.Errorf("facetValue(%q) wrote %q, want the field's own spelling", key, written)
		}
	}
	if _, folded := marked.facetValue("folder"); folded != "" {
		t.Errorf("facetValue(folder) read %q, but no division is taken along it", folded)
	}
}

// TestNarrowingAQueryLeavesEveryOtherByteAlone pins what a reader is owed when
// a row rewrites the words they typed: their spacing, their quoting and their
// capitalisation, everywhere the row is not about. The rebuild is by offset
// into the query rather than by taking it apart and writing it out again, and
// this is what says so.
func TestNarrowingAQueryLeavesEveryOtherByteAlone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		raw    string
		filter Filter
		add    string
		drop   string
	}{
		{"an empty query becomes the constraint", "", Filter{StatusFilterKey, "draft"}, "status:draft", ""},
		{"a plain term keeps its single space", "kafka", Filter{StatusFilterKey, "draft"}, "kafka status:draft", "kafka"},
		{"runs of spaces the reader typed survive", "kafka    streams", Filter{"type", "lesson"}, "kafka    streams type:lesson", "kafka    streams"},
		{"a quoted phrase keeps its quotes", `"owns jobs" kafka`, Filter{"domain", "golang"}, `"owns jobs" kafka domain:golang`, `"owns jobs" kafka`},
		{"a trailing space is not doubled", "kafka ", Filter{"type", "lesson"}, "kafka type:lesson", "kafka "},
		{"a value holding a space is quoted", "kafka", Filter{"domain", "field notes"}, `kafka domain:"field notes"`, "kafka"},
		{"removal takes the space in front of it", "a status:draft b", Filter{StatusFilterKey, "draft"}, "", "a b"},
		{"removal at the head takes the space behind it", "status:draft b", Filter{StatusFilterKey, "draft"}, "", "b"},
		{"removal is by the value matching does, not by spelling", "a status:DRAFT b", Filter{StatusFilterKey, "draft"}, "", "a b"},
		{"a constraint written twice goes entirely", "status:draft x status:Draft", Filter{StatusFilterKey, "draft"}, "", "x"},
		{"another key is left standing", "type:lesson status:draft", Filter{StatusFilterKey, "draft"}, "", "type:lesson"},
		{"a query not carrying it is unchanged", "kafka type:lesson", Filter{StatusFilterKey, "draft"}, "kafka type:lesson status:draft", "kafka type:lesson"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.add != "" {
				got, ok := WithFilter(tt.raw, tt.filter)
				if !ok || got != tt.add {
					t.Errorf("WithFilter(%q) = (%q, %v), want (%q, true)", tt.raw, got, ok, tt.add)
				}
			}
			if got := WithoutFilter(tt.raw, tt.filter); got != tt.drop {
				t.Errorf("WithoutFilter(%q) = %q, want %q", tt.raw, got, tt.drop)
			}
		})
	}
}

// TestAConstraintTheGrammarCannotSpellIsNotOffered pins the refusal, and with
// it where the line actually falls. Quoting a value is not a guess about which
// characters are safe: the candidate is read back by the parser, so a value
// holding quotes of its own is offered whenever the grammar still recovers it
// — a group closes at the last quote standing at a field boundary, which is
// why a value ending in one survives being wrapped in another pair. What is
// refused is a value that comes back as something else, and the refusal is
// reached by reading it back rather than by a rule written here that would
// have to be kept in step with the parser by hand.
func TestAConstraintTheGrammarCannotSpellIsNotOffered(t *testing.T) {
	t.Parallel()

	offered, ok := WithFilter("kafka", Filter{Key: "domain", Value: `say "what"`})
	if !ok {
		t.Errorf("a value whose quotes the grammar recovers was refused, and its row loses its link for nothing")
	} else if got := Parse(offered).Filters(); len(got) != 1 || got[0].Value != fold(`say "what"`) {
		t.Errorf("Parse(%q) filters = %v, want the value it was written from", offered, got)
	}

	// A value whose quote closes a group early comes back as a shorter
	// constraint and a stray word, which is a different search.
	if got, offered := WithFilter("kafka", Filter{Key: "domain", Value: `a" b`}); offered {
		t.Errorf("WithFilter(domain:%q) offered %q, which reads back as another query", `a" b`, got)
	}

	// The refusal has to be about reading back rather than about quotes in
	// general, or every value would be refused and the column would go linkless
	// without anything saying so.
	if _, offered := WithFilter("kafka", Filter{Key: "domain", Value: "field notes"}); !offered {
		t.Error("a value holding a space was refused; quoting is exactly what makes it writable")
	}
}

// facetValueCount reads one value's count out of an answer, failing when the
// value is not there at all — a zero returned for an absent row would let a
// caller's assertion pass for the wrong reason.
func facetValueCount(tb testing.TB, answer Answer, key, value string) int {
	tb.Helper()
	for _, division := range answer.Facets {
		if division.Key != key {
			continue
		}
		for _, candidate := range division.Values {
			if fold(candidate.Value) == fold(value) {
				return candidate.Count
			}
		}
	}
	tb.Fatalf("%s carries no value %q; the answer divided along %s", key, value, facetSummary(answer))
	return 0
}

func facetSummary(answer Answer) string {
	parts := make([]string, 0, len(answer.Facets))
	for _, division := range answer.Facets {
		values := make([]string, 0, len(division.Values))
		for _, value := range division.Values {
			values = append(values, fmt.Sprintf("%s=%d", value.Value, value.Count))
		}
		parts = append(parts, division.Key+"["+strings.Join(values, " ")+"]")
	}
	return strings.Join(parts, " ")
}
