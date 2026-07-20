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
