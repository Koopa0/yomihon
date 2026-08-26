package status

import (
	"testing"

	"github.com/google/go-cmp/cmp"
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
