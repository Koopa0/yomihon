package search

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestParse pins the parse rules documented on Parse, row by row, with
// hand-computed expected Tokens/Filters. NFD fixtures use explicit code
// points so the
// input's Unicode form is unambiguous: "が" is NFD が (か + combining
// dakuten), whose NFC form is "が" (が).
func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  Query
	}{
		{name: "1 empty", input: "", want: Query{}},
		{name: "2 whitespace only", input: "   \t ", want: Query{}},
		{name: "3 bare token", input: "kafka", want: Query{tokens: []string{"kafka"}, bareText: "kafka"}},
		{name: "4 fold uppercase", input: "Kafka", want: Query{tokens: []string{"kafka"}, bareText: "Kafka"}},
		{name: "5 two cjk tokens", input: "深度 工作", want: Query{tokens: []string{"深度", "工作"}, bareText: "深度 工作"}},
		{name: "6 nfd token folds to nfc", input: "が", want: Query{tokens: []string{"が"}, bareText: "が"}},
		{name: "7 type filter", input: "type:lesson", want: Query{filters: []Filter{{Key: "type", Value: "lesson"}}}},
		{
			name:  "8 token order preserved around filter",
			input: "深度 type:lesson 工作",
			want:  Query{tokens: []string{"深度", "工作"}, filters: []Filter{{Key: "type", Value: "lesson"}}, bareText: "深度 工作"},
		},
		{
			name:  "9 repeated topic key is AND",
			input: "topic:a topic:b",
			want:  Query{filters: []Filter{{Key: "topic", Value: "a"}, {Key: "topic", Value: "b"}}},
		},
		{
			name:  "10 repeated type key",
			input: "type:a type:b",
			want:  Query{filters: []Filter{{Key: "type", Value: "a"}, {Key: "type", Value: "b"}}},
		},
		{name: "11 folder drops trailing slash", input: "folder:Writing/", want: Query{filters: []Filter{{Key: "folder", Value: "Writing"}}}},
		{name: "12 slug splits on first colon", input: "slug:a:b", want: Query{filters: []Filter{{Key: "slug", Value: "a:b"}}}},
		{name: "13 unknown key is bare token", input: "foo:bar", want: Query{tokens: []string{"foo:bar"}, bareText: "foo:bar"}},
		{name: "14 classify before fold", input: "Type:lesson", want: Query{tokens: []string{"type:lesson"}, bareText: "Type:lesson"}},
		{name: "15 empty filter value", input: "status:", want: Query{filters: []Filter{{Key: "status", Value: ""}}}},
		{name: "16 literal percent", input: "%", want: Query{tokens: []string{"%"}, bareText: "%"}},
		{name: "17 percent inside token", input: "100%", want: Query{tokens: []string{"100%"}, bareText: "100%"}},
		{name: "18 domain nfc value", input: "domain:日本語", want: Query{filters: []Filter{{Key: "domain", Value: "日本語"}}}},
		{name: "19 slug value not case folded", input: "slug:ABC", want: Query{filters: []Filter{{Key: "slug", Value: "ABC"}}}},
		{name: "20 folder empty value", input: "folder:", want: Query{filters: []Filter{{Key: "folder", Value: ""}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Parse(tt.input)
			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(Query{})); diff != "" {
				t.Errorf("Parse(%q) mismatch (-want +got):\n%s", tt.input, diff)
			}
		})
	}
}

// Double-quoting a phrase is a near-universal search habit, and matching is
// already a contiguous substring test, so the quoted span becomes one bare
// token with the quotes stripped — adjacency comes for free. Before this, the
// quote characters rode along inside the tokens and matched almost nothing,
// and a phrase sitting verbatim in the vault came back as 找不到. The
// full-width 「」 and 『』 pairs of Chinese and Japanese prose quote the same
// way; a quote with no partner is dropped where it stands.
func TestParseQuotedPhrase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  Query
	}{
		{name: "quoted phrase is one token", input: `"semantic retrieval"`, want: Query{tokens: []string{"semantic retrieval"}, bareText: "semantic retrieval"}},
		{name: "quoted phrase still folds", input: `"Semantic Retrieval"`, want: Query{tokens: []string{"semantic retrieval"}, bareText: "Semantic Retrieval"}},
		{name: "corner brackets quote the same way", input: "「深度 工作」", want: Query{tokens: []string{"深度 工作"}, bareText: "深度 工作"}},
		{name: "white corner brackets too", input: "『深度 工作』", want: Query{tokens: []string{"深度 工作"}, bareText: "深度 工作"}},
		{name: "an unclosed quote is a character, not a swallowed rest-of-query", input: `"semantic retrieval`, want: Query{tokens: []string{`"semantic`, "retrieval"}, bareText: `"semantic retrieval`}},
		{name: "empty quotes ask nothing", input: `""`, want: Query{}},
		{name: "whitespace-only quotes ask nothing", input: `" "`, want: Query{}},
		{name: "a quote inside a word is part of the word", input: `don"t`, want: Query{tokens: []string{`don"t`}, bareText: `don"t`}},
		{name: "a closing bracket with no opener is text", input: "読本」", want: Query{tokens: []string{"読本」"}, bareText: "読本」"}},
		{name: "quoting keeps filter-shaped text literal", input: `"type:lesson"`, want: Query{tokens: []string{"type:lesson"}, bareText: "type:lesson"}},
		{
			// The shape a reader produces by pasting a line out of their own
			// note. Stripping the quotes spliced the span onto "cause=", and
			// the vault was searched for bytes it does not hold, so the answer
			// was that the reader's own sentence is not there.
			name:  "a quoted span pressed against a word stays literal",
			input: `cause="lease epoch mismatch"`,
			want:  Query{tokens: []string{`cause="lease`, "epoch", `mismatch"`}, bareText: `cause="lease epoch mismatch"`},
		},
		{
			// The same shape in the prose this vault is mostly written in,
			// where corner brackets sit flush against the words they quote and
			// there is no whitespace anywhere near them.
			name:  "corner brackets inside chinese prose stay literal",
			input: "他說「不得使用」的時候",
			want:  Query{tokens: []string{"他說「不得使用」的時候"}, bareText: "他說「不得使用」的時候"},
		},
		{
			// The opener stands at a field boundary here, so only the closing
			// side can decide: a bracket with a word pressed against it is
			// closing nothing, it is punctuation inside somebody's sentence.
			// Reading it as a closer spliced the tail on and searched for
			// 深度 工作的時候, which the note does not contain.
			name:  "a closing bracket with a word against it closes nothing",
			input: "「深度 工作」的時候",
			want:  Query{tokens: []string{"「深度", "工作」的時候"}, bareText: "「深度 工作」的時候"},
		},
		{
			name:  "a filter beside a phrase stays a filter",
			input: `type:lesson "semantic retrieval"`,
			want:  Query{tokens: []string{"semantic retrieval"}, filters: []Filter{{Key: "type", Value: "lesson"}}, bareText: "semantic retrieval"},
		},
		{name: "quotes pressed against words group nothing and splice nothing", input: `sem"antic ret"rieval`, want: Query{tokens: []string{`sem"antic`, `ret"rieval`}, bareText: `sem"antic ret"rieval`}},
		{
			name:  "quoting a value keeps the filter and carries the space",
			input: `topic:"functional programming"`,
			want:  Query{filters: []Filter{{Key: "topic", Value: "functional programming"}}},
		},
		{
			name:  "corner brackets quote a value the same way",
			input: "topic:「深度 工作」",
			want:  Query{filters: []Filter{{Key: "topic", Value: "深度 工作"}}},
		},
		{
			name:  "a quoted single-word value is still a filter",
			input: `type:"lesson"`,
			want:  Query{filters: []Filter{{Key: "type", Value: "lesson"}}},
		},
		{
			name:  "a quoted folder value drops its trailing slash",
			input: `folder:"My Notes/"`,
			want:  Query{filters: []Filter{{Key: "folder", Value: "My Notes"}}},
		},
		{
			name:  "quoting the key makes the whole field text",
			input: `"topic:functional programming"`,
			want:  Query{tokens: []string{"topic:functional programming"}, bareText: "topic:functional programming"},
		},
		{
			name:  "a quoted value beside a phrase leaves the phrase a token",
			input: `topic:"functional programming" "semantic retrieval"`,
			want: Query{
				tokens:   []string{"semantic retrieval"},
				filters:  []Filter{{Key: "topic", Value: "functional programming"}},
				bareText: "semantic retrieval",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Parse(tt.input)
			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(Query{})); diff != "" {
				t.Errorf("Parse(%q) mismatch (-want +got):\n%s", tt.input, diff)
			}
		})
	}
}

func TestParsePreservesBareTextForSemanticProjection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "pure filter", input: "type:lesson folder:Writing", want: ""},
		{name: "case", input: "Kafka", want: "Kafka"},
		{name: "mixed filters", input: "深度 type:lesson 工作 folder:Writing", want: "深度 工作"},
		{name: "unknown key remains text", input: "foo:Bar", want: "foo:Bar"},
		{name: "uppercase filter name remains text", input: "Type:lesson", want: "Type:lesson"},
		{name: "whitespace becomes ascii space", input: "深度\t  工作", want: "深度 工作"},
		{name: "unicode form is preserved", input: "が", want: "が"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Parse(tt.input).BareText(); got != tt.want {
				t.Errorf("Parse(%q).BareText() = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestParseFilterValueNFD proves a filter value is NFC-normalized (but not
// case-folded) using a genuine NFD form
// (row 18's 日本語 has no canonical decomposition, so it cannot). "domain:" +
// NFD が ("が") must store the NFC form "が", un-case-folded.
func TestParseFilterValueNFD(t *testing.T) {
	t.Parallel()

	got := Parse("domain:が")
	want := Query{filters: []Filter{{Key: "domain", Value: "が"}}}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(Query{})); diff != "" {
		t.Errorf("Parse NFD filter value mismatch (-want +got):\n%s", diff)
	}
}

func TestFilterCapabilityClassification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		key        string
		wantKind   filterKind
		wantFilter bool
	}{
		{name: "type metadata", key: "type", wantKind: filterMetadata, wantFilter: true},
		{name: "status metadata", key: "status", wantKind: filterMetadata, wantFilter: true},
		{name: "domain metadata", key: "domain", wantKind: filterMetadata, wantFilter: true},
		{name: "topic metadata", key: "topic", wantKind: filterMetadata, wantFilter: true},
		{name: "slug metadata", key: "slug", wantKind: filterMetadata, wantFilter: true},
		{name: "folder path", key: "folder", wantKind: filterPath, wantFilter: true},
		{name: "bare token", key: "unknown", wantKind: filterUnknown, wantFilter: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotKind, gotFilter := classifyFilterKey(tt.key)
			if gotKind != tt.wantKind || gotFilter != tt.wantFilter {
				t.Errorf("classifyFilterKey(%q) = (%v, %v), want (%v, %v)", tt.key, gotKind, gotFilter, tt.wantKind, tt.wantFilter)
			}
		})
	}
}

func TestQueryOwnsParsedStateAndCapability(t *testing.T) {
	t.Parallel()

	query := Parse("Kanji type:lesson folder:Writing")
	tokens := query.Tokens()
	filters := query.Filters()
	if query.BareText() != "Kanji" || !query.RequiresMetadata() {
		t.Fatalf("parsed query projection = (%q, %v), want (Kanji, true)", query.BareText(), query.RequiresMetadata())
	}
	tokens[0] = "changed"
	filters[0].Key = "folder"
	if got := query.Tokens(); len(got) != 1 || got[0] != "kanji" {
		t.Fatalf("mutating Tokens result changed query: %v", got)
	}
	if got := query.Filters(); len(got) != 2 || got[0].Key != "type" {
		t.Fatalf("mutating Filters result changed query: %v", got)
	}
	if Parse("folder:Writing").RequiresMetadata() {
		t.Fatal("folder-only query unexpectedly requires metadata")
	}
}
