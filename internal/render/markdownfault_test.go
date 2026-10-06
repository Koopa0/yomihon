package render

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/yuin/goldmark/ast"
)

// TestMarkdownFaultRejectsAnInvalidAttribute keeps an extension fault on the
// renderer's error path, where the page can retain escaped source and a
// diagnostic. It never resumes an anchor with a malformed refusal attached.
func TestMarkdownFaultRejectsAnInvalidAttribute(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		value any
	}{
		{name: "missing value", value: nil},
		{name: "wrong type", value: "wrong attribute"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, entering := range []bool{true, false} {
				var output bytes.Buffer
				writer := bufio.NewWriter(&output)
				status, err := renderMarkdownFault(writer, ast.NewLink(), tt.value, entering)
				if err == nil {
					t.Fatal("invalid Markdown fault was accepted")
				}
				if status != ast.WalkStop || !strings.Contains(err.Error(), "want markdownFault") {
					t.Fatalf("renderMarkdownFault(%T, %t) = %v, %v, want stopped render with an attribute error", tt.value, entering, status, err)
				}
				if err := writer.Flush(); err != nil {
					t.Fatalf("flush: %v", err)
				}
				if got := output.String(); got != "" {
					t.Errorf("invalid attribute wrote %q, want no HTML", got)
				}
			}
		})
	}
}
