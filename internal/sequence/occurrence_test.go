package sequence

import "testing"

// TestTargetSpanIsTheWrittenLink holds the identity an accepted row hands a
// consumer. A row is not its target: it can carry an embed, an anchor or a
// mention beside the lesson it names, and a consumer that reads the same file
// has to be able to say which of those the course listed. The row's own span
// cannot answer that, so a row that carries more than its lesson has two
// different spans, and they have to stay different.
func TestTargetSpanIsTheWrittenLink(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		// want is the source text the accepted row's target span covers.
		want string
	}{
		{
			name: "a bare row",
			body: "## Main {sequence=primary}\n\n- [[Lesson]]\n",
			want: "[[Lesson]]",
		},
		{
			name: "an embed after the lesson link",
			body: "## Main {sequence=primary}\n\n- [[Lesson]] ![[Retired]]\n",
			want: "[[Lesson]]",
		},
		{
			name: "a display alias",
			body: "## Main {sequence=primary}\n\n- [[Lesson|第一課]]\n",
			want: "[[Lesson|第一課]]",
		},
		{
			name: "emphasis wrapping the link",
			body: "## Main {sequence=primary}\n\n- **[[Lesson]]** 補充\n",
			want: "[[Lesson]]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := Parse(tt.body, 1)
			entry := onlyEntry(t, doc)
			if !entry.Accepted() {
				t.Fatalf("row state = %s, want accepted", entry.State)
			}
			if entry.TargetSpan.Zero() {
				t.Fatal("an accepted row carries no target span, so a consumer has nothing to join on")
			}
			if got := tt.body[entry.TargetSpan.Start:entry.TargetSpan.Stop]; got != tt.want {
				t.Errorf("target span covers %q, want %q", got, tt.want)
			}
		})
	}
}

// TestTargetSpanSeparatesTwoWritingsOfOneName holds that a name written on two
// rows is two occurrences. Joining on the name would let either row answer for
// the other; these spans are what keeps them apart.
func TestTargetSpanSeparatesTwoWritingsOfOneName(t *testing.T) {
	t.Parallel()
	doc := Parse("## Main {sequence=primary}\n\n- [[Lesson]]\n- [[Lesson]]\n", 1)
	entries := doc.Groups[0].Entries()
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	if entries[0].Target != entries[1].Target {
		t.Fatalf("the two rows name different notes (%q, %q), so this holds nothing",
			entries[0].Target, entries[1].Target)
	}
	if entries[0].TargetSpan == entries[1].TargetSpan {
		t.Errorf("both rows report the same target span %v, so one can answer for the other",
			entries[0].TargetSpan)
	}
}

// TestTargetSpanIsSetExactlyWhenTargetIs holds the invariant a consumer relies
// on: a row that resolves to a note says where that note is written, and a row
// that resolves to none claims no bytes. A refused row keeps both, because it
// is still a row — what it must not do is look like a lesson.
func TestTargetSpanIsSetExactlyWhenTargetIs(t *testing.T) {
	t.Parallel()
	bodies := []string{
		"## Main {sequence=primary}\n\n- [[Lesson]]\n",
		"## Main {sequence=primary}\n\n- [[One]] 與 [[Two]]\n",
		"## Main {sequence=primary}\n\n- 補充 [[Lesson]]\n",
		"## Main {sequence=primary}\n\n- ![[Retired]] [[Lesson]]\n",
		"## Main {sequence=none}\n\n- [[Lesson]]\n",
	}
	for _, body := range bodies {
		doc := Parse(body, 1)
		for _, g := range doc.Groups {
			for _, e := range g.Entries() {
				if (e.Target == "") != e.TargetSpan.Zero() {
					t.Errorf("target %q and span %v disagree about whether this row resolves to a note, in %q",
						e.Target, e.TargetSpan, body)
				}
				if e.TargetSpan.Zero() {
					continue
				}
				if got := body[e.TargetSpan.Start:e.TargetSpan.Stop]; got[:2] != "[[" {
					t.Errorf("target span covers %q, which is not a written link, in %q", got, body)
				}
			}
		}
	}
}

// onlyEntry is the single row a one-row fixture declares.
func onlyEntry(t *testing.T, doc Document) *Candidate {
	t.Helper()
	var found []*Candidate
	for _, g := range doc.Groups {
		found = append(found, g.Entries()...)
	}
	if len(found) != 1 {
		t.Fatalf("entries = %d, want 1", len(found))
	}
	return found[0]
}
