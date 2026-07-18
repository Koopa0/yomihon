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
