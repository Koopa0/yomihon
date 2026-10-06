package vault_test

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/vault"
)

func TestLeadingByteOrderMarkBelongsToTheFileNotTheBody(t *testing.T) {
	t.Parallel()
	const mark = "\xef\xbb\xbf"
	tests := []struct {
		name  string
		input string
		want  vault.FrontmatterSplit
		found bool
	}{
		{name: "heading without frontmatter", input: mark + "# Title\n", want: vault.FrontmatterSplit{Body: []byte("# Title\n"), BodyStartLine: 1}},
		{name: "only a mark", input: mark, want: vault.FrontmatterSplit{Body: []byte{}, BodyStartLine: 1}},
		{name: "only the first mark", input: mark + mark + "# Title\n", want: vault.FrontmatterSplit{Body: []byte(mark + "# Title\n"), BodyStartLine: 1}},
		{name: "interior mark stays text", input: "x" + mark + "# Title\n", want: vault.FrontmatterSplit{Body: []byte("x" + mark + "# Title\n"), BodyStartLine: 1}},
		{name: "unclosed fence is still body", input: mark + "---\ntitle: Title\n", want: vault.FrontmatterSplit{Body: []byte("---\ntitle: Title\n"), BodyStartLine: 1}},
		{name: "original LF offsets", input: mark + "---\nstatus: draft\n---\n# Title\n", found: true, want: vault.FrontmatterSplit{Content: []byte("status: draft\n"), Body: []byte("# Title\n"), ContentStart: 7, BodyStartLine: 4}},
		{name: "original CRLF offsets", input: mark + "---\r\nstatus: draft\r\n...\r\n# Title\r\n", found: true, want: vault.FrontmatterSplit{Content: []byte("status: draft\r\n"), Body: []byte("# Title\r\n"), ContentStart: 8, BodyStartLine: 4}},
		{name: "body mark after frontmatter stays", input: mark + "---\n---\n" + mark + "# Title\n", found: true, want: vault.FrontmatterSplit{Content: []byte{}, Body: []byte(mark + "# Title\n"), ContentStart: 7, BodyStartLine: 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := []byte(tt.input)
			before := bytes.Clone(data)
			got, found := vault.SplitFrontmatter(data)
			if found != tt.found {
				t.Fatalf("SplitFrontmatter(%q) found = %t, want %t", tt.input, found, tt.found)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("SplitFrontmatter(%q) mismatch (-want +got):\n%s", tt.input, diff)
			}
			if !bytes.Equal(before, data) {
				t.Fatal("SplitFrontmatter changed the original file bytes")
			}
			note := vault.Parse("note.md", data)
			if note.Body != string(tt.want.Body) || note.BodyLine != tt.want.BodyStartLine {
				t.Errorf("Parse(%q) body/line = %q/%d, want %q/%d", tt.input, note.Body, note.BodyLine, tt.want.Body, tt.want.BodyStartLine)
			}
		})
	}
}

func TestByteOrderMarkKeepsStatusSpansInTheOriginalBytes(t *testing.T) {
	t.Parallel()
	for _, input := range []string{"\xef\xbb\xbf---\nstatus: draft\n---\n# Title\n", "\xef\xbb\xbf---\r\nstatus: draft\r\n---\r\n# Title\n"} {
		start, end, ok := vault.StatusValueSpan([]byte(input))
		if !ok {
			t.Fatalf("StatusValueSpan(%q) did not locate the value", input)
		}
		wantStart := 15
		if input[6] == '\r' {
			wantStart = 16
		}
		if start != wantStart || end != wantStart+5 || input[start:end] != "draft" {
			t.Errorf("StatusValueSpan(%q) = [%d:%d] %q, want [%d:%d] draft", input, start, end, input[start:end], wantStart, wantStart+5)
		}
	}
}
