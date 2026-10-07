package vault_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/vault"
)

// TestParseKeepsNonScalarYAMLEvidence preserves the parser's original
// diagnostic and readable body for unsupported mapping keys.
func TestParseKeepsNonScalarYAMLEvidence(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, content string
	}{
		{"nested merge", "base: &b {x: 1}\nm:\n  <<: *b\n  ? [1]\n  : 2\n"},
		{"root key type error before nested merge", "? [0]\n: ignored\nbase: &b {x: 1}\nm:\n  <<: *b\n  ? [1]\n  : 2\n"},
		{"non-specific merge tag", "base: &b {x: 1}\nm:\n  ! <<: *b\n  ? [1]\n  : 2\n"},
		{"quoted text key", "m:\n  '<<': {x: 1}\n  ? [1]\n  : 2\n"},
		{"sibling merge", "base: &b {x: 1}\nmerged:\n  <<: *b\nm:\n  ? [1]\n  : 2\n"},
		{"earlier ordinary key", "base: &b {x: 1}\nordinary:\n  ? [1]\n  : 2\nunsupported:\n  <<: *b\n  ? [2]\n  : 3\n"},
		{"cyclic merge source", "m:\n  c: plain\n  <<:\n    c: &c {self: *c}\n    t:\n      ? [1]\n      : 2\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := vault.Parse("Writing/Bad.md", []byte("---\n"+tc.content+"---\nReadable body.\n"))
			want := &vault.Note{
				RelPath:        "Writing/Bad.md",
				HasFrontmatter: true,
				FMDiagnostic:   "frontmatter is not valid YAML: yaml: invalid map key: []interface {}{1}",
				Body:           "Readable body.\n",
				BodyLine:       strings.Count(tc.content, "\n") + 3,
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("caught: Parse(%q) mismatch (-want +got):\n%s", tc.content, diff)
			}
		})
	}
}

// TestParsePreservesDuplicateYAMLError keeps a refused mapping from being
// reclassified by an unsupported form the decoder never entered.
func TestParsePreservesDuplicateYAMLError(t *testing.T) {
	t.Parallel()
	content := "---\nbase: &b {x: 1}\nduplicate: 1\nduplicate: 2\nunsupported:\n  <<: *b\n  ? [1]\n  : 2\n---\nReadable body.\n"
	got := vault.Parse("Writing/Bad.md", []byte(content))
	want := &vault.Note{
		RelPath:        "Writing/Bad.md",
		HasFrontmatter: true,
		FMDiagnostic:   "frontmatter is not valid YAML: yaml: unmarshal errors:\n  line 4: mapping key \"duplicate\" already defined at line 3",
		Body:           "Readable body.\n",
		BodyLine:       10,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("caught: Parse(duplicate YAML key) mismatch (-want +got):\n%s", diff)
	}
}
