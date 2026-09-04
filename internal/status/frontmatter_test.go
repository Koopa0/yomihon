package status

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/vault"
)

func TestRewriteStatusLinePreservesFrontmatterDialect(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "CRLF",
			in:   "---\r\ntitle: Note\r\nstatus: draft\r\n---\r\nbody\r\n",
			want: "---\r\ntitle: Note\r\nstatus: ready\r\n---\r\nbody\r\n",
		},
		{
			name: "YAML document closer",
			in:   "---\ntitle: Note\nstatus: draft\n...\nbody\n",
			want: "---\ntitle: Note\nstatus: ready\n...\nbody\n",
		},
		{
			name: "closer at EOF",
			in:   "---\ntitle: Note\nstatus: draft\n---",
			want: "---\ntitle: Note\nstatus: ready\n---",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := rewriteStatusLine([]byte(tt.in), "ready")
			if err != nil {
				t.Fatalf("rewriteStatusLine() = %v", err)
			}
			if diff := cmp.Diff(tt.want, string(got)); diff != "" {
				t.Errorf("rewrite mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestRewriteStatusLineKeepsEverythingOnTheLineButTheValue holds what the
// surgical write means on the one line it is allowed to touch. A reason
// written beside the value is the author's words; replacing the whole line
// deleted them, and the deletion was invisible because the note still read
// back as the target status.
func TestRewriteStatusLineKeepsEverythingOnTheLineButTheValue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "a trailing comment survives the flip",
			in:   "---\nstatus: draft # 等原始資料\n---\nbody\n",
			want: "---\nstatus: ready # 等原始資料\n---\nbody\n",
		},
		{
			name: "double quoting survives the flip",
			in:   "---\nstatus: \"draft\"\n---\nbody\n",
			want: "---\nstatus: \"ready\"\n---\nbody\n",
		},
		{
			name: "single quoting survives the flip",
			in:   "---\nstatus: 'draft'\n---\nbody\n",
			want: "---\nstatus: 'ready'\n---\nbody\n",
		},
		{
			name: "trailing spaces survive the flip",
			in:   "---\nstatus: draft   \n---\nbody\n",
			want: "---\nstatus: ready   \n---\nbody\n",
		},
		{
			name: "a comment survives a crlf line",
			in:   "---\r\nstatus: draft # why\r\n---\r\nbody\r\n",
			want: "---\r\nstatus: ready # why\r\n---\r\nbody\r\n",
		},
		{
			// The comment already ends the value, so a break further along the
			// line falls outside it and both readings still agree where it
			// stops. The guard against these characters is that narrow.
			name: "a line break inside a trailing comment does not widen the value",
			in:   "---\nstatus: draft # note\u2028tags: x\n---\nbody\n",
			want: "---\nstatus: ready # note\u2028tags: x\n---\nbody\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := rewriteStatusLine([]byte(tt.in), "ready")
			if err != nil {
				t.Fatalf("rewriteStatusLine() = %v", err)
			}
			if diff := cmp.Diff(tt.want, string(got)); diff != "" {
				t.Errorf("rewrite mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestRewriteStatusLineRefusesAShapeItCannotPreserve holds the other side of
// the same rule. Where the value is not a plain or simply quoted scalar, no
// replacement can leave the rest of the line meaning what it meant, so the
// write refuses and the note keeps its bytes. Replacing the whole line would
// still read back as the target status, which is why these shapes were being
// rewritten into something their author never wrote.
//
// The last shapes here are the same disagreement in the reverse direction. The
// YAML reader ends an unquoted value at U+0085, U+2028 and U+2029 as readily as
// at a newline, and this scan ends it only at a newline, so such a value reaches
// past what the reader calls the value and into what the reader calls the next
// line — a key, or a comment line of the author's own. Replacing that run
// deletes it, and the note still reads back as the target status, so nothing
// downstream notices. Quoted values are refused too, but on other grounds; the
// test below this one holds those.
func TestRewriteStatusLineRefusesAShapeItCannotPreserve(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
	}{
		{name: "a sequence is not a status value", in: "---\nstatus: [draft]\n---\nbody\n"},
		{name: "a mapping is not a status value", in: "---\nstatus: {a: b}\n---\nbody\n"},
		{name: "an anchor would be severed", in: "---\nstatus: &s draft\n---\nbody\n"},
		{name: "an alias names a value elsewhere", in: "---\nstatus: *s\n---\nbody\n"},
		{name: "a tag changes how the value is read", in: "---\nstatus: !!str draft\n---\nbody\n"},
		{name: "a folded scalar continues past the line", in: "---\nstatus: >\n  draft\n---\nbody\n"},
		{name: "a literal scalar continues past the line", in: "---\nstatus: |\n  draft\n---\nbody\n"},
		{name: "no value at all", in: "---\nstatus:\n---\nbody\n"},
		{name: "no space after the colon is not a mapping", in: "---\nstatus:draft\n---\nbody\n"},
		{name: "an unterminated quote", in: "---\nstatus: \"draft\n---\nbody\n"},
		{name: "U+0085 ends the line and the next key would go with it", in: "---\nstatus: draft\u0085tags:\n---\nbody\n"},
		{name: "U+2028 ends the line and the next key would go with it", in: "---\nstatus: draft\u2028tags:\n---\nbody\n"},
		{name: "U+2029 ends the line and the next key would go with it", in: "---\nstatus: draft\u2029tags:\n---\nbody\n"},
		{name: "a break before a comment line would delete the author's words", in: "---\nstatus: draft\u2028# the reason I set it\n---\nbody\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := rewriteStatusLine([]byte(tt.in), "ready")
			if err == nil {
				t.Fatalf("rewriteStatusLine() rewrote a shape it cannot preserve:\n%s", got)
			}
		})
	}
}

// TestRewriteStatusLineRefusesAnInvisibleBreakInsideQuotes holds a refusal that
// is deliberately wider than the disagreement above. Inside quotes the reader
// does not end the line: it reads one key whose value is the whole quoted run,
// which is exactly the run a replacement would cover, so replacing it would in
// fact be correct. It is refused anyway. A character that ends a line in one
// reading and not in another, and that no editor or review shows, is a shape
// nobody can check by looking, and the write face answers those by leaving the
// note alone rather than by deciding which reading was meant. The first two
// assertions here are the premise: they say the reader agrees, so that if that
// ever stops being true this test says so instead of quietly changing meaning.
func TestRewriteStatusLineRefusesAnInvisibleBreakInsideQuotes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		in        string
		wantValue string
	}{
		{
			name:      "double quotes keep U+2028 inside the value",
			in:        "---\nstatus: \"draft\u2028tags:\"\n---\nbody\n",
			wantValue: "draft\u2028tags:",
		},
		{
			name:      "single quotes keep U+2029 inside the value",
			in:        "---\nstatus: 'draft\u2029tags:'\n---\nbody\n",
			wantValue: "draft\u2029tags:",
		},
		{
			// The reader folds this one to a space, so the value it reports is
			// not the bytes on the line at all — a third answer, and another
			// reason not to rewrite the run blind.
			name:      "double quotes fold U+0085 to a space",
			in:        "---\nstatus: \"draft\u0085tags:\"\n---\nbody\n",
			wantValue: "draft tags:",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			n := vault.Parse("note.md", []byte(tt.in))
			if n.FMDiagnostic != "" {
				t.Fatalf("the reader does not parse this at all: %s", n.FMDiagnostic)
			}
			if len(n.Frontmatter) != 1 {
				t.Fatalf("the reader read %d keys, want 1; a second key means the break ended the line after all", len(n.Frontmatter))
			}
			if got, _ := n.String("status"); got != tt.wantValue {
				t.Fatalf("the reader's value = %q, want %q", got, tt.wantValue)
			}
			if got, err := rewriteStatusLine([]byte(tt.in), "ready"); err == nil {
				t.Errorf("rewriteStatusLine() rewrote a value carrying a character nobody can see:\n%s", got)
			}
		})
	}
}

// TestRewriteStatusLineAcceptsTheSeparatorsYAMLAccepts holds the write face
// open on lines the reader already reads. A tab is separation space in YAML,
// so a note written with one parses, shows its status, and offers its
// transitions — and refusing it closed the write face on a note nothing else
// on the page said was wrong, with a recovery notice naming quoting, flow
// mappings and anchors, none of which such a line has.
func TestRewriteStatusLineAcceptsTheSeparatorsYAMLAccepts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "a tab after the colon separates the value",
			in:   "---\nstatus:\tdraft\n---\nbody\n",
			want: "---\nstatus:\tready\n---\nbody\n",
		},
		{
			name: "a tab before a comment ends the value",
			in:   "---\nstatus: draft\t# 已校對\n---\nbody\n",
			want: "---\nstatus: ready\t# 已校對\n---\nbody\n",
		},
		{
			name: "mixed spacing survives",
			in:   "---\nstatus: \t draft \t\n---\nbody\n",
			want: "---\nstatus: \t ready \t\n---\nbody\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := rewriteStatusLine([]byte(tt.in), "ready")
			if err != nil {
				t.Fatalf("rewriteStatusLine() = %v; the reader parses this line and offers its transitions", err)
			}
			if diff := cmp.Diff(tt.want, string(got)); diff != "" {
				t.Errorf("rewrite mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
